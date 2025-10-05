package detector_test

import (
	"testing"
)

// Build Output Directory Tracking Tests

func TestNextJSBuildOutput(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		buildOutput string
	}{
		{
			name: "Next.js with default SSR mode",
			files: map[string]string{
				"next.config.js": `module.exports = {
  reactStrictMode: true,
}`,
				"package.json": `{"dependencies": {"next": "^14.0.0"}}`,
				"pages/index.js": "export default function Home() { return <div>Home</div> }",
			},
			buildOutput: ".next/",
		},
		{
			name: "Next.js with static export",
			files: map[string]string{
				"next.config.js": `module.exports = {
  output: 'export',
  images: {
    unoptimized: true,
  },
}`,
				"package.json": `{"dependencies": {"next": "^14.0.0"}}`,
				"pages/index.js": "export default function Home() { return <div>Home</div> }",
			},
			buildOutput: "out/",
		},
		{
			name: "Next.js with static export (double quotes)",
			files: map[string]string{
				"next.config.js": `module.exports = {
  output: "export"
}`,
				"package.json": `{"dependencies": {"next": "^14.0.0"}}`,
			},
			buildOutput: "out/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Next.js" {
				t.Errorf("Expected framework Next.js, got %s", detection.Framework)
			}

			if detection.Meta["build_output"] != tt.buildOutput {
				t.Errorf("Expected build_output '%s', got '%s'", tt.buildOutput, detection.Meta["build_output"])
			}
		})
	}
}

func TestAstroBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"astro.config.mjs": `export default {}`,
		"package.json": `{"dependencies": {"astro": "^4.0.0"}}`,
		"src/pages/index.astro": "---\n---\n<h1>Hello Astro</h1>",
	})
	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Astro" {
		t.Errorf("Expected framework Astro, got %s", detection.Framework)
	}

	if detection.Meta["build_output"] != "dist/" {
		t.Errorf("Expected build_output 'dist/', got '%s'", detection.Meta["build_output"])
	}
}

func TestGatsbyBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"gatsby-config.js": `module.exports = {}`,
		"package.json": `{"dependencies": {"gatsby": "^5.0.0"}}`,
		"src/pages/index.js": "export default function Home() {}",
	})
	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Gatsby" {
		t.Errorf("Expected framework Gatsby, got %s", detection.Framework)
	}

	if detection.Meta["build_output"] != "public/" {
		t.Errorf("Expected build_output 'public/', got '%s'", detection.Meta["build_output"])
	}
}

func TestSvelteBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"svelte.config.js": `export default {}`,
		"package.json": `{"dependencies": {"@sveltejs/kit": "^2.0.0"}}`,
		"src/routes/+page.svelte": "<h1>Welcome</h1>",
	})
	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Svelte" {
		t.Errorf("Expected framework Svelte, got %s", detection.Framework)
	}

	if detection.Meta["build_output"] != "build/" {
		t.Errorf("Expected build_output 'build/', got '%s'", detection.Meta["build_output"])
	}
}

func TestVueBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"vite.config.js": `export default {}`,
		"package.json": `{"dependencies": {"vue": "^3.3.0"}}`,
		"src/App.vue": "<template><div>App</div></template>",
	})
	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Vue.js" {
		t.Errorf("Expected framework Vue.js, got %s", detection.Framework)
	}

	if detection.Meta["build_output"] != "dist/" {
		t.Errorf("Expected build_output 'dist/', got '%s'", detection.Meta["build_output"])
	}
}

func TestAngularBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"angular.json": `{"projects": {}}`,
		"package.json": `{"dependencies": {"@angular/core": "^17.0.0"}}`,
		"src/main.ts": "import { platformBrowserDynamic } from '@angular/platform-browser-dynamic';",
	})
	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Angular" {
		t.Errorf("Expected framework Angular, got %s", detection.Framework)
	}

	if detection.Meta["build_output"] != "dist/" {
		t.Errorf("Expected build_output 'dist/', got '%s'", detection.Meta["build_output"])
	}
}

func TestSpringBootBuildOutput(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		buildOutput string
		buildTool   string
	}{
		{
			name: "Spring Boot with Maven",
			files: map[string]string{
				"pom.xml": `<?xml version="1.0"?>
<project>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
  </dependencies>
</project>`,
				"src/main/java/Application.java": "public class Application {}",
			},
			buildOutput: "target/",
			buildTool:   "maven",
		},
		{
			name: "Spring Boot with Gradle",
			files: map[string]string{
				"build.gradle": `plugins {
  id 'org.springframework.boot' version '3.2.0'
}

dependencies {
  implementation 'org.springframework.boot:spring-boot-starter-web'
}`,
				"src/main/java/Application.java": "public class Application {}",
			},
			buildOutput: "build/",
			buildTool:   "gradle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Spring Boot" {
				t.Errorf("Expected framework Spring Boot, got %s", detection.Framework)
			}

			if detection.Meta["build_output"] != tt.buildOutput {
				t.Errorf("Expected build_output '%s', got '%s'", tt.buildOutput, detection.Meta["build_output"])
			}

			if detection.Meta["build_tool"] != tt.buildTool {
				t.Errorf("Expected build_tool '%s', got '%s'", tt.buildTool, detection.Meta["build_tool"])
			}
		})
	}
}

func TestASPNetBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"MyApp.csproj": `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>`,
		"Program.cs": "var builder = WebApplication.CreateBuilder(args);",
	})
	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "ASP.NET Core" {
		t.Errorf("Expected framework ASP.NET Core, got %s", detection.Framework)
	}

	if detection.Meta["build_output"] != "out/" {
		t.Errorf("Expected build_output 'out/', got '%s'", detection.Meta["build_output"])
	}
}

func TestRemixBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"remix.config.js": `module.exports = {}`,
		"package.json": `{"dependencies": {"@remix-run/react": "^2.0.0"}}`,
		"app/root.tsx": "export default function App() {}",
	})
	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Remix" {
		t.Errorf("Expected framework Remix, got %s", detection.Framework)
	}

	if detection.Meta["build_output"] != "build/" {
		t.Errorf("Expected build_output 'build/', got '%s'", detection.Meta["build_output"])
	}
}

func TestNuxtBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"nuxt.config.js": `export default {}`,
		"package.json": `{"dependencies": {"nuxt": "^3.0.0"}}`,
		"app.vue": "<template><div>App</div></template>",
	})
	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Nuxt.js" {
		t.Errorf("Expected framework Nuxt.js, got %s", detection.Framework)
	}

	if detection.Meta["build_output"] != ".output/" {
		t.Errorf("Expected build_output '.output/', got '%s'", detection.Meta["build_output"])
	}
}

func TestStaticSiteGeneratorsBuildOutput(t *testing.T) {
	tests := []struct {
		name        string
		framework   string
		files       map[string]string
		buildOutput string
		isStatic    bool
	}{
		{
			name:      "Hugo",
			framework: "Hugo",
			files: map[string]string{
				"config.toml": "title = 'Site'",
				"content/post.md": "# Post",
			},
			buildOutput: "public/",
			isStatic:    true,
		},
		{
			name:      "Eleventy",
			framework: "Eleventy",
			files: map[string]string{
				".eleventy.js": "module.exports = {}",
				"package.json": `{"dependencies": {"@11ty/eleventy": "^2.0.0"}}`,
			},
			buildOutput: "_site/",
			isStatic:    true,
		},
		{
			name:      "Jekyll",
			framework: "Jekyll",
			files: map[string]string{
				"_config.yml": "title: Site",
				"Gemfile": "gem 'jekyll'",
			},
			buildOutput: "_site/",
			isStatic:    true,
		},
		{
			name:      "Docusaurus",
			framework: "Docusaurus",
			files: map[string]string{
				"docusaurus.config.js": "module.exports = {}",
				"package.json": `{"dependencies": {"@docusaurus/core": "^3.0.0"}}`,
				"docs/intro.md": "# Intro",
			},
			buildOutput: "build/",
			isStatic:    false, // Docusaurus can be SSR
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != tt.framework {
				t.Errorf("Expected framework %s, got %s", tt.framework, detection.Framework)
			}

			if detection.Meta["build_output"] != tt.buildOutput {
				t.Errorf("Expected build_output '%s', got '%s'", tt.buildOutput, detection.Meta["build_output"])
			}

			if tt.isStatic {
				if detection.Meta["static"] != "true" {
					t.Errorf("Expected static 'true', got '%s'", detection.Meta["static"])
				}
			}
		})
	}
}
