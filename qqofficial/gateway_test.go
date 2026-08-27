package qqofficial

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"qq-pet-saas/core"
)

func TestLoadConfigRequiresCredentialsAndUsesMinimalDefaultIntents(t *testing.T) {
	t.Setenv("QQBOT_APP_ID", "")
	t.Setenv("QQBOT_APP_SECRET", "")
	if config, enabled, err := LoadConfigFromEnv(); err != nil || enabled || config.AppID != "" {
		t.Fatalf("missing credentials should disable adapter: enabled=%v err=%v config=%#v", enabled, err, config)
	}
	t.Setenv("QQBOT_APP_ID", "app")
	t.Setenv("QQBOT_APP_SECRET", "secret")
	config, enabled, err := LoadConfigFromEnv()
	if err != nil || !enabled {
		t.Fatalf("expected enabled adapter: enabled=%v err=%v", enabled, err)
	}
	expected := IntentsGroupAndC2C | IntentsPublicGuildMessages
	if config.Intents != expected {
		t.Fatalf("expected minimal intents %d, got %d", expected, config.Intents)
	}
}

func TestFetchGatewayInfoUsesAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/gateway/bot" || request.Header.Get("Authorization") != "QQBot access" {
			t.Fatalf("unexpected gateway request: %s %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(writer).Encode(GatewayInfo{URL: "ws://example", Shards: 2})
	}))
	defer server.Close()
	info, err := FetchGatewayInfo(context.Background(), server.URL, staticToken("access"), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if info.URL != "ws://example" || info.Shards != 2 {
		t.Fatalf("unexpected gateway info: %#v", info)
	}
}

func TestGatewayDispatchDeduplicatesAndRoutesOnce(t *testing.T) {
	var output bytes.Buffer
	previousWriter, previousFlags, previousPrefix := log.Writer(), log.Flags(), log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
		log.SetPrefix(previousPrefix)
	})

	sender := &recordingSender{}
	routes := 0
	gateway := NewGateway(Config{AppID: "app"}, staticToken("access"), sender)
	gateway.Route = func(_ context.Context, event core.InboundEvent) (core.OutboundMessage, bool, error) {
		routes++
		return core.OutboundMessage{Text: "ok"}, true, nil
	}
	mentioned := GatewayPayload{ID: "first-delivery", Op: OpDispatch, Sequence: 1, Type: "GROUP_AT_MESSAGE_CREATE", Data: json.RawMessage(`{"id":"message","group_openid":"group","content":"状态","msg_seq":3,"author":{"member_openid":"member"}}`)}
	full := GatewayPayload{ID: "second-delivery", Op: OpDispatch, Sequence: 2, Type: "GROUP_MESSAGE_CREATE", Data: mentioned.Data}
	if err := gateway.HandleDispatch(context.Background(), mentioned); err != nil {
		t.Fatal(err)
	}
	if err := gateway.HandleDispatch(context.Background(), full); err != nil {
		t.Fatal(err)
	}
	if routes != 1 || sender.sends != 1 {
		t.Fatalf("expected one route and send, routes=%d sends=%d", routes, sender.sends)
	}
	if !strings.Contains(output.String(), "忽略重复消息 type=GROUP_MESSAGE_CREATE") || strings.Contains(output.String(), "message_id=message") {
		t.Fatalf("duplicate log must identify the event type without exposing the full message id: %q", output.String())
	}
}

func TestGatewayDispatchDeduplicatesInteractionByEventID(t *testing.T) {
	sender := &recordingSender{}
	routes := 0
	gateway := NewGateway(Config{AppID: "app"}, staticToken("access"), sender)
	gateway.Route = func(_ context.Context, event core.InboundEvent) (core.OutboundMessage, bool, error) {
		routes++
		if event.EventID != "interaction-id" || event.MessageID != "" {
			t.Fatalf("unexpected interaction identity: %#v", event)
		}
		return core.OutboundMessage{Text: "ok"}, true, nil
	}
	payload := GatewayPayload{ID: "outer-delivery", Op: OpDispatch, Type: "INTERACTION_CREATE", Data: json.RawMessage(`{"id":"interaction-id","group_openid":"group","group_member_openid":"member","data":{"resolved":{"button_data":"状态"}}}`)}
	if err := gateway.HandleDispatch(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if err := gateway.HandleDispatch(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if routes != 1 || sender.sends != 1 || sender.acks != 1 {
		t.Fatalf("expected one interaction route, ack and send; routes=%d sends=%d acks=%d", routes, sender.sends, sender.acks)
	}
}

func TestGatewayDispatchFinishesAcceptedMessageAfterRuntimeCancellation(t *testing.T) {
	sender := &contextRecordingSender{}
	gateway := NewGateway(Config{AppID: "app"}, staticToken("access"), sender)
	gateway.Route = func(ctx context.Context, event core.InboundEvent) (core.OutboundMessage, bool, error) {
		if err := ctx.Err(); err != nil {
			t.Fatalf("accepted dispatch inherited canceled runtime context: %v", err)
		}
		return core.OutboundMessage{Text: "ok"}, true, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	payload := GatewayPayload{ID: "delivery", Op: OpDispatch, Sequence: 1, Type: "GROUP_AT_MESSAGE_CREATE", Data: json.RawMessage(`{"id":"message","group_openid":"group","content":"状态","msg_seq":1,"author":{"member_openid":"member"}}`)}
	if err := gateway.HandleDispatch(ctx, payload); err != nil {
		t.Fatal(err)
	}
	if sender.sends != 1 || sender.contextErr != nil {
		t.Fatalf("accepted message did not finish cleanly: sends=%d contextErr=%v", sender.sends, sender.contextErr)
	}
}

func TestRunConnectionClosesEstablishedSocketWhenRuntimeIsCanceled(t *testing.T) {
	connected := make(chan struct{})
	serverClosed := make(chan struct{})
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		_ = connection.WriteJSON(GatewayPayload{Op: OpHello, Data: json.RawMessage(`{"heartbeat_interval":60000}`)})
		var identify interface{}
		if err = connection.ReadJSON(&identify); err != nil {
			return
		}
		close(connected)
		var payload interface{}
		_ = connection.ReadJSON(&payload)
		close(serverClosed)
	}))
	defer server.Close()

	gateway := NewGateway(Config{AppID: "app", Intents: IntentsGroupAndC2C}, staticToken("access"), &recordingSender{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		done <- gateway.runConnection(ctx, wsURL, 0, 1, &shardSession{})
	}()

	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("test gateway did not establish websocket")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runConnection stayed blocked in ReadJSON after runtime cancellation")
	}
	select {
	case <-serverClosed:
	case <-time.After(time.Second):
		t.Fatal("server did not observe the canceled runtime closing its websocket")
	}
}

func TestGatewayDispatchLogsReceivedAndUnhandledMessage(t *testing.T) {
	var output bytes.Buffer
	previousWriter, previousFlags, previousPrefix := log.Writer(), log.Flags(), log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
		log.SetPrefix(previousPrefix)
	})

	gateway := NewGateway(Config{AppID: "app"}, staticToken("access"), &recordingSender{})
	gateway.Route = func(context.Context, core.InboundEvent) (core.OutboundMessage, bool, error) {
		return core.OutboundMessage{}, false, nil
	}
	payload := GatewayPayload{ID: "event", Op: OpDispatch, Sequence: 1, Type: "GROUP_AT_MESSAGE_CREATE", Data: json.RawMessage(`{"id":"message","group_openid":"group","content":" 菜单 ","author":{"member_openid":"member"}}`)}
	if err := gateway.HandleDispatch(context.Background(), payload); err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		`[QQOfficial] 收到消息 type=GROUP_AT_MESSAGE_CREATE content="菜单"`,
		`[QQOfficial] 未匹配指令 type=GROUP_AT_MESSAGE_CREATE content="菜单"`,
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected log %q, got %q", expected, output.String())
		}
	}
}

func TestConsoleMessageTextTruncatesLongMessages(t *testing.T) {
	message := strings.Repeat("宠", 161)
	result := consoleMessageText(message)
	if len([]rune(result)) != 161 || !strings.HasSuffix(result, "…") {
		t.Fatalf("expected 160 runes plus ellipsis, got %q", result)
	}
}

func TestShardCountCannotSilentlyDropRecommendedShards(t *testing.T) {
	if count, err := resolveShardCount(4, 0); err != nil || count != 4 {
		t.Fatalf("expected all recommended shards, count=%d err=%v", count, err)
	}
	if _, err := resolveShardCount(4, 1); err == nil {
		t.Fatal("partial shard override must be rejected")
	}
	if count, err := resolveShardCount(4, 4); err != nil || count != 4 {
		t.Fatalf("matching override should be accepted, count=%d err=%v", count, err)
	}
}

type recordingSender struct {
	sends int
	acks  int
}

type contextRecordingSender struct {
	sends      int
	contextErr error
}

func (sender *contextRecordingSender) Send(ctx context.Context, _ core.InboundEvent, _ core.OutboundMessage) (*SendResult, error) {
	sender.sends++
	sender.contextErr = ctx.Err()
	return &SendResult{ID: "sent"}, sender.contextErr
}

func (sender *contextRecordingSender) AcknowledgeInteraction(context.Context, string, string) error {
	return nil
}

func (sender *recordingSender) Send(context.Context, core.InboundEvent, core.OutboundMessage) (*SendResult, error) {
	sender.sends++
	return &SendResult{ID: "sent"}, nil
}

func (sender *recordingSender) AcknowledgeInteraction(context.Context, string, string) error {
	sender.acks++
	return nil
}
