package expedition

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

const JourneyBadgeCurrencyKey = gameplay.JourneyBadgeCurrencyKey

var (
	ErrAdventureItemNotFound = errors.New("没有找到这个远征物品")
	ErrAdventureItemShort    = errors.New("远征材料数量不足")
	ErrJourneyBadgeShort     = errors.New("旅途徽章不足")
	ErrAdventureShopItem     = errors.New("没有找到这个远征商品")
	ErrAdventureShopLimit    = errors.New("本周期购买数量已达上限")
)

type AdventureShopResult struct {
	Listing          models.AdventureShopItemConfig `json:"listing"`
	Purchase         models.AdventureShopPurchase   `json:"purchase"`
	RemainingBalance int64                          `json:"remaining_balance"`
	Equipment        []*models.PlayerEquipment      `json:"equipment,omitempty"`
}

func creditAdventureItemTx(tx *gorm.DB, accountID, itemKey string, quantity int64, now time.Time) error {
	if quantity <= 0 {
		return errors.New("远征物品发放数量必须大于零")
	}
	var definition models.ItemConfig
	if err := tx.First(&definition, "key = ? AND status IN ?", itemKey, []string{"active", "limited", "hidden"}).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAdventureItemNotFound
		}
		return err
	}
	var row models.GlobalInventoryItem
	lookup := tx.Limit(1).Find(&row, "account_id = ? AND item_key = ?", accountID, itemKey)
	if lookup.Error != nil {
		return lookup.Error
	}
	if quantity > math.MaxInt64-row.Quantity || row.Quantity+quantity > definition.MaxStack {
		return fmt.Errorf("远征物品 %s 超过堆叠上限", definition.Name)
	}
	inventory := gameplay.NewInventoryService(tx)
	inventory.Now = func() time.Time { return now }
	return inventory.CreditTx(tx, accountID, itemKey, quantity)
}

func debitAdventureItemTx(tx *gorm.DB, accountID, itemKey string, quantity int64, now time.Time) error {
	if quantity <= 0 {
		return errors.New("远征物品扣除数量必须大于零")
	}
	inventory := gameplay.NewInventoryService(tx)
	inventory.Now = func() time.Time { return now }
	if err := inventory.DebitTx(tx, accountID, itemKey, quantity); err != nil {
		if errors.Is(err, gameplay.ErrInsufficientItem) {
			return ErrAdventureItemShort
		}
		return err
	}
	return nil
}

func adventureWalletBalanceTx(tx *gorm.DB, accountID string) (models.PlayerWallet, error) {
	return currencyWalletBalanceTx(tx, accountID, JourneyBadgeCurrencyKey)
}

func currencyWalletBalanceTx(tx *gorm.DB, accountID, currencyKey string) (models.PlayerWallet, error) {
	var wallet models.PlayerWallet
	lookup := tx.Limit(1).Find(&wallet, "account_id = ? AND currency_key = ?", accountID, currencyKey)
	if lookup.Error != nil {
		return wallet, lookup.Error
	}
	if lookup.RowsAffected == 0 {
		wallet = models.PlayerWallet{AccountID: accountID, CurrencyKey: currencyKey, Balance: 0}
		if err := tx.Create(&wallet).Error; err != nil {
			return wallet, err
		}
	}
	return wallet, nil
}

func creditAdventureCurrencyTx(tx *gorm.DB, accountID string, quantity int64, reason, reference string, now time.Time) (int64, error) {
	if quantity <= 0 {
		return 0, errors.New("旅途徽章发放数量必须大于零")
	}
	walletService := gameplay.NewWalletService(tx)
	walletService.Now = func() time.Time { return now }
	if err := walletService.CreditTxWithReason(tx, accountID, JourneyBadgeCurrencyKey, quantity, reason, reference); err != nil {
		return 0, err
	}
	wallet, err := adventureWalletBalanceTx(tx, accountID)
	return wallet.Balance, err
}

func debitAdventureCurrencyTx(tx *gorm.DB, accountID string, quantity int64, reason, reference string, now time.Time) (int64, error) {
	return debitConfiguredCurrencyTx(tx, accountID, JourneyBadgeCurrencyKey, quantity, reason, reference, now)
}

func debitConfiguredCurrencyTx(tx *gorm.DB, accountID, currencyKey string, quantity int64, reason, reference string, now time.Time) (int64, error) {
	if quantity <= 0 {
		return 0, errors.New("货币扣除数量必须大于零")
	}
	walletService := gameplay.NewWalletService(tx)
	walletService.Now = func() time.Time { return now }
	if err := walletService.DebitTxWithReason(tx, accountID, currencyKey, quantity, reason, reference); err != nil {
		if errors.Is(err, gameplay.ErrInsufficientFunds) {
			return 0, ErrJourneyBadgeShort
		}
		return 0, err
	}
	wallet, err := currencyWalletBalanceTx(tx, accountID, currencyKey)
	return wallet.Balance, err
}

func adventureShopPeriodKeyTx(tx *gorm.DB, now time.Time, limitType string) string {
	switch limitType {
	case "daily":
		return now.Format("2006-01-02")
	case "weekly":
		year, week := now.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", year, week)
	case "season":
		var event models.LiveEventConfig
		if result := tx.Limit(1).Find(&event, "active = ? AND starts_at <= ? AND ends_at > ?", true, now, now); result.Error == nil && result.RowsAffected > 0 {
			return "season:" + event.Key
		}
		return "season:none"
	case "lifetime":
		return "lifetime"
	default:
		return "none"
	}
}

func adventureShopRemainingTx(tx *gorm.DB, accountID string, listing models.AdventureShopItemConfig, now time.Time) (int64, error) {
	if listing.LimitType == "none" {
		return -1, nil
	}
	var used int64
	if err := tx.Model(&models.AdventureShopPurchase{}).
		Where("account_id = ? AND shop_item_key = ? AND period_key = ?", accountID, listing.Key, adventureShopPeriodKeyTx(tx, now, listing.LimitType)).
		Select("COALESCE(SUM(purchase_units), 0)").Scan(&used).Error; err != nil {
		return 0, err
	}
	remaining := listing.LimitQuantity - used
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

func (service *Service) PurchaseAdventureShop(ctx context.Context, accountID, listingKey string, units int64, idempotencyKey string) (AdventureShopResult, error) {
	result := AdventureShopResult{}
	listingKey, idempotencyKey = strings.TrimSpace(listingKey), strings.TrimSpace(idempotencyKey)
	if listingKey == "" || units <= 0 || idempotencyKey == "" {
		return result, errors.New("购买参数无效")
	}
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var existing models.AdventureShopPurchase
		lookup := tx.Limit(1).Find(&existing, "account_id = ? AND idempotency_key = ?", accountID, idempotencyKey)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected > 0 {
			if err := tx.First(&result.Listing, "key = ?", existing.ShopItemKey).Error; err != nil {
				return err
			}
			wallet, err := currencyWalletBalanceTx(tx, accountID, existing.CurrencyKey)
			if err != nil {
				return err
			}
			result.Purchase, result.RemainingBalance = existing, wallet.Balance
			return nil
		}
		if err := tx.First(&result.Listing, "key = ? AND enabled = ?", listingKey, true).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAdventureShopItem
			}
			return err
		}
		if units > math.MaxInt64/result.Listing.Quantity || units > math.MaxInt64/result.Listing.Price {
			return errors.New("购买数量过大")
		}
		granted, cost := units*result.Listing.Quantity, units*result.Listing.Price
		now := service.Now()
		currencyKey := strings.TrimSpace(result.Listing.CurrencyKey)
		if currencyKey == "" {
			currencyKey = JourneyBadgeCurrencyKey
		}
		periodKey := adventureShopPeriodKeyTx(tx, now, result.Listing.LimitType)
		if result.Listing.LimitType != "none" {
			remaining, remainingErr := adventureShopRemainingTx(tx, accountID, result.Listing, now)
			if remainingErr != nil {
				return remainingErr
			}
			if units > remaining {
				return ErrAdventureShopLimit
			}
		}
		balance, err := debitConfiguredCurrencyTx(tx, accountID, currencyKey, cost, "adventure_shop", listingKey, now)
		if err != nil {
			return err
		}
		switch result.Listing.ProductType {
		case "item":
			if err = creditAdventureItemTx(tx, accountID, result.Listing.ProductKey, granted, now); err != nil {
				return err
			}
		case "equipment":
			result.Equipment = make([]*models.PlayerEquipment, 0, granted)
			for index := int64(0); index < granted; index++ {
				equipment, createErr := service.createEquipmentTx(tx, accountID, result.Listing.ProductKey, "shop:"+listingKey)
				if createErr != nil {
					return createErr
				}
				result.Equipment = append(result.Equipment, equipment)
			}
		case "blueprint_fragment":
			if err = service.grantBlueprintFragmentsTx(tx, accountID, result.Listing.ProductKey, granted); err != nil {
				return err
			}
		default:
			return ErrAdventureShopItem
		}
		result.Purchase = models.AdventureShopPurchase{ID: uuid.NewString(), AccountID: accountID, ShopItemKey: listingKey, PurchaseUnits: units, GrantedQuantity: granted, Cost: cost, CurrencyKey: currencyKey, PeriodKey: periodKey, IdempotencyKey: idempotencyKey, CreatedAt: now}
		if err = tx.Create(&result.Purchase).Error; err != nil {
			return err
		}
		result.RemainingBalance = balance
		return nil
	})
	return result, err
}
