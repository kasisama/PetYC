package gameplay

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"qq-pet-saas/models"
)

func TestUnlockedAdventureSkillsRespectsFormAndLevel(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.AutoMigrate(&models.PlayerAdventureProgress{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create([]models.PetSkillUnlockConfig{
		{FormKey: "lumisprout_base", SkillKey: "pet_skill_01", UnlockLevel: 1, SortOrder: 10},
		{FormKey: "lumisprout_base", SkillKey: "pet_skill_02", UnlockLevel: 5, SortOrder: 20},
		{FormKey: "lumisprout_evolved", SkillKey: "pet_skill_03", UnlockLevel: 1, SortOrder: 10},
	}).Error; err != nil {
		t.Fatal(err)
	}
	keys := UnlockedAdventureSkills(db, "lumisprout_base", 1)
	if len(keys) != 1 || keys[0] != "pet_skill_01" {
		t.Fatalf("level 1 unlocks=%v", keys)
	}
	keys = UnlockedAdventureSkills(db, "lumisprout_base", 5)
	if len(keys) != 2 || keys[0] != "pet_skill_01" || keys[1] != "pet_skill_02" {
		t.Fatalf("level 5 unlocks=%v", keys)
	}
	keys = UnlockedAdventureSkills(db, "lumisprout_evolved", 1)
	if len(keys) != 1 || keys[0] != "pet_skill_03" {
		t.Fatalf("evolved unlocks=%v", keys)
	}
}

func TestAdoptWritesUnlockedAdventureSkillKeys(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.AutoMigrate(&models.PlayerAdventureProgress{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PlayerAccount{ID: "account-1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetSpeciesConfig{
		Key: "lumisprout_base", Name: "光芽兽", FamilyKey: "lumisprout", Stage: "base", Adoptable: true,
		Health: 100, Hunger: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetSkillUnlockConfig{FormKey: "lumisprout_base", SkillKey: "pet_skill_01", UnlockLevel: 1}).Error; err != nil {
		t.Fatal(err)
	}
	pet, err := NewPetService(db).Adopt(context.Background(), "account-1", "lumisprout_base", "小光")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	if json.Unmarshal([]byte(pet.Skills), &keys) != nil {
		t.Fatalf("skills should be JSON keys, got %q", pet.Skills)
	}
	if len(keys) != 1 || keys[0] != "pet_skill_01" {
		t.Fatalf("adopted skills=%v text=%q", keys, pet.Skills)
	}
	if strings.Contains(pet.Skills, "寻路") {
		t.Fatalf("combat skills must not use growth-role flavor names: %q", pet.Skills)
	}
}

func TestRefreshPetSkillsTxReplacesGrowthFlavorNames(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.AutoMigrate(&models.PlayerAdventureProgress{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetSkillUnlockConfig{FormKey: "lumisprout_base", SkillKey: "pet_skill_01", UnlockLevel: 1}).Error; err != nil {
		t.Fatal(err)
	}
	pet := models.PetProfile{ID: "pet-1", AccountID: "account-1", CurrentForm: "lumisprout_base", Skills: "寻路,观察,记录"}
	if err := db.Create(&pet).Error; err != nil {
		t.Fatal(err)
	}
	if err := RefreshPetSkillsTx(db, &pet); err != nil {
		t.Fatal(err)
	}
	var keys []string
	if json.Unmarshal([]byte(pet.Skills), &keys) != nil || len(keys) != 1 || keys[0] != "pet_skill_01" {
		t.Fatalf("legacy flavor skills should be rewritten to unlock keys: %q", pet.Skills)
	}
}
