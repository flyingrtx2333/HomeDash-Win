package handlers

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 推荐服务模板
var defaultServiceTemplates = []ServiceCard{
	{ID: "lucky", Name: "Lucky", Description: "DDNS、反向代理、证书自动化", Port: 16601, Icon: "🍀", Enabled: true},
	{ID: "alist", Name: "Alist", Description: "多网盘整合与 WebDAV", Port: 5244, Icon: "/static/images/alist.png", Enabled: true},
	{ID: "immich", Name: "Immich", Description: "相册备份与 AI 检索", Port: 2283, Icon: "/static/images/immich.png", Enabled: true},
	{ID: "jellyfin", Name: "Jellyfin", Description: "媒体管理与播放", Port: 8096, Icon: "/static/images/jellyfin.jpg", Enabled: true},
	{ID: "comfyui", Name: "ComfyUI", Description: "AI 图像生成工作流", Port: 28000, Icon: "/static/images/comfyui.webp", Enabled: true},
	{ID: "rustdesk", Name: "RustDesk", Description: "开源远程桌面控制", Port: 0, Icon: "/static/images/rustdesk.png", Enabled: false},
	{ID: "sunshine", Name: "Sunshine", Description: "游戏串流服务端", Port: 0, Icon: "☀️", Enabled: false},
	{ID: "moonlight", Name: "Moonlight", Description: "游戏串流客户端", Port: 0, Icon: "🌙", Enabled: false},
}

// InitDefaultServices 初始化默认服务列表
func InitDefaultServices() {
	if _, err := os.Stat(servicesFile); err == nil {
		return // 文件已存在
	}

	defaultServices := []ServiceCard{
		{ID: "lucky", Name: "Lucky", Description: "DDNS、反向代理、证书自动化", Port: 16601, Icon: "🍀", Enabled: true, CreatedAt: time.Now().UnixMilli()},
		{ID: "alist", Name: "Alist", Description: "多网盘整合与 WebDAV", Port: 5244, Icon: "/static/images/alist.png", Enabled: true, CreatedAt: time.Now().UnixMilli()},
		{ID: "immich", Name: "Immich", Description: "相册备份与 AI 检索", Port: 2283, Icon: "/static/images/immich.png", Enabled: true, CreatedAt: time.Now().UnixMilli()},
		{ID: "jellyfin", Name: "Jellyfin", Description: "媒体管理与播放", Port: 8096, Icon: "/static/images/jellyfin.jpg", Enabled: true, CreatedAt: time.Now().UnixMilli()},
		{ID: "comfyui", Name: "ComfyUI", Description: "AI 图像生成工作流", Port: 28000, Icon: "/static/images/comfyui.webp", Enabled: true, CreatedAt: time.Now().UnixMilli()},
		{ID: "rustdesk", Name: "RustDesk", Description: "开源远程桌面控制", Port: 0, Icon: "/static/images/rustdesk.png", Enabled: false, CreatedAt: time.Now().UnixMilli()},
		{ID: "sunshine", Name: "Sunshine", Description: "游戏串流服务端", Port: 0, Icon: "☀️", Enabled: false, CreatedAt: time.Now().UnixMilli()},
		{ID: "moonlight", Name: "Moonlight", Description: "游戏串流客户端", Port: 0, Icon: "🌙", Enabled: false, CreatedAt: time.Now().UnixMilli()},
	}

	saveServices(defaultServices)
}

// GetServices 获取服务列表
func GetServices(c *gin.Context) {
	services := loadServices()
	c.JSON(200, services)
}

// CreateService 创建服务
func CreateService(c *gin.Context) {
	var service ServiceCard
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	// 生成 ID 和时间戳
	service.ID = uuid.New().String()[:8]
	service.CreatedAt = time.Now().UnixMilli()
	service.UpdatedAt = service.CreatedAt
	service.Enabled = true

	services := loadServices()
	services = append(services, service)

	if err := saveServices(services); err != nil {
		c.JSON(500, gin.H{"error": "保存失败"})
		return
	}

	c.JSON(200, service)
}

// UpdateService 更新服务
func UpdateService(c *gin.Context) {
	id := c.Param("id")
	var updated ServiceCard
	if err := c.ShouldBindJSON(&updated); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	services := loadServices()
	found := false
	for i, s := range services {
		if s.ID == id {
			updated.ID = id
			updated.CreatedAt = s.CreatedAt
			updated.UpdatedAt = time.Now().UnixMilli()
			services[i] = updated
			found = true
			break
		}
	}

	if !found {
		c.JSON(404, gin.H{"error": "服务不存在"})
		return
	}

	if err := saveServices(services); err != nil {
		c.JSON(500, gin.H{"error": "保存失败"})
		return
	}

	c.JSON(200, updated)
}

// DeleteService 删除服务
func DeleteService(c *gin.Context) {
	id := c.Param("id")
	services := loadServices()
	newServices := make([]ServiceCard, 0)
	found := false

	for _, s := range services {
		if s.ID == id {
			found = true
		} else {
			newServices = append(newServices, s)
		}
	}

	if !found {
		c.JSON(404, gin.H{"error": "服务不存在"})
		return
	}

	if err := saveServices(newServices); err != nil {
		c.JSON(500, gin.H{"error": "保存失败"})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// ImportServiceTemplate 导入推荐模板
func ImportServiceTemplate(c *gin.Context) {
	services := loadServices()
	now := time.Now().UnixMilli()

	for _, tmpl := range defaultServiceTemplates {
		// 检查是否已存在同名服务
		exists := false
		for _, s := range services {
			if s.ID == tmpl.ID || s.Name == tmpl.Name {
				exists = true
				break
			}
		}
		if !exists {
			newService := tmpl
			newService.CreatedAt = now
			newService.UpdatedAt = now
			services = append(services, newService)
		}
	}

	if err := saveServices(services); err != nil {
		c.JSON(500, gin.H{"error": "保存失败"})
		return
	}

	c.JSON(200, gin.H{"success": true, "count": len(services)})
}

// PingAllServices 批量检测所有服务连通性
func PingAllServices(c *gin.Context) {
	services := loadServices()
	settings := loadSettings()
	serverIP := settings.ServerIP
	if serverIP == "" {
		serverIP = "localhost"
	}

	results := make([]PingResult, 0, len(services))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, s := range services {
		if !s.Enabled || s.Port == 0 {
			continue
		}
		wg.Add(1)
		go func(service ServiceCard) {
			defer wg.Done()
			result := pingService(service.ID, serverIP, service.Port)
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(s)
	}

	wg.Wait()
	c.JSON(200, results)
}

// PingService 检测单个服务连通性
func PingService(c *gin.Context) {
	id := c.Param("id")
	services := loadServices()
	settings := loadSettings()
	serverIP := settings.ServerIP
	if serverIP == "" {
		serverIP = "localhost"
	}

	var targetService *ServiceCard
	for _, s := range services {
		if s.ID == id {
			targetService = &s
			break
		}
	}

	if targetService == nil {
		c.JSON(404, gin.H{"error": "服务不存在"})
		return
	}

	if !targetService.Enabled || targetService.Port == 0 {
		c.JSON(200, PingResult{
			ID:      id,
			Status:  "disabled",
			Latency: 0,
			Message: "服务未启用或无端口",
		})
		return
	}

	result := pingService(id, serverIP, targetService.Port)
	c.JSON(200, result)
}

// pingService 检测服务连通性
func pingService(id, host string, port int) PingResult {
	result := PingResult{
		ID:     id,
		Status: "error",
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	start := time.Now()

	// 尝试 TCP 连接
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	latency := time.Since(start).Milliseconds()
	result.Latency = latency

	if err != nil {
		result.Status = "error"
		result.Message = err.Error()
		return result
	}
	conn.Close()

	// 根据延迟判断状态
	if latency < 200 {
		result.Status = "ok"
	} else if latency < 1000 {
		result.Status = "slow"
	} else {
		result.Status = "error"
	}

	return result
}
