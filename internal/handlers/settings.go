package handlers

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/sys/windows/registry"
)

// GetBackgrounds 获取背景图列表
func GetBackgrounds(c *gin.Context) {
	backgrounds := getBackgroundList(webDir)
	c.JSON(200, backgrounds)
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

// GetSettings 获取用户设置
func GetSettings(c *gin.Context) {
	settings := loadSettings()
	c.JSON(200, settings)
}

// UpdateSettings 更新用户设置
func UpdateSettings(c *gin.Context) {
	var settings UserSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	// 验证设置
	if err := ValidateUserSettings(&settings); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 如果设置了新的 WebDAV 根目录，更新全局变量
	if settings.WebdavRoot != "" {
		webdavRoot = settings.WebdavRoot
	}

	if err := saveSettings(settings); err != nil {
		c.JSON(500, gin.H{"error": "保存设置失败"})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// GetWebdavRoot 获取WebDAV根目录
func GetWebdavRoot(c *gin.Context) {
	c.JSON(200, gin.H{"root": webdavRoot})
}

// UpdateWebdavRoot 更新WebDAV根目录
func UpdateWebdavRoot(c *gin.Context) {
	var req struct {
		Root string `json:"root"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求"})
		return
	}

	// 验证路径是否存在
	if _, err := os.Stat(req.Root); os.IsNotExist(err) {
		c.JSON(400, gin.H{"error": "目录不存在"})
		return
	}

	// 更新全局变量和设置
	webdavRoot = req.Root
	settings := loadSettings()
	settings.WebdavRoot = req.Root
	saveSettings(settings)

	c.JSON(200, gin.H{"success": true, "root": webdavRoot})
}

// GetAppConfig 获取应用配置
func GetAppConfig(c *gin.Context, port string) {
	config := AppConfig{
		Port:      port,
		AutoStart: isAppAutoStartEnabled(),
	}
	c.JSON(200, config)
}

// UpdateAppConfig 更新应用配置
func UpdateAppConfig(c *gin.Context) {
	var config AppConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	// 设置应用开机自启
	if err := setAppAutoStart(config.AutoStart); err != nil {
		c.JSON(500, gin.H{"error": "设置开机自启失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "配置已保存，端口更改需要重启应用才能生效"})
}

// GetServiceAutoStart 获取服务开机自启状态
func GetServiceAutoStart(c *gin.Context) {
	id := c.Param("id")
	enabled := isServiceAutoStartEnabled(id)
	c.JSON(200, gin.H{"autoStart": enabled})
}

// UpdateServiceAutoStart 更新服务开机自启状态
func UpdateServiceAutoStart(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		AutoStart bool `json:"autoStart"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求"})
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
		c.JSON(404, gin.H{"error": "服务不存在"})
		return
	}

	if req.AutoStart && service.LaunchPath == "" {
		c.JSON(400, gin.H{"error": "请先配置启动路径"})
		return
	}

	if err := setServiceAutoStart(id, service.LaunchPath, req.AutoStart); err != nil {
		c.JSON(500, gin.H{"error": "设置失败: " + err.Error()})
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

	c.JSON(200, gin.H{"success": true})
}

// RestartApplication 重启应用
func RestartApplication(c *gin.Context) {
	// 在goroutine中执行重启，避免阻塞响应
	go func() {
		time.Sleep(1 * time.Second) // 等待响应发送完成
		restartApplication()
	}()
	c.JSON(200, gin.H{"success": true, "message": "应用将在1秒后重启"})
}

// GetFavicon 抓取favicon
func GetFavicon(c *gin.Context) {
	targetURL := c.Query("url")
	if targetURL == "" {
		c.JSON(400, gin.H{"error": "缺少 url 参数"})
		return
	}

	faviconURL, err := fetchFavicon(targetURL)
	if err != nil {
		c.JSON(200, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 下载并保存 favicon
	savedPath, err := downloadFavicon(faviconURL, webDir)
	if err != nil {
		c.JSON(200, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "icon": savedPath})
}

// UploadIcon 上传图标（增强安全性）
func UploadIcon(c *gin.Context) {
	file, err := c.FormFile("icon")
	if err != nil {
		c.JSON(400, gin.H{"error": "未找到上传文件"})
		return
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	validExts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true, ".svg": true}
	if !validExts[ext] {
		c.JSON(400, gin.H{"error": "不支持的文件格式"})
		return
	}

	// 限制文件大小 (2MB)
	if file.Size > 2*1024*1024 {
		c.JSON(400, gin.H{"error": "文件过大，最大 2MB"})
		return
	}

	// 验证文件内容（防止伪造扩展名）
	fileHeader, err := file.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": "打开文件失败"})
		return
	}
	defer fileHeader.Close()

	// 读取文件头进行验证
	buffer := make([]byte, 512)
	_, err = fileHeader.Read(buffer)
	if err != nil {
		c.JSON(500, gin.H{"error": "读取文件失败"})
		return
	}

	// 验证文件内容类型
	contentType := http.DetectContentType(buffer)
	validTypes := map[string]bool{
		"image/png":      true,
		"image/jpeg":     true,
		"image/gif":      true,
		"image/webp":     true,
		"image/x-icon":   true,
		"image/svg+xml":  true,
		"application/octet-stream": true, // 某些 .ico 文件可能被识别为此类型
	}

	if !validTypes[contentType] {
		c.JSON(400, gin.H{"error": "文件内容类型不匹配"})
		return
	}

	// 验证文件名，防止路径遍历攻击
	filename := file.Filename
	if strings.ContainsAny(filename, "/\\:*?\"<>|") {
		c.JSON(400, gin.H{"error": "文件名包含非法字符"})
		return
	}

	// 创建 icons 目录
	iconsDir := filepath.Join(webDir, "icons")
	if err := os.MkdirAll(iconsDir, 0755); err != nil {
		c.JSON(500, gin.H{"error": "创建目录失败"})
		return
	}

	// 生成唯一文件名（使用 UUID 避免冲突）
	filename = fmt.Sprintf("%s%s", uuid.New().String()[:8], ext)
	savePath := filepath.Join(iconsDir, filename)

	// 使用安全的保存方法
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(500, gin.H{"error": "保存文件失败"})
		return
	}

	c.JSON(200, gin.H{"success": true, "icon": "/static/icons/" + filename})
}

// ========== Windows 注册表操作 ==========
const (
	appAutoStartKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
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

// restartApplication 重启应用
func restartApplication() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}

	absPath, err := filepath.Abs(exePath)
	if err != nil {
		return
	}

	// Windows 上使用 cmd.exe 启动新进程
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd.exe", "/c", "start", "", absPath)
		cmd.Dir = filepath.Dir(absPath)
		if err := cmd.Start(); err != nil {
			return
		}
	} else {
		// Linux/Mac 上直接执行
		cmd := exec.Command(absPath)
		cmd.Dir = filepath.Dir(absPath)
		if err := cmd.Start(); err != nil {
			return
		}
	}

	// 延迟退出，给新进程启动时间
	time.Sleep(2 * time.Second)
	os.Exit(0)
}
