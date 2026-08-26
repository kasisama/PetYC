package gameplay

import (
	"strings"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

type occupyingRun struct {
	model  any
	status string
	column string
}

func occupyingRunSources() []occupyingRun {
	return []occupyingRun{
		{&models.ActivityRun{}, ActivityStatusRunning, "status"},
		{&models.ExpeditionRun{}, "running", "status"},
		{&models.FishingRun{}, "running", "status"},
		{&models.AdventureExplorationSession{}, "active", "status"},
		{&models.AdventureCombatSession{}, "active", "status"},
		{&models.AdventureExpeditionRun{}, "running", "status"},
	}
}

func occupiedPetIDsTx(tx *gorm.DB, accountID string) ([]string, error) {
	seen := map[string]struct{}{}
	for _, source := range occupyingRunSources() {
		if tx == nil || !tx.Migrator().HasTable(source.model) {
			continue
		}
		var ids []string
		if err := tx.Model(source.model).Where("account_id = ? AND "+source.column+" = ?", accountID, source.status).Pluck("pet_id", &ids).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "no such table") || strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
				continue
			}
			return nil, err
		}
		for _, id := range ids {
			if id = strings.TrimSpace(id); id != "" {
				seen[id] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	return result, nil
}

func CheckAccountRunCapacityTx(tx *gorm.DB, accountID string) error {
	occupied, err := occupiedPetIDsTx(tx, accountID)
	if err != nil {
		return err
	}
	limit := systemPositiveIntTx(tx, MaxConcurrentRunsConfigKey, 1)
	if len(occupied) >= limit {
		return ErrTooManyConcurrentRuns
	}
	return nil
}

func CheckPetAvailableTx(tx *gorm.DB, accountID, petID string) error {
	pet, err := PetByIDTx(tx, accountID, petID)
	if err != nil {
		return err
	}
	if pet.Status != "" && pet.Status != "空闲" {
		return ErrActivityActive
	}
	occupied, err := occupiedPetIDsTx(tx, accountID)
	if err != nil {
		return err
	}
	for _, id := range occupied {
		if id == pet.ID {
			return ErrActivityActive
		}
	}
	return nil
}

func ReservePetRunTx(tx *gorm.DB, accountID, petID string) error {
	if err := CheckPetAvailableTx(tx, accountID, petID); err != nil {
		return err
	}
	return CheckAccountRunCapacityTx(tx, accountID)
}
