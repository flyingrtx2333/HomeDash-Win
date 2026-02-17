package handlers

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
	"github.com/shirou/gopsutil/v3/process"
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

const createNoWindow = 0x08000000 // CREATE_NO_WINDOW

// getFrpcPaths 获取 frpc.exe 和 frpc.toml 路径
// 使用当前工作目录的web
func getFrpcPaths() (exePath, tomlPath string, err error) {
	exeDir := "."
	if wd, e := os.Getwd(); e == nil {
		exeDir = wd
		exeDir = filepath.Join(exeDir, "web")
		log.Printf("[FRPC] 使用工作目录: %s", wd)
	}

	exePath = filepath.Join(exeDir, "frpc.exe")
	tomlPath = filepath.Join(exeDir, "frpc.toml")
	return exePath, tomlPath, nil
}

// GetFrpcConfig 读取 frpc.toml 内容（原始字符串）
func GetFrpcConfig(c *gin.Context) {
	_, tomlPath, err := getFrpcPaths()
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
	_, tomlPath, err := getFrpcPaths()
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

	// 扁平化返回，方便前端使用
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
// 支持两种格式：1) config 原始 toml 字符串（专业模式） 2) serverAddr/serverPort/token/proxies 结构化（简单模式）
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
		// 专业模式：直接使用原始 toml
		tomlBytes = []byte(rawReq.Config)
	} else {
		// 简单模式：从结构化数据生成 toml
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

	_, tomlPath, err := getFrpcPaths()
	if err != nil {
		c.JSON(500, gin.H{"error": "获取配置路径失败"})
		return
	}

	// 确保目录存在
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

// checkFrpcProcess 检测 frpc 进程状态
func checkFrpcProcess() (running bool, pid int32) {
	processes, err := process.Processes()
	if err != nil {
		return false, 0
	}

	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if strings.EqualFold(name, "frpc.exe") {
			return true, p.Pid
		}
	}

	return false, 0
}

// GetFrpcStatus 获取 frpc 运行状态
func GetFrpcStatus(c *gin.Context) {
	running, pid := checkFrpcProcess()
	c.JSON(200, gin.H{"running": running, "pid": pid})
}

// StartFrpc 启动 frpc
func StartFrpc(c *gin.Context) {
	log.Printf("[FRPC] StartFrpc 被调用")
	exePath, tomlPath, err := getFrpcPaths()
	if err != nil {
		log.Printf("[FRPC] 获取路径失败: %v", err)
		c.JSON(500, gin.H{"error": "获取路径失败"})
		return
	}

	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		log.Printf("[FRPC] frpc.exe 不存在: %s", exePath)
		c.JSON(400, gin.H{"error": "frpc.exe 不存在，请将 frpc.exe 放置于程序目录"})
		return
	}

	if running, pid := checkFrpcProcess(); running {
		log.Printf("[FRPC] frpc 已在运行, pid=%d", pid)
		c.JSON(400, gin.H{"error": "frpc 已在运行中"})
		return
	}

	cmd := exec.Command(exePath, "-c", tomlPath)
	cmd.Dir = filepath.Dir(exePath)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: createNoWindow,
		}
	}
	log.Printf("[FRPC] 执行启动: %s -c %s, Dir=%s", exePath, tomlPath, cmd.Dir)
	if err := cmd.Start(); err != nil {
		log.Printf("[FRPC] 启动失败: %v", err)
		c.JSON(500, gin.H{"error": "启动失败: " + err.Error()})
		return
	}
	log.Printf("[FRPC] 启动成功, pid=%d", cmd.Process.Pid)

	c.JSON(200, gin.H{"success": true})
}

// StopFrpc 停止 frpc
func StopFrpc(c *gin.Context) {
	running, pid := checkFrpcProcess()
	if !running {
		c.JSON(200, gin.H{"success": true, "message": "frpc 未运行"})
		return
	}

	if err := stopFrpcProcess(pid); err != nil {
		c.JSON(500, gin.H{"error": "停止失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func stopFrpcProcess(pid int32) error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid))
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
			return cmd.Run()
		}
		return nil
	}

	proc, err := process.NewProcess(pid)
	if err != nil {
		return err
	}
	return proc.Terminate()
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

// UpdateFrpcAutoStart 设置 frpc 开机自启
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

	exePath, tomlPath, err := getFrpcPaths()
	if err != nil {
		c.JSON(500, gin.H{"error": "获取路径失败"})
		return
	}

	if req.AutoStart {
		if _, err := os.Stat(exePath); os.IsNotExist(err) {
			log.Printf("[FRPC] 启用失败: frpc.exe 不存在 %s", exePath)
			c.JSON(400, gin.H{"error": "frpc.exe 不存在，请先将 frpc.exe 放置于程序目录"})
			return
		}
	}

	if err := setFrpcAutoStart(exePath, tomlPath, req.AutoStart); err != nil {
		log.Printf("[FRPC] 设置注册表失败: %v", err)
		c.JSON(500, gin.H{"error": "设置失败: " + err.Error()})
		return
	}
	log.Printf("[FRPC] 开机自启已%s", map[bool]string{true: "启用", false: "禁用"}[req.AutoStart])

	// 启用时若 frpc 未运行，立即启动
	if req.AutoStart {
		if running, _ := checkFrpcProcess(); !running {
			log.Printf("[FRPC] 启用后尝试立即启动 frpc")
			cmd := exec.Command(exePath, "-c", tomlPath)
			cmd.Dir = filepath.Dir(exePath)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				HideWindow:    true,
				CreationFlags: createNoWindow,
			}
			if err := cmd.Start(); err != nil {
				log.Printf("[FRPC] 立即启动失败: %v", err)
			} else {
				log.Printf("[FRPC] 立即启动成功, pid=%d", cmd.Process.Pid)
			}
		}
	}

	c.JSON(200, gin.H{"success": true})
}

func setFrpcAutoStart(exePath, tomlPath string, enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, appAutoStartKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer k.Close()

	if enabled {
		absExe, err := filepath.Abs(exePath)
		if err != nil {
			return err
		}
		absToml, err := filepath.Abs(tomlPath)
		if err != nil {
			return err
		}
		// 带引号避免路径含空格出错
		value := fmt.Sprintf(`"%s" -c "%s"`, absExe, absToml)
		return k.SetStringValue(frpcAutoStartName, value)
	}

	return k.DeleteValue(frpcAutoStartName)
}

// MaybeLaunchFrpcOnStartup 若已启用开机自启，则启动 frpc
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
	if running, pid := checkFrpcProcess(); running {
		log.Printf("[FRPC] frpc 已在运行, pid=%d", pid)
		return
	}

	exePath, tomlPath, err := getFrpcPaths()
	if err != nil {
		log.Printf("[FRPC] 获取路径失败: %v", err)
		return
	}
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		log.Printf("[FRPC] frpc.exe 不存在: %s", exePath)
		return
	}

	cmd := exec.Command(exePath, "-c", tomlPath)
	cmd.Dir = filepath.Dir(exePath)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: createNoWindow,
		}
	}
	log.Printf("[FRPC] 启动时自动运行: %s -c %s", exePath, tomlPath)
	if err := cmd.Start(); err != nil {
		log.Printf("[FRPC] 启动失败: %v", err)
		return
	}
	log.Printf("[FRPC] 启动成功, pid=%d", cmd.Process.Pid)
}
