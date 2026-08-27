package core

import (
	"context"
	"errors"
	"strings"
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

func TestCommandRouterAcceptsSlashPrefixedCommand(t *testing.T) {
	router := NewCommandRouter()
	var receivedText string
	if err := router.Register("宠物菜单", func(_ context.Context, event InboundEvent) (OutboundMessage, error) {
		receivedText = event.Text
		return OutboundMessage{Text: "menu"}, nil
	}); err != nil {
		t.Fatal(err)
	}

	message, handled, err := router.Route(context.Background(), InboundEvent{Text: "  /宠物菜单  "})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("expected slash-prefixed command to be handled")
	}
	if message.Text != "menu" {
		t.Fatalf("expected menu response, got %q", message.Text)
	}
	if receivedText != "宠物菜单" {
		t.Fatalf("expected handler to receive normalized command, got %q", receivedText)
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

func TestCommandRouterRejectsDuplicateCommand(t *testing.T) {
	router := NewCommandRouter()
	handler := func(context.Context, InboundEvent) (OutboundMessage, error) {
		return OutboundMessage{Text: "ok"}, nil
	}
	if err := router.Register("签到", handler); err != nil {
		t.Fatal(err)
	}
	if err := router.Register("签到", handler); err == nil {
		t.Fatal("expected duplicate command registration to fail")
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
	RegisterUnifiedFeature(UnifiedFeature{FuncName: "status", DefaultCommand: "我的宠物", Aliases: []string{"状态", "宠物状态"}, DisplayName: "宠物状态", Category: "基础", Description: "查看宠物近况", Enabled: true}, func(context.Context, InboundEvent) (OutboundMessage, error) {
		return OutboundMessage{Text: "ok"}, nil
	})
	db.Create(&models.CommandConfig{FuncName: "status", Command: "近况", DisplayName: "宠物状态", Category: "基础", Description: "查看宠物近况", Enabled: true})
	if err = RebuildUnifiedRouter(db); err != nil {
		t.Fatal(err)
	}
	if _, handled, _ := RouteInbound(context.Background(), InboundEvent{Text: "我的宠物"}); handled {
		t.Fatal("默认触发词不应在自定义触发词生效后继续匹配")
	}
	message, handled, err := RouteInbound(context.Background(), InboundEvent{Text: "近况"})
	if err != nil || !handled || message.Text != "ok" {
		t.Fatalf("自定义触发词未生效: handled=%v message=%+v err=%v", handled, message, err)
	}
	for _, alias := range []string{"状态", "宠物状态"} {
		message, handled, err = RouteInbound(context.Background(), InboundEvent{Text: alias})
		if err != nil || !handled || message.Text != "ok" {
			t.Fatalf("兼容别名 %q 未生效: handled=%v message=%+v err=%v", alias, handled, message, err)
		}
	}
}

func TestRebuildUnifiedRouterRegistersConfiguredMenuScene(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.CommandConfig{}, &models.MenuConfig{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.MenuConfig{Name: "今日与状态", Reply: "今日图文菜单", Markdown: "# 今日与状态", Image: "https://cdn.example.com/today.webp"}).Error; err != nil {
		t.Fatal(err)
	}

	previousRouter, previousFeatures := unifiedRouter, unifiedFeatures
	unifiedRouter, unifiedFeatures = NewCommandRouter(), make(map[string]UnifiedFeature)
	t.Cleanup(func() { unifiedRouter, unifiedFeatures = previousRouter, previousFeatures })
	if err = RebuildUnifiedRouter(db); err != nil {
		t.Fatal(err)
	}
	message, handled, routeErr := RouteInbound(context.Background(), InboundEvent{Text: "今日与状态"})
	if routeErr != nil || !handled {
		t.Fatalf("菜单场景未注册: handled=%v err=%v", handled, routeErr)
	}
	if message.Text != "今日图文菜单" || message.Image != "https://cdn.example.com/today.webp" {
		t.Fatalf("菜单场景图文不匹配: %#v", message)
	}
	if message.Markdown == nil || message.Markdown.Content != "# 今日与状态" {
		t.Fatalf("菜单场景 Markdown 不匹配: %#v", message.Markdown)
	}
}

func TestRebuildUnifiedRouterDoesNotLetMenuSceneOverrideBusinessCommand(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.CommandConfig{}, &models.MenuConfig{}); err != nil {
		t.Fatal(err)
	}
	previousRouter, previousFeatures := unifiedRouter, unifiedFeatures
	unifiedRouter, unifiedFeatures = NewCommandRouter(), make(map[string]UnifiedFeature)
	t.Cleanup(func() { unifiedRouter, unifiedFeatures = previousRouter, previousFeatures })
	handler := func(context.Context, InboundEvent) (OutboundMessage, error) {
		return OutboundMessage{Text: "real-command"}, nil
	}
	if err = RegisterUnifiedFeature(UnifiedFeature{FuncName: "status", DefaultCommand: "我的宠物", DisplayName: "我的宠物", Enabled: true}, handler); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.MenuConfig{Name: "我的宠物", Reply: "菜单劫持", Image: "https://cdn.example.com/hijack.webp"}).Error; err != nil {
		t.Fatal(err)
	}
	if err = RebuildUnifiedRouter(db); err != nil {
		t.Fatalf("同名菜单不应导致路由重建失败: %v", err)
	}
	message, handled, routeErr := RouteInbound(context.Background(), InboundEvent{Text: "我的宠物"})
	if routeErr != nil || !handled {
		t.Fatalf("真实命令未执行: handled=%v err=%v", handled, routeErr)
	}
	if message.Text != "real-command" || message.Image != "" {
		t.Fatalf("菜单场景劫持了真实命令: %#v", message)
	}
}

func TestRebuildUnifiedRouterMenuSceneReadsLatestConfigOnTrigger(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.CommandConfig{}, &models.MenuConfig{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.MenuConfig{Name: "今日与状态", Reply: "旧菜单", Image: "https://cdn.example.com/old.webp"}).Error; err != nil {
		t.Fatal(err)
	}

	previousRouter, previousFeatures := unifiedRouter, unifiedFeatures
	unifiedRouter, unifiedFeatures = NewCommandRouter(), make(map[string]UnifiedFeature)
	t.Cleanup(func() { unifiedRouter, unifiedFeatures = previousRouter, previousFeatures })
	if err = RebuildUnifiedRouter(db); err != nil {
		t.Fatal(err)
	}
	initial, handled, routeErr := RouteInbound(context.Background(), InboundEvent{Text: "今日与状态"})
	if routeErr != nil || !handled || initial.Markdown != nil {
		t.Fatalf("空 Markdown 应保持纯文本: handled=%v message=%#v err=%v", handled, initial, routeErr)
	}
	if err = db.Model(&models.MenuConfig{}).Where("name = ?", "今日与状态").Updates(map[string]any{
		"reply":    "新菜单",
		"markdown": "**新菜单**",
		"image":    "https://cdn.example.com/new.webp",
	}).Error; err != nil {
		t.Fatal(err)
	}
	message, handled, routeErr := RouteInbound(context.Background(), InboundEvent{Text: "今日与状态"})
	if routeErr != nil || !handled {
		t.Fatalf("菜单场景未触发: handled=%v err=%v", handled, routeErr)
	}
	if message.Text != "新菜单" || message.Image != "https://cdn.example.com/new.webp" {
		t.Fatalf("菜单场景未读取最新配置: %#v", message)
	}
	if message.Markdown == nil || message.Markdown.Content != "**新菜单**" {
		t.Fatalf("菜单场景未读取最新 Markdown: %#v", message.Markdown)
	}
}

func TestRebuildUnifiedRouterRejectsExactTriggerOwnedByDifferentFeatures(t *testing.T) {
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
	handler := func(context.Context, InboundEvent) (OutboundMessage, error) { return OutboundMessage{Text: "ok"}, nil }
	if err = RegisterUnifiedFeature(UnifiedFeature{FuncName: "materials", DefaultCommand: "材料背包", DisplayName: "材料背包", Enabled: true}, handler); err != nil {
		t.Fatal(err)
	}
	if err = RegisterUnifiedFeature(UnifiedFeature{FuncName: "equipment", DefaultCommand: "装备背包", Aliases: []string{"装备"}, DisplayName: "装备背包", Enabled: true}, handler); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.CommandConfig{FuncName: "materials", Command: "装备", DisplayName: "材料背包", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.CommandConfig{FuncName: "equipment", Command: "装备背包", DisplayName: "装备背包", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err = RebuildUnifiedRouter(db); err == nil || !strings.Contains(err.Error(), "materials") || !strings.Contains(err.Error(), "equipment") {
		t.Fatalf("expected named command conflict, got %v", err)
	}
}

func TestSyncUnifiedCommandConfigsPreservesCustomTriggersAndAddsFamiliarCommands(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.CommandConfig{}, &models.SystemConfig{}); err != nil {
		t.Fatal(err)
	}
	previousRouter, previousFeatures := unifiedRouter, unifiedFeatures
	unifiedRouter, unifiedFeatures = NewCommandRouter(), make(map[string]UnifiedFeature)
	t.Cleanup(func() { unifiedRouter, unifiedFeatures = previousRouter, previousFeatures })
	handler := func(context.Context, InboundEvent) (OutboundMessage, error) { return OutboundMessage{Text: "ok"}, nil }
	if err = RegisterUnifiedFeature(UnifiedFeature{FuncName: "今日", DefaultCommand: "今日", DisplayName: "今日", Category: "陪伴", Enabled: true}, handler); err != nil {
		t.Fatal(err)
	}
	if err = RegisterUnifiedFeature(UnifiedFeature{FuncName: "签到", DefaultCommand: "签到", DisplayName: "签到", Category: "基础", Enabled: true}, handler); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.CommandConfig{FuncName: "今日", Command: "每日陪伴", DisplayName: "自定义今日", Category: "自定义", Enabled: false, SortOrder: 99}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&models.CommandConfig{}).Where("func_name = ?", "今日").Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.SystemConfig{Key: "Internal.CommandCatalogVersion", Value: "2"}).Error; err != nil {
		t.Fatal(err)
	}

	if err = SyncUnifiedCommandConfigs(db); err != nil {
		t.Fatal(err)
	}
	var existing models.CommandConfig
	if err = db.First(&existing, "func_name = ?", "今日").Error; err != nil {
		t.Fatal(err)
	}
	if existing.Command != "每日陪伴" || existing.Enabled {
		t.Fatalf("目录升级必须保留自定义触发词和开关: %#v", existing)
	}
	var familiar models.CommandConfig
	if err = db.First(&familiar, "func_name = ?", "签到").Error; err != nil {
		t.Fatal(err)
	}
	if familiar.Command != "签到" || !familiar.Enabled {
		t.Fatalf("应补充熟悉命令: %#v", familiar)
	}
}

func TestSyncUnifiedCommandConfigsBuildsMenuRoutesWithRootDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.CommandConfig{}, &models.SystemConfig{}, &models.MenuConfig{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.MenuConfig{Name: "今日与状态", Reply: "事务外菜单", Image: "https://cdn.example.com/menu.webp"}).Error; err != nil {
		t.Fatal(err)
	}

	previousRouter, previousFeatures := unifiedRouter, unifiedFeatures
	unifiedRouter, unifiedFeatures = NewCommandRouter(), make(map[string]UnifiedFeature)
	t.Cleanup(func() { unifiedRouter, unifiedFeatures = previousRouter, previousFeatures })

	if err = SyncUnifiedCommandConfigs(db); err != nil {
		t.Fatal(err)
	}
	message, handled, routeErr := RouteInbound(context.Background(), InboundEvent{Text: "今日与状态"})
	if routeErr != nil || !handled {
		t.Fatalf("事务提交后菜单路由不可用: handled=%v err=%v", handled, routeErr)
	}
	if message.Text != "事务外菜单" || message.Image != "https://cdn.example.com/menu.webp" {
		t.Fatalf("事务提交后菜单图文不匹配: %#v", message)
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
	if metrics[0].TechnicalResult == "" || metrics[1].TechnicalResult == "" {
		t.Fatalf("expected separate technical results: %#v", metrics)
	}
}
