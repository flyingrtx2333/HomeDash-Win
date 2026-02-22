package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/pkg/policy/featuregate"
	"github.com/fatedier/frp/pkg/policy/security"
	frplog "github.com/fatedier/frp/pkg/util/log"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/sys/windows/registry"
)

// FrpcProxy 单个代理配置
type FrpcProxy struct {
	Name       string `toml:"name" json:"name"`
	Type       string `toml:"type" json:"type"`
	LocalIP    string `toml:"localIP" json:"localIP"`
	LocalPort  int    `toml:"localPort" json:"localPort"`
	RemotePort int    `toml:"remotePort" json:"remotePort"`
}

// FrpcConfigParsed 解析后的 frpc 配置（用于简单模式）
type FrpcConfigParsed struct {
	ServerAddr string      `toml:"serverAddr" json:"serverAddr"`
	ServerPort int         `toml:"serverPort" json:"serverPort"`
	Auth       FrpcAuth    `toml:"auth" json:"auth"`
	Proxies    []FrpcProxy `toml:"proxies" json:"proxies"`
}

// FrpcAuth auth 配置
type FrpcAuth struct {
	Token string `toml:"token" json:"token"`
}

const frpcAutoStartName = "HomeDash-Frpc"

const createNoWindow = 0x08000000 // CREATE_NO_WINDOW (for use by other handlers e.g. websites)

// 内嵌 frp 客户端运行状态
var (
	frpcServiceMu  sync.Mutex
	frpcService    *client.Service
	frpcCancel     context.CancelFunc
	frpcRunning    bool
	frpcRunningPid int32 // 本进程 PID，便于前端显示
)

// getFrpcPaths 获取 frpc.toml 路径（内嵌实现不再需要 exe）
func getFrpcPaths() (tomlPath string, err error) {
	exeDir := "."
	if wd, e := os.Getwd(); e == nil {
		exeDir = wd
		exeDir = filepath.Join(exeDir, "data")
		log.Printf("[FRPC] 使用工作目录: %s", wd)
	}
	tomlPath = filepath.Join(exeDir, "frpc.toml")
	return tomlPath, nil
}

// GetFrpcConfig 读取 frpc.toml 内容（原始字符串）
func GetFrpcConfig(c *gin.Context) {
	tomlPath, err := getFrpcPaths()
	if err != nil {
		c.JSON(500, gin.H{"error": "获取配置路径失败"})
		return
	}

	data, err := os.ReadFile(tomlPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(200, gin.H{"config": ""})
			return
		}
		c.JSON(500, gin.H{"error": "读取配置失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"config": string(data)})
}

// GetFrpcConfigParsed 读取并解析 frpc.toml，返回结构化 JSON（用于简单模式）
func GetFrpcConfigParsed(c *gin.Context) {
	tomlPath, err := getFrpcPaths()
	if err != nil {
		c.JSON(500, gin.H{"error": "获取配置路径失败"})
		return
	}

	data, err := os.ReadFile(tomlPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(200, gin.H{"serverAddr": "", "serverPort": 7000, "token": "", "proxies": []interface{}{}})
			return
		}
		c.JSON(500, gin.H{"error": "读取配置失败: " + err.Error()})
		return
	}

	var cfg FrpcConfigParsed
	if err := toml.Unmarshal(data, &cfg); err != nil {
		c.JSON(500, gin.H{"error": "解析配置失败: " + err.Error()})
		return
	}

	res := gin.H{
		"serverAddr": cfg.ServerAddr,
		"serverPort": cfg.ServerPort,
		"token":      cfg.Auth.Token,
		"proxies":    cfg.Proxies,
	}
	if cfg.Proxies == nil {
		res["proxies"] = []FrpcProxy{}
	}
	c.JSON(200, res)
}

// UpdateFrpcConfig 保存 frpc.toml 内容
func UpdateFrpcConfig(c *gin.Context) {
	var rawReq struct {
		Config     string      `json:"config"`
		ServerAddr string      `json:"serverAddr"`
		ServerPort int         `json:"serverPort"`
		Token      string      `json:"token"`
		Proxies    []FrpcProxy `json:"proxies"`
	}
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	var tomlBytes []byte
	if rawReq.Config != "" {
		tomlBytes = []byte(rawReq.Config)
	} else {
		cfg := FrpcConfigParsed{
			ServerAddr: rawReq.ServerAddr,
			ServerPort: rawReq.ServerPort,
			Auth:       FrpcAuth{Token: rawReq.Token},
			Proxies:    rawReq.Proxies,
		}
		if cfg.Proxies == nil {
			cfg.Proxies = []FrpcProxy{}
		}
		var err error
		tomlBytes, err = toml.Marshal(cfg)
		if err != nil {
			c.JSON(500, gin.H{"error": "生成配置失败: " + err.Error()})
			return
		}
	}

	tomlPath, err := getFrpcPaths()
	if err != nil {
		c.JSON(500, gin.H{"error": "获取配置路径失败"})
		return
	}

	dir := filepath.Dir(tomlPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(500, gin.H{"error": "创建配置目录失败"})
		return
	}

	if err := os.WriteFile(tomlPath, tomlBytes, 0644); err != nil {
		c.JSON(500, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// checkFrpcProcess 检测内嵌 frpc 是否在运行
func checkFrpcProcess() (running bool, pid int32) {
	frpcServiceMu.Lock()
	defer frpcServiceMu.Unlock()
	return frpcRunning, frpcRunningPid
}

// GetFrpcStatus 获取 frpc 运行状态
func GetFrpcStatus(c *gin.Context) {
	running, pid := checkFrpcProcess()
	c.JSON(200, gin.H{"running": running, "pid": pid})
}

// startFrpcService 从 tomlPath 加载配置并启动内嵌 frp 客户端（调用方需已持有 frpcServiceMu）
func startFrpcService(tomlPath string) error {
	cfg, proxyCfgs, visitorCfgs, _, err := config.LoadClientConfig(tomlPath, true)
	if err != nil {
		return err
	}

	if len(cfg.FeatureGates) > 0 {
		if err := featuregate.SetFromMap(cfg.FeatureGates); err != nil {
			return err
		}
	}

	unsafeFeatures := security.NewUnsafeFeatures(nil)
	warning, err := validation.ValidateAllClientConfig(cfg, proxyCfgs, visitorCfgs, unsafeFeatures)
	if warning != nil {
		log.Printf("[FRPC] 配置警告: %v", warning)
	}
	if err != nil {
		return err
	}

	frplog.InitLogger(cfg.Log.To, cfg.Log.Level, int(cfg.Log.MaxDays), cfg.Log.DisablePrintColor)

	svr, err := client.NewService(client.ServiceOptions{
		Common:         cfg,
		ProxyCfgs:      proxyCfgs,
		VisitorCfgs:    visitorCfgs,
		UnsafeFeatures: unsafeFeatures,
		ConfigFilePath: tomlPath,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	frpcService = svr
	frpcCancel = cancel
	frpcRunning = true
	frpcRunningPid = int32(os.Getpid())

	go func() {
		defer func() {
			frpcServiceMu.Lock()
			frpcRunning = false
			frpcRunningPid = 0
			frpcService = nil
			frpcCancel = nil
			frpcServiceMu.Unlock()
		}()
		if runErr := svr.Run(ctx); runErr != nil && runErr != context.Canceled {
			log.Printf("[FRPC] 服务退出: %v", runErr)
		}
	}()
	return nil
}

// StartFrpc 启动内嵌 frpc
func StartFrpc(c *gin.Context) {
	log.Printf("[FRPC] StartFrpc 被调用")
	tomlPath, err := getFrpcPaths()
	if err != nil {
		log.Printf("[FRPC] 获取路径失败: %v", err)
		c.JSON(500, gin.H{"error": "获取路径失败"})
		return
	}

	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		log.Printf("[FRPC] 配置文件不存在: %s", tomlPath)
		c.JSON(400, gin.H{"error": "frpc.toml 不存在，请先保存配置"})
		return
	}

	frpcServiceMu.Lock()
	if frpcRunning {
		frpcServiceMu.Unlock()
		log.Printf("[FRPC] frpc 已在运行")
		c.JSON(400, gin.H{"error": "frpc 已在运行中"})
		return
	}

	err = startFrpcService(tomlPath)
	frpcServiceMu.Unlock()

	if err != nil {
		log.Printf("[FRPC] 启动失败: %v", err)
		c.JSON(500, gin.H{"error": "启动失败: " + err.Error()})
		return
	}
	log.Printf("[FRPC] 启动成功（内嵌 fatedier/frp）")
	c.JSON(200, gin.H{"success": true})
}

// StopFrpc 停止内嵌 frpc
func StopFrpc(c *gin.Context) {
	frpcServiceMu.Lock()
	svr := frpcService
	cancel := frpcCancel
	frpcServiceMu.Unlock()

	if svr == nil || cancel == nil {
		c.JSON(200, gin.H{"success": true, "message": "frpc 未运行"})
		return
	}

	svr.GracefulClose(500 * time.Millisecond)
	cancel()
	c.JSON(200, gin.H{"success": true})
}

// GetFrpcAutoStart 获取 frpc 开机自启状态
func GetFrpcAutoStart(c *gin.Context) {
	enabled := isFrpcAutoStartEnabled()
	c.JSON(200, gin.H{"autoStart": enabled})
}

func isFrpcAutoStartEnabled() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, appAutoStartKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(frpcAutoStartName)
	return err == nil
}

// UpdateFrpcAutoStart 设置 frpc 开机自启（仅记录偏好，由本进程启动时自动拉起内嵌 frpc）
func UpdateFrpcAutoStart(c *gin.Context) {
	log.Printf("[FRPC] UpdateFrpcAutoStart 被调用")
	var req struct {
		AutoStart bool `json:"autoStart"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[FRPC] 解析请求失败: %v", err)
		c.JSON(400, gin.H{"error": "无效的请求"})
		return
	}
	log.Printf("[FRPC] 开机自启: %v", req.AutoStart)

	if runtime.GOOS != "windows" {
		c.JSON(400, gin.H{"error": "仅支持 Windows 系统"})
		return
	}

	if err := setFrpcAutoStart(req.AutoStart); err != nil {
		log.Printf("[FRPC] 设置注册表失败: %v", err)
		c.JSON(500, gin.H{"error": "设置失败: " + err.Error()})
		return
	}
	log.Printf("[FRPC] 开机自启已%s", map[bool]string{true: "启用", false: "禁用"}[req.AutoStart])

	if req.AutoStart {
		if running, _ := checkFrpcProcess(); !running {
			log.Printf("[FRPC] 启用后尝试立即启动内嵌 frpc")
			tomlPath, err := getFrpcPaths()
			if err == nil {
				if _, statErr := os.Stat(tomlPath); statErr == nil {
					frpcServiceMu.Lock()
					if !frpcRunning {
						_ = startFrpcService(tomlPath)
					}
					frpcServiceMu.Unlock()
				}
			}
		}
	}

	c.JSON(200, gin.H{"success": true})
}

func setFrpcAutoStart(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, appAutoStartKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer k.Close()

	if enabled {
		return k.SetStringValue(frpcAutoStartName, "1")
	}
	return k.DeleteValue(frpcAutoStartName)
}

// MaybeLaunchFrpcOnStartup 若已启用开机自启，则启动内嵌 frpc
func MaybeLaunchFrpcOnStartup() {
	log.Printf("[FRPC] MaybeLaunchFrpcOnStartup 被调用")
	if runtime.GOOS != "windows" {
		log.Printf("[FRPC] 非 Windows 系统，跳过")
		return
	}
	if !isFrpcAutoStartEnabled() {
		log.Printf("[FRPC] 开机自启未启用，跳过")
		return
	}
	if running, _ := checkFrpcProcess(); running {
		log.Printf("[FRPC] frpc 已在运行")
		return
	}

	tomlPath, err := getFrpcPaths()
	if err != nil {
		log.Printf("[FRPC] 获取路径失败: %v", err)
		return
	}
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		log.Printf("[FRPC] 配置文件不存在: %s", tomlPath)
		return
	}

	frpcServiceMu.Lock()
	if !frpcRunning {
		err = startFrpcService(tomlPath)
	}
	frpcServiceMu.Unlock()

	if err != nil {
		log.Printf("[FRPC] 启动失败: %v", err)
		return
	}
	log.Printf("[FRPC] 开机自启已启动内嵌 frpc")
}
