package gameplay

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/core"
	"qq-pet-saas/models"
)

func newGameplayDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(
		&models.PlayerAccount{}, &models.PlayerIdentity{}, &models.PetProfile{},
		&models.PetSpeciesConfig{}, &models.PetEvolutionRuleConfig{}, &models.PetEvolutionCostConfig{}, &models.PetSkillUnlockConfig{}, &models.GlobalInventoryItem{}, &models.PlayerWallet{}, &models.WalletLedger{},
		&models.ItemConfig{}, &models.ShopItemConfig{}, &models.ShopPurchaseLog{},
		&models.CompanionJournal{}, &models.CompanionActionDaily{}, &models.CheckinRewardConfig{}, &models.PetBehaviorProfile{},
		&models.ActivityRun{}, &models.ItemUseRecord{}, &models.ExpeditionRun{},
		&models.PersonalityRuleConfig{}, &models.GrowthRoleConfig{}, &models.GrowthStanceConfig{},
		&models.CodexCatalogConfig{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCompanionFeedAndGiftConsumeUnifiedInventoryAndApplyFavorites(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetSpeciesConfig{
		Name: "光芽兽", FavoriteFood: "小饼干", FavoriteGift: "小铃铛",
		Hunger: 100, HungerMax: 100, AffectionBonus: 10,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", MoodPoints: 50,
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 50, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create([]models.ItemConfig{
		{Name: "小饼干", Status: "active", Type: "饱食", Effect: "15", Image: "物品/小饼干.png"},
		{Name: "小铃铛", Status: "active", Type: "好感", Effect: "50", Image: "物品/小铃铛.png"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	inventory := NewInventoryService(db)
	if err := inventory.Credit(context.Background(), "account-1", "小饼干", 2); err != nil {
		t.Fatal(err)
	}
	if err := inventory.Credit(context.Background(), "account-1", "小铃铛", 2); err != nil {
		t.Fatal(err)
	}

	service := NewCompanionService(db)
	feed, err := service.Interact(context.Background(), "account-1", ActionFeed, "小饼干", 1, CompanionRules{})
	if err != nil {
		t.Fatal(err)
	}
	if !feed.FavoriteBonus || feed.HungerBefore != 50 || feed.HungerAfter != 80 || feed.Image != "物品/小饼干.png" {
		t.Fatalf("favorite food result mismatch: %#v", feed)
	}
	gift, err := service.Interact(context.Background(), "account-1", ActionGift, "小铃铛", 1, CompanionRules{})
	if err != nil {
		t.Fatal(err)
	}
	if !gift.FavoriteBonus || gift.AffectionDelta != 110 || gift.Image != "物品/小铃铛.png" {
		t.Fatalf("favorite gift result mismatch: %#v", gift)
	}
	items, err := inventory.List(context.Background(), "account-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	quantities := make(map[string]int64, len(items))
	for _, item := range items {
		quantities[item.ItemName] = item.Quantity
	}
	if quantities["小饼干"] != 1 || quantities["小铃铛"] != 1 {
		t.Fatalf("companion interaction did not debit unified inventory: %#v", quantities)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", "account-1").Error; err != nil {
		t.Fatal(err)
	}
	if pet.Hunger != 80 || pet.Affection != 110 || pet.BondLevel != 2 {
		t.Fatalf("companion interaction did not persist pet state: %#v", pet)
	}
}

func TestCompanionCooldownAndDailyLimitSurviveServiceRestart(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", MoodPoints: 50,
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 21, 9, 0, 0, 0, time.Local)
	first := NewCompanionService(db)
	first.Now = func() time.Time { return now }
	if _, err := first.Interact(context.Background(), "account-1", ActionTouch, "", 1, CompanionRules{}); err != nil {
		t.Fatal(err)
	}

	restarted := NewCompanionService(db)
	restarted.Now = func() time.Time { return now.Add(time.Minute) }
	if _, err := restarted.Interact(context.Background(), "account-1", ActionTouch, "", 1, CompanionRules{}); !errors.Is(err, ErrActionCooldown) {
		t.Fatalf("persisted cooldown should reject interaction after restart, got %v", err)
	}
	restarted.Now = func() time.Time { return now.Add(11 * time.Minute) }
	if _, err := restarted.Interact(context.Background(), "account-1", ActionTouch, "", 1, CompanionRules{}); err != nil {
		t.Fatalf("interaction should succeed after cooldown: %v", err)
	}
	if _, err := restarted.Interact(context.Background(), "account-1", ActionWash, "", 1, CompanionRules{}); err != nil {
		t.Fatal(err)
	}
	secondRestart := NewCompanionService(db)
	secondRestart.Now = func() time.Time { return now.Add(12 * time.Minute) }
	if _, err := secondRestart.Interact(context.Background(), "account-1", ActionWash, "", 1, CompanionRules{}); !errors.Is(err, ErrDailyLimit) {
		t.Fatalf("persisted daily wash limit should reject after restart, got %v", err)
	}
	var touchDaily models.CompanionActionDaily
	if err := db.First(&touchDaily, "account_id = ? AND action = ?", "account-1", ActionTouch).Error; err != nil {
		t.Fatal(err)
	}
	if touchDaily.Count != 2 {
		t.Fatalf("touch count should persist successful interactions only: %#v", touchDaily)
	}
}

func TestCareServiceKeepsRestRecoverAndTreatmentOnOnePetAndWallet(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "星星", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", MoodPoints: 50,
		Readiness: 100, Health: 20, HealthMax: 100, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	wallet := NewWalletService(db)
	if err := wallet.Credit(context.Background(), "account-1", DefaultCurrencyKey, 100); err != nil {
		t.Fatal(err)
	}
	care := NewCareService(db)
	if _, err := care.PutToRest(context.Background(), "account-1", "错误名字"); !errors.Is(err, ErrPetNameMismatch) {
		t.Fatalf("name confirmation should protect resting action, got %v", err)
	}
	pet, err := care.PutToRest(context.Background(), "account-1", "星星")
	if err != nil || pet.Status != PetStatusResting {
		t.Fatalf("put to rest failed: pet=%#v err=%v", pet, err)
	}
	pet, err = care.Resume(context.Background(), "account-1")
	if err != nil || pet.Status != "空闲" {
		t.Fatalf("resume failed: pet=%#v err=%v", pet, err)
	}
	if err = db.Model(&models.PetProfile{}).Where("account_id = ?", "account-1").Updates(map[string]any{"health": 20, "status": "濒死"}).Error; err != nil {
		t.Fatal(err)
	}
	treatment, err := care.Treat(context.Background(), "account-1", DefaultCurrencyKey, 25)
	if err != nil {
		t.Fatal(err)
	}
	if treatment.HealthBefore != 20 || treatment.HealthAfter != 100 || treatment.RemainingBalance != 75 {
		t.Fatalf("treatment result mismatch: %#v", treatment)
	}
	if err = db.First(&pet, "account_id = ?", "account-1").Error; err != nil {
		t.Fatal(err)
	}
	if pet.Health != 100 || pet.Status != "空闲" {
		t.Fatalf("treatment did not restore pet state: %#v", pet)
	}
}

func TestCareTreatmentClearsInjuredStatus(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "风耳狐", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "受伤", Mood: "一般", MoodPoints: 50,
		Readiness: 100, Health: 88, HealthMax: 156, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	care := NewCareService(db)
	if _, err := care.Treat(context.Background(), "account-1", DefaultCurrencyKey, 0); err != nil {
		t.Fatal(err)
	}
	var pet models.PetProfile
	if err := db.First(&pet, "account_id = ?", "account-1").Error; err != nil {
		t.Fatal(err)
	}
	if pet.Health != 156 || pet.Status != "空闲" {
		t.Fatalf("treatment must restore injured pet to idle: %#v", pet)
	}
}

func TestCareTreatmentRollsBackWhenWalletIsInsufficient(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "濒死", Mood: "一般",
		Readiness: 100, Health: 10, HealthMax: 100, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewCareService(db).Treat(context.Background(), "account-1", DefaultCurrencyKey, 10); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected insufficient wallet, got %v", err)
	}
	var pet models.PetProfile
	if err := db.First(&pet, "account_id = ?", "account-1").Error; err != nil {
		t.Fatal(err)
	}
	if pet.Health != 10 || pet.Status != "濒死" {
		t.Fatalf("failed treatment changed pet state: %#v", pet)
	}
}

func TestActivityRunConsumesItemAndClaimsAttributeRewardExactlyOnce(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetSpeciesConfig{Name: "光芽兽", WisdomMax: 20, AttributeBonus: 10}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", MoodPoints: 50,
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100, Wisdom: 10,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Name: "专业书本", Status: "active", Type: "智慧", Effect: "5", Time: 1, Image: "物品/专业书本.png"}).Error; err != nil {
		t.Fatal(err)
	}
	inventory := NewInventoryService(db)
	if err := inventory.Credit(context.Background(), "account-1", "专业书本", 1); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.Local)
	activity := NewActivityService(db)
	activity.Now = func() time.Time { return now }
	started, err := activity.Start(context.Background(), "account-1", ActivityStartRequest{
		Kind: ActivityStudy, ItemName: "专业书本", RequiredItemType: "智慧", RewardAttribute: "智慧",
		DailyLimit: 3, HungerCost: 10, RewardGrowth: 2, StartImage: "活动/学习开始.png", EndImage: "活动/学习完成.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.HungerBefore != 100 || started.HungerAfter != 90 || started.Run.EndsAt != now.Add(time.Minute) || started.Image != "活动/学习开始.png" {
		t.Fatalf("activity start mismatch: %#v", started)
	}
	items, err := inventory.List(context.Background(), "account-1", 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("activity did not consume the unified inventory item: %#v err=%v", items, err)
	}
	if _, err = activity.Complete(context.Background(), "account-1", ActivityStudy, DefaultCurrencyKey); !errors.Is(err, ErrActivityNotReady) {
		t.Fatalf("early completion should be rejected, got %v", err)
	}
	now = now.Add(time.Minute)
	completed, err := activity.Complete(context.Background(), "account-1", ActivityStudy, DefaultCurrencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if completed.AttributeBefore != 10 || completed.AttributeAfter != 16 || completed.GrowthDelta != 2 || completed.Image != "活动/学习完成.png" {
		t.Fatalf("activity completion mismatch: %#v", completed)
	}
	if _, err = activity.Complete(context.Background(), "account-1", ActivityStudy, DefaultCurrencyKey); !errors.Is(err, ErrNoActiveActivity) {
		t.Fatalf("claimed activity must not pay twice, got %v", err)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", "account-1").Error; err != nil {
		t.Fatal(err)
	}
	if pet.Wisdom != 16 || pet.Growth != 2 || pet.Status != "空闲" || pet.Hunger != 90 {
		t.Fatalf("activity result did not persist on PetProfile: %#v", pet)
	}
}

func TestActivityWorkPaysUnifiedWalletAndInventoryFromSnapshot(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetSpeciesConfig{Name: "光芽兽", CurrencyBonus: 20}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "开心", MoodPoints: 70,
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.Local)
	activity := NewActivityService(db)
	activity.Now = func() time.Time { return now }
	if _, err := activity.Start(context.Background(), "account-1", ActivityStartRequest{
		Kind: ActivityWork, ConfigKey: "整理书架", Duration: time.Minute, DailyLimit: 5,
		HungerCost: 15, RewardCurrency: 20, RewardItems: "木材*2#纪念票*1",
	}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	completed, err := activity.Complete(context.Background(), "account-1", ActivityWork, DefaultCurrencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if completed.CurrencyDelta != 30 || completed.RemainingBalance != 30 || len(completed.Items) != 2 {
		t.Fatalf("work reward mismatch: %#v", completed)
	}
	items, err := NewInventoryService(db).List(context.Background(), "account-1", 10)
	quantities := make(map[string]int64, len(items))
	for _, item := range items {
		quantities[item.ItemName] = item.Quantity
	}
	if err != nil || quantities["木材"] != 2 || quantities["纪念票"] != 1 {
		t.Fatalf("work items did not enter unified inventory: %#v err=%v", quantities, err)
	}
}

func TestConcurrentActivityCompletionPaysOnce(t *testing.T) {
	dsn := fmt.Sprintf("file:activity-%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", time.Now().UnixNano())
	db := newGameplayDB(t, dsn)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	if err = db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: ActivityWork, Mood: "一般", MoodPoints: 50,
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 90, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 21, 11, 0, 0, 0, time.Local)
	if err = db.Create(&models.ActivityRun{
		ID: "activity-1", AccountID: "account-1", Kind: ActivityWork, ConfigKey: "整理书架",
		Status: ActivityStatusRunning, RewardCurrency: 10, StartsAt: now.Add(-2 * time.Minute), EndsAt: now.Add(-time.Minute), CreatedAt: now.Add(-2 * time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}
	activity := NewActivityService(db)
	activity.Now = func() time.Time { return now }
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, completeErr := activity.Complete(context.Background(), "account-1", ActivityWork, DefaultCurrencyKey)
			results <- completeErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	successes, rejected := 0, 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, ErrNoActiveActivity):
			rejected++
		default:
			t.Fatalf("unexpected concurrent completion result: %v", result)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("activity should pay once: successes=%d rejected=%d", successes, rejected)
	}
	balance, err := NewWalletService(db).Balance(context.Background(), "account-1", DefaultCurrencyKey)
	if err != nil || balance != 10 {
		t.Fatalf("activity reward was duplicated: balance=%d err=%v", balance, err)
	}
}

func TestEvolutionPreviewAndAwakeningUseConfiguredConditionsAndUnifiedInventory(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&[]models.PetSpeciesConfig{{Key: "lumisprout_base", Name: "光芽兽", FamilyKey: "lumisprout", Stage: "base", Image: "宠物/光芽兽.png"}, {Key: "lumisprout_evolved", Name: "曜叶兽", FamilyKey: "lumisprout", Stage: "evolved", PreviousFormKey: "lumisprout_base", Image: "宠物/曜叶兽.png"}, {Key: "lumisprout_awaken_a", Name: "曦冠灵", FamilyKey: "lumisprout", Stage: "awakened", PreviousFormKey: "lumisprout_evolved", Image: "宠物/曦冠灵.png"}}).Error; err != nil {
		t.Fatal(err)
	}
	db.Create(&[]models.PetEvolutionRuleConfig{{Key: "nono_standard", FromFormKey: "lumisprout_base", ToFormKey: "lumisprout_evolved", RequiredGrowth: 10, RequiredAffection: 5, BranchLabel: "标准进化", Enabled: true}, {Key: "lumisprout_awaken_a_rule", FromFormKey: "lumisprout_evolved", ToFormKey: "lumisprout_awaken_a", RequiredGrowth: 20, RequiredAffection: 10, BranchLabel: "星耀路线", Enabled: true}})
	db.Create(&models.ItemConfig{Key: "light_stone", Name: "光之石", Status: "active"})
	db.Create(&models.PetEvolutionCostConfig{EvolutionKey: "lumisprout_awaken_a_rule", ItemKey: "light_stone", Quantity: 2})
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "lumisprout", Name: "光芽兽", CurrentForm: "lumisprout_base",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般",
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100, Growth: 10, Affection: 5,
	}).Error; err != nil {
		t.Fatal(err)
	}
	evolution := NewEvolutionService(db)
	preview, err := evolution.Preview(context.Background(), "account-1", "进化")
	if err != nil || !preview.Ready || preview.TargetForm != "曜叶兽" || preview.TargetImage != "宠物/曜叶兽.png" {
		t.Fatalf("evolution preview mismatch: %#v err=%v", preview, err)
	}
	completed, err := evolution.Evolve(context.Background(), "account-1")
	if err != nil || completed.TargetForm != "曜叶兽" {
		t.Fatalf("evolution failed: %#v err=%v", completed, err)
	}
	preview, err = evolution.Preview(context.Background(), "account-1", "觉醒")
	if err != nil || preview.Ready || len(preview.Requirements) != 3 {
		t.Fatalf("awakening preview should expose missing growth, affection and item: %#v err=%v", preview, err)
	}
	if err = db.Model(&models.PetProfile{}).Where("account_id = ?", "account-1").Updates(map[string]any{"growth": 20, "affection": 10}).Error; err != nil {
		t.Fatal(err)
	}
	if err = NewInventoryService(db).Credit(context.Background(), "account-1", "光之石", 2); err != nil {
		t.Fatal(err)
	}
	completed, err = evolution.Awaken(context.Background(), "account-1")
	if err != nil || completed.TargetForm != "曦冠灵" || completed.TargetImage != "宠物/曦冠灵.png" {
		t.Fatalf("awakening failed: %#v err=%v", completed, err)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", "account-1").Error; err != nil {
		t.Fatal(err)
	}
	var species models.PetSpeciesConfig
	if err = db.First(&species, "name = ?", "光芽兽").Error; err != nil {
		t.Fatal(err)
	}
	species = models.PetSpeciesConfig{}
	if err = db.First(&species, "key = ?", pet.CurrentForm).Error; err != nil {
		t.Fatal(err)
	}
	if pet.CurrentForm != "lumisprout_awaken_a" || ResolvePetImage(pet, species) != "宠物/曦冠灵.png" {
		t.Fatalf("awakened form or image was not persisted: pet=%#v", pet)
	}
	items, err := NewInventoryService(db).List(context.Background(), "account-1", 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("awakening did not atomically consume required items: %#v err=%v", items, err)
	}
}

func TestAwakeningFailureDoesNotChangeFormOrPartiallyConsumeItems(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	db.Create(&[]models.PetSpeciesConfig{{Key: "lumisprout_evolved", Name: "曜叶兽", FamilyKey: "lumisprout", Stage: "evolved"}, {Key: "lumisprout_awaken_a", Name: "曦冠灵", FamilyKey: "lumisprout", Stage: "awakened", PreviousFormKey: "lumisprout_evolved"}})
	db.Create(&models.PetEvolutionRuleConfig{Key: "lumisprout_awaken_a_rule", FromFormKey: "lumisprout_evolved", ToFormKey: "lumisprout_awaken_a", RequiredGrowth: 20, RequiredAffection: 10, BranchLabel: "星耀路线", Enabled: true})
	db.Create(&[]models.ItemConfig{{Key: "light_stone", Name: "光之石", Status: "active"}, {Key: "stardust", Name: "星尘", Status: "active"}})
	db.Create(&[]models.PetEvolutionCostConfig{{EvolutionKey: "lumisprout_awaken_a_rule", ItemKey: "light_stone", Quantity: 2}, {EvolutionKey: "lumisprout_awaken_a_rule", ItemKey: "stardust", Quantity: 1}})
	db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "lumisprout", Name: "光芽兽", CurrentForm: "lumisprout_evolved",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", Growth: 20, Affection: 10,
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	})
	inventory := NewInventoryService(db)
	if err := inventory.Credit(context.Background(), "account-1", "光之石", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEvolutionService(db).Awaken(context.Background(), "account-1"); !errors.Is(err, ErrEvolutionRequirements) {
		t.Fatalf("missing awakening item should reject before debit, got %v", err)
	}
	var pet models.PetProfile
	db.First(&pet, "account_id = ?", "account-1")
	if pet.CurrentForm != "lumisprout_evolved" {
		t.Fatalf("failed awakening changed form: %#v", pet)
	}
	items, _ := inventory.List(context.Background(), "account-1", 10)
	if len(items) != 1 || items[0].ItemName != "光之石" || items[0].Quantity != 2 {
		t.Fatalf("failed awakening partially consumed inventory: %#v", items)
	}
}

func seedAwakenBranches(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&[]models.PetSpeciesConfig{
		{Key: "lumisprout_evolved", Name: "曜叶兽", FamilyKey: "lumisprout", Stage: "evolved"},
		{Key: "lumisprout_awaken_a", Name: "曦冠灵", FamilyKey: "lumisprout", Stage: "awakened", PreviousFormKey: "lumisprout_evolved"},
		{Key: "lumisprout_awaken_b", Name: "月冕灵", FamilyKey: "lumisprout", Stage: "awakened", PreviousFormKey: "lumisprout_evolved"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]models.PetEvolutionRuleConfig{
		{Key: "lumisprout_awaken_a_rule", FromFormKey: "lumisprout_evolved", ToFormKey: "lumisprout_awaken_a", RequiredGrowth: 20, RequiredAffection: 10, BranchLabel: "曦光路线", Enabled: true, SortOrder: 20},
		{Key: "lumisprout_awaken_b_rule", FromFormKey: "lumisprout_evolved", ToFormKey: "lumisprout_awaken_b", RequiredGrowth: 20, RequiredAffection: 10, BranchLabel: "月影路线", Enabled: true, SortOrder: 30},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "lumisprout", Name: "光芽兽", CurrentForm: "lumisprout_evolved",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", Growth: 20, Affection: 10,
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestAwakenWithoutBranchChoiceFailsWhenMultipleRoutesExist(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	seedAwakenBranches(t, db)
	evolution := NewEvolutionService(db)
	if _, err := evolution.Awaken(context.Background(), "account-1"); !errors.Is(err, ErrEvolutionBranchRequired) {
		t.Fatalf("multiple awaken routes must require an explicit choice, got %v", err)
	}
	if _, err := evolution.Preview(context.Background(), "account-1", "觉醒"); !errors.Is(err, ErrEvolutionBranchRequired) {
		t.Fatalf("preview without a branch must not lock the first route, got %v", err)
	}
	options, err := evolution.ListOptions(context.Background(), "account-1", "觉醒")
	if err != nil || len(options) != 2 || options[0].BranchLabel != "曦光路线" || options[1].BranchLabel != "月影路线" {
		t.Fatalf("expected both awaken branches, got %#v err=%v", options, err)
	}
	var pet models.PetProfile
	db.First(&pet, "account_id = ?", "account-1")
	if pet.CurrentForm != "lumisprout_evolved" {
		t.Fatalf("refusing a missing branch choice must not change form: %#v", pet)
	}
}

func TestAwakenToSelectedBranchPersistsChosenForm(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	seedAwakenBranches(t, db)
	evolution := NewEvolutionService(db)
	preview, err := evolution.PreviewTo(context.Background(), "account-1", "觉醒", "月影路线")
	if err != nil || preview.TargetForm != "月冕灵" || preview.BranchLabel != "月影路线" || !preview.Ready {
		t.Fatalf("selected branch preview mismatch: %#v err=%v", preview, err)
	}
	completed, err := evolution.EvolveTo(context.Background(), "account-1", "月影路线")
	if err != nil || completed.TargetForm != "月冕灵" {
		t.Fatalf("chosen awaken branch failed: %#v err=%v", completed, err)
	}
	var pet models.PetProfile
	db.First(&pet, "account_id = ?", "account-1")
	if pet.CurrentForm != "lumisprout_awaken_b" {
		t.Fatalf("awakened form was not the player-chosen branch: %#v", pet)
	}
}

func TestItemEffectUseIsAuditableAndIdempotent(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般",
		Readiness: 100, Health: 50, HealthMax: 100, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create([]models.ItemConfig{
		{Name: "急救包", Status: "active", Type: "血量", Effect: "30", Image: "物品/急救包.png"},
		{Name: "成长激素", Status: "active", Type: "成长", Effect: "5", Image: "物品/成长激素.png"},
		{Name: "小饼干", Status: "active", Type: "饱食", Effect: "10"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	inventory := NewInventoryService(db)
	for _, item := range []string{"急救包", "成长激素", "小饼干"} {
		if err := inventory.Credit(context.Background(), "account-1", item, 2); err != nil {
			t.Fatal(err)
		}
	}
	items := NewItemEffectService(db)
	first, err := items.Use(context.Background(), "account-1", "急救包", 1, "message-1")
	if err != nil || first.Duplicate || first.Record.BeforeValue != 50 || first.Record.AfterValue != 80 || first.Image != "物品/急救包.png" {
		t.Fatalf("first item use mismatch: %#v err=%v", first, err)
	}
	duplicate, err := items.Use(context.Background(), "account-1", "急救包", 1, "message-1")
	if err != nil || !duplicate.Duplicate || duplicate.Record.ID != first.Record.ID {
		t.Fatalf("duplicate item use should return the original record: %#v err=%v", duplicate, err)
	}
	if _, err = items.Use(context.Background(), "account-1", "成长激素", 2, "message-2"); err != nil {
		t.Fatal(err)
	}
	if _, err = items.Use(context.Background(), "account-1", "小饼干", 1, "message-3"); !errors.Is(err, ErrWrongItemType) {
		t.Fatalf("food should be routed through feed instead of direct use, got %v", err)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", "account-1").Error; err != nil {
		t.Fatal(err)
	}
	if pet.Health != 80 || pet.Growth != 10 {
		t.Fatalf("item effects were not applied exactly once: %#v", pet)
	}
	quantities := make(map[string]int64)
	inventoryRows, _ := inventory.List(context.Background(), "account-1", 10)
	for _, item := range inventoryRows {
		quantities[item.ItemName] = item.Quantity
	}
	if quantities["急救包"] != 1 || quantities["成长激素"] != 0 || quantities["小饼干"] != 2 {
		t.Fatalf("item use inventory mismatch: %#v", quantities)
	}
	var records int64
	if err = db.Model(&models.ItemUseRecord{}).Where("account_id = ?", "account-1").Count(&records).Error; err != nil || records != 2 {
		t.Fatalf("item use audit count mismatch: records=%d err=%v", records, err)
	}
}

func TestShopPurchaseAndSellUseOneWalletInventoryAndStockTransaction(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般",
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Name: "小饼干", Status: "active", Type: "饱食", Effect: "15", Image: "物品/小饼干.png", SellPrice: 4}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ShopItemConfig{ShopType: ShopTypeNormal, Name: "小饼干", Image: "物品/小饼干.png", Stock: 3, Price: 10, Description: "香香脆脆"}).Error; err != nil {
		t.Fatal(err)
	}
	wallet := NewWalletService(db)
	if err := wallet.Credit(context.Background(), "account-1", DefaultCurrencyKey, 100); err != nil {
		t.Fatal(err)
	}
	shop := NewShopService(db)
	purchase, err := shop.Purchase(context.Background(), "account-1", "小饼干", 2)
	if err != nil {
		t.Fatal(err)
	}
	if purchase.Cost != 20 || purchase.RemainingBalance != 80 || purchase.RemainingStock != 1 {
		t.Fatalf("purchase result mismatch: %#v", purchase)
	}
	items, err := shop.Inventory.List(context.Background(), "account-1", 20)
	if err != nil || len(items) != 1 || items[0].Quantity != 2 {
		t.Fatalf("purchased item did not enter the global inventory: %#v err=%v", items, err)
	}
	var listing models.ShopItemConfig
	db.First(&listing, "name = ? AND shop_type = ?", "小饼干", ShopTypeNormal)
	if listing.Stock != 1 {
		t.Fatalf("shop stock mismatch after purchase: %d", listing.Stock)
	}

	sale, err := shop.Sell(context.Background(), "account-1", "小饼干", 1)
	if err != nil {
		t.Fatal(err)
	}
	if sale.Revenue != 4 || sale.RemainingBalance != 84 {
		t.Fatalf("sale result mismatch: %#v", sale)
	}
	items, _ = shop.Inventory.List(context.Background(), "account-1", 20)
	if len(items) != 1 || items[0].Quantity != 1 {
		t.Fatalf("sale did not debit the global inventory: %#v", items)
	}
	db.First(&listing, "name = ? AND shop_type = ?", "小饼干", ShopTypeNormal)
	if listing.Stock != 2 {
		t.Fatalf("finite normal shop stock was not replenished: %d", listing.Stock)
	}
	var ledger []models.WalletLedger
	if err := db.Where("account_id = ?", "account-1").Order("created_at asc").Find(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	reasons := make(map[string]int64, len(ledger))
	for _, entry := range ledger {
		reasons[entry.Reason] += entry.Delta
	}
	if reasons["manual_credit"] != 100 || reasons["shop_purchase"] != -20 || reasons["item_sale"] != 4 {
		t.Fatalf("wallet sources and sinks are not auditable: %#v", ledger)
	}
}

func TestShopPurchaseRollsBackStockWhenWalletIsInsufficient(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般",
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	})
	db.Create(&models.ItemConfig{Name: "昂贵礼物", Status: "active", Type: "好感", Effect: "5"})
	db.Create(&models.ShopItemConfig{ShopType: ShopTypeNormal, Name: "昂贵礼物", Stock: 1, Price: 100})
	shop := NewShopService(db)
	if _, err := shop.Purchase(context.Background(), "account-1", "昂贵礼物", 1); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected insufficient wallet, got %v", err)
	}
	var listing models.ShopItemConfig
	db.First(&listing, "name = ?", "昂贵礼物")
	if listing.Stock != 1 {
		t.Fatalf("failed purchase consumed stock: %d", listing.Stock)
	}
	var inventory int64
	db.Model(&models.GlobalInventoryItem{}).Where("account_id = ?", "account-1").Count(&inventory)
	if inventory != 0 {
		t.Fatalf("failed purchase granted an item: %d", inventory)
	}
}

func TestAffectionShopDebitsPetAffection(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", Affection: 50,
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	})
	db.Create(&models.ItemConfig{Name: "纪念花束", Status: "active", Type: "纪念"})
	db.Create(&models.ShopItemConfig{ShopType: ShopTypeAffection, Name: "纪念花束", Stock: -1, Price: 20})
	result, err := NewShopService(db).Purchase(context.Background(), "account-1", "纪念花束", 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrencyKey != "好感" || result.RemainingBalance != 10 {
		t.Fatalf("affection purchase mismatch: %#v", result)
	}
	var pet models.PetProfile
	db.First(&pet, "account_id = ?", "account-1")
	if pet.Affection != 10 {
		t.Fatalf("affection balance mismatch: %d", pet.Affection)
	}
}

func TestAffectionShopDebitsActivePetOnly(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PlayerAccount{ID: "account-1", ActivePetID: "pet-active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]models.PetProfile{
		{ID: "pet-other", AccountID: "account-1", PetType: "烬爪兽", Name: "备用", CurrentForm: "烬爪兽", Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", Affection: 200, Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100},
		{ID: "pet-active", AccountID: "account-1", PetType: "光芽兽", Name: "当前", CurrentForm: "光芽兽", Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", Affection: 50, Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Name: "纪念花束", Status: "active", Type: "纪念"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ShopItemConfig{ShopType: ShopTypeAffection, Name: "纪念花束", Stock: -1, Price: 20}).Error; err != nil {
		t.Fatal(err)
	}
	result, err := NewShopService(db).Purchase(context.Background(), "account-1", "纪念花束", 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.RemainingBalance != 10 {
		t.Fatalf("active pet remaining affection mismatch: %#v", result)
	}
	var pets []models.PetProfile
	db.Where("account_id = ?", "account-1").Order("id").Find(&pets)
	balances := map[string]int64{}
	for _, pet := range pets {
		balances[pet.ID] = pet.Affection
	}
	if balances["pet-active"] != 10 || balances["pet-other"] != 200 {
		t.Fatalf("affection must debit the active pet only: %#v", balances)
	}
}

func TestShopPurchaseRespectsDailyAndWeeklyLimits(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般",
		Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ItemConfig{Name: "小饼干", Status: "active", Type: "饱食", Effect: "15"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ShopItemConfig{ShopType: ShopTypeNormal, Name: "小饼干", Stock: -1, Price: 1, DailyLimit: 2, WeeklyLimit: 3}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.Local) // Monday
	shop := NewShopService(db)
	shop.Now = func() time.Time { return now }
	wallet := NewWalletService(db)
	wallet.Now = shop.Now
	shop.Wallet = wallet
	if err := wallet.Credit(context.Background(), "account-1", DefaultCurrencyKey, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.Purchase(context.Background(), "account-1", "小饼干", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.Purchase(context.Background(), "account-1", "小饼干", 1); !errors.Is(err, ErrPurchaseLimit) {
		t.Fatalf("daily limit should reject the third unit, got %v", err)
	}
	now = now.Add(24 * time.Hour)
	if _, err := shop.Purchase(context.Background(), "account-1", "小饼干", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.Purchase(context.Background(), "account-1", "小饼干", 1); !errors.Is(err, ErrPurchaseLimit) {
		t.Fatalf("weekly limit should reject the fourth unit, got %v", err)
	}
	listing, err := shop.GetListing(context.Background(), "小饼干")
	if err != nil {
		t.Fatal(err)
	}
	remaining, err := shop.RemainingLimits(context.Background(), "account-1", listing)
	if err != nil || remaining.Daily != 1 || remaining.Weekly != 0 {
		t.Fatalf("remaining limits mismatch: %#v err=%v", remaining, err)
	}
}

func oneBotIdentity(groupID, userID string) core.InboundEvent {
	return core.InboundEvent{
		Platform: core.PlatformOneBot, AppID: "onebot", SceneType: core.SceneGroup,
		SpaceID: groupID, ActorID: userID,
	}
}

func TestAccountServiceSharesOneBotIdentityAcrossGroups(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	service := NewAccountService(db)
	first, err := service.Resolve(context.Background(), oneBotIdentity("group-a", "10001"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Resolve(context.Background(), oneBotIdentity("group-b", "10001"))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("OneBot identity contract should be global across groups: %s != %s", first.ID, second.ID)
	}
	var accounts int64
	if err = db.Model(&models.PlayerAccount{}).Count(&accounts).Error; err != nil || accounts != 1 {
		t.Fatalf("expected one account, count=%d err=%v", accounts, err)
	}
}

func TestConcurrentAccountResolutionCreatesOneAccountAndIdentity(t *testing.T) {
	dsn := fmt.Sprintf("file:accounts-%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", time.Now().UnixNano())
	db := newGameplayDB(t, dsn)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	service := NewAccountService(db)
	event := oneBotIdentity("group-a", "10001")
	start := make(chan struct{})
	results := make(chan *models.PlayerAccount, 2)
	errorsCh := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			account, resolveErr := service.Resolve(context.Background(), event)
			results <- account
			errorsCh <- resolveErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsCh)
	for resolveErr := range errorsCh {
		if resolveErr != nil {
			t.Fatalf("concurrent resolution failed: %v", resolveErr)
		}
	}
	ids := make(map[string]struct{})
	for account := range results {
		if account == nil {
			t.Fatal("concurrent resolution returned a nil account")
		}
		ids[account.ID] = struct{}{}
	}
	if len(ids) != 1 {
		t.Fatalf("concurrent resolution returned different accounts: %#v", ids)
	}
	var accountCount, identityCount int64
	db.Model(&models.PlayerAccount{}).Count(&accountCount)
	db.Model(&models.PlayerIdentity{}).Count(&identityCount)
	if accountCount != 1 || identityCount != 1 {
		t.Fatalf("concurrent resolution left orphan rows: accounts=%d identities=%d", accountCount, identityCount)
	}
}

func TestPetServiceAdoptsFromSpeciesStatsAndRejectsSecondPet(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetSpeciesConfig{
		Name: "光芽兽", Health: 88, HealthMax: 120, Hunger: 70, HungerMax: 110,
		Wisdom: 14, Strength: 12, Defense: 16,
	}).Error; err != nil {
		t.Fatal(err)
	}
	service := NewPetService(db)
	pet, err := service.Adopt(context.Background(), "account-1", "光芽兽", "小光芽兽")
	if err != nil {
		t.Fatal(err)
	}
	if pet.Health != 88 || pet.HealthMax != 120 || pet.Hunger != 70 || pet.HungerMax != 110 || pet.Wisdom != 14 || pet.Strength != 12 || pet.Defense != 16 {
		t.Fatalf("species stats were not copied into the unique pet profile: %#v", pet)
	}
	if pet.CurrentForm != "光芽兽" || pet.Status != "空闲" {
		t.Fatalf("pet lifecycle defaults are incomplete: %#v", pet)
	}
	if _, err = service.Adopt(context.Background(), "account-1", "苔须灵", "小苔须灵"); !errors.Is(err, ErrPetAlreadyExists) {
		t.Fatalf("expected one pet per account, got %v", err)
	}
}

func TestDailyServiceUsesConfiguredRewardAtomically(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	if err := db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", MoodPoints: 50,
		Readiness: 80, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CheckinRewardConfig{
		Type: "checkin_newbie", Day: "1", Currency: 25, Affection: 3,
		Items: "小饼干*2#纪念徽章", Image: "签到/第一天.png",
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 17, 9, 0, 0, 0, time.Local)
	service := NewDailyService(db)
	service.Now = func() time.Time { return now }

	result, err := service.CheckIn(context.Background(), "account-1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Awarded || result.Currency != 25 || result.Affection != 3 || result.Image != "签到/第一天.png" || len(result.Items) != 2 {
		t.Fatalf("configured reward was not returned: %#v", result)
	}
	balance, err := service.Wallet.Balance(context.Background(), "account-1", DefaultCurrencyKey)
	if err != nil || balance != 25 {
		t.Fatalf("wallet reward mismatch: balance=%d err=%v", balance, err)
	}
	items, err := service.Inventory.List(context.Background(), "account-1", 20)
	quantities := make(map[string]int64, len(items))
	for _, item := range items {
		quantities[item.ItemName] = item.Quantity
	}
	if err != nil || len(items) != 2 || quantities["小饼干"] != 2 || quantities["纪念徽章"] != 1 {
		t.Fatalf("inventory reward mismatch: items=%#v err=%v", items, err)
	}
	var pet models.PetProfile
	if err = db.First(&pet, "account_id = ?", "account-1").Error; err != nil {
		t.Fatal(err)
	}
	if pet.Affection != 3 || pet.Readiness != 90 || pet.Mood != "一般" || pet.MoodPoints != 55 {
		t.Fatalf("pet reward mismatch: %#v", pet)
	}

	second, err := service.CheckIn(context.Background(), "account-1")
	if err != nil {
		t.Fatal(err)
	}
	if second.Awarded {
		t.Fatal("same-day check-in must not pay twice")
	}
	balance, _ = service.Wallet.Balance(context.Background(), "account-1", DefaultCurrencyKey)
	if balance != 25 {
		t.Fatalf("duplicate check-in changed wallet balance: %d", balance)
	}
}

func TestDailyServiceRollsBackMalformedConfiguredReward(t *testing.T) {
	db := newGameplayDB(t, ":memory:")
	db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般",
		Readiness: 80, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	})
	db.Create(&models.CheckinRewardConfig{Type: "checkin_newbie", Day: "1", Affection: 3, Items: "坏奖励*零"})
	service := NewDailyService(db)
	service.Now = func() time.Time { return time.Date(2026, 8, 17, 9, 0, 0, 0, time.Local) }
	if _, err := service.CheckIn(context.Background(), "account-1"); err == nil {
		t.Fatal("malformed reward config should fail the transaction")
	}
	var journals int64
	db.Model(&models.CompanionJournal{}).Where("account_id = ?", "account-1").Count(&journals)
	if journals != 0 {
		t.Fatalf("failed reward transaction left a check-in record: %d", journals)
	}
	var pet models.PetProfile
	db.First(&pet, "account_id = ?", "account-1")
	if pet.Affection != 0 || pet.Readiness != 80 {
		t.Fatalf("failed reward transaction changed pet state: %#v", pet)
	}
}

func TestConcurrentDailyCheckInPaysConfiguredRewardOnce(t *testing.T) {
	dsn := fmt.Sprintf("file:daily-%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", time.Now().UnixNano())
	db := newGameplayDB(t, dsn)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	if err = db.Create(&models.PetProfile{
		AccountID: "account-1", PetType: "光芽兽", Name: "光芽兽", CurrentForm: "光芽兽",
		Role: "探索者", Stance: "探索", Status: "空闲", Mood: "一般", MoodPoints: 50,
		Readiness: 80, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.CheckinRewardConfig{
		Type: "checkin_newbie", Day: "1", Currency: 10, Affection: 2, Items: "陪伴印记*1",
	}).Error; err != nil {
		t.Fatal(err)
	}
	service := NewDailyService(db)
	service.Now = func() time.Time { return time.Date(2026, 8, 17, 9, 0, 0, 0, time.Local) }

	start := make(chan struct{})
	results := make(chan *DailyCheckinResult, 2)
	errorsCh := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			result, checkinErr := service.CheckIn(context.Background(), "account-1")
			results <- result
			errorsCh <- checkinErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsCh)
	for checkinErr := range errorsCh {
		if checkinErr != nil {
			t.Fatalf("concurrent check-in returned an unexpected error: %v", checkinErr)
		}
	}
	awarded := 0
	for result := range results {
		if result != nil && result.Awarded {
			awarded++
		}
	}
	if awarded != 1 {
		t.Fatalf("configured reward should be paid once, awarded=%d", awarded)
	}
	var journals int64
	db.Model(&models.CompanionJournal{}).Where("account_id = ?", "account-1").Count(&journals)
	if journals != 1 {
		t.Fatalf("expected one check-in journal, got %d", journals)
	}
	balance, err := service.Wallet.Balance(context.Background(), "account-1", DefaultCurrencyKey)
	if err != nil || balance != 10 {
		t.Fatalf("wallet reward was duplicated: balance=%d err=%v", balance, err)
	}
	var pet models.PetProfile
	db.First(&pet, "account_id = ?", "account-1")
	if pet.Affection != 2 || pet.Readiness != 90 {
		t.Fatalf("pet reward was duplicated: %#v", pet)
	}
}

func TestInventoryAndWalletConditionalDebitsStayNonNegativeUnderConcurrency(t *testing.T) {
	dsn := fmt.Sprintf("file:gameplay-%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", time.Now().UnixNano())
	db := newGameplayDB(t, dsn)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	inventory := NewInventoryService(db)
	wallet := NewWalletService(db)
	if err = inventory.Credit(context.Background(), "account-1", "苹果", 1); err != nil {
		t.Fatal(err)
	}
	if err = wallet.Credit(context.Background(), "account-1", DefaultCurrencyKey, 1); err != nil {
		t.Fatal(err)
	}

	assertOneDebit := func(name string, debit func() error, insufficient error) {
		t.Helper()
		start := make(chan struct{})
		results := make(chan error, 2)
		var wait sync.WaitGroup
		for range 2 {
			wait.Add(1)
			go func() {
				defer wait.Done()
				<-start
				results <- debit()
			}()
		}
		close(start)
		wait.Wait()
		close(results)
		successes, rejected := 0, 0
		for result := range results {
			switch {
			case result == nil:
				successes++
			case errors.Is(result, insufficient):
				rejected++
			default:
				t.Fatalf("%s returned unexpected concurrent error: %v", name, result)
			}
		}
		if successes != 1 || rejected != 1 {
			t.Fatalf("%s debit results mismatch: successes=%d rejected=%d", name, successes, rejected)
		}
	}

	assertOneDebit("inventory", func() error {
		return inventory.Debit(context.Background(), "account-1", "苹果", 1)
	}, ErrInsufficientItem)
	assertOneDebit("wallet", func() error {
		return wallet.Debit(context.Background(), "account-1", DefaultCurrencyKey, 1)
	}, ErrInsufficientFunds)

	var item models.GlobalInventoryItem
	db.First(&item, "account_id = ? AND item_name = ?", "account-1", "苹果")
	if item.Quantity != 0 {
		t.Fatalf("inventory became inconsistent: %d", item.Quantity)
	}
	balance, err := wallet.Balance(context.Background(), "account-1", DefaultCurrencyKey)
	if err != nil || balance != 0 {
		t.Fatalf("wallet became inconsistent: balance=%d err=%v", balance, err)
	}
}
