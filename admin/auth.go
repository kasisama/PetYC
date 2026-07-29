package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"qq-pet-saas/config"
	"qq-pet-saas/security"
)

const (
	adminSessionCookieName    = "qqpet_admin_session"
	rememberedSessionDuration = 15 * 24 * time.Hour
	minAdminPasswordLength    = 6
	maxAdminPasswordBytes     = 72
)

type adminSession struct {
	expiresAt time.Time
}

// SessionManager owns the opaque sessions used by the admin UI. Only token
// digests are retained by the server; raw tokens stay in HttpOnly cookies.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[[sha256.Size]byte]adminSession
	now      func() time.Time
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[[sha256.Size]byte]adminSession),
		now:      time.Now,
	}
}

func (manager *SessionManager) Create(_ bool) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	now := manager.now()
	expiresAt := now.Add(rememberedSessionDuration)

	manager.mu.Lock()
	for storedDigest, session := range manager.sessions {
		if now.After(session.expiresAt) {
			delete(manager.sessions, storedDigest)
		}
	}
	manager.sessions[digest] = adminSession{expiresAt: expiresAt}
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
	delete(manager.sessions, digest)
	manager.mu.Unlock()
}

func (manager *SessionManager) DeleteAll() {
	manager.mu.Lock()
	manager.sessions = make(map[[sha256.Size]byte]adminSession)
	manager.mu.Unlock()
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
