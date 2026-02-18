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
	Theme            string `json:"theme"`            // "dark" | "light" | "fresh"
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
	Version   string `json:"version"`   // 当前应用版本
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

// PythonWebsite Python网站项目
type PythonWebsite struct {
	ID              string            `json:"id"`              // 项目ID
	Name            string            `json:"name"`            // 项目名称
	Path            string            `json:"path"`            // 项目路径（绝对路径）
	Domain          string            `json:"domain"`          // 域名（可选）
	Port            int               `json:"port"`            // 端口
	PythonPath      string            `json:"pythonPath"`      // Python可执行文件路径
	VenvPath        string            `json:"venvPath"`        // 虚拟环境路径
	Framework       string            `json:"framework"`       // 框架类型：flask/django/fastapi/custom
	StartCommand    string            `json:"startCommand"`    // 启动命令（如：flask run, python app.py等）
	WorkingDir      string            `json:"workingDir"`      // 工作目录（项目根目录）
	RequirementsTxt string            `json:"requirementsTxt"` // requirements.txt路径
	EnvironmentVars map[string]string `json:"environmentVars"` // 环境变量
	AutoStart       bool              `json:"autoStart"`       // 是否开机自启
	Enabled         bool              `json:"enabled"`         // 是否启用
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// PythonVersion Python版本信息
type PythonVersion struct {
	Version   string `json:"version"`   // Python版本号（如：3.11.0）
	Path      string `json:"path"`      // Python可执行文件路径
	IsDefault bool   `json:"isDefault"` // 是否为默认版本
}

// AppVersion 当前应用版本号（与 release 保持一致）
const AppVersion = "0.5.0"

// UpdateCheckResponse 更新检查接口返回（与更新服务器约定一致）
type UpdateCheckResponse struct {
	HasUpdate     bool   `json:"hasUpdate"`
	LatestVersion string `json:"latestVersion,omitempty"`
	DownloadURL   string `json:"downloadUrl,omitempty"`
	ReleaseNotes  string `json:"releaseNotes,omitempty"`
}

// Database 数据库配置（支持 MySQL、Redis、MongoDB 等）
type Database struct {
	ID         string `json:"id"`         // 数据库ID
	Name       string `json:"name"`       // 数据库名称
	Type       string `json:"type"`       // 数据库类型：mysql/redis/mongodb
	Host       string `json:"host"`       // 主机地址（默认 localhost）
	Port       int    `json:"port"`       // 端口（MySQL默认3306）
	Username   string `json:"username"`   // 用户名
	Password   string `json:"password"`   // 密码
	Note       string `json:"note"`       // 备注
	BackupDir  string `json:"backupDir"`  // 备份目录
	AutoBackup bool   `json:"autoBackup"` // 是否自动备份
	BackupCron string `json:"backupCron"` // 备份频率（cron表达式，如：0 2 * * * 表示每天凌晨2点）
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

// BackupInfo 备份文件信息
type BackupInfo struct {
	Filename string `json:"filename"` // 文件名
	Size     int64  `json:"size"`     // 文件大小（字节）
	ModTime  int64  `json:"modTime"`  // 修改时间（Unix时间戳）
}

// TableInfo 数据库表信息
type TableInfo struct {
	Name      string `json:"name"`      // 表名
	Rows      int64  `json:"rows"`      // 行数（近似值）
	DataSize  int64  `json:"dataSize"`  // 数据大小（字节）
	IndexSize int64  `json:"indexSize"` // 索引大小（字节）
	Engine    string `json:"engine"`    // 存储引擎
	Comment   string `json:"comment"`   // 表注释
}
