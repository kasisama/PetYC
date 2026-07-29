package core

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"qq-pet-saas/config"
	"qq-pet-saas/security"
)

const (
	maxWebSocketMessageSize = 1 << 20
	writeWait               = 10 * time.Second
	pongWait                = 60 * time.Second
	maxPendingMessages      = 128
	maxConcurrentHandlers   = 64
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type websocketClient struct {
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
}

var (
	ActiveConnections = make(map[*websocket.Conn]*websocketClient)
	connMu            sync.RWMutex
	messageHandlers   = make(chan struct{}, maxConcurrentHandlers)
)

// BroadcastGroupMessage queues a message for every currently connected client.
func BroadcastGroupMessage(groupID int64, text string) {
	payload, err := json.Marshal(OneBotAction{
		Action: "send_group_msg",
		Params: GroupMsgParams{GroupID: groupID, Message: text},
	})
	if err != nil {
		log.Printf("[WebSocket] 序列化群消息失败: %v", err)
		return
	}

	connMu.RLock()
	clients := make([]*websocketClient, 0, len(ActiveConnections))
	for _, client := range ActiveConnections {
		clients = append(clients, client)
	}
	connMu.RUnlock()
	for _, client := range clients {
		enqueueMessage(client, payload)
	}
}

// SendGroupMessage queues a group message without blocking other connections.
func SendGroupMessage(conn *websocket.Conn, groupID int64, text string) {
	payload, err := json.Marshal(OneBotAction{
		Action: "send_group_msg",
		Params: GroupMsgParams{GroupID: groupID, Message: text},
	})
	if err != nil {
		log.Printf("[WebSocket] 序列化群消息失败: %v", err)
		return
	}
	enqueueForConnection(conn, payload)
}

// SendPrivateMessage queues a private message without blocking other connections.
func SendPrivateMessage(conn *websocket.Conn, userID int64, text string) {
	payload, err := json.Marshal(OneBotAction{
		Action: "send_private_msg",
		Params: map[string]interface{}{"user_id": userID, "message": text},
	})
	if err != nil {
		log.Printf("[WebSocket] 序列化私聊消息失败: %v", err)
		return
	}
	enqueueForConnection(conn, payload)
}

func enqueueForConnection(conn *websocket.Conn, payload []byte) {
	connMu.RLock()
	client := ActiveConnections[conn]
	connMu.RUnlock()
	if client != nil {
		enqueueMessage(client, payload)
	}
}

func enqueueMessage(client *websocketClient, payload []byte) {
	select {
	case <-client.done:
		return
	case client.send <- payload:
		return
	default:
		log.Printf("[WebSocket] 连接发送队列已满，丢弃消息")
	}
}

func (client *websocketClient) writePump() {
	for {
		select {
		case <-client.done:
			return
		case payload := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				log.Printf("[WebSocket] 发送消息失败: %v", err)
				return
			}
		}
	}
}

// HandleWebSocket accepts authenticated OneBot reverse WebSocket clients.
func HandleWebSocket(c *gin.Context) {
	credentials, err := security.LoadCredentials()
	if err != nil {
		log.Printf("[WebSocket] 加载连接凭据失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器凭据不可用"})
		return
	}
	if !validWebSocketToken(extractWebSocketToken(c), credentials.WebSocketToken) {
		log.Printf("[WebSocket] 鉴权失败")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "鉴权失败，Token无效"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WebSocket] 升级失败: %v", err)
		return
	}
	client := &websocketClient{conn: conn, send: make(chan []byte, maxPendingMessages), done: make(chan struct{})}
	connMu.Lock()
	ActiveConnections[conn] = client
	connMu.Unlock()
	defer func() {
		connMu.Lock()
		delete(ActiveConnections, conn)
		connMu.Unlock()
		close(client.done)
		_ = conn.Close()
	}()

	go client.writePump()
	conn.SetReadLimit(maxWebSocketMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	log.Printf("[WebSocket] 机器人已连接")

	for {
		messageType, message, readErr := conn.ReadMessage()
		if readErr != nil {
			log.Printf("[WebSocket] 连接断开或读取出错: %v", readErr)
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		select {
		case messageHandlers <- struct{}{}:
			go func(payload []byte) {
				defer func() { <-messageHandlers }()
				processIncomingMessage(conn, payload)
			}(message)
		default:
			log.Printf("[WebSocket] 消息处理队列已满，丢弃消息")
		}
	}
}

func extractWebSocketToken(c *gin.Context) string {
	if token := c.Query("token"); token != "" {
		return token
	}
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	if strings.HasPrefix(header, "Token ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Token "))
	}
	return header
}

func validWebSocketToken(provided, expected string) bool {
	return expected != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func processIncomingMessage(conn *websocket.Conn, rawPayload []byte) {
	var event OneBotEvent
	if err := json.Unmarshal(rawPayload, &event); err != nil {
		return
	}
	if event.PostType != "message" || event.MessageType != "group" {
		return
	}
	config.LockForRead()
	defer config.UnlockForRead()
	RouteMessage(conn, &event)
}
