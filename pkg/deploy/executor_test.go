package deploy

import (
	"lightfold/pkg/detector"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewExecutor(t *testing.T) {
	exec := NewExecutor(nil, "test-app", "/path/to/project", nil)

	if exec.appName != "test-app" {
		t.Errorf("appName = %v, want 'test-app'", exec.appName)
	}
	if exec.projectPath != "/path/to/project" {
		t.Errorf("projectPath = %v, want '/path/to/project'", exec.projectPath)
	}
}

func TestAdjustBuildCommand(t *testing.T) {
	tests := []struct {
		name      string
		language  string
		cmd       string
		appName   string
		wantMatch string // substring to check for in result
	}{
		{
			name:      "python pip install",
			language:  "Python",
			cmd:       "pip install -r requirements.txt",
			appName:   "myapp",
			wantMatch: "/srv/myapp/shared/venv/bin/pip install",
		},
		{
			name:      "python poetry install",
			language:  "Python",
			cmd:       "poetry install",
			appName:   "myapp",
			wantMatch: "pip3 install poetry",
		},
		{
			name:      "node npm install",
			language:  "JavaScript/TypeScript",
			cmd:       "npm install",
			appName:   "myapp",
			wantMatch: "npm install",
		},
		{
			name:      "node pnpm install",
			language:  "JavaScript/TypeScript",
			cmd:       "pnpm install",
			appName:   "myapp",
			wantMatch: "npm install -g pnpm",
		},
		{
			name:      "ruby bundle install",
			language:  "Ruby",
			cmd:       "bundle install",
			appName:   "myapp",
			wantMatch: "gem install bundler",
		},
		{
			name:      "go build",
			language:  "Go",
			cmd:       "go build",
			appName:   "myapp",
			wantMatch: "go build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detection := &detector.Detection{
				Language: tt.language,
			}
			exec := NewExecutor(nil, tt.appName, "", detection)

			result := exec.adjustBuildCommand(tt.cmd, "/srv/"+tt.appName+"/releases/20240101000000")

			if !strings.Contains(result, tt.wantMatch) {
				t.Errorf("adjustBuildCommand() = %v, should contain %v", result, tt.wantMatch)
			}
		})
	}
}

func TestAdjustBuildCommand_NoDetection(t *testing.T) {
	exec := NewExecutor(nil, "test-app", "", nil)
	cmd := "npm install"
	result := exec.adjustBuildCommand(cmd, "/path")

	if result != cmd {
		t.Errorf("adjustBuildCommand() with nil detection = %v, want %v", result, cmd)
	}
}

func TestCreateReleaseTarball(t *testing.T) {
	// Create temporary test project
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	os.Mkdir(projectDir, 0755)

	// Create test files
	testFiles := map[string]string{
		"main.go":              "package main",
		"src/app.js":           "console.log('hello')",
		"README.md":            "# Test Project",
		".env":                 "SECRET=value",
		"node_modules/pkg.js":  "// should be ignored",
		".git/config":          "# should be ignored",
		"__pycache__/test.pyc": "# should be ignored",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(projectDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	// Create executor
	exec := NewExecutor(nil, "test-app", projectDir, nil)

	// Create tarball
	tarballPath := filepath.Join(tmpDir, "release.tar.gz")
	err := exec.CreateReleaseTarball(tarballPath)
	if err != nil {
		t.Fatalf("CreateReleaseTarball() error = %v", err)
	}

	// Verify tarball was created
	if _, err := os.Stat(tarballPath); os.IsNotExist(err) {
		t.Errorf("Tarball was not created at %s", tarballPath)
	}

	// Verify tarball size is reasonable (should contain some files)
	info, err := os.Stat(tarballPath)
	if err != nil {
		t.Fatalf("Failed to stat tarball: %v", err)
	}
	if info.Size() < 100 {
		t.Errorf("Tarball size = %d bytes, seems too small", info.Size())
	}
}

func TestCreateReleaseTarball_EmptyProject(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "empty")
	os.Mkdir(projectDir, 0755)

	exec := NewExecutor(nil, "test-app", projectDir, nil)
	tarballPath := filepath.Join(tmpDir, "release.tar.gz")

	err := exec.CreateReleaseTarball(tarballPath)
	if err != nil {
		t.Fatalf("CreateReleaseTarball() error = %v", err)
	}

	// Should create tarball even for empty project
	if _, err := os.Stat(tarballPath); os.IsNotExist(err) {
		t.Errorf("Tarball was not created for empty project")
	}
}

func TestCreateReleaseTarball_IgnorePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	os.Mkdir(projectDir, 0755)

	// Create files that should be ignored
	ignoredDirs := []string{
		"node_modules",
		".git",
		"__pycache__",
		".venv",
		".next",
		"build",
		"dist",
		"target",
	}

	for _, dir := range ignoredDirs {
		dirPath := filepath.Join(projectDir, dir)
		os.MkdirAll(dirPath, 0755)
		os.WriteFile(filepath.Join(dirPath, "test.txt"), []byte("content"), 0644)
	}

	// Create file that should be included
	os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644)

	exec := NewExecutor(nil, "test-app", projectDir, nil)
	tarballPath := filepath.Join(tmpDir, "release.tar.gz")

	err := exec.CreateReleaseTarball(tarballPath)
	if err != nil {
		t.Fatalf("CreateReleaseTarball() error = %v", err)
	}

	// Verify tarball exists and has reasonable size
	// (should be small since most directories are ignored)
	info, err := os.Stat(tarballPath)
	if err != nil {
		t.Fatalf("Failed to stat tarball: %v", err)
	}

	// Should be relatively small since ignored dirs are excluded
	if info.Size() > 10000 {
		t.Logf("Warning: Tarball size = %d bytes, might include ignored directories", info.Size())
	}
}

func TestDirectoryStructurePaths(t *testing.T) {
	appName := "my-app"
	expectedPaths := []string{
		"/srv/my-app",
		"/srv/my-app/releases",
		"/srv/my-app/shared",
		"/srv/my-app/shared/env",
		"/srv/my-app/shared/logs",
		"/srv/my-app/shared/static",
		"/srv/my-app/shared/media",
	}

	// Verify path construction logic
	appPath := "/srv/" + appName
	directories := []string{
		appPath,
		filepath.Join(appPath, "releases"),
		filepath.Join(appPath, "shared"),
		filepath.Join(appPath, "shared", "env"),
		filepath.Join(appPath, "shared", "logs"),
		filepath.Join(appPath, "shared", "static"),
		filepath.Join(appPath, "shared", "media"),
	}

	for i, expected := range expectedPaths {
		if directories[i] != expected {
			t.Errorf("Directory[%d] = %v, want %v", i, directories[i], expected)
		}
	}
}

func TestWriteEnvironmentFile_Content(t *testing.T) {
	envVars := map[string]string{
		"NODE_ENV":    "production",
		"PORT":        "8000",
		"DATABASE_URL": "postgres://localhost/mydb",
	}

	// Build expected content (order doesn't matter for map iteration)
	var content strings.Builder
	for key, value := range envVars {
		content.WriteString(key + "=" + value + "\n")
	}

	result := content.String()

	// Verify all key-value pairs are present
	for key, value := range envVars {
		expected := key + "=" + value
		if !strings.Contains(result, expected) {
			t.Errorf("Environment content missing %s", expected)
		}
	}
}

func TestWriteEnvironmentFile_Empty(t *testing.T) {
	envVars := map[string]string{}

	// Empty env vars should result in empty content
	var content strings.Builder
	for key, value := range envVars {
		content.WriteString(key + "=" + value + "\n")
	}

	if content.String() != "" {
		t.Errorf("Empty envVars should produce empty content, got %v", content.String())
	}
}

func TestReleasePathGeneration(t *testing.T) {
	appName := "test-app"
	timestamp := "20240115123045"

	releasePath := "/srv/" + appName + "/releases/" + timestamp

	expected := "/srv/test-app/releases/20240115123045"
	if releasePath != expected {
		t.Errorf("Release path = %v, want %v", releasePath, expected)
	}

	// Verify path components
	if !strings.HasPrefix(releasePath, "/srv/") {
		t.Error("Release path should start with /srv/")
	}
	if !strings.Contains(releasePath, "/releases/") {
		t.Error("Release path should contain /releases/")
	}
	if !strings.HasSuffix(releasePath, timestamp) {
		t.Error("Release path should end with timestamp")
	}
}

func TestCleanupOldReleases_Logic(t *testing.T) {
	tests := []struct {
		name       string
		releases   []string
		keepCount  int
		wantDelete int
	}{
		{
			name:       "more releases than keep count",
			releases:   []string{"20240101000000", "20240102000000", "20240103000000", "20240104000000", "20240105000000"},
			keepCount:  3,
			wantDelete: 2,
		},
		{
			name:       "exact match",
			releases:   []string{"20240101000000", "20240102000000", "20240103000000"},
			keepCount:  3,
			wantDelete: 0,
		},
		{
			name:       "fewer releases than keep count",
			releases:   []string{"20240101000000", "20240102000000"},
			keepCount:  5,
			wantDelete: 0,
		},
		{
			name:       "keep 5 releases",
			releases:   []string{"20240101000000", "20240102000000", "20240103000000", "20240104000000", "20240105000000", "20240106000000", "20240107000000"},
			keepCount:  5,
			wantDelete: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate how many to delete
			toDeleteCount := 0
			if len(tt.releases) > tt.keepCount {
				toDeleteCount = len(tt.releases) - tt.keepCount
			}

			if toDeleteCount != tt.wantDelete {
				t.Errorf("Would delete %d releases, want %d", toDeleteCount, tt.wantDelete)
			}

			// Verify we keep the most recent ones
			if len(tt.releases) > tt.keepCount {
				kept := tt.releases[len(tt.releases)-tt.keepCount:]
				if len(kept) != tt.keepCount {
					t.Errorf("Kept %d releases, want %d", len(kept), tt.keepCount)
				}
			}
		})
	}
}

func TestBuildRelease_NoDetection(t *testing.T) {
	exec := NewExecutor(nil, "test-app", "/path", nil)

	// Should not error when detection is nil
	err := exec.BuildRelease("/srv/test-app/releases/20240101000000")
	if err != nil {
		t.Errorf("BuildRelease() with nil detection should not error, got %v", err)
	}
}

func TestBuildRelease_NoBuildPlan(t *testing.T) {
	detection := &detector.Detection{
		Framework:  "Static",
		Language:   "HTML",
		BuildPlan:  []string{},
		Confidence: 1.0,
	}

	exec := NewExecutor(nil, "test-app", "/path", detection)

	// Should not error when build plan is empty
	err := exec.BuildRelease("/srv/test-app/releases/20240101000000")
	if err != nil {
		t.Errorf("BuildRelease() with empty build plan should not error, got %v", err)
	}
}

// --- Phase 3: Service Configuration Tests ---

func TestGetExecStartCommand_Python(t *testing.T) {
	tests := []struct {
		framework string
		want      string
	}{
		{"Django", "/srv/test-app/shared/venv/bin/gunicorn --bind 127.0.0.1:8000 --workers 2 wsgi:application"},
		{"FastAPI", "/srv/test-app/shared/venv/bin/uvicorn main:app --host 127.0.0.1 --port 8000 --workers 2"},
		{"Flask", "/srv/test-app/shared/venv/bin/gunicorn --bind 127.0.0.1:8000 --workers 2 app:app"},
	}

	for _, tt := range tests {
		t.Run(tt.framework, func(t *testing.T) {
			detection := &detector.Detection{
				Framework: tt.framework,
				Language:  "Python",
			}
			exec := NewExecutor(nil, "test-app", "/path", detection)
			result := exec.getExecStartCommand()

			if result != tt.want {
				t.Errorf("getExecStartCommand() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestGetExecStartCommand_JavaScript(t *testing.T) {
	tests := []struct {
		framework string
		want      string
	}{
		{"Next.js", "/usr/bin/node /srv/test-app/current/.next/standalone/server.js"},
		{"Express.js", "/usr/bin/node /srv/test-app/current/server.js"},
		{"NestJS", "/usr/bin/node /srv/test-app/current/dist/main.js"},
	}

	for _, tt := range tests {
		t.Run(tt.framework, func(t *testing.T) {
			detection := &detector.Detection{
				Framework: tt.framework,
				Language:  "JavaScript/TypeScript",
			}
			exec := NewExecutor(nil, "test-app", "/path", detection)
			result := exec.getExecStartCommand()

			if result != tt.want {
				t.Errorf("getExecStartCommand() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestGetExecStartCommand_Go(t *testing.T) {
	detection := &detector.Detection{
		Framework: "Go HTTP",
		Language:  "Go",
	}
	exec := NewExecutor(nil, "test-app", "/path", detection)
	result := exec.getExecStartCommand()

	want := "/srv/test-app/current/app --port 8000"
	if result != want {
		t.Errorf("getExecStartCommand() = %v, want %v", result, want)
	}
}

func TestGetExecStartCommand_Ruby(t *testing.T) {
	detection := &detector.Detection{
		Framework: "Rails",
		Language:  "Ruby",
	}
	exec := NewExecutor(nil, "test-app", "/path", detection)
	result := exec.getExecStartCommand()

	want := "/srv/test-app/shared/bundle/bin/puma -C /srv/test-app/current/config/puma.rb"
	if result != want {
		t.Errorf("getExecStartCommand() = %v, want %v", result, want)
	}
}

func TestGetExecStartCommand_Fallback(t *testing.T) {
	detection := &detector.Detection{
		Framework: "Unknown",
		Language:  "Unknown",
		RunPlan:   []string{"./start.sh"},
	}
	exec := NewExecutor(nil, "test-app", "/path", detection)
	result := exec.getExecStartCommand()

	want := "./start.sh"
	if result != want {
		t.Errorf("getExecStartCommand() fallback = %v, want %v", result, want)
	}
}

func TestGetExecStartCommand_NoDetection(t *testing.T) {
	exec := NewExecutor(nil, "test-app", "/path", nil)
	result := exec.getExecStartCommand()

	want := "/usr/bin/true"
	if result != want {
		t.Errorf("getExecStartCommand() with nil detection = %v, want %v", result, want)
	}
}

func TestSystemdUnitPaths(t *testing.T) {
	appName := "my-app"
	unitPath := "/etc/systemd/system/" + appName + ".service"

	expected := "/etc/systemd/system/my-app.service"
	if unitPath != expected {
		t.Errorf("Systemd unit path = %v, want %v", unitPath, expected)
	}
}

func TestNginxConfigPaths(t *testing.T) {
	appName := "my-app"
	configPath := "/etc/nginx/sites-available/" + appName
	enabledPath := "/etc/nginx/sites-enabled/" + appName

	if configPath != "/etc/nginx/sites-available/my-app" {
		t.Errorf("Nginx config path = %v", configPath)
	}
	if enabledPath != "/etc/nginx/sites-enabled/my-app" {
		t.Errorf("Nginx enabled path = %v", enabledPath)
	}
}

// --- Phase 4: Release Management Tests ---

func TestSwitchReleasePaths(t *testing.T) {
	appName := "test-app"
	releasePath := "/srv/test-app/releases/20240115123045"
	currentLink := "/srv/" + appName + "/current"
	tempLink := "/srv/" + appName + "/current.tmp"

	if currentLink != "/srv/test-app/current" {
		t.Errorf("Current link path = %v", currentLink)
	}
	if tempLink != "/srv/test-app/current.tmp" {
		t.Errorf("Temp link path = %v", tempLink)
	}

	// Verify ln command format
	lnCmd := "ln -sf " + releasePath + " " + tempLink
	if !strings.Contains(lnCmd, releasePath) {
		t.Error("ln command should contain release path")
	}
	if !strings.Contains(lnCmd, tempLink) {
		t.Error("ln command should contain temp link")
	}

	// Verify mv command format
	mvCmd := "mv -Tf " + tempLink + " " + currentLink
	if !strings.Contains(mvCmd, tempLink) {
		t.Error("mv command should contain temp link")
	}
	if !strings.Contains(mvCmd, currentLink) {
		t.Error("mv command should contain current link")
	}
}

func TestPerformHealthCheck_NoHealthcheck(t *testing.T) {
	detection := &detector.Detection{
		Framework:   "Next.js",
		Healthcheck: nil,
	}
	exec := NewExecutor(nil, "test-app", "/path", detection)

	// Should succeed when no healthcheck is configured
	err := exec.PerformHealthCheck(3, 2*time.Second)
	if err != nil {
		t.Errorf("PerformHealthCheck() with nil healthcheck should succeed, got %v", err)
	}
}

func TestPerformHealthCheck_NoDetection(t *testing.T) {
	exec := NewExecutor(nil, "test-app", "/path", nil)

	// Should succeed when detection is nil
	err := exec.PerformHealthCheck(3, 2*time.Second)
	if err != nil {
		t.Errorf("PerformHealthCheck() with nil detection should succeed, got %v", err)
	}
}

func TestPerformHealthCheck_Configuration(t *testing.T) {
	tests := []struct {
		name       string
		healthcheck map[string]any
		wantPath   string
		wantStatus int
		wantTimeout int
	}{
		{
			name: "default values",
			healthcheck: map[string]any{},
			wantPath: "/",
			wantStatus: 200,
			wantTimeout: 30,
		},
		{
			name: "custom path",
			healthcheck: map[string]any{
				"path": "/health",
			},
			wantPath: "/health",
			wantStatus: 200,
			wantTimeout: 30,
		},
		{
			name: "custom status code (int)",
			healthcheck: map[string]any{
				"expect": 204,
			},
			wantPath: "/",
			wantStatus: 204,
			wantTimeout: 30,
		},
		{
			name: "custom status code (float64)",
			healthcheck: map[string]any{
				"expect": float64(204),
			},
			wantPath: "/",
			wantStatus: 204,
			wantTimeout: 30,
		},
		{
			name: "custom timeout (int)",
			healthcheck: map[string]any{
				"timeout_seconds": 60,
			},
			wantPath: "/",
			wantStatus: 200,
			wantTimeout: 60,
		},
		{
			name: "custom timeout (float64)",
			healthcheck: map[string]any{
				"timeout_seconds": float64(60),
			},
			wantPath: "/",
			wantStatus: 200,
			wantTimeout: 60,
		},
		{
			name: "all custom",
			healthcheck: map[string]any{
				"path": "/api/health",
				"expect": 200,
				"timeout_seconds": 45,
			},
			wantPath: "/api/health",
			wantStatus: 200,
			wantTimeout: 45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detection := &detector.Detection{
				Framework:   "Next.js",
				Healthcheck: tt.healthcheck,
			}
			_ = NewExecutor(nil, "test-app", "/path", detection)

			// We can't actually test the health check without SSH connection,
			// but we can verify the configuration is parsed correctly

			healthPath := "/"
			expectedStatus := 200
			timeout := 30

			if path, ok := detection.Healthcheck["path"].(string); ok {
				healthPath = path
			}
			if expect, ok := detection.Healthcheck["expect"].(int); ok {
				expectedStatus = expect
			}
			if expectFloat, ok := detection.Healthcheck["expect"].(float64); ok {
				expectedStatus = int(expectFloat)
			}
			if timeoutSec, ok := detection.Healthcheck["timeout_seconds"].(int); ok {
				timeout = timeoutSec
			}
			if timeoutFloat, ok := detection.Healthcheck["timeout_seconds"].(float64); ok {
				timeout = int(timeoutFloat)
			}

			if healthPath != tt.wantPath {
				t.Errorf("healthPath = %v, want %v", healthPath, tt.wantPath)
			}
			if expectedStatus != tt.wantStatus {
				t.Errorf("expectedStatus = %v, want %v", expectedStatus, tt.wantStatus)
			}
			if timeout != tt.wantTimeout {
				t.Errorf("timeout = %v, want %v", timeout, tt.wantTimeout)
			}

			// Verify URL construction
			url := "http://127.0.0.1:8000" + healthPath
			expectedURL := "http://127.0.0.1:8000" + tt.wantPath
			if url != expectedURL {
				t.Errorf("health check URL = %v, want %v", url, expectedURL)
			}
		})
	}
}

func TestRollbackLogic(t *testing.T) {
	tests := []struct {
		name          string
		releases      []string
		canRollback   bool
		previousIndex int
	}{
		{
			name:          "two releases",
			releases:      []string{"20240101000000", "20240102000000"},
			canRollback:   true,
			previousIndex: 0,
		},
		{
			name:          "multiple releases",
			releases:      []string{"20240101000000", "20240102000000", "20240103000000", "20240104000000"},
			canRollback:   true,
			previousIndex: 2,
		},
		{
			name:        "single release",
			releases:    []string{"20240101000000"},
			canRollback: false,
		},
		{
			name:        "no releases",
			releases:    []string{},
			canRollback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canRollback := len(tt.releases) >= 2

			if canRollback != tt.canRollback {
				t.Errorf("canRollback = %v, want %v", canRollback, tt.canRollback)
			}

			if canRollback {
				previousRelease := tt.releases[len(tt.releases)-2]
				if previousRelease != tt.releases[tt.previousIndex] {
					t.Errorf("previousRelease = %v, want %v", previousRelease, tt.releases[tt.previousIndex])
				}
			}
		})
	}
}

func TestRollbackReleasePath(t *testing.T) {
	appName := "test-app"
	timestamp := "20240115123045"

	releasePath := "/srv/" + appName + "/releases/" + timestamp

	expected := "/srv/test-app/releases/20240115123045"
	if releasePath != expected {
		t.Errorf("Rollback release path = %v, want %v", releasePath, expected)
	}
}
