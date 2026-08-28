package gameplay

import (
	"encoding/json"
	"strings"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func UnlockedAdventureSkills(tx *gorm.DB, formKey string, adventureLevel int) []string {
	formKey = strings.TrimSpace(formKey)
	if tx == nil || formKey == "" || !tx.Migrator().HasTable(&models.PetSkillUnlockConfig{}) {
		return nil
	}
	if adventureLevel < 1 {
		adventureLevel = 1
	}
	rows := make([]models.PetSkillUnlockConfig, 0)
	if err := tx.Where("form_key = ? AND unlock_level <= ?", formKey, adventureLevel).
		Order("unlock_level asc, sort_order asc, skill_key asc").Find(&rows).Error; err != nil {
		return nil
	}
	keys := make([]string, 0, len(rows))
	seen := map[string]struct{}{}
	for _, row := range rows {
		key := strings.TrimSpace(row.SkillKey)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func AdventureLevelTx(tx *gorm.DB, accountID string) int {
	if tx == nil || !tx.Migrator().HasTable(&models.PlayerAdventureProgress{}) {
		return 1
	}
	var progress models.PlayerAdventureProgress
	lookup := tx.Limit(1).Find(&progress, "account_id = ?", accountID)
	if lookup.Error != nil || lookup.RowsAffected == 0 || progress.Level < 1 {
		return 1
	}
	return progress.Level
}

func RefreshPetSkillsTx(tx *gorm.DB, pet *models.PetProfile) error {
	if tx == nil || pet == nil {
		return nil
	}
	formKey := strings.TrimSpace(pet.CurrentForm)
	if formKey == "" {
		formKey = strings.TrimSpace(pet.PetType)
	}
	keys := UnlockedAdventureSkills(tx, formKey, AdventureLevelTx(tx, pet.AccountID))
	encoded, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	pet.Skills = string(encoded)
	if strings.TrimSpace(pet.ID) == "" {
		return nil
	}
	return tx.Model(&models.PetProfile{}).Where("id = ?", pet.ID).Update("skills", pet.Skills).Error
}
