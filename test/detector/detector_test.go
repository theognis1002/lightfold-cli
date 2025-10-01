package detector_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"lightfold/pkg/detector"
)

// Test helper to create temporary test project directories
func createTestProject(t *testing.T, files map[string]string) string {
	t.Helper()
	tmpDir := t.TempDir()

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	return tmpDir
}

// Test helper to get DetectFramework result
func captureDetectFramework(t *testing.T, projectPath string) detector.Detection {
	t.Helper()
	return detector.DetectFramework(projectPath)
}

func TestNextJSDetection(t *testing.T) {
	tests := []struct {
		name           string
		files          map[string]string
		expectedFramework string
		expectedSignals   []string
		minConfidence     float64
	}{
		{
			name: "Next.js with config and package.json",
			files: map[string]string{
				"next.config.js": "module.exports = {}",
				"package.json":   `{"dependencies": {"next": "^13.0.0"}, "scripts": {"dev": "next dev", "build": "next build"}}`,
				"pages/index.js": "export default function Home() { return <div>Hello</div> }",
			},
			expectedFramework: "Next.js",
			expectedSignals:   []string{"next.config", "package.json has next", "package.json scripts for next", "pages/ or app/ folder"},
			minConfidence:     0.8,
		},
		{
			name: "Next.js app directory structure",
			files: map[string]string{
				"next.config.ts": "export default {}",
				"package.json":   `{"dependencies": {"next": "^14.0.0"}}`,
				"app/page.tsx":   "export default function Page() { return <div>App Dir</div> }",
			},
			expectedFramework: "Next.js",
			expectedSignals:   []string{"next.config", "package.json has next", "pages/ or app/ folder"},
			minConfidence:     0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != tt.expectedFramework {
				t.Errorf("Expected framework %s, got %s", tt.expectedFramework, detection.Framework)
			}

			if detection.Confidence < tt.minConfidence {
				t.Errorf("Expected confidence >= %f, got %f", tt.minConfidence, detection.Confidence)
			}

			for _, expectedSignal := range tt.expectedSignals {
				found := false
				for _, signal := range detection.Signals {
					if signal == expectedSignal {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected signal '%s' not found in %v", expectedSignal, detection.Signals)
				}
			}
		})
	}
}

func TestDjangoDetection(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedLanguage  string
	}{
		{
			name: "Django with manage.py and requirements",
			files: map[string]string{
				"manage.py":         "#!/usr/bin/env python\nimport django",
				"requirements.txt":  "Django==4.2.0\npsycopg2==2.9.0",
				"myproject/wsgi.py": "import os\nfrom django.core.wsgi import get_wsgi_application",
			},
			expectedFramework: "Django",
			expectedLanguage:  "Python",
		},
		{
			name: "Django with pyproject.toml",
			files: map[string]string{
				"manage.py":       "#!/usr/bin/env python",
				"pyproject.toml":  `[tool.poetry]\nname = "myproject"\n[tool.poetry.dependencies]\nDjango = "^4.2.0"`,
			},
			expectedFramework: "Django",
			expectedLanguage:  "Python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != tt.expectedFramework {
				t.Errorf("Expected framework %s, got %s", tt.expectedFramework, detection.Framework)
			}

			if detection.Language != tt.expectedLanguage {
				t.Errorf("Expected language %s, got %s", tt.expectedLanguage, detection.Language)
			}
		})
	}
}

func TestExpressJSDetection(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"package.json": `{"dependencies": {"express": "^4.18.0"}, "scripts": {"start": "node server.js"}}`,
		"server.js":    `const express = require('express');\nconst app = express();`,
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Express.js" {
		t.Errorf("Expected Express.js, got %s", detection.Framework)
	}

	if detection.Language != "JavaScript/TypeScript" {
		t.Errorf("Expected JavaScript/TypeScript, got %s", detection.Language)
	}
}

func TestFastAPIDetection(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"main.py":         `from fastapi import FastAPI\napp = FastAPI()`,
		"requirements.txt": "fastapi==0.104.0\nuvicorn==0.24.0",
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "FastAPI" {
		t.Errorf("Expected FastAPI, got %s", detection.Framework)
	}

	if detection.Language != "Python" {
		t.Errorf("Expected Python, got %s", detection.Language)
	}
}

func TestGoDetection(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"go.mod":  "module example.com/myapp\ngo 1.21",
		"main.go": "package main\nfunc main() {}",
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Go" {
		t.Errorf("Expected Go, got %s", detection.Framework)
	}

	if detection.Language != "Go" {
		t.Errorf("Expected Go language, got %s", detection.Language)
	}
}

func TestAstroDetection(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"astro.config.mjs": "export default {}",
		"package.json":     `{"dependencies": {"astro": "^3.0.0"}, "scripts": {"dev": "astro dev", "build": "astro build"}}`,
		"src/pages/index.astro": "---\n---\n<html></html>",
		"public/favicon.ico": "binary",
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Astro" {
		t.Errorf("Expected Astro, got %s", detection.Framework)
	}

	expectedSignals := []string{"astro.config", "package.json has astro", "package.json scripts for astro", "src/ and public/ folders"}
	for _, expectedSignal := range expectedSignals {
		found := false
		for _, signal := range detection.Signals {
			if signal == expectedSignal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected signal '%s' not found in %v", expectedSignal, detection.Signals)
		}
	}
}

func TestAngularDetection(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"angular.json":     `{"version": 1, "projects": {}}`,
		"package.json":     `{"dependencies": {"@angular/core": "^16.0.0"}}`,
		"tsconfig.json":    `{"compilerOptions": {}}`,
		"src/app/app.component.ts": "import { Component } from '@angular/core';",
		"src/main.ts":      "import { platformBrowserDynamic } from '@angular/platform-browser-dynamic';",
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Angular" {
		t.Errorf("Expected Angular, got %s", detection.Framework)
	}

	if detection.Language != "TypeScript" {
		t.Errorf("Expected TypeScript, got %s", detection.Language)
	}
}

func TestVueDetection(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"vue.config.js": "module.exports = {}",
		"package.json":  `{"dependencies": {"@vue/cli": "^5.0.0", "vue": "^3.0.0"}}`,
		"src/App.vue": "<template><div></div></template>",
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Vue.js" {
		t.Errorf("Expected Vue.js, got %s", detection.Framework)
	}
}

func TestSvelteDetection(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"svelte.config.js": "export default {}",
		"package.json":     `{"dependencies": {"@sveltejs/kit": "^1.0.0"}}`,
		"src/routes/+page.svelte": "<h1>Welcome</h1>",
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Svelte" {
		t.Errorf("Expected Svelte, got %s", detection.Framework)
	}
}

func TestSpringBootDetection(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
    </parent>
</project>`,
		"src/main/java/Application.java": "public class Application {}",
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Spring Boot" {
		t.Errorf("Expected Spring Boot, got %s", detection.Framework)
	}

	if detection.Language != "Java" {
		t.Errorf("Expected Java, got %s", detection.Language)
	}
}

func TestNestJSDetection(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"nest-cli.json":       `{"collection": "@nestjs/schematics"}`,
		"package.json":        `{"dependencies": {"@nestjs/core": "^10.0.0"}}`,
		"src/main.ts":         "import { NestFactory } from '@nestjs/core';",
		"src/app.module.ts":   "import { Module } from '@nestjs/common';",
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "NestJS" {
		t.Errorf("Expected NestJS, got %s", detection.Framework)
	}

	if detection.Language != "TypeScript" {
		t.Errorf("Expected TypeScript, got %s", detection.Language)
	}
}

func TestFrameworkPriority(t *testing.T) {
	// Test that specific frameworks are preferred over generic ones
	projectPath := createTestProject(t, map[string]string{
		"Dockerfile":     "FROM node:18",
		"next.config.js": "module.exports = {}",
		"package.json":   `{"dependencies": {"next": "^13.0.0"}}`,
	})

	detection := captureDetectFramework(t, projectPath)

	// Next.js should be detected instead of Generic Docker
	if detection.Framework != "Next.js" {
		t.Errorf("Expected Next.js to be preferred over Generic Docker, got %s", detection.Framework)
	}
}

func TestUnknownFrameworkFallback(t *testing.T) {
	// Create a project with no recognizable framework
	projectPath := createTestProject(t, map[string]string{
		"some_file.txt": "random content",
		"data.json":     "{}",
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Unknown" {
		t.Errorf("Expected Unknown framework, got %s", detection.Framework)
	}

	if detection.Confidence != 0.0 {
		t.Errorf("Expected confidence 0.0, got %f", detection.Confidence)
	}

	expectedSignals := []string{"no strong framework signals"}
	if !reflect.DeepEqual(detection.Signals, expectedSignals) {
		t.Errorf("Expected signals %v, got %v", expectedSignals, detection.Signals)
	}
}

func TestScanTreePerformance(t *testing.T) {
	// Create a project with many files to test scanning performance
	files := make(map[string]string)
	for i := 0; i < 50; i++ {
		files[filepath.Join("src", "components", fmt.Sprintf("Component%d.js", i))] = "export default function Component() {}"
	}
	files["package.json"] = `{"dependencies": {"react": "^18.0.0"}}`

	projectPath := createTestProject(t, files)

	// Test that scanning completes quickly
	start := time.Now()
	detector.DetectFramework(projectPath)
	duration := time.Since(start)

	if duration > 100*time.Millisecond {
		t.Errorf("Detection took too long: %v", duration)
	}
}

// Benchmark tests for performance
func BenchmarkDetectFramework(b *testing.B) {
	projectPath := createTestProject(&testing.T{}, map[string]string{
		"next.config.js": "module.exports = {}",
		"package.json":   `{"dependencies": {"next": "^13.0.0"}}`,
		"pages/index.js": "export default function Home() {}",
		"src/components/Header.js": "export default function Header() {}",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		captureDetectFramework(&testing.T{}, projectPath)
	}
}