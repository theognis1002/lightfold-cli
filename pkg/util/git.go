package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// IsGitRepository checks if the given path is a Git repository
func IsGitRepository(projectPath string) bool {
	gitDir := filepath.Join(projectPath, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GetGitRemoteURL returns the remote origin URL for the Git repository
func GetGitRemoteURL(projectPath string) (string, error) {
	if !IsGitRepository(projectPath) {
		return "", fmt.Errorf("not a git repository: %s", projectPath)
	}

	cmd := exec.Command("git", "-C", projectPath, "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git remote URL: %w", err)
	}

	remoteURL := strings.TrimSpace(string(output))
	if remoteURL == "" {
		return "", fmt.Errorf("no remote origin URL configured")
	}

	return remoteURL, nil
}

// ParseGitHubRepo extracts the organization and repository name from a GitHub URL
// Supports both SSH (git@github.com:org/repo.git) and HTTPS (https://github.com/org/repo.git) formats
func ParseGitHubRepo(remoteURL string) (org, repo string, err error) {
	// Trim whitespace
	remoteURL = strings.TrimSpace(remoteURL)

	// SSH format: git@github.com:org/repo.git
	sshRegex := regexp.MustCompile(`^git@github\.com:([^/]+)/(.+?)(\.git)?$`)
	if matches := sshRegex.FindStringSubmatch(remoteURL); len(matches) >= 3 {
		return matches[1], strings.TrimSuffix(matches[2], ".git"), nil
	}

	// HTTPS format: https://github.com/org/repo.git or https://github.com/org/repo
	httpsRegex := regexp.MustCompile(`^https://github\.com/([^/]+)/(.+?)(\.git)?$`)
	if matches := httpsRegex.FindStringSubmatch(remoteURL); len(matches) >= 3 {
		return matches[1], strings.TrimSuffix(matches[2], ".git"), nil
	}

	return "", "", fmt.Errorf("not a valid GitHub repository URL: %s", remoteURL)
}

// GetGitHubRepo returns the organization and repository name for the project
func GetGitHubRepo(projectPath string) (org, repo string, err error) {
	remoteURL, err := GetGitRemoteURL(projectPath)
	if err != nil {
		return "", "", err
	}

	return ParseGitHubRepo(remoteURL)
}
