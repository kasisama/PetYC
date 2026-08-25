package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const onboardingFileName = "onboarding.json"

type OnboardingState struct {
	SetupCompleted       bool `json:"setup_completed"`
	TourVersionCompleted int  `json:"tour_version_completed"`
}

var onboardingMu sync.Mutex

func LoadOnboardingState() (OnboardingState, error) {
	onboardingMu.Lock()
	defer onboardingMu.Unlock()
	dir, err := dataDir()
	if err != nil {
		return OnboardingState{}, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, onboardingFileName))
	if err == nil {
		var state OnboardingState
		if err = json.Unmarshal(raw, &state); err != nil {
			return state, fmt.Errorf("read onboarding state: %w", err)
		}
		return state, nil
	}
	if !os.IsNotExist(err) {
		return OnboardingState{}, err
	}
	credentials, err := loadCredentialsUnlocked(dir)
	if err != nil {
		return OnboardingState{}, err
	}
	// Existing installations with a real password must not be forced through
	// first-run setup after upgrading.
	return OnboardingState{SetupCompleted: !credentials.PasswordSetupRequired}, nil
}

func SaveOnboardingState(state OnboardingState) error {
	onboardingMu.Lock()
	defer onboardingMu.Unlock()
	dir, err := dataDir()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".onboarding-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err = temp.Chmod(0o600); err == nil {
		_, err = temp.Write(raw)
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(tempPath, filepath.Join(dir, onboardingFileName))
}

func CompleteSetup() error {
	state, err := LoadOnboardingState()
	if err != nil {
		return err
	}
	state.SetupCompleted = true
	return SaveOnboardingState(state)
}
func CompleteTour(version int) error {
	state, err := LoadOnboardingState()
	if err != nil {
		return err
	}
	if version > state.TourVersionCompleted {
		state.TourVersionCompleted = version
	}
	return SaveOnboardingState(state)
}

// loadCredentialsUnlocked mirrors the read-only portion of credential loading
// without taking credentialsMu while onboardingMu is held.
func loadCredentialsUnlocked(dir string) (Credentials, error) {
	var credentials Credentials
	raw, err := os.ReadFile(filepath.Join(dir, credentialsFileName))
	if err == nil {
		if err = json.Unmarshal(raw, &credentials); err != nil {
			return credentials, err
		}
		return credentials, nil
	}
	if os.IsNotExist(err) {
		return Credentials{PasswordSetupRequired: true}, nil
	}
	return credentials, err
}
