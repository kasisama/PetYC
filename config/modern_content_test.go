package config

import (
	"bytes"
	"io"
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
	for _, command := range []string{"宠物菜单", "我的宠物", "签到", "我的背包", "商店", "学习", "打工", "进化", "远征"} {
		if !strings.Contains(migrated.Reply, command) {
			t.Fatalf("主菜单应展示熟悉入口 %q，实际 %q", command, migrated.Reply)
		}
	}
	if strings.Contains(migrated.Reply, "普攻") || strings.Contains(migrated.Reply, "防御") {
		t.Fatalf("主菜单不应把战斗子命令放在一级：%q", migrated.Reply)
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

func TestEnsureModernMenusMigratesLegacyOfficialTemplateWithoutPrompt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.MenuConfig{}); err != nil {
		t.Fatal(err)
	}
	legacy := models.MenuConfig{Name: "主菜单", Reply: legacyOfficialMenus["主菜单"].Reply, Markdown: legacyOfficialMenus["主菜单"].Markdown}
	if err = db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err = EnsureModernMenus(db); err != nil {
		t.Fatal(err)
	}
	var menu models.MenuConfig
	if err = db.First(&menu, "name = ?", "主菜单").Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(menu.Reply, "商店") || strings.Contains(menu.Reply, "普攻") {
		t.Fatalf("旧官方主菜单应自动迁移，实际 %q", menu.Reply)
	}
}

func TestEnsureModernMenusAsksBeforeOverwritingCustomMenu(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.MenuConfig{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.MenuConfig{Name: "主菜单", Reply: "管理员手改的菜单"}).Error; err != nil {
		t.Fatal(err)
	}
	asked := false
	if err = ApplyModernMenus(db, func(scene, current string) bool {
		asked = true
		if scene != "主菜单" || current != "管理员手改的菜单" {
			t.Fatalf("提示内容不正确: scene=%q current=%q", scene, current)
		}
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if !asked {
		t.Fatal("自定义菜单应询问是否覆盖")
	}
	var menu models.MenuConfig
	db.First(&menu, "name = ?", "主菜单")
	if !strings.Contains(menu.Reply, "商店") {
		t.Fatalf("同意覆盖后应写入官方新默认，实际 %q", menu.Reply)
	}

	if err = db.Model(&models.MenuConfig{}).Where("name = ?", "主菜单").Update("reply", "再次手改").Error; err != nil {
		t.Fatal(err)
	}
	if err = ApplyModernMenus(db, func(string, string) bool { return false }); err != nil {
		t.Fatal(err)
	}
	db.First(&menu, "name = ?", "主菜单")
	if menu.Reply != "再次手改" {
		t.Fatalf("拒绝覆盖后应保留自定义，实际 %q", menu.Reply)
	}
}

func TestApplyModernMenusDoesNotTouchCustomConfigProfile(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.MenuConfig{}, &models.AdminConfigState{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.AdminConfigState{ID: 1, ActiveProfileID: "custom-profile"}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.MenuConfig{Name: "主菜单", Reply: "我自己的运营菜单"}).Error; err != nil {
		t.Fatal(err)
	}
	asked := false
	if err = ApplyModernMenus(db, func(string, string) bool {
		asked = true
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if asked {
		t.Fatal("自建配置方案启动时不应询问覆盖官方菜单")
	}
	var menu models.MenuConfig
	if err = db.First(&menu, "name = ?", "主菜单").Error; err != nil {
		t.Fatal(err)
	}
	if menu.Reply != "我自己的运营菜单" {
		t.Fatalf("自建配置方案的菜单被改写了: %q", menu.Reply)
	}
	var count int64
	if err = db.Model(&models.MenuConfig{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("自建配置方案不应被补进官方菜单场景，实际 %d 条", count)
	}
}

func TestPromptMenuOverwriteNonInteractiveKeepsCustom(t *testing.T) {
	prompt := PromptMenuOverwrite(strings.NewReader("y\n"), io.Discard, false)
	if prompt("主菜单", "自定义") {
		t.Fatal("非交互启动不得覆盖自定义菜单")
	}
	var out bytes.Buffer
	prompt = PromptMenuOverwrite(strings.NewReader("y\n"), &out, true)
	if !prompt("主菜单", "自定义") {
		t.Fatal("交互启动输入 y 应覆盖")
	}
	if !strings.Contains(out.String(), "官方默认数据已更新，请问是否覆盖？") || !strings.Contains(out.String(), "主菜单") {
		t.Fatalf("覆盖提示文案不符合预期: %q", out.String())
	}
}

func TestOfficialMenusAllProvideMarkdown(t *testing.T) {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Menus) == 0 {
		t.Fatal("官方配置不应缺少菜单场景")
	}
	for _, menu := range snapshot.Menus {
		if strings.TrimSpace(menu.Markdown) == "" {
			t.Fatalf("菜单 %q 缺少 Markdown 回复", menu.Name)
		}
		if !strings.HasPrefix(strings.TrimSpace(menu.Markdown), "# ") {
			t.Fatalf("菜单 %q 的 Markdown 应以一级标题开头，实际 %q", menu.Name, menu.Markdown)
		}
	}
}
