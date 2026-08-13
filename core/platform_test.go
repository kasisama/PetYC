package core

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
)

func TestCommandRouterUsesLongestMatchingCommand(t *testing.T) {
	router := NewCommandRouter()
	if err := router.Register("远征", func(context.Context, InboundEvent) (OutboundMessage, error) {
		return OutboundMessage{Text: "list"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := router.Register("远征状态", func(context.Context, InboundEvent) (OutboundMessage, error) {
		return OutboundMessage{Text: "status"}, nil
	}); err != nil {
		t.Fatal(err)
	}

	message, handled, err := router.Route(context.Background(), InboundEvent{Text: " 远征状态 "})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if message.Text != "status" {
		t.Fatalf("expected longest command to win, got %q", message.Text)
	}
}

func TestCommandRouterRejectsEmptyCommand(t *testing.T) {
	router := NewCommandRouter()
	err := router.Register("  ", func(context.Context, InboundEvent) (OutboundMessage, error) {
		return OutboundMessage{}, nil
	})
	if err == nil {
		t.Fatal("expected empty command registration to fail")
	}
}

func TestRebuildUnifiedRouterUsesEnabledConfiguredTrigger(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.CommandConfig{}); err != nil {
		t.Fatal(err)
	}
	previousRouter, previousFeatures := unifiedRouter, unifiedFeatures
	unifiedRouter, unifiedFeatures = NewCommandRouter(), make(map[string]UnifiedFeature)
	t.Cleanup(func() { unifiedRouter, unifiedFeatures = previousRouter, previousFeatures })
	RegisterUnifiedFeature(UnifiedFeature{FuncName: "status", DefaultCommand: "状态", DisplayName: "宠物状态", Category: "基础", Description: "查看宠物近况", Enabled: true}, func(context.Context, InboundEvent) (OutboundMessage, error) {
		return OutboundMessage{Text: "ok"}, nil
	})
	db.Create(&models.CommandConfig{FuncName: "status", Command: "近况", DisplayName: "宠物状态", Category: "基础", Description: "查看宠物近况", Enabled: true})
	if err = RebuildUnifiedRouter(db); err != nil {
		t.Fatal(err)
	}
	if _, handled, _ := RouteInbound(context.Background(), InboundEvent{Text: "状态"}); handled {
		t.Fatal("默认触发词不应在自定义触发词生效后继续匹配")
	}
	message, handled, err := RouteInbound(context.Background(), InboundEvent{Text: "近况"})
	if err != nil || !handled || message.Text != "ok" {
		t.Fatalf("自定义触发词未生效: handled=%v message=%+v err=%v", handled, message, err)
	}
}

func TestRetiredLegacyCommandPrefixesAreBlockedBeforeFallback(t *testing.T) {
	for _, command := range []string{"宠物打工 护工", "宠物偷袭 @某人", "宠物钓鱼", "创建家族 星光"} {
		if !IsRetiredLegacyCommand(command) {
			t.Fatalf("下线指令未被识别: %s", command)
		}
	}
	if IsRetiredLegacyCommand("宠物摸头") || IsRetiredLegacyCommand("宠物散步") {
		t.Fatal("低压力陪伴互动不应被下线拦截")
	}
}

func TestOneBotEventConvertsToPlatformEvent(t *testing.T) {
	event := OneBotEvent{
		PostType:    "message",
		MessageType: "group",
		GroupID:     456,
		UserID:      123,
		RawMessage:  " 状态 ",
	}

	inbound := event.ToInbound("legacy-app")
	if inbound.Platform != PlatformOneBot || inbound.SceneType != SceneGroup {
		t.Fatalf("unexpected platform scene: %s/%s", inbound.Platform, inbound.SceneType)
	}
	if inbound.AppID != "legacy-app" || inbound.SpaceID != "456" || inbound.RoomID != "456" || inbound.ActorID != "123" {
		t.Fatalf("unexpected identity mapping: %#v", inbound)
	}
	if inbound.Text != "状态" {
		t.Fatalf("expected trimmed text, got %q", inbound.Text)
	}
}

func TestOutboundMessageFallsBackToText(t *testing.T) {
	message := OutboundMessage{
		Text:     "纯文本内容",
		Markdown: &MarkdownPayload{Content: "**增强内容**"},
	}
	if got := message.Render(false, false); got.Text != "纯文本内容" || got.Markdown != nil || got.Keyboard != nil {
		t.Fatalf("unexpected fallback rendering: %#v", got)
	}
}

func TestRouteOneBotMessageUsesUnifiedRouter(t *testing.T) {
	previous := unifiedRouter
	unifiedRouter = NewCommandRouter()
	t.Cleanup(func() { unifiedRouter = previous })
	if err := RegisterUnifiedHandler("状态", func(_ context.Context, event InboundEvent) (OutboundMessage, error) {
		return OutboundMessage{Text: string(event.Platform) + ":ok"}, nil
	}); err != nil {
		t.Fatal(err)
	}

	message, handled, err := routeOneBotMessage(context.Background(), OneBotEvent{
		PostType: "message", MessageType: "group", GroupID: 9, UserID: 7, RawMessage: "状态",
	})
	if err != nil || !handled {
		t.Fatalf("expected unified route, handled=%v err=%v", handled, err)
	}
	if message.Text != "onebot:ok" {
		t.Fatalf("unexpected response %q", message.Text)
	}
}

func TestCommandRouterRecordsAnonymousSuccessAndFailureMetrics(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.GameplayMetric{}); err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })
	router := NewCommandRouter()
	_ = router.Register("成功", func(context.Context, InboundEvent) (OutboundMessage, error) { return OutboundMessage{Text: "ok"}, nil })
	_ = router.Register("失败", func(context.Context, InboundEvent) (OutboundMessage, error) {
		return OutboundMessage{}, errors.New("boom")
	})
	event := InboundEvent{Platform: PlatformQQGroup, SceneType: SceneGroup, AppID: "app", SpaceID: "group"}
	event.Text = "成功"
	_, _, _ = router.Route(context.Background(), event)
	event.Text = "失败"
	_, _, _ = router.Route(context.Background(), event)
	var metrics []models.GameplayMetric
	if err = db.Order("command").Find(&metrics).Error; err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 2 || db.Migrator().HasColumn(&models.GameplayMetric{}, "actor_id") || db.Migrator().HasColumn(&models.GameplayMetric{}, "message_text") {
		t.Fatalf("expected two anonymous metrics without player content: %#v", metrics)
	}
}
