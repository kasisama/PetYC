//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/config"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
)

func main() {
	path := "pet_game.db"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	abs, err := filepath.Abs(path)
	must(err)
	removeIfExists(abs + "-wal")
	removeIfExists(abs + "-shm")
	if _, err := os.Stat(abs); err == nil {
		backup := abs + ".bak-" + time.Now().Format("20060102-150405")
		if err := os.Rename(abs, backup); err != nil {
			alt := filepath.Join(filepath.Dir(abs), "pet_game.official.db")
			fmt.Printf("live database is locked (%v); building %s instead\n", err, alt)
			abs = alt
			removeIfExists(abs)
			removeIfExists(abs + "-wal")
			removeIfExists(abs + "-shm")
		}
		fmt.Println("backed up previous database ->", backup)
	}
	db, err := gorm.Open(sqlite.Open(abs), &gorm.Config{})
	must(err)
	must(database.MigrateSchema(db))
	must(config.RebuildOfficialDefaults(db))
	must(db.Exec("VACUUM").Error)
	var maps, pets, items, accounts, playerPets, wallets, inventory int64
	must(db.Model(&models.AdventureMapConfig{}).Count(&maps).Error)
	must(db.Model(&models.PetSpeciesConfig{}).Count(&pets).Error)
	must(db.Model(&models.ItemConfig{}).Count(&items).Error)
	must(db.Model(&models.PlayerAccount{}).Count(&accounts).Error)
	must(db.Model(&models.PetProfile{}).Count(&playerPets).Error)
	must(db.Model(&models.PlayerWallet{}).Count(&wallets).Error)
	must(db.Model(&models.GlobalInventoryItem{}).Count(&inventory).Error)
	if accounts != 0 || playerPets != 0 || wallets != 0 || inventory != 0 {
		panic(fmt.Errorf("player data leaked into default db: accounts=%d pets=%d wallets=%d inventory=%d", accounts, playerPets, wallets, inventory))
	}
	var profile models.ConfigProfile
	must(db.First(&profile, "id = ?", config.OfficialProfileID).Error)
	if profile.AppVersion != "0.1.0" || profile.SchemaVersion != 2 {
		panic(fmt.Errorf("official profile is not v0.1.0/schema 2: app=%s schema=%d", profile.AppVersion, profile.SchemaVersion))
	}
	for _, table := range []string{"adventure_item_configs", "player_adventure_inventory_items", "player_adventure_wallets", "adventure_wallet_ledgers"} {
		var name string
		if err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name).Error; err != nil {
			panic(err)
		}
		if name != "" {
			panic(fmt.Errorf("legacy table still present: %s", table))
		}
	}
	var tokenItems int64
	must(db.Model(&models.ItemConfig{}).Where("key = ?", "season_token").Count(&tokenItems).Error)
	if tokenItems != 0 {
		panic(fmt.Errorf("season_token leaked into item catalog: %d", tokenItems))
	}
	fmt.Printf("rebuilt %s maps=%d species=%d items=%d players=0 profile=%s schema=%d\n", abs, maps, pets, items, profile.AppVersion, profile.SchemaVersion)
	var sequences []struct {
		Name string
		Seq  int64
	}
	if err := db.Raw("SELECT name, seq FROM sqlite_sequence ORDER BY name").Scan(&sequences).Error; err == nil {
		for _, row := range sequences {
			fmt.Printf("sqlite_sequence %s=%d\n", row.Name, row.Seq)
		}
	}
}

func removeIfExists(path string) {
	_ = os.Remove(path)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
