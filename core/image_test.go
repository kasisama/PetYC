package core

import (
	"os"
	"path/filepath"
	"testing"

	"qq-pet-saas/config"
)

func TestExistingImageSourceUsesSafeExistingCandidateAndFallsBack(t *testing.T) {
	previousRoot := config.GlobalConfigPath
	t.Cleanup(func() { config.GlobalConfigPath = previousRoot })
	config.GlobalConfigPath = t.TempDir()
	imageDir := filepath.Join(config.GlobalConfigPath, "图片", "核心图片")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imageDir, "领养.jpg"), []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ExistingImageSource("不存在.png", "核心图片/领养.jpg"); got != "核心图片/领养.jpg" {
		t.Fatalf("expected existing fallback, got %q", got)
	}
	if got := ExistingImageSource("../secret.png"); got != "" {
		t.Fatalf("unsafe image path must be rejected, got %q", got)
	}
}
