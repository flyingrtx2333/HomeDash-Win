//go:build windows

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

var (
	shell32      = syscall.NewLazyDLL("shell32.dll")
	shellExecute = shell32.NewProc("ShellExecuteW")
)

func shellOpen(url string) {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, _ := syscall.UTF16PtrFromString(url)
	shellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
}

// pendingUpdate 用于缓存窗口就绪前的状态更新
type pendingUpdate struct {
	port string
}

// App 是整个 GUI 应用的主控制器
type App struct {
	mainWindow  *walk.MainWindow
	logBox      *walk.TextEdit
	statusDot   *walk.Label
	addrLabel   *walk.LinkLabel
	logWriter   *guiLogWriter
	mu          sync.Mutex
	trayIcon    *walk.NotifyIcon
	pendingPort string // 窗口就绪前缓存的端口
	windowReady bool
	earlyLogs   []string // 窗口就绪前缓存的日志
}

func NewApp() *App {
	a := &App{}
	a.logWriter = &guiLogWriter{app: a}
	return a
}

func (a *App) LogWriter() io.Writer {
	// -H windowsgui 编译后 os.Stderr 是无效句柄
	// 开发时（go run）可以同时输出到 stderr，但要检测句柄有效性
	if isStderrValid() {
		return io.MultiWriter(os.Stderr, a.logWriter)
	}
	return a.logWriter
}

// isStderrValid 检测 stderr 是否可用（windowsgui 模式下不可用）
func isStderrValid() bool {
	// 尝试获取 stderr 的文件信息，无效句柄会返回错误
	_, err := os.Stderr.Stat()
	return err == nil
}

// SetRunning 由后台 goroutine 调用，通知服务已启动
func (a *App) SetRunning(running bool, port string) {
	if !running {
		return
	}
	a.mu.Lock()
	ready := a.windowReady
	if !ready {
		a.pendingPort = port // 窗口还没就绪，先缓存
	}
	a.mu.Unlock()

	if ready && a.mainWindow != nil {
		a.applyRunningState(port)
	}
}

// applyRunningState 必须在 UI 线程执行（通过 Synchronize）
func (a *App) applyRunningState(port string) {
	a.mainWindow.Synchronize(func() {
		addr := fmt.Sprintf("http://127.0.0.1:%s", port)
		if a.addrLabel != nil {
			a.addrLabel.SetText(fmt.Sprintf(`<a href="%s">%s</a>`, addr, addr))
		}
		if a.statusDot != nil {
			a.statusDot.SetText("● 运行中")
		}
		// 同步更新托盘提示
		if a.trayIcon != nil {
			a.trayIcon.SetToolTip(fmt.Sprintf("HomeDash Win · %s", addr))
		}
	})
}

// appendLog 线程安全地追加日志，修正 Windows 换行符
func (a *App) appendLog(text string) {
	a.mu.Lock()
	ready := a.windowReady
	if !ready {
		// 窗口还没就绪，先缓存
		a.earlyLogs = append(a.earlyLogs, text)
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()

	if a.mainWindow == nil || a.logBox == nil {
		return
	}

	// 修正换行：将 \n 替换为 \r\n（Windows RichEdit 要求）
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n", "\r\n")

	a.mainWindow.Synchronize(func() {
		// 用 EM_SETSEL + EM_REPLACESEL 追加，比 SetText 高效且不会重排
		l := len(a.logBox.Text())
		a.logBox.SendMessage(0x00B1 /*EM_SETSEL*/, uintptr(l), uintptr(l))
		// EM_REPLACESEL: wParam=0(可撤销), lParam=字符串指针
		ptr, _ := syscall.UTF16PtrFromString(text)
		a.logBox.SendMessage(0x00C2 /*EM_REPLACESEL*/, 0, uintptr(unsafe.Pointer(ptr)))
		// 滚动到底部
		a.logBox.SendMessage(0x00B7 /*EM_SCROLLCARET*/, 0, 0)
	})
}

// Run 创建窗口并启动消息循环（阻塞，必须在主线程调用）
func (a *App) Run() {
	var logBox *walk.TextEdit
	var addrLabel *walk.LinkLabel
	var statusDot *walk.Label
	var mw *walk.MainWindow

	icon := loadIcon()

	// 配色方案：深色 Mica 风格
	// 背景层次：深蓝黑 #0D1117 → #161B22 → #21262D
	// 强调色：电光蓝 #58A6FF，成功绿 #3FB950，警告黄 #D29922

	bgDeep := walk.RGB(13, 17, 23)       // #0D1117 主背景
	bgPanel := walk.RGB(22, 27, 34)      // #161B22 面板背景
	bgInput := walk.RGB(1, 4, 9)         // #010409 日志背景（近纯黑）
	textMuted := walk.RGB(110, 118, 129) // #6E7681 次要文字
	accentBlue := walk.RGB(88, 166, 255) // #58A6FF 强调蓝
	textLog := walk.RGB(201, 209, 217)   // #C9D1D9 日志文字

	_ = textMuted
	_ = bgDeep

	err := MainWindow{
		AssignTo:   &mw,
		Title:      "HomeDash Win",
		Icon:       icon,
		MinSize:    Size{Width: 700, Height: 500},
		Size:       Size{Width: 860, Height: 600},
		Layout:     VBox{Margins: Margins{Left: 0, Top: 0, Right: 0, Bottom: 0}, Spacing: 0},
		Background: SolidColorBrush{Color: walk.RGB(13, 17, 23)},
		Children: []Widget{

			// ── 顶部标题栏 ──────────────────────────────────────────
			Composite{
				Layout:     HBox{Margins: Margins{Left: 20, Top: 14, Right: 20, Bottom: 14}, Spacing: 12},
				Background: SolidColorBrush{Color: bgPanel},
				Children: []Widget{
					// 左侧：图标 + 标题
					Label{
						Text:      "⬡",
						Font:      Font{Family: "Segoe UI", PointSize: 14},
						TextColor: accentBlue,
					},
					Label{
						Text:      "HomeDash Win",
						Font:      Font{Family: "Segoe UI Semibold", PointSize: 12},
						TextColor: walk.RGB(201, 209, 217),
					},
					Label{
						Text:      "v1.0",
						Font:      Font{Family: "Segoe UI", PointSize: 9},
						TextColor: walk.RGB(110, 118, 129),
					},
					HSpacer{},
					// 右侧：状态指示
					Label{
						AssignTo:  &statusDot,
						Text:      "◌  初始化中",
						Font:      Font{Family: "Segoe UI", PointSize: 9},
						TextColor: walk.RGB(210, 153, 34),
					},
				},
			},

			// ── 地址信息栏 ──────────────────────────────────────────
			Composite{
				Layout:     HBox{Margins: Margins{Left: 20, Top: 10, Right: 20, Bottom: 10}, Spacing: 8},
				Background: SolidColorBrush{Color: walk.RGB(13, 17, 23)},
				Children: []Widget{
					Label{
						Text:      "服务地址",
						Font:      Font{Family: "Segoe UI", PointSize: 9},
						TextColor: walk.RGB(110, 118, 129),
					},
					Label{
						Text:      "│",
						Font:      Font{Family: "Segoe UI", PointSize: 9},
						TextColor: walk.RGB(48, 54, 61),
					},
					LinkLabel{
						AssignTo: &addrLabel,
						Text:     "等待服务启动...",
						Font:     Font{Family: "Consolas", PointSize: 9},
						OnLinkActivated: func(link *walk.LinkLabelLink) {
							shellOpen(link.URL())
						},
					},
					HSpacer{},
				},
			},

			// ── 分隔线 ──────────────────────────────────────────────
			Composite{
				MaxSize:    Size{Height: 1},
				MinSize:    Size{Height: 1},
				Background: SolidColorBrush{Color: walk.RGB(33, 38, 45)},
				Layout:     HBox{MarginsZero: true},
			},

			// ── 日志区标题 ──────────────────────────────────────────
			Composite{
				Layout:     HBox{Margins: Margins{Left: 20, Top: 8, Right: 20, Bottom: 4}, Spacing: 8},
				Background: SolidColorBrush{Color: walk.RGB(13, 17, 23)},
				Children: []Widget{
					Label{
						Text:      "▸  控制台输出",
						Font:      Font{Family: "Segoe UI Semibold", PointSize: 9},
						TextColor: walk.RGB(88, 166, 255),
					},
					HSpacer{},
					PushButton{
						Text:    "清空",
						MaxSize: Size{Width: 52, Height: 24},
						MinSize: Size{Width: 52, Height: 24},
						Font:    Font{Family: "Segoe UI", PointSize: 8},
						OnClicked: func() {
							if logBox != nil {
								logBox.SetText("")
							}
						},
					},
				},
			},

			// ── 日志文本框 ──────────────────────────────────────────
			TextEdit{
				AssignTo:   &logBox,
				ReadOnly:   true,
				VScroll:    true,
				HScroll:    false,
				Font:       Font{Family: "Cascadia Code", PointSize: 9},
				TextColor:  textLog,
				Background: SolidColorBrush{Color: bgInput},
				MaxLength:  2 << 20,
			},

			// ── 底部工具栏 ──────────────────────────────────────────
			Composite{
				Layout:     HBox{Margins: Margins{Left: 20, Top: 10, Right: 20, Bottom: 12}, Spacing: 8},
				Background: SolidColorBrush{Color: bgPanel},
				Children: []Widget{
					Label{
						Text:      "最小化不会停止服务",
						Font:      Font{Family: "Segoe UI", PointSize: 8},
						TextColor: walk.RGB(110, 118, 129),
					},
					HSpacer{},
					PushButton{
						Text:    "最小化到托盘",
						MinSize: Size{Width: 100, Height: 28},
						Font:    Font{Family: "Segoe UI", PointSize: 9},
						OnClicked: func() {
							mw.Hide()
						},
					},
					PushButton{
						Text:    "退出",
						MinSize: Size{Width: 60, Height: 28},
						Font:    Font{Family: "Segoe UI", PointSize: 9},
						OnClicked: func() {
							if a.trayIcon != nil {
								a.trayIcon.SetVisible(false)
							}
							walk.App().Exit(0)
						},
					},
				},
			},
		},
	}.Create()

	if err != nil {
		panic(err)
	}

	a.mainWindow = mw
	a.logBox = logBox
	a.addrLabel = addrLabel
	a.statusDot = statusDot

	// 设置托盘
	a.setupTray(icon)

	// 标记窗口就绪，并处理启动期间缓存的状态和日志
	a.mu.Lock()
	a.windowReady = true
	cached := a.pendingPort
	earlyLogs := a.earlyLogs
	a.earlyLogs = nil // 清空缓存
	a.mu.Unlock()

	// 写入缓存的早期日志
	if len(earlyLogs) > 0 {
		for _, logText := range earlyLogs {
			a.appendLog(logText)
		}
	}

	if cached != "" {
		a.applyRunningState(cached)
	}

	// 关闭按钮 → 最小化到托盘
	mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		*canceled = true
		mw.Hide()
	})

	mw.Run()
}

func (a *App) setupTray(icon *walk.Icon) {
	ni, err := walk.NewNotifyIcon(a.mainWindow)
	if err != nil {
		return
	}
	a.trayIcon = ni
	ni.SetIcon(icon)
	ni.SetToolTip("HomeDash Win")
	ni.SetVisible(true)

	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			a.showWindow()
		}
	})

	showAction := walk.NewAction()
	showAction.SetText("📂  显示主窗口")
	showAction.Triggered().Attach(func() { a.showWindow() })

	sep := walk.NewSeparatorAction()

	quitAction := walk.NewAction()
	quitAction.SetText("✕  退出 HomeDash")
	quitAction.Triggered().Attach(func() {
		ni.SetVisible(false)
		walk.App().Exit(0)
	})

	ni.ContextMenu().Actions().Add(showAction)
	ni.ContextMenu().Actions().Add(sep)
	ni.ContextMenu().Actions().Add(quitAction)
}

func (a *App) showWindow() {
	if a.mainWindow == nil {
		return
	}
	a.mainWindow.Show()
	a.mainWindow.SetFocus()
	a.mainWindow.SendMessage(0x0112, 0xF120, 0) // WM_SYSCOMMAND SC_RESTORE
}

// guiLogWriter 实现 io.Writer
type guiLogWriter struct {
	app *App
}

func (w *guiLogWriter) Write(p []byte) (n int, err error) {
	w.app.appendLog(string(p))
	return len(p), nil
}
