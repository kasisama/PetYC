package gameplay

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

var multiPetDBSeq atomic.Int64

func newMultiPetDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:multi-pet-%s-%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", t.Name(), multiPetDBSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	if err = db.AutoMigrate(
		&models.SystemConfig{}, &models.PlayerAccount{}, &models.PetProfile{},
		&models.PetSpeciesConfig{}, &models.GlobalInventoryItem{}, &models.PlayerWallet{}, &models.WalletLedger{},
		&models.ItemConfig{}, &models.ActivityRun{}, &models.ItemUseRecord{}, &models.ExpeditionRun{},
		&models.FishingRun{}, &models.AdventureExplorationSession{}, &models.AdventureCombatSession{},
		&models.AdventureExpeditionRun{}, &models.PlayerEquipment{},
	); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_activity_one_running ON activity_runs(account_id, pet_id) WHERE status = 'running'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_expedition_one_running ON expedition_runs(account_id, pet_id) WHERE status = 'running'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_player_equipment_one_slot ON player_equipments(equipped_pet_id, equipped_slot) WHERE equipped_pet_id <> '' AND equipped_slot <> ''`,
	} {
		if err = db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func seedAdoptableFamily(t *testing.T, db *gorm.DB, key, name string) {
	t.Helper()
	if err := db.Create(&models.PetSpeciesConfig{
		Key: key, Name: name, FamilyKey: key, Stage: "base", Adoptable: true, Archetype: "balanced",
		Health: 80, HealthMax: 100, Hunger: 100, HungerMax: 100, Wisdom: 10, Strength: 10, Defense: 10,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func setSystem(t *testing.T, db *gorm.DB, key, value string) {
	t.Helper()
	if err := db.Save(&models.SystemConfig{Key: key, Value: value}).Error; err != nil {
		t.Fatal(err)
	}
}

func adoptTwo(t *testing.T, db *gorm.DB) (*models.PetProfile, *models.PetProfile) {
	t.Helper()
	setSystem(t, db, MaxPetSlotsConfigKey, "2")
	setSystem(t, db, MaxConcurrentRunsConfigKey, "1")
	seedAdoptableFamily(t, db, "lumisprout_base", "光芽兽")
	seedAdoptableFamily(t, db, "emberpaw_base", "烬爪兽")
	if err := db.Create(&models.PlayerAccount{ID: "account-1"}).Error; err != nil {
		t.Fatal(err)
	}
	pets := NewPetService(db)
	first, err := pets.Adopt(context.Background(), "account-1", "lumisprout_base", "光芽")
	if err != nil {
		t.Fatal(err)
	}
	second, err := pets.Adopt(context.Background(), "account-1", "emberpaw_base", "烬爪")
	if err != nil {
		t.Fatal(err)
	}
	return first, second
}

func TestAccountCanHoldTwoPetsAndSwitchActive(t *testing.T) {
	db := newMultiPetDB(t)
	first, second := adoptTwo(t, db)
	pets := NewPetService(db)
	listed, err := pets.List(context.Background(), "account-1")
	if err != nil || len(listed) != 2 {
		t.Fatalf("want two pets, got %d err=%v", len(listed), err)
	}
	active, err := pets.Get(context.Background(), "account-1")
	if err != nil || active.ID != first.ID {
		t.Fatalf("first adoptee should be active: %#v err=%v", active, err)
	}
	switched, err := pets.SetActive(context.Background(), "account-1", second.ID)
	if err != nil || switched.ID != second.ID {
		t.Fatalf("switch failed: %#v err=%v", switched, err)
	}
	active, err = pets.Get(context.Background(), "account-1")
	if err != nil || active.ID != second.ID {
		t.Fatalf("active pet not persisted: %#v", active)
	}
	if _, err = pets.SetActive(context.Background(), "account-1", "missing"); !errors.Is(err, ErrPetRequired) {
		t.Fatalf("foreign pet should be rejected, got %v", err)
	}
}

func TestCannotSwitchToAnotherAccountPet(t *testing.T) {
	db := newMultiPetDB(t)
	first, _ := adoptTwo(t, db)
	if err := db.Create(&models.PlayerAccount{ID: "account-2"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewPetService(db).SetActive(context.Background(), "account-2", first.ID); !errors.Is(err, ErrPetRequired) {
		t.Fatalf("expected cross-account switch rejection, got %v", err)
	}
}

func TestActivitySettlementStaysOnStartingPetAfterSwitch(t *testing.T) {
	db := newMultiPetDB(t)
	first, second := adoptTwo(t, db)
	if err := db.Create(&models.ItemConfig{Name: "观察笔记", Status: "active", Type: "智慧", Effect: "5", Time: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := NewInventoryService(db).Credit(context.Background(), "account-1", "观察笔记", 1); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.Local)
	activity := NewActivityService(db)
	activity.Now = func() time.Time { return now }
	if _, err := activity.Start(context.Background(), "account-1", ActivityStartRequest{
		Kind: ActivityStudy, ItemName: "观察笔记", RequiredItemType: "智慧", RewardAttribute: "智慧",
		HungerCost: 8, RewardGrowth: 3,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPetService(db).SetActive(context.Background(), "account-1", second.ID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	completed, err := activity.Complete(context.Background(), "account-1", ActivityStudy, DefaultCurrencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Run.PetID != first.ID {
		t.Fatalf("settlement used wrong pet: %#v", completed.Run)
	}
	var original, current models.PetProfile
	if err = db.First(&original, "id = ?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.First(&current, "id = ?", second.ID).Error; err != nil {
		t.Fatal(err)
	}
	if original.Wisdom <= 10 || original.Growth != 3 || original.Status != "空闲" {
		t.Fatalf("starting pet was not settled: %#v", original)
	}
	if current.Wisdom != 10 || current.Growth != 0 || current.Status != "空闲" {
		t.Fatalf("active pet was mutated: %#v", current)
	}
}

func TestEquipmentIsIsolatedPerPetAndExclusive(t *testing.T) {
	db := newMultiPetDB(t)
	first, second := adoptTwo(t, db)
	firstGear := models.PlayerEquipment{ID: "eq-1", AccountID: "account-1", TemplateKey: "weapon_1", EquippedPetID: first.ID, EquippedSlot: "weapon"}
	secondGear := models.PlayerEquipment{ID: "eq-2", AccountID: "account-1", TemplateKey: "weapon_2", EquippedPetID: second.ID, EquippedSlot: "weapon"}
	if err := db.Create(&firstGear).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&secondGear).Error; err != nil {
		t.Fatal(err)
	}
	conflict := models.PlayerEquipment{ID: "eq-3", AccountID: "account-1", TemplateKey: "weapon_3", EquippedPetID: first.ID, EquippedSlot: "weapon"}
	if err := db.Create(&conflict).Error; err == nil {
		t.Fatal("same pet same slot must be unique")
	}
}

func TestDirectedItemUseAndIdempotentRetry(t *testing.T) {
	db := newMultiPetDB(t)
	first, second := adoptTwo(t, db)
	if err := db.Create(&models.ItemConfig{Name: "负重藤环", Status: "active", Type: "成长", Effect: "6"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := NewInventoryService(db).Credit(context.Background(), "account-1", "负重藤环", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPetService(db).SetActive(context.Background(), "account-1", second.ID); err != nil {
		t.Fatal(err)
	}
	effects := NewItemEffectService(db)
	firstUse, err := effects.UseOn(context.Background(), "account-1", first.ID, "负重藤环", 1, "use-key-1")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := effects.UseOn(context.Background(), "account-1", first.ID, "负重藤环", 1, "use-key-1")
	if err != nil || !replay.Duplicate || replay.Record.ID != firstUse.Record.ID {
		t.Fatalf("retry must replay without a second grant: %#v err=%v", replay, err)
	}
	var original, current models.PetProfile
	db.First(&original, "id = ?", first.ID)
	db.First(&current, "id = ?", second.ID)
	if original.Growth != 6 {
		t.Fatalf("directed use missed starting pet: %#v", original)
	}
	if current.Growth != 0 {
		t.Fatalf("directed use mutated active pet: %#v", current)
	}
}

func TestPetSlotLimitsFollowConfiguration(t *testing.T) {
	db := newMultiPetDB(t)
	setSystem(t, db, MaxPetSlotsConfigKey, "1")
	seedAdoptableFamily(t, db, "lumisprout_base", "光芽兽")
	seedAdoptableFamily(t, db, "emberpaw_base", "烬爪兽")
	pets := NewPetService(db)
	if _, err := pets.Adopt(context.Background(), "account-1", "lumisprout_base", "光芽"); err != nil {
		t.Fatal(err)
	}
	if _, err := pets.Adopt(context.Background(), "account-1", "emberpaw_base", "烬爪"); !errors.Is(err, ErrPetAlreadyExists) {
		t.Fatalf("slot 1 must reject second pet, got %v", err)
	}
	setSystem(t, db, MaxPetSlotsConfigKey, "2")
	if _, err := pets.Adopt(context.Background(), "account-1", "emberpaw_base", "烬爪"); err != nil {
		t.Fatalf("slot 2 must allow second pet: %v", err)
	}
}

func TestConcurrentRunLimitsFollowConfiguration(t *testing.T) {
	db := newMultiPetDB(t)
	first, second := adoptTwo(t, db)
	activity := NewActivityService(db)
	if _, err := activity.Start(context.Background(), "account-1", ActivityStartRequest{Kind: ActivityWork, Duration: time.Minute, HungerCost: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPetService(db).SetActive(context.Background(), "account-1", second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := activity.Start(context.Background(), "account-1", ActivityStartRequest{Kind: ActivityFitness, Duration: time.Minute, HungerCost: 5}); !errors.Is(err, ErrTooManyConcurrentRuns) {
		t.Fatalf("max concurrent 1 should reject second pet run, got %v", err)
	}
	setSystem(t, db, MaxConcurrentRunsConfigKey, "2")
	if _, err := activity.Start(context.Background(), "account-1", ActivityStartRequest{Kind: ActivityFitness, Duration: time.Minute, HungerCost: 5}); err != nil {
		t.Fatalf("max concurrent 2 should allow other pet: %v", err)
	}
	if _, err := NewPetService(db).SetActive(context.Background(), "account-1", first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := activity.Start(context.Background(), "account-1", ActivityStartRequest{Kind: ActivityTrain, Duration: time.Minute, HungerCost: 5}); !errors.Is(err, ErrActivityActive) {
		t.Fatalf("same pet cannot start a second run, got %v", err)
	}
}

func TestSQLiteConcurrentAdoptRespectsSlotLimit(t *testing.T) {
	db := newMultiPetDB(t)
	setSystem(t, db, MaxPetSlotsConfigKey, "1")
	seedAdoptableFamily(t, db, "lumisprout_base", "光芽兽")
	pets := NewPetService(db)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := pets.Adopt(context.Background(), "account-1", "lumisprout_base", "光芽"+string(rune('A'+index)))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	var ok, denied int
	for err := range errs {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrPetAlreadyExists) || errors.Is(err, ErrPetRequired) {
			denied++
		} else {
			t.Fatalf("unexpected adopt error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("want exactly one successful adopt, got %d ok %d denied", ok, denied)
	}
	var count int64
	db.Model(&models.PetProfile{}).Where("account_id = ?", "account-1").Count(&count)
	if count != 1 {
		t.Fatalf("slot limit leaked: %d pets", count)
	}
}

func TestSQLiteConcurrentRunReservationRespectsAccountLimit(t *testing.T) {
	db := newMultiPetDB(t)
	first, second := adoptTwo(t, db)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for index, petID := range []string{first.ID, second.ID} {
		wg.Add(1)
		go func(index int, petID string) {
			defer wg.Done()
			<-start
			err := WithTransactionRetry(context.Background(), db, func(tx *gorm.DB) error {
				if err := ReservePetRunTx(tx, "account-1", petID); err != nil {
					return err
				}
				if err := tx.Model(&models.PetProfile{}).Where("id = ?", petID).Update("status", "活动").Error; err != nil {
					return err
				}
				run := models.ActivityRun{ID: "concurrent-run-" + string(rune('a'+index)), AccountID: "account-1", PetID: petID, Kind: ActivityWork, Status: ActivityStatusRunning, StartsAt: time.Now(), EndsAt: time.Now().Add(time.Minute)}
				return tx.Create(&run).Error
			})
			errs <- err
		}(index, petID)
	}
	close(start)
	wg.Wait()
	close(errs)

	successes, rejected := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrTooManyConcurrentRuns), errors.Is(err, ErrActivityActive):
			rejected++
		default:
			t.Fatalf("unexpected concurrent reservation error: %v", err)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("account run limit leaked: successes=%d rejected=%d", successes, rejected)
	}
	var running int64
	if err := db.Model(&models.ActivityRun{}).Where("account_id = ? AND status = ?", "account-1", ActivityStatusRunning).Count(&running).Error; err != nil || running != 1 {
		t.Fatalf("want exactly one running activity, running=%d err=%v", running, err)
	}
}
