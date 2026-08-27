package updater

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyUpdateEndToEnd(t *testing.T) {
	if os.Getenv("PETYC_UPDATE_TARGET") == "1" {
		t.Skip("target process is handled by TestUpdateTargetProcess")
	}
	temp := t.TempDir()
	currentTestBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	extension := filepath.Ext(currentTestBinary)
	target := filepath.Join(temp, "petyc"+extension)
	source := target + ".new"
	backupDir := filepath.Join(temp, "backup")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(backupDir, filepath.Base(target))
	if err := copyFile(currentTestBinary, target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(currentTestBinary, source, 0o700); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(temp, "pet_game.db")
	if err := os.WriteFile(database, []byte("database-before-update"), 0o600); err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	t.Setenv("PETYC_UPDATE_TARGET", "1")
	t.Setenv("PETYC_UPDATE_TARGET_PORT", fmt.Sprint(port))
	t.Setenv("PETYC_UPDATE_TARGET_VERSION", "9.9.9")

	parent := exec.Command(currentTestBinary, "-test.run=TestShortLivedParent")
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	parentPID := parent.Process.Pid
	_ = parent.Wait()

	config := helperConfig{
		ParentPID: parentPID, Target: target, Source: source, Backup: backup,
		Database: database, BackupDirectory: backupDir, WorkingDir: temp,
		HealthURL: fmt.Sprintf("http://127.0.0.1:%d/healthz", port), ExpectedVersion: "9.9.9",
		OriginalArgs: []string{"-test.run=TestUpdateTargetProcess"}, LogPath: filepath.Join(backupDir, "update.log"),
	}
	if err := applyUpdate(config); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, filepath.Join(backupDir, "pet_game.db"), "database-before-update")
	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/shutdown", port))
	if err == nil {
		_ = response.Body.Close()
	}
	time.Sleep(500 * time.Millisecond)
}

func TestShortLivedParent(t *testing.T) {}

func TestUpdateTargetProcess(t *testing.T) {
	if os.Getenv("PETYC_UPDATE_TARGET") != "1" {
		return
	}
	port := os.Getenv("PETYC_UPDATE_TARGET_PORT")
	version := os.Getenv("PETYC_UPDATE_TARGET_VERSION")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"ok","version":%q}`, version)
	})
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("bye"))
		go func() { time.Sleep(50 * time.Millisecond); os.Exit(0) }()
	})
	if err := http.ListenAndServe("127.0.0.1:"+port, mux); err != nil {
		t.Fatal(err)
	}
}
