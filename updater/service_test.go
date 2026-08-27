package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServiceCheckUsesSignedManifestAndPlatform(t *testing.T) {
	temp := t.TempDir()
	executable := filepath.Join(temp, "petyc.exe")
	if err := os.WriteFile(executable, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := validManifest()
	manifest.Version = "1.1.0"
	raw, signature, publicKey := signedManifest(t, manifest)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if filepath.Ext(r.URL.Path) == ".sig" {
			_, _ = w.Write(signature)
			return
		}
		_, _ = w.Write(raw)
	}))
	defer server.Close()
	service := NewService(Config{
		CurrentVersion: "1.0.0", ManifestURL: server.URL + "/manifest", SignatureURL: server.URL + "/manifest.sig",
		PublicKey: publicKey, ExecutablePath: func() (string, error) { return executable, nil },
		RuntimeOS: "windows", RuntimeArch: "amd64", Environment: func(string) string { return "" }, FileExists: func(string) bool { return false },
	})
	info, err := service.Check(t.Context(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || !info.CanAutoUpdate || info.InstallMode != "portable" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestServiceCheckRejectsInvalidSignature(t *testing.T) {
	manifest := validManifest()
	raw, _, publicKey := signedManifest(t, manifest)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if filepath.Ext(r.URL.Path) == ".sig" {
			_, _ = w.Write([]byte("invalid"))
			return
		}
		_, _ = w.Write(raw)
	}))
	defer server.Close()
	service := NewService(Config{CurrentVersion: "1.0.0", ManifestURL: server.URL, SignatureURL: server.URL + ".sig", PublicKey: publicKey})
	if _, err := service.Check(t.Context(), true); err == nil {
		t.Fatal("invalid signature should fail")
	}
}

func TestDownloadRejectsChecksumMismatch(t *testing.T) {
	payload := []byte("new binary")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload) }))
	defer server.Close()
	service := NewService(Config{CurrentVersion: "1.0.0"})
	destination := filepath.Join(t.TempDir(), "petyc.new")
	err := service.download(t.Context(), Artifact{URL: server.URL, SHA256: hex.EncodeToString(make([]byte, sha256.Size)), Size: int64(len(payload))}, destination)
	if err == nil {
		t.Fatal("checksum mismatch should fail")
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("destination should not exist: %v", statErr)
	}
}

func TestSwapAndRollbackFiles(t *testing.T) {
	temp := t.TempDir()
	target := filepath.Join(temp, "petyc")
	source := target + ".new"
	backupDir := filepath.Join(temp, "backup")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(backupDir, "petyc")
	database := filepath.Join(temp, "pet_game.db")
	if err := os.WriteFile(target, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("new"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(database, []byte("db-old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := backupDatabase(database, backupDir); err != nil {
		t.Fatal(err)
	}
	if err := swapExecutable(target, source, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(database, []byte("db-new"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := helperConfig{Target: target, Backup: backup, Database: database, BackupDirectory: backupDir}
	if err := rollbackFiles(config); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, target, "old")
	assertFileContent(t, database, "db-old")
}

func TestRestoreDatabaseRefusesMissingMainBackupWithoutDeletingCurrentData(t *testing.T) {
	temp := t.TempDir()
	database := filepath.Join(temp, "pet_game.db")
	backupDir := filepath.Join(temp, "backup")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(database, []byte("current-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restoreDatabase(database, backupDir); err == nil {
		t.Fatal("missing main backup should fail")
	}
	assertFileContent(t, database, "current-data")
}

func TestBackupDatabaseRequiresMainDatabase(t *testing.T) {
	temp := t.TempDir()
	if err := backupDatabase(filepath.Join(temp, "missing.db"), temp); err == nil {
		t.Fatal("missing main database should block update")
	}
}

func TestAutoUpdateCapabilityFallsBackForManagedEnvironments(t *testing.T) {
	temp := t.TempDir()
	executable := filepath.Join(temp, "petyc")
	if err := os.WriteFile(executable, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		environment func(string) string
		fileExists  func(string) bool
		wantReason  string
	}{
		{
			name:        "docker",
			environment: func(string) string { return "" },
			fileExists:  func(path string) bool { return path == "/.dockerenv" },
			wantReason:  "Docker 环境请通过镜像更新",
		},
		{
			name: "systemd",
			environment: func(key string) string {
				if key == "INVOCATION_ID" {
					return "unit-id"
				}
				return ""
			},
			fileExists: func(string) bool { return false },
			wantReason: "systemd 服务请通过部署命令更新",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(Config{
				RuntimeOS: "linux", RuntimeArch: "amd64",
				ExecutablePath: func() (string, error) { return executable, nil },
				Environment:    test.environment, FileExists: test.fileExists,
			})
			canAutoUpdate, reason := service.autoUpdateCapability()
			if canAutoUpdate || reason != test.wantReason {
				t.Fatalf("capability = (%v, %q), want (false, %q)", canAutoUpdate, reason, test.wantReason)
			}
		})
	}
}

func TestStartInstallRejectsConcurrentTask(t *testing.T) {
	service := NewService(Config{CurrentVersion: "1.0.0"})
	service.mu.Lock()
	service.installing = true
	service.mu.Unlock()

	if err := service.StartInstall(); err == nil {
		t.Fatal("concurrent update should be rejected")
	}
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != expected {
		t.Fatalf("%s = %q, want %q", path, content, expected)
	}
}
