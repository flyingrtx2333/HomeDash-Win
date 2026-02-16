package handlers

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/process"
)

var (
	websiteProcesses = make(map[string]*exec.Cmd) // 存储运行中的项目进程
	websiteProcessMu sync.RWMutex                  // 保护websiteProcesses的互斥锁
	websiteLogFiles  = make(map[string]*os.File)  // 存储日志文件句柄
	websiteLogMu     sync.RWMutex                  // 保护websiteLogFiles的互斥锁
)

// GetWebsites 获取所有网站项目列表
func GetWebsites(c *gin.Context) {
	websites := loadWebsites()
	c.JSON(200, websites)
}

// GetWebsite 获取单个网站项目详情
func GetWebsite(c *gin.Context) {
	id := c.Param("id")
	websites := loadWebsites()

	for _, website := range websites {
		if website.ID == id {
			c.JSON(200, website)
			return
		}
	}

	c.JSON(404, gin.H{"error": "项目不存在"})
}

// CreateWebsite 创建新网站项目
func CreateWebsite(c *gin.Context) {
	var website PythonWebsite
	if err := c.ShouldBindJSON(&website); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	// 验证配置
	if err := ValidateWebsiteConfig(&website); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 生成ID和时间戳
	website.ID = uuid.New().String()[:8]
	website.CreatedAt = time.Now().UnixMilli()
	website.UpdatedAt = website.CreatedAt
	website.Enabled = true

	// 设置默认值
	if website.EnvironmentVars == nil {
		website.EnvironmentVars = make(map[string]string)
	}
	if website.WorkingDir == "" {
		website.WorkingDir = website.Path
	}
	if website.VenvPath == "" && website.Path != "" {
		website.VenvPath = filepath.Join(website.Path, "venv")
	}
	if website.RequirementsTxt == "" && website.Path != "" {
		website.RequirementsTxt = filepath.Join(website.Path, "requirements.txt")
	}

	websites := loadWebsites()
	websites = append(websites, website)

	if err := saveWebsites(websites); err != nil {
		c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
		return
	}

	c.JSON(200, website)
}

// UpdateWebsite 更新网站项目配置
func UpdateWebsite(c *gin.Context) {
	id := c.Param("id")
	var updated PythonWebsite
	if err := c.ShouldBindJSON(&updated); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	// 验证配置
	if err := ValidateWebsiteConfig(&updated); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	websites := loadWebsites()
	found := false
	for i, w := range websites {
		if w.ID == id {
			updated.ID = id
			updated.CreatedAt = w.CreatedAt
			updated.UpdatedAt = time.Now().UnixMilli()

			// 设置默认值
			if updated.EnvironmentVars == nil {
				updated.EnvironmentVars = make(map[string]string)
			}
			if updated.WorkingDir == "" {
				updated.WorkingDir = updated.Path
			}
			if updated.VenvPath == "" && updated.Path != "" {
				updated.VenvPath = filepath.Join(updated.Path, "venv")
			}
			if updated.RequirementsTxt == "" && updated.Path != "" {
				updated.RequirementsTxt = filepath.Join(updated.Path, "requirements.txt")
			}

			websites[i] = updated
			found = true
			break
		}
	}

	if !found {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}

	if err := saveWebsites(websites); err != nil {
		c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
		return
	}

	c.JSON(200, updated)
}

// DeleteWebsite 删除网站项目
func DeleteWebsite(c *gin.Context) {
	id := c.Param("id")
	websites := loadWebsites()
	newWebsites := make([]PythonWebsite, 0)
	found := false

	for _, w := range websites {
		if w.ID == id {
			found = true
			// 如果项目正在运行，先停止
			if status := getWebsiteProcessStatus(id); status.Running {
				stopWebsiteProcess(id, status.PID)
			}
		} else {
			newWebsites = append(newWebsites, w)
		}
	}

	if !found {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}

	if err := saveWebsites(newWebsites); err != nil {
		c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// ValidateWebsiteConfig 验证网站项目配置
// Python路径与虚拟环境路径二选一必填：有虚拟环境则不需要 Python 路径，否则需要 Python 路径
func ValidateWebsiteConfig(website *PythonWebsite) error {
	if website.Name == "" {
		return fmt.Errorf("项目名称不能为空")
	}
	if website.Path == "" {
		return fmt.Errorf("项目路径不能为空")
	}
	if website.Port <= 0 || website.Port > 65535 {
		return fmt.Errorf("端口必须在1-65535之间")
	}
	if website.StartCommand == "" {
		return fmt.Errorf("启动命令不能为空")
	}

	// Python路径与虚拟环境二选一必填
	if website.VenvPath == "" && website.PythonPath == "" {
		return fmt.Errorf("请填写 Python 路径或虚拟环境路径（二选一）")
	}

	// 若有虚拟环境，校验虚拟环境路径存在且有效
	if website.VenvPath != "" {
		if _, err := os.Stat(website.VenvPath); os.IsNotExist(err) {
			return fmt.Errorf("虚拟环境路径不存在: %s", website.VenvPath)
		}
		var pythonInVenv string
		if runtime.GOOS == "windows" {
			pythonInVenv = filepath.Join(website.VenvPath, "Scripts", "python.exe")
		} else {
			pythonInVenv = filepath.Join(website.VenvPath, "bin", "python")
		}
		if _, err := os.Stat(pythonInVenv); os.IsNotExist(err) {
			return fmt.Errorf("虚拟环境中未找到 Python: %s", website.VenvPath)
		}
	} else {
		// 没有虚拟环境时，Python 路径必填且必须存在
		if _, err := os.Stat(website.PythonPath); os.IsNotExist(err) {
			return fmt.Errorf("Python可执行文件不存在: %s", website.PythonPath)
		}
	}

	// 检查端口是否可用
	if !checkPortAvailable(website.Port) {
		return fmt.Errorf("端口 %d 已被占用", website.Port)
	}

	return nil
}

// checkPortAvailable 检查端口是否可用
func checkPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// GetPythonVersions 检测系统中已安装的Python版本
func GetPythonVersions(c *gin.Context) {
	versions := detectPythonVersions()
	c.JSON(200, versions)
}

// detectPythonVersions 检测Python版本
func detectPythonVersions() []PythonVersion {
	var versions []PythonVersion

	if runtime.GOOS == "windows" {
		// Windows: 使用 py launcher 检测
		cmd := exec.Command("py", "-0")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "Installed") {
					continue
				}
				// 解析格式: "-3.11    C:\Python311\python.exe"
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					path := parts[len(parts)-1]
					if _, err := os.Stat(path); err == nil {
						// 获取实际版本号
						actualVersion := getPythonVersion(path)
						if actualVersion != "" {
							versions = append(versions, PythonVersion{
								Version:   actualVersion,
								Path:      path,
								IsDefault: false,
							})
						}
					}
				}
			}
		}

		// 检查常见安装路径
		commonPaths := []string{
			"C:\\Python*",
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Python", "*"),
			filepath.Join(os.Getenv("PROGRAMFILES"), "Python*"),
		}

		for _, pattern := range commonPaths {
			matches, _ := filepath.Glob(pattern)
			for _, match := range matches {
				pythonExe := filepath.Join(match, "python.exe")
				if _, err := os.Stat(pythonExe); err == nil {
					version := getPythonVersion(pythonExe)
					if version != "" {
						// 检查是否已存在
						exists := false
						for _, v := range versions {
							if v.Path == pythonExe {
								exists = true
								break
							}
						}
						if !exists {
							versions = append(versions, PythonVersion{
								Version:   version,
								Path:      pythonExe,
								IsDefault: false,
							})
						}
					}
				}
			}
		}

		// 检查默认python命令
		if pythonPath, err := exec.LookPath("python"); err == nil {
			version := getPythonVersion(pythonPath)
			if version != "" {
				exists := false
				for _, v := range versions {
					if v.Path == pythonPath {
						exists = true
						break
					}
				}
				if !exists {
					versions = append(versions, PythonVersion{
						Version:   version,
						Path:      pythonPath,
						IsDefault: true,
					})
				} else {
					// 标记为默认
					for i := range versions {
						if versions[i].Path == pythonPath {
							versions[i].IsDefault = true
							break
						}
					}
				}
			}
		}
	} else {
		// Linux/Mac: 检查常见路径
		commonPaths := []string{
			"/usr/bin/python3",
			"/usr/local/bin/python3",
			"/opt/homebrew/bin/python3",
		}

		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				version := getPythonVersion(path)
				if version != "" {
					versions = append(versions, PythonVersion{
						Version:   version,
						Path:      path,
						IsDefault: false,
					})
				}
			}
		}

		// 检查默认python3命令
		if pythonPath, err := exec.LookPath("python3"); err == nil {
			version := getPythonVersion(pythonPath)
			if version != "" {
				exists := false
				for _, v := range versions {
					if v.Path == pythonPath {
						exists = true
						break
					}
				}
				if !exists {
					versions = append(versions, PythonVersion{
						Version:   version,
						Path:      pythonPath,
						IsDefault: true,
					})
				}
			}
		}
	}

	return versions
}

// getPythonVersion 获取Python版本号
func getPythonVersion(pythonPath string) string {
	cmd := exec.Command(pythonPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	version := strings.TrimSpace(string(output))
	// 格式: "Python 3.11.0"
	if strings.HasPrefix(version, "Python ") {
		return strings.TrimPrefix(version, "Python ")
	}
	return version
}

// CreateVenv 创建虚拟环境
func CreateVenv(c *gin.Context) {
	id := c.Param("id")
	websites := loadWebsites()

	var website *PythonWebsite
	for i := range websites {
		if websites[i].ID == id {
			website = &websites[i]
			break
		}
	}

	if website == nil {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}

	// 创建虚拟环境必须指定 Python 路径
	if website.PythonPath == "" {
		c.JSON(400, gin.H{"error": "创建虚拟环境需要先指定 Python 路径"})
		return
	}
	if _, err := os.Stat(website.PythonPath); os.IsNotExist(err) {
		c.JSON(400, gin.H{"error": "Python可执行文件不存在: " + website.PythonPath})
		return
	}

	venvPath := website.VenvPath
	if venvPath == "" {
		venvPath = filepath.Join(website.Path, "venv")
	}

	// 检查虚拟环境是否已存在
	if _, err := os.Stat(venvPath); err == nil {
		c.JSON(400, gin.H{"error": "虚拟环境已存在"})
		return
	}

	// 创建虚拟环境
	cmd := exec.Command(website.PythonPath, "-m", "venv", venvPath)
	cmd.Dir = website.Path
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: createNoWindow,
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(500, gin.H{"error": "创建虚拟环境失败: " + string(output)})
		return
	}

	// 更新项目配置
	website.VenvPath = venvPath
	website.UpdatedAt = time.Now().UnixMilli()
	saveWebsites(websites)

	c.JSON(200, gin.H{"success": true, "venvPath": venvPath})
}

// DeleteVenv 删除虚拟环境
func DeleteVenv(c *gin.Context) {
	id := c.Param("id")
	websites := loadWebsites()

	var website *PythonWebsite
	for i := range websites {
		if websites[i].ID == id {
			website = &websites[i]
			break
		}
	}

	if website == nil {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}

	venvPath := website.VenvPath
	if venvPath == "" {
		c.JSON(400, gin.H{"error": "虚拟环境路径未配置"})
		return
	}

	// 删除虚拟环境目录
	if err := os.RemoveAll(venvPath); err != nil {
		c.JSON(500, gin.H{"error": "删除虚拟环境失败: " + err.Error()})
		return
	}

	// 更新项目配置
	website.VenvPath = ""
	website.UpdatedAt = time.Now().UnixMilli()
	saveWebsites(websites)

	c.JSON(200, gin.H{"success": true})
}

// InstallRequirements 安装依赖
func InstallRequirements(c *gin.Context) {
	id := c.Param("id")
	websites := loadWebsites()

	var website *PythonWebsite
	for i := range websites {
		if websites[i].ID == id {
			website = &websites[i]
			break
		}
	}

	if website == nil {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}

	requirementsPath := website.RequirementsTxt
	if requirementsPath == "" {
		requirementsPath = filepath.Join(website.Path, "requirements.txt")
	}

	// 检查requirements.txt是否存在
	if _, err := os.Stat(requirementsPath); os.IsNotExist(err) {
		c.JSON(400, gin.H{"error": "requirements.txt文件不存在"})
		return
	}

	// 确定pip路径
	var pipPath string
	if website.VenvPath != "" {
		if runtime.GOOS == "windows" {
			pipPath = filepath.Join(website.VenvPath, "Scripts", "pip.exe")
		} else {
			pipPath = filepath.Join(website.VenvPath, "bin", "pip")
		}
	} else {
		// 使用系统pip
		if runtime.GOOS == "windows" {
			pipPath = "pip"
		} else {
			pipPath = "pip3"
		}
	}

	// 安装依赖
	cmd := exec.Command(pipPath, "install", "-r", requirementsPath)
	cmd.Dir = website.Path
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: createNoWindow,
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(500, gin.H{"error": "安装依赖失败: " + string(output)})
		return
	}

	c.JSON(200, gin.H{"success": true, "output": string(output)})
}

// StartWebsite 启动网站项目
func StartWebsite(c *gin.Context) {
	id := c.Param("id")
	websites := loadWebsites()

	var website *PythonWebsite
	for i := range websites {
		if websites[i].ID == id {
			website = &websites[i]
			break
		}
	}

	if website == nil {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}

	// 检查是否已在运行
	if status := getWebsiteProcessStatus(id); status.Running {
		c.JSON(400, gin.H{"error": "项目已在运行中"})
		return
	}

	// 检查端口是否可用
	if !checkPortAvailable(website.Port) {
		c.JSON(400, gin.H{"error": fmt.Sprintf("端口 %d 已被占用", website.Port)})
		return
	}

	// 准备启动命令
	var cmd *exec.Cmd
	workingDir := website.WorkingDir
	if workingDir == "" {
		workingDir = website.Path
	}

	// 构建命令
	commandParts := parseCommand(website.StartCommand)
	if len(commandParts) == 0 {
		c.JSON(400, gin.H{"error": "启动命令无效"})
		return
	}

	// 如果有虚拟环境，使用虚拟环境中的Python
	if website.VenvPath != "" {
		var pythonPath string
		if runtime.GOOS == "windows" {
			pythonPath = filepath.Join(website.VenvPath, "Scripts", "python.exe")
		} else {
			pythonPath = filepath.Join(website.VenvPath, "bin", "python")
		}
		// 如果第一个参数是python/python3，替换为虚拟环境的python
		if commandParts[0] == "python" || commandParts[0] == "python3" || strings.HasSuffix(commandParts[0], "python.exe") {
			commandParts[0] = pythonPath
		} else {
			// 否则在命令前添加虚拟环境的python
			newParts := []string{pythonPath}
			commandParts = append(newParts, commandParts...)
		}
		cmd = exec.Command(commandParts[0], commandParts[1:]...)
	} else {
		// 使用配置的Python路径
		if commandParts[0] == "python" || commandParts[0] == "python3" {
			commandParts[0] = website.PythonPath
		}
		cmd = exec.Command(commandParts[0], commandParts[1:]...)
	}

	cmd.Dir = workingDir

	// 设置环境变量
	env := os.Environ()
	for k, v := range website.EnvironmentVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	// 设置FLASK_APP, PORT等常用环境变量
	if website.Framework == "flask" {
		env = append(env, fmt.Sprintf("FLASK_RUN_PORT=%d", website.Port))
	}
	env = append(env, fmt.Sprintf("PORT=%d", website.Port))
	cmd.Env = env

	// 创建日志目录
	logDir := filepath.Join(website.Path, "logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", id))

	// 打开日志文件
	logFileHandle, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		c.JSON(500, gin.H{"error": "创建日志文件失败: " + err.Error()})
		return
	}

	// 重定向输出到日志文件
	cmd.Stdout = logFileHandle
	cmd.Stderr = logFileHandle

	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: createNoWindow,
		}
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		logFileHandle.Close()
		c.JSON(500, gin.H{"error": "启动失败: " + err.Error()})
		return
	}

	// 保存进程和日志文件句柄
	websiteProcessMu.Lock()
	websiteProcesses[id] = cmd
	websiteProcessMu.Unlock()

	websiteLogMu.Lock()
	websiteLogFiles[id] = logFileHandle
	websiteLogMu.Unlock()

	c.JSON(200, gin.H{"success": true, "pid": cmd.Process.Pid})
}

// StopWebsite 停止网站项目
func StopWebsite(c *gin.Context) {
	id := c.Param("id")

	status := getWebsiteProcessStatus(id)
	if !status.Running {
		c.JSON(200, gin.H{"success": true, "message": "项目未运行"})
		return
	}

	if err := stopWebsiteProcess(id, status.PID); err != nil {
		c.JSON(500, gin.H{"error": "停止失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// stopWebsiteProcess 停止项目进程
func stopWebsiteProcess(id string, pid int32) error {
	websiteProcessMu.Lock()
	cmd, exists := websiteProcesses[id]
	if exists {
		delete(websiteProcesses, id)
	}
	websiteProcessMu.Unlock()

	// 关闭日志文件
	websiteLogMu.Lock()
	if logFile, exists := websiteLogFiles[id]; exists {
		logFile.Close()
		delete(websiteLogFiles, id)
	}
	websiteLogMu.Unlock()

	// 如果cmd存在，尝试通过cmd停止
	if cmd != nil && cmd.Process != nil {
		if runtime.GOOS == "windows" {
			cmd.Process.Kill()
		} else {
			cmd.Process.Signal(os.Interrupt)
			time.Sleep(2 * time.Second)
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
				cmd.Process.Kill()
			}
		}
		return nil
	}

	// 否则通过PID停止
	if runtime.GOOS == "windows" {
		killCmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
		return killCmd.Run()
	} else {
		proc, err := process.NewProcess(pid)
		if err != nil {
			return err
		}
		proc.Terminate()
		time.Sleep(2 * time.Second)
		exists, _ := process.PidExists(pid)
		if exists {
			proc.Kill()
		}
		return nil
	}
}

// GetWebsiteStatus 获取网站项目运行状态
func GetWebsiteStatus(c *gin.Context) {
	id := c.Param("id")
	status := getWebsiteProcessStatus(id)
	c.JSON(200, status)
}

// getWebsiteProcessStatus 获取项目进程状态
func getWebsiteProcessStatus(id string) ProcessStatus {
	websiteProcessMu.RLock()
	cmd, exists := websiteProcesses[id]
	websiteProcessMu.RUnlock()

	if exists && cmd != nil && cmd.Process != nil {
		// 检查进程是否还在运行
		if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
			return ProcessStatus{
				Running: true,
				PID:     int32(cmd.Process.Pid),
			}
		}
		// 进程已退出，清理
		websiteProcessMu.Lock()
		delete(websiteProcesses, id)
		websiteProcessMu.Unlock()
	}

	// 内存中无记录时，根据端口查找占用进程（如应用重启后或进程由外部启动）
	websites := loadWebsites()
	for _, w := range websites {
		if w.ID == id {
			if w.Port > 0 {
				pid := findProcessByPort(w.Port)
				if pid > 0 {
					return ProcessStatus{
						Running: true,
						PID:     pid,
					}
				}
			}
			break
		}
	}

	return ProcessStatus{Running: false, PID: 0}
}

// findProcessByPort 查找占用指定端口的进程（优先通过 LISTENING 状态查找 TCP 监听）
func findProcessByPort(port int) int32 {
	if runtime.GOOS == "windows" {
		// 使用 netstat -ano 获取连接列表
		cmd := exec.Command("netstat", "-ano")
		output, err := cmd.Output()
		if err != nil {
			log.Printf("[findProcessByPort] netstat 执行失败: %v", err)
			return 0
		}
		portStr := fmt.Sprintf(":%d", port)
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// 只处理 LISTENING 状态的 TCP 行（服务端监听）
			if !strings.Contains(line, "LISTENING") {
				continue
			}
			if !strings.Contains(line, "TCP") {
				continue
			}
			if !strings.Contains(line, portStr) {
				continue
			}
			// 精确匹配端口，避免 :5000 匹配到 :50001
			idx := strings.Index(line, portStr)
			if idx >= 0 && idx+len(portStr) < len(line) {
				next := line[idx+len(portStr)]
				if next >= '0' && next <= '9' {
					continue
				}
			}
			parts := strings.Fields(line)
			if len(parts) < 1 {
				continue
			}
			var pid int32
			if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &pid); err == nil && pid > 0 {
				return pid
			}
		}
		// 若未找到 LISTENING，再尝试匹配任意包含该端口的行（兼容不同输出格式）
		for _, line := range lines {
			if !strings.Contains(line, portStr) {
				continue
			}
			idx := strings.Index(line, portStr)
			if idx >= 0 && idx+len(portStr) < len(line) {
				next := line[idx+len(portStr)]
				if next >= '0' && next <= '9' {
					continue
				}
			}
			parts := strings.Fields(line)
			if len(parts) < 1 {
				continue
			}
			var pid int32
			if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &pid); err == nil && pid > 0 {
				return pid
			}
		}
	} else {
		// Linux/macOS: 使用 lsof 或 ss
		cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-t")
		output, err := cmd.Output()
		if err == nil {
			pidStr := strings.TrimSpace(string(output))
			if pidStr != "" {
				lines := strings.Split(pidStr, "\n")
				if len(lines) > 0 {
					var pid int32
					if _, err := fmt.Sscanf(lines[0], "%d", &pid); err == nil && pid > 0 {
						return pid
					}
				}
			}
		}
	}
	return 0
}

// GetWebsiteLogs 获取网站项目日志
func GetWebsiteLogs(c *gin.Context) {
	id := c.Param("id")
	websites := loadWebsites()

	var website *PythonWebsite
	for _, w := range websites {
		if w.ID == id {
			website = &w
			break
		}
	}

	if website == nil {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}

	logDir := filepath.Join(website.Path, "logs")
	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", id))

	// 读取日志文件
	data, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(200, gin.H{"logs": ""})
			return
		}
		c.JSON(500, gin.H{"error": "读取日志失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"logs": string(data)})
}

// StreamWebsiteLogs 流式输出网站项目日志（WebSocket）
func StreamWebsiteLogs(c *gin.Context) {
	id := c.Param("id")
	websites := loadWebsites()

	var website *PythonWebsite
	for _, w := range websites {
		if w.ID == id {
			website = &w
			break
		}
	}

	if website == nil {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}

	// 升级到WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	logDir := filepath.Join(website.Path, "logs")
	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", id))

	// 打开日志文件
	file, err := os.Open(logFile)
	if err != nil {
		conn.WriteJSON(gin.H{"error": "打开日志文件失败"})
		return
	}
	defer file.Close()

	// 读取文件末尾
	file.Seek(0, io.SeekEnd)
	reader := bufio.NewReader(file)

	// 持续读取并发送
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			break
		}

		if err := conn.WriteJSON(gin.H{"log": line}); err != nil {
			break
		}
	}
}

// ProjectDetectResult 项目检测结果
type ProjectDetectResult struct {
	Framework       string `json:"framework"`       // 检测到的框架类型
	VenvPath        string `json:"venvPath"`       // 检测到的虚拟环境路径
	RequirementsTxt string `json:"requirementsTxt"` // 检测到的requirements.txt路径
	StartCommand    string `json:"startCommand"`    // 建议的启动命令
}

// DetectProjectInfo 检测项目信息
func DetectProjectInfo(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	if req.Path == "" {
		c.JSON(400, gin.H{"error": "项目路径不能为空"})
		return
	}

	log.Printf("[检测项目] 开始检测路径: %s", req.Path)

	// 检查路径是否存在
	pathInfo, err := os.Stat(req.Path)
	if err != nil {
		log.Printf("[检测项目] 路径不存在: %v", err)
		c.JSON(400, gin.H{"error": "路径不存在: " + err.Error()})
		return
	}
	if !pathInfo.IsDir() {
		log.Printf("[检测项目] 路径不是目录")
		c.JSON(400, gin.H{"error": "路径不是目录"})
		return
	}

	result := ProjectDetectResult{
		Framework: "custom",
	}

	// 检测框架类型
	result.Framework = detectFramework(req.Path)
	if result.Framework == "" {
		result.Framework = "custom"
	}
	log.Printf("[检测项目] 检测到框架: %s", result.Framework)

	// 检测虚拟环境
	result.VenvPath = detectVenv(req.Path)
	log.Printf("[检测项目] 检测到虚拟环境: %s", result.VenvPath)

	// 检测requirements.txt
	result.RequirementsTxt = detectRequirementsTxt(req.Path)
	log.Printf("[检测项目] 检测到requirements.txt: %s", result.RequirementsTxt)

	// 生成建议的启动命令
	result.StartCommand = suggestStartCommand(result.Framework, req.Path)
	log.Printf("[检测项目] 建议启动命令: %s", result.StartCommand)

	c.JSON(200, result)
}

// detectFramework 检测Python框架类型
func detectFramework(projectPath string) string {
	// 检查Django
	if _, err := os.Stat(filepath.Join(projectPath, "manage.py")); err == nil {
		return "django"
	}

	// 检查Flask - 查找app.py或main.py中的Flask导入
	flaskFiles := []string{"app.py", "main.py", "run.py", "__init__.py"}
	for _, filename := range flaskFiles {
		filePath := filepath.Join(projectPath, filename)
		if content, err := os.ReadFile(filePath); err == nil {
			contentStr := strings.ToLower(string(content))
			if strings.Contains(contentStr, "from flask import") || strings.Contains(contentStr, "import flask") {
				return "flask"
			}
		}
	}

	// 检查FastAPI - 查找main.py或app.py中的FastAPI导入
	for _, filename := range flaskFiles {
		filePath := filepath.Join(projectPath, filename)
		if content, err := os.ReadFile(filePath); err == nil {
			contentStr := strings.ToLower(string(content))
			if strings.Contains(contentStr, "from fastapi import") || strings.Contains(contentStr, "import fastapi") {
				return "fastapi"
			}
		}
	}

	// 检查requirements.txt中的依赖
	reqPath := filepath.Join(projectPath, "requirements.txt")
	if content, err := os.ReadFile(reqPath); err == nil {
		contentStr := strings.ToLower(string(content))
		if strings.Contains(contentStr, "django") {
			return "django"
		}
		if strings.Contains(contentStr, "flask") {
			return "flask"
		}
		if strings.Contains(contentStr, "fastapi") || strings.Contains(contentStr, "uvicorn") {
			return "fastapi"
		}
	}

	// 检查pyproject.toml
	pyprojectPath := filepath.Join(projectPath, "pyproject.toml")
	if content, err := os.ReadFile(pyprojectPath); err == nil {
		contentStr := strings.ToLower(string(content))
		if strings.Contains(contentStr, "django") {
			return "django"
		}
		if strings.Contains(contentStr, "flask") {
			return "flask"
		}
		if strings.Contains(contentStr, "fastapi") {
			return "fastapi"
		}
	}

	return ""
}

// detectVenv 检测虚拟环境路径
func detectVenv(projectPath string) string {
	// 常见的虚拟环境目录名
	venvNames := []string{"venv", ".venv", "env", ".env", "virtualenv"}
	for _, venvName := range venvNames {
		venvPath := filepath.Join(projectPath, venvName)
		if info, err := os.Stat(venvPath); err == nil && info.IsDir() {
			// 检查是否是有效的虚拟环境（包含Scripts或bin目录）
			if runtime.GOOS == "windows" {
				scriptsPath := filepath.Join(venvPath, "Scripts", "python.exe")
				if _, err := os.Stat(scriptsPath); err == nil {
					return venvPath
				}
			} else {
				binPath := filepath.Join(venvPath, "bin", "python")
				if _, err := os.Stat(binPath); err == nil {
					return venvPath
				}
			}
		}
	}
	return ""
}

// detectRequirementsTxt 检测requirements.txt路径
func detectRequirementsTxt(projectPath string) string {
	reqPath := filepath.Join(projectPath, "requirements.txt")
	if _, err := os.Stat(reqPath); err == nil {
		return reqPath
	}
	return ""
}

// suggestStartCommand 根据框架类型建议启动命令
func suggestStartCommand(framework, projectPath string) string {
	switch framework {
	case "flask":
		// 检查是否有app.py或main.py
		if _, err := os.Stat(filepath.Join(projectPath, "app.py")); err == nil {
			return "flask run"
		}
		if _, err := os.Stat(filepath.Join(projectPath, "main.py")); err == nil {
			return "flask run"
		}
		return "flask run"
	case "django":
		return "python manage.py runserver"
	case "fastapi":
		// 尝试找到main.py或app.py
		if _, err := os.Stat(filepath.Join(projectPath, "main.py")); err == nil {
			return "uvicorn main:app --reload"
		}
		if _, err := os.Stat(filepath.Join(projectPath, "app.py")); err == nil {
			return "uvicorn app:app --reload"
		}
		return "uvicorn main:app --reload"
	default:
		// 检查是否有app.py或main.py
		if _, err := os.Stat(filepath.Join(projectPath, "app.py")); err == nil {
			return "python app.py"
		}
		if _, err := os.Stat(filepath.Join(projectPath, "main.py")); err == nil {
			return "python main.py"
		}
		return "python app.py"
	}
}

// BrowseDirectory 浏览目录（用于文件选择器）
func BrowseDirectory(c *gin.Context) {
	reqPath := c.Query("path")
	
	// 特殊处理：如果path为空或"root"，返回Windows盘符列表
	if reqPath == "" || reqPath == "root" {
		if runtime.GOOS == "windows" {
			drives := []FileInfo{}
			for drive := 'A'; drive <= 'Z'; drive++ {
				drivePath := string(drive) + ":\\"
				if info, err := os.Stat(drivePath); err == nil && info.IsDir() {
					drives = append(drives, FileInfo{
						Name:    string(drive) + ":",
						Path:    drivePath,
						IsDir:   true,
						Size:    0,
						ModTime: 0,
					})
				}
			}
			c.JSON(200, gin.H{
				"path":  "root",
				"files": drives,
			})
			return
		} else {
			// Linux/Mac: 从根目录开始
			reqPath = "/"
		}
	}

	// 规范化路径
	cleanPath := filepath.Clean(reqPath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		c.JSON(500, gin.H{"error": "路径解析失败: " + err.Error()})
		return
	}

	// 检查路径是否存在且是目录
	info, err := os.Stat(absPath)
	if err != nil {
		c.JSON(404, gin.H{"error": "路径不存在: " + err.Error()})
		return
	}
	if !info.IsDir() {
		c.JSON(400, gin.H{"error": "路径不是目录"})
		return
	}

	// 读取目录内容
	entries, err := os.ReadDir(absPath)
	if err != nil {
		c.JSON(500, gin.H{"error": "读取目录失败: " + err.Error()})
		return
	}

	var dirs []FileInfo
	var files []FileInfo

	for _, entry := range entries {
		// 跳过隐藏文件（可选）
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileInfo := FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(absPath, entry.Name()),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UnixMilli(),
		}

		if entry.IsDir() {
			dirs = append(dirs, fileInfo)
		} else {
			files = append(files, fileInfo)
		}
	}

	// 排序
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	// 合并：目录在前
	allFiles := append(dirs, files...)

	c.JSON(200, gin.H{
		"path":  absPath,
		"files": allFiles,
	})
}
