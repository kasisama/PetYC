package qqofficial

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	sender := &recordingSender{}
	routes := 0
	gateway := NewGateway(Config{AppID: "app"}, staticToken("access"), sender)
	gateway.Route = func(_ context.Context, event core.InboundEvent) (core.OutboundMessage, bool, error) {
		routes++
		return core.OutboundMessage{Text: "ok"}, true, nil
	}
	payload := GatewayPayload{ID: "event", Op: OpDispatch, Sequence: 1, Type: "GROUP_AT_MESSAGE_CREATE", Data: json.RawMessage(`{"id":"message","group_openid":"group","content":"状态","author":{"member_openid":"member"}}`)}
	if err := gateway.HandleDispatch(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if err := gateway.HandleDispatch(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if routes != 1 || sender.sends != 1 {
		t.Fatalf("expected one route and send, routes=%d sends=%d", routes, sender.sends)
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

func (sender *recordingSender) Send(context.Context, core.InboundEvent, core.OutboundMessage) (*SendResult, error) {
	sender.sends++
	return &SendResult{ID: "sent"}, nil
}

func (sender *recordingSender) AcknowledgeInteraction(context.Context, string, string) error {
	sender.acks++
	return nil
}
