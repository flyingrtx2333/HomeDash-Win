package handlers

// BackgroundInfo 背景图信息
type BackgroundInfo struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Thumb string `json:"thumb"`
}

// UserSettings 用户设置
type UserSettings struct {
	ServerIP         string `json:"serverIp"`
	BackgroundURL    string `json:"backgroundUrl"`
	Theme            string `json:"theme"`            // "dark" | "light"
	WebdavRoot       string `json:"webdavRoot"`       // WebDAV 挂载根目录
	ComfyUIServerURL string `json:"comfyuiServerUrl"` // ComfyUI服务器地址
}

// ServiceCard 服务卡片
type ServiceCard struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Port          int    `json:"port"`
	Icon          string `json:"icon"`
	Enabled       bool   `json:"enabled"`
	LaunchPath    string `json:"launchPath"`    // 启动路径（可执行文件路径，向后兼容）
	LaunchCommand string `json:"launchCommand"` // 启动命令（支持参数）
	ProcessName   string `json:"processName"`   // 进程名（用于检测和停止）
	AutoStart     bool   `json:"autoStart"`     // 是否开机自启
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}

// AppConfig 应用配置
type AppConfig struct {
	Port      string `json:"port"`      // 应用端口
	AutoStart bool   `json:"autoStart"` // 是否开机自启
}

// PingResult 连通性检测结果
type PingResult struct {
	ID      string `json:"id"`
	Status  string `json:"status"`  // "ok" | "slow" | "error"
	Latency int64  `json:"latency"` // 毫秒
	Message string `json:"message,omitempty"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

// DockerContainer Docker 容器信息
type DockerContainer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Ports   string `json:"ports"`
	Created string `json:"created"`
}

// DockerImage Docker 镜像信息
type DockerImage struct {
	ID      string `json:"id"`
	Repo    string `json:"repo"`
	Tag     string `json:"tag"`
	Size    string `json:"size"`
	Created string `json:"created"`
}

// ProcessStatus 进程状态
type ProcessStatus struct {
	Running bool  `json:"running"`
	PID     int32 `json:"pid"`
}
