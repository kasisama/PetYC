package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	minAdminPasswordLength    = 6
	maxAdminPasswordBytes     = 72
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
}

func NewAuthHandler(sessions *SessionManager) *AuthHandler {
	return &AuthHandler{sessions: sessions}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

func (handler *AuthHandler) Login(c *gin.Context) {
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号或密码错误"})
		return
	}

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
	token, err := c.Cookie(adminSessionCookieName)
	if err != nil || !handler.sessions.Validate(token) {
		if token != "" {
			clearAdminSessionCookie(c)
		}
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}

	credentials, err := security.LoadCredentials()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "管理员凭据暂时不可用"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"username":      credentials.AdminUsername,
	})
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
	if utf8.RuneCountInString(request.NewPassword) < minAdminPasswordLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码至少需要 6 个字符"})
		return
	}
	if len(request.NewPassword) > maxAdminPasswordBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码不能超过 72 个字节"})
		return
	}
	if request.NewPassword != request.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "两次输入的新密码不一致"})
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
		if strings.HasSuffix(path, "/configs/reload") || strings.HasSuffix(path, "/configs/reset") {
			c.Next()
			return
		}
		config.LockForRead()
		defer config.UnlockForRead()
		c.Next()
	}
}
