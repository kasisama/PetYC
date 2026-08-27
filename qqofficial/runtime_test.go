package qqofficial

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"qq-pet-saas/security"
)

func TestLoadConfigUsesPersistedRuntimeConfigAfterInitialEnvironmentImport(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQBOT_APP_ID", "persisted-app")
	t.Setenv("QQBOT_APP_SECRET", "persisted-secret")
	t.Setenv("QQBOT_MARKDOWN_ENABLED", "true")

	first, enabled, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !enabled || first.AppID != "persisted-app" || first.Secret != "persisted-secret" || !first.MarkdownEnabled {
		t.Fatalf("LoadConfig() = %#v, %v", first, enabled)
	}

	t.Setenv("QQBOT_APP_ID", "changed-app")
	t.Setenv("QQBOT_APP_SECRET", "changed-secret")
	second, enabled, err := LoadConfig()
	if err != nil {
		t.Fatalf("second LoadConfig() error = %v", err)
	}
	if !enabled || second != first {
		t.Fatalf("environment overrode persisted config: first = %#v, second = %#v", first, second)
	}
}

func TestLoadConfigUsesConfiguredGroupAndGuildIntents(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQBOT_APP_ID", "intent-app")
	t.Setenv("QQBOT_APP_SECRET", "intent-secret")
	stored, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	stored.QQOfficial.GroupEventsEnabled = true
	stored.QQOfficial.GuildEventsEnabled = false
	if err = security.SaveRuntimeConfig(stored); err != nil {
		t.Fatal(err)
	}

	config, enabled, err := LoadConfig()
	if err != nil || !enabled {
		t.Fatalf("LoadConfig() failed: enabled=%v err=%v", enabled, err)
	}
	if config.Intents&IntentsGroupAndC2C == 0 || config.Intents&IntentsPublicGuildMessages != 0 {
		t.Fatalf("unexpected configured intents: %d", config.Intents)
	}
}

func TestApplyDefaultConfigStopsExistingRuntimeWhenQQIsDisabled(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQBOT_APP_ID", "")
	t.Setenv("QQBOT_APP_SECRET", "")
	runtimeConfig, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	runtimeConfig.QQOfficial = security.QQOfficialRuntimeConfig{}
	if err := security.SaveRuntimeConfig(runtimeConfig); err != nil {
		t.Fatal(err)
	}

	cancelled := false
	defaultRuntime.Lock()
	previousStatus, previousClient, previousCancel := defaultRuntime.status, defaultRuntime.client, defaultRuntime.cancel
	previousConfig, previousDeduper := defaultRuntime.config, defaultRuntime.deduper
	previousRunning, previousGeneration := defaultRuntime.running, defaultRuntime.generation
	_, cancel := context.WithCancel(context.Background())
	defaultRuntime.status = newRuntimeStatus(Config{AppID: "old-app", Secret: "old-secret", Intents: IntentsGroupAndC2C})
	defaultRuntime.cancel = func() { cancelled = true; cancel() }
	defaultRuntime.config = Config{AppID: "old-app", Secret: "old-secret", Intents: IntentsGroupAndC2C}
	defaultRuntime.running = true
	defaultRuntime.Unlock()
	t.Cleanup(func() {
		defaultRuntime.Lock()
		defaultRuntime.status, defaultRuntime.client, defaultRuntime.cancel = previousStatus, previousClient, previousCancel
		defaultRuntime.config, defaultRuntime.deduper = previousConfig, previousDeduper
		defaultRuntime.running, defaultRuntime.generation = previousRunning, previousGeneration
		defaultRuntime.Unlock()
	})

	if err := ApplyDefaultConfig(); err != nil {
		t.Fatalf("ApplyDefaultConfig() error = %v", err)
	}
	if !cancelled {
		t.Fatal("existing QQ runtime was not stopped")
	}
	snapshot := DefaultRuntimeSnapshot()
	if snapshot.Configured || snapshot.SessionState != "not_started" {
		t.Fatalf("snapshot after disable = %#v", snapshot)
	}
}

func TestApplyDefaultConfigSkipsRestartWhenEffectiveConfigIsUnchanged(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQBOT_APP_ID", "same-app")
	t.Setenv("QQBOT_APP_SECRET", "same-secret")
	config, enabled, err := LoadConfig()
	if err != nil || !enabled {
		t.Fatalf("LoadConfig() failed: enabled=%v err=%v", enabled, err)
	}

	cancelled := false
	defaultRuntime.Lock()
	previousStatus, previousClient, previousCancel := defaultRuntime.status, defaultRuntime.client, defaultRuntime.cancel
	previousConfig, previousDeduper := defaultRuntime.config, defaultRuntime.deduper
	previousRunning, previousGeneration := defaultRuntime.running, defaultRuntime.generation
	defaultRuntime.status = newRuntimeStatus(config)
	defaultRuntime.cancel = func() { cancelled = true }
	defaultRuntime.config = config
	defaultRuntime.running = true
	defaultRuntime.Unlock()
	t.Cleanup(func() {
		defaultRuntime.Lock()
		defaultRuntime.status, defaultRuntime.client, defaultRuntime.cancel = previousStatus, previousClient, previousCancel
		defaultRuntime.config, defaultRuntime.deduper = previousConfig, previousDeduper
		defaultRuntime.running, defaultRuntime.generation = previousRunning, previousGeneration
		defaultRuntime.Unlock()
	})

	if err = ApplyDefaultConfig(); err != nil {
		t.Fatalf("ApplyDefaultConfig() error = %v", err)
	}
	if cancelled {
		t.Fatal("unchanged effective config restarted the QQ runtime")
	}
}

func TestApplyDefaultConfigRestartsStoppedRuntimeWithSameConfig(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQBOT_APP_ID", "stopped-app")
	t.Setenv("QQBOT_APP_SECRET", "stopped-secret")
	config, enabled, err := LoadConfig()
	if err != nil || !enabled {
		t.Fatalf("LoadConfig() failed: enabled=%v err=%v", enabled, err)
	}

	oldCancelCalled := false
	defaultRuntime.Lock()
	previousStatus, previousClient, previousCancel := defaultRuntime.status, defaultRuntime.client, defaultRuntime.cancel
	previousConfig, previousDeduper := defaultRuntime.config, defaultRuntime.deduper
	previousRunning, previousGeneration := defaultRuntime.running, defaultRuntime.generation
	defaultRuntime.status = newRuntimeStatus(config)
	defaultRuntime.cancel = func() { oldCancelCalled = true }
	defaultRuntime.config = config
	defaultRuntime.running = false
	defaultRuntime.Unlock()
	t.Cleanup(func() {
		defaultRuntime.Lock()
		currentCancel := defaultRuntime.cancel
		defaultRuntime.status, defaultRuntime.client, defaultRuntime.cancel = previousStatus, previousClient, previousCancel
		defaultRuntime.config, defaultRuntime.deduper = previousConfig, previousDeduper
		defaultRuntime.running, defaultRuntime.generation = previousRunning, previousGeneration
		defaultRuntime.Unlock()
		if currentCancel != nil {
			currentCancel()
		}
	})

	if err = ApplyDefaultConfig(); err != nil {
		t.Fatalf("ApplyDefaultConfig() error = %v", err)
	}
	if !oldCancelCalled {
		t.Fatal("stopped runtime with unchanged config was not replaced")
	}
}

func TestConfiguredRuntimeReusesDeduplicatorAcrossReconnects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"access_token": "access", "expires_in": 7200})
		case "/gateway/bot":
			_ = json.NewEncoder(writer).Encode(GatewayInfo{URL: "ws://127.0.0.1:1", Shards: 1})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	defaultRuntime.Lock()
	previousStatus, previousClient, previousCancel := defaultRuntime.status, defaultRuntime.client, defaultRuntime.cancel
	previousConfig, previousDeduper := defaultRuntime.config, defaultRuntime.deduper
	previousRunning, previousGeneration := defaultRuntime.running, defaultRuntime.generation
	defaultRuntime.status, defaultRuntime.client, defaultRuntime.cancel = nil, nil, nil
	defaultRuntime.config, defaultRuntime.deduper = Config{}, nil
	defaultRuntime.running, defaultRuntime.generation = false, 0
	defaultRuntime.Unlock()
	t.Cleanup(func() {
		defaultRuntime.Lock()
		cancel := defaultRuntime.cancel
		defaultRuntime.status, defaultRuntime.client, defaultRuntime.cancel = previousStatus, previousClient, previousCancel
		defaultRuntime.config, defaultRuntime.deduper = previousConfig, previousDeduper
		defaultRuntime.running, defaultRuntime.generation = previousRunning, previousGeneration
		defaultRuntime.Unlock()
		if cancel != nil {
			cancel()
		}
	})

	config := Config{AppID: "app", Secret: "secret", APIBase: server.URL, TokenURL: server.URL + "/token", Intents: IntentsGroupAndC2C}
	first, err := startConfigured(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	second, err := startConfigured(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if first.Deduper == nil || first.Deduper != second.Deduper {
		t.Fatal("runtime reconnect replaced the process-level message deduplicator")
	}
}

func TestRuntimeStatusSnapshotIsSafeDuringConcurrentUpdates(t *testing.T) {
	status := newRuntimeStatus(Config{AppID: "123456789", Secret: "never-return-me", Intents: IntentsGroupAndC2C | IntentsPublicGuildMessages})
	var waitGroup sync.WaitGroup
	for index := 0; index < 50; index++ {
		waitGroup.Add(2)
		go func() {
			defer waitGroup.Done()
			status.update(func(snapshot *RuntimeSnapshot) { snapshot.QueueDepth++ })
		}()
		go func() { defer waitGroup.Done(); _ = status.Snapshot() }()
	}
	waitGroup.Wait()
	snapshot := status.Snapshot()
	if snapshot.QueueDepth != 50 {
		t.Fatalf("expected queue depth 50, got %d", snapshot.QueueDepth)
	}
	if snapshot.MaskedAppID != "***6789" {
		t.Fatalf("unexpected masked app id: %s", snapshot.MaskedAppID)
	}
}
