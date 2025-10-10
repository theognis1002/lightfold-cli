package utils_test

import (
	"os"
	"path/filepath"
	"testing"

	"lightfold/cmd/utils"
	"lightfold/pkg/config"
	"lightfold/pkg/util"
)

func TestResolveTarget_UsesTargetFlag(t *testing.T) {
	projectDir := t.TempDir()

	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"demo": {
				ProjectPath: projectDir,
				Framework:   "Next.js",
				Provider:    "digitalocean",
			},
		},
	}

	target, name, err := utils.ResolveTarget(cfg, "demo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if name != "demo" {
		t.Fatalf("expected target name 'demo', got %s", name)
	}

	if target.ProjectPath != projectDir {
		t.Fatalf("expected project path %s, got %s", projectDir, target.ProjectPath)
	}
}

func TestResolveTarget_ResolvesByPathArgument(t *testing.T) {
	projectDir := t.TempDir()

	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"demo": {
				ProjectPath: projectDir,
				Framework:   "Next.js",
				Provider:    "digitalocean",
			},
		},
	}

	target, name, err := utils.ResolveTarget(cfg, "", projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if name != "demo" {
		t.Fatalf("expected target name 'demo', got %s", name)
	}

	if target.ProjectPath != projectDir {
		t.Fatalf("expected project path %s, got %s", projectDir, target.ProjectPath)
	}
}

func TestResolveTarget_DefaultsToCurrentDirectory(t *testing.T) {
	projectDir := t.TempDir()

	// Resolve symlinks to get canonical path (important on macOS where /var is a symlink to /private/var)
	canonicalPath, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		canonicalPath = projectDir // Fallback if EvalSymlinks fails
	}
	absPath, err := filepath.Abs(canonicalPath)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			"demo": {
				ProjectPath: absPath,
				Framework:   "Next.js",
				Provider:    "digitalocean",
			},
		},
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	target, name, err := utils.ResolveTarget(cfg, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if name != "demo" {
		t.Fatalf("expected target name 'demo', got %s", name)
	}

	if target.ProjectPath != absPath {
		t.Fatalf("expected project path %s, got %s", absPath, target.ProjectPath)
	}
}

func TestResolveTarget_FallsBackToSanitizedName(t *testing.T) {
	projectDir := t.TempDir()
	sanitized := util.GetTargetName(projectDir)

	cfg := &config.Config{
		Targets: map[string]config.TargetConfig{
			sanitized: {
				ProjectPath: filepath.Join(t.TempDir(), "archived-path"),
				Framework:   "Next.js",
				Provider:    "digitalocean",
			},
		},
	}

	target, name, err := utils.ResolveTarget(cfg, "", projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if name != sanitized {
		t.Fatalf("expected target name '%s', got %s", sanitized, name)
	}

	if target.Framework != "Next.js" {
		t.Fatalf("expected framework to remain Next.js, got %s", target.Framework)
	}
}
