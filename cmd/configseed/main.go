// Command configseed creates the reviewed v0.0.1 default snapshot from the
// current SQLite configuration tables. It never reads player tables.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	appconfig "qq-pet-saas/config"
)

func main() {
	dbPath := flag.String("db", "pet_game.db", "source SQLite database")
	outPath := flag.String("out", "config/defaults/config_v0.0.1.json", "output snapshot")
	assetOut := flag.String("assets", "config/defaults/assets", "output asset root")
	flag.Parse()
	db, err := gorm.Open(sqlite.Open(*dbPath), &gorm.Config{})
	must(err)
	snapshot, err := appconfig.CaptureSnapshot(db)
	must(err)
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	must(err)
	must(os.MkdirAll(filepath.Dir(*outPath), 0o755))
	must(os.WriteFile(*outPath, append(raw, '\n'), 0o644))
	for _, relative := range referencedAssets(snapshot) {
		must(copyAsset(relative, *assetOut))
	}
	fmt.Printf("generated %s with %d rows\n", *outPath, snapshot.Summary().Rows)
}

func referencedAssets(snapshot appconfig.ConfigSnapshot) []string {
	set := map[string]struct{}{}
	add := func(values ...string) {
		for _, value := range values {
			value = filepath.ToSlash(strings.TrimSpace(value))
			value = strings.TrimPrefix(value, "图片/")
			if value != "" && value != "." && !strings.Contains(value, "..") {
				set[value] = struct{}{}
			}
		}
	}
	for _, row := range snapshot.PetSpecies {
		add(row.Image, row.AdoptImage, row.TrainStartImg, row.TrainEndImg, row.StudyStartImg, row.StudyEndImg, row.FitnessStartImg, row.FitnessEndImg, row.EvolutionImage, row.AwakenImage)
	}
	for _, row := range snapshot.Items {
		add(row.Image)
	}
	for _, row := range snapshot.ShopItems {
		add(row.Image)
	}
	for _, row := range snapshot.CheckinRewards {
		add(row.Image)
	}
	for _, row := range snapshot.WorkSettings {
		add(row.StartImage, row.EndImage)
	}
	for _, row := range snapshot.Images {
		add(row.Path)
	}
	for _, row := range snapshot.ExpeditionTemplates {
		add(row.StartImage, row.EndImage)
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	return result
}

func copyAsset(relative, outputRoot string) error {
	source := filepath.Join("图片", filepath.FromSlash(relative))
	data, err := os.ReadFile(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	target := filepath.Join(outputRoot, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
