package core

import (
	"strings"
	"testing"
)

func TestStartupSummaryShowsUsefulEndpoints(t *testing.T) {
	tests := []struct {
		name    string
		address string
		admin   string
		oneBot  string
	}{
		{name: "bare address", address: "127.0.0.1:8080", admin: "http://127.0.0.1:8080/admin", oneBot: "ws://127.0.0.1:8080/v1/ws"},
		{name: "http address from server manager", address: "http://127.0.0.1:8080", admin: "http://127.0.0.1:8080/admin", oneBot: "ws://127.0.0.1:8080/v1/ws"},
		{name: "https address", address: "https://pet.example.com", admin: "https://pet.example.com/admin", oneBot: "wss://pet.example.com/v1/ws"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			summary := startupSummary(test.address)
			for _, expected := range []string{"QQ-Pet 已就绪", test.admin, test.oneBot} {
				if !strings.Contains(summary, expected) {
					t.Fatalf("startup summary should contain %q, got %q", expected, summary)
				}
			}
			if strings.Contains(summary, "://http://") {
				t.Fatalf("startup summary must not duplicate URL schemes: %q", summary)
			}
		})
	}
}
