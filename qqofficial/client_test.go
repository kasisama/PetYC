package qqofficial

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"qq-pet-saas/core"
)

func TestTokenProviderCachesUntilRefreshWindow(t *testing.T) {
	var calls atomic.Int32
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", request.Method)
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"access_token": "token-value", "expires_in": "7200"})
	}))
	defer server.Close()
	provider := NewTokenProvider("app", "secret", server.URL, server.Client())
	provider.Now = func() time.Time { return now }

	first, err := provider.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first != "token-value" || second != first || calls.Load() != 1 {
		t.Fatalf("expected one cached token request, calls=%d first=%q second=%q", calls.Load(), first, second)
	}
}

func TestMapDispatchMapsGroupAndGuildIdentity(t *testing.T) {
	groupData := json.RawMessage(`{"id":"group-message","group_openid":"group-openid","content":" 状态 ","timestamp":"2026-08-12T10:00:00+08:00","author":{"member_openid":"member-openid"}}`)
	group, ok, err := MapDispatch("app", GatewayPayload{ID: "event-1", Op: OpDispatch, Sequence: 8, Type: "GROUP_AT_MESSAGE_CREATE", Data: groupData})
	if err != nil || !ok {
		t.Fatalf("group mapping failed: ok=%v err=%v", ok, err)
	}
	if group.Platform != core.PlatformQQGroup || group.SpaceID != "group-openid" || group.ActorID != "member-openid" || group.Text != "状态" {
		t.Fatalf("unexpected group event: %#v", group)
	}

	guildData := json.RawMessage(`{"id":"guild-message","guild_id":"guild-id","channel_id":"channel-id","content":" 远征 2 ","timestamp":"2026-08-12T10:00:00+08:00","author":{"id":"member-id"}}`)
	guild, ok, err := MapDispatch("app", GatewayPayload{ID: "event-2", Op: OpDispatch, Sequence: 9, Type: "AT_MESSAGE_CREATE", Data: guildData})
	if err != nil || !ok {
		t.Fatalf("guild mapping failed: ok=%v err=%v", ok, err)
	}
	if guild.Platform != core.PlatformQQGuild || guild.SpaceID != "guild-id" || guild.RoomID != "channel-id" || guild.ActorID != "member-id" {
		t.Fatalf("unexpected guild event: %#v", guild)
	}
}

func TestClientSendsPlainTextFallbackToGroup(t *testing.T) {
	var path string
	var authorization string
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.RequestURI
		authorization = request.Header.Get("Authorization")
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"sent"}`))
	}))
	defer server.Close()
	client := NewClient("app", staticToken("access"), server.URL, server.Client())
	event := core.InboundEvent{Platform: core.PlatformQQGroup, SceneType: core.SceneGroup, SpaceID: "group id", MessageID: "source"}
	message := core.OutboundMessage{Text: "纯文本", Markdown: &core.MarkdownPayload{Content: "**增强**"}, Keyboard: &core.KeyboardPayload{}}
	if _, err := client.Send(context.Background(), event, message); err != nil {
		t.Fatal(err)
	}
	if path != "/v2/groups/group%20id/messages" || authorization != "QQBot access" {
		t.Fatalf("unexpected request path=%q authorization=%q", path, authorization)
	}
	if body["content"] != "纯文本" || body["msg_type"] != float64(0) || body["msg_id"] != "source" {
		t.Fatalf("unexpected fallback payload: %#v", body)
	}
	if _, exists := body["markdown"]; exists {
		t.Fatalf("markdown must be omitted without capability: %#v", body)
	}
}

func TestClientRendersOfficialCommandKeyboardWhenEnabled(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"sent"}`))
	}))
	defer server.Close()
	client := NewClient("app", staticToken("access"), server.URL, server.Client())
	client.MarkdownEnabled = true
	client.KeyboardEnabled = true
	event := core.InboundEvent{Platform: core.PlatformQQGroup, SceneType: core.SceneGroup, SpaceID: "group", MessageID: "source"}
	message := core.OutboundMessage{
		Text:     "发送远征 1",
		Markdown: &core.MarkdownPayload{Content: "**选择远征**"},
		Keyboard: &core.KeyboardPayload{Rows: [][]core.KeyboardButton{{{Label: "林间巡查", Command: "远征 1"}}}},
	}
	if _, err := client.Send(context.Background(), event, message); err != nil {
		t.Fatal(err)
	}
	if body["msg_type"] != float64(2) {
		t.Fatalf("expected markdown message: %#v", body)
	}
	keyboard, ok := body["keyboard"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected keyboard object: %#v", body["keyboard"])
	}
	content := keyboard["content"].(map[string]interface{})
	rows := content["rows"].([]interface{})
	buttons := rows[0].(map[string]interface{})["buttons"].([]interface{})
	action := buttons[0].(map[string]interface{})["action"].(map[string]interface{})
	if action["type"] != float64(2) || action["data"] != "远征 1" {
		t.Fatalf("unexpected command button action: %#v", action)
	}
}

func TestDeduplicatorRejectsRepeatedMessageSequence(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	deduper := NewDeduplicator(10 * time.Minute)
	deduper.Now = func() time.Time { return now }
	event := core.InboundEvent{AppID: "app", EventID: "event", MessageID: "message", MessageSeq: 1}
	if !deduper.Accept(event) {
		t.Fatal("first event should be accepted")
	}
	if deduper.Accept(event) {
		t.Fatal("duplicate event should be rejected")
	}
	now = now.Add(11 * time.Minute)
	if !deduper.Accept(event) {
		t.Fatal("expired dedupe entry should be accepted")
	}
}

func TestIdentifyAndResumePayloadsUseQQBotToken(t *testing.T) {
	identify := identifyPayload("access", IntentsGroupAndC2C|IntentsPublicGuildMessages, 1, 4)
	encoded, _ := json.Marshal(identify)
	if !strings.Contains(string(encoded), `"token":"QQBot access"`) || !strings.Contains(string(encoded), `"shard":[1,4]`) {
		t.Fatalf("unexpected identify payload %s", encoded)
	}
	resume := resumePayload("access", "session", 42)
	encoded, _ = json.Marshal(resume)
	if !strings.Contains(string(encoded), `"session_id":"session"`) || !strings.Contains(string(encoded), `"seq":42`) {
		t.Fatalf("unexpected resume payload %s", encoded)
	}
}

func TestRateLimiterUsesOfficialSceneBudgets(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter()
	group := core.InboundEvent{Platform: core.PlatformQQGroup, AppID: "app", SpaceID: "group"}
	if delay := limiter.Reserve(group, now); delay != 0 {
		t.Fatalf("first group reply should be immediate, got %v", delay)
	}
	if delay := limiter.Reserve(group, now); delay != 3*time.Second {
		t.Fatalf("same group should be limited to 20/min, got %v", delay)
	}
	guild := core.InboundEvent{Platform: core.PlatformQQGuild, AppID: "app", RoomID: "channel"}
	if delay := limiter.Reserve(guild, now); delay != 0 {
		t.Fatalf("first channel reply should be immediate, got %v", delay)
	}
	if delay := limiter.Reserve(guild, now); delay != 200*time.Millisecond {
		t.Fatalf("same channel should be limited to 5/sec, got %v", delay)
	}
}

type staticToken string

func (token staticToken) Token(context.Context) (string, error) { return string(token), nil }
