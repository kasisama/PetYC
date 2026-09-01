package admin

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	appconfig "qq-pet-saas/config"
	"qq-pet-saas/models"
)

func archiveTestSnapshot() appconfig.ConfigSnapshot {
	return appconfig.ConfigSnapshot{
		System:     []models.SystemConfig{{Key: "Core.Currency", Value: "金币"}},
		Commands:   []models.CommandConfig{{FuncName: "Help", Command: "帮助", Enabled: true}},
		PetSpecies: []models.PetSpeciesConfig{{Name: "米塔"}},
		Items:      []models.ItemConfig{{Name: "苹果", Status: "active"}},
	}
}

func TestProfileArchiveContainsOnlyManifestAndWhitelistConfig(t *testing.T) {
	profile := models.ConfigProfile{ID: "profile", Name: "安全方案", AppVersion: "0.0.1", SchemaVersion: 1}
	raw, err := buildProfileArchive(profile, archiveTestSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "player_account") || strings.Contains(string(raw), "app_secret") || strings.Contains(string(raw), "onebot_token") {
		t.Fatal("archive leaked forbidden data")
	}
	manifest, snapshot, assets, err := parseProfileArchive(raw)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Name != profile.Name || snapshot.Summary().Rows != 4 || len(assets) != 0 {
		t.Fatalf("unexpected archive: %+v %+v", manifest, snapshot.Summary())
	}
}

func TestProfileArchivePreservesMenuMarkdown(t *testing.T) {
	snapshot := archiveTestSnapshot()
	snapshot.Menus = []models.MenuConfig{{Name: "主菜单", Reply: "纯文本菜单", Markdown: "# 宠物菜单"}}

	raw, err := buildProfileArchive(models.ConfigProfile{ID: "profile", Name: "菜单配置", AppVersion: "0.0.1", SchemaVersion: 1}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	_, imported, _, err := parseProfileArchive(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Menus) != 1 || imported.Menus[0].Reply != "纯文本菜单" || imported.Menus[0].Markdown != "# 宠物菜单" {
		t.Fatalf("菜单 Markdown 未在配置包中保留: %#v", imported.Menus)
	}
}

func TestProfileAssetsIncludeAndRewriteMenuImages(t *testing.T) {
	snapshot := archiveTestSnapshot()
	snapshot.Menus = []models.MenuConfig{{Name: "主菜单", Reply: "欢迎", Image: "上传/menu.webp"}}
	paths := snapshotAssetPaths(snapshot)
	if len(paths) != 1 || paths[0] != "上传/menu.webp" {
		t.Fatalf("菜单图片未进入配置包资产清单: %#v", paths)
	}
	rewriteSnapshotAssets(&snapshot, map[string][]byte{"上传/menu.webp": {1}}, "导入/profile")
	if snapshot.Menus[0].Image != "导入/profile/上传/menu.webp" {
		t.Fatalf("菜单图片导入路径未重写: %q", snapshot.Menus[0].Image)
	}
}

func TestProfileAssetsKeepRemoteAndEmptyMenuImages(t *testing.T) {
	snapshot := archiveTestSnapshot()
	snapshot.Menus = []models.MenuConfig{
		{Name: "主菜单", Reply: "欢迎", Image: "https://cdn.example.com/menu.webp"},
		{Name: "帮助", Reply: "帮助", Image: ""},
	}
	if paths := snapshotAssetPaths(snapshot); len(paths) != 0 {
		t.Fatalf("远程或空菜单图片不应进入资产清单: %#v", paths)
	}
	rewriteSnapshotAssets(&snapshot, map[string][]byte{"上传/menu.webp": {1}}, "导入/profile")
	if snapshot.Menus[0].Image != "https://cdn.example.com/menu.webp" {
		t.Fatalf("HTTPS 菜单图片被改写: %q", snapshot.Menus[0].Image)
	}
	if snapshot.Menus[1].Image != "" {
		t.Fatalf("空菜单图片被改写: %q", snapshot.Menus[1].Image)
	}
}

func makeArchive(t *testing.T, files map[string][]byte, declared map[string][]byte) []byte {
	t.Helper()
	hashes := map[string]string{}
	for name, raw := range declared {
		sum := sha256.Sum256(raw)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	manifest, _ := json.Marshal(profileManifest{SchemaVersion: 1, AppVersion: "0.0.1", Name: "导入", CreatedAt: time.Now(), Files: hashes})
	files["manifest.json"] = manifest
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, raw := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write(raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestProfileImportRejectsUnlistedAndTraversalFiles(t *testing.T) {
	configRaw, _ := json.Marshal(archiveTestSnapshot())
	unlisted := makeArchive(t, map[string][]byte{"config.json": configRaw, "assets/extra.png": {1, 2, 3}}, map[string][]byte{"config.json": configRaw})
	if _, _, _, err := parseProfileArchive(unlisted); err == nil || !strings.Contains(err.Error(), "未在清单") {
		t.Fatalf("expected unlisted rejection, got %v", err)
	}
	traversal := makeArchive(t, map[string][]byte{"config.json": configRaw, "assets/../escape.png": {1}}, map[string][]byte{"config.json": configRaw, "assets/../escape.png": {1}})
	if _, _, _, err := parseProfileArchive(traversal); err == nil || !strings.Contains(err.Error(), "不安全路径") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
}

func TestActivateProfileRejectsIncompleteSnapshot(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	modelsToMigrate := []any{
		&models.SystemConfig{}, &models.CommandConfig{}, &models.PetSpeciesConfig{}, &models.PetEvolutionRuleConfig{},
		&models.PetEvolutionCostConfig{}, &models.PetSkillUnlockConfig{}, &models.AdventureLevelConfig{}, &models.ItemConfig{}, &models.ShopItemConfig{},
		&models.CheckinRewardConfig{}, &models.WorkSettingConfig{}, &models.MenuConfig{}, &models.ImageConfig{}, &models.LiveEventConfig{},
		&models.RewardTrackConfig{}, &models.GrowthRoleConfig{}, &models.GrowthStanceConfig{}, &models.PersonalityRuleConfig{},
		&models.CodexCatalogConfig{}, &models.ExpeditionTemplateConfig{}, &models.ChanceGameConfig{}, &models.ChanceRewardConfig{},
		&models.AdminConfigState{}, &models.ConfigProfile{}, &models.PetProfile{}, &models.GlobalInventoryItem{}, &models.EventProgress{},
		&models.ActivityRun{}, &models.ExpeditionRun{}, &models.TradeOffer{}, &models.FishingRun{},
		&models.AdventureMapConfig{}, &models.AdventureZoneConfig{}, &models.AdventureZonePrerequisiteConfig{},
		&models.AdventureObjectiveConfig{}, &models.AdventureExplorationStageConfig{}, &models.AdventureStoryEventConfig{}, &models.AdventureStoryEventChoiceConfig{},
		&models.AdventureMonsterConfig{}, &models.AdventureSkillConfig{},
		&models.AdventureMonsterSkillConfig{}, &models.AdventureEncounterConfig{}, &models.AdventureEncounterEffectConfig{}, &models.AdventureLootPoolConfig{},
		&models.AdventureLootEntryConfig{}, &models.CurrencyConfig{},
		&models.AdventureShopItemConfig{}, &models.AdventureExpeditionConfig{}, &models.AdventureBossConfig{},
		&models.AdventureBossRewardTierConfig{}, &models.EquipmentTemplateConfig{}, &models.EquipmentAffixConfig{},
		&models.EquipmentRecipeConfig{}, &models.EquipmentRecipeMaterialConfig{}, &models.LiveEventChoiceConfig{},
		&models.LiveEventExpeditionSourceConfig{},
		&models.PlayerAdventureNodeProgress{}, &models.PlayerAdventureEventState{},
		&models.AdventureExplorationSession{}, &models.AdventureCombatSession{}, &models.AdventureExpeditionRun{},
		&models.PlayerEquipment{}, &models.AdventureBossInstance{},
	}
	if err = db.AutoMigrate(modelsToMigrate...); err != nil {
		t.Fatal(err)
	}
	current, err := appconfig.CreateProfileFromSnapshot(db, "当前方案", "", "user", false, archiveTestSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Transaction(func(tx *gorm.DB) error { return appconfig.SetActiveProfile(tx, current.ID, false) }); err != nil {
		t.Fatal(err)
	}
	incomplete, err := appconfig.CreateProfileFromSnapshot(db, "残缺方案", "", "user", false, archiveTestSnapshot())
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterProfileRoutes(router.Group("/api/admin"), db)
	request, _ := http.NewRequest(http.MethodPost, "/api/admin/config/profiles/"+incomplete.ID+"/activate", bytes.NewReader([]byte(`{}`)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err = json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析响应失败: %v %s", err, recorder.Body.String())
	}
	if recorder.Code != http.StatusConflict || response.Code != 4093 || !strings.Contains(response.Msg, "需要 3 张永久地图") {
		t.Fatalf("残缺方案激活应返回 4093，实际 HTTP %d code=%d msg=%s", recorder.Code, response.Code, response.Msg)
	}
	status, err := appconfig.GetConfigStatus(db)
	if err != nil {
		t.Fatal(err)
	}
	if status.ActiveProfileID != current.ID {
		t.Fatalf("激活失败后当前方案被改成了 %s", status.ActiveProfileID)
	}
}
