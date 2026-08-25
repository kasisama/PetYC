package database

import (
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/gameplayrules"
	"qq-pet-saas/models"
)

var DB *gorm.DB

func InitDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("pet_game.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	if err = MigrateSchema(DB); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	if err = gameplayrules.EnsureDefaults(DB); err != nil {
		log.Fatalf("初始化成长与图鉴默认配置失败: %v", err)
	}
}

// MigrateSchema creates the complete current schema and its database-level
// invariants on an already opened connection. Keeping this separate from
// InitDB lets isolated tools and end-to-end fixtures use the exact production
// schema without touching the application's pet_game.db file.
func MigrateSchema(db *gorm.DB) error {
	// 旧索引只允许每个活动里程碑配置一个物品。必须先删除，再由
	// AutoMigrate 创建包含物品名的新唯一索引。
	if db.Migrator().HasIndex(&models.RewardTrackConfig{}, "idx_reward_track") {
		if err := db.Migrator().DropIndex(&models.RewardTrackConfig{}, "idx_reward_track"); err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(migrationModels()...); err != nil {
		return err
	}
	return ensureGameConstraints(db)
}

func ensureGameConstraints(db *gorm.DB) error {
	// Keep historical claimed runs while making the active slot unique. A plain
	// unique(account_id,status) index would incorrectly allow only one claimed
	// expedition in a player's lifetime.
	statements := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_expedition_one_running
			ON expedition_runs(account_id) WHERE status = 'running'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_activity_one_running
			ON activity_runs(account_id) WHERE status = 'running'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_fishing_one_running
			ON fishing_runs(account_id) WHERE status = 'running'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_adventure_exploration_one_active
			ON adventure_exploration_sessions(account_id) WHERE status = 'active'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_adventure_combat_one_active
			ON adventure_combat_sessions(account_id) WHERE status = 'active'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_adventure_expedition_one_running
			ON adventure_expedition_runs(account_id) WHERE status = 'running'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_player_equipment_one_slot
			ON player_equipments(account_id, equipped_slot) WHERE equipped_slot <> ''`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrationModels() []interface{} {
	return []interface{}{
		&models.SystemConfig{},
		&models.CommandConfig{},
		&models.PetSpeciesConfig{},
		&models.ItemConfig{},
		&models.ShopItemConfig{},
		&models.CheckinRewardConfig{},
		&models.WorkSettingConfig{},
		&models.MenuConfig{},
		&models.ImageConfig{},
		&models.GroupSwitch{},
		&models.AdminConfigState{},
		&models.ConfigProfile{},
		&models.PlayerAccount{},
		&models.PlayerIdentity{},
		&models.PetProfile{},
		&models.GlobalInventoryItem{},
		&models.PlayerWallet{},
		&models.WalletLedger{},
		&models.CompanionJournal{},
		&models.CompanionActionDaily{},
		&models.ActivityRun{},
		&models.ItemUseRecord{},
		&models.ExpeditionTemplateConfig{},
		&models.ExpeditionRun{},
		&models.CodexEntry{},
		&models.Community{},
		&models.CommunityMember{},
		&models.ExpeditionSquad{},
		&models.SquadMember{},
		&models.IdentityBindToken{},
		&models.NotificationPreference{},
		&models.NotificationJob{},
		&models.CommunityBoss{},
		&models.BossContribution{},
		&models.CommunityFacility{},
		&models.SeasonVote{},
		&models.CommunityHelpRequest{},
		&models.HelpGiftLog{},
		&models.HelpGiftDailyQuota{},
		&models.PetBehaviorProfile{},
		&models.LiveEventConfig{},
		&models.RewardTrackConfig{},
		&models.EventProgress{},
		&models.EventProgressGrant{},
		&models.EventRewardClaim{},
		&models.ChanceGameConfig{},
		&models.ChanceRewardConfig{},
		&models.ChanceDailyState{},
		&models.ChancePlayerState{},
		&models.ChanceOutcome{},
		&models.FishingRun{},
		&models.BattleRecord{},
		&models.TradeOffer{},
		&models.TradeAudit{},
		&models.GrowthRoleConfig{},
		&models.GrowthStanceConfig{},
		&models.PersonalityRuleConfig{},
		&models.CodexCatalogConfig{},
		&models.GameplayMetric{},
		&models.AdminAuditLog{},
		&models.AdminOperationKey{},
		&models.ImageAsset{},
		&models.AdventureMapConfig{},
		&models.AdventureZoneConfig{},
		&models.AdventureZonePrerequisiteConfig{},
		&models.AdventureObjectiveConfig{},
		&models.AdventureMonsterConfig{},
		&models.AdventureSkillConfig{},
		&models.AdventureMonsterSkillConfig{},
		&models.AdventureEncounterConfig{},
		&models.AdventureLootPoolConfig{},
		&models.AdventureLootEntryConfig{},
		&models.AdventureExpeditionConfig{},
		&models.AdventureBossConfig{},
		&models.AdventureBossRewardTierConfig{},
		&models.EquipmentTemplateConfig{},
		&models.EquipmentAffixConfig{},
		&models.EquipmentRecipeConfig{},
		&models.EquipmentRecipeMaterialConfig{},
		&models.LiveEventChoiceConfig{},
		&models.LiveEventExpeditionSourceConfig{},
		&models.PlayerAdventureProgress{},
		&models.PlayerZoneProgress{},
		&models.PlayerObjectiveProgress{},
		&models.AdventureExplorationSession{},
		&models.AdventureCombatSession{},
		&models.AdventureCombatTurn{},
		&models.PlayerEquipment{},
		&models.PlayerBlueprintProgress{},
		&models.AdventureExpeditionRun{},
		&models.AdventureBossInstance{},
		&models.AdventureBossContribution{},
		&models.AdventureBossRewardClaim{},
		&models.EquipmentCraftRecord{},
	}
}
