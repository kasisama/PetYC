package core

import (
	"github.com/gorilla/websocket"
)

// OneBotEvent 上行消息数据 (NapCat 推送过来的群消息)
type OneBotEvent struct {
	PostType    string `json:"post_type"`    // message
	MessageType string `json:"message_type"` // group / private
	GroupID     int64  `json:"group_id"`
	UserID      int64  `json:"user_id"`
	RawMessage  string `json:"raw_message"` // 消息文本内容
	Sender      struct {
		Nickname string `json:"nickname"`
	} `json:"sender"`
}

// OneBotAction 下行回复数据 (SaaS 发送给 NapCat 执行的操作)
type OneBotAction struct {
	Action string      `json:"action"` // send_group_msg / send_private_msg
	Params interface{} `json:"params"`
}

// GroupMsgParams 群消息发送参数
type GroupMsgParams struct {
	GroupID int64  `json:"group_id"`
	Message string `json:"message"`
}

// MessageHandler 核心路由分发的业务处理器签名
type MessageHandler func(conn *websocket.Conn, event *OneBotEvent)
