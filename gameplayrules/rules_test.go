package gameplayrules

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func newRulesTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(
		&models.GrowthRoleConfig{},
		&models.GrowthStanceConfig{},
		&models.PersonalityRuleConfig{},
		&models.CodexCatalogConfig{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestLoadUsesCompleteBuiltInDefaultsForEmptyTables(t *testing.T) {
	rules, warnings := Load(newRulesTestDB(t))
	if len(rules.Roles) != 4 || len(rules.Stances) != 4 || len(rules.Personalities) != 3 || len(rules.Codex) < 3 {
		t.Fatalf("expected complete defaults, got %+v", rules)
	}
	if len(warnings) != 4 {
		t.Fatalf("expected one fallback warning per empty config domain, got %v", warnings)
	}
}

func TestLoadUsesSavedCustomRoleConfiguration(t *testing.T) {
	db := newRulesTestDB(t)
	custom := models.GrowthRoleConfig{
		Name: "采集者", Description: "专注素材调查", Skill1: "辨识", Skill2: "采样", Skill3: "整理", Enabled: true, SortOrder: 1,
	}
	if err := db.Create(&custom).Error; err != nil {
		t.Fatal(err)
	}
	rules, warnings := Load(db)
	if len(rules.Roles) != 1 || rules.Roles[0].Name != custom.Name || rules.Roles[0].Skill2 != custom.Skill2 {
		t.Fatalf("custom role was not loaded: %+v", rules.Roles)
	}
	if len(warnings) != 3 {
		t.Fatalf("expected only remaining domains to fall back, got %v", warnings)
	}
}
