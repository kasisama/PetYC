package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

//go:embed defaults/config_v0.0.1.json defaults/assets
var officialDefaults embed.FS

func LoadOfficialSnapshot() (ConfigSnapshot, error) {
	raw, err := officialDefaults.ReadFile("defaults/config_v0.0.1.json")
	if err != nil {
		return ConfigSnapshot{}, err
	}
	return DecodeSnapshot(string(raw))
}

// EnsureOfficialDefaults initializes an empty database and makes the immutable
// official profile available on both new and upgraded installations.
func EnsureOfficialDefaults(db *gorm.DB) error {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		return fmt.Errorf("读取官方默认配置失败: %w", err)
	}
	var systemCount int64
	if err := db.Model(&models.SystemConfig{}).Count(&systemCount).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if systemCount == 0 {
			if err := ApplySnapshot(tx, snapshot); err != nil {
				return err
			}
		} else {
			var adventureCount int64
			if err := tx.Model(&models.AdventureMapConfig{}).Count(&adventureCount).Error; err != nil {
				return err
			}
			if adventureCount == 0 {
				catalog := AdventureCatalog{Maps: snapshot.AdventureMaps, Zones: snapshot.AdventureZones, Prerequisites: snapshot.AdventurePrereqs, Objectives: snapshot.AdventureObjectives, Monsters: snapshot.AdventureMonsters, Skills: snapshot.AdventureSkills, MonsterSkills: snapshot.AdventureMonsterSkills, Encounters: snapshot.AdventureEncounters, LootPools: snapshot.AdventureLootPools, LootEntries: snapshot.AdventureLootEntries, Expeditions: snapshot.AdventureExpeditions, Bosses: snapshot.AdventureBosses, BossRewardTiers: snapshot.AdventureBossRewardTiers, EquipmentTemplates: snapshot.EquipmentTemplates, EquipmentAffixes: snapshot.EquipmentAffixes, EquipmentRecipes: snapshot.EquipmentRecipes, EquipmentRecipeMaterials: snapshot.EquipmentRecipeMaterials}
				if err := ReplaceAdventureCatalog(tx, catalog); err != nil {
					return err
				}
			}
		}
		if err := migrateLegacyLiveEventChoices(tx); err != nil {
			return err
		}
		payload, err := EncodeSnapshot(snapshot)
		if err != nil {
			return err
		}
		profile := models.ConfigProfile{ID: OfficialProfileID, Name: "官方默认 v0.0.1", Description: "QQ-Pet v0.0.1 内置安全默认配置", Source: "official", SchemaVersion: ProfileSchemaVersion, AppVersion: ApplicationVersion, Payload: payload, Builtin: true}
		if err := tx.Where("id = ?", OfficialProfileID).Assign(map[string]any{"name": profile.Name, "description": profile.Description, "source": profile.Source, "schema_version": profile.SchemaVersion, "app_version": profile.AppVersion, "payload": profile.Payload, "builtin": true}).FirstOrCreate(&profile).Error; err != nil {
			return err
		}
		state, err := getOrCreateConfigState(tx)
		if err != nil {
			return err
		}
		if state.ActiveProfileID == "" {
			return tx.Model(state).Updates(map[string]any{"active_profile_id": OfficialProfileID, "profile_dirty": systemCount != 0}).Error
		}
		return nil
	})
}

func migrateLegacyLiveEventChoices(tx *gorm.DB) error {
	var events []models.LiveEventConfig
	if err := tx.Find(&events).Error; err != nil {
		return err
	}
	effects := []string{"community_material_gain_percent", "facility_upgrade_cost_reduction_percent", "boss_damage_gain_percent", "adventure_xp_gain_percent", "expedition_reward_gain_percent"}
	for _, event := range events {
		var count int64
		if err := tx.Model(&models.LiveEventChoiceConfig{}).Where("event_key = ?", event.Key).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		var labels []string
		if json.Unmarshal([]byte(event.StoryChoices), &labels) != nil || len(labels) < 2 || len(labels) > 5 {
			continue
		}
		for index, label := range labels {
			effect := ""
			value := 0
			if index < len(effects) {
				effect, value = effects[index], 20
			}
			choice := models.LiveEventChoiceConfig{EventKey: event.Key, ChoiceKey: fmt.Sprintf("choice-%d", index+1), Label: strings.TrimSpace(label), EffectType: effect, EffectValue: value, SortOrder: (index + 1) * 10}
			if err := tx.Create(&choice).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// ExtractOfficialAssets copies only the new reviewed default assets. It never
// reads the historical 初始数据 directory.
func ExtractOfficialAssets(targetRoot string) error {
	return fs.WalkDir(officialDefaults, "defaults/assets", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(path, "defaults/assets"), "/")
		if rel == "" {
			return nil
		}
		destination := filepath.Join(targetRoot, filepath.FromSlash(rel))
		if _, err := os.Stat(destination); err == nil {
			return nil
		}
		data, err := officialDefaults.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
}
