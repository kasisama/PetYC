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
		&models.PlayerWallet{}, &models.WalletLedger{}, &models.ActivityRun{}, &models.ItemUseRecord{}, &models.ExpeditionTemplateConfig{}, &models.ExpeditionRun{}, &models.EventProgress{}, &models.EventProgressGrant{}, &models.EventRewardClaim{}, &models.ChanceGameConfig{}, &models.ChanceRewardConfig{}, &models.ChanceDailyState{}, &models.ChancePlayerState{}, &models.ChanceOutcome{}, &models.FishingRun{}, &models.BattleRecord{}, &models.TradeOffer{}, &models.TradeAudit{}, &models.Community{}, &models.ExpeditionSquad{},
		&models.IdentityBindToken{},
	} {
		if !types[reflect.TypeOf(required)] {
			t.Fatalf("migration is missing %T", required)
		}
	}
}

func TestFreshSchemaExcludesLegacyPlayerTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"user_pets", "backpack_items", "families"} {
		var count int64
		if err = db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("legacy table %s must not exist in a clean database", table)
		}
	}
}

func TestMigrateSchemaAddsMenuMarkdownWithoutChangingExistingReply(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`CREATE TABLE menu_configs (name text PRIMARY KEY, reply text, image text)`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`INSERT INTO menu_configs(name, reply, image) VALUES (?, ?, ?)`, "主菜单", "管理员旧内容", "上传/menu.webp").Error; err != nil {
		t.Fatal(err)
	}
	if err = MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&models.MenuConfig{}, "Markdown") {
		t.Fatal("menu_configs.markdown was not added")
	}
	var row models.MenuConfig
	if err = db.First(&row, "name = ?", "主菜单").Error; err != nil {
		t.Fatal(err)
	}
	if row.Reply != "管理员旧内容" || row.Markdown != "" || row.Image != "上传/menu.webp" {
		t.Fatalf("legacy menu changed during migration: %#v", row)
	}
}

func TestMigrateSchemaAllowsOnlyOneRunningExpeditionPerAccount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	first := models.ExpeditionRun{ID: "run-1", AccountID: "account-1", Tier: 1, Name: "巡查", Stance: "探索", Status: "running", RewardItem: "木材"}
	second := models.ExpeditionRun{ID: "run-2", AccountID: "account-1", Tier: 1, Name: "巡查", Stance: "探索", Status: "running", RewardItem: "木材"}
	if err = db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&second).Error; err == nil {
		t.Fatal("expected the second running expedition to violate the partial unique index")
	}
	if err = db.Model(&first).Updates(map[string]interface{}{"status": "claimed"}).Error; err != nil {
		t.Fatal(err)
	}
	second.Status = "running"
	if err = db.Create(&second).Error; err != nil {
		t.Fatalf("a new run should be allowed after the active slot is released: %v", err)
	}
	third := models.ExpeditionRun{ID: "run-3", AccountID: "account-1", Tier: 1, Name: "巡查", Stance: "探索", Status: "claimed", RewardItem: "木材"}
	if err = db.Create(&third).Error; err != nil {
		t.Fatalf("claimed expedition history must remain append-only: %v", err)
	}
}

func TestMigrateSchemaAllowsOnlyOneRunningActivityPerAccount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	first := models.ActivityRun{ID: "activity-1", AccountID: "account-1", Kind: "学习", ConfigKey: "学习", Status: "running"}
	second := models.ActivityRun{ID: "activity-2", AccountID: "account-1", Kind: "打工", ConfigKey: "整理书架", Status: "running"}
	if err = db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&second).Error; err == nil {
		t.Fatal("expected the second running activity to violate the partial unique index")
	}
	if err = db.Model(&first).Update("status", "claimed").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&second).Error; err != nil {
		t.Fatalf("a new activity should be allowed after the active slot is released: %v", err)
	}
	third := models.ActivityRun{ID: "activity-3", AccountID: "account-1", Kind: "学习", ConfigKey: "学习", Status: "claimed"}
	if err = db.Create(&third).Error; err != nil {
		t.Fatalf("claimed activity history must remain append-only: %v", err)
	}
}

func TestMigrateSchemaAllowsOnlyOneRunningFishingRunPerAccount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	first := models.FishingRun{ID: "fish-1", AccountID: "account-1", Status: "running", ActionKey: "cast-1", RewardKey: "fish", RewardName: "小鱼"}
	second := models.FishingRun{ID: "fish-2", AccountID: "account-1", Status: "running", ActionKey: "cast-2", RewardKey: "fish", RewardName: "小鱼"}
	if err = db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&second).Error; err == nil {
		t.Fatal("expected the second running fishing run to violate the partial unique index")
	}
	if err = db.Model(&first).Update("status", "claimed").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&second).Error; err != nil {
		t.Fatalf("a new fishing run should be allowed after claim: %v", err)
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

	if err = MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	rows := []models.RewardTrackConfig{
		{EventKey: "forest-week", Milestone: 100, RewardType: "item", RewardKey: "wood", RewardName: "木材", Quantity: 2},
		{EventKey: "forest-week", Milestone: 100, RewardType: "item", RewardKey: "survey_log", RewardName: "调查记录", Quantity: 5},
	}
	if err = db.Create(&rows).Error; err != nil {
		t.Fatalf("same milestone should accept multiple reward items: %v", err)
	}
}

func TestOpenSQLiteEnablesWALAndBusyTimeout(t *testing.T) {
	path := t.TempDir() + "/pet_game.db"
	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	var journal string
	if err = db.Raw("PRAGMA journal_mode").Scan(&journal).Error; err != nil {
		t.Fatal(err)
	}
	if journal != "wal" {
		t.Fatalf("journal_mode=%q, want wal", journal)
	}
	var timeout int
	if err = db.Raw("PRAGMA busy_timeout").Scan(&timeout).Error; err != nil {
		t.Fatal(err)
	}
	if timeout < 5000 {
		t.Fatalf("busy_timeout=%d, want >= 5000", timeout)
	}
	if sqlDB.Stats().MaxOpenConnections != 1 {
		t.Fatalf("MaxOpenConnections=%d, want 1 for file-backed SQLite", sqlDB.Stats().MaxOpenConnections)
	}
}
