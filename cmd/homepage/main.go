package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/net/webdav"
	"golang.org/x/sys/windows/registry"

	"homecloud-ultimate/internal/monitor"
)

const defaultPort = "29678"

// BackgroundInfo 背景图信息
type BackgroundInfo struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Thumb string `json:"thumb"`
}

// UserSettings 用户设置
type UserSettings struct {
	ServerIP        string `json:"serverIp"`
	BackgroundURL   string `json:"backgroundUrl"`
	Theme           string `json:"theme"`            // "dark" | "light"
	WebdavRoot      string `json:"webdavRoot"`      // WebDAV 挂载根目录
	ComfyUIServerURL string `json:"comfyuiServerUrl"` // ComfyUI服务器地址
}

// ServiceCard 服务卡片
type ServiceCard struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Port          int    `json:"port"`
	Icon          string `json:"icon"`
	Enabled       bool   `json:"enabled"`
	LaunchPath    string `json:"launchPath"`    // 启动路径（可执行文件路径，向后兼容）
	LaunchCommand string `json:"launchCommand"` // 启动命令（支持参数）
	ProcessName   string `json:"processName"`   // 进程名（用于检测和停止）
	AutoStart     bool   `json:"autoStart"`      // 是否开机自启
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}

// AppConfig 应用配置
type AppConfig struct {
	Port      string `json:"port"`      // 应用端口
	AutoStart bool   `json:"autoStart"` // 是否开机自启
}

// PingResult 连通性检测结果
type PingResult struct {
	ID      string `json:"id"`
	Status  string `json:"status"`  // "ok" | "slow" | "error"
	Latency int64  `json:"latency"` // 毫秒
	Message string `json:"message,omitempty"`
}

// 推荐服务模板
var defaultServiceTemplates = []ServiceCard{
	{ID: "lucky", Name: "Lucky", Description: "DDNS、反向代理、证书自动化", Port: 16601, Icon: "🍀", Enabled: true},
	{ID: "alist", Name: "Alist", Description: "多网盘整合与 WebDAV", Port: 5244, Icon: "/static/images/alist.png", Enabled: true},
	{ID: "immich", Name: "Immich", Description: "相册备份与 AI 检索", Port: 2283, Icon: "/static/images/immich.png", Enabled: true},
	{ID: "jellyfin", Name: "Jellyfin", Description: "媒体管理与播放", Port: 8096, Icon: "/static/images/jellyfin.jpg", Enabled: true},
	{ID: "comfyui", Name: "ComfyUI", Description: "AI 图像生成工作流", Port: 28000, Icon: "/static/images/comfyui.webp", Enabled: true},
	{ID: "rustdesk", Name: "RustDesk", Description: "开源远程桌面控制", Port: 0, Icon: "/static/images/rustdesk.png", Enabled: false},
	{ID: "sunshine", Name: "Sunshine", Description: "游戏串流服务端", Port: 0, Icon: "☀️", Enabled: false},
	{ID: "moonlight", Name: "Moonlight", Description: "游戏串流客户端", Port: 0, Icon: "🌙", Enabled: false},
}

// FileInfo 文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

// DockerContainer Docker 容器信息
type DockerContainer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Ports   string `json:"ports"`
	Created string `json:"created"`
}

var (
	webDir       string
	settingsFile string
	servicesFile string
	settingsMu   sync.RWMutex
	servicesMu   sync.RWMutex
	monitorHub   *monitor.Hub
	webdavRoot   string // WebDAV 根目录
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	webDir = resolveWebDir()
	settingsFile = filepath.Join(webDir, "settings.json")
	servicesFile = filepath.Join(webDir, "services.json")

	// WebDAV 根目录：优先从设置文件加载，否则使用环境变量或默认用户目录
	webdavRoot = os.Getenv("WEBDAV_ROOT")
	if webdavRoot == "" {
		homeDir, _ := os.UserHomeDir()
		webdavRoot = homeDir
	}

	// 初始化默认服务（如果不存在）
	initDefaultServices()

	// 从设置文件加载 WebDAV 根目录
	savedSettings := loadSettings()
	if savedSettings.WebdavRoot != "" {
		webdavRoot = savedSettings.WebdavRoot
	}

	// 初始化监控 Hub
	monitorHub = monitor.NewHub()
	go monitorHub.Run()

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(webDir, "index.html"))
	})

	// 静态文件服务
	router.StaticFS("/static", http.Dir(webDir))

	// 背景图列表 API
	router.GET("/api/backgrounds", func(c *gin.Context) {
		backgrounds := getBackgroundList(webDir)
		c.JSON(http.StatusOK, backgrounds)
	})

	// 用户设置 API
	router.GET("/api/settings", func(c *gin.Context) {
		settings := loadSettings()
		c.JSON(http.StatusOK, settings)
	})

	router.POST("/api/settings", func(c *gin.Context) {
		var settings UserSettings
		if err := c.ShouldBindJSON(&settings); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
			return
		}
		// 如果设置了新的 WebDAV 根目录，更新全局变量
		if settings.WebdavRoot != "" {
			webdavRoot = settings.WebdavRoot
		}
		if err := saveSettings(settings); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存设置失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// WebDAV 根目录 API
	router.GET("/api/webdav-root", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"root": webdavRoot})
	})

	router.POST("/api/webdav-root", func(c *gin.Context) {
		var req struct {
			Root string `json:"root"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
			return
		}

		// 验证路径是否存在
		if _, err := os.Stat(req.Root); os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "目录不存在"})
			return
		}

		// 更新全局变量和设置
		webdavRoot = req.Root
		settings := loadSettings()
		settings.WebdavRoot = req.Root
		saveSettings(settings)

		c.JSON(http.StatusOK, gin.H{"success": true, "root": webdavRoot})
	})

	// 服务卡片 CRUD API
	router.GET("/api/services", func(c *gin.Context) {
		services := loadServices()
		c.JSON(http.StatusOK, services)
	})

	router.POST("/api/services", func(c *gin.Context) {
		var service ServiceCard
		if err := c.ShouldBindJSON(&service); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
			return
		}

		// 生成 ID 和时间戳
		service.ID = uuid.New().String()[:8]
		service.CreatedAt = time.Now().UnixMilli()
		service.UpdatedAt = service.CreatedAt
		service.Enabled = true

		services := loadServices()
		services = append(services, service)

		if err := saveServices(services); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
			return
		}

		c.JSON(http.StatusOK, service)
	})

	router.PUT("/api/services/:id", func(c *gin.Context) {
		id := c.Param("id")
		var updated ServiceCard
		if err := c.ShouldBindJSON(&updated); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
			return
		}

		services := loadServices()
		found := false
		for i, s := range services {
			if s.ID == id {
				updated.ID = id
				updated.CreatedAt = s.CreatedAt
				updated.UpdatedAt = time.Now().UnixMilli()
				services[i] = updated
				found = true
				break
			}
		}

		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "服务不存在"})
			return
		}

		if err := saveServices(services); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
			return
		}

		c.JSON(http.StatusOK, updated)
	})

	router.DELETE("/api/services/:id", func(c *gin.Context) {
		id := c.Param("id")
		services := loadServices()
		newServices := make([]ServiceCard, 0)
		found := false

		for _, s := range services {
			if s.ID == id {
				found = true
			} else {
				newServices = append(newServices, s)
			}
		}

		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "服务不存在"})
			return
		}

		if err := saveServices(newServices); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// WebSocket 监控端点
	router.GET("/ws/monitor", func(c *gin.Context) {
		monitorHub.HandleWebSocket(c.Writer, c.Request)
	})

	// 导入推荐模板 API
	router.POST("/api/services/import-template", func(c *gin.Context) {
		services := loadServices()
		now := time.Now().UnixMilli()

		for _, tmpl := range defaultServiceTemplates {
			// 检查是否已存在同名服务
			exists := false
			for _, s := range services {
				if s.ID == tmpl.ID || s.Name == tmpl.Name {
					exists = true
					break
				}
			}
			if !exists {
				newService := tmpl
				newService.CreatedAt = now
				newService.UpdatedAt = now
				services = append(services, newService)
			}
		}

		if err := saveServices(services); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "count": len(services)})
	})

	// 连通性检测 API - 批量检测所有服务
	router.GET("/api/ping-all", func(c *gin.Context) {
		services := loadServices()
		settings := loadSettings()
		serverIP := settings.ServerIP
		if serverIP == "" {
			serverIP = "localhost"
		}

		results := make([]PingResult, 0, len(services))
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, s := range services {
			if !s.Enabled || s.Port == 0 {
				continue
			}
			wg.Add(1)
			go func(service ServiceCard) {
				defer wg.Done()
				result := pingService(service.ID, serverIP, service.Port)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}(s)
		}

		wg.Wait()
		c.JSON(http.StatusOK, results)
	})

	// 连通性检测 API - 单个服务
	router.GET("/api/services/:id/ping", func(c *gin.Context) {
		id := c.Param("id")
		services := loadServices()
		settings := loadSettings()
		serverIP := settings.ServerIP
		if serverIP == "" {
			serverIP = "localhost"
		}

		var targetService *ServiceCard
		for _, s := range services {
			if s.ID == id {
				targetService = &s
				break
			}
		}

		if targetService == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "服务不存在"})
			return
		}

		if !targetService.Enabled || targetService.Port == 0 {
			c.JSON(http.StatusOK, PingResult{
				ID:      id,
				Status:  "disabled",
				Latency: 0,
				Message: "服务未启用或无端口",
			})
			return
		}

		result := pingService(id, serverIP, targetService.Port)
		c.JSON(http.StatusOK, result)
	})

	// Favicon 抓取 API
	router.GET("/api/favicon", func(c *gin.Context) {
		targetURL := c.Query("url")
		if targetURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 url 参数"})
			return
		}

		faviconURL, err := fetchFavicon(targetURL)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}

		// 下载并保存 favicon
		savedPath, err := downloadFavicon(faviconURL, webDir)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "icon": savedPath})
	})

	// 进程列表 API
	router.GET("/api/processes", func(c *gin.Context) {
		processes := monitor.GetTopProcesses(20)
		c.JSON(http.StatusOK, processes)
	})

	// 图标上传 API
	router.POST("/api/upload-icon", func(c *gin.Context) {
		file, err := c.FormFile("icon")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "未找到上传文件"})
			return
		}

		// 检查文件类型
		ext := strings.ToLower(filepath.Ext(file.Filename))
		validExts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true, ".svg": true}
		if !validExts[ext] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件格式"})
			return
		}

		// 限制文件大小 (2MB)
		if file.Size > 2*1024*1024 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件过大，最大 2MB"})
			return
		}

		// 创建 icons 目录
		iconsDir := filepath.Join(webDir, "icons")
		if err := os.MkdirAll(iconsDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败"})
			return
		}

		// 生成唯一文件名
		filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join(iconsDir, filename)

		if err := c.SaveUploadedFile(file, savePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "icon": "/static/icons/" + filename})
	})

	// ========== 文件管理 API ==========
	// 获取文件列表
	router.GET("/api/files", func(c *gin.Context) {
		reqPath := c.Query("path")
		if reqPath == "" {
			reqPath = "/"
		}

		// 安全检查：防止路径遍历
		fullPath := filepath.Join(webdavRoot, filepath.Clean(reqPath))
		if !strings.HasPrefix(fullPath, webdavRoot) {
			c.JSON(http.StatusForbidden, gin.H{"error": "禁止访问"})
			return
		}

		entries, err := os.ReadDir(fullPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取目录失败: " + err.Error()})
			return
		}

		var files []FileInfo
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			relPath := filepath.Join(reqPath, entry.Name())
			files = append(files, FileInfo{
				Name:    entry.Name(),
				Path:    filepath.ToSlash(relPath),
				IsDir:   entry.IsDir(),
				Size:    info.Size(),
				ModTime: info.ModTime().UnixMilli(),
			})
		}

		// 排序：文件夹在前，然后按名称排序
		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDir != files[j].IsDir {
				return files[i].IsDir
			}
			return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		})

		c.JSON(http.StatusOK, gin.H{
			"path":  reqPath,
			"root":  webdavRoot,
			"files": files,
		})
	})

	// 创建文件夹
	router.POST("/api/files/mkdir", func(c *gin.Context) {
		var req struct {
			Path string `json:"path"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
			return
		}

		fullPath := filepath.Join(webdavRoot, filepath.Clean(req.Path))
		if !strings.HasPrefix(fullPath, webdavRoot) {
			c.JSON(http.StatusForbidden, gin.H{"error": "禁止访问"})
			return
		}

		if err := os.MkdirAll(fullPath, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文件夹失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// 删除文件/文件夹
	router.DELETE("/api/files", func(c *gin.Context) {
		reqPath := c.Query("path")
		if reqPath == "" || reqPath == "/" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的路径"})
			return
		}

		fullPath := filepath.Join(webdavRoot, filepath.Clean(reqPath))
		if !strings.HasPrefix(fullPath, webdavRoot) || fullPath == webdavRoot {
			c.JSON(http.StatusForbidden, gin.H{"error": "禁止删除"})
			return
		}

		if err := os.RemoveAll(fullPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// 上传文件
	router.POST("/api/files/upload", func(c *gin.Context) {
		targetPath := c.PostForm("path")
		if targetPath == "" {
			targetPath = "/"
		}

		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "未找到上传文件"})
			return
		}

		fullPath := filepath.Join(webdavRoot, filepath.Clean(targetPath), file.Filename)
		if !strings.HasPrefix(fullPath, webdavRoot) {
			c.JSON(http.StatusForbidden, gin.H{"error": "禁止访问"})
			return
		}

		if err := c.SaveUploadedFile(file, fullPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// 下载文件
	router.GET("/api/files/download", func(c *gin.Context) {
		reqPath := c.Query("path")
		if reqPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的路径"})
			return
		}

		fullPath := filepath.Join(webdavRoot, filepath.Clean(reqPath))
		if !strings.HasPrefix(fullPath, webdavRoot) {
			c.JSON(http.StatusForbidden, gin.H{"error": "禁止访问"})
			return
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
			return
		}

		if info.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能下载文件夹"})
			return
		}

		c.FileAttachment(fullPath, filepath.Base(fullPath))
	})

	// WebDAV 服务
	webdavHandler := &webdav.Handler{
		Prefix:     "/webdav",
		FileSystem: webdav.Dir(webdavRoot),
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				log.Printf("WebDAV [%s] %s: %v", r.Method, r.URL.Path, err)
			}
		},
	}

	router.Any("/webdav/*path", func(c *gin.Context) {
		webdavHandler.ServeHTTP(c.Writer, c.Request)
	})

	// ========== WebSocket 终端 API ==========
	router.GET("/ws/terminal", handleTerminalWebSocket)

	// ========== Docker API ==========
	router.GET("/api/docker/containers", func(c *gin.Context) {
		containers := getDockerContainers()
		c.JSON(http.StatusOK, containers)
	})

	router.GET("/api/docker/images", func(c *gin.Context) {
		images := getDockerImages()
		c.JSON(http.StatusOK, images)
	})

	// ========== 应用配置 API ==========
	router.GET("/api/app-config", func(c *gin.Context) {
		config := AppConfig{
			Port:      port,
			AutoStart: isAppAutoStartEnabled(),
		}
		c.JSON(http.StatusOK, config)
	})

	router.POST("/api/app-config", func(c *gin.Context) {
		var config AppConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
			return
		}

		// 保存端口到配置文件（需要重启才能生效）
		// 注意：端口更改需要重启应用，这里只保存到设置文件
		// 实际端口由环境变量 PORT 或默认值控制
		// 可以将端口保存到 settings.json 的额外字段中，但当前版本不实现

		// 设置应用开机自启
		if err := setAppAutoStart(config.AutoStart); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "设置开机自启失败: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "配置已保存，端口更改需要重启应用才能生效"})
	})

	// ========== 服务启动和自启 API ==========
	router.POST("/api/services/:id/launch", func(c *gin.Context) {
		id := c.Param("id")
		services := loadServices()
		var service *ServiceCard
		for i := range services {
			if services[i].ID == id {
				service = &services[i]
				break
			}
		}

		if service == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "服务不存在"})
			return
		}

		// 优先使用 LaunchCommand，否则使用 LaunchPath（向后兼容）
		var launchCmd string
		if service.LaunchCommand != "" {
			launchCmd = service.LaunchCommand
		} else if service.LaunchPath != "" {
			launchCmd = service.LaunchPath
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "服务未配置启动命令或启动路径"})
			return
		}

		if err := launchService(launchCmd); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "启动失败: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	router.GET("/api/services/:id/autostart", func(c *gin.Context) {
		id := c.Param("id")
		enabled := isServiceAutoStartEnabled(id)
		c.JSON(http.StatusOK, gin.H{"autoStart": enabled})
	})

	router.POST("/api/services/:id/autostart", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			AutoStart bool `json:"autoStart"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
			return
		}

		services := loadServices()
		var service *ServiceCard
		for i := range services {
			if services[i].ID == id {
				service = &services[i]
				break
			}
		}

		if service == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "服务不存在"})
			return
		}

		if req.AutoStart && service.LaunchPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请先配置启动路径"})
			return
		}

		if err := setServiceAutoStart(id, service.LaunchPath, req.AutoStart); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "设置失败: " + err.Error()})
			return
		}

		// 更新服务配置
		service.AutoStart = req.AutoStart
		for i := range services {
			if services[i].ID == id {
				services[i] = *service
				break
			}
		}
		saveServices(services)

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// ========== 服务进程检测和停止 API ==========
	router.GET("/api/services/:id/process-status", func(c *gin.Context) {
		id := c.Param("id")
		services := loadServices()
		var service *ServiceCard
		for i := range services {
			if services[i].ID == id {
				service = &services[i]
				break
			}
		}

		if service == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "服务不存在"})
			return
		}

		// 检查是否有启动配置
		hasLaunchConfig := service.LaunchCommand != "" || service.LaunchPath != ""
		if !hasLaunchConfig {
			c.JSON(http.StatusOK, gin.H{"running": false, "pid": 0})
			return
		}

		status := checkServiceProcess(service.ProcessName, service.LaunchPath, service.LaunchCommand)
		c.JSON(http.StatusOK, status)
	})

	router.POST("/api/services/:id/stop", func(c *gin.Context) {
		id := c.Param("id")
		services := loadServices()
		var service *ServiceCard
		for i := range services {
			if services[i].ID == id {
				service = &services[i]
				break
			}
		}

		if service == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "服务不存在"})
			return
		}

		// 检查是否有启动配置
		hasLaunchConfig := service.LaunchCommand != "" || service.LaunchPath != ""
		if !hasLaunchConfig {
			c.JSON(http.StatusBadRequest, gin.H{"error": "服务未配置启动命令或启动路径"})
			return
		}

		// 先检查进程是否存在
		status := checkServiceProcess(service.ProcessName, service.LaunchPath, service.LaunchCommand)
		if !status.Running {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "进程未运行"})
			return
		}

		// 停止进程
		if err := stopServiceProcess(status.PID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "停止失败: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// ========== 应用重启 API ==========
	router.POST("/api/app/restart", func(c *gin.Context) {
		// 在goroutine中执行重启，避免阻塞响应
		go func() {
			time.Sleep(1 * time.Second) // 等待响应发送完成
			restartApplication()
		}()
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "应用将在1秒后重启"})
	})

	// ========== ComfyUI API ==========
	router.GET("/api/comfyui/config", func(c *gin.Context) {
		settings := loadSettings()
		c.JSON(http.StatusOK, gin.H{
			"serverUrl": settings.ComfyUIServerURL,
		})
	})

	router.POST("/api/comfyui/config", func(c *gin.Context) {
		var config struct {
			ServerURL string `json:"serverUrl"`
		}
		if err := c.ShouldBindJSON(&config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
			return
		}

		settings := loadSettings()
		settings.ComfyUIServerURL = config.ServerURL
		saveSettings(settings)

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	router.POST("/api/comfyui/workflow/execute", func(c *gin.Context) {
		var req struct {
			Workflow map[string]interface{} `json:"workflow"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
			return
		}

		settings := loadSettings()
		if settings.ComfyUIServerURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请先配置ComfyUI服务器地址"})
			return
		}

		promptId, err := submitComfyUIWorkflow(settings.ComfyUIServerURL, req.Workflow)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "提交工作流失败: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "promptId": promptId})
	})

	router.GET("/api/comfyui/workflow/status/:id", func(c *gin.Context) {
		promptId := c.Param("id")
		settings := loadSettings()
		if settings.ComfyUIServerURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请先配置ComfyUI服务器地址"})
			return
		}

		status, err := getComfyUIWorkflowStatus(settings.ComfyUIServerURL, promptId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询状态失败: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, status)
	})

	addr := "0.0.0.0:" + port
	log.Printf("HomeDash Win is running at http://%s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}

// initDefaultServices 初始化默认服务列表
func initDefaultServices() {
	if _, err := os.Stat(servicesFile); err == nil {
		return // 文件已存在
	}

	defaultServices := []ServiceCard{
		{ID: "lucky", Name: "Lucky", Description: "DDNS、反向代理、证书自动化", Port: 16601, Icon: "🍀", Enabled: true, CreatedAt: time.Now().UnixMilli()},
		{ID: "alist", Name: "Alist", Description: "多网盘整合与 WebDAV", Port: 5244, Icon: "/static/images/alist.png", Enabled: true, CreatedAt: time.Now().UnixMilli()},
		{ID: "immich", Name: "Immich", Description: "相册备份与 AI 检索", Port: 2283, Icon: "/static/images/immich.png", Enabled: true, CreatedAt: time.Now().UnixMilli()},
		{ID: "jellyfin", Name: "Jellyfin", Description: "媒体管理与播放", Port: 8096, Icon: "/static/images/jellyfin.jpg", Enabled: true, CreatedAt: time.Now().UnixMilli()},
		{ID: "comfyui", Name: "ComfyUI", Description: "AI 图像生成工作流", Port: 28000, Icon: "/static/images/comfyui.webp", Enabled: true, CreatedAt: time.Now().UnixMilli()},
		{ID: "rustdesk", Name: "RustDesk", Description: "开源远程桌面控制", Port: 0, Icon: "/static/images/rustdesk.png", Enabled: false, CreatedAt: time.Now().UnixMilli()},
		{ID: "sunshine", Name: "Sunshine", Description: "游戏串流服务端", Port: 0, Icon: "☀️", Enabled: false, CreatedAt: time.Now().UnixMilli()},
		{ID: "moonlight", Name: "Moonlight", Description: "游戏串流客户端", Port: 0, Icon: "🌙", Enabled: false, CreatedAt: time.Now().UnixMilli()},
	}

	saveServices(defaultServices)
}

// loadServices 加载服务列表
func loadServices() []ServiceCard {
	servicesMu.RLock()
	defer servicesMu.RUnlock()

	var services []ServiceCard
	data, err := os.ReadFile(servicesFile)
	if err != nil {
		return services
	}

	json.Unmarshal(data, &services)
	return services
}

// saveServices 保存服务列表
func saveServices(services []ServiceCard) error {
	servicesMu.Lock()
	defer servicesMu.Unlock()

	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(servicesFile, data, 0644)
}

// loadSettings 从文件加载用户设置
func loadSettings() UserSettings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()

	settings := UserSettings{
		ServerIP:      "localhost",
		BackgroundURL: "",
	}

	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return settings
	}

	json.Unmarshal(data, &settings)
	return settings
}

// saveSettings 保存用户设置到文件
func saveSettings(settings UserSettings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsFile, data, 0644)
}

// getBackgroundList 获取背景图列表
func getBackgroundList(webDir string) []BackgroundInfo {
	bgDir := filepath.Join(webDir, "backgrounds")
	var backgrounds []BackgroundInfo

	if _, err := os.Stat(bgDir); os.IsNotExist(err) {
		return backgrounds
	}

	entries, err := os.ReadDir(bgDir)
	if err != nil {
		return backgrounds
	}

	validExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !validExts[ext] {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ext)
		url := "/static/backgrounds/" + entry.Name()
		backgrounds = append(backgrounds, BackgroundInfo{Name: name, URL: url, Thumb: url})
	}

	return backgrounds
}

func resolveWebDir() string {
	candidates := []string{"web", filepath.Join("..", "web")}
	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			absPath, _ := filepath.Abs(dir)
			log.Printf("✓ Using web directory: %s (absolute: %s)", dir, absPath)
			return dir
		}
	}
	log.Println("⚠ Warning: web directory not found, falling back to 'web'")
	return "web"
}

// pingService 检测服务连通性
func pingService(id, host string, port int) PingResult {
	result := PingResult{
		ID:     id,
		Status: "error",
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	start := time.Now()

	// 尝试 TCP 连接
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	latency := time.Since(start).Milliseconds()
	result.Latency = latency

	if err != nil {
		result.Status = "error"
		result.Message = err.Error()
		return result
	}
	conn.Close()

	// 根据延迟判断状态
	if latency < 200 {
		result.Status = "ok"
	} else if latency < 1000 {
		result.Status = "slow"
	} else {
		result.Status = "error"
	}

	return result
}

// fetchFavicon 从 URL 获取 favicon 地址
func fetchFavicon(targetURL string) (string, error) {
	// 确保 URL 有协议前缀
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "http://" + targetURL
	}

	// 尝试直接获取 /favicon.ico
	faviconURL := strings.TrimSuffix(targetURL, "/") + "/favicon.ico"

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Head(faviconURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		return faviconURL, nil
	}

	// 尝试解析 HTML 获取 favicon
	resp, err = client.Get(targetURL)
	if err != nil {
		return "", fmt.Errorf("无法访问目标网站: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024)) // 限制读取 100KB
	if err != nil {
		return "", fmt.Errorf("读取页面失败: %v", err)
	}

	// 解析 <link rel="icon"> 或 <link rel="shortcut icon">
	htmlContent := string(body)

	// 正则匹配 favicon link
	patterns := []string{
		`<link[^>]*rel=["'](?:shortcut )?icon["'][^>]*href=["']([^"']+)["']`,
		`<link[^>]*href=["']([^"']+)["'][^>]*rel=["'](?:shortcut )?icon["']`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(htmlContent)
		if len(matches) > 1 {
			iconHref := matches[1]
			// 处理相对路径
			if strings.HasPrefix(iconHref, "//") {
				return "http:" + iconHref, nil
			} else if strings.HasPrefix(iconHref, "/") {
				// 获取 base URL
				return strings.TrimSuffix(targetURL, "/") + iconHref, nil
			} else if strings.HasPrefix(iconHref, "http") {
				return iconHref, nil
			}
		}
	}

	// 如果都没找到，返回默认的 favicon.ico
	return faviconURL, nil
}

// ========== 终端 WebSocket 处理 ==========
var termUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func handleTerminalWebSocket(c *gin.Context) {
	conn, err := termUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("终端 WebSocket 升级失败: %v", err)
		return
	}
	defer conn.Close()

	// 确定使用的 shell
	var shell string
	var shellArgs []string
	if runtime.GOOS == "windows" {
		shell = "powershell.exe"
		shellArgs = []string{"-NoLogo", "-NoProfile", "-Command", "-"}
	} else {
		shell = "/bin/bash"
		shellArgs = []string{}
	}

	// 发送欢迎消息
	welcomeMsg := fmt.Sprintf("HomeDash Terminal - 连接到: %s", shell)
	conn.WriteMessage(websocket.TextMessage, []byte(welcomeMsg))

	// 处理命令
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("终端 WebSocket 错误: %v", err)
			}
			break
		}

		cmdStr := strings.TrimSpace(string(message))
		if cmdStr == "" {
			continue
		}

		// 执行命令
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-Command", cmdStr)
		} else {
			cmd = exec.Command(shell, append(shellArgs, "-c", cmdStr)...)
		}
		cmd.Dir = webdavRoot

		// 合并 stdout 和 stderr
		output, err := cmd.CombinedOutput()
		if err != nil {
			// 如果有输出，先发送输出
			if len(output) > 0 {
				// 按行发送，过滤空行
				lines := strings.Split(string(output), "\n")
				for _, line := range lines {
					line = strings.TrimRight(line, "\r\n")
					if line != "" {
						conn.WriteMessage(websocket.TextMessage, []byte("\x1b[31m"+line+"\x1b[0m"))
					}
				}
			} else {
				conn.WriteMessage(websocket.TextMessage, []byte("\x1b[31m执行失败: "+err.Error()+"\x1b[0m"))
			}
			continue
		}

		// 按行发送输出，过滤空行
		if len(output) > 0 {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimRight(line, "\r\n")
				if line != "" {
					conn.WriteMessage(websocket.TextMessage, []byte(line))
				}
			}
		}
	}
}

// ========== Docker 辅助函数 ==========
func getDockerContainers() []DockerContainer {
	var containers []DockerContainer

	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}|{{.State}}|{{.Ports}}|{{.CreatedAt}}")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("获取 Docker 容器列表失败: %v", err)
		return containers
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 7)
		if len(parts) >= 5 {
			container := DockerContainer{
				ID:    parts[0],
				Name:  parts[1],
				Image: parts[2],
				Status: parts[3],
				State: parts[4],
			}
			if len(parts) >= 6 {
				container.Ports = parts[5]
			}
			if len(parts) >= 7 {
				container.Created = parts[6]
			}
			containers = append(containers, container)
		}
	}

	return containers
}

// DockerImage Docker 镜像信息
type DockerImage struct {
	ID      string `json:"id"`
	Repo    string `json:"repo"`
	Tag     string `json:"tag"`
	Size    string `json:"size"`
	Created string `json:"created"`
}

func getDockerImages() []DockerImage {
	var images []DockerImage

	cmd := exec.Command("docker", "images", "--format", "{{.ID}}|{{.Repository}}|{{.Tag}}|{{.Size}}|{{.CreatedAt}}")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("获取 Docker 镜像列表失败: %v", err)
		return images
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) >= 4 {
			image := DockerImage{
				ID:   parts[0],
				Repo: parts[1],
				Tag:  parts[2],
				Size: parts[3],
			}
			if len(parts) >= 5 {
				image.Created = parts[4]
			}
			images = append(images, image)
		}
	}

	return images
}

// ========== Windows 注册表操作 ==========
const (
	appAutoStartKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	appAutoStartName = "HomeDash-Win"
)

// isAppAutoStartEnabled 检查应用是否已设置开机自启
func isAppAutoStartEnabled() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, appAutoStartKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(appAutoStartName)
	return err == nil
}

// setAppAutoStart 设置应用开机自启
func setAppAutoStart(enabled bool) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("仅支持 Windows 系统")
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, appAutoStartKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer k.Close()

	if enabled {
		// 获取当前可执行文件的完整路径
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("获取可执行文件路径失败: %v", err)
		}
		absPath, err := filepath.Abs(exePath)
		if err != nil {
			return fmt.Errorf("获取绝对路径失败: %v", err)
		}
		return k.SetStringValue(appAutoStartName, absPath)
	} else {
		return k.DeleteValue(appAutoStartName)
	}
}

// isServiceAutoStartEnabled 检查服务是否已设置开机自启
func isServiceAutoStartEnabled(serviceID string) bool {
	if runtime.GOOS != "windows" {
		return false
	}

	keyName := fmt.Sprintf("HomeDash-Service-%s", serviceID)
	k, err := registry.OpenKey(registry.CURRENT_USER, appAutoStartKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(keyName)
	return err == nil
}

// setServiceAutoStart 设置服务开机自启
func setServiceAutoStart(serviceID, launchPath string, enabled bool) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("仅支持 Windows 系统")
	}

	keyName := fmt.Sprintf("HomeDash-Service-%s", serviceID)
	k, err := registry.OpenKey(registry.CURRENT_USER, appAutoStartKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer k.Close()

	if enabled {
		if launchPath == "" {
			return fmt.Errorf("启动路径不能为空")
		}
		// 转换为绝对路径
		absPath, err := filepath.Abs(launchPath)
		if err != nil {
			return fmt.Errorf("获取绝对路径失败: %v", err)
		}
		return k.SetStringValue(keyName, absPath)
	} else {
		return k.DeleteValue(keyName)
	}
}

// launchService 启动服务（支持带参数的命令）
func launchService(launchCmd string) error {
	// 解析命令和参数
	parts := parseCommand(launchCmd)
	if len(parts) == 0 {
		return fmt.Errorf("启动命令为空")
	}

	// 第一个部分是可执行文件路径
	exePath := parts[0]
	absPath, err := filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("获取绝对路径失败: %v", err)
	}

	// 验证可执行文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", absPath)
	}

	// Windows 上使用 start 命令启动，避免阻塞
	if runtime.GOOS == "windows" {
		// 如果有参数，需要将整个命令作为字符串传递给 cmd
		if len(parts) > 1 {
			// 构建完整命令，处理包含空格的路径
			fullCmd := fmt.Sprintf(`"%s"`, absPath)
			for i := 1; i < len(parts); i++ {
				// 如果参数包含空格，用引号包裹
				if strings.Contains(parts[i], " ") {
					fullCmd += fmt.Sprintf(` "%s"`, parts[i])
				} else {
					fullCmd += " " + parts[i]
				}
			}
			cmd := exec.Command("cmd.exe", "/c", "start", "", fullCmd)
			return cmd.Run()
		} else {
			cmd := exec.Command("cmd.exe", "/c", "start", "", absPath)
			return cmd.Run()
		}
	} else {
		// Linux/Mac 上直接执行
		cmd := exec.Command(absPath, parts[1:]...)
		return cmd.Start() // 使用 Start 而不是 Run，避免阻塞
	}
}

// parseCommand 解析命令字符串，支持引号包裹的参数
func parseCommand(cmdStr string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	escapeNext := false

	for i, r := range cmdStr {
		if escapeNext {
			current.WriteRune(r)
			escapeNext = false
			continue
		}

		if r == '\\' {
			escapeNext = true
			continue
		}

		if r == '"' {
			inQuotes = !inQuotes
			continue
		}

		if r == ' ' && !inQuotes {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}

		// 处理最后一个字符
		if i == len(cmdStr)-1 && current.Len() > 0 {
			parts = append(parts, current.String())
		}
	}

	return parts
}

// ProcessStatus 进程状态
type ProcessStatus struct {
	Running bool  `json:"running"`
	PID     int32 `json:"pid"`
}

// checkServiceProcess 检测服务进程是否存在
// processName: 进程名（优先使用），如果为空则从 launchPath 或 launchCommand 提取
func checkServiceProcess(processName, launchPath, launchCommand string) ProcessStatus {
	status := ProcessStatus{Running: false, PID: 0}

	// 优先使用 processName
	var exeName string
	if processName != "" {
		exeName = processName
	} else if launchPath != "" {
		// 从启动路径提取可执行文件名
		exeName = filepath.Base(launchPath)
	} else if launchCommand != "" {
		// 从启动命令提取可执行文件名
		parts := parseCommand(launchCommand)
		if len(parts) > 0 {
			exeName = filepath.Base(parts[0])
		}
	}

	if exeName == "" {
		return status
	}

	// 获取所有进程
	processes, err := process.Processes()
	if err != nil {
		log.Printf("获取进程列表失败: %v", err)
		return status
	}

	// 精确匹配进程名（不区分大小写）
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if strings.EqualFold(name, exeName) {
			status.Running = true
			status.PID = p.Pid
			return status
		}
	}

	return status
}

// stopServiceProcess 停止服务进程
func stopServiceProcess(pid int32) error {
	if runtime.GOOS == "windows" {
		// 先尝试优雅关闭
		cmd := exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid))
		err := cmd.Run()
		if err == nil {
			// 等待进程退出（最多 5 秒）
			for i := 0; i < 10; i++ {
				exists, _ := process.PidExists(pid)
				if !exists {
					return nil
				}
				time.Sleep(500 * time.Millisecond)
			}
		}

		// 如果优雅关闭失败或超时，强制终止
		cmd = exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
		return cmd.Run()
	} else {
		// Linux/Mac 上使用 kill 命令
		// 先尝试 SIGTERM
		proc, err := process.NewProcess(pid)
		if err != nil {
			return err
		}
		proc.Terminate()
		
		// 等待进程退出
		time.Sleep(2 * time.Second)
		
		// 如果还在运行，强制终止
		exists, _ := process.PidExists(pid)
		if exists {
			proc.Kill()
		}
		
		return nil
	}
}

// downloadFavicon 下载 favicon 并保存
func downloadFavicon(faviconURL, webDir string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(faviconURL)
	if err != nil {
		return "", fmt.Errorf("下载 favicon 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("favicon 不存在: HTTP %d", resp.StatusCode)
	}

	// 读取内容
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024)) // 限制 1MB
	if err != nil {
		return "", fmt.Errorf("读取 favicon 失败: %v", err)
	}

	// 创建 icons 目录
	iconsDir := filepath.Join(webDir, "icons")
	if err := os.MkdirAll(iconsDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	// 生成文件名（使用 URL 的 MD5 哈希）
	hash := md5.Sum([]byte(faviconURL))
	ext := filepath.Ext(faviconURL)
	if ext == "" || len(ext) > 5 {
		ext = ".ico"
	}
	filename := fmt.Sprintf("%x%s", hash, ext)
	filePath := filepath.Join(iconsDir, filename)

	// 保存文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("保存 favicon 失败: %v", err)
	}

	return "/static/icons/" + filename, nil
}

// submitComfyUIWorkflow 提交ComfyUI工作流
func submitComfyUIWorkflow(serverURL string, workflow map[string]interface{}) (string, error) {
	// 构建请求体
	reqBody := map[string]interface{}{
		"prompt": workflow,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// 发送POST请求
	resp, err := http.Post(serverURL+"/prompt", "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// 提取prompt_id
	promptId, ok := result["prompt_id"].(string)
	if !ok {
		return "", fmt.Errorf("无法获取prompt_id")
	}

	return promptId, nil
}

// getComfyUIWorkflowStatus 查询ComfyUI工作流状态
func getComfyUIWorkflowStatus(serverURL, promptId string) (map[string]interface{}, error) {
	// 1. 先查询队列状态
	queueResp, err := http.Get(serverURL + "/queue")
	if err == nil {
		defer queueResp.Body.Close()
		if queueResp.StatusCode == http.StatusOK {
			var queueData map[string]interface{}
			if json.NewDecoder(queueResp.Body).Decode(&queueData) == nil {
				// 检查运行队列
				running, _ := queueData["queue_running"].([]interface{})
				for _, item := range running {
					itemData, ok := item.([]interface{})
					if !ok || len(itemData) < 2 {
						continue
					}
					// 队列格式: [序号, prompt_id, workflow, extra_info, output_nodes]
					itemPromptId, ok := itemData[1].(string)
					if ok && itemPromptId == promptId {
						return map[string]interface{}{
							"completed": false,
							"progress":  50,
							"message":  "执行中",
						}, nil
					}
				}

				// 检查等待队列
				pending, _ := queueData["queue_pending"].([]interface{})
				for _, item := range pending {
					itemData, ok := item.([]interface{})
					if !ok || len(itemData) < 2 {
						continue
					}
					itemPromptId, ok := itemData[1].(string)
					if ok && itemPromptId == promptId {
						return map[string]interface{}{
							"completed": false,
							"progress":  10,
							"message":  "等待执行",
						}, nil
					}
				}
			}
		}
	}

	// 2. 如果不在队列中，查询历史记录
	historyResp, err := http.Get(serverURL + "/history?max_items=64")
	if err != nil {
		return map[string]interface{}{
			"completed": false,
			"progress":  0,
			"message":  "查询失败",
		}, nil
	}
	defer historyResp.Body.Close()

	if historyResp.StatusCode != http.StatusOK {
		return map[string]interface{}{
			"completed": false,
			"progress":  0,
			"message":  "等待执行",
		}, nil
	}

	var history map[string]interface{}
	if err := json.NewDecoder(historyResp.Body).Decode(&history); err != nil {
		return map[string]interface{}{
			"completed": false,
			"progress":  0,
			"message":  "解析失败",
		}, nil
	}

	// 检查历史记录中是否有该prompt_id
	promptData, ok := history[promptId].(map[string]interface{})
	if !ok {
		return map[string]interface{}{
			"completed": false,
			"progress":  0,
			"message":  "等待执行",
		}, nil
	}

	// 检查状态
	statusData, ok := promptData["status"].(map[string]interface{})
	if ok {
		completed, _ := statusData["completed"].(bool)
		if completed {
			// 执行完成，提取图片
			outputs, _ := promptData["outputs"].(map[string]interface{})
			images := []map[string]string{}

			// 遍历所有节点的输出
			for nodeId, nodeOutput := range outputs {
				nodeData, ok := nodeOutput.(map[string]interface{})
				if !ok {
					continue
				}
				imagesData, ok := nodeData["images"].([]interface{})
				if !ok {
					continue
				}
				for _, imgData := range imagesData {
					img, ok := imgData.(map[string]interface{})
					if !ok {
						continue
					}
					filename, _ := img["filename"].(string)
					subfolder, _ := img["subfolder"].(string)
					imgType, _ := img["type"].(string)
					if filename != "" {
						// 构建图片URL
						imgURL := fmt.Sprintf("%s/view?filename=%s", serverURL, filename)
						if subfolder != "" {
							imgURL += fmt.Sprintf("&subfolder=%s", subfolder)
						}
						if imgType != "" {
							imgURL += fmt.Sprintf("&type=%s", imgType)
						}
						images = append(images, map[string]string{
							"url":      imgURL,
							"filename": filename,
							"nodeId":   nodeId,
						})
					}
				}
			}

			return map[string]interface{}{
				"completed": true,
				"progress":  100,
				"message":  "执行完成",
				"images":   images,
			}, nil
		}
	}

	return map[string]interface{}{
		"completed": false,
		"progress":  0,
		"message":  "等待执行",
	}, nil
}

// restartApplication 重启应用
func restartApplication() {
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("获取可执行文件路径失败: %v", err)
		return
	}

	absPath, err := filepath.Abs(exePath)
	if err != nil {
		log.Printf("获取绝对路径失败: %v", err)
		return
	}

	// Windows 上使用 cmd.exe 启动新进程
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd.exe", "/c", "start", "", absPath)
		cmd.Dir = filepath.Dir(absPath)
		if err := cmd.Start(); err != nil {
			log.Printf("启动新进程失败: %v", err)
			return
		}
	} else {
		// Linux/Mac 上直接执行
		cmd := exec.Command(absPath)
		cmd.Dir = filepath.Dir(absPath)
		if err := cmd.Start(); err != nil {
			log.Printf("启动新进程失败: %v", err)
			return
		}
	}

	// 延迟退出，给新进程启动时间
	time.Sleep(2 * time.Second)
	os.Exit(0)
}
