//go:build windows

package updater

import (
	"errors"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

var makeExecutable = func(string) error { return nil }

func configureDetached(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
		HideWindow:    true,
	}
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	milliseconds := uint32(timeout / time.Millisecond)
	result, err := windows.WaitForSingleObject(handle, milliseconds)
	if err != nil {
		return err
	}
	if result != windows.WAIT_OBJECT_0 {
		return errors.New("旧进程未在期限内退出")
	}
	return nil
}
