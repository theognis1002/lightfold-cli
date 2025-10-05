package util

import (
	"testing"
)

func TestParseGitHubRepo(t *testing.T) {
	tests := []struct {
		name        string
		remoteURL   string
		expectedOrg string
		expectedRepo string
		expectError bool
	}{
		{
			name:         "SSH format with .git",
			remoteURL:    "git@github.com:theognis1002/lightfold-cli.git",
			expectedOrg:  "theognis1002",
			expectedRepo: "lightfold-cli",
			expectError:  false,
		},
		{
			name:         "SSH format without .git",
			remoteURL:    "git@github.com:theognis1002/lightfold-cli",
			expectedOrg:  "theognis1002",
			expectedRepo: "lightfold-cli",
			expectError:  false,
		},
		{
			name:         "HTTPS format with .git",
			remoteURL:    "https://github.com/theognis1002/lightfold-cli.git",
			expectedOrg:  "theognis1002",
			expectedRepo: "lightfold-cli",
			expectError:  false,
		},
		{
			name:         "HTTPS format without .git",
			remoteURL:    "https://github.com/theognis1002/lightfold-cli",
			expectedOrg:  "theognis1002",
			expectedRepo: "lightfold-cli",
			expectError:  false,
		},
		{
			name:        "Invalid URL",
			remoteURL:   "https://gitlab.com/user/repo.git",
			expectError: true,
		},
		{
			name:        "Empty URL",
			remoteURL:   "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, repo, err := ParseGitHubRepo(tt.remoteURL)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if org != tt.expectedOrg {
				t.Errorf("Expected org %s, got %s", tt.expectedOrg, org)
			}

			if repo != tt.expectedRepo {
				t.Errorf("Expected repo %s, got %s", tt.expectedRepo, repo)
			}
		})
	}
}

func TestIsGitRepository(t *testing.T) {
	// Test project root (should be a git repo)
	if !IsGitRepository("../..") {
		t.Error("Expected project root to be a git repository")
	}

	// Test a non-existent directory
	if IsGitRepository("/tmp/non-existent-dir-12345") {
		t.Error("Expected non-existent directory to not be a git repository")
	}
}
