package gameplay

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"qq-pet-saas/models"
)

const (
	ShopTypeNormal    = "shop_normal"
	ShopTypeAffection = "shop_affection"
)

type ShopListing struct {
	ID          uint
	ShopType    string
	Name        string
	Image       string
	Stock       int64
	Price       int64
	DailyLimit  int64
	WeeklyLimit int64
	Description string
}

type ShopLimitRemaining struct {
	Daily  int64
	Weekly int64
}

type ShopPage struct {
	Items      []ShopListing
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
}

type PurchaseResult struct {
	Listing          ShopListing
	Quantity         int64
	Cost             int64
	CurrencyKey      string
	RemainingBalance int64
	RemainingStock   int64
}

type SellResult struct {
	Item             models.ItemConfig
	Quantity         int64
	Revenue          int64
	CurrencyKey      string
	RemainingBalance int64
}

type ShopService struct {
	DB        *gorm.DB
	Now       func() time.Time
	Inventory *InventoryService
	Wallet    *WalletService
}

func NewShopService(db *gorm.DB) *ShopService {
	return &ShopService{
		DB: db, Now: time.Now,
		Inventory: NewInventoryService(db), Wallet: NewWalletService(db),
	}
}

func (service *ShopService) List(ctx context.Context, shopType string, page, pageSize int) (*ShopPage, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	shopType = normalizeShopType(shopType)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 5
	}
	query := service.DB.WithContext(ctx).Model(&models.ShopItemConfig{}).
		Where("shop_type = ?", shopType).
		Where(`EXISTS (SELECT 1 FROM item_configs
			WHERE item_configs.name = shop_item_configs.name
			AND (item_configs.status = '' OR item_configs.status IN ('active', 'limited')))`)
	result := &ShopPage{Page: page, PageSize: pageSize, Items: make([]ShopListing, 0)}
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, err
	}
	if result.Total == 0 {
		return result, nil
	}
	result.TotalPages = int((result.Total + int64(pageSize) - 1) / int64(pageSize))
	if page > result.TotalPages {
		result.Page = result.TotalPages
	}
	var rows []models.ShopItemConfig
	if err := query.Order("name").Offset((result.Page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result.Items = append(result.Items, listingFromModel(row))
	}
	return result, nil
}

func (service *ShopService) GetListing(ctx context.Context, name string) (*ShopListing, error) {
	return service.getListing(service.DB.WithContext(ctx), name, "")
}

func (service *ShopService) GetItem(ctx context.Context, name string) (*models.ItemConfig, error) {
	return getItem(service.DB.WithContext(ctx), name)
}

func (service *ShopService) Purchase(ctx context.Context, accountID, itemName string, quantity int64) (*PurchaseResult, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	if quantity <= 0 || quantity > 9999 {
		return nil, ErrInvalidQuantity
	}
	result := &PurchaseResult{}
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		*result = PurchaseResult{}
		if err := requirePet(tx, accountID); err != nil {
			return err
		}
		listing, err := service.getListing(tx, itemName, "")
		if err != nil {
			return err
		}
		if err = service.enforcePurchaseLimitsTx(tx, accountID, listing, quantity); err != nil {
			return err
		}
		item, err := getItem(tx, listing.Name)
		if err != nil {
			return err
		}
		if item.ObtainType == 1 {
			if quantity != 1 {
				return ErrOneTimeItem
			}
			var owned int64
			if err = tx.Model(&models.GlobalInventoryItem{}).
				Where("account_id = ? AND item_name = ? AND quantity > 0", accountID, item.Name).
				Count(&owned).Error; err != nil {
				return err
			}
			if owned > 0 {
				return ErrOneTimeItem
			}
		}
		if listing.Price < 0 || (listing.Price > 0 && quantity > math.MaxInt64/listing.Price) {
			return ErrInvalidQuantity
		}
		cost := listing.Price * quantity
		if listing.Stock != -1 {
			stockUpdate := tx.Model(&models.ShopItemConfig{}).
				Where("id = ? AND stock >= ?", listing.ID, quantity).
				Update("stock", gorm.Expr("stock - ?", quantity))
			if stockUpdate.Error != nil {
				return stockUpdate.Error
			}
			if stockUpdate.RowsAffected != 1 {
				return ErrOutOfStock
			}
		}

		currencyKey := DefaultCurrencyKey
		if listing.ShopType == ShopTypeAffection {
			currencyKey = "好感"
			activePet, petErr := ActivePetTx(tx, accountID)
			if petErr != nil {
				return petErr
			}
			payment := tx.Model(&models.PetProfile{}).
				Where("id = ? AND affection >= ?", activePet.ID, cost).
				Update("affection", gorm.Expr("affection - ?", cost))
			if payment.Error != nil {
				return payment.Error
			}
			if payment.RowsAffected != 1 {
				return ErrInsufficientFunds
			}
		} else if cost > 0 {
			if err = service.wallet().DebitTxWithReason(tx, accountID, currencyKey, cost, "shop_purchase", listing.Name); err != nil {
				return err
			}
		}
		if err = service.inventory().CreditTx(tx, accountID, item.Name, quantity); err != nil {
			return err
		}
		if err = tx.Create(&models.ShopPurchaseLog{
			AccountID: accountID, ShopItemID: listing.ID, ItemName: listing.Name,
			Quantity: quantity, CreatedAt: service.now(),
		}).Error; err != nil {
			return err
		}
		result.Listing = *listing
		result.Quantity = quantity
		result.Cost = cost
		result.CurrencyKey = currencyKey
		if listing.Stock == -1 {
			result.RemainingStock = -1
		} else {
			result.RemainingStock = listing.Stock - quantity
		}
		if listing.ShopType == ShopTypeAffection {
			activePet, petErr := ActivePetTx(tx, accountID)
			if petErr != nil {
				return petErr
			}
			pet := *activePet
			result.RemainingBalance = pet.Affection
		} else {
			var wallet models.PlayerWallet
			lookup := tx.Limit(1).Find(&wallet, "account_id = ? AND currency_key = ?", accountID, currencyKey)
			if lookup.Error != nil {
				return lookup.Error
			}
			result.RemainingBalance = wallet.Balance
		}
		return nil
	})
	return result, err
}

func (service *ShopService) Sell(ctx context.Context, accountID, itemName string, quantity int64) (*SellResult, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	if quantity <= 0 || quantity > 99999999 {
		return nil, ErrInvalidQuantity
	}
	result := &SellResult{CurrencyKey: DefaultCurrencyKey}
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		*result = SellResult{CurrencyKey: DefaultCurrencyKey}
		if err := requirePet(tx, accountID); err != nil {
			return err
		}
		item, err := getItem(tx, itemName)
		if err != nil {
			return err
		}
		if item.SellPrice <= 0 {
			return ErrNotSellable
		}
		if quantity > math.MaxInt64/item.SellPrice {
			return ErrInvalidQuantity
		}
		if err = service.inventory().DebitTx(tx, accountID, item.Name, quantity); err != nil {
			return err
		}
		revenue := item.SellPrice * quantity
		if err = service.wallet().CreditTxWithReason(tx, accountID, result.CurrencyKey, revenue, "item_sale", item.Name); err != nil {
			return err
		}
		// A finite normal-shop listing can be replenished by player sales. The
		// affection shop is deliberately excluded because it uses another economy.
		if err = tx.Model(&models.ShopItemConfig{}).
			Where("name = ? AND shop_type = ? AND stock >= 0", item.Name, ShopTypeNormal).
			Update("stock", gorm.Expr("stock + ?", quantity)).Error; err != nil {
			return err
		}
		var wallet models.PlayerWallet
		if err = tx.First(&wallet, "account_id = ? AND currency_key = ?", accountID, result.CurrencyKey).Error; err != nil {
			return err
		}
		result.Item = *item
		result.Quantity = quantity
		result.Revenue = revenue
		result.RemainingBalance = wallet.Balance
		return nil
	})
	return result, err
}

func (service *ShopService) getListing(db *gorm.DB, name, shopType string) (*ShopListing, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrShopItemNotFound
	}
	query := db.Where("name = ?", name)
	if shopType != "" {
		query = query.Where("shop_type = ?", normalizeShopType(shopType))
	} else {
		query = query.Order("CASE WHEN shop_type = 'shop_normal' THEN 0 ELSE 1 END")
	}
	var row models.ShopItemConfig
	if err := query.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrShopItemNotFound
		}
		return nil, err
	}
	item, err := getItem(db, row.Name)
	if err != nil {
		return nil, err
	}
	if !itemCanBeObtained(item.Status) {
		return nil, ErrItemUnavailable
	}
	listing := listingFromModel(row)
	return &listing, nil
}

func getItem(db *gorm.DB, name string) (*models.ItemConfig, error) {
	var item models.ItemConfig
	err := db.First(&item, "name = ?", strings.TrimSpace(name)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrItemNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func requirePet(db *gorm.DB, accountID string) error {
	_, err := ActivePetTx(db, accountID)
	return err
}

func (service *ShopService) RemainingLimits(ctx context.Context, accountID string, listing *ShopListing) (ShopLimitRemaining, error) {
	if service == nil || service.DB == nil {
		return ShopLimitRemaining{}, ErrDatabaseUnavailable
	}
	if listing == nil {
		return ShopLimitRemaining{}, ErrShopItemNotFound
	}
	return remainingLimitsTx(service.DB.WithContext(ctx), accountID, listing, service.now())
}

func (service *ShopService) enforcePurchaseLimitsTx(tx *gorm.DB, accountID string, listing *ShopListing, quantity int64) error {
	remaining, err := remainingLimitsTx(tx, accountID, listing, service.now())
	if err != nil {
		return err
	}
	if listing.DailyLimit > 0 && quantity > remaining.Daily {
		return ErrPurchaseLimit
	}
	if listing.WeeklyLimit > 0 && quantity > remaining.Weekly {
		return ErrPurchaseLimit
	}
	return nil
}

func remainingLimitsTx(tx *gorm.DB, accountID string, listing *ShopListing, now time.Time) (ShopLimitRemaining, error) {
	remaining := ShopLimitRemaining{Daily: -1, Weekly: -1}
	if listing == nil {
		return remaining, ErrShopItemNotFound
	}
	if listing.DailyLimit > 0 {
		used, err := purchasedSinceTx(tx, accountID, listing.ID, shopWindowStart(now, "daily"))
		if err != nil {
			return remaining, err
		}
		remaining.Daily = listing.DailyLimit - used
		if remaining.Daily < 0 {
			remaining.Daily = 0
		}
	}
	if listing.WeeklyLimit > 0 {
		used, err := purchasedSinceTx(tx, accountID, listing.ID, shopWindowStart(now, "weekly"))
		if err != nil {
			return remaining, err
		}
		remaining.Weekly = listing.WeeklyLimit - used
		if remaining.Weekly < 0 {
			remaining.Weekly = 0
		}
	}
	return remaining, nil
}

func purchasedSinceTx(tx *gorm.DB, accountID string, shopItemID uint, since time.Time) (int64, error) {
	var used int64
	err := tx.Model(&models.ShopPurchaseLog{}).
		Where("account_id = ? AND shop_item_id = ? AND created_at >= ?", accountID, shopItemID, since).
		Select("COALESCE(SUM(quantity), 0)").Scan(&used).Error
	return used, err
}

func shopWindowStart(now time.Time, kind string) time.Time {
	year, month, day := now.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	if kind == "weekly" {
		weekday := int(start.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = start.AddDate(0, 0, 1-weekday)
	}
	return start
}

func (service *ShopService) now() time.Time {
	if service != nil && service.Now != nil {
		return service.Now()
	}
	return time.Now()
}

func itemCanBeObtained(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "" || status == "active" || status == "limited"
}

func normalizeShopType(shopType string) string {
	if shopType == ShopTypeAffection {
		return ShopTypeAffection
	}
	return ShopTypeNormal
}

func listingFromModel(row models.ShopItemConfig) ShopListing {
	return ShopListing{
		ID: row.ID, ShopType: row.ShopType, Name: row.Name, Image: row.Image,
		Stock: row.Stock, Price: row.Price, Description: row.Description,
		DailyLimit: row.DailyLimit, WeeklyLimit: row.WeeklyLimit,
	}
}

func (service *ShopService) inventory() *InventoryService {
	if service.Inventory == nil {
		service.Inventory = NewInventoryService(service.DB)
	}
	service.Inventory.Now = service.Now
	return service.Inventory
}

func (service *ShopService) wallet() *WalletService {
	if service.Wallet == nil {
		service.Wallet = NewWalletService(service.DB)
	}
	service.Wallet.Now = service.Now
	return service.Wallet
}
