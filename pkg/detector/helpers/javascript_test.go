package helpers

import (
	"os"
	"path/filepath"
	"testing"
)

// mockFSReader implements FSReader for testing
type mockFSReader struct {
	root string
}

func newMockFSReader(root string) *mockFSReader {
	return &mockFSReader{root: root}
}

func (m *mockFSReader) Has(path string) bool {
	fullPath := filepath.Join(m.root, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (m *mockFSReader) Read(path string) string {
	fullPath := filepath.Join(m.root, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}
	return string(data)
}

func (m *mockFSReader) DirExists(path string) bool {
	fullPath := filepath.Join(m.root, path)
	fi, err := os.Stat(fullPath)
	return err == nil && fi.IsDir()
}

func TestParseNextConfig(t *testing.T) {
	tests := []struct {
		name           string
		configContent  string
		configFile     string
		hasAppDir      bool
		hasPagesDir    bool
		expectedOutput string
		expectedRouter string
		expectedBuild  string
	}{
		{
			name:           "standalone mode with app router",
			configContent:  `module.exports = { output: 'standalone' }`,
			configFile:     "next.config.js",
			hasAppDir:      true,
			expectedOutput: "standalone",
			expectedRouter: "app",
			expectedBuild:  ".next/standalone/",
		},
		{
			name:           "export mode with pages router",
			configContent:  `module.exports = { output: "export" }`,
			configFile:     "next.config.js",
			hasPagesDir:    true,
			expectedOutput: "export",
			expectedRouter: "pages",
			expectedBuild:  "out/",
		},
		{
			name:           "default mode",
			configContent:  `module.exports = {}`,
			configFile:     "next.config.js",
			hasAppDir:      true,
			expectedOutput: "default",
			expectedRouter: "app",
			expectedBuild:  ".next/",
		},
		{
			name:           "typescript config with standalone",
			configContent:  `export default { output: 'standalone' }`,
			configFile:     "next.config.ts",
			hasPagesDir:    true,
			expectedOutput: "standalone",
			expectedRouter: "pages",
			expectedBuild:  ".next/standalone/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Write config file
			os.WriteFile(filepath.Join(tmpDir, tt.configFile), []byte(tt.configContent), 0644)

			// Create directories
			if tt.hasAppDir {
				os.Mkdir(filepath.Join(tmpDir, "app"), 0755)
			}
			if tt.hasPagesDir {
				os.Mkdir(filepath.Join(tmpDir, "pages"), 0755)
			}

			fs := newMockFSReader(tmpDir)
			config := ParseNextConfig(fs)

			if config.OutputMode != tt.expectedOutput {
				t.Errorf("OutputMode = %v, want %v", config.OutputMode, tt.expectedOutput)
			}
			if config.Router != tt.expectedRouter {
				t.Errorf("Router = %v, want %v", config.Router, tt.expectedRouter)
			}
			if config.BuildOutput != tt.expectedBuild {
				t.Errorf("BuildOutput = %v, want %v", config.BuildOutput, tt.expectedBuild)
			}
		})
	}
}

func TestParsePackageJSON(t *testing.T) {
	tests := []struct {
		name             string
		packageJSON      string
		expectedScripts  map[string]string
		expectedHasDep   string
		expectedHasNoDep string
	}{
		{
			name: "full package.json",
			packageJSON: `{
				"scripts": {
					"dev": "next dev",
					"build": "next build",
					"start": "next start",
					"start:prod": "node server.js"
				},
				"dependencies": {
					"next": "^14.0.0",
					"react": "^18.0.0"
				},
				"devDependencies": {
					"typescript": "^5.0.0"
				}
			}`,
			expectedScripts: map[string]string{
				"dev":        "next dev",
				"build":      "next build",
				"start":      "next start",
				"start:prod": "node server.js",
			},
			expectedHasDep:   "next",
			expectedHasNoDep: "vue",
		},
		{
			name:            "empty package.json",
			packageJSON:     `{}`,
			expectedScripts: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(tt.packageJSON), 0644)

			fs := newMockFSReader(tmpDir)
			pkg := ParsePackageJSON(fs)

			// Check scripts
			for k, v := range tt.expectedScripts {
				if pkg.Scripts[k] != v {
					t.Errorf("Script[%s] = %v, want %v", k, pkg.Scripts[k], v)
				}
			}

			// Check dependencies
			if tt.expectedHasDep != "" {
				if _, exists := pkg.Dependencies[tt.expectedHasDep]; !exists {
					if _, exists := pkg.DevDeps[tt.expectedHasDep]; !exists {
						t.Errorf("Expected dependency %s not found", tt.expectedHasDep)
					}
				}
			}

			if tt.expectedHasNoDep != "" {
				if _, exists := pkg.Dependencies[tt.expectedHasNoDep]; exists {
					t.Errorf("Unexpected dependency %s found", tt.expectedHasNoDep)
				}
			}
		})
	}
}

func TestDetectFrameworkAdapter(t *testing.T) {
	tests := []struct {
		name            string
		framework       string
		deps            map[string]string
		expectedType    string
		expectedRunMode string
	}{
		{
			name:      "Remix with Node adapter",
			framework: "remix",
			deps: map[string]string{
				"@remix-run/react": "^2.0.0",
				"@remix-run/node":  "^2.0.0",
			},
			expectedType:    "node",
			expectedRunMode: "server",
		},
		{
			name:      "Remix with Cloudflare adapter",
			framework: "remix",
			deps: map[string]string{
				"@remix-run/cloudflare": "^2.0.0",
			},
			expectedType:    "cloudflare",
			expectedRunMode: "server",
		},
		{
			name:      "SvelteKit with static adapter",
			framework: "sveltekit",
			deps: map[string]string{
				"@sveltejs/kit":            "^2.0.0",
				"@sveltejs/adapter-static": "^3.0.0",
			},
			expectedType:    "static",
			expectedRunMode: "static",
		},
		{
			name:      "SvelteKit with node adapter",
			framework: "sveltekit",
			deps: map[string]string{
				"@sveltejs/kit":          "^2.0.0",
				"@sveltejs/adapter-node": "^2.0.0",
			},
			expectedType:    "node",
			expectedRunMode: "server",
		},
		{
			name:      "Astro with Node adapter",
			framework: "astro",
			deps: map[string]string{
				"astro":         "^4.0.0",
				"@astrojs/node": "^8.0.0",
			},
			expectedType:    "node",
			expectedRunMode: "server",
		},
		{
			name:      "Astro static (no adapter)",
			framework: "astro",
			deps: map[string]string{
				"astro": "^4.0.0",
			},
			expectedType:    "static",
			expectedRunMode: "static",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := PackageJSON{
				Dependencies: tt.deps,
			}

			adapter := DetectFrameworkAdapter(pkg, tt.framework)

			if adapter.Type != tt.expectedType {
				t.Errorf("Type = %v, want %v", adapter.Type, tt.expectedType)
			}
			if adapter.RunMode != tt.expectedRunMode {
				t.Errorf("RunMode = %v, want %v", adapter.RunMode, tt.expectedRunMode)
			}
		})
	}
}

func TestGetProductionStartScript(t *testing.T) {
	tests := []struct {
		name     string
		scripts  map[string]string
		expected string
	}{
		{
			name: "has start:prod",
			scripts: map[string]string{
				"start":      "next start",
				"start:prod": "node server.js",
			},
			expected: "start:prod",
		},
		{
			name: "has serve",
			scripts: map[string]string{
				"start": "vite",
				"serve": "vite preview",
			},
			expected: "serve",
		},
		{
			name: "has preview",
			scripts: map[string]string{
				"start":   "vite",
				"preview": "vite preview",
			},
			expected: "preview",
		},
		{
			name: "only start",
			scripts: map[string]string{
				"start": "node index.js",
			},
			expected: "start",
		},
		{
			name: "start:production over serve",
			scripts: map[string]string{
				"start:production": "node dist/server.js",
				"serve":            "http-server",
			},
			expected: "start:production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := PackageJSON{
				Scripts: tt.scripts,
			}

			result := GetProductionStartScript(pkg)
			if result != tt.expected {
				t.Errorf("GetProductionStartScript() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetProductionBuildScript(t *testing.T) {
	tests := []struct {
		name     string
		scripts  map[string]string
		expected string
	}{
		{
			name: "has build:prod",
			scripts: map[string]string{
				"build":      "vite build",
				"build:prod": "vite build --mode production",
			},
			expected: "build:prod",
		},
		{
			name: "only build",
			scripts: map[string]string{
				"build": "next build",
			},
			expected: "build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := PackageJSON{
				Scripts: tt.scripts,
			}

			result := GetProductionBuildScript(pkg)
			if result != tt.expected {
				t.Errorf("GetProductionBuildScript() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectMonorepoType(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		pkgJSON  string
		expected string
	}{
		{
			name:     "turborepo",
			files:    []string{"turbo.json"},
			expected: "turborepo",
		},
		{
			name:     "nx",
			files:    []string{"nx.json"},
			expected: "nx",
		},
		{
			name:     "lerna",
			files:    []string{"lerna.json"},
			expected: "lerna",
		},
		{
			name:     "npm workspaces",
			files:    []string{"package.json"},
			pkgJSON:  `{"workspaces": ["packages/*"]}`,
			expected: "npm-workspaces",
		},
		{
			name:     "pnpm workspaces",
			files:    []string{"pnpm-workspace.yaml"},
			expected: "pnpm-workspaces",
		},
		{
			name:     "no monorepo",
			files:    []string{"package.json"},
			pkgJSON:  `{"name": "my-app"}`,
			expected: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for _, file := range tt.files {
				content := "{}"
				if file == "package.json" && tt.pkgJSON != "" {
					content = tt.pkgJSON
				}
				os.WriteFile(filepath.Join(tmpDir, file), []byte(content), 0644)
			}

			fs := newMockFSReader(tmpDir)
			result := DetectMonorepoType(fs)
			if result != tt.expected {
				t.Errorf("DetectMonorepoType() = %v, want %v", result, tt.expected)
			}
		})
	}
}
