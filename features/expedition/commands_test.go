package expedition

import (
	"context"
	"strings"
	"testing"
	"time"

	"qq-pet-saas/core"
	"qq-pet-saas/models"
)

func TestTextCommandsSupportAdoptAndExpeditionFlow(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "状态")
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "领养") {
		t.Fatalf("unexpected initial status: handled=%v err=%v message=%q", handled, err, message.Text)
	}
	event.Text = "领养 诺诺"
	message, _, err = router.Route(context.Background(), event)
	if err != nil || !strings.Contains(message.Text, "领养成功") {
		t.Fatalf("unexpected adopt response: err=%v message=%q", err, message.Text)
	}
	event.Text = "远征 2"
	message, _, err = router.Route(context.Background(), event)
	if err != nil || !strings.Contains(message.Text, "遗迹调查已开始") || !strings.Contains(message.Text, "远征状态") {
		t.Fatalf("unexpected expedition response: err=%v message=%q", err, message.Text)
	}
}

func TestExpeditionMenuProvidesOptionalButtonsWithTextFallback(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "远征")
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled {
		t.Fatalf("menu route failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(message.Text, "远征 1/2/3") || message.Keyboard == nil || len(message.Keyboard.Rows) != 1 || len(message.Keyboard.Rows[0]) != 3 {
		t.Fatalf("expected text fallback and three optional buttons: %#v", message)
	}
}

func TestDailyJournalDoesNotResetAndOnlyRewardsOncePerDay(t *testing.T) {
	service, db, now := newTestService(t)
	account, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "42", "今日"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Adopt(context.Background(), account.ID, "诺诺", "诺诺"); err != nil {
		t.Fatal(err)
	}
	first, rewarded, err := service.RecordDaily(context.Background(), account.ID, "陪伴")
	if err != nil || !rewarded || first != 1 {
		t.Fatalf("unexpected first journal result: streak=%d rewarded=%v err=%v", first, rewarded, err)
	}
	_, rewarded, err = service.RecordDaily(context.Background(), account.ID, "陪伴")
	if err != nil || rewarded {
		t.Fatalf("same day must not reward twice: rewarded=%v err=%v", rewarded, err)
	}
	*now = now.AddDate(0, 0, 3)
	streak, rewarded, err := service.RecordDaily(context.Background(), account.ID, "陪伴")
	if err != nil || !rewarded || streak != 2 {
		t.Fatalf("rolling journal must retain earlier day: streak=%d rewarded=%v err=%v", streak, rewarded, err)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", account.ID).Error; err != nil {
		t.Fatal(err)
	}
	if pet.Affection != 2 {
		t.Fatalf("expected exactly two daily rewards, got affection=%d", pet.Affection)
	}
}

func TestValidatePlayerNameRejectsContactAndControlContent(t *testing.T) {
	for _, candidate := range []string{"a", "联系QQ123", "https://example.com", "坏\n名字"} {
		if err := ValidatePlayerName(candidate); err == nil {
			t.Fatalf("expected %q to be rejected", candidate)
		}
	}
	if err := ValidatePlayerName("星光小队"); err != nil {
		t.Fatalf("expected ordinary name to pass: %v", err)
	}
}

func TestCreateSquadIsCommunityScoped(t *testing.T) {
	service, _, _ := newTestService(t)
	event := oneBotEvent("100", "42", "小队 创建 星光队")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	squad, err := service.CreateSquad(context.Background(), event, account.ID, "星光队")
	if err != nil {
		t.Fatal(err)
	}
	if squad.MaxMembers != 12 || squad.CommunityID != communityID(event) {
		t.Fatalf("unexpected squad: %#v", squad)
	}
}

func TestDeleteAccountRemovesGlobalProgressAndIdentities(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "42", "确认删除我的数据")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Adopt(context.Background(), account.ID, "诺诺", "诺诺"); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.PetBehaviorProfile{AccountID: account.ID, Care: 3, Trait: "温柔"}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.SeasonVote{SeasonKey: "S001", CommunityID: "community", AccountID: account.ID, Choice: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err = service.DeleteAccount(context.Background(), account.ID); err != nil {
		t.Fatal(err)
	}
	var identities int64
	var pets int64
	var behaviors int64
	var votes int64
	db.Model(&models.PlayerIdentity{}).Where("account_id = ?", account.ID).Count(&identities)
	db.Model(&models.PetProfile{}).Where("account_id = ?", account.ID).Count(&pets)
	db.Model(&models.PetBehaviorProfile{}).Where("account_id = ?", account.ID).Count(&behaviors)
	db.Model(&models.SeasonVote{}).Where("account_id = ?", account.ID).Count(&votes)
	if identities != 0 || pets != 0 || behaviors != 0 || votes != 0 {
		t.Fatalf("account data remains: identities=%d pets=%d behaviors=%d votes=%d", identities, pets, behaviors, votes)
	}
}

func TestExpeditionStatusReportsRemainingTime(t *testing.T) {
	service, _, now := newTestService(t)
	event := oneBotEvent("100", "42", "远征状态")
	account, _ := service.ResolveAccount(context.Background(), event)
	_, _ = service.Adopt(context.Background(), account.ID, "诺诺", "诺诺")
	run, _ := service.StartExpedition(context.Background(), account.ID, 1)
	*now = now.Add(5 * time.Minute)
	text, err := service.FormatExpeditionStatus(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, run.Name) || !strings.Contains(text, "5分钟") {
		t.Fatalf("unexpected status text: %q", text)
	}
}

func TestBossCommandSupportsStatusAndAsynchronousSupport(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "领养 诺诺")
	_, _, _ = router.Route(context.Background(), event)
	account, _ := service.ResolveAccount(context.Background(), event)
	if err := service.AddInventory(context.Background(), account.ID, "调查记录", 10); err != nil {
		t.Fatal(err)
	}
	event.Text = "首领"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "迷雾守卫") {
		t.Fatalf("unexpected boss status: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	event.Text = "首领 支援 10"
	message, _, err = router.Route(context.Background(), event)
	if err != nil || !strings.Contains(message.Text, "协作支援完成") {
		t.Fatalf("unexpected boss support: err=%v text=%q", err, message.Text)
	}
}

func TestSeasonAndFacilityCommandsHaveTextOnlyFlows(t *testing.T) {
	service, db, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "赛季")
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "回复“赛季 投票 1/2/3”") {
		t.Fatalf("unexpected season text: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	account, _ := service.ResolveAccount(context.Background(), event)
	community, _ := service.GetCommunity(context.Background(), event, account.ID)
	db.Model(&models.Community{}).Where("id = ?", community.ID).Update("materials", 100)
	event.Text = "设施 升级 研究站"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "研究站已升级到 Lv.2") {
		t.Fatalf("unexpected facility text: handled=%v err=%v text=%q", handled, err, message.Text)
	}
}

func TestHelpRequestCommandsUseCodeBasedLimitedSupport(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "requester", "求助 木材 5")
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "求助已发布") || !strings.Contains(message.Text, "支援 ") {
		t.Fatalf("unexpected help request response: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	event = oneBotEvent("100", "viewer", "求助列表")
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "木材 0/5") {
		t.Fatalf("unexpected help list response: handled=%v err=%v text=%q", handled, err, message.Text)
	}
}

func TestRoleCommandShowsDeterministicThreeSkillLoadout(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "领养 诺诺")
	_, _, _ = router.Route(context.Background(), event)
	event.Text = "定位 守护者"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "护盾、警戒、稳固") {
		t.Fatalf("unexpected role response: handled=%v err=%v text=%q", handled, err, message.Text)
	}
}

func TestInvalidTacticalInputReturnsActionableText(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "42", "领养 诺诺")
	_, _, _ = router.Route(context.Background(), event)
	event.Text = "编队 乱来"
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "攻击、守护、支援或探索") {
		t.Fatalf("expected actionable validation response: handled=%v err=%v text=%q", handled, err, message.Text)
	}
}

func TestRetiredCommandsAlwaysReturnModernReplacementGuide(t *testing.T) {
	service, _, _ := newTestService(t)
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"钓鱼", "抽奖", "宠物交易"} {
		message, handled, err := router.Route(context.Background(), oneBotEvent("100", "42", command))
		if err != nil || !handled || (!strings.Contains(message.Text, "远征") && !strings.Contains(message.Text, "求助")) {
			t.Fatalf("retired command %q did not return a replacement guide: handled=%v err=%v text=%q", command, handled, err, message.Text)
		}
	}
}

func TestRoleAndStanceMenusReadConfiguredRules(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Create(&models.GrowthRoleConfig{Name: "采集者", Description: "专注素材调查", Skill1: "辨识", Skill2: "采样", Skill3: "整理", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.GrowthStanceConfig{Name: "潜行", Description: "避开风险", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	router := core.NewCommandRouter()
	if err := RegisterCommands(router.Register, func() *Service { return service }); err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "configured-menu", "定位")
	message, handled, err := router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "采集者：专注素材调查") || strings.Contains(message.Text, "探索者：") {
		t.Fatalf("role menu did not use configured rules: handled=%v err=%v text=%q", handled, err, message.Text)
	}
	event.Text = "编队"
	message, handled, err = router.Route(context.Background(), event)
	if err != nil || !handled || !strings.Contains(message.Text, "潜行：避开风险") || strings.Contains(message.Text, "攻击：") {
		t.Fatalf("stance menu did not use configured rules: handled=%v err=%v text=%q", handled, err, message.Text)
	}
}
