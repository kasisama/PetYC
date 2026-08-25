package admin

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/security"
)

func TestSessionManagerCreatesAndRevokesSession(t *testing.T) {
	manager := newMemorySessionManager()
	token, _, err := manager.Create(false)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if token == "" {
		t.Fatal("Create() returned an empty token")
	}
	if !manager.Validate(token) {
		t.Fatal("Validate() rejected a new session")
	}

	manager.Delete(token)
	if manager.Validate(token) {
		t.Fatal("Validate() accepted a deleted session")
	}
}

func TestSessionManagerRejectsExpiredSession(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	manager := newMemorySessionManager()
	manager.now = func() time.Time { return now }

	token, expiresAt, err := manager.Create(true)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if want := now.Add(rememberedSessionDuration); !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", expiresAt, want)
	}

	now = expiresAt.Add(time.Second)
	if manager.Validate(token) {
		t.Fatal("Validate() accepted an expired session")
	}
}

func TestSessionManagerDeleteAllRevokesEverySession(t *testing.T) {
	manager := newMemorySessionManager()
	first, _, err := manager.Create(false)
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	second, _, err := manager.Create(true)
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}

	manager.DeleteAll()
	if manager.Validate(first) || manager.Validate(second) {
		t.Fatal("DeleteAll() left a session active")
	}
}

func newMemorySessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[[sha256.Size]byte]adminSession),
		now:      time.Now,
	}
}

func TestSessionManagerCreatePrunesExpiredSessions(t *testing.T) {
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	manager := newMemorySessionManager()
	manager.now = func() time.Time { return now }
	if _, _, err := manager.Create(false); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	now = now.Add(browserSessionDuration + time.Second)
	if _, _, err := manager.Create(false); err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if len(manager.sessions) != 1 {
		t.Fatalf("session count = %d, want 1 active session", len(manager.sessions))
	}
}

func TestSessionManagerRememberMeSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, adminSessionsFileName)
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)

	first := &SessionManager{
		sessions:  make(map[[sha256.Size]byte]adminSession),
		now:       func() time.Time { return now },
		storePath: path,
	}
	token, _, err := first.Create(true)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// 模拟 go run 重启：新的 SessionManager 从磁盘恢复
	second := &SessionManager{
		sessions:  make(map[[sha256.Size]byte]adminSession),
		now:       func() time.Time { return now },
		storePath: path,
	}
	second.loadFromDisk()
	if !second.Validate(token) {
		t.Fatal("remembered session was lost after manager reload")
	}

	// 未勾选记住我的会话不应落盘
	ephemeral, _, err := first.Create(false)
	if err != nil {
		t.Fatalf("ephemeral Create() error = %v", err)
	}
	third := &SessionManager{
		sessions:  make(map[[sha256.Size]byte]adminSession),
		now:       func() time.Time { return now },
		storePath: path,
	}
	third.loadFromDisk()
	if third.Validate(ephemeral) {
		t.Fatal("non-remember session should not survive reload")
	}
	if !third.Validate(token) {
		t.Fatal("remembered session should still be valid")
	}
}

func TestRequireAdminSessionRejectsMissingOrWrongCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := NewSessionManager()
	router := gin.New()
	router.Use(RequireAdminSession(manager))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, cookieValue := range []string{"", "wrong-session"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/protected", nil)
		if cookieValue != "" {
			request.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: cookieValue})
		}
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("cookie %q returned %d, want %d", cookieValue, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestRequireAdminSessionAcceptsValidCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := NewSessionManager()
	token, _, err := manager.Create(false)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	router := gin.New()
	router.Use(RequireAdminSession(manager))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: token})
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestAdminLoginRejectsWrongCredentials(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	recorder := performJSONRequest(
		router,
		http.MethodPost,
		"/api/admin/auth/login",
		`{"username":"admin","password":"wrong","remember":false}`,
		nil,
	)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "账号或密码错误" {
		t.Fatalf("error = %#v", payload["error"])
	}
}

func TestAdminLoginCreatesBrowserSessionCookieByDefault(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	recorder := performJSONRequest(
		router,
		http.MethodPost,
		"/api/admin/auth/login",
		`{"username":"admin","password":"admin-test-password","remember":false}`,
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookie := responseCookie(t, recorder, adminSessionCookieName)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie security attributes = %#v", cookie)
	}
	if cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
		t.Fatalf("browser session cookie unexpectedly persisted: %#v", cookie)
	}
}

func TestAdminLoginRememberMeCreatesFifteenDayCookie(t *testing.T) {
	router, manager := newAuthTestRouter(t)
	now := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	recorder := performJSONRequest(
		router,
		http.MethodPost,
		"/api/admin/auth/login",
		`{"username":"admin","password":"admin-test-password","remember":true}`,
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookie := responseCookie(t, recorder, adminSessionCookieName)
	if cookie.MaxAge != int(rememberedSessionDuration/time.Second) {
		t.Fatalf("MaxAge = %d, want %d", cookie.MaxAge, int(rememberedSessionDuration/time.Second))
	}
	if want := now.Add(rememberedSessionDuration); !cookie.Expires.Equal(want) {
		t.Fatalf("Expires = %v, want %v", cookie.Expires, want)
	}
}

func TestAdminSessionReportsAuthenticationState(t *testing.T) {
	router, manager := newAuthTestRouter(t)

	anonymous := performJSONRequest(router, http.MethodGet, "/api/admin/auth/session", "", nil)
	if anonymous.Code != http.StatusOK {
		t.Fatalf("anonymous status = %d", anonymous.Code)
	}
	assertAuthenticatedResponse(t, anonymous, false)

	token, _, err := manager.Create(false)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	authenticated := performJSONRequest(
		router,
		http.MethodGet,
		"/api/admin/auth/session",
		"",
		&http.Cookie{Name: adminSessionCookieName, Value: token},
	)
	if authenticated.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d", authenticated.Code)
	}
	assertAuthenticatedResponse(t, authenticated, true)
}

func TestAdminLogoutRevokesOnlyCurrentSession(t *testing.T) {
	router, manager := newAuthTestRouter(t)
	current, _, err := manager.Create(false)
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	other, _, err := manager.Create(true)
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}

	recorder := performJSONRequest(
		router,
		http.MethodPost,
		"/api/admin/auth/logout",
		"",
		&http.Cookie{Name: adminSessionCookieName, Value: current},
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if manager.Validate(current) {
		t.Fatal("current session remains valid")
	}
	if !manager.Validate(other) {
		t.Fatal("other session was unexpectedly revoked")
	}
}

func TestAdminPasswordChangeRevokesAllSessions(t *testing.T) {
	router, manager := newAuthTestRouter(t)
	current, _, err := manager.Create(false)
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	other, _, err := manager.Create(true)
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}

	recorder := performJSONRequest(
		router,
		http.MethodPut,
		"/api/admin/auth/password",
		`{"current_password":"admin-test-password","new_password":"replacement-password","confirm_password":"replacement-password"}`,
		&http.Cookie{Name: adminSessionCookieName, Value: current},
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if manager.Validate(current) || manager.Validate(other) {
		t.Fatal("password change left an existing session valid")
	}
	oldOK, err := security.VerifyAdminCredentials("admin", "admin-test-password")
	if err != nil {
		t.Fatalf("verify old password error = %v", err)
	}
	newOK, err := security.VerifyAdminCredentials("admin", "replacement-password")
	if err != nil {
		t.Fatalf("verify new password error = %v", err)
	}
	if oldOK || !newOK {
		t.Fatalf("old password valid = %v, new password valid = %v", oldOK, newOK)
	}
}

func TestAdminPasswordChangeValidatesInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "wrong current password",
			body: `{"current_password":"wrong","new_password":"replacement-password","confirm_password":"replacement-password"}`,
		},
		{
			name: "short new password",
			body: `{"current_password":"admin-test-password","new_password":"12345","confirm_password":"12345"}`,
		},
		{
			name: "confirmation mismatch",
			body: `{"current_password":"admin-test-password","new_password":"replacement-password","confirm_password":"different-password"}`,
		},
		{
			name: "password exceeds bcrypt limit",
			body: `{"current_password":"admin-test-password","new_password":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirm_password":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, manager := newAuthTestRouter(t)
			token, _, err := manager.Create(false)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			recorder := performJSONRequest(
				router,
				http.MethodPut,
				"/api/admin/auth/password",
				test.body,
				&http.Cookie{Name: adminSessionCookieName, Value: token},
			)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestInitialPasswordSetupRequiresLoopbackAndDisablesSetupMode(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQPET_WS_TOKEN", "")
	manager := newMemorySessionManager()
	handler := NewAuthHandler(manager)
	router := gin.New()
	router.POST("/api/admin/auth/setup", handler.SetupPassword)

	body := `{"username":"owner","password":"local-password","confirm_password":"local-password"}`
	remote := performJSONRequest(router, http.MethodPost, "/api/admin/auth/setup", body, nil)
	if remote.Code != http.StatusForbidden {
		t.Fatalf("remote setup status = %d, want %d", remote.Code, http.StatusForbidden)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/admin/auth/setup", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:45678"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("local setup status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	credentials, err := security.LoadCredentials()
	if err != nil || credentials.PasswordSetupRequired {
		t.Fatalf("credentials after setup = %#v, %v", credentials, err)
	}
	valid, err := security.VerifyAdminCredentials("owner", "local-password")
	if err != nil || !valid {
		t.Fatalf("configured credentials valid = %v, error = %v", valid, err)
	}
}

func TestAdminLoginRateLimitBlocksRepeatedFailures(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	for index := 0; index < maxLoginFailures; index++ {
		recorder := performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", `{"username":"admin","password":"wrong"}`, nil)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", index+1, recorder.Code)
		}
	}
	recorder := performJSONRequest(router, http.MethodPost, "/api/admin/auth/login", `{"username":"admin","password":"wrong"}`, nil)
	if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") == "" {
		t.Fatalf("blocked status = %d, retry-after = %q", recorder.Code, recorder.Header().Get("Retry-After"))
	}
}

func TestRequireSameOriginRejectsCrossSiteWrites(t *testing.T) {
	router := gin.New()
	router.Use(RequireSameOrigin())
	router.POST("/write", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	bad := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/write", nil)
	bad.Host = "127.0.0.1"
	bad.Header.Set("Origin", "https://evil.example")
	badRecorder := httptest.NewRecorder()
	router.ServeHTTP(badRecorder, bad)
	if badRecorder.Code != http.StatusForbidden {
		t.Fatalf("cross-site status = %d", badRecorder.Code)
	}

	good := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/write", nil)
	good.Host = "127.0.0.1"
	good.Header.Set("Origin", "http://127.0.0.1")
	goodRecorder := httptest.NewRecorder()
	router.ServeHTTP(goodRecorder, good)
	if goodRecorder.Code != http.StatusNoContent {
		t.Fatalf("same-origin status = %d", goodRecorder.Code)
	}
}

func newAuthTestRouter(t *testing.T) (*gin.Engine, *SessionManager) {
	t.Helper()
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQPET_WS_TOKEN", "")
	if _, err := security.LoadCredentials(); err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if err := security.SetInitialAdminPassword("admin", "admin-test-password"); err != nil {
		t.Fatalf("SetInitialAdminPassword() error = %v", err)
	}

	gin.SetMode(gin.TestMode)
	manager := NewSessionManager()
	handler := NewAuthHandler(manager)
	router := gin.New()
	router.POST("/api/admin/auth/login", handler.Login)
	router.POST("/api/admin/auth/setup", handler.SetupPassword)
	router.GET("/api/admin/auth/session", handler.Session)
	protected := router.Group("/api/admin", RequireAdminSession(manager))
	protected.POST("/auth/logout", handler.Logout)
	protected.PUT("/auth/password", handler.ChangePassword)
	protected.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router, manager
}

func performJSONRequest(
	router http.Handler,
	method string,
	path string,
	body string,
	cookie *http.Cookie,
) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func responseCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response has no %q cookie", name)
	return nil
}

func assertAuthenticatedResponse(t *testing.T, recorder *httptest.ResponseRecorder, want bool) {
	t.Helper()
	var payload struct {
		Authenticated bool `json:"authenticated"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Authenticated != want {
		t.Fatalf("authenticated = %v, want %v", payload.Authenticated, want)
	}
}
