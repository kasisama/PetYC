package expedition

import (
	"context"
	"testing"
	"time"

	"qq-pet-saas/config"
	"qq-pet-saas/gameplay"
	"qq-pet-saas/models"
)

func adoptTwoExpeditionPets(t *testing.T, service *Service) (*models.PlayerAccount, *models.PetProfile, *models.PetProfile) {
	t.Helper()
	account, err := service.ResolveAccount(context.Background(), oneBotEvent("multi-pet", "owner", ""))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		gameplay.MaxPetSlotsConfigKey:       "2",
		gameplay.MaxConcurrentRunsConfigKey: "1",
	} {
		if err = service.DB.Save(&models.SystemConfig{Key: key, Value: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
	starters := config.StarterPets()
	if len(starters) == 0 {
		t.Fatal("starter pet configuration is empty")
	}
	first, err := service.Adopt(context.Background(), account.ID, starters[0], "光芽")
	if err != nil {
		t.Fatal(err)
	}
	secondSpecies := models.PetSpeciesConfig{
		Key: "emberpaw_test_base", Name: "测试烬爪兽", FamilyKey: "emberpaw_test", Stage: "base", Adoptable: true,
		Archetype: "strength", Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100, Strength: 15, Defense: 10, Wisdom: 8,
	}
	if err = service.DB.Create(&secondSpecies).Error; err != nil {
		t.Fatal(err)
	}
	second, err := service.Adopt(context.Background(), account.ID, secondSpecies.Key, "烬爪")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = gameplay.NewPetService(service.DB).SetActive(context.Background(), account.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	return account, first, second
}

func TestExpeditionSettlementStaysOnStartingPetAfterSwitch(t *testing.T) {
	service, db, now := newTestService(t)
	account, first, second := adoptTwoExpeditionPets(t, service)
	run, err := service.StartExpedition(context.Background(), account.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if run.PetID != first.ID {
		t.Fatalf("run captured wrong pet: got=%s want=%s", run.PetID, first.ID)
	}
	if _, err = gameplay.NewPetService(db).SetActive(context.Background(), account.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	*now = run.EndsAt.Add(time.Second)
	if _, err = service.ClaimExpedition(context.Background(), account.ID); err != nil {
		t.Fatal(err)
	}
	var settledFirst, untouchedSecond models.PetProfile
	if err = db.First(&settledFirst, "id = ?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.First(&untouchedSecond, "id = ?", second.ID).Error; err != nil {
		t.Fatal(err)
	}
	if settledFirst.Growth != run.RewardGrowth || settledFirst.Status != "空闲" {
		t.Fatalf("starting pet was not settled: %#v", settledFirst)
	}
	if untouchedSecond.Growth != 0 || untouchedSecond.Status != "空闲" {
		t.Fatalf("active pet was mutated: %#v", untouchedSecond)
	}
}

func TestFishingSettlementStaysOnStartingPetAfterSwitch(t *testing.T) {
	service, db, now := newTestService(t)
	seedChanceGames(t, db)
	account, first, second := adoptTwoExpeditionPets(t, service)
	if err := gameplay.NewWalletService(db).Credit(context.Background(), account.ID, gameplay.DefaultCurrencyKey, 10); err != nil {
		t.Fatal(err)
	}
	run, _, _, err := service.StartFishing(context.Background(), account.ID, "multi-pet-fishing")
	if err != nil {
		t.Fatal(err)
	}
	if run.PetID != first.ID {
		t.Fatalf("run captured wrong pet: got=%s want=%s", run.PetID, first.ID)
	}
	if _, err = gameplay.NewPetService(db).SetActive(context.Background(), account.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	*now = run.ReadyAt.Add(time.Second)
	claimed, err := service.ClaimFishing(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.PetID != first.ID {
		t.Fatalf("claim used wrong pet: %#v", claimed)
	}
	var settledFirst, untouchedSecond models.PetProfile
	db.First(&settledFirst, "id = ?", first.ID)
	db.First(&untouchedSecond, "id = ?", second.ID)
	if settledFirst.Status != "空闲" || untouchedSecond.Status != "空闲" {
		t.Fatalf("fishing status leaked: first=%s second=%s", settledFirst.Status, untouchedSecond.Status)
	}
}

func TestAdventureRetreatStaysOnStartingPetAfterSwitch(t *testing.T) {
	service, db, _ := newTestService(t)
	service.RandomIntn = func(int) (int, error) { return 0, nil }
	account, first, second := adoptTwoExpeditionPets(t, service)
	for _, row := range []any{
		&models.AdventureMapConfig{Key: "multi-map", Name: "多宠测试地图", Region: "测试原野", RecommendedLevel: 1, Enabled: true},
		&models.AdventureZoneConfig{Key: "multi-zone", MapKey: "multi-map", Name: "多宠测试区域", RecommendedLevel: 1, DifficultyPermille: 1000, Enabled: true},
		&models.AdventureMonsterConfig{Key: "multi-monster", Name: "测试生物", Level: 1, MaxHealth: 100, Attack: 1, Enabled: true},
		&models.AdventureEncounterConfig{ZoneKey: "multi-zone", EncounterKey: "multi-encounter", EncounterType: "monster", TargetKey: "multi-monster", Name: "测试遭遇", Weight: 1, Enabled: true},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	exploration, err := service.ExploreZone(context.Background(), account.ID, "multi-zone")
	if err != nil {
		t.Fatal(err)
	}
	if exploration.Session.PetID != first.ID || exploration.Combat == nil || exploration.Combat.PetID != first.ID {
		t.Fatalf("adventure captured wrong pet: %#v", exploration)
	}
	if _, err = gameplay.NewPetService(db).SetActive(context.Background(), account.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	result, err := service.CombatAction(context.Background(), account.ID, "multi-retreat", "retreat")
	if err != nil {
		t.Fatal(err)
	}
	if result.Session.PetID != first.ID || result.Session.Status != "retreated" {
		t.Fatalf("retreat settled wrong session: %#v", result.Session)
	}
	var settledFirst, untouchedSecond models.PetProfile
	db.First(&settledFirst, "id = ?", first.ID)
	db.First(&untouchedSecond, "id = ?", second.ID)
	if settledFirst.Status != "空闲" || untouchedSecond.Status != "空闲" {
		t.Fatalf("adventure status leaked: first=%s second=%s", settledFirst.Status, untouchedSecond.Status)
	}
}

func TestSetStanceOnlyUpdatesActivePet(t *testing.T) {
	service, db, _ := newTestService(t)
	account, first, second := adoptTwoExpeditionPets(t, service)
	if _, err := gameplay.NewPetService(db).SetActive(context.Background(), account.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.SetStance(context.Background(), account.ID, "守护"); err != nil {
		t.Fatal(err)
	}
	var original, active models.PetProfile
	db.First(&original, "id = ?", first.ID)
	db.First(&active, "id = ?", second.ID)
	if original.Stance == "守护" || active.Stance != "守护" {
		t.Fatalf("stance leaked across pets: original=%s active=%s", original.Stance, active.Stance)
	}
}
