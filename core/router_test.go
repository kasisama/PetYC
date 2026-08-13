package core

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
)

func TestOneBotGroupEnabledHonorsAdminSwitch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.GroupSwitch{}); err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })
	if err = db.Create(&models.GroupSwitch{GroupID: 100, IsActive: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&models.GroupSwitch{}).Where("group_id = ?", 100).Update("is_active", false).Error; err != nil {
		t.Fatal(err)
	}
	if oneBotGroupEnabled(100) {
		t.Fatal("disabled group must not route modern commands")
	}
	if !oneBotGroupEnabled(200) {
		t.Fatal("groups without an explicit switch should be enabled")
	}
}
