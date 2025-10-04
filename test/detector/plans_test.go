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
			name:     "Bun detection (bun.lockb)",
			files:    map[string]string{"bun.lockb": "binary content", "package.json": "{}"},
			testFunc: detector.DetectPackageManager,
			expected: "bun",
		},
		{
			name:     "Bun detection (bun.lock)",
			files:    map[string]string{"bun.lock": "binary content", "package.json": "{}"},
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
		planFunc        func(string) ([]string, []string, map[string]any, []string, map[string]string)
		projectFiles    map[string]string
		expectedCommands []string
		expectedPM      string
	}{
		{
			name:     "Next.js with pnpm",
			planFunc: detector.NextPlan,
			projectFiles: map[string]string{
				"pnpm-lock.yaml": "lockfileVersion: 6.0",
				"package.json":   "{}",
			},
			expectedCommands: []string{"pnpm install", "pnpm run build"},
			expectedPM:      "pnpm",
		},
		{
			name:     "Next.js with bun (bun.lockb)",
			planFunc: detector.NextPlan,
			projectFiles: map[string]string{
				"bun.lockb":    "binary content",
				"package.json": "{}",
			},
			expectedCommands: []string{"bun install", "bun run build"},
			expectedPM:      "bun",
		},
		{
			name:     "Next.js with bun (bun.lock)",
			planFunc: detector.NextPlan,
			projectFiles: map[string]string{
				"bun.lock":     "binary content",
				"package.json": "{}",
			},
			expectedCommands: []string{"bun install", "bun run build"},
			expectedPM:      "bun",
		},
		{
			name:     "Django with poetry",
			planFunc: detector.DjangoPlan,
			projectFiles: map[string]string{
				"poetry.lock":   "[[package]]",
				"pyproject.toml": "[tool.poetry]",
			},
			expectedCommands: []string{"poetry install"},
			expectedPM:      "poetry",
		},
		{
			name:     "Go service",
			planFunc: detector.GoPlan,
			projectFiles: map[string]string{
				"go.mod":  "module test",
				"main.go": "package main",
			},
			expectedCommands: []string{"go build -o app ./..."},
			expectedPM:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.projectFiles)
			build, _, _, _, meta := tt.planFunc(projectPath)

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

			// Verify package manager is stored in metadata
			if tt.expectedPM != "" {
				if pm, ok := meta["package_manager"]; !ok {
					t.Error("Expected package_manager in meta but not found")
				} else if pm != tt.expectedPM {
					t.Errorf("Expected package_manager '%s', got '%s'", tt.expectedPM, pm)
				}
			}
		})
	}
}

func TestFrameworkHealthChecks(t *testing.T) {
	tests := []struct {
		name          string
		planFunc      func(string) ([]string, []string, map[string]any, []string, map[string]string)
		projectFiles  map[string]string
		expectedPath  string
		expectedCode  int
	}{
		{
			name:     "Next.js health check",
			planFunc: detector.NextPlan,
			projectFiles: map[string]string{
				"package.json": "{}",
			},
			expectedPath: "/",
			expectedCode: 200,
		},
		{
			name:     "Django health check",
			planFunc: detector.DjangoPlan,
			projectFiles: map[string]string{
				"manage.py": "#!/usr/bin/env python",
			},
			expectedPath: "/healthz",
			expectedCode: 200,
		},
		{
			name:     "Go health check",
			planFunc: detector.GoPlan,
			projectFiles: map[string]string{
				"go.mod": "module test",
			},
			expectedPath: "/healthz",
			expectedCode: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.projectFiles)
			_, _, health, _, _ := tt.planFunc(projectPath)

			if health == nil {
				t.Fatal("Expected health check config, got nil")
			}

			path, ok := health["path"].(string)
			if !ok {
				t.Error("Health check 'path' should be a string")
			}
			if path != tt.expectedPath {
				t.Errorf("Expected health path '%s', got '%s'", tt.expectedPath, path)
			}

			expect, ok := health["expect"].(int)
			if !ok {
				t.Error("Health check 'expect' should be an int")
			}
			if expect != tt.expectedCode {
				t.Errorf("Expected health code %d, got %d", tt.expectedCode, expect)
			}
		})
	}
}

func TestFrameworkRunCommands(t *testing.T) {
	tests := []struct {
		name         string
		planFunc     func(string) ([]string, []string, map[string]any, []string, map[string]string)
		projectFiles map[string]string
		minCommands  int
	}{
		{
			name:     "Next.js run command",
			planFunc: detector.NextPlan,
			projectFiles: map[string]string{
				"package.json": "{}",
			},
			minCommands: 1,
		},
		{
			name:     "Django run command",
			planFunc: detector.DjangoPlan,
			projectFiles: map[string]string{
				"manage.py": "#!/usr/bin/env python",
			},
			minCommands: 1,
		},
		{
			name:     "Go run command",
			planFunc: detector.GoPlan,
			projectFiles: map[string]string{
				"go.mod": "module test",
			},
			minCommands: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.projectFiles)
			_, run, _, _, _ := tt.planFunc(projectPath)

			if len(run) < tt.minCommands {
				t.Errorf("Expected at least %d run command(s), got %d: %v", tt.minCommands, len(run), run)
			}
		})
	}
}

func TestFrameworkEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name         string
		planFunc     func(string) ([]string, []string, map[string]any, []string, map[string]string)
		projectFiles map[string]string
		requiredVars []string
	}{
		{
			name:     "Next.js env vars",
			planFunc: detector.NextPlan,
			projectFiles: map[string]string{
				"package.json": "{}",
			},
			requiredVars: []string{"NEXT_PUBLIC_*, any server-only envs"},
		},
		{
			name:     "Django env vars",
			planFunc: detector.DjangoPlan,
			projectFiles: map[string]string{
				"manage.py": "#!/usr/bin/env python",
			},
			requiredVars: []string{"DJANGO_SETTINGS_MODULE"},
		},
		{
			name:     "Go env vars",
			planFunc: detector.GoPlan,
			projectFiles: map[string]string{
				"go.mod": "module test",
			},
			requiredVars: []string{"PORT", "any app-specific envs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.projectFiles)
			_, _, _, envVars, _ := tt.planFunc(projectPath)

			for _, required := range tt.requiredVars {
				found := false
				for _, envVar := range envVars {
					if envVar == required {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected env var '%s' not found in: %v", required, envVars)
				}
			}
		})
	}
}

func TestPackageManagerPriority(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected string
	}{
		{
			name: "bun over pnpm (bun.lockb)",
			files: map[string]string{
				"bun.lockb":      "binary",
				"pnpm-lock.yaml": "lockfileVersion: 6.0",
				"package.json":   "{}",
			},
			expected: "bun",
		},
		{
			name: "bun over pnpm (bun.lock)",
			files: map[string]string{
				"bun.lock":       "binary",
				"pnpm-lock.yaml": "lockfileVersion: 6.0",
				"package.json":   "{}",
			},
			expected: "bun",
		},
		{
			name: "pnpm over yarn",
			files: map[string]string{
				"pnpm-lock.yaml": "lockfileVersion: 6.0",
				"yarn.lock":      "# yarn lockfile v1",
				"package.json":   "{}",
			},
			expected: "pnpm",
		},
		{
			name: "yarn over npm",
			files: map[string]string{
				"yarn.lock":    "# yarn lockfile v1",
				"package.json": "{}",
			},
			expected: "yarn",
		},
		{
			name: "uv over poetry",
			files: map[string]string{
				"uv.lock":     "version = 1",
				"poetry.lock": "[[package]]",
			},
			expected: "uv",
		},
		{
			name: "poetry over pipenv",
			files: map[string]string{
				"poetry.lock":  "[[package]]",
				"Pipfile.lock": "{}",
			},
			expected: "poetry",
		},
		{
			name: "pipenv over pip",
			files: map[string]string{
				"Pipfile.lock":     "{}",
				"requirements.txt": "django==4.2.0",
			},
			expected: "pipenv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)

			var result string
			if strings.Contains(tt.name, "bun") || strings.Contains(tt.name, "pnpm") || strings.Contains(tt.name, "yarn") || strings.Contains(tt.name, "npm") {
				result = detector.DetectPackageManager(projectPath)
			} else {
				result = detector.DetectPythonPackageManager(projectPath)
			}

			if result != tt.expected {
				t.Errorf("Expected %s to have priority, got %s", tt.expected, result)
			}
		})
	}
}

func TestAllFrameworkPlansReturnValidStructure(t *testing.T) {
	// Only test exported plan functions
	planFuncs := map[string]func(string) ([]string, []string, map[string]any, []string, map[string]string){
		"Next.js": detector.NextPlan,
		"Django":  detector.DjangoPlan,
		"Go":      detector.GoPlan,
	}

	// Create minimal test projects for each framework
	projectFiles := map[string]string{
		"package.json": "{}",
		"main.py":      "# python file",
		"go.mod":       "module test",
		"pom.xml":      "<project></project>",
	}

	for name, planFunc := range planFuncs {
		t.Run(name, func(t *testing.T) {
			projectPath := createTestProject(t, projectFiles)
			build, run, health, envVars, meta := planFunc(projectPath)

			// Verify build commands exist
			if len(build) == 0 {
				t.Error("Build commands should not be empty")
			}

			// Verify run commands exist
			if len(run) == 0 {
				t.Error("Run commands should not be empty")
			}

			// Verify health check is configured
			if health == nil {
				t.Error("Health check should not be nil")
			} else {
				if _, ok := health["path"]; !ok {
					t.Error("Health check should have 'path' key")
				}
				if _, ok := health["expect"]; !ok {
					t.Error("Health check should have 'expect' key")
				}
			}

			// Environment variables can be empty (not required)
			_ = envVars

			// Verify meta is returned (can be empty for non-JS/Python frameworks)
			if meta == nil {
				t.Error("Meta should not be nil")
			}
		})
	}
}

// TestPackageManagerInDetection verifies that package manager info flows through detection
func TestPackageManagerInDetection(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		expectedPM  string
		framework   string
	}{
		{
			name: "Next.js with bun (bun.lockb)",
			files: map[string]string{
				"next.config.js": "module.exports = {}",
				"package.json":   `{"dependencies": {"next": "^13.0.0"}}`,
				"bun.lockb":      "binary content",
			},
			expectedPM: "bun",
			framework:  "Next.js",
		},
		{
			name: "Next.js with bun (bun.lock)",
			files: map[string]string{
				"next.config.js": "module.exports = {}",
				"package.json":   `{"dependencies": {"next": "^13.0.0"}}`,
				"bun.lock":       "binary content",
			},
			expectedPM: "bun",
			framework:  "Next.js",
		},
		{
			name: "Next.js with pnpm",
			files: map[string]string{
				"next.config.js":  "module.exports = {}",
				"package.json":    `{"dependencies": {"next": "^13.0.0"}}`,
				"pnpm-lock.yaml":  "lockfileVersion: 6.0",
			},
			expectedPM: "pnpm",
			framework:  "Next.js",
		},
		{
			name: "Django with poetry",
			files: map[string]string{
				"manage.py":     "#!/usr/bin/env python",
				"poetry.lock":   "[[package]]",
				"pyproject.toml": `[tool.poetry]
name = "myapp"
[tool.poetry.dependencies]
django = "^4.2.0"`,
			},
			expectedPM: "poetry",
			framework:  "Django",
		},
		{
			name: "FastAPI with uv",
			files: map[string]string{
				"main.py":  `from fastapi import FastAPI\napp = FastAPI()`,
				"uv.lock":  "version = 1",
			},
			expectedPM: "uv",
			framework:  "FastAPI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := detector.DetectFramework(projectPath)

			if detection.Framework != tt.framework {
				t.Errorf("Expected framework '%s', got '%s'", tt.framework, detection.Framework)
			}

			if pm, ok := detection.Meta["package_manager"]; !ok {
				t.Error("Expected package_manager in detection Meta but not found")
			} else if pm != tt.expectedPM {
				t.Errorf("Expected package_manager '%s', got '%s'", tt.expectedPM, pm)
			}

			// Verify the build plan uses the correct package manager
			foundCorrectPM := false
			for _, cmd := range detection.BuildPlan {
				if strings.Contains(cmd, tt.expectedPM) {
					foundCorrectPM = true
					break
				}
			}
			if !foundCorrectPM {
				t.Errorf("Expected build plan to use '%s', but commands are: %v", tt.expectedPM, detection.BuildPlan)
			}
		})
	}
}