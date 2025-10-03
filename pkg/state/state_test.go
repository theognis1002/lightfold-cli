package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestStateDir creates a temporary directory for testing
// and sets the HOME env var to use it
func setupTestStateDir(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "lightfold-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Override HOME environment variable
	originalHome := os.Getenv("HOME")
	testHome := filepath.Join(tmpDir, "home")
	os.Setenv("HOME", testHome)

	cleanup := func() {
		os.Setenv("HOME", originalHome)
		os.RemoveAll(tmpDir)
	}

	return testHome, cleanup
}

func TestLoadState(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "test-target"

	// Test loading non-existent state (should return empty state)
	state, err := LoadState(targetName)
	if err != nil {
		t.Fatalf("Expected no error for non-existent state, got: %v", err)
	}
	if state.Created {
		t.Error("Expected Created to be false for new state")
	}
	if state.Configured {
		t.Error("Expected Configured to be false for new state")
	}
}

func TestSaveAndLoadState(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "test-target"

	// Create and save state
	state := &TargetState{
		Created:       true,
		Configured:    true,
		LastCommit:    "abc123",
		LastDeploy:    time.Now(),
		LastRelease:   "20231003120000",
		ProvisionedID: "droplet-12345",
	}

	if err := SaveState(targetName, state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Load and verify
	loadedState, err := LoadState(targetName)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if !loadedState.Created {
		t.Error("Expected Created to be true")
	}
	if !loadedState.Configured {
		t.Error("Expected Configured to be true")
	}
	if loadedState.LastCommit != "abc123" {
		t.Errorf("Expected LastCommit 'abc123', got '%s'", loadedState.LastCommit)
	}
	if loadedState.ProvisionedID != "droplet-12345" {
		t.Errorf("Expected ProvisionedID 'droplet-12345', got '%s'", loadedState.ProvisionedID)
	}
}

func TestMarkCreated(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "test-target"

	// Mark as created
	if err := MarkCreated(targetName, "server-123"); err != nil {
		t.Fatalf("Failed to mark created: %v", err)
	}

	// Verify state
	state, err := LoadState(targetName)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if !state.Created {
		t.Error("Expected Created to be true")
	}
	if state.ProvisionedID != "server-123" {
		t.Errorf("Expected ProvisionedID 'server-123', got '%s'", state.ProvisionedID)
	}
}

func TestMarkConfigured(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "test-target"

	// Mark as configured
	if err := MarkConfigured(targetName); err != nil {
		t.Fatalf("Failed to mark configured: %v", err)
	}

	// Verify state
	state, err := LoadState(targetName)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if !state.Configured {
		t.Error("Expected Configured to be true")
	}
}

func TestUpdateDeployment(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "test-target"
	commitHash := "abc123def456"
	releaseTimestamp := "20231003120000"

	// Update deployment
	if err := UpdateDeployment(targetName, commitHash, releaseTimestamp); err != nil {
		t.Fatalf("Failed to update deployment: %v", err)
	}

	// Verify state
	state, err := LoadState(targetName)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if state.LastCommit != commitHash {
		t.Errorf("Expected LastCommit '%s', got '%s'", commitHash, state.LastCommit)
	}
	if state.LastRelease != releaseTimestamp {
		t.Errorf("Expected LastRelease '%s', got '%s'", releaseTimestamp, state.LastRelease)
	}
	if state.LastDeploy.IsZero() {
		t.Error("Expected LastDeploy to be set")
	}
}

func TestIsCreated(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "test-target"

	// Should be false initially
	if IsCreated(targetName) {
		t.Error("Expected IsCreated to be false initially")
	}

	// Mark as created
	if err := MarkCreated(targetName, ""); err != nil {
		t.Fatalf("Failed to mark created: %v", err)
	}

	// Should be true now
	if !IsCreated(targetName) {
		t.Error("Expected IsCreated to be true after marking")
	}
}

func TestIsConfigured(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "test-target"

	// Should be false initially
	if IsConfigured(targetName) {
		t.Error("Expected IsConfigured to be false initially")
	}

	// Mark as configured
	if err := MarkConfigured(targetName); err != nil {
		t.Fatalf("Failed to mark configured: %v", err)
	}

	// Should be true now
	if !IsConfigured(targetName) {
		t.Error("Expected IsConfigured to be true after marking")
	}
}

func TestGetLastCommit(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "test-target"

	// Should return empty string initially
	if commit := GetLastCommit(targetName); commit != "" {
		t.Errorf("Expected empty commit, got '%s'", commit)
	}

	// Update deployment
	expectedCommit := "abc123def456"
	if err := UpdateDeployment(targetName, expectedCommit, "20231003120000"); err != nil {
		t.Fatalf("Failed to update deployment: %v", err)
	}

	// Should return the commit now
	if commit := GetLastCommit(targetName); commit != expectedCommit {
		t.Errorf("Expected commit '%s', got '%s'", expectedCommit, commit)
	}
}

func TestGetProvisionedID(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "test-target"

	// Should return empty string initially
	if id := GetProvisionedID(targetName); id != "" {
		t.Errorf("Expected empty provisioned ID, got '%s'", id)
	}

	// Mark as created with provisioned ID
	expectedID := "droplet-12345"
	if err := MarkCreated(targetName, expectedID); err != nil {
		t.Fatalf("Failed to mark created: %v", err)
	}

	// Should return the ID now
	if id := GetProvisionedID(targetName); id != expectedID {
		t.Errorf("Expected provisioned ID '%s', got '%s'", expectedID, id)
	}
}

func TestDeleteState(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "test-target"

	// Create some state
	if err := MarkCreated(targetName, "server-123"); err != nil {
		t.Fatalf("Failed to mark created: %v", err)
	}

	// Verify state file exists
	statePath := GetTargetStatePath(targetName)
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("State file should exist")
	}

	// Delete state
	if err := DeleteState(targetName); err != nil {
		t.Fatalf("Failed to delete state: %v", err)
	}

	// Verify state file is deleted
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("State file should be deleted")
	}

	// Should be idempotent - deleting again should not error
	if err := DeleteState(targetName); err != nil {
		t.Errorf("DeleteState should be idempotent, got error: %v", err)
	}
}

func TestDeleteStateIdempotent(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "non-existent-target"

	// Deleting non-existent state should not error
	if err := DeleteState(targetName); err != nil {
		t.Errorf("Expected no error deleting non-existent state, got: %v", err)
	}
}

func TestGetTargetStatePath(t *testing.T) {
	testHome, cleanup := setupTestStateDir(t)
	defer cleanup()

	targetName := "my-app-prod"
	expectedPath := filepath.Join(testHome, ".lightfold", "state", "my-app-prod.json")

	actualPath := GetTargetStatePath(targetName)
	if actualPath != expectedPath {
		t.Errorf("Expected path '%s', got '%s'", expectedPath, actualPath)
	}
}

func TestMultipleTargets(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	// Create multiple targets
	targets := []string{"target-1", "target-2", "target-3"}

	for _, target := range targets {
		if err := MarkCreated(target, "server-"+target); err != nil {
			t.Fatalf("Failed to create state for %s: %v", target, err)
		}
	}

	// Verify all exist
	for _, target := range targets {
		if !IsCreated(target) {
			t.Errorf("Expected %s to be created", target)
		}
		if id := GetProvisionedID(target); id != "server-"+target {
			t.Errorf("Expected ID 'server-%s', got '%s'", target, id)
		}
	}

	// Delete one target
	if err := DeleteState("target-2"); err != nil {
		t.Fatalf("Failed to delete target-2: %v", err)
	}

	// Verify target-2 is gone, others remain
	if IsCreated("target-2") {
		t.Error("Expected target-2 to be deleted")
	}
	if !IsCreated("target-1") || !IsCreated("target-3") {
		t.Error("Expected target-1 and target-3 to still exist")
	}
}
