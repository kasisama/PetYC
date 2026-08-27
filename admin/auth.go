package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/config"
	"qq-pet-saas/security"
)

const (
	adminSessionCookieName = "qqpet_admin_session"
	adminSessionsFileName  = "admin_sessions.json"
	// 未勾选「记住我」：浏览器会话 Cookie，服务端侧保留 12 小时。
	browserSessionDuration = 12 * time.Hour
	// 勾选「记住我」：持久 Cookie + 落盘，跨进程重启仍有效。
	rememberedSessionDuration = 15 * 24 * time.Hour
	minAdminPasswordLength    = 8
	maxAdminPasswordBytes     = 72
	loginAttemptWindow        = 5 * time.Minute
	loginBlockDuration        = 10 * time.Minute
	maxLoginFailures          = 5
)

type adminSession struct {
	expiresAt time.Time
	remember  bool
}

// SessionManager owns the opaque sessions used by the admin UI. Only token
// digests are retained by the server; raw tokens stay in HttpOnly cookies.
// Remembered sessions are also written to disk so go run / 进程重启后仍可校验。
type SessionManager struct {
	mu       sync.Mutex
	sessions map[[sha256.Size]byte]adminSession
	now      func() time.Time
	// storePath empty disables persistence (unit tests).
	storePath string
}

func NewSessionManager() *SessionManager {
	manager := &SessionManager{
		sessions: make(map[[sha256.Size]byte]adminSession),
		now:      time.Now,
	}
	if dir, err := security.DataDir(); err == nil {
		manager.storePath = filepath.Join(dir, adminSessionsFileName)
		manager.loadFromDisk()
	}
	return manager
}

func (manager *SessionManager) Create(remember bool) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	now := manager.now()
	ttl := browserSessionDuration
	if remember {
		ttl = rememberedSessionDuration
	}
	expiresAt := now.Add(ttl)

	manager.mu.Lock()
	manager.pruneExpiredLocked(now)
	manager.sessions[digest] = adminSession{expiresAt: expiresAt, remember: remember}
	if remember {
		_ = manager.persistLocked()
	}
	manager.mu.Unlock()
	return token, expiresAt, nil
}

func (manager *SessionManager) Validate(token string) bool {
	if token == "" {
		return false
	}
	digest := sha256.Sum256([]byte(token))
	now := manager.now()

	manager.mu.Lock()
	defer manager.mu.Unlock()
	session, exists := manager.sessions[digest]
	if !exists {
		return false
	}
	if now.After(session.expiresAt) {
		delete(manager.sessions, digest)
		if session.remember {
			_ = manager.persistLocked()
		}
		return false
	}
	return true
}

func (manager *SessionManager) Delete(token string) {
	if token == "" {
		return
	}
	digest := sha256.Sum256([]byte(token))
	manager.mu.Lock()
	session, existed := manager.sessions[digest]
	delete(manager.sessions, digest)
	if existed && session.remember {
		_ = manager.persistLocked()
	}
	manager.mu.Unlock()
}

func (manager *SessionManager) DeleteAll() {
	manager.mu.Lock()
	manager.sessions = make(map[[sha256.Size]byte]adminSession)
	_ = manager.persistLocked()
	manager.mu.Unlock()
}

func (manager *SessionManager) pruneExpiredLocked(now time.Time) {
	changed := false
	for storedDigest, session := range manager.sessions {
		if now.After(session.expiresAt) {
			delete(manager.sessions, storedDigest)
			if session.remember {
				changed = true
			}
		}
	}
	if changed {
		_ = manager.persistLocked()
	}
}

type sessionStoreFile struct {
	Sessions map[string]sessionStoreEntry `json:"sessions"`
}

type sessionStoreEntry struct {
	ExpiresAt time.Time `json:"expires_at"`
}

func (manager *SessionManager) loadFromDisk() {
	if manager.storePath == "" {
		return
	}
	raw, err := os.ReadFile(manager.storePath)
	if err != nil {
		return
	}
	var file sessionStoreFile
	if err := json.Unmarshal(raw, &file); err != nil || file.Sessions == nil {
		return
	}
	now := manager.now()
	for key, entry := range file.Sessions {
		if now.After(entry.ExpiresAt) {
			continue
		}
		digestBytes, err := hex.DecodeString(key)
		if err != nil || len(digestBytes) != sha256.Size {
			continue
		}
		var digest [sha256.Size]byte
		copy(digest[:], digestBytes)
		manager.sessions[digest] = adminSession{
			expiresAt: entry.ExpiresAt,
			remember:  true,
		}
	}
}

func (manager *SessionManager) persistLocked() error {
	if manager.storePath == "" {
		return nil
	}
	file := sessionStoreFile{Sessions: make(map[string]sessionStoreEntry)}
	for digest, session := range manager.sessions {
		if !session.remember {
			continue
		}
		file.Sessions[hex.EncodeToString(digest[:])] = sessionStoreEntry{
			ExpiresAt: session.expiresAt,
		}
	}
	if err := os.MkdirAll(filepath.Dir(manager.storePath), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	tmp := manager.storePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, manager.storePath)
}

// RequireAdminSession protects administrative APIs with an opaque Cookie.
func RequireAdminSession(manager *SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(adminSessionCookieName)
		if err != nil || !manager.Validate(token) {
			clearAdminSessionCookie(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已失效，请重新登录"})
			return
		}
		c.Set("admin_session_token", token)
		c.Next()
	}
}

type AuthHandler struct {
	sessions *SessionManager
	limiter  *loginRateLimiter
}

func NewAuthHandler(sessions *SessionManager) *AuthHandler {
	return &AuthHandler{sessions: sessions, limiter: newLoginRateLimiter()}
}

type loginAttempt struct {
	windowStarted time.Time
	failures      int
	blockedUntil  time.Time
}

type loginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
	now      func() time.Time
}

func newLoginRateLimiter() *loginRateLimiter {
	return &loginRateLimiter{attempts: make(map[string]loginAttempt), now: time.Now}
}

func (limiter *loginRateLimiter) allow(key string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now()
	attempt := limiter.attempts[key]
	if now.Before(attempt.blockedUntil) {
		return false
	}
	if attempt.windowStarted.IsZero() || now.Sub(attempt.windowStarted) > loginAttemptWindow {
		delete(limiter.attempts, key)
	}
	return true
}

func (limiter *loginRateLimiter) fail(key string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now()
	attempt := limiter.attempts[key]
	if attempt.windowStarted.IsZero() || now.Sub(attempt.windowStarted) > loginAttemptWindow {
		attempt = loginAttempt{windowStarted: now}
	}
	attempt.failures++
	if attempt.failures >= maxLoginFailures {
		attempt.blockedUntil = now.Add(loginBlockDuration)
	}
	limiter.attempts[key] = attempt
}

func (limiter *loginRateLimiter) clear(key string) {
	limiter.mu.Lock()
	delete(limiter.attempts, key)
	limiter.mu.Unlock()
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

func (handler *AuthHandler) Login(c *gin.Context) {
	key := requestRemoteIP(c.Request)
	if !handler.limiter.allow(key) {
		c.Header("Retry-After", "600")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过于频繁，请稍后再试"})
		return
	}
	credentials, err := security.LoadCredentials()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "管理员凭据暂时不可用"})
		return
	}
	if credentials.PasswordSetupRequired {
		c.JSON(http.StatusConflict, gin.H{"error": "请先在本机设置管理员密码", "setup_required": true})
		return
	}
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil ||
		strings.TrimSpace(request.Username) == "" ||
		request.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入账号和密码"})
		return
	}

	username := strings.TrimSpace(request.Username)
	valid, err := security.VerifyAdminCredentials(username, request.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "管理员凭据暂时不可用"})
		return
	}
	if !valid {
		handler.limiter.fail(key)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号或密码错误"})
		return
	}
	handler.limiter.clear(key)

	token, expiresAt, err := handler.sessions.Create(request.Remember)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建登录会话失败"})
		return
	}
	setAdminSessionCookie(c, token, expiresAt, request.Remember)
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"username":      username,
	})
}

func (handler *AuthHandler) Session(c *gin.Context) {
	credentials, credentialsErr := security.LoadCredentials()
	if credentialsErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "管理员凭据暂时不可用"})
		return
	}
	if credentials.PasswordSetupRequired {
		c.JSON(http.StatusOK, gin.H{"authenticated": false, "setup_required": true, "username": credentials.AdminUsername})
		return
	}
	token, err := c.Cookie(adminSessionCookieName)
	if err != nil || !handler.sessions.Validate(token) {
		if token != "" {
			clearAdminSessionCookie(c)
		}
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"username":      credentials.AdminUsername,
	})
}

type setupPasswordRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (handler *AuthHandler) SetupPassword(c *gin.Context) {
	if !requestIsLoopback(c.Request) && !security.WebSetupEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"error": "首次密码只能在运行服务的本机设置"})
		return
	}
	var request setupPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Username) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入管理员账号和新密码"})
		return
	}
	if message := validateNewPassword(request.Password, request.ConfirmPassword); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}
	if err := security.SetInitialAdminPassword(strings.TrimSpace(request.Username), request.Password); err != nil {
		if errors.Is(err, security.ErrAdminSetupNotRequired) {
			c.JSON(http.StatusConflict, gin.H{"error": "管理员密码已经设置"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置管理员密码失败"})
		return
	}
	handler.sessions.DeleteAll()
	c.JSON(http.StatusOK, gin.H{"message": "管理员密码设置成功，请登录"})
}

func (handler *AuthHandler) Logout(c *gin.Context) {
	token, _ := c.Get("admin_session_token")
	if value, ok := token.(string); ok {
		handler.sessions.Delete(value)
	}
	clearAdminSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "已退出登录"})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (handler *AuthHandler) ChangePassword(c *gin.Context) {
	var request changePasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.CurrentPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入当前密码和新密码"})
		return
	}
	if message := validateNewPassword(request.NewPassword, request.ConfirmPassword); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}

	if err := security.ChangeAdminPassword(request.CurrentPassword, request.NewPassword); err != nil {
		if errors.Is(err, security.ErrInvalidAdminPassword) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前密码错误"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "修改密码失败"})
		return
	}

	handler.sessions.DeleteAll()
	clearAdminSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功，请重新登录"})
}

func validateNewPassword(password, confirmation string) string {
	if utf8.RuneCountInString(password) < minAdminPasswordLength {
		return "新密码至少需要 8 个字符"
	}
	if len(password) > maxAdminPasswordBytes {
		return "新密码不能超过 72 个字节"
	}
	if password != confirmation {
		return "两次输入的新密码不一致"
	}
	return ""
}

// RequireSameOrigin rejects browser cross-site writes while keeping local CLI
// clients usable when they omit browser-only headers.
func RequireSameOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			c.Next()
			return
		}
		if strings.EqualFold(c.GetHeader("Sec-Fetch-Site"), "cross-site") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "拒绝跨站请求"})
			return
		}
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			request, err := http.NewRequest(http.MethodGet, origin, nil)
			if err != nil || request.URL.Host == "" || !strings.EqualFold(request.URL.Host, c.Request.Host) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "请求来源不受信任"})
				return
			}
		}
		c.Next()
	}
}

func requestIsLoopback(request *http.Request) bool {
	ip := net.ParseIP(requestRemoteIP(request))
	return ip != nil && ip.IsLoopback()
}

func requestRemoteIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	return strings.TrimSpace(host)
}

func setAdminSessionCookie(c *gin.Context, token string, expiresAt time.Time, remember bool) {
	cookie := &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestUsesHTTPS(c.Request),
		SameSite: http.SameSiteStrictMode,
	}
	if remember {
		cookie.MaxAge = int(rememberedSessionDuration / time.Second)
		cookie.Expires = expiresAt
	}
	http.SetCookie(c.Writer, cookie)
}

func clearAdminSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   requestUsesHTTPS(c.Request),
		SameSite: http.SameSiteStrictMode,
	})
}

func requestUsesHTTPS(request *http.Request) bool {
	return request.TLS != nil || strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https")
}

// ProtectConfigReads prevents requests from observing partially refreshed
// global configuration maps. Reload and reset acquire the writer lock instead.
func ProtectConfigReads() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasSuffix(path, "/config/reload") || strings.HasSuffix(path, "/config/reset") {
			c.Next()
			return
		}
		config.LockForRead()
		defer config.UnlockForRead()
		c.Next()
	}
}
