package handlers

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/process"
)

// LaunchService 启动服务
func LaunchService(c *gin.Context) {
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
		c.JSON(404, gin.H{"error": "服务不存在"})
		return
	}

	// 优先使用 LaunchCommand，否则使用 LaunchPath（向后兼容）
	var launchCmd string
	if service.LaunchCommand != "" {
		launchCmd = service.LaunchCommand
	} else if service.LaunchPath != "" {
		launchCmd = service.LaunchPath
	} else {
		c.JSON(400, gin.H{"error": "服务未配置启动命令或启动路径"})
		return
	}

	if err := launchService(launchCmd); err != nil {
		c.JSON(500, gin.H{"error": "启动失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// launchService 启动服务进程
func launchService(launchCmd string) error {
	// 假设 launchCmd 是 `C:\alist.exe server` parts 应该是 ["C:\alist.exe", "server"]
	parts := parseCommand(launchCmd)
	if len(parts) == 0 {
		return fmt.Errorf("启动命令为空")
	}

	// 直接执行，不要嵌套 cmd.exe /c start
	// 第一个元素是程序名，后面的解构为参数
	cmd := exec.Command(parts[0], parts[1:]...)
	err := cmd.Start()
	if err != nil {
		return err
	}

	return nil
}

// parseCommand 解析命令字符串，支持引号包裹的参数
func parseCommand(cmdStr string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false

	for i, r := range cmdStr {
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

// GetServiceProcessStatus 获取服务进程状态
func GetServiceProcessStatus(c *gin.Context) {
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
		c.JSON(404, gin.H{"error": "服务不存在"})
		return
	}

	// 检查是否有启动配置
	hasLaunchConfig := service.LaunchCommand != "" || service.LaunchPath != ""
	if !hasLaunchConfig {
		c.JSON(200, gin.H{"running": false, "pid": 0})
		return
	}

	status := checkServiceProcess(service.ProcessName, service.LaunchPath, service.LaunchCommand)
	c.JSON(200, status)
}

// StopService 停止服务进程
func StopService(c *gin.Context) {
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
		c.JSON(404, gin.H{"error": "服务不存在"})
		return
	}

	// 检查是否有启动配置
	hasLaunchConfig := service.LaunchCommand != "" || service.LaunchPath != ""
	if !hasLaunchConfig {
		c.JSON(400, gin.H{"error": "服务未配置启动命令或启动路径"})
		return
	}

	// 先检查进程是否存在
	status := checkServiceProcess(service.ProcessName, service.LaunchPath, service.LaunchCommand)
	if !status.Running {
		c.JSON(200, gin.H{"success": true, "message": "进程未运行"})
		return
	}

	// 停止进程
	if err := stopServiceProcess(status.PID); err != nil {
		c.JSON(500, gin.H{"error": "停止失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
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
