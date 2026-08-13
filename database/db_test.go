package database

import (
	"reflect"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/models"
)

func TestMigrationModelsIncludeModernGameTables(t *testing.T) {
	types := make(map[reflect.Type]bool)
	for _, model := range migrationModels() {
		types[reflect.TypeOf(model)] = true
	}
	for _, required := range []interface{}{
		&models.PlayerAccount{}, &models.PlayerIdentity{}, &models.PetProfile{},
		&models.ExpeditionRun{}, &models.Community{}, &models.ExpeditionSquad{},
		&models.IdentityBindToken{},
	} {
		if !types[reflect.TypeOf(required)] {
			t.Fatalf("migration is missing %T", required)
		}
	}
}

func TestMigrateSchemaAllowsMultipleRewardsAtSameMilestone(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`CREATE TABLE reward_track_configs (
		id integer PRIMARY KEY AUTOINCREMENT,
		event_key text NOT NULL,
		milestone integer NOT NULL,
		item_name text NOT NULL,
		quantity integer NOT NULL,
		description text,
		sort_order integer
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("CREATE UNIQUE INDEX idx_reward_track ON reward_track_configs(event_key, milestone)").Error; err != nil {
		t.Fatal(err)
	}

	if err = migrateSchema(db); err != nil {
		t.Fatal(err)
	}
	rows := []models.RewardTrackConfig{
		{EventKey: "forest-week", Milestone: 100, ItemName: "木材", Quantity: 2},
		{EventKey: "forest-week", Milestone: 100, ItemName: "调查记录", Quantity: 5},
	}
	if err = db.Create(&rows).Error; err != nil {
		t.Fatalf("same milestone should accept multiple reward items: %v", err)
	}
}
