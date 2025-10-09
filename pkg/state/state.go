package state

import (
	"encoding/json"
	"fmt"
	"lightfold/pkg/config"
	"os"
	"path/filepath"
	"time"
)

type TargetState struct {
	LastCommit      string    `json:"last_commit,omitempty"`
	LastDeploy      time.Time `json:"last_deploy,omitempty"`
	Created         bool      `json:"created"`
	Configured      bool      `json:"configured"`
	LastRelease     string    `json:"last_release,omitempty"`
	ProvisionedID   string    `json:"provisioned_id,omitempty"`
	Builder         string    `json:"builder,omitempty"`
	SSLConfigured   bool      `json:"ssl_configured,omitempty"`
	LastSSLRenewal  time.Time `json:"last_ssl_renewal,omitempty"`
	ConfigureFailed bool      `json:"configure_failed,omitempty"`
	ConfigureError  string    `json:"configure_error,omitempty"`
	PushFailed      bool      `json:"push_failed,omitempty"`
	PushError       string    `json:"push_error,omitempty"`
	LastFailure     time.Time `json:"last_failure,omitempty"`
}

func GetStatePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(config.LocalConfigDir, config.LocalStateDir)
	}
	return filepath.Join(homeDir, config.LocalConfigDir, config.LocalStateDir)
}

func GetTargetStatePath(targetName string) string {
	return filepath.Join(GetStatePath(), targetName+".json")
}

func LoadState(targetName string) (*TargetState, error) {
	statePath := GetTargetStatePath(targetName)

	stateDir := filepath.Dir(statePath)
	if err := os.MkdirAll(stateDir, config.PermDirectory); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return &TargetState{}, nil
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state TargetState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

func SaveState(targetName string, state *TargetState) error {
	statePath := GetTargetStatePath(targetName)

	stateDir := filepath.Dir(statePath)
	if err := os.MkdirAll(stateDir, config.PermDirectory); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(statePath, data, config.PermConfigFile); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

func MarkCreated(targetName string, provisionedID string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.Created = true
	if provisionedID != "" {
		state.ProvisionedID = provisionedID
	}

	return SaveState(targetName, state)
}

func MarkConfigured(targetName string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.Configured = true
	return SaveState(targetName, state)
}

func UpdateDeployment(targetName, commitHash, releaseTimestamp string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.LastCommit = commitHash
	state.LastDeploy = time.Now()
	state.LastRelease = releaseTimestamp

	return SaveState(targetName, state)
}

func IsCreated(targetName string) bool {
	state, err := LoadState(targetName)
	if err != nil {
		return false
	}
	return state.Created
}

func IsConfigured(targetName string) bool {
	state, err := LoadState(targetName)
	if err != nil {
		return false
	}
	return state.Configured
}

func GetLastCommit(targetName string) string {
	state, err := LoadState(targetName)
	if err != nil {
		return ""
	}
	return state.LastCommit
}

func GetProvisionedID(targetName string) string {
	state, err := LoadState(targetName)
	if err != nil {
		return ""
	}
	return state.ProvisionedID
}

func UpdateBuilder(targetName, builder string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.Builder = builder
	return SaveState(targetName, state)
}

func GetTargetState(targetName string) (*TargetState, error) {
	return LoadState(targetName)
}

func DeleteState(targetName string) error {
	statePath := GetTargetStatePath(targetName)

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return nil
	}

	if err := os.Remove(statePath); err != nil {
		return fmt.Errorf("failed to delete state file: %w", err)
	}

	return nil
}

func MarkSSLConfigured(targetName string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.SSLConfigured = true
	state.LastSSLRenewal = time.Now()

	return SaveState(targetName, state)
}

func UpdateSSLRenewal(targetName string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.LastSSLRenewal = time.Now()

	return SaveState(targetName, state)
}

func IsSSLConfigured(targetName string) bool {
	state, err := LoadState(targetName)
	if err != nil {
		return false
	}
	return state.SSLConfigured
}

func GetLastSSLRenewal(targetName string) time.Time {
	state, err := LoadState(targetName)
	if err != nil {
		return time.Time{}
	}
	return state.LastSSLRenewal
}

func MarkConfigureFailed(targetName string, errMsg string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.ConfigureFailed = true
	state.ConfigureError = errMsg
	state.LastFailure = time.Now()

	return SaveState(targetName, state)
}

func MarkPushFailed(targetName string, errMsg string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.PushFailed = true
	state.PushError = errMsg
	state.LastFailure = time.Now()

	return SaveState(targetName, state)
}

func ClearConfigureFailure(targetName string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.ConfigureFailed = false
	state.ConfigureError = ""

	return SaveState(targetName, state)
}

func ClearPushFailure(targetName string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.PushFailed = false
	state.PushError = ""

	return SaveState(targetName, state)
}
