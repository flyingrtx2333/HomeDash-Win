package handlers

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// LogEntry 日志条目
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
}

// GetLogs 获取日志列表
func GetLogs(c *gin.Context) {
	service := c.Query("service")
	level := c.Query("level")
	limit := c.DefaultQuery("limit", "100")

	// 解析limit
	limitNum := 100
	fmt.Sscanf(limit, "%d", &limitNum)
	if limitNum > 1000 {
		limitNum = 1000
	}

	logs := make([]LogEntry, 0)

	// 如果没有指定服务，返回示例日志
	if service == "" {
		logs = append(logs, generateSampleLogs(limitNum)...)
		c.JSON(200, gin.H{
			"logs":  logs,
			"total": len(logs),
		})
		return
	}

	// 根据服务获取日志
	logPath := getServiceLogPath(service)
	if logPath == "" {
		c.JSON(200, gin.H{
			"logs":  logs,
			"total": 0,
		})
		return
	}

	// 读取日志文件
	fileLogs, err := readLogFile(logPath, limitNum, level)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"logs":  fileLogs,
		"total": len(fileLogs),
	})
}

// GetLogServices 获取支持日志查看的服务列表
func GetLogServices(c *gin.Context) {
	services := []gin.H{
		{"id": "system", "name": "系统日志", "path": ""},
		{"id": "openclaw", "name": "OpenClaw", "path": getOpenClawLogPath()},
		{"id": "lucky", "name": "Lucky", "path": getLuckyLogPath()},
		{"id": "alist", "name": "Alist", "path": getAlistLogPath()},
		{"id": "docker", "name": "Docker", "path": ""},
	}
	c.JSON(200, services)
}

// StreamLogs WebSocket流式日志（简化版，返回最近日志）
func StreamLogs(c *gin.Context) {
	service := c.Query("service")
	if service == "" {
		c.JSON(400, gin.H{"error": "请指定服务"})
		return
	}

	// 返回最近50条日志
	logPath := getServiceLogPath(service)
	if logPath == "" {
		c.JSON(200, gin.H{"logs": []LogEntry{}})
		return
	}

	logs, err := readLogFile(logPath, 50, "")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"logs": logs})
}

// 获取服务日志路径
func getServiceLogPath(service string) string {
	switch service {
	case "openclaw":
		return getOpenClawLogPath()
	case "lucky":
		return getLuckyLogPath()
	case "alist":
		return getAlistLogPath()
	case "system":
		return getSystemLogPath()
	default:
		return ""
	}
}

// 获取OpenClaw日志路径
func getOpenClawLogPath() string {
	// 常见路径
	paths := []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "OpenClaw", "logs", "openclaw.log"),
		filepath.Join(os.Getenv("APPDATA"), "OpenClaw", "logs", "openclaw.log"),
		`C:\ProgramData\OpenClaw\logs\openclaw.log`,
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// 获取Lucky日志路径
func getLuckyLogPath() string {
	paths := []string{
		`C:\lucky\logs\lucky.log`,
		filepath.Join(os.Getenv("USERPROFILE"), "lucky", "logs", "lucky.log"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// 获取Alist日志路径
func getAlistLogPath() string {
	paths := []string{
		`C:\alist-windows-amd64\log\log.log`,
		filepath.Join(os.Getenv("USERPROFILE"), "alist", "log", "log.log"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// 获取系统日志路径（Windows Event Log需要通过其他方式获取）
func getSystemLogPath() string {
	return ""
}

// 读取日志文件
func readLogFile(path string, limit int, levelFilter string) ([]LogEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var logs []LogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() && len(logs) < limit {
		line := scanner.Text()
		entry := parseLogLine(line)

		// 过滤日志级别
		if levelFilter != "" && entry.Level != levelFilter {
			continue
		}

		logs = append(logs, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}

// 解析日志行（简化版）
func parseLogLine(line string) LogEntry {
	entry := LogEntry{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Level:     "INFO",
		Source:    "system",
		Message:   line,
	}

	// 尝试解析常见的日志格式
	// 格式1: [2024-01-01 12:00:00] [INFO] message
	if strings.HasPrefix(line, "[") {
		parts := strings.SplitN(line, "]", 3)
		if len(parts) >= 2 {
			entry.Timestamp = strings.TrimPrefix(parts[0], "[")
			if len(parts) >= 3 {
				levelStr := strings.TrimSpace(strings.TrimPrefix(parts[1], "["))
				entry.Level = strings.ToUpper(levelStr)
				entry.Message = strings.TrimSpace(parts[2])
			}
		}
	}

	// 检测日志级别关键词
	upperLine := strings.ToUpper(line)
	if strings.Contains(upperLine, "ERROR") || strings.Contains(upperLine, "ERR") {
		entry.Level = "ERROR"
	} else if strings.Contains(upperLine, "WARN") || strings.Contains(upperLine, "WARNING") {
		entry.Level = "WARN"
	} else if strings.Contains(upperLine, "DEBUG") {
		entry.Level = "DEBUG"
	}

	return entry
}

// 生成示例日志
func generateSampleLogs(count int) []LogEntry {
	logs := []LogEntry{
		{Timestamp: time.Now().Format("2006-01-02 15:04:05"), Level: "INFO", Source: "system", Message: "HomeDash 服务启动成功"},
		{Timestamp: time.Now().Add(-1 * time.Minute).Format("2006-01-02 15:04:05"), Level: "INFO", Source: "monitor", Message: "系统监控初始化完成"},
		{Timestamp: time.Now().Add(-2 * time.Minute).Format("2006-01-02 15:04:05"), Level: "WARN", Source: "network", Message: "检测到网络延迟较高"},
		{Timestamp: time.Now().Add(-5 * time.Minute).Format("2006-01-02 15:04:05"), Level: "INFO", Source: "service", Message: "OpenClaw 服务状态: 运行中"},
		{Timestamp: time.Now().Add(-10 * time.Minute).Format("2006-01-02 15:04:05"), Level: "INFO", Source: "service", Message: "Lucky 服务状态: 运行中"},
	}

	if count > len(logs) {
		count = len(logs)
	}
	return logs[:count]
}

// ClearLogs 清空日志（仅支持应用日志）
func ClearLogs(c *gin.Context) {
	service := c.Param("service")
	if service == "" {
		c.JSON(400, gin.H{"error": "请指定服务"})
		return
	}

	// 只有特定服务支持清空
	if service != "openclaw" && service != "lucky" && service != "alist" {
		c.JSON(400, gin.H{"error": "该服务不支持清空日志"})
		return
	}

	logPath := getServiceLogPath(service)
	if logPath == "" {
		c.JSON(404, gin.H{"error": "未找到日志文件"})
		return
	}

	// 备份并清空
	backupPath := logPath + ".backup." + time.Now().Format("20060102150405")
	if err := os.Rename(logPath, backupPath); err != nil {
		c.JSON(500, gin.H{"error": "备份日志失败: " + err.Error()})
		return
	}

	// 创建新的空日志文件
	if _, err := os.Create(logPath); err != nil {
		c.JSON(500, gin.H{"error": "创建新日志文件失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "日志已清空并备份", "backup": backupPath})
}
