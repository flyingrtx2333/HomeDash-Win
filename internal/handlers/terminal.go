package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/aymanbagabas/go-pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var termUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境请改为严格检查 Origin
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// HandleTerminalWebSocket 处理终端 WebSocket 连接
func HandleTerminalWebSocket(c *gin.Context) {
	conn, err := termUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 创建 PTY
	ptmx, err := pty.New()
	if err != nil {
		errMsg := fmt.Sprintf("\x1b[31m启动 PTY 失败: %s\x1b[0m", err)
		conn.WriteMessage(websocket.TextMessage, []byte(errMsg))
		return
	}
	defer ptmx.Close()

	// 创建命令对象并绑定到 PTY
	var cmd *pty.Cmd
	powershellPath := `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	cmd = ptmx.Command(powershellPath, "-NoLogo", "-NoProfile") // 可选加 -NoProfile 加速启动

	// 设置工作目录（如果有）
	if webdavRoot != "" {
		cmd.Dir = webdavRoot
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		errMsg := fmt.Sprintf("\x1b[31m启动终端失败 (Windows 请确保 10+ 并启用 ConPTY): %s\x1b[0m", err)
		conn.WriteMessage(websocket.TextMessage, []byte(errMsg))
		return
	}

	// 设置初始窗口大小（推荐 80x24 或更大）
	_ = ptmx.Resize(80, 24) // cols, rows

	// 监听进程退出
	go func() {
		cmd.Wait()
		conn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[33m[进程已退出]\x1b[0m"))
		conn.Close()
	}()

	// 双向转发
	var wg sync.WaitGroup
	wg.Add(2)

	// PTY 输出 → WebSocket
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&wsWriter{conn: conn}, ptmx) // 推荐用 io.Copy 简化（需 wsWriter 实现 io.Writer）
	}()

	// WebSocket 输入 → PTY
	go func() {
		defer wg.Done()
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			if messageType == websocket.TextMessage {
				// 检查是否为 resize 消息
				var msg struct {
					Type string `json:"type"`
					Cols int    `json:"cols"`
					Rows int    `json:"rows"`
				}
				if json.Unmarshal(message, &msg) == nil && msg.Type == "resize" {
					if msg.Cols > 0 && msg.Rows > 0 {
						_ = ptmx.Resize(msg.Cols, msg.Rows) // 直接用 Resize(cols, rows)
					}
					continue
				}
			}

			// 普通输入
			_, err = ptmx.Write(message)
			if err != nil {
				break
			}
		}
		// WS 断开 → 杀进程
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	wg.Wait()
}

// wsWriter 是一个简单 wrapper，让 io.Copy 支持写到 WebSocket
type wsWriter struct {
	conn *websocket.Conn
}

func (w *wsWriter) Write(p []byte) (n int, err error) {
	err = w.conn.WriteMessage(websocket.TextMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
