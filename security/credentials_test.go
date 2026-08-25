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
	if !first.PasswordSetupRequired {
		t.Fatal("new credentials must require initial password setup")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(first.AdminPasswordHash), []byte(legacyAdminPassword)); err == nil {
		t.Fatal("public legacy password must not match a fresh credentials file")
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

func TestLoadCredentialsEnvironmentIsOnlyUsedByRuntimeConfigImport(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQPET_WS_TOKEN", "ws-from-env")

	runtimeConfig, err := LoadRuntimeConfig()
	if err != nil {
		t.Fatalf("LoadRuntimeConfig() error = %v", err)
	}
	credentials, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if runtimeConfig.OneBotToken != "ws-from-env" {
		t.Fatalf("runtime config token = %q", runtimeConfig.OneBotToken)
	}
	if credentials.WebSocketToken != "ws-from-env" {
		t.Fatalf("LoadCredentials() = %#v, want imported runtime value", credentials)
	}
}

func TestVerifyAdminCredentialsRequiresSetupAndThenAcceptsConfiguredPassword(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	t.Setenv("QQPET_WS_TOKEN", "")
	if ok, err := VerifyAdminCredentials("admin", legacyAdminPassword); err != nil || ok {
		t.Fatalf("credentials before setup = %v, %v; want rejected", ok, err)
	}
	if err := SetInitialAdminPassword("admin", "configured-secret"); err != nil {
		t.Fatalf("SetInitialAdminPassword() error = %v", err)
	}

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{name: "configured login", username: "admin", password: "configured-secret", want: true},
		{name: "wrong username", username: "other", password: "configured-secret", want: false},
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
	if err := SetInitialAdminPassword("admin", "configured-secret"); err != nil {
		t.Fatal(err)
	}

	if err := ChangeAdminPassword("configured-secret", "new-secret"); err != nil {
		t.Fatalf("ChangeAdminPassword() error = %v", err)
	}

	oldOK, err := VerifyAdminCredentials("admin", "configured-secret")
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
	if err := SetInitialAdminPassword("admin", "configured-secret"); err != nil {
		t.Fatal(err)
	}

	err := ChangeAdminPassword("wrong", "new-secret")
	if err != ErrInvalidAdminPassword {
		t.Fatalf("ChangeAdminPassword() error = %v, want %v", err, ErrInvalidAdminPassword)
	}
}
