package routes

import (
	"log"
	"net/http"
	"path/filepath"

	"homedash/internal/handlers"

	"github.com/gin-gonic/gin"
)

// pageConfig 页面配置：每个路由只渲染对应内容模板和可选脚本
type pageConfig struct {
	ContentTemplate string
	Scripts         []string
}

// 路由与页面配置映射（多路由独立页面，不再 SPA 全量加载）
var pageRoutes = map[string]pageConfig{
	"/":         {"pages/home-content", nil},
	"/monitor":  {"pages/monitor-content", nil},
	"/process":  {"pages/process-content", nil},
	"/webdav":   {"pages/webdav-content", []string{"webdav"}},
	"/logs":     {"pages/logs", nil},
	"/terminal": {"pages/terminal-content", []string{"terminal"}},
	"/docker":   {"pages/docker-content", nil},
	"/comfyui":  {"pages/comfyui-content", nil},
	"/settings": {"pages/settings-content", nil},
	"/frpc":     {"pages/frpc-content", []string{"frpc"}},
	"/websites":     {"pages/websites-content", []string{"websites"}},
	"/websites-npm": {"pages/websites-npm-content", []string{"websites_npm"}},
	"/database":     {"pages/database-content", []string{"database"}},
}

// SetupRoutes 设置所有路由
func SetupRoutes(router *gin.Engine, webDir string, port string) {
	// 初始化模板
	templatePath := filepath.Join(webDir, "templates")
	handlers.InitTemplates(templatePath)

	// 设置HTML模板渲染（失败则终止，避免后续 c.HTML 时 nil 崩溃）
	tmpl, err := handlers.LoadTemplates()
	if err != nil {
		log.Fatalf("加载模板失败: %v", err)
	}
	router.SetHTMLTemplate(tmpl)

	// 多路由模式：每个路径返回 master 布局 + 当前页内容，独立加载
	for path, cfg := range pageRoutes {
		path, cfg := path, cfg
		currentPath := path
		if currentPath == "" {
			currentPath = "/"
		}
		router.GET(path, func(c *gin.Context) {
			data := gin.H{
				"ContentTemplate": cfg.ContentTemplate,
				"CurrentPath":     currentPath,
				"Scripts":         cfg.Scripts,
			}
			c.HTML(200, "master.html", data)
		})
	}

	// 静态文件服务
	router.StaticFS("/static", http.Dir(webDir+"/static"))

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

	// ========== 内网穿透 FRPC ==========
	{
		api.GET("/frpc/config", handlers.GetFrpcConfig)
		api.GET("/frpc/config/parsed", handlers.GetFrpcConfigParsed)
		api.POST("/frpc/config", handlers.UpdateFrpcConfig)
		api.GET("/frpc/status", handlers.GetFrpcStatus)
		api.POST("/frpc/start", handlers.StartFrpc)
		api.POST("/frpc/stop", handlers.StopFrpc)
		api.GET("/frpc/autostart", handlers.GetFrpcAutoStart)
		api.POST("/frpc/autostart", handlers.UpdateFrpcAutoStart)
	}

	// ========== 网站项目管理 ==========
	{
		// 项目CRUD
		api.GET("/websites", handlers.GetWebsites)
		api.GET("/websites/:id", handlers.GetWebsite)
		api.POST("/websites", handlers.CreateWebsite)
		api.PUT("/websites/:id", handlers.UpdateWebsite)
		api.DELETE("/websites/:id", handlers.DeleteWebsite)

		// Python版本
		api.GET("/websites/python/versions", handlers.GetPythonVersions)

		// 项目检测
		api.POST("/websites/detect", handlers.DetectProjectInfo)

		// 目录浏览（用于文件选择器）
		api.GET("/websites/browse", handlers.BrowseDirectory)

		// 环境选项（项目 .venv + conda 列表）
		api.GET("/websites/envs", handlers.GetWebsiteEnvOptions)

		// 虚拟环境
		api.POST("/websites/:id/venv/create", handlers.CreateVenv)
		api.DELETE("/websites/:id/venv", handlers.DeleteVenv)
		api.POST("/websites/:id/requirements/install", handlers.InstallRequirements)

		// 项目运行
		api.POST("/websites/:id/start", handlers.StartWebsite)
		api.POST("/websites/:id/stop", handlers.StopWebsite)
		api.GET("/websites/:id/status", handlers.GetWebsiteStatus)

		// 日志
		api.GET("/websites/:id/logs", handlers.GetWebsiteLogs)
		api.POST("/websites/:id/logs/clear", handlers.ClearWebsiteLogs)
		router.GET("/ws/websites/:id/logs", handlers.StreamWebsiteLogs)
	}

	// ========== Node.js 项目管理 ==========
	{
		api.GET("/npm-projects", handlers.GetNpmProjects)
		api.GET("/npm-projects/:id", handlers.GetNpmProject)
		api.POST("/npm-projects", handlers.CreateNpmProject)
		api.PUT("/npm-projects/:id", handlers.UpdateNpmProject)
		api.DELETE("/npm-projects/:id", handlers.DeleteNpmProject)
		api.POST("/npm-projects/:id/start", handlers.StartNpmProject)
		api.POST("/npm-projects/:id/stop", handlers.StopNpmProject)
		api.GET("/npm-projects/:id/status", handlers.GetNpmProjectStatus)
		api.GET("/npm-projects/:id/logs", handlers.GetNpmProjectLogs)
		api.POST("/npm-projects/:id/logs/clear", handlers.ClearNpmProjectLogs)
		api.POST("/npm-projects/:id/install", handlers.InstallNpmDependencies)
	}

	// ========== 数据库管理 ==========
	{
		// 数据库CRUD
		api.GET("/databases", handlers.GetDatabases)
		api.GET("/databases/:id", handlers.GetDatabase)
		api.POST("/databases", handlers.CreateDatabase)
		api.PUT("/databases/:id", handlers.UpdateDatabase)
		api.DELETE("/databases/:id", handlers.DeleteDatabase)

		// MySQL 操作
		api.POST("/databases/:id/test", handlers.TestConnection)
		api.POST("/databases/:id/change-password", handlers.ChangePassword)
		api.GET("/databases/:id/export", handlers.ExportSQL)
		api.POST("/databases/:id/import", handlers.ImportSQL)
		api.GET("/databases/:id/tables", handlers.GetTables)

		// 备份管理
		api.GET("/databases/:id/backups", handlers.GetBackups)
		api.POST("/databases/:id/backup", handlers.CreateBackup)
		api.DELETE("/databases/:id/backups/:filename", handlers.DeleteBackup)
		api.GET("/databases/:id/backups/:filename/download", handlers.DownloadBackup)
		api.GET("/databases/:id/backup-config", handlers.GetBackupConfig)
		api.POST("/databases/:id/backup-config", handlers.UpdateBackupConfig)
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

		// 版本与更新检查
		api.GET("/version", handlers.GetAppVersion)
		api.GET("/update-check", handlers.CheckUpdate)

		// Favicon 和图标
		api.GET("/favicon", handlers.GetFavicon)
		api.POST("/upload-icon", handlers.UploadIcon)
	}
}
