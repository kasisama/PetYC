package gameplay

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

const (
	MaxPetSlotsConfigKey       = "Core.MaxPetSlots"
	MaxConcurrentRunsConfigKey = "Core.MaxConcurrentRuns"
)

func ActivePetTx(tx *gorm.DB, accountID string) (*models.PetProfile, error) {
	if tx == nil || strings.TrimSpace(accountID) == "" {
		return nil, ErrPetRequired
	}
	var account models.PlayerAccount
	if result := tx.Limit(1).Find(&account, "id = ?", accountID); result.Error != nil {
		return nil, result.Error
	} else if result.RowsAffected > 0 && account.ActivePetID != "" {
		var pet models.PetProfile
		if err := tx.First(&pet, "id = ? AND account_id = ?", account.ActivePetID, accountID).Error; err == nil {
			return &pet, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	var pet models.PetProfile
	if err := tx.Where("account_id = ?", accountID).Order("created_at asc, id asc").First(&pet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPetRequired
		}
		return nil, err
	}
	if account.ID != "" && account.ActivePetID != pet.ID {
		if err := tx.Model(&models.PlayerAccount{}).Where("id = ?", accountID).Update("active_pet_id", pet.ID).Error; err != nil {
			return nil, err
		}
	}
	return &pet, nil
}

func ActivePet(ctx context.Context, db *gorm.DB, accountID string) (*models.PetProfile, error) {
	if db == nil {
		return nil, ErrDatabaseUnavailable
	}
	return ActivePetTx(db.WithContext(ctx), accountID)
}

func PetByIDTx(tx *gorm.DB, accountID, petID string) (*models.PetProfile, error) {
	if strings.TrimSpace(petID) == "" {
		return ActivePetTx(tx, accountID)
	}
	var pet models.PetProfile
	if err := tx.First(&pet, "id = ? AND account_id = ?", petID, accountID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPetRequired
		}
		return nil, err
	}
	return &pet, nil
}

func MaxPetSlotsTx(tx *gorm.DB) int {
	return systemPositiveIntTx(tx, MaxPetSlotsConfigKey, 1)
}

func MaxConcurrentRunsTx(tx *gorm.DB) int {
	return systemPositiveIntTx(tx, MaxConcurrentRunsConfigKey, 1)
}

func systemPositiveIntTx(tx *gorm.DB, key string, fallback int) int {
	if tx == nil || !tx.Migrator().HasTable(&models.SystemConfig{}) {
		return fallback
	}
	var row models.SystemConfig
	if result := tx.Limit(1).Find(&row, "key = ?", key); result.Error == nil && result.RowsAffected > 0 {
		if value, err := strconv.Atoi(strings.TrimSpace(row.Value)); err == nil && value > 0 {
			return value
		}
	}
	return fallback
}
