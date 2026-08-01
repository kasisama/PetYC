package admin

import (
	"io/fs"
	"strings"
	"testing"
)

// TestEmbeddedAdminUsesAccountLoginInsteadOfPromptToken 验证嵌入的 Vue SPA 产物：
//  1. index.html 是合法的 SPA 入口（有 <script type="module"> 加载 Vite 打包的 JS）。
//  2. 打包的 JS/CSS 中包含新认证流所需的 API 路径和界面文案。
//  3. 旧 prompt-token 认证流的特征字符串已被彻底移除。
//
// 注：Vite 生产构建会对变量名做 mangle，但字符串字面量（API 路径、模板文本）不受影响，
// 因此针对字符串字面量的断言在哈希文件名变化后依然稳定。
func TestEmbeddedAdminUsesAccountLoginInsteadOfPromptToken(t *testing.T) {
	// 1. 验证 index.html 是 Vue SPA 的启动壳。
	raw, err := Assets.ReadFile("dist/index.html")
	if err != nil {
		t.Fatalf("read embedded admin page: %v", err)
	}
	if !strings.Contains(string(raw), `<script type="module"`) {
		t.Error(`index.html 缺少 <script type="module"，不是有效的 Vue SPA 入口`)
	}

	// 2. 收集 dist/assets 下所有文件的文本内容，统一在其中做字符串检查。
	var sb strings.Builder
	walkErr := fs.WalkDir(Assets, "dist/assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, readErr := Assets.ReadFile(path)
		if readErr != nil {
			t.Errorf("read embedded asset %s: %v", path, readErr)
			return nil
		}
		sb.Write(b)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk embedded dist/assets: %v", walkErr)
	}
	combined := sb.String()

	// 新认证流依赖这些端点路径和 UI 字符串——它们是字符串字面量，minify 后依然保留。
	for _, required := range []string{
		"/api/admin/auth/login",
		"/api/admin/auth/session",
		"/api/admin/auth/logout",
		"/api/admin/auth/password",
		"记住我",
		"修改密码",
		"退出登录",
	} {
		if !strings.Contains(combined, required) {
			t.Errorf("打包产物缺少必要字符串 %q", required)
		}
	}

	// 旧 prompt-token 流的特征字符串不应出现在任何产物中。
	for _, legacy := range []string{
		"window.prompt",
		"qqpet.adminToken",
		"sessionStorage.getItem(adminTokenStorageKey)",
		"Authorization', `Bearer",
	} {
		if strings.Contains(combined, legacy) {
			t.Errorf("打包产物仍含有旧 token 流特征字符串 %q", legacy)
		}
	}
}
