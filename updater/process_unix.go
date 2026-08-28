//go:build !windows

package updater

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

var makeExecutable = func(path string) error {
	return os.Chmod(path, 0o755)
}

func configureDetached(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("旧进程未在期限内退出")
}
