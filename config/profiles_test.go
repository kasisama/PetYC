package config

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func profileTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	modelsToMigrate := []any{
		&models.SystemConfig{}, &models.CommandConfig{}, &models.PetSpeciesConfig{}, &models.PetEvolutionRuleConfig{},
		&models.PetEvolutionCostConfig{}, &models.PetSkillUnlockConfig{}, &models.AdventureLevelConfig{}, &models.ItemConfig{}, &models.ShopItemConfig{},
		&models.CheckinRewardConfig{}, &models.WorkSettingConfig{}, &models.MenuConfig{}, &models.ImageConfig{}, &models.LiveEventConfig{},
		&models.RewardTrackConfig{}, &models.GrowthRoleConfig{}, &models.GrowthStanceConfig{}, &models.PersonalityRuleConfig{},
		&models.CodexCatalogConfig{}, &models.ExpeditionTemplateConfig{}, &models.ChanceGameConfig{}, &models.ChanceRewardConfig{},
		&models.AdminConfigState{}, &models.ConfigProfile{}, &models.PetProfile{}, &models.GlobalInventoryItem{}, &models.EventProgress{},
		&models.ActivityRun{}, &models.ExpeditionRun{}, &models.TradeOffer{}, &models.FishingRun{},
		&models.AdventureMapConfig{}, &models.AdventureZoneConfig{}, &models.AdventureZonePrerequisiteConfig{},
		&models.AdventureObjectiveConfig{}, &models.AdventureMonsterConfig{}, &models.AdventureSkillConfig{},
		&models.AdventureExplorationStageConfig{}, &models.AdventureStoryEventConfig{}, &models.AdventureStoryEventChoiceConfig{},
		&models.AdventureMonsterSkillConfig{}, &models.AdventureEncounterConfig{}, &models.AdventureEncounterEffectConfig{}, &models.AdventureLootPoolConfig{},
		&models.AdventureLootEntryConfig{}, &models.CurrencyConfig{},
		&models.AdventureShopItemConfig{}, &models.AdventureExpeditionConfig{}, &models.AdventureBossConfig{},
		&models.AdventureBossRewardTierConfig{}, &models.EquipmentTemplateConfig{}, &models.EquipmentAffixConfig{},
		&models.EquipmentRecipeConfig{}, &models.EquipmentRecipeMaterialConfig{}, &models.LiveEventChoiceConfig{},
		&models.LiveEventExpeditionSourceConfig{},
		&models.AdventureExplorationSession{}, &models.AdventureCombatSession{}, &models.AdventureExpeditionRun{},
		&models.PlayerEquipment{}, &models.AdventureBossInstance{},
	}
	if err = db.AutoMigrate(modelsToMigrate...); err != nil {
		t.Fatal(err)
	}
	return db
}

func minimalSnapshot() ConfigSnapshot {
	return ConfigSnapshot{
		System:     []models.SystemConfig{{Key: "Core.Currency", Value: "金币"}},
		Commands:   []models.CommandConfig{{FuncName: "Help", Command: "帮助", DisplayName: "帮助", Enabled: true}},
		PetSpecies: []models.PetSpeciesConfig{{Name: "米塔"}},
		Items:      []models.ItemConfig{{Name: "苹果", Status: "active"}},
	}
}

func TestSnapshotExcludesLocalAndPreservesPlayerData(t *testing.T) {
	db := profileTestDB(t)
	if err := db.Create(&[]models.SystemConfig{{Key: "Core.MasterQQ", Value: "123456"}, {Key: "Core.Currency", Value: "旧币"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CommandConfig{FuncName: "Help", Command: "help", DisplayName: "Help", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetSpeciesConfig{Name: "米塔"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Name: "苹果", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetProfile{AccountID: "player-1", PetType: "米塔", Name: "我的宠物", CurrentForm: "米塔"}).Error; err != nil {
		t.Fatal(err)
	}

	snapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "123456") || strings.Contains(payload, "player-1") {
		t.Fatal("snapshot leaked local or player data")
	}

	target := minimalSnapshot()
	target.System[0].Value = "新币"
	if err = db.Transaction(func(tx *gorm.DB) error { return ApplySnapshot(tx, target) }); err != nil {
		t.Fatal(err)
	}
	var local models.SystemConfig
	if err = db.First(&local, "key = ?", "Core.MasterQQ").Error; err != nil || local.Value != "123456" {
		t.Fatalf("local config not preserved: %+v %v", local, err)
	}
	var player models.PetProfile
	if err = db.First(&player, "account_id = ?", "player-1").Error; err != nil || player.Name != "我的宠物" {
		t.Fatalf("player changed: %+v %v", player, err)
	}
}

func TestCaptureSnapshotIncludesNodeAdventureCollections(t *testing.T) {
	db := profileTestDB(t)
	rows := []any{
		&models.AdventureMapConfig{Key: "node-map", Name: "节点地图", Region: "试炼", RecommendedLevel: 1},
		&models.AdventureZoneConfig{Key: "node-zone", MapKey: "node-map", Name: "节点区域", RecommendedLevel: 1, DifficultyPermille: 1000, ExplorationMode: "node"},
		&models.AdventureExplorationStageConfig{Key: "node-stage", ZoneKey: "node-zone", Name: "开始调查", ProgressEnd: 100, EventKey: "node-event"},
		&models.AdventureStoryEventConfig{Key: "node-event", ZoneKey: "node-zone", StageKey: "node-stage", Name: "调查选择", EventType: "mainline", Weight: 1},
		&models.AdventureStoryEventChoiceConfig{EventKey: "node-event", ChoiceKey: "option_1", Label: "记录", RiskLevel: "low"},
		&models.AdventureStoryEventChoiceConfig{EventKey: "node-event", ChoiceKey: "option_2", Label: "追踪", RiskLevel: "high"},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.AdventureStages) != 1 || len(snapshot.AdventureStoryEvents) != 1 || len(snapshot.AdventureStoryChoices) != 2 {
		t.Fatalf("节点探索集合未进入配置快照: %#v", snapshot)
	}
}

func TestCompatibilityBlocksMissingReferencedKeys(t *testing.T) {
	db := profileTestDB(t)
	if err := db.Create(&models.PetProfile{AccountID: "p1", PetType: "不存在宠物", Name: "宠物", CurrentForm: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.GlobalInventoryItem{AccountID: "p1", ItemName: "不存在物品", Quantity: 2}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.EventProgress{AccountID: "p1", EventKey: "missing-event", Progress: 9}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TradeOffer{Code: "TRADE1", SellerAccountID: "p1", ItemName: "交易锁定物品", Quantity: 1, CurrencyKey: "coin", Status: "open"}).Error; err != nil {
		t.Fatal(err)
	}
	conflicts, err := CheckSnapshotCompatibility(db, minimalSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 3 {
		t.Fatalf("want 3 conflicts, got %+v", conflicts)
	}
	for _, conflict := range conflicts {
		if conflict.Kind == "items" && conflict.AffectedCount != 2 {
			t.Fatalf("want inventory and trade conflicts, got %+v", conflict)
		}
	}
}

func TestOfficialDefaultsAreVersionedAndContainNoLocalKeys(t *testing.T) {
	db := profileTestDB(t)
	if err := EnsureOfficialDefaults(db); err != nil {
		t.Fatal(err)
	}
	var profile models.ConfigProfile
	if err := db.First(&profile, "id = ?", OfficialProfileID).Error; err != nil {
		t.Fatal(err)
	}
	if !profile.Builtin || profile.AppVersion != "0.1.0" || profile.SchemaVersion != 2 {
		t.Fatalf("unexpected official profile: %+v", profile)
	}
	snapshot, err := DecodeSnapshot(profile.Payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range snapshot.System {
		if _, forbidden := LocalSystemKeys[row.Key]; forbidden {
			t.Fatalf("official defaults contain %s", row.Key)
		}
	}
	if snapshot.Summary().Rows < 300 {
		t.Fatalf("official defaults unexpectedly small: %+v", snapshot.Summary())
	}
}

func TestRepairKnownBrokenFishingConfigRestoresBackupPool(t *testing.T) {
	db := profileTestDB(t)
	brokenGame := models.ChanceGameConfig{GameKey: "fishing", Name: "生态垂钓", Enabled: true, CostCurrency: 10, DailyLimit: 5, PityThreshold: 5, PityRewardKey: "echo_shell", DurationSecond: 60}
	brokenRewards := []models.ChanceRewardConfig{
		{GameKey: "fishing", RewardKey: "fishing_meadow_fiber", Name: "原野纤维", ItemName: "原野纤维", Quantity: 1, Weight: 60, Enabled: true, SortOrder: 0},
		{GameKey: "fishing", RewardKey: "fishing_survey_ink", Name: "调查墨水", ItemName: "调查墨水", Quantity: 1, Weight: 28, Enabled: true, SortOrder: 10},
		{GameKey: "fishing", RewardKey: "fishing_pressed_flower", Name: "栖光压花", ItemName: "栖光压花", Quantity: 1, Weight: 10, Rare: true, Enabled: true, SortOrder: 20},
		{GameKey: "fishing", RewardKey: "fishing_star_core", Name: "星辉晶核", ItemName: "星辉晶核", Quantity: 1, Weight: 2, Rare: true, Enabled: true, SortOrder: 30},
	}
	if err := db.Create(&brokenGame).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&brokenRewards).Error; err != nil {
		t.Fatal(err)
	}
	if err := repairKnownBrokenFishingConfig(db); err != nil {
		t.Fatal(err)
	}
	var game models.ChanceGameConfig
	if err := db.First(&game, "game_key = ?", "fishing").Error; err != nil {
		t.Fatal(err)
	}
	if game.Name != "水域垂钓" || game.CostCurrency != 5 || game.DailyLimit != 20 || game.PityRewardKey != "water-sample" {
		t.Fatalf("钓鱼规则未恢复备份值: %#v", game)
	}
	var rewards []models.ChanceRewardConfig
	if err := db.Where("game_key = ?", "fishing").Order("sort_order").Find(&rewards).Error; err != nil {
		t.Fatal(err)
	}
	if len(rewards) != 4 || rewards[0].RewardKey != "small-fish" || rewards[1].RewardKey != "shell" || rewards[2].RewardKey != "pearl" || rewards[3].RewardKey != "water-sample" {
		t.Fatalf("钓鱼奖池未恢复: %#v", rewards)
	}
}

func TestRepairKnownBrokenFishingConfigPreservesCustomizedPool(t *testing.T) {
	db := profileTestDB(t)
	custom := models.ChanceRewardConfig{GameKey: "fishing", RewardKey: "custom-fish", Name: "自定义鱼", ItemName: "自定义鱼", Quantity: 1, Weight: 100, Enabled: true}
	if err := db.Create(&custom).Error; err != nil {
		t.Fatal(err)
	}
	if err := repairKnownBrokenFishingConfig(db); err != nil {
		t.Fatal(err)
	}
	var rewards []models.ChanceRewardConfig
	if err := db.Where("game_key = ?", "fishing").Find(&rewards).Error; err != nil {
		t.Fatal(err)
	}
	if len(rewards) != 1 || rewards[0].RewardKey != "custom-fish" {
		t.Fatalf("自定义奖池不应被覆盖: %#v", rewards)
	}
}

func TestEnsureOfficialDefaultsDoesNotOverwriteNonEmptyDatabase(t *testing.T) {
	db := profileTestDB(t)
	if err := db.Create(&models.SystemConfig{Key: "Core.Currency", Value: "金币"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CommandConfig{FuncName: "Help", Command: "帮助", DisplayName: "帮助", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetSpeciesConfig{Name: "米塔"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Name: "苹果", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ExpeditionTemplateConfig{Tier: 2, Name: "遗迹调查", Enabled: true, DurationMinutes: 60, RewardItem: "古代零件", RewardQuantity: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.GlobalInventoryItem{AccountID: "player-1", ItemName: "苹果", Quantity: 7}).Error; err != nil {
		t.Fatal(err)
	}

	if err := EnsureOfficialDefaults(db); err != nil {
		t.Fatalf("ensure must not fail on an existing database: %v", err)
	}
	var legacyCount, mapCount int64
	if err := db.Model(&models.ExpeditionTemplateConfig{}).Count(&legacyCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.AdventureMapConfig{}).Count(&mapCount).Error; err != nil {
		t.Fatal(err)
	}
	if legacyCount != 1 || mapCount != 0 {
		t.Fatalf("startup must not overwrite live config: legacy=%d maps=%d", legacyCount, mapCount)
	}
	var inventory models.GlobalInventoryItem
	if err := db.First(&inventory, "account_id = ? AND item_name = ?", "player-1", "苹果").Error; err != nil {
		t.Fatal(err)
	}
	if inventory.Quantity != 7 {
		t.Fatalf("player inventory changed during config ensure: %+v", inventory)
	}
}

func TestRebuildOfficialDefaultsReplacesConfigAndKeepsPlayerInventory(t *testing.T) {
	db := profileTestDB(t)
	if err := db.Create(&models.SystemConfig{Key: "Core.Currency", Value: "金币"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Name: "苹果", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ExpeditionTemplateConfig{Tier: 2, Name: "遗迹调查", Enabled: true, DurationMinutes: 60, RewardItem: "古代零件", RewardQuantity: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.GlobalInventoryItem{AccountID: "player-1", ItemName: "苹果", Quantity: 7}).Error; err != nil {
		t.Fatal(err)
	}
	if err := RebuildOfficialDefaults(db); err != nil {
		t.Fatalf("rebuild failed: %v", err)
	}
	var legacyCount, mapCount, currencyCount int64
	if err := db.Model(&models.ExpeditionTemplateConfig{}).Count(&legacyCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.AdventureMapConfig{}).Count(&mapCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.CurrencyConfig{}).Count(&currencyCount).Error; err != nil {
		t.Fatal(err)
	}
	if legacyCount != 0 || mapCount != 3 || currencyCount != 3 {
		t.Fatalf("unexpected rebuild result: legacy=%d maps=%d currencies=%d", legacyCount, mapCount, currencyCount)
	}
	for _, table := range []string{"adventure_item_configs", "player_adventure_inventory_items", "player_adventure_wallets", "adventure_wallet_ledgers"} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("deprecated economy table must not be migrated: %s", table)
		}
	}
	var inventory models.GlobalInventoryItem
	if err := db.First(&inventory, "account_id = ? AND item_name = ?", "player-1", "苹果").Error; err != nil {
		t.Fatal(err)
	}
	if inventory.Quantity != 7 {
		t.Fatalf("player inventory changed during rebuild: %+v", inventory)
	}
	var codex models.CodexCatalogConfig
	if err := db.First(&codex, "category = ? AND entry_key = ?", "区域生态", "sunlit_steppe_z1").Error; err != nil {
		t.Fatal(err)
	}
	if codex.SourceType != "zone" || codex.SourceKey != "sunlit_steppe_z1" {
		t.Fatalf("codex source was not rebuilt: %+v", codex)
	}
}
