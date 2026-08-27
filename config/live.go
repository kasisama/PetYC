package config

import (
	"strconv"
	"strings"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

// LiveString reads a SystemConfig row. Command handlers should use this
// instead of the process-wide Core/Interaction maps so admin saves take
// effect on the next message without a memory reload.
func LiveString(db *gorm.DB, key, fallback string) string {
	if db == nil || !db.Migrator().HasTable(&models.SystemConfig{}) {
		return fallback
	}
	var row models.SystemConfig
	if result := db.Limit(1).Find(&row, "key = ?", key); result.Error != nil || result.RowsAffected == 0 {
		return fallback
	}
	value := strings.TrimSpace(row.Value)
	if value == "" {
		return fallback
	}
	return value
}

func LiveInt64(db *gorm.DB, key string, fallback int64) int64 {
	raw := LiveString(db, key, "")
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func LiveImagePath(db *gorm.DB, name string, fallbacks ...string) string {
	if db != nil && db.Migrator().HasTable(&models.ImageConfig{}) {
		var row models.ImageConfig
		if result := db.Limit(1).Find(&row, "name = ?", name); result.Error == nil && result.RowsAffected > 0 {
			if path := strings.TrimSpace(row.Path); path != "" {
				return path
			}
		}
	}
	for _, fallback := range fallbacks {
		if strings.TrimSpace(fallback) != "" {
			return fallback
		}
	}
	return ""
}

func LiveStarterSpecies(db *gorm.DB) []models.PetSpeciesConfig {
	names := SplitConfigList(LiveString(db, "Core.InitialPets", "光芽兽"))
	if len(names) == 0 {
		names = []string{"光芽兽"}
	}
	if db == nil || !db.Migrator().HasTable(&models.PetSpeciesConfig{}) {
		result := make([]models.PetSpeciesConfig, 0, len(names))
		for _, name := range names {
			result = append(result, models.PetSpeciesConfig{Key: name, Name: name})
		}
		return result
	}
	result := make([]models.PetSpeciesConfig, 0, len(names))
	for _, name := range names {
		var species models.PetSpeciesConfig
		lookup := db.Limit(1).Find(&species, "key = ? OR name = ?", name, name)
		if lookup.Error != nil || lookup.RowsAffected == 0 {
			result = append(result, models.PetSpeciesConfig{Key: name, Name: name})
			continue
		}
		result = append(result, species)
	}
	return result
}

func LiveImageHost(db *gorm.DB) string {
	if db != nil && db.Migrator().HasTable(&models.SystemConfig{}) {
		return strings.TrimRight(LiveString(db, "Core.ImageHost", ""), "/")
	}
	return strings.TrimRight(strings.TrimSpace(Core.ImageHost), "/")
}

func LiveSpecies(db *gorm.DB, name string) (models.PetSpeciesConfig, bool) {
	name = strings.TrimSpace(name)
	if name == "" || db == nil || !db.Migrator().HasTable(&models.PetSpeciesConfig{}) {
		return models.PetSpeciesConfig{}, false
	}
	var species models.PetSpeciesConfig
	lookup := db.Limit(1).Find(&species, "key = ? OR name = ?", name, name)
	if lookup.Error != nil || lookup.RowsAffected == 0 {
		return models.PetSpeciesConfig{}, false
	}
	return species, true
}
