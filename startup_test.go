package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"qq-pet-saas/security"
)

func TestShouldRunLinuxTerminalSetup(t *testing.T) {
	t.Setenv("QQPET_WEB_SETUP", "")
	if !shouldRunLinuxTerminalSetup("linux", false) {
		t.Fatal("fresh Linux install should require terminal setup by default")
	}
	if shouldRunLinuxTerminalSetup("linux", true) {
		t.Fatal("completed Linux install must not run terminal setup")
	}
	if shouldRunLinuxTerminalSetup("windows", false) {
		t.Fatal("non-Linux install must not run Linux terminal setup")
	}
}

func TestShouldRunLinuxTerminalSetupAllowsOptInWebSetup(t *testing.T) {
	t.Setenv("QQPET_WEB_SETUP", "1")
	if shouldRunLinuxTerminalSetup("linux", false) {
		t.Fatal("web setup opt-in should bypass Linux terminal setup")
	}
}

func TestRunServerWithInteractiveRetryUsesSuggestedPortAndPersistsIt(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	config := validRuntimeConfig(8080)
	var attempts []int
	readyAddress := ""
	start := func(_ string, port int, onReady func(string) error) error {
		attempts = append(attempts, port)
		if len(attempts) == 1 {
			return fmt.Errorf("listen: %w", syscall.EADDRINUSE)
		}
		readyAddress = fmt.Sprintf("http://127.0.0.1:%d", port)
		return onReady(readyAddress)
	}
	var output bytes.Buffer
	err := runServerWithInteractiveRetry(
		&config, true, strings.NewReader("\n"), &output,
		func(address string) { readyAddress = address }, start,
		func(string, int) (int, bool) { return 8081, true },
	)
	if err != nil {
		t.Fatalf("runServerWithInteractiveRetry() error = %v", err)
	}
	if !reflect.DeepEqual(attempts, []int{8080, 8081}) {
		t.Fatalf("attempts = %v, want [8080 8081]", attempts)
	}
	if config.Port != 8081 || readyAddress != "http://127.0.0.1:8081" {
		t.Fatalf("config/ready = %d/%q", config.Port, readyAddress)
	}
	stored, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if stored.Port != 8081 {
		t.Fatalf("stored port = %d, want 8081", stored.Port)
	}
	if !strings.Contains(output.String(), "建议端口 8081") || !strings.Contains(output.String(), "新端口 8081 已保存") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRunServerWithInteractiveRetryRejectsInvalidAndRetriesOccupiedPort(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	config := validRuntimeConfig(8080)
	var attempts []int
	start := func(_ string, port int, onReady func(string) error) error {
		attempts = append(attempts, port)
		if port == 9001 {
			return onReady("http://127.0.0.1:9001")
		}
		return fmt.Errorf("listen: %w", syscall.EADDRINUSE)
	}
	var output bytes.Buffer
	err := runServerWithInteractiveRetry(
		&config, true, strings.NewReader("abc\n70000\n9000\n9001\n"), &output,
		nil, start, func(_ string, current int) (int, bool) { return current + 1, true },
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(attempts, []int{8080, 9000, 9001}) {
		t.Fatalf("attempts = %v", attempts)
	}
	if strings.Count(output.String(), "端口必须是 1 到 65535") != 2 {
		t.Fatalf("invalid-port prompts missing: %q", output.String())
	}
}

func TestRunServerWithInteractiveRetryAllowsUserToQuit(t *testing.T) {
	config := validRuntimeConfig(8080)
	err := runServerWithInteractiveRetry(
		&config, true, strings.NewReader("q\n"), io.Discard, nil,
		func(string, int, func(string) error) error { return syscall.EADDRINUSE },
		func(string, int) (int, bool) { return 8081, true },
	)
	if !errors.Is(err, errStartupCancelled) {
		t.Fatalf("error = %v, want errStartupCancelled", err)
	}
	if config.Port != 8080 {
		t.Fatalf("port changed after cancellation: %d", config.Port)
	}
}

func TestRunServerWithInteractiveRetryDoesNotBlockNonInteractiveStartup(t *testing.T) {
	config := validRuntimeConfig(8080)
	want := fmt.Errorf("listen: %w", syscall.EADDRINUSE)
	err := runServerWithInteractiveRetry(
		&config, false, panicReader{}, io.Discard, nil,
		func(string, int, func(string) error) error { return want },
		func(string, int) (int, bool) { t.Fatal("port finder must not run"); return 0, false },
	)
	if !errors.Is(err, syscall.EADDRINUSE) {
		t.Fatalf("error = %v", err)
	}
}

func TestRunServerWithInteractiveRetryDoesNotPromptForOtherErrors(t *testing.T) {
	config := validRuntimeConfig(8080)
	want := errors.New("permission denied")
	err := runServerWithInteractiveRetry(
		&config, true, panicReader{}, io.Discard, nil,
		func(string, int, func(string) error) error { return want },
		func(string, int) (int, bool) { t.Fatal("port finder must not run"); return 0, false },
	)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestIsAddressInUseRecognizesRealListenerConflict(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	conflict, err := net.Listen("tcp", listener.Addr().String())
	if err == nil {
		conflict.Close()
		t.Fatal("second listener unexpectedly bound to occupied address")
	}
	if !isAddressInUse(err) {
		t.Fatalf("isAddressInUse(%v) = false", err)
	}
}

func validRuntimeConfig(port int) security.RuntimeConfig {
	return security.RuntimeConfig{ListenAddress: "127.0.0.1", Port: port, OneBotToken: "test-token"}
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("non-interactive startup attempted to read stdin")
}
