package detector_test

import (
	"strings"
	"testing"

	"lightfold/pkg/detector"
)

func TestPackageManagerDetection(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		testFunc func(string) string
		expected string
	}{
		{
			name:     "Bun detection",
			files:    map[string]string{"bun.lockb": "binary content", "package.json": "{}"},
			testFunc: detector.DetectPackageManager,
			expected: "bun",
		},
		{
			name:     "pnpm detection",
			files:    map[string]string{"pnpm-lock.yaml": "lockfileVersion: 6.0", "package.json": "{}"},
			testFunc: detector.DetectPackageManager,
			expected: "pnpm",
		},
		{
			name:     "Yarn detection",
			files:    map[string]string{"yarn.lock": "# yarn lockfile v1", "package.json": "{}"},
			testFunc: detector.DetectPackageManager,
			expected: "yarn",
		},
		{
			name:     "npm fallback",
			files:    map[string]string{"package.json": "{}"},
			testFunc: detector.DetectPackageManager,
			expected: "npm",
		},
		{
			name:     "uv detection",
			files:    map[string]string{"uv.lock": "version = 1"},
			testFunc: detector.DetectPythonPackageManager,
			expected: "uv",
		},
		{
			name:     "Poetry detection",
			files:    map[string]string{"poetry.lock": "[[package]]"},
			testFunc: detector.DetectPythonPackageManager,
			expected: "poetry",
		},
		{
			name:     "Pipenv detection",
			files:    map[string]string{"Pipfile.lock": "{}"},
			testFunc: detector.DetectPythonPackageManager,
			expected: "pipenv",
		},
		{
			name:     "pip fallback",
			files:    map[string]string{"requirements.txt": "django==4.2.0"},
			testFunc: detector.DetectPythonPackageManager,
			expected: "pip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			result := tt.testFunc(projectPath)

			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestJSCommandGeneration(t *testing.T) {
	tests := []struct {
		pm               string
		expectedInstall  string
		expectedBuild    string
		expectedStart    string
	}{
		{"bun", "bun install", "bun run build", "bun run start"},
		{"pnpm", "pnpm install", "pnpm run build", "pnpm start"},
		{"yarn", "yarn install", "yarn build", "yarn start"},
		{"npm", "npm install", "npm run build", "npm start"},
	}

	for _, tt := range tests {
		t.Run(tt.pm, func(t *testing.T) {
			if got := detector.GetJSInstallCommand(tt.pm); got != tt.expectedInstall {
				t.Errorf("GetJSInstallCommand(%s) = %s, want %s", tt.pm, got, tt.expectedInstall)
			}

			if got := detector.GetJSBuildCommand(tt.pm); got != tt.expectedBuild {
				t.Errorf("GetJSBuildCommand(%s) = %s, want %s", tt.pm, got, tt.expectedBuild)
			}

			if got := detector.GetJSStartCommand(tt.pm); got != tt.expectedStart {
				t.Errorf("GetJSStartCommand(%s) = %s, want %s", tt.pm, got, tt.expectedStart)
			}
		})
	}
}

func TestPythonCommandGeneration(t *testing.T) {
	tests := []struct {
		pm       string
		expected string
	}{
		{"uv", "uv sync"},
		{"poetry", "poetry install"},
		{"pipenv", "pipenv install"},
		{"pip", "pip install -r requirements.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.pm, func(t *testing.T) {
			if got := detector.GetPythonInstallCommand(tt.pm); got != tt.expected {
				t.Errorf("GetPythonInstallCommand(%s) = %s, want %s", tt.pm, got, tt.expected)
			}
		})
	}
}

func TestBuildPlans(t *testing.T) {
	tests := []struct {
		name            string
		planFunc        func(string) ([]string, []string, map[string]any, []string)
		projectFiles    map[string]string
		expectedCommands []string
	}{
		{
			name:     "Next.js with pnpm",
			planFunc: detector.NextPlan,
			projectFiles: map[string]string{
				"pnpm-lock.yaml": "lockfileVersion: 6.0",
				"package.json":   "{}",
			},
			expectedCommands: []string{"pnpm install", "next build"},
		},
		{
			name:     "Django with poetry",
			planFunc: detector.DjangoPlan,
			projectFiles: map[string]string{
				"poetry.lock":   "[[package]]",
				"pyproject.toml": "[tool.poetry]",
			},
			expectedCommands: []string{"poetry install"},
		},
		{
			name:     "Go service",
			planFunc: detector.GoPlan,
			projectFiles: map[string]string{
				"go.mod":  "module test",
				"main.go": "package main",
			},
			expectedCommands: []string{"go build -o app ./..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.projectFiles)
			build, _, _, _ := tt.planFunc(projectPath)

			for _, expectedCmd := range tt.expectedCommands {
				found := false
				for _, cmd := range build {
					if strings.Contains(cmd, expectedCmd) || cmd == expectedCmd {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected command '%s' not found in build plan: %v", expectedCmd, build)
				}
			}
		})
	}
}