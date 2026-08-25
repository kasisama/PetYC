package config

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func TestEnsureModernMenusCreatesCurrentDefaultsWithoutDeletingExistingMenus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.MenuConfig{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&models.MenuConfig{Name: "宠物攻击", Reply: "管理员保留的旧菜单"})
	if err = EnsureModernMenus(db); err != nil {
		t.Fatal(err)
	}
	var rows []models.MenuConfig
	db.Order("name").Find(&rows)
	if len(rows) != 8 {
		t.Fatalf("期望保留旧菜单并补齐七个菜单，实际 %+v", rows)
	}
	var preserved bool
	for _, row := range rows {
		if row.Name == "宠物攻击" {
			preserved = row.Reply == "管理员保留的旧菜单"
		}
	}
	if !preserved {
		t.Fatal("菜单初始化不应删除管理员已有菜单")
	}
	var migrated models.MenuConfig
	db.First(&migrated, "name = ?", "主菜单")
	for _, command := range []string{"宠物菜单", "我的宠物", "签到", "我的背包", "远征"} {
		if !strings.Contains(migrated.Reply, command) {
			t.Fatalf("主菜单应展示熟悉入口 %q，实际 %q", command, migrated.Reply)
		}
	}
	for _, decoration := range []string{"🐾", "🌟", "🍖", "🧭", "🏕️", "🔐", "💡"} {
		if !strings.Contains(migrated.Reply, decoration) {
			t.Fatalf("主菜单应保留分区 emoji %q，实际 %q", decoration, migrated.Reply)
		}
	}
	db.Model(&models.MenuConfig{}).Where("name = ?", "主菜单").Update("reply", "管理员自定义")
	if err = EnsureModernMenus(db); err != nil {
		t.Fatal(err)
	}
	var menu models.MenuConfig
	db.First(&menu, "name = ?", "主菜单")
	if menu.Reply != "管理员自定义" {
		t.Fatalf("二次启动不应覆盖管理员修改，实际 %q", menu.Reply)
	}
}
