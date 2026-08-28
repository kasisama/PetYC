package expedition

import (
	"context"
	"strings"
	"testing"

	"qq-pet-saas/core"
	"qq-pet-saas/models"
)

func TestAdventureMapShowsDisplayNamesAndNameCommands(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "map-names", "地图")
	accountID := accountIDForTest(t, service, event)
	rows := []any{
		&models.AdventureMapConfig{Key: "sunlit_steppe", Name: "晴野草甸", Region: "原野", RecommendedLevel: 1, Enabled: true, SortOrder: 10},
		&models.AdventureZoneConfig{Key: "sunlit_steppe_z1", MapKey: "sunlit_steppe", Name: "萤草坡", RecommendedLevel: 1, Enabled: true, SortOrder: 10},
		&models.AdventureZoneConfig{Key: "sunlit_steppe_z2", MapKey: "sunlit_steppe", Name: "风车溪谷", RecommendedLevel: 3, Enabled: true, SortOrder: 20},
		&models.AdventureZonePrerequisiteConfig{ZoneKey: "sunlit_steppe_z2", PrerequisiteZoneKey: "sunlit_steppe_z1"},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	_ = accountID

	message, err := handleAdventureMaps(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "萤草坡") || !strings.Contains(message.Text, "风车溪谷") {
		t.Fatalf("地图应显示区域中文名: %q", message.Text)
	}
	if !strings.Contains(message.Text, "需先完成萤草坡") {
		t.Fatalf("未解锁提示应使用前置区域中文名: %q", message.Text)
	}
	if strings.Contains(message.Text, "sunlit_steppe") {
		t.Fatalf("地图不应向玩家展示内部区域编号: %q", message.Text)
	}
	if message.Markdown == nil || !strings.Contains(message.Markdown.Content, "## 晴野草甸") || !strings.Contains(message.Markdown.Content, "**萤草坡**") {
		t.Fatalf("地图 Markdown 应按大地图分层并加粗当前区域: %#v", message.Markdown)
	}
	assertPlainTextCompatible(t, message)
	if !strings.Contains(message.Text, "探索 萤草坡") && !strings.Contains(message.Text, "探索 区域名") {
		t.Fatalf("无 Markdown 的机器人也必须能靠纯文本探索: %q", message.Text)
	}
	if message.Keyboard == nil || len(message.Keyboard.Rows) == 0 || len(message.Keyboard.Rows[0]) == 0 {
		t.Fatalf("已解锁区域应提供探索按钮: %#v", message.Keyboard)
	}
	button := message.Keyboard.Rows[0][0]
	if button.Label != "探索 萤草坡" || button.Command != "探索 萤草坡" {
		t.Fatalf("探索按钮应发送中文区域名，而不是内部编号: %#v", button)
	}
}

func TestAdventureMapCollapsesLockedWorlds(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "map-collapse", "地图")
	_ = accountIDForTest(t, service, event)
	rows := []any{
		&models.AdventureMapConfig{Key: "sunlit_steppe", Name: "栖光原野", Region: "晨光草原", RecommendedLevel: 1, Enabled: true, SortOrder: 10},
		&models.AdventureMapConfig{Key: "tide_ruins", Name: "潮痕遗址", Region: "沉潮遗迹", RecommendedLevel: 7, Enabled: true, SortOrder: 20},
		&models.AdventureZoneConfig{Key: "sunlit_steppe_z1", MapKey: "sunlit_steppe", Name: "萤草坡", RecommendedLevel: 1, Enabled: true, SortOrder: 10},
		&models.AdventureZoneConfig{Key: "sunlit_steppe_z2", MapKey: "sunlit_steppe", Name: "风车溪谷", RecommendedLevel: 3, Enabled: true, SortOrder: 20},
		&models.AdventureZoneConfig{Key: "tide_ruins_z1", MapKey: "tide_ruins", Name: "退潮长廊", RecommendedLevel: 7, Enabled: true, SortOrder: 10},
		&models.AdventureZonePrerequisiteConfig{ZoneKey: "sunlit_steppe_z2", PrerequisiteZoneKey: "sunlit_steppe_z1"},
		&models.AdventureZonePrerequisiteConfig{ZoneKey: "tide_ruins_z1", PrerequisiteZoneKey: "sunlit_steppe_z2"},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	message, err := handleAdventureMaps(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	body := message.Text
	if message.Markdown != nil {
		body += "\n" + message.Markdown.Content
	}
	if !strings.Contains(body, "栖光原野") || !strings.Contains(body, "潮痕遗址") {
		t.Fatalf("地图应同时展示当前世界和后续世界标题: %q", body)
	}
	if !strings.Contains(body, "萤草坡") || !strings.Contains(body, "风车溪谷") {
		t.Fatalf("当前世界应展开区域列表: %q", body)
	}
	if strings.Contains(body, "退潮长廊") {
		t.Fatalf("尚未开放的世界不应展开全部锁定区域: %q", body)
	}
	if !strings.Contains(body, "需先完成风车溪谷") && !strings.Contains(body, "尚未开放") {
		t.Fatalf("收起的世界应给出简短开放条件: %q", body)
	}
}

func TestExploreCommandAcceptsZoneName(t *testing.T) {
	service, db, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	event := oneBotEvent("100", "explore-name", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	rows := []any{
		&models.AdventureMapConfig{Key: "starter", Name: "初始探索区", Region: "原野", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "forest", MapKey: "starter", Name: "森林边缘", RecommendedLevel: 1, DifficultyPermille: 1000, HungerCost: 1, ReadinessCost: 1, Enabled: true},
		&models.AdventureEncounterConfig{ZoneKey: "forest", EncounterKey: "safe-trail", EncounterType: "safe", Name: "林间小径", Description: "一段安静的调查小路。", Weight: 1, Enabled: true},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	event.Text = "探索 森林边缘"
	message, err := handleAdventureExplore(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "森林边缘") {
		t.Fatalf("按中文名探索应进入对应区域: %q", message.Text)
	}
	if message.Keyboard == nil {
		t.Fatalf("探索成功后应提供继续按钮")
	}
	foundContinue := false
	for _, row := range message.Keyboard.Rows {
		for _, button := range row {
			if button.Command == "探索 森林边缘" {
				foundContinue = true
			}
			if strings.Contains(button.Command, "forest") {
				t.Fatalf("继续探索按钮不应发送内部编号: %#v", button)
			}
		}
	}
	if !foundContinue {
		t.Fatalf("继续探索按钮应发送中文区域名: %#v", message.Keyboard)
	}
}

func TestCombatPanelShowsVersusNamesAndNamedDamage(t *testing.T) {
	message := formatAdventureCombatMessage(adventureCombatView{
		Title: "萤草坡 · 第 1 回合", PlayerName: "光芽兽", MonsterName: "萤绒团",
		PlayerHP: 86, MonsterHP: 32, PlayerDamage: 15, MonsterDamage: 2,
	})
	body := message.Text
	if message.Markdown != nil {
		body += "\n" + message.Markdown.Content
	}
	if !strings.Contains(body, "光芽兽") || !strings.Contains(body, "萤绒团") || !strings.Contains(body, "vs") {
		t.Fatalf("对阵面板应展示双方名字: %q", body)
	}
	if !strings.Contains(body, "生命 86") || !strings.Contains(body, "生命 32") {
		t.Fatalf("对阵面板应展示双方生命: %q", body)
	}
	if !strings.Contains(message.Text, "光芽兽 对 萤绒团 造成 15 点伤害") || !strings.Contains(message.Text, "萤绒团 对 光芽兽 造成 2 点伤害") {
		t.Fatalf("伤害应写成谁打了谁: %q", message.Text)
	}
	if strings.Contains(body, "宠物造成") || strings.Contains(body, "宠物受到") || strings.Contains(body, "我方生命") || strings.Contains(body, "敌方生命") {
		t.Fatalf("不应再用模糊的我方/敌方/宠物称呼: %q", body)
	}
	if message.Markdown == nil || !strings.Contains(message.Markdown.Content, "**光芽兽**") || !strings.Contains(message.Markdown.Content, "**15**") {
		t.Fatalf("Markdown 应对阵双方加粗并强调伤害: %#v", message.Markdown)
	}
	assertPlainTextCompatible(t, message)
	if !strings.Contains(message.Text, "普攻") {
		t.Fatalf("无 Markdown 的机器人也必须能靠纯文本操作战斗: %q", message.Text)
	}
}

func TestExploreCombatEncounterShowsBothFighters(t *testing.T) {
	service, db, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	event := oneBotEvent("100", "combat-panel", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	rows := []any{
		&models.AdventureMapConfig{Key: "starter", Name: "初始探索区", Region: "原野", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "meadow", MapKey: "starter", Name: "萤草坡", RecommendedLevel: 1, DifficultyPermille: 1000, HungerCost: 1, ReadinessCost: 1, Enabled: true},
		&models.AdventureMonsterConfig{Key: "fluff", Name: "萤绒团", Level: 1, MaxHealth: 47, Attack: 2, Defense: 1, Enabled: true},
		&models.AdventureEncounterConfig{ZoneKey: "meadow", EncounterKey: "fluff-encounter", EncounterType: "monster", TargetKey: "fluff", Name: "萤绒团", Description: "调查途中记录到的萤绒团。", Weight: 1, Enabled: true},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	event.Text = "探索 萤草坡"
	start, err := handleAdventureExplore(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(start.Text, "光芽兽") || !strings.Contains(start.Text, "萤绒团") || !strings.Contains(start.Text, "vs") {
		t.Fatalf("遭遇开始应对阵展示双方: %q", start.Text)
	}
	if strings.Contains(start.Text, "敌方生命") {
		t.Fatalf("遭遇开始不应再用敌方生命这种称呼: %q", start.Text)
	}

	event.Text = "普攻"
	turn, err := handleAdventureCombatAction(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(turn.Text, "光芽兽 对 萤绒团 造成") && !strings.Contains(turn.Text, "击败了") {
		t.Fatalf("回合战报应写出双方名字: %q", turn.Text)
	}
	if strings.Contains(turn.Text, "宠物造成") || strings.Contains(turn.Text, "我方生命") {
		t.Fatalf("回合战报不应再用模糊称呼: %q", turn.Text)
	}
}

func TestRewardTextUsesDisplayNamesWithoutInternalIDs(t *testing.T) {
	equipment := rewardText(AdventureReward{Type: "equipment", Key: "equipment_01", Name: "原野短杖", Quantity: 1, Equipment: &models.PlayerEquipment{ID: "ce7489f0-9442-410c-a773-fdf855fe7196", TemplateKey: "equipment_01"}})
	if equipment != "获得装备：原野短杖" {
		t.Fatalf("装备奖励应只显示中文名: %q", equipment)
	}
	if strings.Contains(equipment, "equipment_01") || strings.Contains(equipment, "ce7489f0") {
		t.Fatalf("装备奖励泄漏了字段名或编号: %q", equipment)
	}
	fragment := rewardText(AdventureReward{Type: "blueprint_fragment", Key: "equipment_01", Name: "原野短杖蓝图碎片", Quantity: 2})
	if fragment != "获得：原野短杖蓝图碎片 ×2" {
		t.Fatalf("蓝图奖励应显示中文名: %q", fragment)
	}
}

func TestEquipmentBagShowsTemplateNames(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "equip-names", "装备背包")
	accountID := accountIDForTest(t, service, event)
	if err := db.Create(&models.EquipmentTemplateConfig{Key: "equipment_01", Name: "原野短杖", Slot: "weapon", Rarity: "common", RequiredLevel: 1, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PlayerEquipment{ID: "eq-1", AccountID: accountID, TemplateKey: "equipment_01", Rarity: "common"}).Error; err != nil {
		t.Fatal(err)
	}

	message, err := handleEquipment(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "原野短杖") {
		t.Fatalf("装备背包应显示装备中文名: %q", message.Text)
	}
	if strings.Contains(message.Text, "equipment_01") {
		t.Fatalf("装备背包不应展示模板字段名: %q", message.Text)
	}
	if message.Keyboard == nil || len(message.Keyboard.Rows) == 0 || len(message.Keyboard.Rows[0]) == 0 {
		t.Fatalf("未穿戴装备应提供穿戴按钮: %#v", message.Keyboard)
	}
	button := message.Keyboard.Rows[0][0]
	if !strings.Contains(button.Label, "原野短杖") {
		t.Fatalf("穿戴按钮应使用中文名: %#v", button)
	}
	if button.Command != "穿戴 1" {
		t.Fatalf("穿戴按钮不应发送装备内部编号: %#v", button)
	}
	if strings.Contains(message.Text, "eq-1") {
		t.Fatalf("装备背包不应展示装备内部编号: %q", message.Text)
	}
}

func TestEquipCommandAcceptsBagNumber(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "equip-by-number", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	if err := db.Create(&models.EquipmentTemplateConfig{Key: "equipment_01", Name: "原野短杖", Slot: "weapon", Rarity: "common", RequiredLevel: 1, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PlayerEquipment{ID: "eq-hidden", AccountID: accountID, TemplateKey: "equipment_01", Rarity: "common"}).Error; err != nil {
		t.Fatal(err)
	}

	event.Text = "穿戴 1"
	message, err := handleEquip(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "原野短杖") {
		t.Fatalf("按背包序号穿戴应提示中文名: %q", message.Text)
	}
	if strings.Contains(message.Text, "eq-hidden") || strings.Contains(message.Text, "equipment_01") {
		t.Fatalf("穿戴结果不应展示内部编号: %q", message.Text)
	}
}

func TestAdventureKeyboardFitsOfficialRowLimit(t *testing.T) {
	buttons := make([]core.KeyboardButton, 0, 7)
	for i := 0; i < 7; i++ {
		buttons = append(buttons, core.KeyboardButton{Label: "探索 区域", Command: "探索 区域"})
	}
	rows := chunkKeyboardButtons(buttons, 2)
	if len(rows) != 4 {
		t.Fatalf("按钮应按每行 2 个拆开，得到 4 行，实际 %d 行", len(rows))
	}
	if len(rows[0]) != 2 || len(rows[3]) != 1 {
		t.Fatalf("最后一行应容纳剩余按钮: %#v", rows)
	}
}
