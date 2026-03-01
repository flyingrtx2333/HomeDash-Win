package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"homedash/internal/ui"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	npmProcesses = make(map[string]*exec.Cmd)
	npmProcessMu sync.RWMutex
	npmLogFiles  = make(map[string]*os.File)
	npmLogMu     sync.RWMutex
)

// GetNpmProjects 获取所有 Node 项目列表
func GetNpmProjects(c *gin.Context) {
	projects := loadNpmProjects()
	c.JSON(200, projects)
}

// GetNpmProject 获取单个 Node 项目
func GetNpmProject(c *gin.Context) {
	id := c.Param("id")
	projects := loadNpmProjects()
	for _, p := range projects {
		if p.ID == id {
			c.JSON(200, p)
			return
		}
	}
	c.JSON(404, gin.H{"error": "项目不存在"})
}

// CreateNpmProject 创建 Node 项目
func CreateNpmProject(c *gin.Context) {
	var project NodeProject
	if err := c.ShouldBindJSON(&project); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}
	if err := validateNpmProject(&project); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	project.ID = uuid.New().String()[:8]
	project.CreatedAt = time.Now().UnixMilli()
	project.UpdatedAt = project.CreatedAt
	if project.WorkingDir == "" {
		project.WorkingDir = project.Path
	}
	projects := loadNpmProjects()
	projects = append(projects, project)
	if err := saveNpmProjects(projects); err != nil {
		c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
		return
	}
	c.JSON(200, project)
}

// UpdateNpmProject 更新 Node 项目
func UpdateNpmProject(c *gin.Context) {
	id := c.Param("id")
	var updated NodeProject
	if err := c.ShouldBindJSON(&updated); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}
	if err := validateNpmProject(&updated); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	projects := loadNpmProjects()
	found := false
	for i, p := range projects {
		if p.ID == id {
			updated.ID = id
			updated.CreatedAt = p.CreatedAt
			updated.UpdatedAt = time.Now().UnixMilli()
			if updated.WorkingDir == "" {
				updated.WorkingDir = updated.Path
			}
			projects[i] = updated
			found = true
			break
		}
	}
	if !found {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}
	if err := saveNpmProjects(projects); err != nil {
		c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
		return
	}
	c.JSON(200, updated)
}

// DeleteNpmProject 删除 Node 项目
func DeleteNpmProject(c *gin.Context) {
	id := c.Param("id")
	projects := loadNpmProjects()
	newProjects := make([]NodeProject, 0)
	found := false
	for _, p := range projects {
		if p.ID == id {
			found = true
			if status := getNpmProcessStatus(id); status.Running {
				stopNpmProcess(id, status.PID)
			}
		} else {
			newProjects = append(newProjects, p)
		}
	}
	if !found {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}
	if err := saveNpmProjects(newProjects); err != nil {
		c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func validateNpmProject(p *NodeProject) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("项目名称不能为空")
	}
	if strings.TrimSpace(p.Path) == "" {
		return fmt.Errorf("项目路径不能为空")
	}
	if strings.TrimSpace(p.StartCommand) == "" {
		return fmt.Errorf("启动命令不能为空")
	}
	if p.Port > 0 && !checkPortAvailable(p.Port) {
		return fmt.Errorf("端口 %d 已被占用", p.Port)
	}
	return nil
}

// StartNpmProject 启动 Node 项目
func StartNpmProject(c *gin.Context) {
	id := c.Param("id")
	projects := loadNpmProjects()
	var project *NodeProject
	for i := range projects {
		if projects[i].ID == id {
			project = &projects[i]
			break
		}
	}
	if project == nil {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}
	pid, err := startNpmProjectByID(project)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "已在运行") {
			c.JSON(400, gin.H{"error": "项目已在运行中"})
			return
		}
		if strings.Contains(msg, "已被占用") {
			c.JSON(400, gin.H{"error": msg})
			return
		}
		c.JSON(500, gin.H{"error": msg})
		return
	}
	c.JSON(200, gin.H{"success": true, "pid": pid})
}

func startNpmProjectByID(project *NodeProject) (pid int, err error) {
	id := project.ID
	if status := getNpmProcessStatus(id); status.Running {
		return 0, fmt.Errorf("项目已在运行中")
	}
	if project.Port > 0 && !checkPortAvailable(project.Port) {
		return 0, fmt.Errorf("端口 %d 已被占用", project.Port)
	}
	workingDir := project.WorkingDir
	if workingDir == "" {
		workingDir = project.Path
	}
	commandParts := parseCommand(project.StartCommand)
	if len(commandParts) == 0 {
		return 0, fmt.Errorf("启动命令无效")
	}
	cmd := ui.HideWindow(commandParts[0], commandParts[1:]...)
	cmd.Dir = workingDir
	cmd.Env = os.Environ()
	if project.Port > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PORT=%d", project.Port))
	}

	logDir := filepath.Join(project.Path, "logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, id+".log")
	logFileHandle, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("创建日志文件失败: %w", err)
	}
	cmd.Stdout = logFileHandle
	cmd.Stderr = logFileHandle
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	log.Printf("[Node项目] 启动: %s, cmd=%s, dir=%s", project.Name, cmd.String(), workingDir)
	if err := cmd.Start(); err != nil {
		logFileHandle.Close()
		return 0, fmt.Errorf("启动失败: %w", err)
	}
	npmProcessMu.Lock()
	npmProcesses[id] = cmd
	npmProcessMu.Unlock()
	npmLogMu.Lock()
	npmLogFiles[id] = logFileHandle
	npmLogMu.Unlock()
	return cmd.Process.Pid, nil
}

// StopNpmProject 停止 Node 项目
func StopNpmProject(c *gin.Context) {
	id := c.Param("id")
	status := getNpmProcessStatus(id)
	if !status.Running {
		c.JSON(200, gin.H{"success": true, "message": "项目未运行"})
		return
	}
	if err := stopNpmProcess(id, status.PID); err != nil {
		c.JSON(500, gin.H{"error": "停止失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func stopNpmProcess(id string, pid int32) error {
	npmProcessMu.Lock()
	cmd, exists := npmProcesses[id]
	if exists {
		delete(npmProcesses, id)
	}
	npmProcessMu.Unlock()

	npmLogMu.Lock()
	if logFile, exists := npmLogFiles[id]; exists {
		logFile.Close()
		delete(npmLogFiles, id)
	}
	npmLogMu.Unlock()

	runTaskkill := func(targetPID int32) error {
		if targetPID <= 0 {
			return nil
		}
		killCmd := ui.HideWindow("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", targetPID))
		out, err := killCmd.CombinedOutput()
		if err == nil {
			return nil
		}
		exitCode := -1
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
		outStr := strings.ToLower(strings.TrimSpace(string(out)))
		if exitCode == 128 ||
			strings.Contains(outStr, "not found") ||
			strings.Contains(outStr, "not exist") ||
			strings.Contains(outStr, "no instance") ||
			strings.Contains(outStr, "找不到") ||
			strings.Contains(outStr, "没有找到") ||
			strings.Contains(outStr, "不存在") {
			return nil
		}
		log.Printf("[stopNpmProcess] taskkill 失败 id=%s pid=%d err=%v", id, targetPID, err)
		return err
	}

	if cmd != nil && cmd.Process != nil {
		if err := runTaskkill(int32(cmd.Process.Pid)); err != nil {
			_ = cmd.Process.Kill()
		}
		return nil
	}
	return runTaskkill(pid)
}

// GetNpmProjectStatus 获取 Node 项目运行状态
func GetNpmProjectStatus(c *gin.Context) {
	id := c.Param("id")
	status := getNpmProcessStatus(id)
	c.JSON(200, status)
}

func getNpmProcessStatus(id string) ProcessStatus {
	npmProcessMu.RLock()
	cmd, exists := npmProcesses[id]
	npmProcessMu.RUnlock()

	if exists && cmd != nil && cmd.Process != nil {
		if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
			return ProcessStatus{Running: true, PID: int32(cmd.Process.Pid)}
		}
		npmProcessMu.Lock()
		delete(npmProcesses, id)
		npmProcessMu.Unlock()
	}

	projects := loadNpmProjects()
	for _, p := range projects {
		if p.ID == id && p.Port > 0 {
			pid := findNpmProcessByPort(p.Port)
			if pid > 0 {
				return ProcessStatus{Running: true, PID: pid}
			}
			break
		}
	}
	return ProcessStatus{Running: false, PID: 0}
}

func findNpmProcessByPort(port int) int32 {
	cmd := ui.HideWindow("netstat", "-ano")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	portStr := fmt.Sprintf(":%d", port)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "LISTENING") || !strings.Contains(line, "TCP") || !strings.Contains(line, portStr) {
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
	return 0
}

// GetNpmProjectLogs 获取 Node 项目日志
func GetNpmProjectLogs(c *gin.Context) {
	id := c.Param("id")
	projects := loadNpmProjects()
	var project *NodeProject
	for _, p := range projects {
		if p.ID == id {
			project = &p
			break
		}
	}
	if project == nil {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}
	logDir := filepath.Join(project.Path, "logs")
	logFile := filepath.Join(logDir, id+".log")
	data, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(200, gin.H{"logs": ""})
			return
		}
		c.JSON(500, gin.H{"error": "读取日志失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"logs": decodeLogContent(data)})
}

// ClearNpmProjectLogs 清空 Node 项目日志
func ClearNpmProjectLogs(c *gin.Context) {
	id := c.Param("id")
	projects := loadNpmProjects()
	var project *NodeProject
	for _, p := range projects {
		if p.ID == id {
			project = &p
			break
		}
	}
	if project == nil {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}
	logDir := filepath.Join(project.Path, "logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, id+".log")
	f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		c.JSON(500, gin.H{"error": "清空日志失败: " + err.Error()})
		return
	}
	f.Close()
	c.JSON(200, gin.H{"success": true})
}

// InstallNpmDependencies 安装依赖（npm/yarn/pnpm install），实时输出到弹窗
func InstallNpmDependencies(c *gin.Context) {
	id := c.Param("id")
	projects := loadNpmProjects()
	var project *NodeProject
	for _, p := range projects {
		if p.ID == id {
			project = &p
			break
		}
	}
	if project == nil {
		c.JSON(404, gin.H{"error": "项目不存在"})
		return
	}
	workDir := project.WorkingDir
	if workDir == "" {
		workDir = project.Path
	}
	var req struct {
		PackageManager string `json:"packageManager"`
	}
	_ = c.ShouldBindJSON(&req)
	pm := strings.ToLower(strings.TrimSpace(req.PackageManager))
	if pm == "" {
		pm = "npm"
	}
	var name string
	var args []string
	switch pm {
	case "yarn":
		name = "yarn"
		args = []string{"install"}
	case "pnpm":
		name = "pnpm"
		args = []string{"install"}
	default:
		name = "npm"
		args = []string{"install"}
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		cmdSync := ui.HideWindow(name, args...)
		cmdSync.Dir = workDir
		cmdSync.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
		output, err := cmdSync.CombinedOutput()
		if err != nil {
			c.String(500, string(output)+"\n[INSTALL_FAILED] "+err.Error()+"\n")
			return
		}
		c.String(200, string(output))
		return
	}

	cmd := ui.HideWindow(name, args...)
	cmd.Dir = workDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	streamWriter := &flushWriter{w: c.Writer, f: flusher}
	cmd.Stdout = streamWriter
	cmd.Stderr = streamWriter
	log.Printf("[Node项目] 安装依赖: %s %v, dir=%s", name, args, workDir)
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(streamWriter, "\n[INSTALL_FAILED] %v\n", err)
	}
}

// MaybeLaunchNpmProjectsOnStartup 应用启动时自动启动勾选了「开机自启」的 Node 项目
func MaybeLaunchNpmProjectsOnStartup() {
	projects := loadNpmProjects()
	for i := range projects {
		p := &projects[i]
		if !p.AutoStart {
			continue
		}
		pid, err := startNpmProjectByID(p)
		if err != nil {
			log.Printf("[Node项目自启] %s 启动失败: %v", p.Name, err)
			continue
		}
		log.Printf("[Node项目自启] %s 已启动 (PID: %d)", p.Name, pid)
	}
}
