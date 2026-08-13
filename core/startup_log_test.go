package core

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestRegisterHandlerDoesNotLogEachCommand(t *testing.T) {
	var output bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		handlersMu.Lock()
		delete(handlers, "测试静默注册")
		handlersMu.Unlock()
	})

	RegisterHandler("测试静默注册", nil)

	if output.Len() != 0 {
		t.Fatalf("registering one command should stay silent, got %q", output.String())
	}
}

func TestStartupSummaryShowsUsefulEndpoints(t *testing.T) {
	summary := startupSummary("127.0.0.1:8080")
	for _, expected := range []string{
		"QQ-Pet SaaS 已就绪",
		"http://127.0.0.1:8080/admin",
		"ws://127.0.0.1:8080/v1/ws",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("startup summary should contain %q, got %q", expected, summary)
		}
	}
}
