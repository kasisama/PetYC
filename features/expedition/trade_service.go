package expedition

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

var (
	ErrTradeNotFound       = errors.New("交易单不存在")
	ErrTradeNotOpen        = errors.New("交易单已经结束")
	ErrTradeOwnOffer       = errors.New("不能接受自己发布的交易")
	ErrTradeExpired        = errors.New("交易单已经过期，托管物品已退回")
	ErrTradeSellerRequired = errors.New("只有发布者可以取消这笔交易")
)

func (service *Service) CreateTradeOffer(ctx context.Context, sellerAccountID, itemName string, quantity, price int64) (*models.TradeOffer, error) {
	itemName = strings.TrimSpace(itemName)
	if itemName == "" || quantity <= 0 || quantity > 999 || price <= 0 || price > 1_000_000 {
		return nil, errors.New("交易数量或价格无效")
	}
	token, err := service.TokenSource()
	if err != nil {
		return nil, err
	}
	code := normalizeTradeCode(token)
	if code == "" {
		code = strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:8])
	}
	now := service.Now()
	offer := models.TradeOffer{
		Code: code, SellerAccountID: sellerAccountID, ItemName: itemName, Quantity: quantity,
		Price: price, CurrencyKey: gameplay.DefaultCurrencyKey, Status: "open",
		CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour),
	}
	err = gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		var catalog models.ItemConfig
		catalogLookup := tx.Limit(1).Find(&catalog, "name = ? AND status IN ?", itemName, []string{"active", "limited"})
		if catalogLookup.Error != nil {
			return catalogLookup.Error
		}
		if catalogLookup.RowsAffected == 0 {
			return errors.New("该物品当前不能交易")
		}
		if err := gameplay.NewInventoryService(tx).DebitTx(tx, sellerAccountID, itemName, quantity); err != nil {
			return err
		}
		if err := tx.Create(&offer).Error; err != nil {
			return err
		}
		return createTradeAuditTx(tx, offer.Code, sellerAccountID, "created", fmt.Sprintf("托管 %s ×%d，售价 %d", itemName, quantity, price), now)
	})
	return &offer, err
}

func (service *Service) GetTradeOffer(ctx context.Context, code string) (*models.TradeOffer, error) {
	code = normalizeTradeCode(code)
	var offer models.TradeOffer
	if err := service.DB.WithContext(ctx).First(&offer, "code = ?", code).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTradeNotFound
		}
		return nil, err
	}
	if offer.Status == "open" && !service.Now().Before(offer.ExpiresAt) {
		if err := service.expireTradeOffer(ctx, &offer); err != nil {
			return nil, err
		}
	}
	return &offer, nil
}

func (service *Service) ListTradeOffers(ctx context.Context, limit int) ([]models.TradeOffer, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	var offers []models.TradeOffer
	err := service.DB.WithContext(ctx).Where("status = ? AND expires_at > ?", "open", service.Now()).Order("created_at asc").Limit(limit).Find(&offers).Error
	return offers, err
}

func (service *Service) AcceptTradeOffer(ctx context.Context, buyerAccountID, code string) (*models.TradeOffer, error) {
	offer, err := service.GetTradeOffer(ctx, code)
	if err != nil {
		return nil, err
	}
	if offer.Status == "expired" {
		return nil, ErrTradeExpired
	}
	if offer.Status != "open" {
		return nil, ErrTradeNotOpen
	}
	if offer.SellerAccountID == buyerAccountID {
		return nil, ErrTradeOwnOffer
	}
	now := service.Now()
	err = gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		processing := tx.Model(&models.TradeOffer{}).
			Where("code = ? AND status = ? AND expires_at > ? AND seller_account_id <> ?", offer.Code, "open", now, buyerAccountID).
			Updates(map[string]interface{}{"status": "processing", "buyer_account_id": buyerAccountID})
		if processing.Error != nil {
			return processing.Error
		}
		if processing.RowsAffected != 1 {
			return ErrTradeNotOpen
		}
		wallet := gameplay.NewWalletService(tx)
		if err := wallet.DebitTxWithReason(tx, buyerAccountID, offer.CurrencyKey, offer.Price, "trade_purchase", offer.Code); err != nil {
			return err
		}
		if err := wallet.CreditTxWithReason(tx, offer.SellerAccountID, offer.CurrencyKey, offer.Price, "trade_sale", offer.Code); err != nil {
			return err
		}
		if err := gameplay.NewInventoryService(tx).CreditTx(tx, buyerAccountID, offer.ItemName, offer.Quantity); err != nil {
			return err
		}
		completed := tx.Model(&models.TradeOffer{}).Where("code = ? AND status = ?", offer.Code, "processing").Updates(map[string]interface{}{"status": "completed", "completed_at": now})
		if completed.Error != nil {
			return completed.Error
		}
		if completed.RowsAffected != 1 {
			return ErrTradeNotOpen
		}
		if err := createTradeAuditTx(tx, offer.Code, buyerAccountID, "accepted", fmt.Sprintf("支付 %d，获得 %s ×%d", offer.Price, offer.ItemName, offer.Quantity), now); err != nil {
			return err
		}
		return createTradeAuditTx(tx, offer.Code, offer.SellerAccountID, "completed", fmt.Sprintf("收到 %d", offer.Price), now)
	})
	if err != nil {
		return nil, err
	}
	return service.GetTradeOffer(ctx, offer.Code)
}

func (service *Service) CancelTradeOffer(ctx context.Context, sellerAccountID, code string) (*models.TradeOffer, error) {
	code = normalizeTradeCode(code)
	var offer models.TradeOffer
	now := service.Now()
	err := gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		if err := tx.First(&offer, "code = ?", code).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTradeNotFound
			}
			return err
		}
		if offer.SellerAccountID != sellerAccountID {
			return ErrTradeSellerRequired
		}
		updated := tx.Model(&models.TradeOffer{}).Where("code = ? AND status = ?", code, "open").Updates(map[string]interface{}{"status": "cancelled", "completed_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrTradeNotOpen
		}
		if err := gameplay.NewInventoryService(tx).CreditTx(tx, sellerAccountID, offer.ItemName, offer.Quantity); err != nil {
			return err
		}
		return createTradeAuditTx(tx, code, sellerAccountID, "cancelled", "托管物品已退回", now)
	})
	if err != nil {
		return nil, err
	}
	return service.GetTradeOffer(ctx, code)
}

func (service *Service) expireTradeOffer(ctx context.Context, offer *models.TradeOffer) error {
	now := service.Now()
	return gameplay.WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		updated := tx.Model(&models.TradeOffer{}).Where("code = ? AND status = ? AND expires_at <= ?", offer.Code, "open", now).Updates(map[string]interface{}{"status": "expired", "completed_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			return tx.First(offer, "code = ?", offer.Code).Error
		}
		if err := gameplay.NewInventoryService(tx).CreditTx(tx, offer.SellerAccountID, offer.ItemName, offer.Quantity); err != nil {
			return err
		}
		if err := createTradeAuditTx(tx, offer.Code, offer.SellerAccountID, "expired", "托管物品已退回", now); err != nil {
			return err
		}
		offer.Status = "expired"
		offer.CompletedAt = &now
		return nil
	})
}

func createTradeAuditTx(tx *gorm.DB, offerCode, accountID, action, detail string, now time.Time) error {
	return tx.Create(&models.TradeAudit{ID: uuid.NewString(), OfferCode: offerCode, AccountID: accountID, Action: action, Detail: detail, CreatedAt: now}).Error
}

func normalizeTradeCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "-", "")
	if len(code) > 8 {
		code = code[:8]
	}
	return code
}
