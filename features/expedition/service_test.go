package expedition

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/core"
	"qq-pet-saas/models"
)

func newTestService(t *testing.T) (*Service, *gorm.DB, *time.Time) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	err = db.AutoMigrate(
		&models.PlayerAccount{}, &models.PlayerIdentity{}, &models.PetProfile{},
		&models.GlobalInventoryItem{}, &models.CompanionJournal{}, &models.ExpeditionRun{},
		&models.CodexEntry{}, &models.Community{}, &models.CommunityMember{},
		&models.ExpeditionSquad{}, &models.SquadMember{}, &models.IdentityBindToken{},
		&models.NotificationPreference{}, &models.CommunityBoss{}, &models.BossContribution{},
		&models.CommunityFacility{}, &models.SeasonVote{},
		&models.CommunityHelpRequest{}, &models.HelpGiftLog{},
		&models.PetBehaviorProfile{},
		&models.LiveEventConfig{}, &models.RewardTrackConfig{},
		&models.GrowthRoleConfig{}, &models.GrowthStanceConfig{},
		&models.PersonalityRuleConfig{}, &models.CodexCatalogConfig{},
	)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.Local)
	service := NewService(db)
	service.Now = func() time.Time { return now }
	service.TokenSource = func() (string, error) { return "ABC12345", nil }
	return service, db, &now
}

func TestConfiguredGrowthRulesDriveRoleStanceAndPersonality(t *testing.T) {
	service, db, now := newTestService(t)
	if err := db.Create(&models.GrowthRoleConfig{Name: "采集者", Description: "素材专家", Skill1: "辨识", Skill2: "采样", Skill3: "整理", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.GrowthStanceConfig{Name: "潜行", Description: "避开风险", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PersonalityRuleConfig{Name: "细心", Dimension: "care", MinThreshold: 1, Description: "一次照料即可形成", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "configured", "")
	account, _ := service.ResolveAccount(context.Background(), event)
	_, _ = service.Adopt(context.Background(), account.ID, "诺诺", "配置测试宠物")
	pet, err := service.SetRole(context.Background(), account.ID, "采集者")
	if err != nil || pet.Skills != "辨识、采样、整理" {
		t.Fatalf("configured role did not drive loadout: pet=%+v err=%v", pet, err)
	}
	if err = service.SetStance(context.Background(), account.ID, "潜行"); err != nil {
		t.Fatalf("configured stance was rejected: %v", err)
	}
	if _, _, err = service.RecordDaily(context.Background(), account.ID, "陪伴"); err != nil {
		t.Fatal(err)
	}
	*now = now.AddDate(0, 0, 1)
	var behavior models.PetBehaviorProfile
	if err = db.First(&behavior, "account_id = ?", account.ID).Error; err != nil || behavior.Trait != "细心" {
		t.Fatalf("configured personality rule did not apply: %+v err=%v", behavior, err)
	}
}

func TestCommunityBossAggregatesPositiveContributionWithoutPlayerLoss(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "42", "首领 支援 10")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Adopt(context.Background(), account.ID, "诺诺", "诺诺"); err != nil {
		t.Fatal(err)
	}
	if err = service.AddInventory(context.Background(), account.ID, "调查记录", 10); err != nil {
		t.Fatal(err)
	}
	boss, damage, err := service.ChallengeBoss(context.Background(), event, account.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if damage <= 0 || boss.CurrentHP != boss.MaxHP-damage {
		t.Fatalf("unexpected boss result: boss=%#v damage=%d", boss, damage)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", account.ID).Error; err != nil {
		t.Fatal(err)
	}
	if pet.Growth < 0 || pet.Readiness < 0 {
		t.Fatalf("cooperative PVE must not remove permanent progress: %#v", pet)
	}
}

func TestJoinSquadHonorsCommunityBoundary(t *testing.T) {
	service, _, _ := newTestService(t)
	leaderEvent := oneBotEvent("100", "leader", "")
	leader, _ := service.ResolveAccount(context.Background(), leaderEvent)
	squad, err := service.CreateSquad(context.Background(), leaderEvent, leader.ID, "星光队")
	if err != nil {
		t.Fatal(err)
	}
	memberEvent := oneBotEvent("100", "member", "")
	member, _ := service.ResolveAccount(context.Background(), memberEvent)
	if err = service.JoinSquad(context.Background(), memberEvent, member.ID, squad.Name); err != nil {
		t.Fatal(err)
	}
	otherEvent := oneBotEvent("200", "other", "")
	other, _ := service.ResolveAccount(context.Background(), otherEvent)
	if err = service.JoinSquad(context.Background(), otherEvent, other.ID, squad.Name); err == nil {
		t.Fatal("expected squad to be isolated to its community")
	}
}

func TestSeasonVoteCanChangeWithoutResettingPermanentCodex(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "42", "赛季 投票 2")
	account, _ := service.ResolveAccount(context.Background(), event)
	entry := models.CodexEntry{AccountID: account.ID, Category: "地区", EntryKey: "永久记录", Progress: 80}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}
	season := service.CurrentSeason()
	if err := service.VoteSeason(context.Background(), event, account.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := service.VoteSeason(context.Background(), event, account.ID, 1); err != nil {
		t.Fatal(err)
	}
	var votes int64
	db.Model(&models.SeasonVote{}).Where("season_key = ? AND account_id = ?", season.Key, account.ID).Count(&votes)
	if votes != 1 {
		t.Fatalf("expected vote update instead of duplicate, got %d", votes)
	}
	var preserved models.CodexEntry
	if err := db.First(&preserved, "account_id = ? AND entry_key = ?", account.ID, "永久记录").Error; err != nil || preserved.Progress != 80 {
		t.Fatalf("season voting changed permanent codex: %#v err=%v", preserved, err)
	}
}

func TestFacilityUpgradeConsumesCommunityMaterials(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "42", "设施 升级 研究站")
	account, _ := service.ResolveAccount(context.Background(), event)
	community, _ := service.GetCommunity(context.Background(), event, account.ID)
	db.Model(&models.Community{}).Where("id = ?", community.ID).Update("materials", 100)
	facility, err := service.UpgradeFacility(context.Background(), event, account.ID, "研究站")
	if err != nil {
		t.Fatal(err)
	}
	if facility.Level != 2 {
		t.Fatalf("expected level 2 facility, got %#v", facility)
	}
	db.First(community, "id = ?", community.ID)
	if community.Materials != 0 {
		t.Fatalf("expected upgrade cost consumed, materials=%d", community.Materials)
	}
}

func TestCommunityHelpRequestTransfersLimitedGiftWithoutFreeTrading(t *testing.T) {
	service, db, _ := newTestService(t)
	requesterEvent := oneBotEvent("100", "requester", "求助 木材 5")
	requester, _ := service.ResolveAccount(context.Background(), requesterEvent)
	request, err := service.CreateHelpRequest(context.Background(), requesterEvent, requester.ID, "木材", 5)
	if err != nil {
		t.Fatal(err)
	}
	donorEvent := oneBotEvent("100", "donor", "支援 "+request.Code+" 3")
	donor, _ := service.ResolveAccount(context.Background(), donorEvent)
	if err = service.AddInventory(context.Background(), donor.ID, "木材", 3); err != nil {
		t.Fatal(err)
	}
	updated, err := service.SupportHelpRequest(context.Background(), donorEvent, donor.ID, request.Code, 3)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Fulfilled != 3 || updated.Status != "open" {
		t.Fatalf("unexpected request state: %#v", updated)
	}
	var received models.GlobalInventoryItem
	if err = db.Where("account_id = ? AND item_name = ?", requester.ID, "木材").First(&received).Error; err != nil || received.Quantity != 3 {
		t.Fatalf("requester did not receive limited gift: %#v err=%v", received, err)
	}
	otherEvent := oneBotEvent("200", "other", "")
	other, _ := service.ResolveAccount(context.Background(), otherEvent)
	if err = service.AddInventory(context.Background(), other.ID, "木材", 1); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SupportHelpRequest(context.Background(), otherEvent, other.ID, request.Code, 1); err == nil {
		t.Fatal("help request must not cross community boundary")
	}
}

func TestPersonalityEmergesFromRepeatedCareBehavior(t *testing.T) {
	service, db, now := newTestService(t)
	event := oneBotEvent("100", "42", "今日")
	account, _ := service.ResolveAccount(context.Background(), event)
	_, _ = service.Adopt(context.Background(), account.ID, "诺诺", "诺诺")
	for day := 0; day < 3; day++ {
		if _, _, err := service.RecordDaily(context.Background(), account.ID, "陪伴"); err != nil {
			t.Fatal(err)
		}
		*now = now.AddDate(0, 0, 1)
	}
	var behavior models.PetBehaviorProfile
	if err := db.First(&behavior, "account_id = ?", account.ID).Error; err != nil {
		t.Fatal(err)
	}
	if behavior.Trait != "温柔" {
		t.Fatalf("expected behavior-derived trait, got %#v", behavior)
	}
}

func TestSetRoleAssignsDeterministicSkillLoadout(t *testing.T) {
	service, _, _ := newTestService(t)
	event := oneBotEvent("100", "42", "定位 守护者")
	account, _ := service.ResolveAccount(context.Background(), event)
	_, _ = service.Adopt(context.Background(), account.ID, "诺诺", "诺诺")
	pet, err := service.SetRole(context.Background(), account.ID, "守护者")
	if err != nil {
		t.Fatal(err)
	}
	if pet.Role != "守护者" || !strings.Contains(pet.Skills, "护盾") {
		t.Fatalf("unexpected role loadout: %#v", pet)
	}
}

func TestCurrentSeasonUsesPublishedLiveEventConfiguration(t *testing.T) {
	service, db, now := newTestService(t)
	event := models.LiveEventConfig{
		Key: "summer-ruins", Name: "夏日遗迹", Region: "遗迹", Active: true,
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(7 * 24 * time.Hour),
		StoryChoices: `["修复灯塔","救助旅伴","调查深井"]`,
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatal(err)
	}
	season := service.CurrentSeason()
	if season.Key != event.Key || season.Name != event.Name || season.Choices[1] != "救助旅伴" {
		t.Fatalf("published event not used: %#v", season)
	}
}

func oneBotEvent(group, actor, text string) core.InboundEvent {
	return core.InboundEvent{Platform: core.PlatformOneBot, SceneType: core.SceneGroup, AppID: "legacy", SpaceID: group, RoomID: group, ActorID: actor, Text: text}
}

func officialGroupEvent(group, actor, text string) core.InboundEvent {
	return core.InboundEvent{Platform: core.PlatformQQGroup, SceneType: core.SceneGroup, AppID: "official", SpaceID: group, RoomID: group, ActorID: actor, Text: text}
}

func TestResolveAccountSharesOneBotIdentityAcrossGroups(t *testing.T) {
	service, _, _ := newTestService(t)
	first, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "42", "状态"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ResolveAccount(context.Background(), oneBotEvent("200", "42", "状态"))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected shared account, got %s and %s", first.ID, second.ID)
	}
}

func TestExpeditionClaimIsAutomaticAndIdempotent(t *testing.T) {
	service, db, now := newTestService(t)
	event := oneBotEvent("100", "42", "远征 1")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Adopt(context.Background(), account.ID, "诺诺", "诺诺"); err != nil {
		t.Fatal(err)
	}
	run, err := service.StartExpedition(context.Background(), account.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	*now = run.EndsAt.Add(time.Second)
	result, err := service.ClaimExpedition(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Records != 6 || result.Growth != 4 {
		t.Fatalf("unexpected deterministic rewards: %#v", result)
	}
	if _, err = service.ClaimExpedition(context.Background(), account.ID); err != ErrNothingToClaim {
		t.Fatalf("expected idempotent second claim rejection, got %v", err)
	}
	var item models.GlobalInventoryItem
	if err = db.Where("account_id = ? AND item_name = ?", account.ID, "调查记录").First(&item).Error; err != nil {
		t.Fatal(err)
	}
	if item.Quantity != 6 {
		t.Fatalf("expected one reward grant, got %d", item.Quantity)
	}
}

func TestCommunityProgressIsIsolated(t *testing.T) {
	service, _, _ := newTestService(t)
	account, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "42", "营地"))
	if err != nil {
		t.Fatal(err)
	}
	if err = service.AddInventory(context.Background(), account.ID, "木材", 40); err != nil {
		t.Fatal(err)
	}
	first, err := service.Contribute(context.Background(), oneBotEvent("100", "42", ""), account.ID, "木材", 20)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.GetCommunity(context.Background(), oneBotEvent("200", "42", ""), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Materials != 20 || second.Materials != 0 {
		t.Fatalf("community progress leaked: first=%d second=%d", first.Materials, second.Materials)
	}
}

func TestBindingTokenMergesOfficialIdentityOnce(t *testing.T) {
	service, db, _ := newTestService(t)
	sourceEvent := oneBotEvent("100", "42", "生成绑定码")
	source, err := service.ResolveAccount(context.Background(), sourceEvent)
	if err != nil {
		t.Fatal(err)
	}
	token, err := service.GenerateBindToken(context.Background(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	targetEvent := officialGroupEvent("group-openid", "member-openid", "绑定 "+token)
	target, err := service.ResolveAccount(context.Background(), targetEvent)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := service.RedeemBindToken(context.Background(), targetEvent, token)
	if err != nil {
		t.Fatal(err)
	}
	if merged.ID != source.ID {
		t.Fatalf("expected merged account %s, got %s", source.ID, merged.ID)
	}
	var orphan int64
	db.Model(&models.PlayerAccount{}).Where("id = ?", target.ID).Count(&orphan)
	if orphan != 0 {
		t.Fatalf("expected empty target account to be removed, count=%d", orphan)
	}
	if _, err = service.RedeemBindToken(context.Background(), targetEvent, token); err != ErrInvalidBindToken {
		t.Fatalf("expected token to be single-use, got %v", err)
	}
}

func TestBindingRejectsTargetIdentityWithIndependentProgress(t *testing.T) {
	service, _, _ := newTestService(t)
	sourceEvent := oneBotEvent("100", "42", "生成绑定码")
	source, _ := service.ResolveAccount(context.Background(), sourceEvent)
	token, _ := service.GenerateBindToken(context.Background(), source.ID)
	targetEvent := officialGroupEvent("group-openid", "member-openid", "")
	target, _ := service.ResolveAccount(context.Background(), targetEvent)
	_, _ = service.Adopt(context.Background(), target.ID, "菀菀", "菀菀")
	if _, err := service.RedeemBindToken(context.Background(), targetEvent, token); err != ErrBindConflict {
		t.Fatalf("expected independent progress conflict, got %v", err)
	}
}
