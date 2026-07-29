# Admin Account Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the admin prompt/Bearer-token flow with a retryable `admin` account login, optional 15-day remembrance, logout, and password change.

**Architecture:** Persist the fixed username and bcrypt password hash in the existing credentials file, while keeping the WebSocket token independent. Authenticate browser requests with opaque random HttpOnly cookies backed by an in-memory session manager; render login and password-management states in the existing embedded Vue page.

**Tech Stack:** Go 1.26, Gin, `golang.org/x/crypto/bcrypt`, embedded Vue 3, Go `httptest`.

## Global Constraints

- Fixed username: `admin`.
- Initial password: `123456`.
- “Remember me” is unchecked by default and lasts exactly 15 days when selected.
- A non-remembered login uses a browser-session Cookie.
- Passwords are never persisted in plaintext.
- New passwords contain at least 6 characters and at most 72 UTF-8 bytes.
- WebSocket token loading and validation remain unchanged.
- The workspace is not a Git repository, so commit steps are documented but cannot be executed here.

---

### Task 1: Persist and verify administrator credentials

**Files:**
- Modify: `security/credentials.go`
- Modify: `security/credentials_test.go`

**Interfaces:**
- Consumes: `QQPET_DATA_DIR`, `QQPET_WS_TOKEN`, and the existing `credentials.json`.
- Produces: `VerifyAdminCredentials(username, password string) (bool, error)`, `ChangeAdminPassword(currentPassword, newPassword string) error`, `ErrInvalidAdminPassword`.

- [ ] **Step 1: Write failing credential migration and password-change tests**

```go
func TestLoadCredentialsAddsDefaultAdminLogin(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	credentials, err := LoadCredentials()
	if err != nil { t.Fatal(err) }
	if credentials.AdminUsername != "admin" || credentials.AdminPasswordHash == "" {
		t.Fatalf("unexpected credentials: %#v", credentials)
	}
	ok, err := VerifyAdminCredentials("admin", "123456")
	if err != nil || !ok { t.Fatalf("default login failed: ok=%v err=%v", ok, err) }
}

func TestChangeAdminPasswordInvalidatesOldPassword(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	if _, err := LoadCredentials(); err != nil { t.Fatal(err) }
	if err := ChangeAdminPassword("123456", "new-secret"); err != nil { t.Fatal(err) }
	oldOK, _ := VerifyAdminCredentials("admin", "123456")
	newOK, _ := VerifyAdminCredentials("admin", "new-secret")
	if oldOK || !newOK { t.Fatalf("old=%v new=%v", oldOK, newOK) }
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./security -run 'TestLoadCredentialsAddsDefaultAdminLogin|TestChangeAdminPasswordInvalidatesOldPassword' -v`

Expected: build failure because the new fields and functions do not exist.

- [ ] **Step 3: Implement locked loading, bcrypt verification, and atomic writes**

Add these public fields and functions:

```go
type Credentials struct {
	AdminUsername    string `json:"admin_username"`
	AdminPasswordHash string `json:"admin_password_hash"`
	WebSocketToken   string `json:"websocket_token"`
}

var ErrInvalidAdminPassword = errors.New("invalid admin password")

func VerifyAdminCredentials(username, password string) (bool, error)
func ChangeAdminPassword(currentPassword, newPassword string) error
```

`LoadCredentials` must initialize missing administrator fields with `admin` and a bcrypt hash of `123456`, preserve/generate the WebSocket token, and persist changes with a same-directory temporary file followed by `os.Rename`.

- [ ] **Step 4: Format and verify GREEN**

Run: `gofmt -w security/credentials.go security/credentials_test.go`

Run: `go test ./security -v`

Expected: all `security` tests pass.

- [ ] **Step 5: Commit**

```bash
git add security/credentials.go security/credentials_test.go
git commit -m "feat: persist admin account credentials"
```

Skip in this workspace because no `.git` directory exists.

### Task 2: Add opaque server-side sessions and authentication middleware

**Files:**
- Modify: `admin/auth.go`
- Modify: `admin/auth_test.go`

**Interfaces:**
- Consumes: Cookie name `qqpet_admin_session` and a clock.
- Produces: `SessionManager`, `NewSessionManager()`, `Create(bool)`, `Validate(string)`, `Delete(string)`, `DeleteAll()`, and `RequireAdminSession(*SessionManager)`.

- [ ] **Step 1: Write failing tests for session creation, expiry, deletion, and middleware**

```go
func TestSessionManagerCreatesAndRevokesSession(t *testing.T) {
	manager := NewSessionManager()
	token, _, err := manager.Create(false)
	if err != nil { t.Fatal(err) }
	if !manager.Validate(token) { t.Fatal("new session rejected") }
	manager.Delete(token)
	if manager.Validate(token) { t.Fatal("deleted session accepted") }
}

func TestRequireAdminSessionRejectsMissingCookie(t *testing.T) {
	router := gin.New()
	router.Use(RequireAdminSession(NewSessionManager()))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./admin -run 'TestSessionManager|TestRequireAdminSession' -v`

Expected: build failure because the session interfaces do not exist.

- [ ] **Step 3: Implement the minimal session manager and middleware**

Use a 32-byte `crypto/rand` token, encode it with raw URL-safe base64, hash it with SHA-256 for map keys, expire stored sessions after 15 days, and return JSON 401 while clearing an invalid Cookie.

- [ ] **Step 4: Format and verify GREEN**

Run: `gofmt -w admin/auth.go admin/auth_test.go`

Run: `go test ./admin -run 'TestSessionManager|TestRequireAdminSession' -v`

Expected: all focused tests pass.

- [ ] **Step 5: Commit**

```bash
git add admin/auth.go admin/auth_test.go
git commit -m "feat: add admin browser sessions"
```

Skip in this workspace because no `.git` directory exists.

### Task 3: Expose login, session, logout, and password-change APIs

**Files:**
- Modify: `admin/auth.go`
- Modify: `admin/auth_test.go`
- Modify: `admin/handler.go`

**Interfaces:**
- Consumes: `security.VerifyAdminCredentials`, `security.ChangeAdminPassword`, `SessionManager`.
- Produces: `AuthHandler.Login`, `AuthHandler.Session`, `AuthHandler.Logout`, `AuthHandler.ChangePassword`.

- [ ] **Step 1: Write failing HTTP behavior tests**

Tests must submit real JSON and assert:

```go
// Wrong credentials.
POST /api/admin/auth/login
{"username":"admin","password":"wrong","remember":false}
// => 401 {"error":"账号或密码错误"}

// Session-only login.
POST /api/admin/auth/login
{"username":"admin","password":"123456","remember":false}
// => 200; Set-Cookie has HttpOnly and no Max-Age.

// Remembered login.
POST /api/admin/auth/login
{"username":"admin","password":"123456","remember":true}
// => 200; Set-Cookie Max-Age is 1296000.

// Password update.
PUT /api/admin/auth/password
{"current_password":"123456","new_password":"654321","confirm_password":"654321"}
// => 200; every previously issued session becomes invalid.
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./admin -run 'TestAdmin(Login|Session|Logout|Password)' -v`

Expected: failures because the routes and handlers are missing.

- [ ] **Step 3: Implement handlers and route grouping**

Register `POST /api/admin/auth/login` and `GET /api/admin/auth/session` publicly. Register logout, password change, and all existing admin data APIs under `RequireAdminSession`. Validate non-empty login fields, enforce six-character new passwords, require matching confirmation, and revoke every session after a successful password change.

- [ ] **Step 4: Verify GREEN and existing backend behavior**

Run: `gofmt -w admin/auth.go admin/auth_test.go admin/handler.go`

Run: `go test ./admin ./security ./core -v`

Expected: all tests pass, including existing WebSocket tests.

- [ ] **Step 5: Commit**

```bash
git add admin/auth.go admin/auth_test.go admin/handler.go
git commit -m "feat: expose admin login and password APIs"
```

Skip in this workspace because no `.git` directory exists.

### Task 4: Replace the browser prompt with account UI

**Files:**
- Create: `admin/frontend_auth_test.go`
- Modify: `admin/dist/index.html`

**Interfaces:**
- Consumes: the four `/api/admin/auth/*` endpoints and Cookie authentication.
- Produces: retryable login form, optional remembrance, change-password modal, logout button, and global 401 fallback.

- [ ] **Step 1: Write a failing embedded-asset regression test**

```go
func TestEmbeddedAdminUsesAccountLoginInsteadOfPromptToken(t *testing.T) {
	raw, err := Assets.ReadFile("dist/index.html")
	if err != nil { t.Fatal(err) }
	html := string(raw)
	for _, required := range []string{
		"/api/admin/auth/login",
		"/api/admin/auth/session",
		"/api/admin/auth/logout",
		"/api/admin/auth/password",
		"记住我",
	} {
		if !strings.Contains(html, required) { t.Errorf("missing %q", required) }
	}
	if strings.Contains(html, "window.prompt") || strings.Contains(html, "qqpet.adminToken") {
		t.Fatal("legacy prompt token flow remains")
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./admin -run TestEmbeddedAdminUsesAccountLoginInsteadOfPromptToken -v`

Expected: failure because the login endpoints and text are absent and the prompt flow remains.

- [ ] **Step 3: Implement the Vue authentication states**

Add `auth.ready`, `auth.authenticated`, `auth.username`, login form state, change-password modal state, and request functions. On mount call only the session endpoint; load dashboard data only after authenticated. Replace the Bearer-token wrapper with same-origin Cookie requests and a single 401 callback. Add top-bar buttons for password change and logout.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./admin -run TestEmbeddedAdminUsesAccountLoginInsteadOfPromptToken -v`

Expected: the embedded-asset regression test passes.

- [ ] **Step 5: Commit**

```bash
git add admin/dist/index.html admin/frontend_auth_test.go
git commit -m "feat: add admin account login interface"
```

Skip in this workspace because no `.git` directory exists.

### Task 5: Full verification

**Files:**
- Verify all modified files.

**Interfaces:**
- Consumes: Tasks 1–4.
- Produces: a tested, buildable account-login flow.

- [ ] **Step 1: Run formatting**

Run: `gofmt -w security/credentials.go security/credentials_test.go admin/auth.go admin/auth_test.go admin/handler.go admin/frontend_auth_test.go`

Expected: exit code 0.

- [ ] **Step 2: Run the complete test suite**

Run: `go test ./...`

Expected: exit code 0 with no failing package.

- [ ] **Step 3: Build every package**

Run: `go build ./...`

Expected: exit code 0.

- [ ] **Step 4: Review requirements against the diff**

Confirm the prompt and `sessionStorage` token are gone, login can be retried, remember-me is opt-in for 15 days, a 401 returns to login, password change revokes sessions, and WebSocket credentials are untouched.

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "test: verify admin account login flow"
```

Skip in this workspace because no `.git` directory exists.
