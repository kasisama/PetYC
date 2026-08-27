package qqofficial

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"qq-pet-saas/core"
)

const (
	OpDispatch       = 0
	OpHeartbeat      = 1
	OpIdentify       = 2
	OpResume         = 6
	OpReconnect      = 7
	OpInvalidSession = 9
	OpHello          = 10
	OpHeartbeatACK   = 11

	IntentsGroupAndC2C         = 1 << 25
	IntentsInteraction         = 1 << 26
	IntentsMessageAudit        = 1 << 27
	IntentsPublicGuildMessages = 1 << 30
)

type GatewayPayload struct {
	ID       string          `json:"id,omitempty"`
	Op       int             `json:"op"`
	Data     json.RawMessage `json:"d,omitempty"`
	Sequence int             `json:"s,omitempty"`
	Type     string          `json:"t,omitempty"`
}

type groupMessageEvent struct {
	ID           string `json:"id"`
	GroupOpenID  string `json:"group_openid"`
	Content      string `json:"content"`
	Timestamp    string `json:"timestamp"`
	MsgSeq       int    `json:"msg_seq"`
	MessageScene struct {
		Ext []string `json:"ext"`
	} `json:"message_scene"`
	Author struct {
		MemberOpenID string `json:"member_openid"`
		Username     string `json:"username"`
	} `json:"author"`
}

type c2cMessageEvent struct {
	ID         string `json:"id"`
	UserOpenID string `json:"user_openid"`
	Content    string `json:"content"`
	Timestamp  string `json:"timestamp"`
	MsgSeq     int    `json:"msg_seq"`
	Author     struct {
		UserOpenID string `json:"user_openid"`
		Username   string `json:"username"`
	} `json:"author"`
}

type guildMessageEvent struct {
	ID        string `json:"id"`
	GuildID   string `json:"guild_id"`
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"author"`
}

type interactionEvent struct {
	ID            string `json:"id"`
	Type          int    `json:"type"`
	Scene         string `json:"scene"`
	GroupOpenID   string `json:"group_openid"`
	GuildID       string `json:"guild_id"`
	ChannelID     string `json:"channel_id"`
	GroupMemberID string `json:"group_member_openid"`
	UserOpenID    string `json:"user_openid"`
	Data          struct {
		Resolved struct {
			ButtonData string `json:"button_data"`
		} `json:"resolved"`
	} `json:"data"`
}

func MapDispatch(appID string, payload GatewayPayload) (core.InboundEvent, bool, error) {
	switch payload.Type {
	case "GROUP_AT_MESSAGE_CREATE", "GROUP_MESSAGE_CREATE":
		var message groupMessageEvent
		if err := json.Unmarshal(payload.Data, &message); err != nil {
			return core.InboundEvent{}, false, err
		}
		return core.InboundEvent{
			Platform: core.PlatformQQGroup, SceneType: core.SceneGroup, AppID: appID,
			SpaceID: message.GroupOpenID, RoomID: message.GroupOpenID, ActorID: message.Author.MemberOpenID, ActorName: strings.TrimSpace(message.Author.Username),
			MessageID: message.ID, ReferenceID: messageReferenceID(message.MessageScene.Ext), EventID: payload.ID, MessageSeq: message.MsgSeq,
			Text: normalizeGroupMessageContent(message.Content), Timestamp: parseTimestamp(message.Timestamp),
		}, true, nil
	case "C2C_MESSAGE_CREATE":
		var message c2cMessageEvent
		if err := json.Unmarshal(payload.Data, &message); err != nil {
			return core.InboundEvent{}, false, err
		}
		actorID := message.Author.UserOpenID
		if actorID == "" {
			actorID = message.UserOpenID
		}
		return core.InboundEvent{
			Platform: core.PlatformQQGroup, SceneType: core.SceneDirect, AppID: appID,
			SpaceID: actorID, RoomID: actorID, ActorID: actorID, ActorName: strings.TrimSpace(message.Author.Username),
			MessageID: message.ID, EventID: payload.ID, MessageSeq: message.MsgSeq,
			Text: strings.TrimSpace(message.Content), Timestamp: parseTimestamp(message.Timestamp),
		}, true, nil
	case "AT_MESSAGE_CREATE", "MESSAGE_CREATE":
		var message guildMessageEvent
		if err := json.Unmarshal(payload.Data, &message); err != nil {
			return core.InboundEvent{}, false, err
		}
		return core.InboundEvent{
			Platform: core.PlatformQQGuild, SceneType: core.SceneGuild, AppID: appID,
			SpaceID: message.GuildID, RoomID: message.ChannelID, ActorID: message.Author.ID, ActorName: strings.TrimSpace(message.Author.Username),
			MessageID: message.ID, EventID: payload.ID, Text: strings.TrimSpace(message.Content),
			Timestamp: parseTimestamp(message.Timestamp),
		}, true, nil
	case "INTERACTION_CREATE":
		var interaction interactionEvent
		if err := json.Unmarshal(payload.Data, &interaction); err != nil {
			return core.InboundEvent{}, false, err
		}
		if interaction.Data.Resolved.ButtonData == "" {
			return core.InboundEvent{}, false, nil
		}
		eventID := strings.TrimSpace(interaction.ID)
		if interaction.GroupOpenID != "" {
			return core.InboundEvent{Platform: core.PlatformQQGroup, SceneType: core.SceneGroup, AppID: appID, SpaceID: interaction.GroupOpenID, RoomID: interaction.GroupOpenID, ActorID: interaction.GroupMemberID, EventID: eventID, Text: strings.TrimSpace(interaction.Data.Resolved.ButtonData), Timestamp: time.Now()}, true, nil
		}
		if interaction.GuildID != "" {
			return core.InboundEvent{Platform: core.PlatformQQGuild, SceneType: core.SceneGuild, AppID: appID, SpaceID: interaction.GuildID, RoomID: interaction.ChannelID, ActorID: interaction.UserOpenID, EventID: eventID, Text: strings.TrimSpace(interaction.Data.Resolved.ButtonData), Timestamp: time.Now()}, true, nil
		}
		return core.InboundEvent{}, false, nil
	default:
		return core.InboundEvent{}, false, nil
	}
}

// normalizeGroupMessageContent removes the leading mention marker included in
// QQ Official group messages while preserving mentions in the command body.
func normalizeGroupMessageContent(content string) string {
	text := strings.TrimSpace(content)
	if !strings.HasPrefix(text, "<@") {
		return text
	}

	closing := strings.IndexByte(text, '>')
	if closing < 3 {
		return text
	}
	identifier := strings.TrimPrefix(text[2:closing], "!")
	if identifier == "" || strings.ContainsAny(identifier, " \t\r\n<>") {
		return text
	}
	return strings.TrimSpace(text[closing+1:])
}

func messageReferenceID(values []string) string {
	for _, value := range values {
		key, reference, found := strings.Cut(strings.TrimSpace(value), "=")
		if found && key == "msg_idx" {
			return strings.TrimSpace(reference)
		}
	}
	return ""
}

func parseTimestamp(value string) time.Time {
	if value == "" {
		return time.Now()
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now()
	}
	return parsed
}

func identifyPayload(token string, intents, shardID, shardCount int) interface{} {
	return struct {
		Op   int `json:"op"`
		Data struct {
			Token      string            `json:"token"`
			Intents    int               `json:"intents"`
			Shard      [2]int            `json:"shard"`
			Properties map[string]string `json:"properties"`
		} `json:"d"`
	}{
		Op: OpIdentify,
		Data: struct {
			Token      string            `json:"token"`
			Intents    int               `json:"intents"`
			Shard      [2]int            `json:"shard"`
			Properties map[string]string `json:"properties"`
		}{Token: "QQBot " + token, Intents: intents, Shard: [2]int{shardID, shardCount}, Properties: map[string]string{"$os": "windows", "$browser": "qq-pet-saas", "$device": "qq-pet-saas"}},
	}
}

func resumePayload(token, sessionID string, sequence int) interface{} {
	return struct {
		Op   int `json:"op"`
		Data struct {
			Token     string `json:"token"`
			SessionID string `json:"session_id"`
			Sequence  int    `json:"seq"`
		} `json:"d"`
	}{
		Op: OpResume,
		Data: struct {
			Token     string `json:"token"`
			SessionID string `json:"session_id"`
			Sequence  int    `json:"seq"`
		}{Token: "QQBot " + token, SessionID: sessionID, Sequence: sequence},
	}
}

func validateEvent(event core.InboundEvent) error {
	if event.SpaceID == "" || event.ActorID == "" || event.Text == "" {
		return fmt.Errorf("QQ 事件缺少场景、用户或文本")
	}
	return nil
}
