// Package security manages the local credentials used by the self-hosted app.
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const (
	credentialsFileName  = "credentials.json"
	defaultAdminUsername = "admin"
	legacyAdminPassword  = "123456"
)

type Credentials struct {
	AdminUsername         string `json:"admin_username"`
	AdminPasswordHash     string `json:"admin_password_hash"`
	PasswordSetupRequired bool   `json:"password_setup_required"`
	WebSocketToken        string `json:"websocket_token"`
}

var (
	credentialsMu            sync.Mutex
	ErrInvalidAdminPassword  = errors.New("invalid admin password")
	ErrAdminSetupNotRequired = errors.New("admin password setup is not required")
)

func dataDir() (string, error) {
	if dir := os.Getenv("QQPET_DATA_DIR"); dir != "" {
		return dir, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "qq-pet-saas"), nil
}

// DataDir returns the application data directory (credentials, sessions, etc.).
func DataDir() (string, error) {
	return dataDir()
}

// CredentialsPath returns the location of the generated credentials file.
func CredentialsPath() string {
	dir, err := dataDir()
	if err != nil {
		return credentialsFileName
	}
	return filepath.Join(dir, credentialsFileName)
}

// LoadCredentials loads persisted credentials and initializes missing fields.
func LoadCredentials() (Credentials, error) {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()
	return loadCredentialsLocked()
}

func loadCredentialsLocked() (Credentials, error) {
	dir, err := dataDir()
	if err != nil {
		return Credentials{}, fmt.Errorf("resolve application data directory: %w", err)
	}
	path := filepath.Join(dir, credentialsFileName)

	var credentials Credentials
	if raw, readErr := os.ReadFile(path); readErr == nil {
		if err := json.Unmarshal(raw, &credentials); err != nil {
			return Credentials{}, fmt.Errorf("read credentials file: %w", err)
		}
	} else if !os.IsNotExist(readErr) {
		return Credentials{}, fmt.Errorf("read credentials file: %w", readErr)
	}

	changed := false
	if credentials.AdminUsername == "" {
		credentials.AdminUsername = defaultAdminUsername
		changed = true
	}
	if credentials.AdminPasswordHash == "" {
		oneTimeSecret, tokenErr := generateToken()
		if tokenErr != nil {
			return Credentials{}, tokenErr
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(oneTimeSecret), bcrypt.DefaultCost)
		if hashErr != nil {
			return Credentials{}, fmt.Errorf("hash initial admin secret: %w", hashErr)
		}
		credentials.AdminPasswordHash = string(hash)
		credentials.PasswordSetupRequired = true
		changed = true
	} else if !credentials.PasswordSetupRequired && bcrypt.CompareHashAndPassword(
		[]byte(credentials.AdminPasswordHash), []byte(legacyAdminPassword),
	) == nil {
		// 旧版本公开默认密码不能继续作为可登录凭据。升级时立即使其失效，
		// 并要求管理员通过本机初始化向导设置自己的密码。
		oneTimeSecret, tokenErr := generateToken()
		if tokenErr != nil {
			return Credentials{}, tokenErr
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(oneTimeSecret), bcrypt.DefaultCost)
		if hashErr != nil {
			return Credentials{}, fmt.Errorf("replace legacy admin password: %w", hashErr)
		}
		credentials.AdminPasswordHash = string(hash)
		credentials.PasswordSetupRequired = true
		changed = true
	}
	if credentials.WebSocketToken == "" {
		credentials.WebSocketToken, err = generateToken()
		if err != nil {
			return Credentials{}, err
		}
		changed = true
	}

	if changed {
		if err := writeCredentialsFile(dir, path, credentials); err != nil {
			return Credentials{}, err
		}
	}

	return credentials, nil
}

// VerifyAdminCredentials checks a username and password against the persisted
// administrator account without exposing which value was incorrect.
func VerifyAdminCredentials(username, password string) (bool, error) {
	credentials, err := LoadCredentials()
	if err != nil {
		return false, err
	}
	if credentials.PasswordSetupRequired {
		return false, nil
	}
	passwordMatches := bcrypt.CompareHashAndPassword(
		[]byte(credentials.AdminPasswordHash),
		[]byte(password),
	) == nil
	usernameMatches := subtle.ConstantTimeCompare(
		[]byte(username),
		[]byte(credentials.AdminUsername),
	) == 1
	return usernameMatches && passwordMatches, nil
}

// SetInitialAdminPassword completes the one-time local setup flow. It cannot
// overwrite an already initialized administrator account.
func SetInitialAdminPassword(username, password string) error {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()

	credentials, err := loadCredentialsLocked()
	if err != nil {
		return err
	}
	if !credentials.PasswordSetupRequired {
		return ErrAdminSetupNotRequired
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash initial admin password: %w", err)
	}
	if username != "" {
		credentials.AdminUsername = username
	}
	credentials.AdminPasswordHash = string(hash)
	credentials.PasswordSetupRequired = false
	dir, err := dataDir()
	if err != nil {
		return fmt.Errorf("resolve application data directory: %w", err)
	}
	return writeCredentialsFile(dir, filepath.Join(dir, credentialsFileName), credentials)
}

// ChangeAdminPassword verifies the existing password and atomically persists
// a bcrypt hash for the replacement password.
func ChangeAdminPassword(currentPassword, newPassword string) error {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()

	credentials, err := loadCredentialsLocked()
	if err != nil {
		return err
	}
	if credentials.PasswordSetupRequired {
		return ErrInvalidAdminPassword
	}
	if bcrypt.CompareHashAndPassword(
		[]byte(credentials.AdminPasswordHash),
		[]byte(currentPassword),
	) != nil {
		return ErrInvalidAdminPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash new admin password: %w", err)
	}
	credentials.AdminPasswordHash = string(hash)

	dir, err := dataDir()
	if err != nil {
		return fmt.Errorf("resolve application data directory: %w", err)
	}
	if err := writeCredentialsFile(
		dir,
		filepath.Join(dir, credentialsFileName),
		credentials,
	); err != nil {
		return err
	}
	return nil
}

func writeCredentialsFile(dir, path string, credentials Credentials) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create application data directory: %w", err)
	}
	payload, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("encode credentials file: %w", err)
	}

	temp, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary credentials file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("secure temporary credentials file: %w", err)
	}
	if _, err := temp.Write(payload); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary credentials file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary credentials file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary credentials file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace credentials file: %w", err)
	}
	return nil
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
