package routes

import (
	"net/http"
	"path/filepath"

	"homedash/internal/handlers"

	"github.com/gin-gonic/gin"
)

// SetupRoutes 设置所有路由
func SetupRoutes(router *gin.Engine, webDir string, port string) {
	// 初始化模板
	templatePath := filepath.Join(webDir, "templates")
	handlers.InitTemplates(templatePath)

	// 设置HTML模板渲染
	tmpl, err := handlers.LoadTemplates()
	if err == nil {
		router.SetHTMLTemplate(tmpl)
	}

	// SPA模式：所有路由都返回同一个页面（包含所有page-view）
	router.GET("/", func(c *gin.Context) {
		c.HTML(200, "home.html", gin.H{})
	})
	router.GET("/monitor", func(c *gin.Context) {
		c.HTML(200, "home.html", gin.H{})
	})
	router.GET("/process", func(c *gin.Context) {
		c.HTML(200, "home.html", gin.H{})
	})
	router.GET("/webdav", func(c *gin.Context) {
		c.HTML(200, "home.html", gin.H{})
	})
	router.GET("/terminal", func(c *gin.Context) {
		c.HTML(200, "home.html", gin.H{})
	})
	router.GET("/docker", func(c *gin.Context) {
		c.HTML(200, "home.html", gin.H{})
	})
	router.GET("/comfyui", func(c *gin.Context) {
		c.HTML(200, "home.html", gin.H{})
	})
	router.GET("/settings", func(c *gin.Context) {
		c.HTML(200, "home.html", gin.H{})
	})

	// 静态文件服务
	router.StaticFS("/static", http.Dir(webDir))

	// ========== 首页服务入口 ==========
	api := router.Group("/api")
	{
		// 服务管理
		api.GET("/services", handlers.GetServices)
		api.POST("/services", handlers.CreateService)
		api.PUT("/services/:id", handlers.UpdateService)
		api.DELETE("/services/:id", handlers.DeleteService)
		api.POST("/services/import-template", handlers.ImportServiceTemplate)
		api.GET("/services/:id/ping", handlers.PingService)
		api.GET("/ping-all", handlers.PingAllServices)

		// 服务启动和停止
		api.POST("/services/:id/launch", handlers.LaunchService)
		api.GET("/services/:id/process-status", handlers.GetServiceProcessStatus)
		api.POST("/services/:id/stop", handlers.StopService)
	}

	// ========== 系统监控 ==========
	{
		api.GET("/processes", handlers.GetProcesses)
		router.GET("/ws/monitor", handlers.HandleMonitorWebSocket)
	}

	// ========== 进程管理 ==========
	// (已在服务管理中包含)

	// ========== WEBDAV管理 ==========
	{
		api.GET("/files", handlers.GetFileList)
		api.POST("/files/mkdir", handlers.CreateDirectory)
		api.DELETE("/files", handlers.DeleteFile)
		api.POST("/files/upload", handlers.UploadFile)
		api.GET("/files/download", handlers.DownloadFile)

		// WebDAV 服务
		webdavHandler := handlers.GetWebdavHandler()
		router.Any("/webdav/*path", func(c *gin.Context) {
			webdavHandler.ServeHTTP(c.Writer, c.Request)
		})
	}

	// ========== SSH终端 ==========
	{
		router.GET("/ws/terminal", handlers.HandleTerminalWebSocket)
	}

	// ========== DOCKER管理 ==========
	{
		api.GET("/docker/containers", handlers.GetDockerContainers)
		api.GET("/docker/images", handlers.GetDockerImages)
	}

	// ========== AI绘画管理 ==========
	{
		api.GET("/comfyui/config", handlers.GetComfyUIConfig)
		api.POST("/comfyui/config", handlers.UpdateComfyUIConfig)
		api.POST("/comfyui/workflow/execute", handlers.ExecuteComfyUIWorkflow)
		api.GET("/comfyui/workflow/status/:id", handlers.GetComfyUIWorkflowStatus)
	}

	// ========== 日志查看器 ==========
	{
		api.GET("/logs", handlers.GetLogs)
		api.GET("/logs/services", handlers.GetLogServices)
		api.GET("/logs/stream", handlers.StreamLogs)
		api.POST("/logs/:service/clear", handlers.ClearLogs)
	}

	// ========== 程序设置 ==========
	{
		// 背景图
		api.GET("/backgrounds", handlers.GetBackgrounds)

		// 用户设置
		api.GET("/settings", handlers.GetSettings)
		api.POST("/settings", handlers.UpdateSettings)
		api.GET("/ping", handlers.GetSettings)

		// WebDAV 根目录
		api.GET("/webdav-root", handlers.GetWebdavRoot)
		api.POST("/webdav-root", handlers.UpdateWebdavRoot)

		// 应用配置
		api.GET("/app-config", func(c *gin.Context) {
			handlers.GetAppConfig(c, port)
		})
		api.POST("/app-config", handlers.UpdateAppConfig)

		// 服务开机自启
		api.GET("/services/:id/autostart", handlers.GetServiceAutoStart)
		api.POST("/services/:id/autostart", handlers.UpdateServiceAutoStart)

		// 应用重启
		api.POST("/app/restart", handlers.RestartApplication)

		// Favicon 和图标
		api.GET("/favicon", handlers.GetFavicon)
		api.POST("/upload-icon", handlers.UploadIcon)
	}
}
