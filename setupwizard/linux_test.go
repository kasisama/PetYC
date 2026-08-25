package setupwizard

import (
	"bytes"
	"strings"
	"testing"

	"qq-pet-saas/security"
)

func passwordSequence(values ...string) PasswordReader {
	index := 0
	return func(string) (string, error) { value := values[index]; index++; return value, nil }
}

func TestInvalidLinuxWizardDoesNotCommitAdminSetup(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	credentials, err := security.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	config, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	_, err = RunLinux(config, credentials, strings.NewReader("operator\n\ninvalid-port\n"), &bytes.Buffer{}, passwordSequence("correct-password", "correct-password"))
	if err == nil {
		t.Fatal("expected invalid port error")
	}
	credentials, err = security.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if !credentials.PasswordSetupRequired {
		t.Fatal("failed wizard committed administrator password")
	}
	state, err := security.LoadOnboardingState()
	if err != nil {
		t.Fatal(err)
	}
	if state.SetupCompleted {
		t.Fatal("failed wizard marked setup complete")
	}
}

func TestLinuxWizardCanSkipPlatformAndCompletesAtomically(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	credentials, err := security.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	config, err := security.LoadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	updated, err := RunLinux(config, credentials, strings.NewReader("operator\n\n8088\n0\n"), &output, passwordSequence("correct-password", "correct-password"))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Port != 8088 {
		t.Fatalf("want port 8088, got %d", updated.Port)
	}
	ok, err := security.VerifyAdminCredentials("operator", "correct-password")
	if err != nil || !ok {
		t.Fatalf("admin credentials not saved: %v %v", ok, err)
	}
	state, err := security.LoadOnboardingState()
	if err != nil || !state.SetupCompleted {
		t.Fatalf("setup not complete: %+v %v", state, err)
	}
	if strings.Contains(output.String(), credentials.WebSocketToken) {
		t.Fatal("skipped OneBot flow printed token")
	}
}
