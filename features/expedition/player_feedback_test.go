package expedition

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

func TestPetBusyMessageUsesStatusSpecificNextStep(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("feedback-status", "player", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)

	for _, test := range []struct {
		status string
		want   []string
	}{
		{gameplay.PetStatusResting, []string{"正在休养", "找回"}},
		{"远征", []string{"正在远征", "领取"}},
		{"探索", []string{"正在探索", "当前遭遇"}},
		{"探索战斗", []string{"地图战斗", "普攻"}},
		{"首领战斗", []string{"地图首领", "撤退"}},
		{"钓鱼", []string{"正在钓鱼", "收竿"}},
		{"打工", []string{"正在打工", "完成打工"}},
	} {
		if err := db.Model(&models.PetProfile{}).Where("account_id = ?", accountID).Update("status", test.status).Error; err != nil {
			t.Fatal(err)
		}
		message, busy := petBusyMessage(context.Background(), service, accountID)
		if !busy {
			t.Fatalf("状态 %q 应被识别为忙碌", test.status)
		}
		for _, want := range test.want {
			if !strings.Contains(message.Text, want) {
				t.Fatalf("状态 %q 缺少 %q: %q", test.status, want, message.Text)
			}
		}
	}
}

func TestCompanionBusyStatusDoesNotUseCooldownCopy(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("feedback-companion", "player", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	if err := db.Model(&models.PetProfile{}).Where("account_id = ?", accountID).Update("status", "远征").Error; err != nil {
		t.Fatal(err)
	}
	event.Text = "摸头"
	message, err := handleCompanion(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "正在远征") || !strings.Contains(message.Text, "领取") || strings.Contains(message.Text, "还想休息一会儿") {
		t.Fatalf("忙碌陪伴提示不正确: %q", message.Text)
	}
}

func TestEquipBelowRequiredLevelExplainsRequirement(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("feedback-equip-level", "player", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	if err := db.Create(&models.EquipmentTemplateConfig{Key: "equipment_03", Name: "晨露罗盘", Slot: "treasure", Rarity: "common", RequiredLevel: 3, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PlayerEquipment{ID: "eq-compass", AccountID: accountID, TemplateKey: "equipment_03", Rarity: "common"}).Error; err != nil {
		t.Fatal(err)
	}

	event.Text = "穿戴 1"
	message, err := handleEquip(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message.Text, "这次操作没有完成") {
		t.Fatalf("等级不足不应伪装成技术故障: %q", message.Text)
	}
	for _, want := range []string{"冒险等级 3", "当前等级 1"} {
		if !strings.Contains(message.Text, want) {
			t.Fatalf("穿戴失败应说明等级要求，缺少 %q: %q", want, message.Text)
		}
	}
}

func TestEquipmentBagShowsRequiredLevelWhenTooLow(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("feedback-equip-bag", "player", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	if err := db.Create(&[]models.EquipmentTemplateConfig{
		{Key: "equipment_03", Name: "晨露罗盘", Slot: "treasure", Rarity: "common", RequiredLevel: 3, Enabled: true},
		{Key: "equipment_01", Name: "原野短杖", Slot: "weapon", Rarity: "common", RequiredLevel: 1, Enabled: true},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]models.PlayerEquipment{
		{ID: "eq-staff", AccountID: accountID, TemplateKey: "equipment_01", Rarity: "common"},
		{ID: "eq-compass", AccountID: accountID, TemplateKey: "equipment_03", Rarity: "common"},
	}).Error; err != nil {
		t.Fatal(err)
	}

	event.Text = "装备背包"
	message, err := handleEquipment(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "晨露罗盘") || !strings.Contains(message.Text, "需等级3") {
		t.Fatalf("等级不足的装备应在背包中标明需求等级: %q", message.Text)
	}
	if strings.Contains(message.Text, "原野短杖") && strings.Contains(message.Text, "需等级1") {
		t.Fatalf("已满足等级的装备不必重复标注需求等级: %q", message.Text)
	}
	if message.Keyboard == nil {
		t.Fatal("可穿戴装备应保留穿戴按钮")
	}
	joined := ""
	for _, row := range message.Keyboard.Rows {
		for _, button := range row {
			joined += button.Label + " " + button.Command + "\n"
		}
	}
	if !strings.Contains(joined, "原野短杖") {
		t.Fatalf("可穿戴装备应提供穿戴按钮: %q", joined)
	}
	if strings.Contains(joined, "晨露罗盘") {
		t.Fatalf("等级不足的装备不应提供穿戴按钮: %q", joined)
	}
}

func TestEquipmentBusinessErrorsExplainTheProblem(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{ErrEquipmentLocked, "锁定"},
		{ErrRecipeLocked, "蓝图"},
		{ErrEquipmentEquipped, "卸下"},
	} {
		message, err := adventureBusinessError(test.err)
		if err != nil {
			t.Fatalf("%v 不应落到技术故障: %v", test.err, err)
		}
		if strings.Contains(message.Text, "这次操作没有完成") || !strings.Contains(message.Text, test.want) {
			t.Fatalf("%v 应说明原因，得到 %q", test.err, message.Text)
		}
	}
}

func TestCombatSkillCooldownShowsReasonAndPanel(t *testing.T) {
	message, err := adventureBusinessError(&CombatSkillCooldownError{SkillName: "熄爪突袭", RemainingTurns: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Text, "熄爪突袭") || !strings.Contains(message.Text, "2 回合") {
		t.Fatalf("技能冷却原因不清楚: %q", message.Text)
	}

	panel := formatAdventureCombatMessage(adventureCombatView{
		Title: "萤草坡 · 第 2 回合", PlayerName: "光芽", MonsterName: "萤绒团",
		PlayerHP: 100, MonsterHP: 80, SkillNames: []string{"熄爪突袭", "光束"},
		SkillCooldowns: map[string]int{"熄爪突袭": 1},
	})
	if !strings.Contains(panel.Text, "技能冷却：熄爪突袭（还需 1 回合）") {
		t.Fatalf("战斗面板未展示技能冷却: %q", panel.Text)
	}

	var cooldowns map[string]int
	if err := json.Unmarshal([]byte(decrementCooldowns(`{"skill":2}`)), &cooldowns); err != nil {
		t.Fatal(err)
	}
	if cooldowns["skill"] != 1 {
		t.Fatalf("技能冷却未按回合递减: %#v", cooldowns)
	}
}

func TestTreatInsufficientFundsShowsCostBalanceAndRepeatableSources(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("feedback-treat", "player", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	if err := db.Model(&models.PetProfile{}).Where("account_id = ?", accountID).Updates(map[string]any{"status": "受伤", "health": 1, "health_max": 100}).Error; err != nil {
		t.Fatal(err)
	}
	var wallet models.PlayerWallet
	lookup := db.Where("account_id = ? AND currency_key = ?", accountID, gameplay.DefaultCurrencyKey).First(&wallet)
	if lookup.Error != nil {
		wallet = models.PlayerWallet{AccountID: accountID, CurrencyKey: gameplay.DefaultCurrencyKey}
	}
	wallet.Balance = 20
	if err := db.Save(&wallet).Error; err != nil {
		t.Fatal(err)
	}

	event.Text = "治疗"
	message, err := handleTreatPet(context.Background(), event, service)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"治疗费用不足", "本次治疗需要：100", "当前余额：20", "还差：80", "打工", "出售 物品名*数量"} {
		if !strings.Contains(message.Text, want) {
			t.Fatalf("治疗提示缺少 %q: %q", want, message.Text)
		}
	}
	if strings.Contains(message.Text, "签到") || strings.Contains(message.Text, "远征") {
		t.Fatalf("治疗提示不应推荐签到或远征: %q", message.Text)
	}
}

func TestConfiguredExpeditionClaimDoesNotCreditPrimaryCurrency(t *testing.T) {
	service, db, now := newTestService(t)
	event := oneBotEvent("feedback-expedition", "player", "领养 光芽兽")
	if _, err := handleAdopt(context.Background(), event, service); err != nil {
		t.Fatal(err)
	}
	accountID := accountIDForTest(t, service, event)
	var pet models.PetProfile
	if err := db.Where("account_id = ?", accountID).First(&pet).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err := json.Marshal(AdventureExpeditionSnapshot{Config: models.AdventureExpeditionConfig{Name: "测试远征"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AdventureExpeditionRun{
		ID: "feedback-expedition-run", AccountID: accountID, PetID: pet.ID, Status: "running", SnapshotJSON: string(snapshot),
		StartedAt: now.Add(-2 * time.Hour), EndsAt: now.Add(-time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	before, err := gameplay.NewWalletService(db).Balance(context.Background(), accountID, gameplay.DefaultCurrencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ClaimAdventureExpedition(context.Background(), accountID); err != nil {
		t.Fatal(err)
	}
	after, err := gameplay.NewWalletService(db).Balance(context.Background(), accountID, gameplay.DefaultCurrencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("未配置货币奖励的远征不应增加主货币: before=%d after=%d", before, after)
	}
}
