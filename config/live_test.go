package config

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func liveTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:live-config-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.SystemConfig{}, &models.ImageConfig{}, &models.PetSpeciesConfig{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestLiveStringReadsSystemConfigNotMemory(t *testing.T) {
	db := liveTestDB(t)
	Core.CoinName = "内存星砂"
	if err := db.Create(&models.SystemConfig{Key: "Core.CoinName", Value: "库内星砂"}).Error; err != nil {
		t.Fatal(err)
	}
	if got := LiveString(db, "Core.CoinName", "星砂"); got != "库内星砂" {
		t.Fatalf("LiveString=%q, want 库内星砂", got)
	}
	if got := LiveInt64(db, "Core.InitialCoin", 100); got != 100 {
		t.Fatalf("missing key should use fallback, got %d", got)
	}
	if err := db.Create(&models.SystemConfig{Key: "Core.InitialCoin", Value: "240"}).Error; err != nil {
		t.Fatal(err)
	}
	if got := LiveInt64(db, "Core.InitialCoin", 100); got != 240 {
		t.Fatalf("LiveInt64=%d, want 240", got)
	}
}

func TestLiveImagePathReadsImageConfig(t *testing.T) {
	db := liveTestDB(t)
	Images = map[string]string{"领养": "内存/领养.png"}
	if err := db.Create(&models.ImageConfig{Name: "领养", Path: "核心图片/领养.jpg"}).Error; err != nil {
		t.Fatal(err)
	}
	if got := LiveImagePath(db, "领养", "核心图片/领养宠物.jpg"); got != "核心图片/领养.jpg" {
		t.Fatalf("LiveImagePath=%q", got)
	}
	if got := LiveImagePath(db, "治疗", "核心图片/治疗.jpg"); got != "核心图片/治疗.jpg" {
		t.Fatalf("missing image should use fallback, got %q", got)
	}
}

func TestLiveStarterSpeciesReadsInitialPetsFromDatabase(t *testing.T) {
	db := liveTestDB(t)
	Core.InitialPets = []string{"内存兽"}
	if err := db.Create(&models.SystemConfig{Key: "Core.InitialPets", Value: "团子,苔须灵"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]models.PetSpeciesConfig{
		{Key: "团子", Name: "团子", Description: "这是一个可爱的小兔子。", Adoptable: true, FamilyKey: "团子", Stage: "base"},
		{Key: "苔须灵", Name: "苔须灵", Description: "林地向导", Adoptable: true, FamilyKey: "苔须灵", Stage: "base"},
		{Key: "烬爪兽", Name: "烬爪兽", Adoptable: true, FamilyKey: "烬爪兽", Stage: "base"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	pets := LiveStarterSpecies(db)
	if len(pets) != 2 || pets[0].Name != "团子" || pets[1].Name != "苔须灵" {
		t.Fatalf("starter species=%#v", pets)
	}
}
