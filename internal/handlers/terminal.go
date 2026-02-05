package handlers

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var termUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// HandleTerminalWebSocket 处理终端WebSocket连接
func HandleTerminalWebSocket(c *gin.Context) {
	conn, err := termUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 确定使用的 shell
	var shell string
	var shellArgs []string
	if runtime.GOOS == "windows" {
		shell = "powershell.exe"
		shellArgs = []string{"-NoLogo", "-NoProfile", "-Command", "-"}
	} else {
		shell = "/bin/bash"
		shellArgs = []string{}
	}

	// 发送欢迎消息
	welcomeMsg := fmt.Sprintf("HomeDash Terminal - 连接到: %s", shell)
	conn.WriteMessage(websocket.TextMessage, []byte(welcomeMsg))

	// 处理命令
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// 连接关闭
			}
			break
		}

		cmdStr := strings.TrimSpace(string(message))
		if cmdStr == "" {
			continue
		}

		// 执行命令
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-Command", cmdStr)
		} else {
			cmd = exec.Command(shell, append(shellArgs, "-c", cmdStr)...)
		}
		cmd.Dir = webdavRoot

		// 合并 stdout 和 stderr
		output, err := cmd.CombinedOutput()
		if err != nil {
			// 如果有输出，先发送输出
			if len(output) > 0 {
				// 按行发送，过滤空行
				lines := strings.Split(string(output), "\n")
				for _, line := range lines {
					line = strings.TrimRight(line, "\r\n")
					if line != "" {
						conn.WriteMessage(websocket.TextMessage, []byte("\x1b[31m"+line+"\x1b[0m"))
					}
				}
			} else {
				conn.WriteMessage(websocket.TextMessage, []byte("\x1b[31m执行失败: "+err.Error()+"\x1b[0m"))
			}
			continue
		}

		// 按行发送输出，过滤空行
		if len(output) > 0 {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimRight(line, "\r\n")
				if line != "" {
					conn.WriteMessage(websocket.TextMessage, []byte(line))
				}
			}
		}
	}
}
