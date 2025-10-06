package state

import (
	"encoding/json"
	"fmt"
	"lightfold/pkg/config"
	"os"
	"path/filepath"
	"time"
)

// TargetState tracks the deployment state for a specific target
type TargetState struct {
	LastCommit    string    `json:"last_commit,omitempty"`
	LastDeploy    time.Time `json:"last_deploy,omitempty"`
	Created       bool      `json:"created"`
	Configured    bool      `json:"configured"`
	LastRelease   string    `json:"last_release,omitempty"`
	ProvisionedID string    `json:"provisioned_id,omitempty"`
	Builder       string    `json:"builder,omitempty"`
}

// GetStatePath returns the path to the state directory
func GetStatePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(config.LocalConfigDir, config.LocalStateDir)
	}
	return filepath.Join(homeDir, config.LocalConfigDir, config.LocalStateDir)
}

// GetTargetStatePath returns the path to a specific target's state file
func GetTargetStatePath(targetName string) string {
	return filepath.Join(GetStatePath(), targetName+".json")
}

// LoadState loads the state for a specific target
func LoadState(targetName string) (*TargetState, error) {
	statePath := GetTargetStatePath(targetName)

	// Ensure state directory exists
	stateDir := filepath.Dir(statePath)
	if err := os.MkdirAll(stateDir, config.PermDirectory); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	// If state file doesn't exist, return empty state
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

// SaveState saves the state for a specific target
func SaveState(targetName string, state *TargetState) error {
	statePath := GetTargetStatePath(targetName)

	// Ensure state directory exists
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

// MarkCreated marks a target as created
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

// MarkConfigured marks a target as configured
func MarkConfigured(targetName string) error {
	state, err := LoadState(targetName)
	if err != nil {
		return err
	}

	state.Configured = true
	return SaveState(targetName, state)
}

// UpdateDeployment updates the deployment state after a successful push
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

// IsCreated checks if a target has been created
func IsCreated(targetName string) bool {
	state, err := LoadState(targetName)
	if err != nil {
		return false
	}
	return state.Created
}

// IsConfigured checks if a target has been configured
func IsConfigured(targetName string) bool {
	state, err := LoadState(targetName)
	if err != nil {
		return false
	}
	return state.Configured
}

// GetLastCommit returns the last deployed commit hash
func GetLastCommit(targetName string) string {
	state, err := LoadState(targetName)
	if err != nil {
		return ""
	}
	return state.LastCommit
}

// GetProvisionedID returns the provisioned server ID for a target
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

// DeleteState removes the state file for a specific target
func DeleteState(targetName string) error {
	statePath := GetTargetStatePath(targetName)

	// Check if state file exists
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return nil // Already deleted, no error
	}

	if err := os.Remove(statePath); err != nil {
		return fmt.Errorf("failed to delete state file: %w", err)
	}

	return nil
}
