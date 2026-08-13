package qqofficial

import (
	"context"
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
	previousStatus, previousCancel := defaultRuntime.status, defaultRuntime.cancel
	_, cancel := context.WithCancel(context.Background())
	defaultRuntime.status = newRuntimeStatus(Config{AppID: "old-app", Secret: "old-secret", Intents: IntentsGroupAndC2C})
	defaultRuntime.cancel = func() { cancelled = true; cancel() }
	defaultRuntime.Unlock()
	t.Cleanup(func() {
		defaultRuntime.Lock()
		defaultRuntime.status, defaultRuntime.cancel = previousStatus, previousCancel
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
