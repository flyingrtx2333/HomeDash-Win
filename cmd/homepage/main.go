package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"homedash/internal/handlers"
	"homedash/internal/monitor"
	"homedash/internal/routes"

	"github.com/gin-gonic/gin"
)

const defaultPort = "29678"

// 自动禁用控制台的快速编辑模式
func disableQuickEdit() {
	if runtime.GOOS != "windows" {
		return
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	getStdHandle := kernel32.NewProc("GetStdHandle")

	const (
		STD_INPUT_HANDLE       = uint32(-10 & 0xFFFFFFFF)
		ENABLE_QUICK_EDIT_MODE = 0x0040
		ENABLE_EXTENDED_FLAGS  = 0x0080
	)

	var mode uint32
	// 获取标准输入句柄
	handle, _, _ := getStdHandle.Call(uintptr(STD_INPUT_HANDLE))
	if handle == 0 {
		return // 无法获取句柄，可能不是控制台环境
	}

	// 获取当前模式
	ret, _, _ := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	if ret == 0 {
		return // 获取模式失败
	}

	// 移除快速编辑模式位
	mode &^= ENABLE_QUICK_EDIT_MODE
	// 必须加上这个标志位才能使更改生效
	mode |= ENABLE_EXTENDED_FLAGS

	// 设置新模式
	setConsoleMode.Call(handle, uintptr(mode))
}

func main() {
	// 查找项目根目录
	projectRoot, err := findProjectRoot()
	if err != nil {
		log.Fatalf("查找项目根目录失败: %v", err)
	}

	// 切换到项目根目录
	err = os.Chdir(projectRoot)
	if err != nil {
		log.Fatalf("切换工作目录失败: %v", err)
	}

	log.Printf("当前工作目录已修正为: %s", projectRoot)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	webDir := resolveWebDir()
	settingsFile := filepath.Join(webDir, "settings.json")
	servicesFile := filepath.Join(webDir, "services.json")

	// WebDAV 根目录：优先从设置文件加载，否则使用环境变量或默认用户目录
	webdavRoot := os.Getenv("WEBDAV_ROOT")
	if webdavRoot == "" {
		homeDir, _ := os.UserHomeDir()
		webdavRoot = homeDir
	}

	if runtime.GOOS == "windows" {
		// disableQuickEdit()
	}

	// 初始化处理器全局变量
	handlers.InitHandlers(webDir, settingsFile, servicesFile, webdavRoot)

	// 初始化默认服务
	handlers.InitDefaultServices()

	// 从设置文件加载 WebDAV 根目录
	savedSettings := handlers.LoadSettings()
	if savedSettings.WebdavRoot != "" {
		handlers.SetWebdavRoot(savedSettings.WebdavRoot)
	}

	// 初始化监控 Hub
	monitorHub := monitor.NewHub()
	go monitorHub.Run()
	handlers.InitMonitor(monitorHub)

	// 创建路由
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 设置所有路由
	routes.SetupRoutes(router, webDir, port)

	addr := "0.0.0.0:" + port
	log.Printf("HomeDash Win is running at http://%s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}

// findProjectRoot 查找项目根目录（包含go.mod的目录）
func findProjectRoot() (string, error) {
	// 首先尝试从当前工作目录向上查找
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := wd
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// 已经到达根目录
			break
		}
		dir = parent
	}

	// 如果找不到，尝试从可执行文件路径向上查找
	exePath, err := os.Executable()
	if err != nil {
		return wd, nil // 返回当前工作目录作为后备
	}

	dir = filepath.Dir(exePath)
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// 如果还是找不到，返回当前工作目录
	return wd, nil
}

// resolveWebDir 解析web目录路径
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
