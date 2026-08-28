// Command adminserver starts an isolated copy of the real administration
// server for browser integration tests. It uses a temporary SQLite database
// and credential directory, so running the suite never changes local data.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"qq-pet-saas/admin"
	"qq-pet-saas/core"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
	"qq-pet-saas/updater"
)

const (
	listenAddress = "127.0.0.1:18080"
	accountID     = "00000000-0000-0000-0000-000000000001"
)

func main() {
	tempDir, err := os.MkdirTemp("", "qqpet-admin-e2e-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	if err = os.Setenv("QQPET_DATA_DIR", filepath.Join(tempDir, "runtime")); err != nil {
		log.Fatal(err)
	}

	db, err := gorm.Open(sqlite.Open(filepath.Join(tempDir, "integration.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatal(err)
	}
	if err = database.MigrateSchema(db); err != nil {
		log.Fatal(err)
	}
	if err = seed(db); err != nil {
		log.Fatal(err)
	}
	database.DB = db
	core.BuildVersion = "dev"
	admin.UpdateService = updater.NewService(updater.Config{CurrentVersion: "dev"})

	appRouter := core.NewAppRouter()
	server := &http.Server{
		Addr: listenAddress,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/admin/config/status":
				writeTestResponse(w, map[string]interface{}{"db_revision": 1, "loaded_revision": 1, "pending_reload": false})
			case "/api/admin/audit-logs":
				writeTestResponse(w, map[string]interface{}{"items": []interface{}{}, "total": 0})
			case "/api/admin/platforms/status":
				writeTestResponse(w, map[string]interface{}{"onebot": map[string]interface{}{"connected": false}, "qq_official": map[string]interface{}{"connected": false, "capabilities": map[string]interface{}{}}})
			case "/api/admin/onboarding/status":
				writeTestResponse(w, map[string]interface{}{"setup_completed": true, "tour_version_completed": 1, "current_tour_version": 1})
			case "/api/admin/updates/check":
				writeTestResponse(w, map[string]interface{}{
					"currentVersion": "dev", "latestVersion": "", "available": false,
					"canAutoUpdate": false, "installMode": "manual", "reason": "开发构建不支持在线更新",
					"releaseUrl": "https://github.com/kasisama/PetYC/releases/latest",
				})
			default:
				appRouter.ServeHTTP(w, r)
			}
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownSignal.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	fmt.Printf("isolated admin integration server listening on http://%s\n", listenAddress)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func writeTestResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "msg": "success", "data": data})
}

func seed(db *gorm.DB) error {
	now := time.Now()
	records := []interface{}{
		&models.PlayerAccount{ID: accountID, CreatedAt: now, UpdatedAt: now},
		&models.PlayerIdentity{AccountID: accountID, Platform: "onebot", AppID: "integration-app", SceneType: "group", ScopeID: "integration-group", SubjectID: "integration-player", CreatedAt: now},
		&models.PetSpeciesConfig{Key: "lumisprout_base", Name: "光芽兽", FamilyKey: "lumisprout", Stage: "base", Adoptable: true, Archetype: "balanced", FavoriteFood: "调查便当", FavoriteGift: "晴野明信片", Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100, Description: "真实集成测试中的原创调查伙伴"},
		&models.PetProfile{AccountID: accountID, PetType: "lumisprout_base", Name: "光芽兽", CurrentForm: "lumisprout_base", Role: "探索者", Stance: "探索", Status: "空闲", Mood: "愉快", MoodPoints: 80, Readiness: 100, Health: 100, HealthMax: 100, Hunger: 100, HungerMax: 100, Wisdom: 20, Strength: 18, Defense: 16, BondLevel: 2, Growth: 42, CreatedAt: now, UpdatedAt: now},
		&models.ItemConfig{Key: "survey_record", Name: "调查记录", Category: "material", Rarity: "common", Stackable: true, MaxStack: 999999, Status: "active", Type: "材料", Description: "记录远征线索"},
		&models.GlobalInventoryItem{AccountID: accountID, ItemKey: "survey_record", ItemName: "调查记录", Quantity: 2, UpdatedAt: now},
		&models.AdminConfigState{DBRevision: 1, LoadedRevision: 1, SavedAt: &now, LoadedAt: &now},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			return err
		}
	}
	return nil
}
