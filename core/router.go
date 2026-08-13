package core

import (
	"sort"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
)

var (
	handlers   = make(map[string]MessageHandler)
	handlersMu sync.RWMutex
)

// RegisterHandler 注册指令和对应的业务处理器 (并发安全)
func RegisterHandler(command string, handler MessageHandler) {
	if command == "" {
		return
	}
	handlersMu.Lock()
	defer handlersMu.Unlock()
	handlers[command] = handler
}

// RouteMessage 将收到的 OneBot 事件路由分发给对应的处理器
func RouteMessage(conn *websocket.Conn, event *OneBotEvent) {
	// 如果是群消息，校验后台管理的群组开关状态
	if event.GroupID != 0 {
		var gSwitches []models.GroupSwitch
		err := database.DB.Where("group_id = ?", event.GroupID).Limit(1).Find(&gSwitches).Error
		if err == nil && len(gSwitches) > 0 {
			if !gSwitches[0].IsActive {
				// 管理员已在此群关闭了 QQ 宠物服务，直接静默忽略
				return
			}
		}
	}

	handlersMu.RLock()
	// 获取所有指令并按长度从大到小排序，防止短指令前缀误匹配长指令（如 "领养" 误匹配 "领养宠物"）
	var cmds []string
	for cmd := range handlers {
		cmds = append(cmds, cmd)
	}
	sort.Slice(cmds, func(i, j int) bool {
		return len(cmds[i]) > len(cmds[j])
	})

	var matchedHandler MessageHandler
	for _, cmd := range cmds {
		// 清理首尾空格后进行前缀匹配
		cleanMsg := strings.TrimSpace(event.RawMessage)
		if strings.HasPrefix(cleanMsg, cmd) {
			matchedHandler = handlers[cmd]
			break
		}
	}
	handlersMu.RUnlock()

	if matchedHandler != nil {
		matchedHandler(conn, event)
	}
}
