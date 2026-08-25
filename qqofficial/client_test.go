package qqofficial

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"qq-pet-saas/config"
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
	groupData := json.RawMessage(`{"id":"group-message","group_openid":"group-openid","content":" 状态 ","timestamp":"2026-08-12T10:00:00+08:00","author":{"member_openid":"member-openid","username":"小满"}}`)
	group, ok, err := MapDispatch("app", GatewayPayload{ID: "event-1", Op: OpDispatch, Sequence: 8, Type: "GROUP_AT_MESSAGE_CREATE", Data: groupData})
	if err != nil || !ok {
		t.Fatalf("group mapping failed: ok=%v err=%v", ok, err)
	}
	if group.Platform != core.PlatformQQGroup || group.SpaceID != "group-openid" || group.ActorID != "member-openid" || group.ActorName != "小满" || group.Text != "状态" {
		t.Fatalf("unexpected group event: %#v", group)
	}

	guildData := json.RawMessage(`{"id":"guild-message","guild_id":"guild-id","channel_id":"channel-id","content":" 远征 2 ","timestamp":"2026-08-12T10:00:00+08:00","author":{"id":"member-id","username":"巡林客"}}`)
	guild, ok, err := MapDispatch("app", GatewayPayload{ID: "event-2", Op: OpDispatch, Sequence: 9, Type: "AT_MESSAGE_CREATE", Data: guildData})
	if err != nil || !ok {
		t.Fatalf("guild mapping failed: ok=%v err=%v", ok, err)
	}
	if guild.Platform != core.PlatformQQGuild || guild.SpaceID != "guild-id" || guild.RoomID != "channel-id" || guild.ActorID != "member-id" || guild.ActorName != "巡林客" {
		t.Fatalf("unexpected guild event: %#v", guild)
	}

	c2cData := json.RawMessage(`{"id":"direct-message","content":" 菜单 ","timestamp":"2026-08-12T10:00:00+08:00","author":{"user_openid":"user-openid"}}`)
	direct, ok, err := MapDispatch("app", GatewayPayload{ID: "event-3", Op: OpDispatch, Sequence: 10, Type: "C2C_MESSAGE_CREATE", Data: c2cData})
	if err != nil || !ok {
		t.Fatalf("c2c mapping failed: ok=%v err=%v", ok, err)
	}
	if direct.Platform != core.PlatformQQGroup || direct.SceneType != core.SceneDirect || direct.ActorID != "user-openid" || direct.Text != "菜单" {
		t.Fatalf("unexpected c2c event: %#v", direct)
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

func TestClientAddressesOfficialGroupWithoutLeakingOpenID(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"sent"}`))
	}))
	defer server.Close()
	client := NewClient("app", staticToken("access"), server.URL, server.Client())
	client.MarkdownEnabled = true
	event := core.InboundEvent{Platform: core.PlatformQQGroup, SceneType: core.SceneGroup, SpaceID: "group", ActorID: "opaque-openid", ActorName: " <小 满> ", MessageID: "source"}
	message := core.OutboundMessage{Text: "宠物近况", Markdown: &core.MarkdownPayload{Content: "**宠物近况**"}}
	if _, err := client.Send(context.Background(), event, message); err != nil {
		t.Fatal(err)
	}
	if body["content"] != "@小 满\n宠物近况" || strings.Contains(fmt.Sprint(body), "opaque-openid") {
		t.Fatalf("unexpected addressed group payload: %#v", body)
	}
	reference, ok := body["message_reference"].(map[string]interface{})
	if !ok || reference["message_id"] != "source" {
		t.Fatalf("group reply must quote the source: %#v", body)
	}
	markdown := body["markdown"].(map[string]interface{})
	if markdown["content"] != "@小 满\n\n**宠物近况**" {
		t.Fatalf("markdown must address the actor too: %#v", markdown)
	}
}

func TestClientAddressesGuildTextAndMarkdownWithNativeMention(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"sent"}`))
	}))
	defer server.Close()
	client := NewClient("app", staticToken("access"), server.URL, server.Client())
	client.MarkdownEnabled = true
	event := core.InboundEvent{Platform: core.PlatformQQGuild, SceneType: core.SceneGuild, RoomID: "channel", ActorID: "member-id", MessageID: "source"}
	message := core.OutboundMessage{Text: "远征归来", Markdown: &core.MarkdownPayload{Content: "**远征归来**"}}
	if _, err := client.Send(context.Background(), event, message); err != nil {
		t.Fatal(err)
	}
	if body["content"] != "<@member-id>\n远征归来" {
		t.Fatalf("guild text must mention the actor: %#v", body)
	}
	markdown := body["markdown"].(map[string]interface{})
	if markdown["content"] != "<@member-id>\n\n**远征归来**" {
		t.Fatalf("guild markdown must mention the actor: %#v", markdown)
	}
}

func TestClientSendsC2CReplyToUserEndpoint(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.RequestURI
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"sent"}`))
	}))
	defer server.Close()
	client := NewClient("app", staticToken("access"), server.URL, server.Client())
	event := core.InboundEvent{Platform: core.PlatformQQGroup, SceneType: core.SceneDirect, ActorID: "user id", MessageID: "source"}
	if _, err := client.Send(context.Background(), event, core.OutboundMessage{Text: "私聊回复"}); err != nil {
		t.Fatal(err)
	}
	if path != "/v2/users/user%20id/messages" {
		t.Fatalf("unexpected C2C request path=%q", path)
	}
}

func TestClientSendsGuildImageAsPublicURL(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"sent"}`))
	}))
	defer server.Close()
	client := NewClient("app", staticToken("access"), server.URL, server.Client())
	event := core.InboundEvent{Platform: core.PlatformQQGuild, SceneType: core.SceneGuild, RoomID: "channel", MessageID: "source"}
	if _, err := client.Send(context.Background(), event, core.OutboundMessage{Text: "宠物近况", Image: "https://cdn.example.com/pet.png"}); err != nil {
		t.Fatal(err)
	}
	if body["image"] != "https://cdn.example.com/pet.png" {
		t.Fatalf("频道图片未写入独立 image 字段: %#v", body)
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

func TestClientUploadsGroupImageThenSendsText(t *testing.T) {
	var messages []map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v2/groups/group/files":
			var body map[string]interface{}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body["url"] != "https://cdn.example.com/pet.png" || body["file_type"] != float64(1) {
				t.Errorf("unexpected media upload: %#v", body)
			}
			_, _ = writer.Write([]byte(`{"file_info":"media-token"}`))
		case "/v2/groups/group/messages":
			var body map[string]interface{}
			_ = json.NewDecoder(request.Body).Decode(&body)
			messages = append(messages, body)
			_, _ = writer.Write([]byte(`{"id":"sent"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := NewClient("app", staticToken("access"), server.URL, server.Client())
	limiter := &countingLimiter{}
	client.Limiter = limiter
	event := core.InboundEvent{Platform: core.PlatformQQGroup, SceneType: core.SceneGroup, SpaceID: "group", MessageID: "source"}
	message := core.OutboundMessage{Text: "宠物近况", Image: "https://cdn.example.com/pet.png"}
	if _, err := client.Send(context.Background(), event, message); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected media and text messages, got %#v", messages)
	}
	if messages[0]["msg_type"] != float64(7) || messages[0]["msg_seq"] != float64(1) {
		t.Fatalf("unexpected media message: %#v", messages[0])
	}
	media := messages[0]["media"].(map[string]interface{})
	if media["file_info"] != "media-token" {
		t.Fatalf("unexpected media file info: %#v", media)
	}
	if messages[1]["content"] != "宠物近况" || messages[1]["msg_seq"] != float64(2) {
		t.Fatalf("unexpected text follow-up: %#v", messages[1])
	}
	if limiter.calls.Load() != 3 {
		t.Fatalf("external requests reserved = %d, want 3 (upload, media, text)", limiter.calls.Load())
	}
}

func TestClientUploadsLocalGroupImageWithoutPublicImageHost(t *testing.T) {
	temp := t.TempDir()
	imageDir := filepath.Join(temp, "图片", "宠物图片")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	imageData := []byte("small-local-image")
	if err := os.WriteFile(filepath.Join(imageDir, "pet.png"), imageData, 0o644); err != nil {
		t.Fatal(err)
	}
	previousPath, previousHost := config.GlobalConfigPath, config.Core.ImageHost
	config.GlobalConfigPath, config.Core.ImageHost = temp, ""
	t.Cleanup(func() {
		config.GlobalConfigPath, config.Core.ImageHost = previousPath, previousHost
	})

	var uploaded []byte
	var prepared, finished, merged bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v2/groups/group/upload_prepare":
			prepared = true
			_, _ = fmt.Fprintf(writer, `{"upload_id":"upload-1","block_size":"%d","parts":[{"index":1,"presigned_url":%q,"block_size":"%d"}]}`, len(imageData), server.URL+"/cos-part", len(imageData))
		case "/cos-part":
			if request.Method != http.MethodPut {
				t.Errorf("unexpected part method %s", request.Method)
			}
			uploaded, _ = io.ReadAll(request.Body)
			writer.WriteHeader(http.StatusOK)
		case "/v2/groups/group/upload_part_finish":
			finished = true
			_, _ = writer.Write([]byte(`{}`))
		case "/v2/groups/group/files":
			merged = true
			_, _ = writer.Write([]byte(`{"file_info":"local-media"}`))
		case "/v2/groups/group/messages":
			_, _ = writer.Write([]byte(`{"id":"sent"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := NewClient("app", staticToken("access"), server.URL, server.Client())
	client.Limiter = nil
	event := core.InboundEvent{Platform: core.PlatformQQGroup, SceneType: core.SceneGroup, SpaceID: "group", MessageID: "source"}
	if _, err := client.Send(context.Background(), event, core.OutboundMessage{Text: "status", Image: "宠物图片\\pet.png"}); err != nil {
		t.Fatal(err)
	}
	if !prepared || !finished || !merged || !bytes.Equal(uploaded, imageData) {
		t.Fatalf("local upload incomplete: prepared=%v finished=%v merged=%v uploaded=%q", prepared, finished, merged, uploaded)
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

type countingLimiter struct{ calls atomic.Int32 }

func (limiter *countingLimiter) Wait(context.Context, core.InboundEvent) error {
	limiter.calls.Add(1)
	return nil
}
