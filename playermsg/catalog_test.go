package playermsg

import (
	"strings"
	"testing"
)

func TestCatalogSamplesRenderWithoutPlayerBannedTerms(t *testing.T) {
	banned := []string{"新版", "旧版", "下线", "固定进度", "平台额度", "内部账号", "不含抽奖", "迁移", "SQL", "http://", "https://"}
	for _, definition := range Catalog() {
		if definition.Key == "" || definition.Kind == "" || definition.Tone == "" {
			t.Fatalf("message metadata is incomplete: %#v", definition)
		}
		sample, err := Render(definition.Key, definition.Variables)
		if err != nil {
			t.Fatalf("render %s: %v", definition.Key, err)
		}
		for _, word := range banned {
			if strings.Contains(sample, word) {
				t.Fatalf("message %s contains banned term %q: %s", definition.Key, word, sample)
			}
		}
	}
}

func TestRenderRejectsUnknownMessageKey(t *testing.T) {
	if _, err := Render("missing.key", nil); err == nil {
		t.Fatal("expected unknown key error")
	}
}
