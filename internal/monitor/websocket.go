package monitor

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Hub 管理所有 WebSocket 连接
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan SystemStats
	register   chan *Client
	unregister chan *Client
	collector  *Collector
	mu         sync.RWMutex
}

// Client WebSocket 客户端
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan SystemStats
}

// NewHub 创建新的 Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan SystemStats),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		collector:  NewCollector(),
	}
}

// Run 运行 Hub
func (h *Hub) Run() {
	// 启动数据采集协程
	go h.collectLoop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("客户端连接，当前连接数: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("客户端断开，当前连接数: %d", len(h.clients))

		case stats := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- stats:
				default:
					// 发送失败，关闭连接
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// collectLoop 定时采集系统信息
func (h *Hub) collectLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.RLock()
		clientCount := len(h.clients)
		h.mu.RUnlock()

		// 只有在有客户端连接时才采集
		if clientCount > 0 {
			stats := h.collector.Collect()
			h.broadcast <- stats
		}
	}
}

// HandleWebSocket 处理 WebSocket 连接
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan SystemStats, 10),
	}

	h.register <- client

	// 启动写协程
	go client.writePump()

	// 启动读协程（处理客户端消息和检测断开）
	go client.readPump()

	// 立即发送一次数据
	stats := h.collector.Collect()
	client.send <- stats
}

// writePump 向客户端发送数据
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for stats := range c.send {
		err := c.conn.WriteJSON(stats)
		if err != nil {
			log.Printf("发送数据失败: %v", err)
			return
		}
	}
}

// readPump 读取客户端消息
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	// 设置读取超时
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket 错误: %v", err)
			}
			break
		}
	}
}

// GetClientCount 获取当前连接数
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
