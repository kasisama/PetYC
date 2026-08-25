package gameplay

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/models"
)

type InventoryService struct {
	DB  *gorm.DB
	Now func() time.Time
}

func NewInventoryService(db *gorm.DB) *InventoryService {
	return &InventoryService{DB: db, Now: time.Now}
}

func (service *InventoryService) Credit(ctx context.Context, accountID, itemName string, quantity int64) error {
	if service == nil || service.DB == nil {
		return ErrDatabaseUnavailable
	}
	return service.CreditTx(service.DB.WithContext(ctx), accountID, itemName, quantity)
}

func (service *InventoryService) CreditTx(tx *gorm.DB, accountID, itemName string, quantity int64) error {
	itemName = strings.TrimSpace(itemName)
	if quantity <= 0 || accountID == "" || itemName == "" {
		return ErrInvalidQuantity
	}
	now := service.now()
	item := models.GlobalInventoryItem{AccountID: accountID, ItemName: itemName, Quantity: quantity, UpdatedAt: now}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "account_id"}, {Name: "item_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"quantity":   gorm.Expr("quantity + ?", quantity),
			"updated_at": now,
		}),
	}).Create(&item).Error
}

func (service *InventoryService) Debit(ctx context.Context, accountID, itemName string, quantity int64) error {
	if service == nil || service.DB == nil {
		return ErrDatabaseUnavailable
	}
	return service.DebitTx(service.DB.WithContext(ctx), accountID, itemName, quantity)
}

func (service *InventoryService) DebitTx(tx *gorm.DB, accountID, itemName string, quantity int64) error {
	itemName = strings.TrimSpace(itemName)
	if quantity <= 0 || accountID == "" || itemName == "" {
		return ErrInvalidQuantity
	}
	result := tx.Model(&models.GlobalInventoryItem{}).
		Where("account_id = ? AND item_name = ? AND quantity >= ?", accountID, itemName, quantity).
		Updates(map[string]interface{}{"quantity": gorm.Expr("quantity - ?", quantity), "updated_at": service.now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInsufficientItem
	}
	return nil
}

func (service *InventoryService) Transfer(ctx context.Context, fromAccountID, toAccountID, itemName string, quantity int64) error {
	if fromAccountID == toAccountID {
		return ErrInvalidQuantity
	}
	return service.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := service.DebitTx(tx, fromAccountID, itemName, quantity); err != nil {
			return err
		}
		return service.CreditTx(tx, toAccountID, itemName, quantity)
	})
}

func (service *InventoryService) List(ctx context.Context, accountID string, limit int) ([]models.GlobalInventoryItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	items := make([]models.GlobalInventoryItem, 0)
	err := service.DB.WithContext(ctx).Where("account_id = ? AND quantity > 0", accountID).
		Order("item_name").Limit(limit).Find(&items).Error
	return items, err
}

func (service *InventoryService) now() time.Time {
	if service != nil && service.Now != nil {
		return service.Now()
	}
	return time.Now()
}
