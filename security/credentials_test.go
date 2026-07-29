package security

import (
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestLoadCredentialsGeneratesAndPersistsAdminLoginAndWebSocketToken(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQPET_WS_TOKEN", "")

	first, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if first.AdminUsername != defaultAdminUsername {
		t.Fatalf("AdminUsername = %q, want %q", first.AdminUsername, defaultAdminUsername)
	}
	if first.AdminPasswordHash == "" {
		t.Fatal("AdminPasswordHash is empty")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(first.AdminPasswordHash), []byte(defaultAdminPassword)); err != nil {
		t.Fatalf("default password does not match stored hash: %v", err)
	}
	if first.WebSocketToken == "" {
		t.Fatal("WebSocketToken is empty")
	}
	if _, err := filepath.Abs(CredentialsPath()); err != nil {
		t.Fatalf("CredentialsPath() must be a valid path: %v", err)
	}

	second, err := LoadCredentials()
	if err != nil {
		t.Fatalf("second LoadCredentials() error = %v", err)
	}
	if second != first {
		t.Fatalf("LoadCredentials() did not reuse persisted credentials")
	}
}

func TestLoadCredentialsEnvironmentOverridesPersistedTokens(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQPET_WS_TOKEN", "ws-from-env")

	credentials, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if credentials.WebSocketToken != "ws-from-env" {
		t.Fatalf("LoadCredentials() = %#v, want environment values", credentials)
	}
}

func TestVerifyAdminCredentialsAcceptsDefaultAndRejectsWrongValues(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQPET_WS_TOKEN", "")

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{name: "default login", username: "admin", password: "123456", want: true},
		{name: "wrong username", username: "other", password: "123456", want: false},
		{name: "wrong password", username: "admin", password: "wrong", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ok, err := VerifyAdminCredentials(test.username, test.password)
			if err != nil {
				t.Fatalf("VerifyAdminCredentials() error = %v", err)
			}
			if ok != test.want {
				t.Fatalf("VerifyAdminCredentials() = %v, want %v", ok, test.want)
			}
		})
	}
}

func TestChangeAdminPasswordInvalidatesOldPassword(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQPET_WS_TOKEN", "")

	if err := ChangeAdminPassword("123456", "new-secret"); err != nil {
		t.Fatalf("ChangeAdminPassword() error = %v", err)
	}

	oldOK, err := VerifyAdminCredentials("admin", "123456")
	if err != nil {
		t.Fatalf("verify old password error = %v", err)
	}
	newOK, err := VerifyAdminCredentials("admin", "new-secret")
	if err != nil {
		t.Fatalf("verify new password error = %v", err)
	}
	if oldOK || !newOK {
		t.Fatalf("old password valid = %v, new password valid = %v", oldOK, newOK)
	}
}

func TestChangeAdminPasswordRejectsWrongCurrentPassword(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQPET_WS_TOKEN", "")

	err := ChangeAdminPassword("wrong", "new-secret")
	if err != ErrInvalidAdminPassword {
		t.Fatalf("ChangeAdminPassword() error = %v, want %v", err, ErrInvalidAdminPassword)
	}
}
