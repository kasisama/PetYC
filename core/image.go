package core

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"qq-pet-saas/config"
)

// ExistingImageSource returns the first safe, available image source. HTTP(S)
// sources are passed through; local sources must resolve inside an image root.
func ExistingImageSource(candidates ...string) string {
	for _, source := range candidates {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if parsed, err := url.Parse(source); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			return source
		}
		normalized := strings.TrimPrefix(strings.ReplaceAll(source, "\\", "/"), "./")
		normalized = strings.TrimPrefix(normalized, "图片/")
		clean := filepath.Clean(filepath.FromSlash(normalized))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			continue
		}
		for _, root := range []string{filepath.Join(config.GlobalConfigPath, "图片"), filepath.Join(".", "图片")} {
			absoluteRoot, err := filepath.Abs(root)
			if err != nil {
				continue
			}
			candidate, err := filepath.Abs(filepath.Join(absoluteRoot, clean))
			if err != nil {
				continue
			}
			relative, err := filepath.Rel(absoluteRoot, candidate)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				continue
			}
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				return filepath.ToSlash(clean)
			}
		}
	}
	return ""
}
