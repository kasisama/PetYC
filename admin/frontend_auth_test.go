package admin

import (
	"strings"
	"testing"
)

func TestEmbeddedAdminUsesAccountLoginInsteadOfPromptToken(t *testing.T) {
	raw, err := Assets.ReadFile("dist/index.html")
	if err != nil {
		t.Fatalf("read embedded admin page: %v", err)
	}
	html := string(raw)
	for _, required := range []string{
		"/api/admin/auth/login",
		"/api/admin/auth/session",
		"/api/admin/auth/logout",
		"/api/admin/auth/password",
		"loginForm.remember = false",
		"记住我",
		"修改密码",
		"退出登录",
	} {
		if !strings.Contains(html, required) {
			t.Errorf("embedded admin page is missing %q", required)
		}
	}
	for _, legacy := range []string{
		"window.prompt",
		"qqpet.adminToken",
		"sessionStorage.getItem(adminTokenStorageKey)",
		"Authorization', `Bearer",
	} {
		if strings.Contains(html, legacy) {
			t.Errorf("embedded admin page still contains legacy token flow %q", legacy)
		}
	}
}
