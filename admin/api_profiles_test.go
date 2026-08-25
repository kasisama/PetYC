package admin

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

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
