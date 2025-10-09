package runtime_test

import (
	"lightfold/pkg/runtime"
	"lightfold/pkg/state"
	"testing"
)

func TestGetRuntimeFromLanguage(t *testing.T) {
	tests := []struct {
		language string
		want     runtime.Runtime
	}{
		{"JavaScript/TypeScript", runtime.RuntimeNodeJS},
		{"Python", runtime.RuntimePython},
		{"Go", runtime.RuntimeGo},
		{"PHP", runtime.RuntimePHP},
		{"Ruby", runtime.RuntimeRuby},
		{"Java", runtime.RuntimeJava},
		{"Unknown", runtime.RuntimeUnknown},
		{"", runtime.RuntimeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			got := runtime.GetRuntimeFromLanguage(tt.language)
			if got != tt.want {
				t.Errorf("GetRuntimeFromLanguage(%q) = %q, want %q", tt.language, got, tt.want)
			}
		})
	}
}

func TestGetRuntimeInfo(t *testing.T) {
	tests := []struct {
		runtime runtime.Runtime
		wantLen int // Expected number of packages
	}{
		{runtime.RuntimeNodeJS, 2},  // nodejs, npm
		{runtime.RuntimePython, 2},  // python3-pip, python3-venv
		{runtime.RuntimeGo, 1},      // golang-go
		{runtime.RuntimePHP, 5},     // php, php-fpm, etc.
		{runtime.RuntimeRuby, 1},    // ruby-full
		{runtime.RuntimeJava, 2},    // default-jre, default-jdk
		{runtime.RuntimeUnknown, 0}, // nothing
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			info := runtime.GetRuntimeInfo(tt.runtime)

			if len(info.Packages) != tt.wantLen {
				t.Errorf("GetRuntimeInfo(%q).Packages length = %d, want %d", tt.runtime, len(info.Packages), tt.wantLen)
			}

			if info.Runtime != tt.runtime {
				t.Errorf("GetRuntimeInfo(%q).Runtime = %q, want %q", tt.runtime, info.Runtime, tt.runtime)
			}

			// Ensure at least basic structure is present
			if tt.runtime != runtime.RuntimeUnknown {
				if info.Packages == nil {
					t.Errorf("GetRuntimeInfo(%q).Packages is nil", tt.runtime)
				}
				if info.Directories == nil {
					t.Errorf("GetRuntimeInfo(%q).Directories is nil", tt.runtime)
				}
				if info.Commands == nil {
					t.Errorf("GetRuntimeInfo(%q).Commands is nil", tt.runtime)
				}
			}
		})
	}
}

func TestGetLanguageFromFramework(t *testing.T) {
	tests := []struct {
		framework string
		want      string
	}{
		// JavaScript/TypeScript
		{"Next.js", "JavaScript/TypeScript"},
		{"Express.js", "JavaScript/TypeScript"},
		{"NestJS", "JavaScript/TypeScript"},
		{"Nuxt.js", "JavaScript/TypeScript"},
		{"Remix", "JavaScript/TypeScript"},
		{"SvelteKit", "JavaScript/TypeScript"},
		{"Astro", "JavaScript/TypeScript"},
		{"Fastify", "JavaScript/TypeScript"},

		// Python
		{"Django", "Python"},
		{"FastAPI", "Python"},
		{"Flask", "Python"},

		// Go
		{"Gin", "Go"},
		{"Echo", "Go"},
		{"Fiber", "Go"},

		// PHP
		{"Laravel", "PHP"},
		{"Symfony", "PHP"},
		{"CodeIgniter", "PHP"},

		// Ruby
		{"Rails", "Ruby"},
		{"Sinatra", "Ruby"},

		// Java
		{"Spring Boot", "Java"},

		// Unknown
		{"Unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.framework, func(t *testing.T) {
			got := runtime.GetLanguageFromFramework(tt.framework)
			if got != tt.want {
				t.Errorf("GetLanguageFromFramework(%q) = %q, want %q", tt.framework, got, tt.want)
			}
		})
	}
}

func TestGetRequiredRuntimesForApps(t *testing.T) {
	tests := []struct {
		name    string
		apps    []state.DeployedApp
		wantLen int
		wantHas []runtime.Runtime
	}{
		{
			name: "single JS app",
			apps: []state.DeployedApp{
				{Framework: "Next.js"},
			},
			wantLen: 1,
			wantHas: []runtime.Runtime{runtime.RuntimeNodeJS},
		},
		{
			name: "JS + Python",
			apps: []state.DeployedApp{
				{Framework: "Next.js"},
				{Framework: "Django"},
			},
			wantLen: 2,
			wantHas: []runtime.Runtime{runtime.RuntimeNodeJS, runtime.RuntimePython},
		},
		{
			name: "multiple same language",
			apps: []state.DeployedApp{
				{Framework: "Next.js"},
				{Framework: "Express.js"},
			},
			wantLen: 1,
			wantHas: []runtime.Runtime{runtime.RuntimeNodeJS},
		},
		{
			name: "JS + Python + Go",
			apps: []state.DeployedApp{
				{Framework: "Next.js"},
				{Framework: "Django"},
				{Framework: "Gin"},
			},
			wantLen: 3,
			wantHas: []runtime.Runtime{runtime.RuntimeNodeJS, runtime.RuntimePython, runtime.RuntimeGo},
		},
		{
			name: "unknown framework",
			apps: []state.DeployedApp{
				{Framework: "Unknown"},
			},
			wantLen: 0,
			wantHas: []runtime.Runtime{},
		},
		{
			name:    "empty apps",
			apps:    []state.DeployedApp{},
			wantLen: 0,
			wantHas: []runtime.Runtime{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runtime.GetRequiredRuntimesForApps(tt.apps)

			if len(got) != tt.wantLen {
				t.Errorf("GetRequiredRuntimesForApps() returned %d runtimes, want %d", len(got), tt.wantLen)
			}

			for _, rt := range tt.wantHas {
				if !got[rt] {
					t.Errorf("GetRequiredRuntimesForApps() missing expected runtime %q", rt)
				}
			}
		})
	}
}

func TestGetRuntimeInfoPackageNames(t *testing.T) {
	// Test specific package names to ensure they're correct
	tests := []struct {
		runtime     runtime.Runtime
		wantPackage string
	}{
		{runtime.RuntimeNodeJS, "nodejs"},
		{runtime.RuntimeNodeJS, "npm"},
		{runtime.RuntimePython, "python3-pip"},
		{runtime.RuntimePython, "python3-venv"},
		{runtime.RuntimeGo, "golang-go"},
		{runtime.RuntimePHP, "php"},
		{runtime.RuntimeRuby, "ruby-full"},
		{runtime.RuntimeJava, "default-jre"},
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime)+"_"+tt.wantPackage, func(t *testing.T) {
			info := runtime.GetRuntimeInfo(tt.runtime)
			found := false
			for _, pkg := range info.Packages {
				if pkg == tt.wantPackage {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("GetRuntimeInfo(%q).Packages does not contain %q", tt.runtime, tt.wantPackage)
			}
		})
	}
}

func TestGetRuntimeInfoDirectories(t *testing.T) {
	// Test that Node.js includes important cleanup directories
	nodeInfo := runtime.GetRuntimeInfo(runtime.RuntimeNodeJS)
	expectedDirs := []string{
		"/usr/local/bin/node",
		"/home/deploy/.bun",
		"/home/deploy/.npm",
	}

	for _, dir := range expectedDirs {
		found := false
		for _, d := range nodeInfo.Directories {
			if d == dir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RuntimeNodeJS directories missing %q", dir)
		}
	}

	// Test that Python includes important cleanup directories
	pythonInfo := runtime.GetRuntimeInfo(runtime.RuntimePython)
	expectedPythonDirs := []string{
		"/home/deploy/.local/lib/python*",
		"/home/deploy/.local/bin/poetry",
	}

	for _, dir := range expectedPythonDirs {
		found := false
		for _, d := range pythonInfo.Directories {
			if d == dir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RuntimePython directories missing %q", dir)
		}
	}
}
