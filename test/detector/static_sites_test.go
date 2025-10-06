package detector_test

import (
	"testing"
)

// Static Site Generator Detection Tests

func TestHugoDetection(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedLanguage  string
		expectedSignals   []string
		minConfidence     float64
	}{
		{
			name: "Hugo with config.toml and content",
			files: map[string]string{
				"config.toml": `baseURL = 'https://example.com/'
languageCode = 'en-us'
title = 'My Hugo Site'
theme = 'ananke'`,
				"content/posts/first-post.md": `---
title: "First Post"
date: 2024-01-01
---

# Hello World`,
				"themes/ananke/theme.toml": "name = 'Ananke'",
			},
			expectedFramework: "Hugo",
			expectedLanguage:  "Go",
			expectedSignals:   []string{"hugo config file", "content/ directory", "themes/ directory"},
			minConfidence:     0.8,
		},
		{
			name: "Hugo with hugo.yaml",
			files: map[string]string{
				"hugo.yaml": `baseURL: https://example.com/
languageCode: en-us
title: My Hugo Site`,
				"content/posts/post.md": "# Post",
			},
			expectedFramework: "Hugo",
			expectedLanguage:  "Go",
			expectedSignals:   []string{"hugo config file", "content/ directory"},
			minConfidence:     0.8,
		},
		{
			name: "Hugo minimal setup",
			files: map[string]string{
				"config.yaml":       `title: Hugo Site`,
				"content/_index.md": "# Home",
			},
			expectedFramework: "Hugo",
			expectedLanguage:  "Go",
			expectedSignals:   []string{"hugo config file", "content/ directory"},
			minConfidence:     0.8,
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

			// Verify build output
			if detection.Meta["build_output"] != "public/" {
				t.Errorf("Expected build_output 'public/', got %s", detection.Meta["build_output"])
			}

			// Verify static flag
			if detection.Meta["static"] != "true" {
				t.Errorf("Expected static 'true', got %s", detection.Meta["static"])
			}
		})
	}
}

func TestEleventyDetection(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedLanguage  string
		expectedSignals   []string
		packageManager    string
		minConfidence     float64
	}{
		{
			name: "Eleventy with .eleventy.js",
			files: map[string]string{
				".eleventy.js": `module.exports = function(eleventyConfig) {
  eleventyConfig.addPassthroughCopy("css");
  return {
    dir: {
      input: "src",
      output: "_site"
    }
  };
};`,
				"package.json": `{
  "name": "eleventy-site",
  "scripts": {
    "build": "eleventy",
    "serve": "eleventy --serve"
  },
  "dependencies": {
    "@11ty/eleventy": "^2.0.1"
  }
}`,
				"src/index.md":   "# Hello Eleventy",
				"pnpm-lock.yaml": "lockfileVersion: '6.0'",
			},
			expectedFramework: "Eleventy",
			expectedLanguage:  "JavaScript/TypeScript",
			expectedSignals:   []string{"eleventy config", "package.json has @11ty/eleventy"},
			packageManager:    "pnpm",
			minConfidence:     0.9,
		},
		{
			name: "Eleventy with eleventy.config.js",
			files: map[string]string{
				"eleventy.config.js": `module.exports = {
  dir: {
    input: ".",
    output: "_site"
  }
};`,
				"package.json": `{
  "dependencies": {
    "@11ty/eleventy": "^2.0.0"
  }
}`,
			},
			expectedFramework: "Eleventy",
			expectedLanguage:  "JavaScript/TypeScript",
			expectedSignals:   []string{"eleventy config", "package.json has @11ty/eleventy"},
			packageManager:    "npm",
			minConfidence:     0.9,
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

			// Verify package manager
			if detection.Meta["package_manager"] != tt.packageManager {
				t.Errorf("Expected package manager %s, got %s", tt.packageManager, detection.Meta["package_manager"])
			}

			// Verify build output
			if detection.Meta["build_output"] != "_site/" {
				t.Errorf("Expected build_output '_site/', got %s", detection.Meta["build_output"])
			}

			// Verify static flag
			if detection.Meta["static"] != "true" {
				t.Errorf("Expected static 'true', got %s", detection.Meta["static"])
			}
		})
	}
}

func TestJekyllDetection(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedLanguage  string
		expectedSignals   []string
		minConfidence     float64
	}{
		{
			name: "Jekyll with _config.yml and Gemfile",
			files: map[string]string{
				"_config.yml": `title: My Jekyll Site
description: A blog built with Jekyll
baseurl: ""
url: "https://example.com"

theme: minima`,
				"Gemfile": `source "https://rubygems.org"

gem "jekyll", "~> 4.3.2"
gem "minima", "~> 2.5"

group :jekyll_plugins do
  gem "jekyll-feed", "~> 0.12"
end`,
				"_posts/2024-01-01-welcome.md": `---
layout: post
title:  "Welcome to Jekyll!"
date:   2024-01-01 10:00:00 -0500
---

Hello Jekyll!`,
				"index.md": "# Home",
			},
			expectedFramework: "Jekyll",
			expectedLanguage:  "Ruby",
			expectedSignals:   []string{"_config.yml", "jekyll in Gemfile", "_posts/ or _site/ directory"},
			minConfidence:     0.9,
		},
		{
			name: "Jekyll minimal setup",
			files: map[string]string{
				"_config.yml": `title: Site`,
				"Gemfile":     `gem "jekyll"`,
			},
			expectedFramework: "Jekyll",
			expectedLanguage:  "Ruby",
			expectedSignals:   []string{"_config.yml", "jekyll in Gemfile"},
			minConfidence:     0.9,
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

			// Verify build output
			if detection.Meta["build_output"] != "_site/" {
				t.Errorf("Expected build_output '_site/', got %s", detection.Meta["build_output"])
			}

			// Verify static flag
			if detection.Meta["static"] != "true" {
				t.Errorf("Expected static 'true', got %s", detection.Meta["static"])
			}
		})
	}
}

func TestDocusaurusDetection(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedLanguage  string
		expectedSignals   []string
		packageManager    string
		minConfidence     float64
	}{
		{
			name: "Docusaurus with docusaurus.config.js",
			files: map[string]string{
				"docusaurus.config.js": `module.exports = {
  title: 'My Site',
  tagline: 'Dinosaurs are cool',
  url: 'https://example.com',
  baseUrl: '/',
  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',
  favicon: 'img/favicon.ico',
  organizationName: 'myorg',
  projectName: 'myproject',
  themeConfig: {
    navbar: {
      title: 'My Site',
    },
  },
  presets: [
    [
      '@docusaurus/preset-classic',
      {
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
        },
        blog: {
          showReadingTime: true,
        },
      },
    ],
  ],
};`,
				"package.json": `{
  "name": "docusaurus-site",
  "scripts": {
    "build": "docusaurus build",
    "serve": "docusaurus serve"
  },
  "dependencies": {
    "@docusaurus/core": "^3.0.0",
    "@docusaurus/preset-classic": "^3.0.0",
    "react": "^18.0.0",
    "react-dom": "^18.0.0"
  }
}`,
				"docs/intro.md": "# Introduction\n\nWelcome!",
				"blog/hello.md": "# Hello Blog",
				"yarn.lock":     "# yarn lock",
			},
			expectedFramework: "Docusaurus",
			expectedLanguage:  "JavaScript/TypeScript",
			expectedSignals:   []string{"docusaurus config", "package.json has @docusaurus/core", "docs/ or blog/ directory"},
			packageManager:    "yarn",
			minConfidence:     0.9,
		},
		{
			name: "Docusaurus with TypeScript config",
			files: map[string]string{
				"docusaurus.config.ts": `import type {Config} from '@docusaurus/types';

const config: Config = {
  title: 'My Site',
  tagline: 'Tagline',
  url: 'https://example.com',
  baseUrl: '/',
};

export default config;`,
				"package.json": `{
  "dependencies": {
    "@docusaurus/core": "^3.0.0"
  }
}`,
				"docs/getting-started.md": "# Getting Started",
			},
			expectedFramework: "Docusaurus",
			expectedLanguage:  "JavaScript/TypeScript",
			expectedSignals:   []string{"docusaurus config", "package.json has @docusaurus/core", "docs/ or blog/ directory"},
			packageManager:    "npm",
			minConfidence:     0.9,
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

			// Verify package manager
			if detection.Meta["package_manager"] != tt.packageManager {
				t.Errorf("Expected package manager %s, got %s", tt.packageManager, detection.Meta["package_manager"])
			}

			// Verify build output
			if detection.Meta["build_output"] != "build/" {
				t.Errorf("Expected build_output 'build/', got %s", detection.Meta["build_output"])
			}
		})
	}
}
