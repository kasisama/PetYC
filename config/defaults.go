package config

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

//go:embed defaults/config_v0.1.0.json defaults/assets
var officialDefaults embed.FS

func LoadOfficialSnapshot() (ConfigSnapshot, error) {
	raw, err := officialDefaults.ReadFile("defaults/config_v0.1.0.json")
	if err != nil {
		return ConfigSnapshot{}, err
	}
	return DecodeSnapshot(string(raw))
}

func officialProfile(snapshot ConfigSnapshot, payload string) models.ConfigProfile {
	return models.ConfigProfile{ID: OfficialProfileID, Name: "原创调查默认 v0.1.0", Description: "自然遗迹调查首季 70 天默认运营配置", Source: "official", SchemaVersion: ProfileSchemaVersion, AppVersion: ApplicationVersion, Payload: payload, Builtin: true}
}

func upsertOfficialProfile(tx *gorm.DB, snapshot ConfigSnapshot) error {
	payload, err := EncodeSnapshot(snapshot)
	if err != nil {
		return err
	}
	profile := officialProfile(snapshot, payload)
	return tx.Where("id = ?", OfficialProfileID).Assign(map[string]any{"name": profile.Name, "description": profile.Description, "source": profile.Source, "schema_version": profile.SchemaVersion, "app_version": profile.AppVersion, "payload": profile.Payload, "builtin": true}).FirstOrCreate(&profile).Error
}

func activateOfficialProfile(tx *gorm.DB) error {
	state, err := getOrCreateConfigState(tx)
	if err != nil {
		return err
	}
	return tx.Model(state).Updates(map[string]any{"active_profile_id": OfficialProfileID, "profile_dirty": false}).Error
}

// EnsureOfficialDefaults only seeds an empty database. Existing operator
// databases keep their live tables; the official profile metadata is refreshed
// so it can be activated explicitly.
func EnsureOfficialDefaults(db *gorm.DB) error {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		return fmt.Errorf("读取官方默认配置失败: %w", err)
	}
	var systemCount, profileCount int64
	if err := db.Model(&models.SystemConfig{}).Count(&systemCount).Error; err != nil {
		return err
	}
	if err := db.Model(&models.ConfigProfile{}).Count(&profileCount).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if systemCount == 0 && profileCount == 0 {
			if err := ApplySnapshot(tx, snapshot); err != nil {
				return err
			}
			if err := AnchorSeasonSchedule(tx, time.Now()); err != nil {
				return err
			}
		}
		if err := upsertOfficialProfile(tx, snapshot); err != nil {
			return err
		}
		state, err := getOrCreateConfigState(tx)
		if err != nil {
			return err
		}
		if state.ActiveProfileID == "" {
			return activateOfficialProfile(tx)
		}
		return nil
	})
}

// RebuildOfficialDefaults replaces live configuration with the official v0.1.0
// snapshot. Player rows are not deleted.
func RebuildOfficialDefaults(db *gorm.DB) error {
	snapshot, err := LoadOfficialSnapshot()
	if err != nil {
		return fmt.Errorf("读取官方默认配置失败: %w", err)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := ApplySnapshot(tx, snapshot); err != nil {
			return err
		}
		if err := AnchorSeasonSchedule(tx, time.Now()); err != nil {
			return err
		}
		if err := upsertOfficialProfile(tx, snapshot); err != nil {
			return err
		}
		return activateOfficialProfile(tx)
	})
}

func AnchorSeasonSchedule(tx *gorm.DB, now time.Time) error {
	if tx == nil || !tx.Migrator().HasTable(&models.LiveEventConfig{}) {
		return nil
	}
	var events []models.LiveEventConfig
	if err := tx.Find(&events).Error; err != nil {
		return err
	}
	for _, event := range events {
		duration := event.EndsAt.Sub(event.StartsAt)
		if duration <= 0 {
			duration = 70 * 24 * time.Hour
		}
		if err := tx.Model(&models.LiveEventConfig{}).Where("key = ?", event.Key).Updates(map[string]any{"starts_at": now, "ends_at": now.Add(duration)}).Error; err != nil {
			return err
		}
	}
	if !tx.Migrator().HasTable(&models.AdventureBossConfig{}) {
		return nil
	}
	var bosses []models.AdventureBossConfig
	if err := tx.Order("recommended_level asc, key asc").Find(&bosses).Error; err != nil {
		return err
	}
	offsets := []time.Duration{10 * 24 * time.Hour, 31 * 24 * time.Hour, 56 * 24 * time.Hour}
	for index, boss := range bosses {
		offset := offsets[index%len(offsets)]
		if err := tx.Model(&models.AdventureBossConfig{}).Where("key = ?", boss.Key).Update("schedule_anchor", now.Add(offset)).Error; err != nil {
			return err
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
