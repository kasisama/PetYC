package gameplay

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

type ItemUseResult struct {
	Record    models.ItemUseRecord
	PetName   string
	Image     string
	Duplicate bool
}

type ItemEffectService struct {
	DB        *gorm.DB
	Inventory *InventoryService
}

func NewItemEffectService(db *gorm.DB) *ItemEffectService {
	return &ItemEffectService{DB: db, Inventory: NewInventoryService(db)}
}

func (service *ItemEffectService) Use(ctx context.Context, accountID, itemName string, quantity int64, idempotencyKey string) (*ItemUseResult, error) {
	return service.UseOn(ctx, accountID, "", itemName, quantity, idempotencyKey)
}

func (service *ItemEffectService) UseOn(ctx context.Context, accountID, petID, itemName string, quantity int64, idempotencyKey string) (*ItemUseResult, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	itemName = strings.TrimSpace(itemName)
	if itemName == "" || quantity <= 0 || quantity > 9999 {
		return nil, ErrInvalidQuantity
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewString()
	}
	if existing, found, err := service.findExisting(ctx, accountID, idempotencyKey); err != nil {
		return nil, err
	} else if found {
		existing.Duplicate = true
		return existing, nil
	}
	result := &ItemUseResult{}
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		activePet, err := PetByIDTx(tx, accountID, petID)
		if err != nil {
			return err
		}
		pet := *activePet
		if pet.Status != "" && pet.Status != "空闲" && pet.Status != "濒死" {
			return ErrActivityActive
		}
		item, err := getItem(tx, itemName)
		if err != nil {
			return err
		}
		effect, err := strconv.ParseInt(strings.TrimSpace(item.Effect), 10, 64)
		if err != nil || effect <= 0 {
			return ErrWrongItemType
		}
		amount := effect * quantity
		record := models.ItemUseRecord{
			ID: uuid.NewString(), AccountID: accountID, PetID: pet.ID, IdempotencyKey: idempotencyKey,
			ItemName: item.Name, Quantity: quantity, EffectType: item.Type, CreatedAt: service.inventory().now(),
		}
		switch strings.TrimSpace(item.Type) {
		case "血量":
			maximum := pet.HealthMax
			if maximum <= 0 {
				maximum = 100
			}
			if pet.Health >= maximum && pet.Status != "濒死" {
				return ErrTreatmentNotNeeded
			}
			record.BeforeValue = pet.Health
			pet.Health = minInt64(maximum, pet.Health+amount)
			if pet.Health > 0 && pet.Status == "濒死" {
				pet.Status = "空闲"
			}
			record.AfterValue = pet.Health
		case "成长":
			record.BeforeValue = pet.Growth
			pet.Growth += amount
			record.AfterValue = pet.Growth
		default:
			return ErrWrongItemType
		}
		record.AppliedAmount = record.AfterValue - record.BeforeValue
		if err := service.inventory().DebitTx(tx, accountID, item.Name, quantity); err != nil {
			return err
		}
		if err := tx.Save(&pet).Error; err != nil {
			return err
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		result.Record = record
		result.PetName = pet.Name
		result.Image = item.Image
		return nil
	})
	if err != nil && isUniqueConstraint(err) {
		if existing, found, findErr := service.findExisting(ctx, accountID, idempotencyKey); findErr != nil {
			return nil, findErr
		} else if found {
			existing.Duplicate = true
			return existing, nil
		}
	}
	return result, err
}

func (service *ItemEffectService) findExisting(ctx context.Context, accountID, key string) (*ItemUseResult, bool, error) {
	var record models.ItemUseRecord
	find := service.DB.WithContext(ctx).Limit(1).Find(&record, "account_id = ? AND idempotency_key = ?", accountID, key)
	if find.Error != nil {
		return nil, false, find.Error
	}
	if find.RowsAffected == 0 {
		return nil, false, nil
	}
	petRow, err := PetByIDTx(service.DB.WithContext(ctx), accountID, record.PetID)
	if err != nil {
		return nil, false, err
	}
	pet := *petRow
	var item models.ItemConfig
	if err := service.DB.WithContext(ctx).First(&item, "name = ?", record.ItemName).Error; err != nil {
		return nil, false, err
	}
	return &ItemUseResult{Record: record, PetName: pet.Name, Image: item.Image}, true, nil
}

func (service *ItemEffectService) inventory() *InventoryService {
	if service.Inventory == nil {
		service.Inventory = NewInventoryService(service.DB)
	}
	return service.Inventory
}
