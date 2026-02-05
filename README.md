# 🖥️ HomeDash Win V0.3.0

> 轻量级 Windows 家庭服务器看板

HomeDash Win 是一个专为 Windows 家庭服务器设计的综合看板工具，提供**服务入口管理**、**实时系统监控**、**文件管理**、**Web 终端**和 **Docker 管理**等功能，让你轻松管理家庭服务器上的所有服务。
![HomeDash 界面预览](assets/index.png)


---

## ✨ 功能特性

### 核心功能

| 功能 | 描述 |
|------|------|
| 🎯 **服务入口管理** | 自由添加、编辑、删除服务卡片，支持一键导入推荐模板 |
| 🔍 **连通性检测** | 实时检测服务状态（绿色=正常，黄色=延迟高，红色=不可用） |
| 🖼️ **Favicon 自动抓取** | 输入端口后自动获取目标服务的网站图标 |
| 🎨 **图标自定义** | 支持上传图片、拖拽上传、Emoji 选择，自定义服务图标 |
| ▶️ **服务启动/停止** | 一键启动和停止服务，支持启动命令和进程名配置 |
| 🔄 **进程状态检测** | 自动检测服务进程运行状态，实时显示启动/停止状态 |
| 🚀 **服务开机自启** | 为每个服务配置开机自启，系统启动时自动运行 |
| 📊 **实时系统监控** | WebSocket 实时推送 CPU、内存、GPU、磁盘使用情况 |
| 📈 **顶部状态栏** | 实时显示网页延迟、CPU、内存、GPU、网络流量等关键指标 |
| 🌡️ **温度监控** | 实时显示 CPU 和 GPU 温度，高温预警 |
| ⚙️ **进程管理** | 查看系统进程列表，按 CPU/内存占用排序（Top 20） |
| 📁 **文件管理** | WebDAV 服务端 + 可视化文件管理器，支持浏览、上传、下载、删除文件 |
| 💻 **Web SSH 终端** | 集成式 Web 终端，类似 Xshell 体验，支持命令历史、快捷键 |
| 🐳 **Docker 管理** | 查看 Docker 容器列表和状态，支持镜像查看 |
| 🎨 **AI绘画** | 集成 ComfyUI，支持工作流执行和图像生成 |
| 🔧 **WebDAV 配置** | 可自定义 WebDAV 挂载目录，支持通过 WebDAV 协议访问文件 |
| ⚙️ **应用设置** | 端口配置、应用开机自启、一键重启面板 |
| 🎨 **主题切换** | 支持深色/浅色主题切换，所有页面自动适配 |
| 🖼️ **背景设置** | 预设背景选择，支持自定义背景图片 |

---

## 📸 界面预览

![HomeDash 界面预览](assets/monitor.png)

![HomeDash 界面预览](assets/ssh.png)
---

## 🚀 快速开始

在release中下载最新版本exe直接运行即可

```powershell
# 克隆项目
git clone https://github.com/flyingrtx2333/HomeDash-Win.git
cd HomeDash-Win

# 安装依赖
go mod tidy

# 运行
go run ./cmd/homepage

# 或编译后运行
go build -o homedash.exe ./cmd/homepage
./homedash.exe
```

### 访问面板

打开浏览器访问 `http://localhost:29678`

默认端口 `29678`，可通过环境变量修改：

```powershell
$env:PORT="8080"; ./homedash.exe
```

## ⚙️ 配置说明

### 连通性检测规则

| 状态 | 延迟 | 显示 |
|------|------|------|
| ✓ 正常 | < 200ms | 绿色 |
| ⚠ 延迟 | 200-1000ms | 黄色 |
| ✗ 错误 | > 1000ms 或连接失败 | 红色 |

### 服务配置 (services.json)

```json
[
  {
    "id": "alist",
    "name": "Alist",
    "description": "多网盘整合",
    "port": 5244,
    "icon": "/static/icons/abc123.ico",
    "enabled": true,
    "launchCommand": "C:\\Program Files\\Alist\\alist.exe server --data \"C:\\Alist\"",
    "processName": "alist.exe",
    "autoStart": false
  }
]
```

**字段说明**：
- `launchCommand`: 启动命令（完整路径和参数）
- `processName`: 进程名（用于进程检测和停止）
- `autoStart`: 是否开机自启
- `port`: 端口号（0 表示本地应用，不通过 HTTP 访问）

### 用户设置 (settings.json)

```json
{
  "serverIp": "192.168.1.100",
  "backgroundUrl": "/static/backgrounds/mountain.jpg",
  "theme": "dark",
  "webdavRoot": "C:\\Users\\Public"
}
```

### WebDAV 配置

WebDAV 服务默认挂载到用户主目录，可通过以下方式配置：

1. **通过 Web 界面**：在「文件管理」页面顶部设置挂载目录
2. **通过环境变量**：设置 `WEBDAV_ROOT` 环境变量
3. **通过设置文件**：在 `settings.json` 中设置 `webdavRoot` 字段

WebDAV 访问地址：`http://localhost:29678/webdav/`

支持所有标准 WebDAV 客户端（Windows 资源管理器、RaiDrive、rclone 等）。

---

## 🔧 开机自启

### 应用开机自启

在「设置」页面启用「开机自启」，程序将添加到注册表里,系统启动时会自动运行 HomeDash 应用。

### 服务开机自启

在服务编辑弹窗中，启用「开机自启」选项。系统启动时，HomeDash 会自动运行配置了开机自启的服务。

**注意**：服务开机自启需要先配置「启动命令」和「进程名」。

---

## 💡 使用技巧

### WebDAV 挂载

1. 在「文件管理」页面设置挂载目录
2. 复制 WebDAV 地址（格式：`http://服务器IP:29678/webdav/`）
3. 在 Windows 资源管理器中：
   - 右键「此电脑」→「添加网络位置」
   - 输入 WebDAV 地址
   - 完成挂载

### 主题切换

点击右下角设置按钮，可切换深色/浅色主题，所有页面自动适配。

### 服务启动配置

1. 在服务编辑弹窗中，展开「高级选项」
2. 配置「启动命令」：完整的可执行文件路径和参数（例如：`C:\Program Files\Alist\alist.exe server --data "C:\Alist"`）
3. 配置「进程名」：用于进程检测（例如：`alist.exe`）
4. 启用「开机自启」：系统启动时自动运行该服务
5. 保存后，服务卡片会显示「启动」或「停止」按钮

### AI绘画（ComfyUI）

1. 在「AI绘画」页面，点击右上角「⚙️」按钮配置 ComfyUI 服务器地址
2. 配置完成后，点击「🔗」测试连接
3. 选择工作流卡片，点击「执行」按钮
4. 填写工作流参数（提示词、采样步数、图像尺寸等）
5. 执行后可在弹窗中查看进度和生成结果

### 图标自定义

- **上传图片**：拖拽图片到上传区域或点击上传，支持 PNG、JPG、GIF、WebP、ICO（最大 2MB）
- **Emoji 选择**：从预设的 Emoji 图标中选择
- **图片路径**：直接输入图片路径（例如：`/static/images/xxx.png`）

---

## 📚 推荐搭配

以下是一些适合家庭服务器的优秀开源项目：

| 项目 | 用途 | 链接 |
|------|------|------|
| Lucky | DDNS + 反向代理 | https://lucky666.cn |
| Alist | 网盘聚合 | https://alist.nn.ci |
| Jellyfin | 媒体服务器 | https://jellyfin.org |
| Immich | 照片备份 | https://immich.app |
| Sunshine | 游戏串流 | https://github.com/LizardByte/Sunshine |

---

## 🛠️ 技术栈

- **后端**：Go 1.22+ (Gin, Gorilla WebSocket, golang.org/x/net/webdav)
- **前端**：原生 HTML/CSS/JavaScript (SPA)
- **系统监控**：gopsutil (CPU、内存、磁盘、网络)
- **GPU 监控**：nvidia-smi (NVIDIA 显卡)
- **Docker**：Docker CLI

---

## TODO
 - 下载功能
 - 日志功能
 - github.com/docker/docker/client
 

## 📜 许可证

[MIT License](LICENSE)

---

<p align="center">
  <sub>Made with ❤️ by <a href="https://github.com/flyingrtx2333">@flyingrtx2333</a> for home server enthusiasts</sub>
</p>
