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
	defaultAdminPassword = "123456"
)

type Credentials struct {
	AdminUsername     string `json:"admin_username"`
	AdminPasswordHash string `json:"admin_password_hash"`
	WebSocketToken    string `json:"websocket_token"`
}

var (
	credentialsMu           sync.Mutex
	ErrInvalidAdminPassword = errors.New("invalid admin password")
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

// CredentialsPath returns the location of the generated credentials file.
func CredentialsPath() string {
	dir, err := dataDir()
	if err != nil {
		return credentialsFileName
	}
	return filepath.Join(dir, credentialsFileName)
}

// LoadCredentials loads persisted credentials, initializes missing admin
// fields, and lets QQPET_WS_TOKEN override the local WebSocket token.
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

	if token := os.Getenv("QQPET_WS_TOKEN"); token != "" {
		credentials.WebSocketToken = token
	}

	changed := false
	if credentials.AdminUsername == "" {
		credentials.AdminUsername = defaultAdminUsername
		changed = true
	}
	if credentials.AdminPasswordHash == "" {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), bcrypt.DefaultCost)
		if hashErr != nil {
			return Credentials{}, fmt.Errorf("hash default admin password: %w", hashErr)
		}
		credentials.AdminPasswordHash = string(hash)
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

// ChangeAdminPassword verifies the existing password and atomically persists
// a bcrypt hash for the replacement password.
func ChangeAdminPassword(currentPassword, newPassword string) error {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()

	credentials, err := loadCredentialsLocked()
	if err != nil {
		return err
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
