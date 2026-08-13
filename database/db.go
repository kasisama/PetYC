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
	if err = migrateSchema(DB); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	if err = gameplayrules.EnsureDefaults(DB); err != nil {
		log.Fatalf("初始化成长与图鉴默认配置失败: %v", err)
	}
}

func migrateSchema(db *gorm.DB) error {
	// 旧索引只允许每个活动里程碑配置一个物品。必须先删除，再由
	// AutoMigrate 创建包含物品名的新唯一索引。
	if db.Migrator().HasIndex(&models.RewardTrackConfig{}, "idx_reward_track") {
		if err := db.Migrator().DropIndex(&models.RewardTrackConfig{}, "idx_reward_track"); err != nil {
			return err
		}
	}
	return db.AutoMigrate(migrationModels()...)
}

func migrationModels() []interface{} {
	return []interface{}{
		&models.UserPet{},
		&models.BackpackItem{},
		&models.Family{},
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
		&models.PlayerAccount{},
		&models.PlayerIdentity{},
		&models.PetProfile{},
		&models.GlobalInventoryItem{},
		&models.CompanionJournal{},
		&models.ExpeditionRun{},
		&models.CodexEntry{},
		&models.Community{},
		&models.CommunityMember{},
		&models.ExpeditionSquad{},
		&models.SquadMember{},
		&models.IdentityBindToken{},
		&models.NotificationPreference{},
		&models.CommunityBoss{},
		&models.BossContribution{},
		&models.CommunityFacility{},
		&models.SeasonVote{},
		&models.CommunityHelpRequest{},
		&models.HelpGiftLog{},
		&models.PetBehaviorProfile{},
		&models.LiveEventConfig{},
		&models.RewardTrackConfig{},
		&models.GrowthRoleConfig{},
		&models.GrowthStanceConfig{},
		&models.PersonalityRuleConfig{},
		&models.CodexCatalogConfig{},
		&models.GameplayMetric{},
		&models.AdminAuditLog{},
		&models.AdminOperationKey{},
	}
}
