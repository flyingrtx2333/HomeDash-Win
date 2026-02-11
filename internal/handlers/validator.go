package handlers

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ValidateServiceConfig 验证服务配置
func ValidateServiceConfig(service *ServiceCard) error {
	if service.Name == "" {
		return fmt.Errorf("服务名称不能为空")
	}

	if len(service.Name) > 100 {
		return fmt.Errorf("服务名称过长（最大100字符）")
	}

	// 验证端口
	if service.Port < 0 || service.Port > 65535 {
		return fmt.Errorf("端口号无效")
	}

	// 验证启动路径（如果提供）
	if service.LaunchPath != "" {
		// 检查是否为绝对路径
		if !filepath.IsAbs(service.LaunchPath) {
			return fmt.Errorf("启动路径必须是绝对路径")
		}

		// 检查文件是否存在
		if _, err := os.Stat(service.LaunchPath); os.IsNotExist(err) {
			return fmt.Errorf("启动路径指向的文件不存在")
		}

		// 检查文件扩展名（Windows 可执行文件）
		ext := strings.ToLower(filepath.Ext(service.LaunchPath))
		validExts := map[string]bool{".exe": true, ".bat": true, ".cmd": true, ".ps1": true}
		if !validExts[ext] {
			return fmt.Errorf("启动路径必须是可执行文件")
		}
	}

	// 验证启动命令（如果提供）
	if service.LaunchCommand != "" {
		// 检查是否包含危险命令
		dangerousPatterns := []string{
			"del ", "rm ", "format ", "shutdown ",
			"rmdir ", "rd ", "deltree ",
		}
		for _, pattern := range dangerousPatterns {
			if strings.Contains(strings.ToLower(service.LaunchCommand), pattern) {
				return fmt.Errorf("启动命令包含危险操作")
			}
		}
	}

	// 验证进程名（如果提供）
	if service.ProcessName != "" {
		// 检查是否包含非法字符
		if strings.ContainsAny(service.ProcessName, "/\\:*?\"<>|") {
			return fmt.Errorf("进程名包含非法字符")
		}

		// 验证文件扩展名
		if !strings.HasSuffix(strings.ToLower(service.ProcessName), ".exe") {
			return fmt.Errorf("进程名必须以 .exe 结尾")
		}
	}

	// 验证图标（如果提供）
	if service.Icon != "" {
		// 检查是否为 emoji（简单验证）
		if !strings.HasPrefix(service.Icon, "/static/") {
			// emoji 检查（每个 rune 的 Unicode 值）
			for _, r := range service.Icon {
				if r > 0x1F600 && r < 0x1F64F { // 表情范围
					return nil
				}
			}
			// 如果不是 emoji 也不是路径，可能是无效的图标
			return fmt.Errorf("图标格式无效")
		}
	}

	return nil
}

// ValidateUserSettings 验证用户设置
func ValidateUserSettings(settings *UserSettings) error {
	// 验证 ServerIP（如果提供）
	if settings.ServerIP != "" {
		if !isValidIP(settings.ServerIP) {
			return fmt.Errorf("服务器 IP 地址无效")
		}
	}

	// 验证主题
	if settings.Theme != "" && settings.Theme != "dark" && settings.Theme != "light" {
		return fmt.Errorf("主题必须是 'dark' 或 'light'")
	}

	// 验证 WebDAV 根目录（如果提供）
	if settings.WebdavRoot != "" {
		if !filepath.IsAbs(settings.WebdavRoot) {
			return fmt.Errorf("WebDAV 根目录必须是绝对路径")
		}

		// 检查目录是否存在
		if _, err := os.Stat(settings.WebdavRoot); os.IsNotExist(err) {
			return fmt.Errorf("WebDAV 根目录不存在")
		}
	}

	// 验证 ComfyUI 服务器 URL（如果提供）
	if settings.ComfyUIServerURL != "" {
		if !isValidURL(settings.ComfyUIServerURL) {
			return fmt.Errorf("ComfyUI 服务器 URL 格式无效")
		}
	}

	return nil
}

// isValidIP 验证 IP 地址
func isValidIP(ip string) bool {
	// 检查是否为 "localhost"
	if ip == "localhost" {
		return true
	}

	// 尝试解析为 IP 地址
	if parsedIP := net.ParseIP(ip); parsedIP != nil {
		return true
	}

	return false
}

// isValidURL 验证 URL 格式
func isValidURL(url string) bool {
	// 简单的 URL 验证
	pattern := `^https?://[a-zA-Z0-9\-\.]+(:[0-9]+)?(/.*)?$`
	re := regexp.MustCompile(pattern)
	return re.MatchString(url)
}

// ValidateFileUpload 验证文件上传
func ValidateFileUpload(filename string, size int64, maxSize int64) error {
	if filename == "" {
		return fmt.Errorf("文件名不能为空")
	}

	// 检查文件名长度
	if len(filename) > 255 {
		return fmt.Errorf("文件名过长")
	}

	// 检查非法字符
	if strings.ContainsAny(filename, "/\\:*?\"<>|") {
		return fmt.Errorf("文件名包含非法字符")
	}

	// 检查文件大小
	if size > maxSize {
		return fmt.Errorf("文件过大")
	}

	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(filename))
	dangerousExts := map[string]bool{
		".exe": true, ".bat": true, ".cmd": true, ".sh": true,
		".ps1": true, ".vbs": true, ".js": true,
	}
	if dangerousExts[ext] {
		return fmt.Errorf("不允许上传此类型的文件")
	}

	return nil
}
