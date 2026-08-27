package updater

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type helperLaunchConfig struct {
	Target          string
	Source          string
	Database        string
	WorkingDir      string
	HealthURL       string
	ExpectedVersion string
	OriginalArgs    []string
}

type helperConfig struct {
	ParentPID       int      `json:"parentPid"`
	Target          string   `json:"target"`
	Source          string   `json:"source"`
	Backup          string   `json:"backup"`
	Database        string   `json:"database"`
	BackupDirectory string   `json:"backupDirectory"`
	WorkingDir      string   `json:"workingDir"`
	HealthURL       string   `json:"healthUrl"`
	ExpectedVersion string   `json:"expectedVersion"`
	OriginalArgs    []string `json:"originalArgs"`
	LogPath         string   `json:"logPath"`
}

func launchHelper(config helperLaunchConfig) error {
	timestamp := time.Now().UTC().Format("20060102-150405")
	directory := filepath.Dir(config.Target)
	helperSuffix := fmt.Sprintf(".update-helper-%d", os.Getpid())
	if strings.EqualFold(filepath.Ext(config.Target), ".exe") {
		helperSuffix += ".exe"
	}
	helperPath := config.Target + helperSuffix
	if err := copyFile(config.Target, helperPath, 0o700); err != nil {
		return fmt.Errorf("准备更新辅助程序: %w", err)
	}

	backupDirectory := filepath.Join(directory, ".petyc-backups", timestamp)
	if err := os.MkdirAll(backupDirectory, 0o700); err != nil {
		_ = os.Remove(helperPath)
		return fmt.Errorf("创建更新备份目录: %w", err)
	}
	encoded, err := encodeHelperConfig(helperConfig{
		ParentPID:       os.Getpid(),
		Target:          config.Target,
		Source:          config.Source,
		Backup:          filepath.Join(backupDirectory, filepath.Base(config.Target)),
		Database:        config.Database,
		BackupDirectory: backupDirectory,
		WorkingDir:      config.WorkingDir,
		HealthURL:       config.HealthURL,
		ExpectedVersion: config.ExpectedVersion,
		OriginalArgs:    config.OriginalArgs,
		LogPath:         filepath.Join(backupDirectory, "update.log"),
	})
	if err != nil {
		_ = os.Remove(helperPath)
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(backupDirectory, "helper-launch.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		_ = os.Remove(helperPath)
		return fmt.Errorf("创建更新日志: %w", err)
	}
	command := exec.Command(helperPath, "--apply-update", encoded)
	command.Dir = config.WorkingDir
	command.Stdin = nil
	command.Stdout = logFile
	command.Stderr = logFile
	configureDetached(command)
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		_ = os.Remove(helperPath)
		return fmt.Errorf("启动更新辅助程序: %w", err)
	}
	_ = command.Process.Release()
	_ = logFile.Close()
	return nil
}

func encodeHelperConfig(config helperConfig) (string, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("编码更新参数: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeHelperConfig(value string) (helperConfig, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return helperConfig{}, fmt.Errorf("解码更新参数: %w", err)
	}
	var config helperConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return helperConfig{}, fmt.Errorf("解析更新参数: %w", err)
	}
	if config.ParentPID <= 0 || config.Target == "" || config.Source == "" || config.Backup == "" || config.WorkingDir == "" || config.HealthURL == "" || config.ExpectedVersion == "" {
		return helperConfig{}, errors.New("更新参数不完整")
	}
	return config, nil
}

// MaybeRunHelper handles the private --apply-update process mode before the
// application opens its database or starts any network listeners.
func MaybeRunHelper(args []string) (bool, error) {
	if len(args) == 0 || args[0] != "--apply-update" {
		return false, nil
	}
	if len(args) != 2 {
		return true, errors.New("更新辅助模式参数无效")
	}
	config, err := decodeHelperConfig(args[1])
	if err != nil {
		return true, err
	}
	return true, applyUpdate(config)
}

func applyUpdate(config helperConfig) error {
	logFile, err := os.OpenFile(config.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	logLine := func(format string, values ...any) {
		_, _ = fmt.Fprintf(logFile, time.Now().UTC().Format(time.RFC3339)+" "+format+"\n", values...)
		_ = logFile.Sync()
	}

	logLine("等待旧进程退出 pid=%d", config.ParentPID)
	if err := waitForProcessExit(config.ParentPID, 45*time.Second); err != nil {
		return fmt.Errorf("等待旧进程退出: %w", err)
	}
	// Give Windows a short grace period to release executable and SQLite handles.
	time.Sleep(300 * time.Millisecond)

	if err := backupDatabase(config.Database, config.BackupDirectory); err != nil {
		return fmt.Errorf("备份数据库: %w", err)
	}
	logLine("数据库备份完成")
	if err := swapExecutable(config.Target, config.Source, config.Backup); err != nil {
		return err
	}
	logLine("程序文件替换完成")

	process, err := startApplication(config.Target, config.OriginalArgs, config.WorkingDir, logFile)
	if err != nil {
		_ = rollbackFiles(config)
		return fmt.Errorf("启动新版本: %w", err)
	}
	logLine("新版本已启动 pid=%d", process.Pid)
	if err := waitForHealth(config.HealthURL, config.ExpectedVersion, 60*time.Second); err == nil {
		logLine("新版本健康检查通过 version=%s", config.ExpectedVersion)
		_ = process.Release()
		return nil
	} else {
		logLine("新版本健康检查失败: %v，开始回滚", err)
		_ = process.Kill()
		_, _ = process.Wait()
		if rollbackErr := rollbackFiles(config); rollbackErr != nil {
			return fmt.Errorf("健康检查失败: %v；回滚失败: %w", err, rollbackErr)
		}
		oldProcess, startErr := startApplication(config.Target, config.OriginalArgs, config.WorkingDir, logFile)
		if startErr != nil {
			return fmt.Errorf("健康检查失败: %v；旧版本恢复后启动失败: %w", err, startErr)
		}
		_ = oldProcess.Release()
		logLine("旧版本已恢复并重新启动")
		return fmt.Errorf("新版本健康检查失败，已恢复旧版本: %w", err)
	}
}

func swapExecutable(target, source, backup string) error {
	if filepath.Dir(target) != filepath.Dir(source) {
		return errors.New("更新包与目标程序必须位于同一目录")
	}
	if err := renameWithRetry(target, backup, 15*time.Second); err != nil {
		return fmt.Errorf("备份旧程序: %w", err)
	}
	if err := renameWithRetry(source, target, 15*time.Second); err != nil {
		_ = renameWithRetry(backup, target, 5*time.Second)
		return fmt.Errorf("安装新程序: %w", err)
	}
	if filepath.Ext(target) != ".exe" {
		if err := os.Chmod(target, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func renameWithRetry(source, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastError error
	for {
		lastError = os.Rename(source, target)
		if lastError == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return lastError
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func backupDatabase(database, backupDirectory string) error {
	mainBackedUp := false
	for _, suffix := range []string{"", "-wal", "-shm"} {
		source := database + suffix
		if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := copyFile(source, filepath.Join(backupDirectory, filepath.Base(source)), 0o600); err != nil {
			return err
		}
		if suffix == "" {
			mainBackedUp = true
		}
	}
	if !mainBackedUp {
		return errors.New("主数据库文件不存在，拒绝继续更新")
	}
	return nil
}

func restoreDatabase(database, backupDirectory string) error {
	mainBackup := filepath.Join(backupDirectory, filepath.Base(database))
	if _, err := os.Stat(mainBackup); err != nil {
		return fmt.Errorf("主数据库备份不可用: %w", err)
	}

	prepared := make(map[string]string)
	defer func() {
		for _, temporary := range prepared {
			_ = os.Remove(temporary)
		}
	}()
	for _, suffix := range []string{"", "-wal", "-shm"} {
		destination := database + suffix
		backup := filepath.Join(backupDirectory, filepath.Base(destination))
		if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		temporary := destination + ".restore"
		_ = os.Remove(temporary)
		if err := copyFile(backup, temporary, 0o600); err != nil {
			return err
		}
		prepared[suffix] = temporary
	}
	// The process is stopped at this point. Replace sidecars first and the main
	// database last, after every available backup has been copied successfully.
	for _, suffix := range []string{"-wal", "-shm", ""} {
		destination := database + suffix
		temporary, exists := prepared[suffix]
		if !exists {
			if suffix != "" {
				_ = os.Remove(destination)
			}
			continue
		}
		_ = os.Remove(destination)
		if err := os.Rename(temporary, destination); err != nil {
			return err
		}
		delete(prepared, suffix)
	}
	return nil
}

func rollbackFiles(config helperConfig) error {
	_ = os.Remove(config.Target)
	if err := renameWithRetry(config.Backup, config.Target, 10*time.Second); err != nil {
		return fmt.Errorf("恢复旧程序: %w", err)
	}
	if err := restoreDatabase(config.Database, config.BackupDirectory); err != nil {
		return fmt.Errorf("恢复数据库: %w", err)
	}
	return nil
}

func copyFile(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func startApplication(target string, args []string, workingDir string, logFile *os.File) (*os.Process, error) {
	command := exec.Command(target, args...)
	command.Dir = workingDir
	command.Stdin = nil
	command.Stdout = logFile
	command.Stderr = logFile
	configureDetached(command)
	if err := command.Start(); err != nil {
		return nil, err
	}
	return command.Process, nil
}

func waitForHealth(address, expectedVersion string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	var lastError error
	for time.Now().Before(deadline) {
		response, err := client.Get(address)
		if err == nil {
			var payload struct {
				Status  string `json:"status"`
				Version string `json:"version"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&payload)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK && decodeErr == nil && payload.Status == "ok" && payload.Version == expectedVersion {
				return nil
			}
			lastError = fmt.Errorf("健康响应无效: HTTP %d, status=%s, version=%s", response.StatusCode, payload.Status, payload.Version)
		} else {
			lastError = err
		}
		time.Sleep(time.Second)
	}
	if lastError == nil {
		lastError = errors.New("健康检查超时")
	}
	return lastError
}

// CleanupStaleHelpers removes helper copies left by completed updates. It is
// best-effort because Windows may still hold a just-exited helper briefly.
func CleanupStaleHelpers(executable string) {
	matches, _ := filepath.Glob(executable + ".update-helper-*")
	for _, match := range matches {
		_ = os.Remove(match)
	}
	_ = os.Remove(executable + ".new.download")
}
