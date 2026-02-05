package handlers

import (
	"homedash/internal/monitor"

	"github.com/gin-gonic/gin"
)

var monitorHub *monitor.Hub

// InitMonitor 初始化监控Hub
func InitMonitor(hub *monitor.Hub) {
	monitorHub = hub
}

// GetMonitorHub 获取监控Hub
func GetMonitorHub() *monitor.Hub {
	return monitorHub
}

// HandleMonitorWebSocket 处理监控WebSocket连接
func HandleMonitorWebSocket(c *gin.Context) {
	if monitorHub == nil {
		c.JSON(500, gin.H{"error": "监控服务未初始化"})
		return
	}
	monitorHub.HandleWebSocket(c.Writer, c.Request)
}

// GetProcesses 获取进程列表
func GetProcesses(c *gin.Context) {
	processes := monitor.GetTopProcesses(20)
	c.JSON(200, processes)
}
