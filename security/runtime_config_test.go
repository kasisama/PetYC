package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRuntimeConfigImportsEnvironmentOnlyWhenFileIsCreated(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("PORT", "9123")
	t.Setenv("LISTEN_ADDRESS", "127.0.0.1")
	t.Setenv("QQPET_WS_TOKEN", "first-onebot-token")
	t.Setenv("QQBOT_APP_ID", "first-app-id")
	t.Setenv("QQBOT_APP_SECRET", "first-app-secret")
	t.Setenv("QQBOT_MARKDOWN_ENABLED", "true")

	first, err := LoadRuntimeConfig()
	if err != nil {
		t.Fatalf("LoadRuntimeConfig() error = %v", err)
	}
	if first.Port != 9123 || first.ListenAddress != "127.0.0.1" || first.OneBotToken != "first-onebot-token" {
		t.Fatalf("first runtime config = %#v", first)
	}
	if first.QQOfficial.AppID != "first-app-id" || first.QQOfficial.AppSecret != "first-app-secret" || !first.QQOfficial.MarkdownEnabled {
		t.Fatalf("first QQ config = %#v", first.QQOfficial)
	}
	if !first.QQOfficial.GroupEventsEnabled || !first.QQOfficial.GuildEventsEnabled {
		t.Fatalf("required QQ event subscriptions should default to enabled: %#v", first.QQOfficial)
	}

	t.Setenv("PORT", "9234")
	t.Setenv("QQPET_WS_TOKEN", "second-onebot-token")
	t.Setenv("QQBOT_APP_ID", "second-app-id")
	t.Setenv("QQBOT_APP_SECRET", "second-app-secret")

	second, err := LoadRuntimeConfig()
	if err != nil {
		t.Fatalf("second LoadRuntimeConfig() error = %v", err)
	}
	if second != first {
		t.Fatalf("environment was imported again: first = %#v, second = %#v", first, second)
	}
}

func TestLoadRuntimeConfigUpgradesLegacyFileDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QQPET_DATA_DIR", dir)
	legacy := `{"port":8080,"onebot_token":"legacy-token","qq_official":{}}`
	if err := os.WriteFile(filepath.Join(dir, runtimeConfigFileName), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}

	config, err := LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.ListenAddress != "127.0.0.1" || !config.QQOfficial.GroupEventsEnabled || !config.QQOfficial.GuildEventsEnabled {
		t.Fatalf("legacy defaults were not upgraded: %#v", config)
	}
}

func TestSaveRuntimeConfigAtomicallyReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QQPET_DATA_DIR", dir)
	t.Setenv("PORT", "")
	t.Setenv("QQPET_WS_TOKEN", "")

	config, err := LoadRuntimeConfig()
	if err != nil {
		t.Fatalf("LoadRuntimeConfig() error = %v", err)
	}
	config.Port = 8234
	config.OneBotToken = "updated-token"
	config.QQOfficial.AppID = "app-id"
	config.QQOfficial.AppSecret = "app-secret"
	if err := SaveRuntimeConfig(config); err != nil {
		t.Fatalf("SaveRuntimeConfig() error = %v", err)
	}

	loaded, err := LoadRuntimeConfig()
	if err != nil {
		t.Fatalf("LoadRuntimeConfig() after save error = %v", err)
	}
	if loaded != config {
		t.Fatalf("loaded config = %#v, want %#v", loaded, config)
	}
	if _, err := os.Stat(filepath.Join(dir, runtimeConfigFileName)); err != nil {
		t.Fatalf("runtime config file was not persisted: %v", err)
	}
	tempFiles, err := filepath.Glob(filepath.Join(dir, ".runtime-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tempFiles) != 0 {
		t.Fatalf("temporary files left behind: %v", tempFiles)
	}
}

func TestSaveRuntimeConfigRejectsInvalidPortWithoutReplacingFile(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("PORT", "8080")
	t.Setenv("QQPET_WS_TOKEN", "stable-token")

	original, err := LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	invalid := original
	invalid.Port = 70000
	if err := SaveRuntimeConfig(invalid); err == nil {
		t.Fatal("SaveRuntimeConfig() accepted an invalid port")
	}
	loaded, err := LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded != original {
		t.Fatalf("invalid save replaced config: got %#v, want %#v", loaded, original)
	}
}

func TestSaveRuntimeConfigSynchronizesOneBotCredential(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("PORT", "8080")
	t.Setenv("QQPET_WS_TOKEN", "")

	config, err := LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	config.OneBotToken = "api-updated-token"
	if err := SaveRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}

	credentials, err := LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if credentials.WebSocketToken != "api-updated-token" {
		t.Fatalf("WebSocketToken = %q, want API-updated token", credentials.WebSocketToken)
	}
}
