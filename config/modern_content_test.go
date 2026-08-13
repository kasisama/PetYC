package config

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func TestEnsureModernMenusReplacesLegacyOnlyOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.MenuConfig{}, &models.SystemConfig{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&models.MenuConfig{Name: "宠物攻击", Reply: "旧偷袭菜单"})
	if err = EnsureModernMenus(db); err != nil {
		t.Fatal(err)
	}
	var rows []models.MenuConfig
	db.Order("name").Find(&rows)
	if len(rows) != 7 {
		t.Fatalf("期望七个新版菜单，实际 %+v", rows)
	}
	for _, row := range rows {
		if row.Name == "宠物攻击" {
			t.Fatal("旧菜单未被移除")
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
