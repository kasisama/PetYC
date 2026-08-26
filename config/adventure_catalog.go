package config

import (
	"encoding/json"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

// AdventureCatalog is the public, configuration-only contract used by the
// adventure editor. It deliberately contains no player, combat, boss-instance,
// equipment-instance, progress, or audit rows.
type AdventureCatalog struct {
	Maps                     []models.AdventureMapConfig              `json:"maps"`
	Zones                    []models.AdventureZoneConfig             `json:"zones"`
	Prerequisites            []models.AdventureZonePrerequisiteConfig `json:"prerequisites"`
	Objectives               []models.AdventureObjectiveConfig        `json:"objectives"`
	Monsters                 []models.AdventureMonsterConfig          `json:"monsters"`
	Skills                   []models.AdventureSkillConfig            `json:"skills"`
	MonsterSkills            []models.AdventureMonsterSkillConfig     `json:"monster_skills"`
	Encounters               []models.AdventureEncounterConfig        `json:"encounters"`
	EncounterEffects         []models.AdventureEncounterEffectConfig  `json:"encounter_effects"`
	LootPools                []models.AdventureLootPoolConfig         `json:"loot_pools"`
	LootEntries              []models.AdventureLootEntryConfig        `json:"loot_entries"`
	Currencies               []models.CurrencyConfig                  `json:"currencies"`
	Items                    []models.ItemConfig                      `json:"items"`
	ShopItems                []models.AdventureShopItemConfig         `json:"shop_items"`
	Expeditions              []models.AdventureExpeditionConfig       `json:"expeditions"`
	Bosses                   []models.AdventureBossConfig             `json:"bosses"`
	BossRewardTiers          []models.AdventureBossRewardTierConfig   `json:"boss_reward_tiers"`
	EquipmentTemplates       []models.EquipmentTemplateConfig         `json:"equipment_templates"`
	EquipmentAffixes         []models.EquipmentAffixConfig            `json:"equipment_affixes"`
	EquipmentRecipes         []models.EquipmentRecipeConfig           `json:"equipment_recipes"`
	EquipmentRecipeMaterials []models.EquipmentRecipeMaterialConfig   `json:"equipment_recipe_materials"`
}

func CaptureAdventureCatalog(db *gorm.DB) (AdventureCatalog, error) {
	var result AdventureCatalog
	queries := []struct {
		target any
		order  string
	}{
		{&result.Maps, "sort_order asc, key asc"}, {&result.Zones, "map_key asc, sort_order asc, key asc"},
		{&result.Prerequisites, "zone_key asc, prerequisite_zone_key asc"}, {&result.Objectives, "zone_key asc, sort_order asc, key asc"},
		{&result.Monsters, "key asc"}, {&result.Skills, "key asc"}, {&result.MonsterSkills, "monster_key asc, sort_order asc, skill_key asc"},
		{&result.Encounters, "zone_key asc, sort_order asc, encounter_key asc"}, {&result.EncounterEffects, "encounter_key asc, effect_type asc, target_key asc"}, {&result.LootPools, "key asc"},
		{&result.LootEntries, "pool_key asc, sort_order asc, id asc"}, {&result.Expeditions, "zone_key asc"},
		{&result.Currencies, "sort_order asc, key asc"}, {&result.Items, "name asc, key asc"},
		{&result.ShopItems, "sort_order asc, key asc"},
		{&result.Bosses, "map_key asc, zone_key asc, key asc"}, {&result.BossRewardTiers, "boss_key asc, threshold asc"},
		{&result.EquipmentTemplates, "slot asc, rarity asc, key asc"}, {&result.EquipmentAffixes, "pool_key asc, key asc"},
		{&result.EquipmentRecipes, "equipment_key asc"}, {&result.EquipmentRecipeMaterials, "equipment_key asc, item_name asc"},
	}
	for _, query := range queries {
		if err := db.Order(query.order).Find(query.target).Error; err != nil {
			return result, err
		}
	}
	return result, nil
}

func assignAdventureCatalog(snapshot *ConfigSnapshot, catalog AdventureCatalog) {
	snapshot.AdventureMaps = catalog.Maps
	snapshot.AdventureZones = catalog.Zones
	snapshot.AdventurePrereqs = catalog.Prerequisites
	snapshot.AdventureObjectives = catalog.Objectives
	snapshot.AdventureMonsters = catalog.Monsters
	snapshot.AdventureSkills = catalog.Skills
	snapshot.AdventureMonsterSkills = catalog.MonsterSkills
	snapshot.AdventureEncounters = catalog.Encounters
	snapshot.AdventureEncounterEffects = catalog.EncounterEffects
	snapshot.AdventureLootPools = catalog.LootPools
	snapshot.AdventureLootEntries = catalog.LootEntries
	snapshot.Currencies = catalog.Currencies
	snapshot.Items = catalog.Items
	snapshot.AdventureShopItems = catalog.ShopItems
	snapshot.AdventureExpeditions = catalog.Expeditions
	snapshot.AdventureBosses = catalog.Bosses
	snapshot.AdventureBossRewardTiers = catalog.BossRewardTiers
	snapshot.EquipmentTemplates = catalog.EquipmentTemplates
	snapshot.EquipmentAffixes = catalog.EquipmentAffixes
	snapshot.EquipmentRecipes = catalog.EquipmentRecipes
	snapshot.EquipmentRecipeMaterials = catalog.EquipmentRecipeMaterials
}

func ValidateAdventureCatalog(db *gorm.DB, catalog AdventureCatalog) error {
	snapshot, err := CaptureSnapshot(db)
	if err != nil {
		return err
	}
	assignAdventureCatalog(&snapshot, catalog)
	return ValidateSnapshot(snapshot)
}

// ReplaceAdventureCatalog validates and atomically replaces only adventure
// configuration. Runtime/player tables are never read, deleted, or copied.
func ReplaceAdventureCatalog(tx *gorm.DB, catalog AdventureCatalog) error {
	if err := ValidateAdventureCatalog(tx, catalog); err != nil {
		return err
	}
	tables := []any{
		&models.EquipmentRecipeMaterialConfig{}, &models.EquipmentRecipeConfig{}, &models.EquipmentAffixConfig{}, &models.EquipmentTemplateConfig{},
		&models.AdventureBossRewardTierConfig{}, &models.AdventureBossConfig{}, &models.AdventureExpeditionConfig{},
		&models.AdventureShopItemConfig{}, &models.CurrencyConfig{}, &models.ItemConfig{},
		&models.AdventureLootEntryConfig{}, &models.AdventureLootPoolConfig{}, &models.AdventureEncounterEffectConfig{}, &models.AdventureEncounterConfig{},
		&models.AdventureMonsterSkillConfig{}, &models.AdventureSkillConfig{}, &models.AdventureMonsterConfig{},
		&models.AdventureObjectiveConfig{}, &models.AdventureZonePrerequisiteConfig{}, &models.AdventureZoneConfig{}, &models.AdventureMapConfig{},
	}
	for _, table := range tables {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(table).Error; err != nil {
			return err
		}
	}
	rows := []any{catalog.Maps, catalog.Zones, catalog.Prerequisites, catalog.Objectives, catalog.Monsters, catalog.Skills, catalog.MonsterSkills, catalog.Encounters, catalog.EncounterEffects, catalog.LootPools, catalog.LootEntries, catalog.Currencies, catalog.Items, catalog.ShopItems, catalog.Expeditions, catalog.Bosses, catalog.BossRewardTiers, catalog.EquipmentTemplates, catalog.EquipmentAffixes, catalog.EquipmentRecipes, catalog.EquipmentRecipeMaterials}
	for _, value := range rows {
		raw, _ := json.Marshal(value)
		if string(raw) == "[]" || string(raw) == "null" {
			continue
		}
		if err := tx.Create(value).Error; err != nil {
			return err
		}
	}
	return nil
}
