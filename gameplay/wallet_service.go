package gameplay

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/models"
)

const (
	PrimaryCurrencyKey      = "primary_coin"
	JourneyBadgeCurrencyKey = "journey_badge"
	SeasonTokenCurrencyKey  = "season_token"
	DefaultCurrencyKey      = PrimaryCurrencyKey
)

type WalletService struct {
	DB  *gorm.DB
	Now func() time.Time
}

func NewWalletService(db *gorm.DB) *WalletService {
	return &WalletService{DB: db, Now: time.Now}
}

func (service *WalletService) Credit(ctx context.Context, accountID, currencyKey string, amount int64) error {
	if service == nil || service.DB == nil {
		return ErrDatabaseUnavailable
	}
	return service.CreditTx(service.DB.WithContext(ctx), accountID, currencyKey, amount)
}

func (service *WalletService) CreditTx(tx *gorm.DB, accountID, currencyKey string, amount int64) error {
	return service.CreditTxWithReason(tx, accountID, currencyKey, amount, "manual_credit", "")
}

func (service *WalletService) CreditTxWithReason(tx *gorm.DB, accountID, currencyKey string, amount int64, reason, referenceKey string) error {
	currencyKey = normalizeCurrencyKey(currencyKey)
	if amount <= 0 || accountID == "" {
		return ErrInvalidQuantity
	}
	now := service.now()
	wallet := models.PlayerWallet{AccountID: accountID, CurrencyKey: currencyKey, Balance: amount, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "account_id"}, {Name: "currency_key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"balance":    gorm.Expr("balance + ?", amount),
			"updated_at": now,
		}),
	}).Create(&wallet).Error; err != nil {
		return err
	}
	return service.recordLedger(tx, accountID, currencyKey, amount, reason, referenceKey)
}

func (service *WalletService) Debit(ctx context.Context, accountID, currencyKey string, amount int64) error {
	if service == nil || service.DB == nil {
		return ErrDatabaseUnavailable
	}
	return service.DebitTx(service.DB.WithContext(ctx), accountID, currencyKey, amount)
}

func (service *WalletService) DebitTx(tx *gorm.DB, accountID, currencyKey string, amount int64) error {
	return service.DebitTxWithReason(tx, accountID, currencyKey, amount, "manual_debit", "")
}

func (service *WalletService) DebitTxWithReason(tx *gorm.DB, accountID, currencyKey string, amount int64, reason, referenceKey string) error {
	currencyKey = normalizeCurrencyKey(currencyKey)
	if amount <= 0 || accountID == "" {
		return ErrInvalidQuantity
	}
	result := tx.Model(&models.PlayerWallet{}).
		Where("account_id = ? AND currency_key = ? AND balance >= ?", accountID, currencyKey, amount).
		Updates(map[string]interface{}{"balance": gorm.Expr("balance - ?", amount), "updated_at": service.now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInsufficientFunds
	}
	return service.recordLedger(tx, accountID, currencyKey, -amount, reason, referenceKey)
}

func (service *WalletService) Balance(ctx context.Context, accountID, currencyKey string) (int64, error) {
	var wallet models.PlayerWallet
	err := service.DB.WithContext(ctx).Where("account_id = ? AND currency_key = ?", accountID, normalizeCurrencyKey(currencyKey)).First(&wallet).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return wallet.Balance, err
}

func normalizeCurrencyKey(currencyKey string) string {
	currencyKey = strings.TrimSpace(currencyKey)
	if currencyKey == "" {
		return DefaultCurrencyKey
	}
	return currencyKey
}

func (service *WalletService) recordLedger(tx *gorm.DB, accountID, currencyKey string, delta int64, reason, referenceKey string) error {
	var wallet models.PlayerWallet
	if err := tx.First(&wallet, "account_id = ? AND currency_key = ?", accountID, currencyKey).Error; err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "adjustment"
	}
	return tx.Create(&models.WalletLedger{
		ID: uuid.NewString(), AccountID: accountID, CurrencyKey: currencyKey, Delta: delta,
		BalanceAfter: wallet.Balance, Reason: reason, ReferenceKey: strings.TrimSpace(referenceKey), CreatedAt: service.now(),
	}).Error
}

func (service *WalletService) now() time.Time {
	if service != nil && service.Now != nil {
		return service.Now()
	}
	return time.Now()
}
