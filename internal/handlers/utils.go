package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var (
	webDir       string
	settingsFile string
	servicesFile string
	settingsMu   sync.RWMutex
	servicesMu   sync.RWMutex
	webdavRoot   string // WebDAV 根目录
)

// InitHandlers 初始化处理器全局变量
func InitHandlers(wd, sf, svf, wr string) {
	webDir = wd
	settingsFile = sf
	servicesFile = svf
	webdavRoot = wr
}

// GetWebDir 获取web目录
func GetWebDir() string {
	return webDir
}


// SetWebdavRoot 设置WebDAV根目录
func SetWebdavRoot(root string) {
	webdavRoot = root
}

// loadServices 加载服务列表
func loadServices() []ServiceCard {
	servicesMu.RLock()
	defer servicesMu.RUnlock()

	var services []ServiceCard
	data, err := os.ReadFile(servicesFile)
	if err != nil {
		return services
	}

	json.Unmarshal(data, &services)
	return services
}

// saveServices 保存服务列表
func saveServices(services []ServiceCard) error {
	servicesMu.Lock()
	defer servicesMu.Unlock()

	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(servicesFile, data, 0644)
}

// LoadSettings 从文件加载用户设置（导出供外部使用）
func LoadSettings() UserSettings {
	return loadSettings()
}

// loadSettings 从文件加载用户设置
func loadSettings() UserSettings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()

	settings := UserSettings{
		ServerIP:      "localhost",
		BackgroundURL: "",
	}

	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return settings
	}

	json.Unmarshal(data, &settings)
	return settings
}

// saveSettings 保存用户设置到文件
func saveSettings(settings UserSettings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsFile, data, 0644)
}

// resolveWebDir 解析web目录路径
func resolveWebDir() string {
	candidates := []string{"web", filepath.Join("..", "web")}
	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			return dir
		}
	}
	return "web"
}
