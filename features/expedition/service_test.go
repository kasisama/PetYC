package expedition

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/config"
	"qq-pet-saas/core"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/gameplayrules"
	"qq-pet-saas/models"
)

func newTestService(t *testing.T) (*Service, *gorm.DB, *time.Time) {
	t.Helper()
	if len(config.Core.InitialPets) == 0 {
		config.Core.InitialPets = []string{"光芽兽"}
	}
	dsn := fmt.Sprintf("file:expedition-test-%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	err = db.AutoMigrate(
		&models.SystemConfig{}, &models.PlayerAccount{}, &models.PlayerIdentity{}, &models.PetProfile{},
		&models.GlobalInventoryItem{}, &models.PlayerWallet{}, &models.WalletLedger{}, &models.CompanionJournal{}, &models.CompanionActionDaily{}, &models.ActivityRun{}, &models.ItemUseRecord{}, &models.ExpeditionTemplateConfig{}, &models.ExpeditionRun{},
		&models.CodexEntry{}, &models.Community{}, &models.CommunityMember{},
		&models.ExpeditionSquad{}, &models.SquadMember{}, &models.IdentityBindToken{},
		&models.NotificationPreference{}, &models.NotificationJob{}, &models.CommunityBoss{}, &models.BossContribution{},
		&models.CommunityFacility{}, &models.SeasonVote{},
		&models.CommunityHelpRequest{}, &models.HelpGiftLog{}, &models.HelpGiftDailyQuota{},
		&models.PetBehaviorProfile{},
		&models.LiveEventConfig{}, &models.RewardTrackConfig{}, &models.EventProgress{}, &models.EventProgressGrant{}, &models.EventRewardClaim{},
		&models.ChanceGameConfig{}, &models.ChanceRewardConfig{}, &models.ChanceDailyState{}, &models.ChancePlayerState{}, &models.ChanceOutcome{}, &models.FishingRun{}, &models.BattleRecord{}, &models.TradeOffer{}, &models.TradeAudit{},
		&models.GrowthRoleConfig{}, &models.GrowthStanceConfig{},
		&models.PersonalityRuleConfig{}, &models.CodexCatalogConfig{},
		&models.PetSpeciesConfig{}, &models.CheckinRewardConfig{},
		&models.PetEvolutionRuleConfig{}, &models.PetEvolutionCostConfig{}, &models.PetSkillUnlockConfig{}, &models.AdventureLevelConfig{},
		&models.ItemConfig{}, &models.ShopItemConfig{},
		&models.AdventureMapConfig{}, &models.AdventureZoneConfig{}, &models.AdventureZonePrerequisiteConfig{},
		&models.AdventureObjectiveConfig{}, &models.AdventureMonsterConfig{}, &models.AdventureSkillConfig{},
		&models.AdventureMonsterSkillConfig{}, &models.AdventureEncounterConfig{}, &models.AdventureEncounterEffectConfig{}, &models.AdventureLootPoolConfig{},
		&models.AdventureLootEntryConfig{}, &models.CurrencyConfig{},
		&models.AdventureShopItemConfig{}, &models.AdventureExpeditionConfig{}, &models.AdventureBossConfig{},
		&models.AdventureBossRewardTierConfig{}, &models.EquipmentTemplateConfig{}, &models.EquipmentAffixConfig{},
		&models.EquipmentRecipeConfig{}, &models.EquipmentRecipeMaterialConfig{}, &models.LiveEventChoiceConfig{},
		&models.LiveEventExpeditionSourceConfig{}, &models.PlayerAdventureProgress{}, &models.PlayerZoneProgress{},
		&models.PlayerObjectiveProgress{}, &models.AdventureExplorationSession{}, &models.AdventureCombatSession{},
		&models.AdventureCombatTurn{}, &models.PlayerEquipment{}, &models.PlayerBlueprintProgress{},
		&models.AdventureShopPurchase{},
		&models.AdventureExpeditionRun{}, &models.AdventureBossInstance{}, &models.AdventureBossContribution{},
		&models.AdventureBossRewardClaim{}, &models.EquipmentCraftRecord{},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range config.StarterPets() {
		species := models.PetSpeciesConfig{Key: name, Name: name, FamilyKey: name, Stage: "base", Adoptable: true, Archetype: "balanced"}
		if configured, ok := config.Pets[name]; ok {
			species.Health = configured.Health
			species.HealthMax = configured.HealthMax
			species.Hunger = configured.Hunger
			species.HungerMax = configured.HungerMax
			species.Wisdom = configured.Wisdom
			species.Strength = configured.Strength
			species.Defense = configured.Defense
			species.FavoriteFood = configured.FavoriteFood
			species.FavoriteGift = configured.FavoriteGift
		}
		if err = db.Create(&species).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Exec(`CREATE UNIQUE INDEX idx_expedition_one_running ON expedition_runs(account_id, pet_id) WHERE status = 'running'`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`CREATE UNIQUE INDEX idx_activity_one_running ON activity_runs(account_id, pet_id) WHERE status = 'running'`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`CREATE UNIQUE INDEX idx_fishing_one_running ON fishing_runs(account_id, pet_id) WHERE status = 'running'`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`CREATE UNIQUE INDEX idx_adventure_exploration_one_active ON adventure_exploration_sessions(account_id, pet_id) WHERE status = 'active'`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`CREATE UNIQUE INDEX idx_adventure_combat_one_active ON adventure_combat_sessions(account_id, pet_id) WHERE status = 'active'`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`CREATE UNIQUE INDEX idx_adventure_expedition_one_running ON adventure_expedition_runs(account_id, pet_id) WHERE status = 'running'`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`CREATE UNIQUE INDEX idx_player_equipment_one_slot ON player_equipments(equipped_pet_id, equipped_slot) WHERE equipped_pet_id <> '' AND equipped_slot <> ''`).Error; err != nil {
		t.Fatal(err)
	}
	if err = gameplayrules.EnsureDefaults(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.Local)
	// Legacy fixtures stay explicit in tests so production has no hidden expedition fallback.
	legacyTemplates := []models.ExpeditionTemplateConfig{
		{Tier: 1, Name: "林间巡查", Enabled: true, DurationMinutes: 10, HungerCost: 5, ReadinessCost: 5, RewardItem: "林地样本", RewardQuantity: 1, RewardRecords: 6, RewardGrowth: 4, CodexCategory: "生物", CodexEntry: "林间足迹", CodexProgress: 15},
		{Tier: 2, Name: "遗迹调查", Enabled: true, DurationMinutes: 120, HungerCost: 12, ReadinessCost: 15, RewardItem: "古代零件", RewardQuantity: 3, RewardRecords: 12, RewardGrowth: 10, CodexCategory: "遗迹", CodexEntry: "遗迹守卫", CodexProgress: 15},
		{Tier: 3, Name: "深层生态勘察", Enabled: true, DurationMinutes: 480, HungerCost: 20, ReadinessCost: 25, RewardItem: "生态样本", RewardQuantity: 2, RewardRecords: 25, RewardGrowth: 24, CodexCategory: "生态", CodexEntry: "深层生态", CodexProgress: 20},
	}
	if err = db.Create(&legacyTemplates).Error; err != nil {
		t.Fatal(err)
	}
	testEvent := models.LiveEventConfig{Key: "test-season", Name: "测试活动", Region: "测试区域", Active: true, StartsAt: now.Add(-24 * time.Hour), EndsAt: now.Add(30 * 24 * time.Hour), StoryChoices: `["共建增益","设施折扣","首领增益"]`}
	if err = db.Create(&testEvent).Error; err != nil {
		t.Fatal(err)
	}
	testChoices := []models.LiveEventChoiceConfig{
		{EventKey: testEvent.Key, ChoiceKey: "build", Label: "共建增益", EffectType: "community_material_gain_percent", EffectValue: 20, SortOrder: 10},
		{EventKey: testEvent.Key, ChoiceKey: "facility", Label: "设施折扣", EffectType: "facility_upgrade_cost_reduction_percent", EffectValue: 20, SortOrder: 20},
		{EventKey: testEvent.Key, ChoiceKey: "boss", Label: "首领增益", EffectType: "boss_damage_gain_percent", EffectValue: 20, SortOrder: 30},
	}
	if err = db.Create(&testChoices).Error; err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	service.Now = func() time.Time { return now }
	service.TokenSource = func() (string, error) { return "ABC12345", nil }
	return service, db, &now
}

func TestConcurrentStartExpeditionCreatesOnlyOneRunningRecord(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "runner", "远征 1")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽"); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, startErr := service.StartExpedition(context.Background(), account.ID, 1)
			results <- startErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	successes, rejected := 0, 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, ErrExpeditionActive):
			rejected++
		default:
			t.Fatalf("unexpected concurrent start error: %v", result)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("expected one start and one rejection, successes=%d rejected=%d", successes, rejected)
	}
	var running int64
	if err = db.Model(&models.ExpeditionRun{}).Where("account_id = ? AND status = ?", account.ID, "running").Count(&running).Error; err != nil || running != 1 {
		t.Fatalf("active expedition invariant failed: running=%d err=%v", running, err)
	}
}

func TestConfiguredExpeditionTemplateConsumesCostsAndUsesCatalogReward(t *testing.T) {
	service, db, now := newTestService(t)
	template := models.ExpeditionTemplateConfig{
		Tier: 4, Name: "星尘勘探", Enabled: true, DurationMinutes: 30,
		HungerCost: 10, ReadinessCost: 20, RequiredItem: "旧地图", RequiredQuantity: 2,
		RewardItem: "秘银", RewardQuantity: 1, RewardRecords: 10, RewardGrowth: 8, RewardCurrency: 12,
		CodexCategory: "生物", CodexEntry: "星尘足迹", CodexProgress: 10,
		StartImage: "expedition/start.png", EndImage: "expedition/end.png", Description: "追踪夜空落下的星尘。",
	}
	if err := db.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CodexCatalogConfig{Category: "生物", EntryKey: "星尘足迹", Region: "星野", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	event := oneBotEvent("100", "configured-runner", "远征 4")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽"); err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&models.PetProfile{}).Where("account_id = ?", account.ID).Updates(map[string]interface{}{
		"stance": "支援", "wisdom": 40, "strength": 20, "defense": 20,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err = service.AddInventory(context.Background(), account.ID, "旧地图", 2); err != nil {
		t.Fatal(err)
	}

	run, err := service.StartExpedition(context.Background(), account.ID, 4)
	if err != nil {
		t.Fatal(err)
	}
	if run.Name != "星尘勘探" || run.EndsAt.Sub(run.StartedAt) != 30*time.Minute || run.RewardRecords != 14 || run.RewardGrowth != 9 {
		t.Fatalf("unexpected configured run snapshot: %#v", run)
	}
	if run.StartImage != "expedition/start.png" || !strings.Contains(run.BonusText, "支援姿态") {
		t.Fatalf("expected configured image and stance bonus, got %#v", run)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", account.ID).Error; err != nil {
		t.Fatal(err)
	}
	if pet.Hunger != 90 || pet.Readiness != 80 || pet.Status != "远征" {
		t.Fatalf("unexpected expedition costs/status: %#v", pet)
	}
	var mapItem models.GlobalInventoryItem
	if err = db.First(&mapItem, "account_id = ? AND item_name = ?", account.ID, "旧地图").Error; err != nil {
		t.Fatal(err)
	}
	if mapItem.Quantity != 0 {
		t.Fatalf("expected required item to be consumed, got %d", mapItem.Quantity)
	}

	*now = run.EndsAt.Add(time.Second)
	result, err := service.ClaimExpedition(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Currency != 12 || result.CodexEntry != "星尘足迹" || result.Progress != 10 || result.Image != "expedition/end.png" {
		t.Fatalf("unexpected configured claim result: %#v", result)
	}
	var wallet models.PlayerWallet
	if err = db.First(&wallet, "account_id = ? AND currency_key = ?", account.ID, gameplay.DefaultCurrencyKey).Error; err != nil {
		t.Fatal(err)
	}
	if wallet.Balance != 12 {
		t.Fatalf("expected configured currency reward, got %d", wallet.Balance)
	}
	var ledger models.WalletLedger
	if err = db.First(&ledger, "account_id = ? AND reason = ?", account.ID, "expedition_reward").Error; err != nil {
		t.Fatal(err)
	}
	if ledger.Delta != 12 || ledger.ReferenceKey != run.ID {
		t.Fatalf("unexpected expedition wallet ledger: %#v", ledger)
	}
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
	_, _ = service.Adopt(context.Background(), account.ID, "光芽兽", "配置测试宠物")
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
	if _, err = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽"); err != nil {
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

func TestLeadingSeasonChoiceChangesCommunitySettlement(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("season-community", "voter", "赛季 投票 1")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.ItemConfig{Name: "木材", Status: "active", Type: "材料"}).Error; err != nil {
		t.Fatal(err)
	}
	if err = service.AddInventory(context.Background(), account.ID, "木材", 10); err != nil {
		t.Fatal(err)
	}
	if err = service.VoteSeason(context.Background(), event, account.ID, 1); err != nil {
		t.Fatal(err)
	}
	community, err := service.Contribute(context.Background(), event, account.ID, "木材", 10)
	if err != nil {
		t.Fatal(err)
	}
	if community.Materials != 12 {
		t.Fatalf("choice 1 did not increase contribution settlement: %d", community.Materials)
	}

	if err = service.VoteSeason(context.Background(), event, account.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&models.Community{}).Where("id = ?", community.ID).Update("materials", 80).Error; err != nil {
		t.Fatal(err)
	}
	facility, err := service.UpgradeFacility(context.Background(), event, account.ID, "研究站")
	if err != nil {
		t.Fatal(err)
	}
	if facility.Level != 2 {
		t.Fatalf("choice 2 discounted upgrade was not applied: %#v", facility)
	}
	if err = db.First(community, "id = ?", community.ID).Error; err != nil {
		t.Fatal(err)
	}
	if community.Materials != 0 {
		t.Fatalf("choice 2 should consume discounted 80 materials, left %d", community.Materials)
	}

	if err = service.VoteSeason(context.Background(), event, account.ID, 3); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽"); err != nil {
		t.Fatal(err)
	}
	if err = service.AddInventory(context.Background(), account.ID, "调查记录", 10); err != nil {
		t.Fatal(err)
	}
	_, damage, err := service.ChallengeBoss(context.Background(), event, account.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if damage != 36 {
		t.Fatalf("choice 3 did not increase boss contribution: %d", damage)
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

func TestConcurrentHelpSupportCannotOverfillOrLoseInventory(t *testing.T) {
	service, db, _ := newTestService(t)
	requesterEvent := oneBotEvent("100", "requester", "求助 木材 5")
	requester, err := service.ResolveAccount(context.Background(), requesterEvent)
	if err != nil {
		t.Fatal(err)
	}
	request, err := service.CreateHelpRequest(context.Background(), requesterEvent, requester.ID, "木材", 5)
	if err != nil {
		t.Fatal(err)
	}
	type donor struct {
		id    string
		event core.InboundEvent
	}
	donors := make([]donor, 0, 2)
	for index := 1; index <= 2; index++ {
		event := oneBotEvent("100", fmt.Sprintf("donor-%d", index), "")
		account, resolveErr := service.ResolveAccount(context.Background(), event)
		if resolveErr != nil {
			t.Fatal(resolveErr)
		}
		if addErr := service.AddInventory(context.Background(), account.ID, "木材", 4); addErr != nil {
			t.Fatal(addErr)
		}
		donors = append(donors, donor{id: account.ID, event: event})
	}

	start := make(chan struct{})
	results := make(chan error, len(donors))
	var wait sync.WaitGroup
	for _, current := range donors {
		current := current
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, supportErr := service.SupportHelpRequest(context.Background(), current.event, current.id, request.Code, 4)
			results <- supportErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	successes, rejected := 0, 0
	for result := range results {
		if result == nil {
			successes++
			continue
		}
		if strings.Contains(result.Error(), "还需要 1 个木材") {
			rejected++
			continue
		}
		t.Fatalf("unexpected concurrent support error: %v", result)
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("expected one support and one capacity rejection, successes=%d rejected=%d", successes, rejected)
	}

	var saved models.CommunityHelpRequest
	if err = db.First(&saved, "code = ?", request.Code).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Fulfilled != 4 || saved.Status != "open" {
		t.Fatalf("request progress was overwritten or overfilled: %#v", saved)
	}
	var received models.GlobalInventoryItem
	if err = db.First(&received, "account_id = ? AND item_name = ?", requester.ID, "木材").Error; err != nil || received.Quantity != 4 {
		t.Fatalf("requester inventory mismatch: %#v err=%v", received, err)
	}
	var donorTotal int64
	if err = db.Model(&models.GlobalInventoryItem{}).Where("account_id IN ? AND item_name = ?", []string{donors[0].id, donors[1].id}, "木材").Select("COALESCE(SUM(quantity), 0)").Scan(&donorTotal).Error; err != nil || donorTotal != 4 {
		t.Fatalf("donor inventory was lost or duplicated: total=%d err=%v", donorTotal, err)
	}
	var logs int64
	if err = db.Model(&models.HelpGiftLog{}).Where("request_code = ?", request.Code).Count(&logs).Error; err != nil || logs != 1 {
		t.Fatalf("support log mismatch: logs=%d err=%v", logs, err)
	}
}

func TestPersonalityEmergesFromRepeatedCareBehavior(t *testing.T) {
	service, db, now := newTestService(t)
	event := oneBotEvent("100", "42", "今日")
	account, _ := service.ResolveAccount(context.Background(), event)
	_, _ = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽")
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
	_, _ = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽")
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

func TestEventProgressAndRewardTrackAreIdempotent(t *testing.T) {
	service, db, now := newTestService(t)
	event := models.LiveEventConfig{
		Key: "forest-week", Name: "森林调查周", Region: "森林", Active: true,
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(7 * 24 * time.Hour),
		StoryChoices: `["记录线索","救助旅伴","修复营地"]`,
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]models.ItemConfig{
		{Key: "wood", Name: "木材", Status: "active"},
		{Key: "survey_log", Name: "调查记录", Status: "active"},
		{Key: "eco_sample", Name: "生态样本", Status: "active"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	rewards := []models.RewardTrackConfig{
		{EventKey: event.Key, Milestone: 10, RewardType: "item", RewardKey: "wood", RewardName: "木材", Quantity: 5},
		{EventKey: event.Key, Milestone: 10, RewardType: "item", RewardKey: "survey_log", RewardName: "调查记录", Quantity: 2},
	}
	if err := db.Create(&rewards).Error; err != nil {
		t.Fatal(err)
	}
	account, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "event-player", "活动"))
	if err != nil {
		t.Fatal(err)
	}

	progress, granted, err := service.AddEventProgress(context.Background(), account.ID, "expedition:run-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if progress != 10 || len(granted) != 2 {
		t.Fatalf("unexpected first event settlement: progress=%d rewards=%#v", progress, granted)
	}
	progress, granted, err = service.AddEventProgress(context.Background(), account.ID, "expedition:run-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if progress != 10 || len(granted) != 0 {
		t.Fatalf("duplicate source moved progress or rewarded again: progress=%d rewards=%#v", progress, granted)
	}
	var wood, records models.GlobalInventoryItem
	if err = db.First(&wood, "account_id = ? AND item_name = ?", account.ID, "木材").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.First(&records, "account_id = ? AND item_name = ?", account.ID, "调查记录").Error; err != nil {
		t.Fatal(err)
	}
	if wood.Quantity != 5 || records.Quantity != 2 {
		t.Fatalf("event rewards duplicated or missing: wood=%d records=%d", wood.Quantity, records.Quantity)
	}
	var grantCount, claimCount int64
	db.Model(&models.EventProgressGrant{}).Where("account_id = ?", account.ID).Count(&grantCount)
	db.Model(&models.EventRewardClaim{}).Where("account_id = ?", account.ID).Count(&claimCount)
	if grantCount != 1 || claimCount != 2 {
		t.Fatalf("unexpected event audit rows: grants=%d claims=%d", grantCount, claimCount)
	}

	lateReward := models.RewardTrackConfig{EventKey: event.Key, Milestone: 5, RewardType: "item", RewardKey: "eco_sample", RewardName: "生态样本", Quantity: 2}
	if err = db.Create(&lateReward).Error; err != nil {
		t.Fatal(err)
	}
	progress, granted, err = service.ClaimEventRewards(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if progress != 10 || len(granted) != 1 || granted[0].RewardName != "生态样本" || granted[0].RewardKey != "eco_sample" {
		t.Fatalf("new configured milestone was not claimable: progress=%d rewards=%#v", progress, granted)
	}
	_, granted, err = service.ClaimEventRewards(context.Background(), account.ID)
	if err != nil || len(granted) != 0 {
		t.Fatalf("event reward claim was not idempotent: err=%v rewards=%#v", err, granted)
	}
}

func TestEventCurrencyMilestoneCreditsSeasonWallet(t *testing.T) {
	service, db, now := newTestService(t)
	if err := db.Create(&models.LiveEventConfig{
		Key: "season-01", Name: "调查季", Region: "全域", Active: true,
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(7 * 24 * time.Hour), StoryChoices: `["一","二","三"]`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CurrencyConfig{Key: gameplay.SeasonTokenCurrencyKey, Name: "遗迹季印", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.RewardTrackConfig{EventKey: "season-01", Milestone: 8, RewardType: "currency", RewardKey: gameplay.SeasonTokenCurrencyKey, RewardName: "遗迹季印", Quantity: 6}).Error; err != nil {
		t.Fatal(err)
	}
	account, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "season-player", "活动"))
	if err != nil {
		t.Fatal(err)
	}
	progress, granted, err := service.AddEventProgress(context.Background(), account.ID, "season:run-1", 8)
	if err != nil || progress != 8 || len(granted) != 1 || granted[0].RewardType != "currency" {
		t.Fatalf("currency milestone failed: progress=%d rewards=%#v err=%v", progress, granted, err)
	}
	var wallet models.PlayerWallet
	if err = db.First(&wallet, "account_id = ? AND currency_key = ?", account.ID, gameplay.SeasonTokenCurrencyKey).Error; err != nil || wallet.Balance != 6 {
		t.Fatalf("season token wallet mismatch: %#v err=%v", wallet, err)
	}
	_, granted, err = service.AddEventProgress(context.Background(), account.ID, "season:run-1", 8)
	if err != nil || len(granted) != 0 {
		t.Fatalf("duplicate source granted again: %#v err=%v", granted, err)
	}
}

func TestLotteryPublishesPityAndSettlesExactlyOnce(t *testing.T) {
	service, db, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	game := models.ChanceGameConfig{GameKey: "lottery", Name: "测试抽奖", Enabled: true, CostCurrency: 10, DailyLimit: 3, PityThreshold: 3, PityRewardKey: "rare"}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}
	rewards := []models.ChanceRewardConfig{
		{GameKey: "lottery", RewardKey: "common", Name: "普通徽章", Weight: 99, ItemName: "普通徽章", Quantity: 1, Enabled: true},
		{GameKey: "lottery", RewardKey: "rare", Name: "珍稀徽章", Weight: 1, ItemName: "珍稀徽章", Quantity: 1, Rare: true, Enabled: true},
	}
	if err := db.Create(&rewards).Error; err != nil {
		t.Fatal(err)
	}
	account, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "lottery-player", "抽奖"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.AdoptWithStarter(context.Background(), account.ID, "光芽兽", "光芽兽", gameplay.DefaultCurrencyKey, 100); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 3; index++ {
		result, playErr := service.PlayLottery(context.Background(), account.ID, fmt.Sprintf("message:%d", index))
		if playErr != nil {
			t.Fatal(playErr)
		}
		if index < 3 && result.Outcome.RewardKey != "common" {
			t.Fatalf("unexpected non-pity reward: %#v", result)
		}
		if index == 3 && (!result.Outcome.PityTriggered || result.Outcome.RewardKey != "rare") {
			t.Fatalf("third draw should trigger configured pity: %#v", result)
		}
	}
	repeated, err := service.PlayLottery(context.Background(), account.ID, "message:3")
	if err != nil || !repeated.Repeated || repeated.Outcome.RewardKey != "rare" {
		t.Fatalf("message retry was not idempotent: result=%#v err=%v", repeated, err)
	}
	if _, err = service.PlayLottery(context.Background(), account.ID, "message:4"); !errors.Is(err, ErrDailyLimitReached) {
		t.Fatalf("expected daily limit, got %v", err)
	}
	balance, err := gameplay.NewWalletService(db).Balance(context.Background(), account.ID, gameplay.DefaultCurrencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 70 {
		t.Fatalf("lottery costs duplicated or missing: %d", balance)
	}
	var outcomes, costs int64
	db.Model(&models.ChanceOutcome{}).Where("account_id = ? AND game_key = ?", account.ID, "lottery").Count(&outcomes)
	db.Model(&models.WalletLedger{}).Where("account_id = ? AND reason = ?", account.ID, "lottery_cost").Count(&costs)
	if outcomes != 3 || costs != 3 {
		t.Fatalf("unexpected lottery audit counts: outcomes=%d costs=%d", outcomes, costs)
	}
}

func TestFishingUsesTimedClaimAndAuditedReward(t *testing.T) {
	service, db, now := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	if err := db.Create(&models.ChanceGameConfig{GameKey: "fishing", Name: "测试垂钓", Enabled: true, CostCurrency: 5, DailyLimit: 2, DurationSecond: 60}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ChanceRewardConfig{GameKey: "fishing", RewardKey: "fish", Name: "小鱼", Weight: 100, ItemName: "小鱼", Quantity: 1, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	account, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "fishing-player", "抛竿"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.AdoptWithStarter(context.Background(), account.ID, "光芽兽", "光芽兽", gameplay.DefaultCurrencyKey, 20); err != nil {
		t.Fatal(err)
	}
	run, attempts, limit, err := service.StartFishing(context.Background(), account.ID, "message:cast-1")
	if err != nil || attempts != 1 || limit != 2 || run.ReadyAt.Sub(run.StartedAt) != time.Minute {
		t.Fatalf("unexpected fishing start: run=%#v attempts=%d limit=%d err=%v", run, attempts, limit, err)
	}
	if _, _, _, err = service.StartFishing(context.Background(), account.ID, "message:cast-2"); !errors.Is(err, ErrFishingActive) {
		t.Fatalf("expected one active fishing run, got %v", err)
	}
	if _, err = service.ClaimFishing(context.Background(), account.ID); !errors.Is(err, ErrFishingNotReady) {
		t.Fatalf("expected early claim rejection, got %v", err)
	}
	*now = run.ReadyAt.Add(time.Second)
	claimed, err := service.ClaimFishing(context.Background(), account.ID)
	if err != nil || claimed.RewardKey != "fish" {
		t.Fatalf("unexpected fishing claim: run=%#v err=%v", claimed, err)
	}
	if _, err = service.ClaimFishing(context.Background(), account.ID); !errors.Is(err, ErrNoFishingRun) {
		t.Fatalf("second fishing claim should be rejected, got %v", err)
	}
	var item models.GlobalInventoryItem
	if err = db.First(&item, "account_id = ? AND item_name = ?", account.ID, "小鱼").Error; err != nil {
		t.Fatal(err)
	}
	var outcomes int64
	db.Model(&models.ChanceOutcome{}).Where("account_id = ? AND game_key = ?", account.ID, "fishing").Count(&outcomes)
	if item.Quantity != 1 || outcomes != 1 {
		t.Fatalf("fishing reward was not exactly once: item=%d outcomes=%d", item.Quantity, outcomes)
	}
}

func TestRockPaperScissorsIsEqualChanceAndIdempotent(t *testing.T) {
	service, db, _ := newTestService(t)
	service.RandomIntn = func(limit int) (int, error) {
		if limit != 3 {
			t.Fatalf("expected three equal opponent choices, got %d", limit)
		}
		return 0, nil
	}
	account, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "battle-player", "猜拳 布"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽"); err != nil {
		t.Fatal(err)
	}
	result, err := service.PlayRockPaperScissors(context.Background(), account.ID, "message:battle-1", "布")
	if err != nil || result.Record.OpponentChoice != "石头" || result.Record.Result != "胜利" || result.Record.RewardCurrency != 5 {
		t.Fatalf("unexpected battle result: %#v err=%v", result, err)
	}
	repeated, err := service.PlayRockPaperScissors(context.Background(), account.ID, "message:battle-1", "布")
	if err != nil || !repeated.Repeated {
		t.Fatalf("battle retry was not idempotent: %#v err=%v", repeated, err)
	}
	balance, err := gameplay.NewWalletService(db).Balance(context.Background(), account.ID, gameplay.DefaultCurrencyKey)
	if err != nil || balance != 5 {
		t.Fatalf("battle reward duplicated or missing: balance=%d err=%v", balance, err)
	}
}

func TestTradeEscrowSettlesInventoryAndWalletAtomically(t *testing.T) {
	service, db, _ := newTestService(t)
	if err := db.Create(&models.ItemConfig{Name: "木材", Status: "active", Type: "材料"}).Error; err != nil {
		t.Fatal(err)
	}
	seller, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "trade-seller", "宠物交易"))
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "trade-buyer", "交易列表"))
	if err != nil {
		t.Fatal(err)
	}
	if err = service.AddInventory(context.Background(), seller.ID, "木材", 5); err != nil {
		t.Fatal(err)
	}
	if err = gameplay.NewWalletService(db).Credit(context.Background(), buyer.ID, gameplay.DefaultCurrencyKey, 100); err != nil {
		t.Fatal(err)
	}
	offer, err := service.CreateTradeOffer(context.Background(), seller.ID, "木材", 3, 40)
	if err != nil {
		t.Fatal(err)
	}
	var sellerItem models.GlobalInventoryItem
	if err = db.First(&sellerItem, "account_id = ? AND item_name = ?", seller.ID, "木材").Error; err != nil {
		t.Fatal(err)
	}
	if sellerItem.Quantity != 2 {
		t.Fatalf("trade item was not escrowed: %d", sellerItem.Quantity)
	}
	completed, err := service.AcceptTradeOffer(context.Background(), buyer.ID, offer.Code)
	if err != nil || completed.Status != "completed" || completed.BuyerAccountID != buyer.ID {
		t.Fatalf("unexpected trade completion: offer=%#v err=%v", completed, err)
	}
	if _, err = service.AcceptTradeOffer(context.Background(), buyer.ID, offer.Code); !errors.Is(err, ErrTradeNotOpen) {
		t.Fatalf("completed trade should not settle again, got %v", err)
	}
	var buyerItem models.GlobalInventoryItem
	if err = db.First(&buyerItem, "account_id = ? AND item_name = ?", buyer.ID, "木材").Error; err != nil {
		t.Fatal(err)
	}
	sellerBalance, _ := gameplay.NewWalletService(db).Balance(context.Background(), seller.ID, gameplay.DefaultCurrencyKey)
	buyerBalance, _ := gameplay.NewWalletService(db).Balance(context.Background(), buyer.ID, gameplay.DefaultCurrencyKey)
	if buyerItem.Quantity != 3 || sellerBalance != 40 || buyerBalance != 60 {
		t.Fatalf("trade settlement mismatch: item=%d seller=%d buyer=%d", buyerItem.Quantity, sellerBalance, buyerBalance)
	}
	var audits int64
	db.Model(&models.TradeAudit{}).Where("offer_code = ?", offer.Code).Count(&audits)
	if audits != 3 {
		t.Fatalf("expected create/accept/complete trade audit, got %d", audits)
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
	if _, err = service.Adopt(context.Background(), account.ID, "光芽兽", "光芽兽"); err != nil {
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
	if result.Records != 7 || result.Growth != 5 || result.CodexEntry != "林间足迹" || result.Progress != 20 {
		t.Fatalf("unexpected deterministic rewards: %#v", result)
	}
	if _, err = service.ClaimExpedition(context.Background(), account.ID); err != ErrNothingToClaim {
		t.Fatalf("expected idempotent second claim rejection, got %v", err)
	}
	var item models.GlobalInventoryItem
	if err = db.Where("account_id = ? AND item_name = ?", account.ID, "调查记录").First(&item).Error; err != nil {
		t.Fatal(err)
	}
	if item.Quantity != 7 {
		t.Fatalf("expected one reward grant, got %d", item.Quantity)
	}
}

func TestCommunityProgressIsIsolated(t *testing.T) {
	service, db, _ := newTestService(t)
	account, err := service.ResolveAccount(context.Background(), oneBotEvent("100", "42", "营地"))
	if err != nil {
		t.Fatal(err)
	}
	if err = service.AddInventory(context.Background(), account.ID, "木材", 40); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.ItemConfig{Name: "木材", Status: "active", Type: "材料"}).Error; err != nil {
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

func TestCommunityContributionRejectsItemsOutsideCatalog(t *testing.T) {
	service, db, _ := newTestService(t)
	event := oneBotEvent("100", "catalog-check", "共建 伪造材料 5")
	account, err := service.ResolveAccount(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.AddInventory(context.Background(), account.ID, "伪造材料", 5); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Contribute(context.Background(), event, account.ID, "伪造材料", 5); err == nil || !strings.Contains(err.Error(), "不能用于社区共建") {
		t.Fatalf("expected catalog validation error, got %v", err)
	}
	var item models.GlobalInventoryItem
	if err = db.First(&item, "account_id = ? AND item_name = ?", account.ID, "伪造材料").Error; err != nil {
		t.Fatal(err)
	}
	if item.Quantity != 5 {
		t.Fatalf("rejected contribution consumed inventory: %d", item.Quantity)
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
	if _, err := service.Adopt(context.Background(), target.ID, "光芽兽", "光芽兽"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RedeemBindToken(context.Background(), targetEvent, token); err != ErrBindConflict {
		t.Fatalf("expected independent progress conflict, got %v", err)
	}
}
