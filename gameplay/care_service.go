package gameplay

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

const PetStatusResting = "休养中"

type TreatmentResult struct {
	PetName          string
	HealthBefore     int64
	HealthAfter      int64
	Cost             int64
	CurrencyKey      string
	RemainingBalance int64
}

type CareService struct {
	DB     *gorm.DB
	Wallet *WalletService
}

func NewCareService(db *gorm.DB) *CareService {
	return &CareService{DB: db, Wallet: NewWalletService(db)}
}

func (service *CareService) PutToRest(ctx context.Context, accountID, expectedName string) (*models.PetProfile, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	expectedName = strings.TrimSpace(expectedName)
	var pet models.PetProfile
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		activePet, err := ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		pet = *activePet
		if pet.Name != expectedName {
			return ErrPetNameMismatch
		}
		if pet.Status == PetStatusResting {
			return ErrPetAlreadyResting
		}
		if pet.Status != "" && pet.Status != "空闲" {
			return ErrActionCooldown
		}
		var active int64
		if err := tx.Model(&models.ExpeditionRun{}).Where("account_id = ? AND status = ?", accountID, "running").Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return ErrActionCooldown
		}
		pet.Status = PetStatusResting
		return tx.Save(&pet).Error
	})
	return &pet, err
}

func (service *CareService) Resume(ctx context.Context, accountID string) (*models.PetProfile, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	var pet models.PetProfile
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		activePet, err := ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		pet = *activePet
		if pet.Status != PetStatusResting && pet.Status != "逃跑" {
			return ErrPetNotAway
		}
		pet.Status = "空闲"
		return tx.Save(&pet).Error
	})
	return &pet, err
}

func (service *CareService) Treat(ctx context.Context, accountID, currencyKey string, cost int64) (*TreatmentResult, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	if cost < 0 {
		return nil, ErrInvalidQuantity
	}
	currencyKey = normalizeCurrencyKey(currencyKey)
	result := &TreatmentResult{Cost: cost, CurrencyKey: currencyKey}
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		activePet, err := ActivePetTx(tx, accountID)
		if err != nil {
			return err
		}
		pet := *activePet
		healthMax := pet.HealthMax
		if healthMax <= 0 {
			healthMax = 100
		}
		needsTreat := pet.Health < healthMax || pet.Status == "濒死" || pet.Status == "受伤" || pet.Status == "探索" || pet.Status == "探索战斗"
		if !needsTreat {
			return ErrTreatmentNotNeeded
		}
		if cost > 0 {
			if err := service.wallet().DebitTxWithReason(tx, accountID, currencyKey, cost, "pet_treatment", pet.Name); err != nil {
				return err
			}
		}
		result.PetName = pet.Name
		result.HealthBefore = pet.Health
		result.HealthAfter = healthMax
		pet.Health = healthMax
		pet.HealthMax = healthMax
		switch pet.Status {
		case "濒死", "受伤", "探索", "探索战斗":
			pet.Status = "空闲"
		}
		if err := ReleasePetAdventureOccupancyTx(tx, accountID, pet.ID, time.Now()); err != nil {
			return err
		}
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		var wallet models.PlayerWallet
		find := tx.Where("account_id = ? AND currency_key = ?", accountID, currencyKey).Limit(1).Find(&wallet)
		if find.Error != nil {
			return find.Error
		}
		result.RemainingBalance = wallet.Balance
		return nil
	})
	return result, err
}

func (service *CareService) wallet() *WalletService {
	if service.Wallet == nil {
		service.Wallet = NewWalletService(service.DB)
	}
	return service.Wallet
}
