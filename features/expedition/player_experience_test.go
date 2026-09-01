package expedition

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

func TestRiskCopyUsesConfiguredCurrencyAndLotterySource(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Save(&models.SystemConfig{Key: "Core.CoinName", Value: "星砂"}).Error; err != nil {
		t.Fatal(err)
	}
	guess, err := handleRockPaperScissors(context.Background(), oneBotEvent("100", "coin-copy", "猜拳"), service)
	if err != nil {
		t.Fatal(err)
	}
	trade, err := handleTrade(context.Background(), oneBotEvent("100", "coin-copy", "宠物交易"), service)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{guess.Text, trade.Text} {
		if !strings.Contains(message, "星砂") || strings.Contains(message, "金币") {
			t.Fatalf("货币文案未统一: %q", message)
		}
	}
	if err := db.Create(&models.ChanceGameConfig{GameKey: "lottery", Name: "遗迹抽签", Enabled: true, CostItem: "遗迹抽签券", CostQuantity: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ChanceRewardConfig{GameKey: "lottery", RewardKey: "light-stone", Name: "光之石", Weight: 1, ItemName: "光之石", Quantity: 1, Rare: true, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	rules, err := service.GetChanceRules(context.Background(), "lottery")
	if err != nil {
		t.Fatal(err)
	}
	lottery := chanceRulesMessage(db, rules, "发送“抽奖一次”即可抽取。")
	if !strings.Contains(lottery.Text, "石环牧径") || strings.Contains(lottery.Text, "不售卖") {
		t.Fatalf("抽签券来源说明不完整: %q", lottery.Text)
	}
	missing := riskErrorMessageFor(db, gameplay.ErrInsufficientItem, "lottery")
	if !strings.Contains(missing.Text, "石环牧径") {
		t.Fatalf("缺券提示没有来源引导: %q", missing.Text)
	}
}

func TestEmptyCommandsOfferUsableItemsAndProducts(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "empty-choices", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	items := []models.ItemConfig{
		{Key: "food", Name: "活力脆饼", Type: "饱食", Status: "active", Effect: "18"},
		{Key: "book", Name: "专业书本", Type: "智慧", Status: "active", Effect: "6"},
		{Key: "bad-gift", Name: "无效礼盒", Type: "成长", Status: "active", Effect: "5"},
		{Key: "chime", Name: "风铃草结", Type: "礼物", Status: "active", Effect: "8"},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	inventory := gameplay.NewInventoryService(db)
	for _, item := range items {
		if err := inventory.Credit(context.Background(), accountID, item.Name, 1); err != nil {
			t.Fatal(err)
		}
	}
	event.Text = "喂养"
	feed, err := handleCompanion(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	assertButtonCommand(t, feed, "喂养 活力脆饼")
	event.Text = "学习"
	study, err := handleStartActivity(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	assertButtonCommand(t, study, "学习 专业书本")
	event.Text = "送礼"
	gift, err := handleCompanion(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	assertButtonCommand(t, gift, "送礼 风铃草结")
	if strings.Contains(gift.Text, "无效礼盒") {
		t.Fatalf("不应列出不可送礼的物品: %q", gift.Text)
	}
	if err := db.Create(&models.ItemConfig{Key: "biscuit", Name: "小饼干", Type: "饱食", Status: "active", Effect: "10"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ShopItemConfig{ShopType: gameplay.ShopTypeNormal, Name: "小饼干", Stock: -1, Price: 10}).Error; err != nil {
		t.Fatal(err)
	}
	event.Text = "购买"
	buy, err := handleBuy(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	assertButtonCommand(t, buy, "购买 小饼干")
}

func TestCombatOffersAndAcceptsChineseSkillCommand(t *testing.T) {
	service, db, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	event := oneBotEvent("100", "skill-button", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	var pet models.PetProfile
	if err := db.First(&pet, "account_id = ?", accountID).Error; err != nil {
		t.Fatal(err)
	}
	rows := []any{
		&models.AdventureSkillConfig{Key: "pet_skill_01", Name: "芽光连击", PowerPermille: 1000, AccuracyPermille: 1000, Enabled: true},
		&models.PetSkillUnlockConfig{FormKey: pet.CurrentForm, SkillKey: "pet_skill_01", UnlockLevel: 1},
		&models.AdventureMapConfig{Key: "starter", Name: "初始探索区", Region: "原野", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "meadow", MapKey: "starter", Name: "萤草坡", RecommendedLevel: 1, DifficultyPermille: 1000, HungerCost: 1, ReadinessCost: 1, Enabled: true},
		&models.AdventureMonsterConfig{Key: "fluff", Name: "萤绒团", Level: 1, MaxHealth: 100, Attack: 1, Defense: 1, Enabled: true},
		&models.AdventureEncounterConfig{ZoneKey: "meadow", EncounterKey: "fluff-encounter", EncounterType: "monster", TargetKey: "fluff", Name: "萤绒团", Weight: 1, Enabled: true},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := gameplay.RefreshPetSkillsTx(db, &pet); err != nil {
		t.Fatal(err)
	}
	event.Text = "探索 萤草坡"
	start, err := handleAdventureExplore(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(start.Text, "战斗技能：芽光连击") || strings.Contains(start.Text, "pet_skill_01") {
		t.Fatalf("战斗正文未展示中文技能命令: %q", start.Text)
	}
	assertButtonCommand(t, start, "战斗技能 芽光连击")
	assertButtonLabel(t, start, "战斗技能 芽光连击")
	event.Text = "战斗技能 芽光连击"
	turn, err := handleAdventureCombatAction(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(turn.Text, "战斗行动无效") {
		t.Fatalf("中文技能命令没有转换为内部 key: %q", turn.Text)
	}
}

func TestConfiguredExpeditionCollapsesUnavailableWorlds(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "expedition-collapse", "远征")
	accountID := accountIDForTest(t, service, event)
	rows := []any{
		&models.AdventureMapConfig{Key: "map-one", Name: "栖光原野", Enabled: true, SortOrder: 10},
		&models.AdventureMapConfig{Key: "map-two", Name: "潮痕遗址", Enabled: true, SortOrder: 20},
		&models.AdventureZoneConfig{Key: "zone-one", MapKey: "map-one", Name: "萤草坡", Enabled: true, SortOrder: 10},
		&models.AdventureZoneConfig{Key: "zone-two", MapKey: "map-two", Name: "退潮长廊", Enabled: true, SortOrder: 10},
		&models.AdventureZonePrerequisiteConfig{ZoneKey: "zone-two", PrerequisiteZoneKey: "zone-one"},
		&models.AdventureExpeditionConfig{ZoneKey: "zone-one", Name: "萤草巡查", DurationMinutes: 10, Enabled: true},
		&models.AdventureExpeditionConfig{ZoneKey: "zone-two", Name: "退潮巡查", DurationMinutes: 10, Enabled: true},
		&models.PlayerZoneProgress{AccountID: accountID, ZoneKey: "zone-one", ExplorationPercent: 100, ExpeditionUnlocked: true},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	message, err := handleConfiguredExpedition(context.Background(), event, service, accountID, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "萤草坡") || strings.Contains(message.Text, "退潮长廊") {
		t.Fatalf("远征列表未正确收起未开放世界: %q", message.Text)
	}
	assertButtonCommand(t, message, "远征 萤草坡")
}

func TestTodayTodoIsReadOnlyAndTodayOpensTodo(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "todo-readonly", "今日待办")
	withoutPet, err := handleTodayTodo(context.Background(), event, service)
	if err != nil || !strings.Contains(withoutPet.Text, "领养宠物") {
		t.Fatalf("无宠物待办错误: err=%v text=%q", err, withoutPet.Text)
	}
	event.Text = "领养 光芽兽"
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	if err := db.Model(&models.PetProfile{}).Where("account_id = ?", accountID).Updates(map[string]any{"status": "受伤", "hunger": 20}).Error; err != nil {
		t.Fatal(err)
	}
	var before int64
	db.Model(&models.CompanionJournal{}).Where("account_id = ?", accountID).Count(&before)
	event.Text = "今日待办"
	todo, err := handleTodayTodo(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(todo.Text, "建议优先\n· 宠物需要恢复") || !strings.Contains(todo.Text, "签到") {
		t.Fatalf("待办优先级错误: %q", todo.Text)
	}
	var after int64
	db.Model(&models.CompanionJournal{}).Where("account_id = ?", accountID).Count(&after)
	if before != after {
		t.Fatalf("查询待办不应写入签到记录: before=%d after=%d", before, after)
	}
	definitions := commandDefinitions()
	aliases := map[string]string{}
	for _, definition := range definitions {
		for _, alias := range definition.feature.Aliases {
			aliases[alias] = definition.feature.FuncName
		}
	}
	if aliases["今日"] != "today_todo" || aliases["待办"] != "today_todo" {
		t.Fatalf("今日应作为今日待办别名: %#v", aliases)
	}
}

func assertButtonCommand(t *testing.T, message core.OutboundMessage, command string) {
	t.Helper()
	if message.Keyboard == nil {
		t.Fatalf("缺少键盘，期望按钮 %q: %#v", command, message)
	}
	for _, row := range message.Keyboard.Rows {
		for _, button := range row {
			if button.Command == command {
				return
			}
		}
	}
	t.Fatalf("未找到按钮 %q: %#v", command, message.Keyboard)
}

func TestBossCommandOpensMapBossNotCommunityBoss(t *testing.T) {
	service, _, _ := newTestService(t)
	event := oneBotEvent("100", "boss-route", "首领")
	if _, err := handleAdopt(context.Background(), oneBotEvent("100", "boss-route", "领养 光芽兽"), service); err != nil {
		t.Fatal(err)
	}
	message, err := handleAdventureBoss(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message.Text, "本周社区首领") {
		t.Fatalf("首领入口仍走进社区首领: %q", message.Text)
	}
	if !strings.Contains(message.Text, "地图首领") && !strings.Contains(message.Text, "当前没有正在出现") {
		t.Fatalf("首领入口应进入地图首领: %q", message.Text)
	}
}

func TestConfiguredExpeditionGuidesExploreLoop(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "expedition-guide", "远征")
	accountID := accountIDForTest(t, service, event)
	if err := db.Create(&models.AdventureMapConfig{Key: "map-one", Name: "栖光原野", Enabled: true, SortOrder: 10}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AdventureZoneConfig{Key: "zone-one", MapKey: "map-one", Name: "萤草坡", Enabled: true, SortOrder: 10}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AdventureExpeditionConfig{ZoneKey: "zone-one", Name: "萤草巡查", DurationMinutes: 10, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PlayerZoneProgress{AccountID: accountID, ZoneKey: "zone-one", ExplorationPercent: 100, ExpeditionUnlocked: true}).Error; err != nil {
		t.Fatal(err)
	}
	message, err := handleConfiguredExpedition(context.Background(), event, service, accountID, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "探索地图") || !strings.Contains(message.Text, "地图首领") {
		t.Fatalf("远征列表缺少探索→远征→首领引导: %q", message.Text)
	}
}

func TestConfiguredExpeditionStatusShowsClaimReadyAfterEnd(t *testing.T) {
	service, db, now := newTestService(t)
	event := oneBotEvent("100", "expedition-overdue", "远征状态")
	accountID := accountIDForTest(t, service, event)
	snapshot, err := json.Marshal(AdventureExpeditionSnapshot{
		Config: models.AdventureExpeditionConfig{Name: "萤草坡定期调查"},
		Zone:   models.AdventureZoneConfig{Name: "萤草坡"},
		Grade:  "成功",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.AdventureExpeditionRun{
		ID: "run-overdue", AccountID: accountID, PetID: "pet-1", MapKey: "map-one", ZoneKey: "zone-one",
		Status: "running", SnapshotJSON: string(snapshot), StartedAt: now.Add(-30 * time.Minute), EndsAt: now.Add(-21 * time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}
	message, found, err := configuredExpeditionStatus(context.Background(), service, accountID)
	if err != nil || !found {
		t.Fatalf("应查到进行中的区域远征: found=%v err=%v", found, err)
	}
	if strings.Contains(message.Text, "剩余") || strings.Contains(message.Text, "1秒") {
		t.Fatalf("到期后不应再显示剩余 1 秒: %q", message.Text)
	}
	if !strings.Contains(message.Text, "已完成") || !strings.Contains(message.Text, "领取") {
		t.Fatalf("到期后应提示领取: %q", message.Text)
	}
	assertButtonLabel(t, message, "领取")
}

func TestInventoryAndCodexPaginate(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "bag-page", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	for i := 1; i <= 25; i++ {
		name := fmt.Sprintf("材料%02d", i)
		if err := db.Create(&models.GlobalInventoryItem{AccountID: accountID, ItemKey: name, ItemName: name, Quantity: 1}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.CodexEntry{AccountID: accountID, Category: "足迹", EntryKey: name, Progress: 10}).Error; err != nil {
			t.Fatal(err)
		}
	}
	page1, err := handleInventory(context.Background(), oneBotEvent("100", "bag-page", "我的背包"), service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page1.Text, "第 1/") || !strings.Contains(page1.Text, "材料01") || strings.Contains(page1.Text, "材料21") {
		t.Fatalf("背包第一页不正确: %q", page1.Text)
	}
	assertButtonLabel(t, page1, "我的背包 2")
	page2, err := handleInventory(context.Background(), oneBotEvent("100", "bag-page", "我的背包 2"), service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page2.Text, "材料21") || strings.Contains(page2.Text, "材料01") {
		t.Fatalf("背包第二页不正确: %q", page2.Text)
	}
	codex, err := handleCodex(context.Background(), oneBotEvent("100", "bag-page", "图鉴 2"), service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(codex.Text, "第 2/") {
		t.Fatalf("图鉴分页不正确: %q", codex.Text)
	}
}

func TestShopShowsTextAndButtonsForPageNavigation(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "shop-pages", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 6; index++ {
		name := fmt.Sprintf("商品%02d", index)
		if err := db.Create(&models.ItemConfig{Key: name, Name: name, Type: "饱食", Status: "active", Effect: "1"}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.ShopItemConfig{ShopType: gameplay.ShopTypeNormal, Name: name, Stock: -1, Price: int64(index)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	message, err := handleShop(context.Background(), oneBotEvent("100", "shop-pages", "商店"), service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "第 1/2 页") || !strings.Contains(message.Text, "➡️ 下一页：商店 2") {
		t.Fatalf("商店首页必须用文字说明翻页: %q", message.Text)
	}
	assertButtonCommand(t, message, "商店 2")
	page2, err := handleShop(context.Background(), oneBotEvent("100", "shop-pages", "商店 2"), service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page2.Text, "⬅️ 上一页：商店 1") {
		t.Fatalf("商店末页必须能返回上一页: %q", page2.Text)
	}
	assertButtonCommand(t, page2, "商店 1")
}

func TestLotteryWithoutConfigDoesNotUseDefaultPool(t *testing.T) {
	service, _, _ := newTestService(t)
	if _, err := handleAdopt(context.Background(), oneBotEvent("100", "lottery-empty", "领养 光芽兽"), service); err != nil {
		t.Fatal(err)
	}
	message, err := handleLottery(context.Background(), oneBotEvent("100", "lottery-empty", "抽奖"), service)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message.Text, "小鱼") || strings.Contains(message.Text, "陪伴印记") || strings.Contains(message.Text, "光之石") {
		t.Fatalf("空库抽奖不应回落默认奖池: %q", message.Text)
	}
	if !strings.Contains(message.Text, "还没有配置奖池") && !strings.Contains(message.Text, "暂时没有开放") {
		t.Fatalf("空库抽奖应友好失败: %q", message.Text)
	}
}

func assertButtonLabel(t *testing.T, message core.OutboundMessage, label string) {
	t.Helper()
	if message.Keyboard == nil {
		t.Fatalf("缺少键盘，期望标签 %q: %#v", label, message)
	}
	for _, row := range message.Keyboard.Rows {
		for _, button := range row {
			if button.Label == label && button.Command == label {
				return
			}
		}
	}
	t.Fatalf("未找到 Label=Command=%q 的按钮: %#v", label, message.Keyboard)
}
