//go:build windows

// 打包前需先生成 Windows 资源（图标等），任选其一：
//   go generate ./cmd/homedash/
//   go-winres make --in winres/winres.json --out cmd/homedash/rsrc
// 图标需放在 winres/ 下（logo.png、logo16.png），尺寸不超过 256×256。然后再执行 go build。
//go:generate go-winres make --in ../../winres/winres.json --out rsrc
package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"homedash/internal/handlers"
	"homedash/internal/monitor"
	"homedash/internal/routes"
	"homedash/internal/ui"

	"github.com/gin-gonic/gin"
)

const defaultPort = "29678"

func main() {
	// 1. 先创建 App 对象拿到 writer
	app := ui.NewApp()
	w := app.LogWriter()

	// 2. 在任何 gin/log 初始化之前完成重定向
	//    windowsgui 模式下 os.Stdout/Stderr 是无效句柄，必须全部重定向
	log.SetOutput(w)
	log.SetFlags(log.Ldate | log.Ltime)

	// gin 必须在 SetMode 之前设置 DefaultWriter，否则 Logger 中间件
	// 会在内部 init() 里缓存 os.Stdout
	gin.DefaultWriter = w
	gin.DefaultErrorWriter = w
	gin.SetMode(gin.DebugMode)

	// 3. 后台跑服务
	go runServer(app)

	// 4. 主线程跑 GUI 消息循环（阻塞直到退出）
	app.Run()
}

func runServer(app *ui.App) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		log.Fatalf("查找项目根目录失败: %v", err)
	}
	if err = os.Chdir(projectRoot); err != nil {
		log.Fatalf("切换工作目录失败: %v", err)
	}
	log.Printf("工作目录: %s", projectRoot)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	webDir := resolveWebDir()
	settingsFile := filepath.Join(webDir, "settings.json")
	servicesFile := filepath.Join(webDir, "services.json")

	webdavRoot := os.Getenv("WEBDAV_ROOT")
	if webdavRoot == "" {
		homeDir, _ := os.UserHomeDir()
		webdavRoot = homeDir
	}

	handlers.InitHandlers(webDir, settingsFile, servicesFile, webdavRoot)
	handlers.InitDefaultServices()

	savedSettings := handlers.LoadSettings()
	if savedSettings.WebdavRoot != "" {
		handlers.SetWebdavRoot(savedSettings.WebdavRoot)
	}

	monitorHub := monitor.NewHub()
	go monitorHub.Run()
	handlers.InitMonitor(monitorHub)

	router := gin.New()
	// 用已重定向的 writer 创建 Logger，不要用默认的 gin.Logger()
	router.Use(gin.LoggerWithWriter(io.Discard)) // gin 请求日志按需开启，避免刷屏
	router.Use(gin.Recovery())

	routes.SetupRoutes(router, webDir, port)

	handlers.MaybeLaunchFrpcOnStartup()
	handlers.MaybeLaunchWebsitesOnStartup()

	log.Printf("HomeDash Win 已启动 → http://127.0.0.1:%s", port)
	app.SetRunning(true, port)

	if err := router.Run("0.0.0.0:" + port); err != nil {
		log.Fatal(err)
	}
}

func findProjectRoot() (string, error) {
	wd, _ := os.Getwd()
	for dir := wd; ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	exePath, err := os.Executable()
	if err != nil {
		return wd, nil
	}
	for dir := filepath.Dir(exePath); ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return wd, nil
}

func resolveWebDir() string {
	for _, dir := range []string{"web", filepath.Join("..", "web")} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(dir)
			log.Printf("Web 目录: %s", abs)
			return dir
		}
	}
	log.Println("⚠ 未找到 web 目录，使用默认值 'web'")
	return "web"
}
