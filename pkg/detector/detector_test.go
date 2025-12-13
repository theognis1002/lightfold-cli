package detector

import (
	"testing"
	"testing/fstest"
)

func TestDetectFrameworkFS_TieBreakPrefersDeterministicWinner(t *testing.T) {
	fsys := fstest.MapFS{
		"package.json": {
			Data: []byte(`{
				"dependencies": {
					"next": "14.0.0",
					"@remix-run/react": "1.0.0"
				}
			}`),
			Mode: 0o644,
		},
	}

	detection := DetectFrameworkFS(fsys)

	if detection.Framework != "Next.js" {
		t.Fatalf("expected Next.js to win tie-break, got %s", detection.Framework)
	}

	if detection.Meta["package_manager"] != "npm" {
		t.Fatalf("expected default npm package manager, got %s", detection.Meta["package_manager"])
	}
}

func TestDetectFrameworkFS_DockerComposePriority(t *testing.T) {
	// Docker Compose should win over any framework detection
	tests := []struct {
		name     string
		filename string
	}{
		{"docker-compose.yml", "docker-compose.yml"},
		{"docker-compose.yaml", "docker-compose.yaml"},
		{"compose.yml", "compose.yml"},
		{"compose.yaml", "compose.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				tt.filename: {
					Data: []byte("version: '3'\nservices:\n  app:\n    build: ."),
					Mode: 0o644,
				},
				"package.json": {
					Data: []byte(`{"dependencies": {"next": "14.0.0"}}`),
					Mode: 0o644,
				},
			}

			detection := DetectFrameworkFS(fsys)

			if detection.Framework != "Docker Compose" {
				t.Fatalf("expected Docker Compose to win over Next.js, got %s", detection.Framework)
			}

			if detection.BuildPlan[0] != "docker compose build" {
				t.Fatalf("expected 'docker compose build', got %s", detection.BuildPlan[0])
			}
		})
	}
}

func TestDetectFrameworkFS_DocusaurusFalsePositive(t *testing.T) {
	// Just having a docs/ folder should NOT trigger Docusaurus detection
	fsys := fstest.MapFS{
		"package.json": {
			Data: []byte(`{"dependencies": {"express": "4.0.0"}}`),
			Mode: 0o644,
		},
		"docs/readme.md": {
			Data: []byte("# Documentation"),
			Mode: 0o644,
		},
		"server.js": {
			Data: []byte("const express = require('express')"),
			Mode: 0o644,
		},
	}

	detection := DetectFrameworkFS(fsys)

	if detection.Framework == "Docusaurus" {
		t.Fatal("docs/ folder alone should not trigger Docusaurus detection")
	}
}

func TestDetectFrameworkFS_DocusaurusWithConfig(t *testing.T) {
	// Real Docusaurus project with config + dependency + docs should be detected
	fsys := fstest.MapFS{
		"package.json": {
			Data: []byte(`{"dependencies": {"@docusaurus/core": "2.0.0"}}`),
			Mode: 0o644,
		},
		"docusaurus.config.js": {
			Data: []byte("module.exports = {}"),
			Mode: 0o644,
		},
		"docs/intro.md": {
			Data: []byte("# Intro"),
			Mode: 0o644,
		},
	}

	detection := DetectFrameworkFS(fsys)

	if detection.Framework != "Docusaurus" {
		t.Fatalf("expected Docusaurus, got %s", detection.Framework)
	}

	// Verify all 3 signals are present
	expectedSignals := map[string]bool{
		"docusaurus config":                 false,
		"package.json has @docusaurus/core": false,
		"docs/ or blog/ directory":          false,
	}

	for _, signal := range detection.Signals {
		if _, ok := expectedSignals[signal]; ok {
			expectedSignals[signal] = true
		}
	}

	for signal, found := range expectedSignals {
		if !found {
			t.Errorf("expected signal %q not found in %v", signal, detection.Signals)
		}
	}
}

func TestDetectFrameworkFS_JSPackageManagerPriority(t *testing.T) {
	base := fstest.MapFS{
		"package.json": {
			Data: []byte(`{
				"scripts": {},
				"dependencies": {
					"next": "14.0.0"
				}
			}`),
			Mode: 0o644,
		},
	}

	tests := []struct {
		name        string
		extraFiles  map[string]*fstest.MapFile
		wantPM      string
		wantInstall string
	}{
		{
			name: "bun highest priority",
			extraFiles: map[string]*fstest.MapFile{
				"bun.lockb":      {Data: []byte(""), Mode: 0o644},
				"pnpm-lock.yaml": {Data: []byte(""), Mode: 0o644},
				"yarn.lock":      {Data: []byte(""), Mode: 0o644},
			},
			wantPM:      "bun",
			wantInstall: "bun install",
		},
		{
			name: "yarn berry preference",
			extraFiles: map[string]*fstest.MapFile{
				".yarnrc.yml":    {Data: []byte("nodeLinker: pnp"), Mode: 0o644},
				"pnpm-lock.yaml": {Data: []byte(""), Mode: 0o644},
			},
			wantPM:      "yarn-berry",
			wantInstall: "yarn install",
		},
		{
			name: "pnpm when no bun or yarn berry",
			extraFiles: map[string]*fstest.MapFile{
				"pnpm-lock.yaml": {Data: []byte(""), Mode: 0o644},
				"yarn.lock":      {Data: []byte(""), Mode: 0o644},
			},
			wantPM:      "pnpm",
			wantInstall: "pnpm install",
		},
		{
			name: "yarn classic fallback",
			extraFiles: map[string]*fstest.MapFile{
				"yarn.lock": {Data: []byte(""), Mode: 0o644},
			},
			wantPM:      "yarn",
			wantInstall: "yarn install",
		},
		{
			name:        "npm default",
			extraFiles:  map[string]*fstest.MapFile{},
			wantPM:      "npm",
			wantInstall: "npm install",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{}
			for name, file := range base {
				fsys[name] = &fstest.MapFile{Data: append([]byte(nil), file.Data...), Mode: file.Mode}
			}
			for name, file := range tt.extraFiles {
				fsys[name] = &fstest.MapFile{Data: append([]byte(nil), file.Data...), Mode: file.Mode}
			}

			detection := DetectFrameworkFS(fsys)

			if detection.Framework != "Next.js" {
				t.Fatalf("expected Next.js detection, got %s", detection.Framework)
			}

			if got := detection.Meta["package_manager"]; got != tt.wantPM {
				t.Fatalf("expected package manager %s, got %s", tt.wantPM, got)
			}

			if len(detection.BuildPlan) == 0 || detection.BuildPlan[0] != tt.wantInstall {
				t.Fatalf("expected install command %q, got %v", tt.wantInstall, detection.BuildPlan)
			}
		})
	}
}
