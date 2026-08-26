package expedition

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

func seedAdventureMaterialShop(t *testing.T, service *Service, limitType string, limit int64) {
	t.Helper()
	rows := []any{
		&models.CurrencyConfig{Key: JourneyBadgeCurrencyKey, Name: "旅途徽章", Builtin: true, Enabled: true},
		&models.ItemConfig{Key: "forest-sample", Name: "林地样本", Category: "material", Rarity: "common", Stackable: true, MaxStack: 999, Status: "active"},
		&models.AdventureShopItemConfig{Key: "forest-sample-shop", Name: "林地样本补给", ProductType: "item", ProductKey: "forest-sample", Quantity: 2, Price: 10, LimitType: limitType, LimitQuantity: limit, Enabled: true},
	}
	for _, row := range rows {
		if err := service.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestAdventureShopPurchaseIsAtomicIdempotentAndIsolated(t *testing.T) {
	service, db, _ := newTestService(t)
	seedAdventureMaterialShop(t, service, "daily", 5)
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := creditAdventureCurrencyTx(tx, "player", 100, "test", "seed", service.Now())
		return err
	}); err != nil {
		t.Fatal(err)
	}

	result, err := service.PurchaseAdventureShop(context.Background(), "player", "forest-sample-shop", 2, "message-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.RemainingBalance != 80 || result.Purchase.GrantedQuantity != 4 {
		t.Fatalf("unexpected purchase result: %#v", result)
	}
	if _, err = service.PurchaseAdventureShop(context.Background(), "player", "forest-sample-shop", 2, "message-1"); err != nil {
		t.Fatal(err)
	}

	var adventureItem models.GlobalInventoryItem
	if err = db.First(&adventureItem, "account_id = ? AND item_key = ?", "player", "forest-sample").Error; err != nil || adventureItem.Quantity != 4 {
		t.Fatalf("adventure item mismatch: row=%#v err=%v", adventureItem, err)
	}
	var purchases int64
	db.Model(&models.AdventureShopPurchase{}).Where("account_id = ?", "player").Count(&purchases)
	if purchases != 1 {
		t.Fatalf("economy idempotency failed: purchases=%d", purchases)
	}
}

func TestAdventureShopLimitAndGrantFailureRollbackEverything(t *testing.T) {
	service, db, _ := newTestService(t)
	seedAdventureMaterialShop(t, service, "daily", 1)
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := creditAdventureCurrencyTx(tx, "player", 100, "test", "seed", service.Now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PurchaseAdventureShop(context.Background(), "player", "forest-sample-shop", 1, "message-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PurchaseAdventureShop(context.Background(), "player", "forest-sample-shop", 1, "message-2"); !errors.Is(err, ErrAdventureShopLimit) {
		t.Fatalf("expected limit error, got %v", err)
	}
	invalid := models.AdventureShopItemConfig{Key: "broken", Name: "损坏商品", ProductType: "item", ProductKey: "missing", Quantity: 1, Price: 30, LimitType: "none", Enabled: true}
	if err := db.Create(&invalid).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.PurchaseAdventureShop(context.Background(), "player", "broken", 1, "message-3"); !errors.Is(err, ErrAdventureItemNotFound) {
		t.Fatalf("expected grant failure, got %v", err)
	}
	var wallet models.PlayerWallet
	if err := db.First(&wallet, "account_id = ?", "player").Error; err != nil || wallet.Balance != 90 {
		t.Fatalf("wallet was not rolled back: %#v err=%v", wallet, err)
	}
	var purchases, debitLedgers int64
	db.Model(&models.AdventureShopPurchase{}).Where("account_id = ?", "player").Count(&purchases)
	db.Model(&models.WalletLedger{}).Where("account_id = ? AND currency_key = ? AND delta < 0", "player", JourneyBadgeCurrencyKey).Count(&debitLedgers)
	if purchases != 1 || debitLedgers != 1 {
		t.Fatalf("failed transaction left rows: purchases=%d debit_ledgers=%d", purchases, debitLedgers)
	}
}

func TestAdventureLootUsesUnifiedInventoryAndWallet(t *testing.T) {
	service, db, _ := newTestService(t)
	seedAdventureMaterialShop(t, service, "none", 0)
	pool := models.AdventureLootPoolConfig{Key: "isolated", Name: "隔离奖励", Rolls: 0}
	entries := []models.AdventureLootEntryConfig{
		{PoolKey: pool.Key, RewardType: "item", RewardKey: "forest-sample", MinQuantity: 2, MaxQuantity: 2, Guaranteed: true},
		{PoolKey: pool.Key, RewardType: "currency", RewardKey: JourneyBadgeCurrencyKey, MinQuantity: 3, MaxQuantity: 3, Guaranteed: true},
	}
	if err := db.Create(&pool).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := service.grantLootPoolTx(tx, "player", pool.Key, "test", false)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var regularItems, regularWallets int64
	db.Model(&models.GlobalInventoryItem{}).Where("account_id = ?", "player").Count(&regularItems)
	db.Model(&models.PlayerWallet{}).Where("account_id = ?", "player").Count(&regularWallets)
	if regularItems != 1 || regularWallets != 1 {
		t.Fatalf("unified economy was not credited: items=%d wallets=%d", regularItems, regularWallets)
	}
}

func TestSeasonShopDebitsSeasonTokenAndRespectsSeasonLimit(t *testing.T) {
	service, db, now := newTestService(t)
	if err := db.Create(&models.LiveEventConfig{Key: "season-01", Name: "季", Region: "全域", StoryChoices: `["一","二","三"]`, StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), Active: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CurrencyConfig{Key: gameplay.SeasonTokenCurrencyKey, Name: "遗迹季印", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CurrencyConfig{Key: JourneyBadgeCurrencyKey, Name: "调查徽章", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Key: "season_memento", Name: "首季纪念叶", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AdventureShopItemConfig{Key: "season-pin", Name: "纪念叶", ProductType: "item", ProductKey: "season_memento", Quantity: 1, Price: 10, CurrencyKey: gameplay.SeasonTokenCurrencyKey, LimitType: "season", LimitQuantity: 1, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := gameplay.NewWalletService(db).Credit(context.Background(), "player", gameplay.SeasonTokenCurrencyKey, 40); err != nil {
		t.Fatal(err)
	}
	if err := gameplay.NewWalletService(db).Credit(context.Background(), "player", JourneyBadgeCurrencyKey, 80); err != nil {
		t.Fatal(err)
	}
	result, err := service.PurchaseAdventureShop(context.Background(), "player", "season-pin", 1, "season-msg-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.RemainingBalance != 30 {
		t.Fatalf("season token was not debited: %#v", result)
	}
	if _, err = service.PurchaseAdventureShop(context.Background(), "player", "season-pin", 1, "season-msg-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.PurchaseAdventureShop(context.Background(), "player", "season-pin", 1, "season-msg-2"); !errors.Is(err, ErrAdventureShopLimit) {
		t.Fatalf("season limit should apply, got %v", err)
	}
	var season, badge models.PlayerWallet
	db.First(&season, "account_id = ? AND currency_key = ?", "player", gameplay.SeasonTokenCurrencyKey)
	db.First(&badge, "account_id = ? AND currency_key = ?", "player", JourneyBadgeCurrencyKey)
	if season.Balance != 30 || badge.Balance != 80 {
		t.Fatalf("wrong wallets after season shop: season=%d badge=%d", season.Balance, badge.Balance)
	}
}
