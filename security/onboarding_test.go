package security

import "testing"

func TestOnboardingStatePersistsAndTourVersionIsMonotonic(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	if _, err := LoadCredentials(); err != nil {
		t.Fatal(err)
	}
	state, err := LoadOnboardingState()
	if err != nil {
		t.Fatal(err)
	}
	if state.SetupCompleted {
		t.Fatal("fresh install must require setup")
	}
	if err = CompleteSetup(); err != nil {
		t.Fatal(err)
	}
	if err = CompleteTour(3); err != nil {
		t.Fatal(err)
	}
	if err = CompleteTour(2); err != nil {
		t.Fatal(err)
	}
	state, err = LoadOnboardingState()
	if err != nil {
		t.Fatal(err)
	}
	if !state.SetupCompleted || state.TourVersionCompleted != 3 {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestExistingPasswordInfersCompletedSetup(t *testing.T) {
	t.Setenv("QQPET_DATA_DIR", t.TempDir())
	credentials, err := LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if !credentials.PasswordSetupRequired {
		t.Fatal("fresh credentials should require setup")
	}
	if err = SetInitialAdminPassword("operator", "strong-password"); err != nil {
		t.Fatal(err)
	}
	state, err := LoadOnboardingState()
	if err != nil {
		t.Fatal(err)
	}
	if !state.SetupCompleted {
		t.Fatal("existing initialized administrator should not be forced through setup")
	}
}
