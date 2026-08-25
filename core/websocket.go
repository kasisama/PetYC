package core

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	lastOneBotMessage atomic.Int64
)

func OneBotStatusSnapshot() interface{} {
	connMu.RLock()
	connectionCount := len(ActiveConnections)
	connMu.RUnlock()
	lastUnix := lastOneBotMessage.Load()
	var lastMessageAt *time.Time
	if lastUnix > 0 {
		value := time.Unix(lastUnix, 0)
		lastMessageAt = &value
	}
	return map[string]interface{}{"configured": true, "connected": connectionCount > 0, "connection_count": connectionCount, "last_message_at": lastMessageAt}
}

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

// BroadcastNotification queues a proactive OneBot message on every active
// reverse WebSocket connection. Durable idempotency is handled by the outbox.
func BroadcastNotification(event InboundEvent, message OutboundMessage) error {
	var action string
	var params interface{}
	text := oneBotOutboundText(event, message)
	switch event.SceneType {
	case SceneGroup:
		groupID, err := strconv.ParseInt(strings.TrimSpace(event.SpaceID), 10, 64)
		if err != nil || groupID <= 0 {
			return errors.New("OneBot 群目标无效")
		}
		action = "send_group_msg"
		params = GroupMsgParams{GroupID: groupID, Message: text}
	case SceneDirect:
		userID, err := strconv.ParseInt(strings.TrimSpace(event.ActorID), 10, 64)
		if err != nil || userID <= 0 {
			return errors.New("OneBot 私聊目标无效")
		}
		action = "send_private_msg"
		params = map[string]interface{}{"user_id": userID, "message": text}
	default:
		return fmt.Errorf("OneBot 不支持通知场景: %s", event.SceneType)
	}
	payload, err := json.Marshal(OneBotAction{Action: action, Params: params})
	if err != nil {
		return err
	}
	connMu.RLock()
	clients := make([]*websocketClient, 0, len(ActiveConnections))
	for _, client := range ActiveConnections {
		clients = append(clients, client)
	}
	connMu.RUnlock()
	if len(clients) == 0 {
		return errors.New("OneBot 当前没有在线连接")
	}
	for _, client := range clients {
		enqueueMessage(client, payload)
	}
	return nil
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
	if event.PostType != "message" || (event.MessageType != "group" && event.MessageType != "private") {
		return
	}
	lastOneBotMessage.Store(time.Now().Unix())
	config.LockForRead()
	defer config.UnlockForRead()
	if event.MessageType == "group" && !GroupEnabled(InboundEvent{Platform: PlatformOneBot, SceneType: SceneGroup, SpaceID: strconv.FormatInt(event.GroupID, 10)}) {
		return
	}
	message, handled, err := routeOneBotMessage(context.Background(), event)
	if err != nil {
		log.Printf("[WebSocket] 统一命令处理失败: %v", err)
		return
	}
	if handled {
		outboundText := oneBotOutboundText(event.ToInbound("onebot"), message)
		if event.MessageType == "group" {
			SendGroupMessage(conn, event.GroupID, outboundText)
		} else {
			SendPrivateMessage(conn, event.UserID, outboundText)
		}
		return
	}
	if event.MessageType != "group" {
		return
	}
}

func oneBotOutboundText(event InboundEvent, message OutboundMessage) string {
	parts := make([]string, 0, 3)
	if event.SceneType == SceneGroup {
		if actorID, err := strconv.ParseInt(strings.TrimSpace(event.ActorID), 10, 64); err == nil && actorID > 0 {
			parts = append(parts, fmt.Sprintf("[CQ:at,qq=%d]", actorID))
		}
	}
	image := oneBotImageCQ(message.Image)
	if image != "" {
		parts = append(parts, image)
	}
	if strings.TrimSpace(message.Text) != "" {
		parts = append(parts, message.Text)
	}
	return strings.Join(parts, "\n")
}

func oneBotImageCQ(source string) string {
	source = ExistingImageSource(source)
	if source == "" {
		return ""
	}
	if parsed, err := url.Parse(source); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		return fmt.Sprintf("[CQ:image,file=%s]", source)
	}
	normalized := strings.TrimPrefix(strings.ReplaceAll(source, "\\", "/"), "./")
	normalized = strings.TrimPrefix(normalized, "图片/")
	clean := filepath.Clean(filepath.FromSlash(normalized))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return ""
	}
	if host := strings.TrimRight(strings.TrimSpace(config.Core.ImageHost), "/"); host != "" {
		segments := strings.Split(filepath.ToSlash(clean), "/")
		for index, segment := range segments {
			segments[index] = url.PathEscape(segment)
		}
		return fmt.Sprintf("[CQ:image,file=%s/images/%s]", host, strings.Join(segments, "/"))
	}
	for _, root := range []string{filepath.Join(config.GlobalConfigPath, "图片"), filepath.Join(".", "图片")} {
		absoluteRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		candidate, err := filepath.Abs(filepath.Join(absoluteRoot, clean))
		if err != nil {
			continue
		}
		relative, err := filepath.Rel(absoluteRoot, candidate)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(candidate)}).String()
			return fmt.Sprintf("[CQ:image,file=%s]", fileURL)
		}
	}
	return ""
}
