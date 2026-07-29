package database

import (
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

var DB *gorm.DB

// InitDB 初始化 SQLite 数据库
func InitDB() {
	var err error
	// 使用本地文件 pet_game.db，不存在会自动创建
	DB, err = gorm.Open(sqlite.Open("pet_game.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	// 自动迁移模式：包含所有玩家数据及核心配置数据表
	err = DB.AutoMigrate(
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
	)
	if err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	log.Println("数据库初始化成功，已自动迁移表结构。")
}

