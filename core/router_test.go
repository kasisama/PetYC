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

func TestOfficialGroupUsesSamePersistedSwitch(t *testing.T) {
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

	event := InboundEvent{Platform: PlatformQQGroup, SceneType: SceneGroup, SpaceID: "opaque-group"}
	if !GroupEnabled(event) {
		t.Fatal("首次出现的官方群应默认启用")
	}
	var stored models.GroupSwitch
	if err = db.First(&stored, "platform = ? AND space_id = ?", string(PlatformQQGroup), "opaque-group").Error; err != nil {
		t.Fatal(err)
	}
	if stored.GroupID >= 0 || stored.GroupName == "" {
		t.Fatalf("官方群应登记为可运营场景: %#v", stored)
	}
	if err = db.Model(&stored).Update("is_active", false).Error; err != nil {
		t.Fatal(err)
	}
	if GroupEnabled(event) {
		t.Fatal("后台停用的官方群不得继续路由命令")
	}
	if !GroupEnabled(InboundEvent{Platform: PlatformQQGroup, SceneType: SceneDirect, ActorID: "user"}) {
		t.Fatal("C2C 不应被群开关拦截")
	}
}
