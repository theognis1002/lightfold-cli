package detector_test

import (
	"testing"
)

// Astro Static vs SSR Detection Tests
// Ensures proper detection of deployment type (static vs SSR)

func TestAstroStaticDetection(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedLanguage  string
		expectedSignals   []string
		expectedMeta      map[string]string
		minConfidence     float64
	}{
		{
			name: "Astro static site (no adapter)",
			files: map[string]string{
				"astro.config.mjs": `import { defineConfig } from 'astro/config';

export default defineConfig({
  // No adapter = static site generation
});`,
				"package.json": `{
  "name": "astro-static",
  "type": "module",
  "scripts": {
    "dev": "astro dev",
    "build": "astro build",
    "preview": "astro preview"
  },
  "dependencies": {
    "astro": "^4.0.0"
  }
}`,
				"src/pages/index.astro": `---
// Static page
---
<html>
  <body>
    <h1>Hello Astro</h1>
  </body>
</html>`,
			},
			expectedFramework: "Astro",
			expectedLanguage:  "JavaScript/TypeScript",
			expectedSignals:   []string{"astro.config", "package.json has astro", "package.json scripts for astro"},
			expectedMeta: map[string]string{
				"adapter":         "static",
				"run_mode":        "static",
				"deployment_type": "static",
				"build_output":    "dist/",
			},
			minConfidence: 0.9,
		},
		{
			name: "Astro with explicit static adapter",
			files: map[string]string{
				"astro.config.mjs": `import { defineConfig } from 'astro/config';
import staticAdapter from '@astrojs/static';

export default defineConfig({
  adapter: staticAdapter(),
});`,
				"package.json": `{
  "name": "astro-explicit-static",
  "dependencies": {
    "astro": "^4.0.0",
    "@astrojs/static": "^1.0.0"
  }
}`,
				"src/pages/about.astro": "---\n---\n<h1>About</h1>",
			},
			expectedFramework: "Astro",
			expectedLanguage:  "JavaScript/TypeScript",
			expectedSignals:   []string{"astro.config", "package.json has astro"},
			expectedMeta: map[string]string{
				"adapter":         "static",
				"run_mode":        "static",
				"deployment_type": "static",
			},
			minConfidence: 0.9,
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

			// Verify meta fields
			for key, expectedValue := range tt.expectedMeta {
				actualValue, ok := detection.Meta[key]
				if !ok {
					t.Errorf("Expected meta key '%s' not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected meta['%s'] = '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Verify health check is nil for static sites
			if detection.Healthcheck != nil {
				t.Errorf("Expected nil health check for static site, got %v", detection.Healthcheck)
			}

			// Verify run plan includes static site comment
			if len(detection.RunPlan) > 0 && detection.RunPlan[0] != "# Static site - serve dist/ with nginx" {
				t.Errorf("Expected static site run plan, got: %v", detection.RunPlan)
			}
		})
	}
}

func TestAstroSSRDetection(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedLanguage  string
		expectedSignals   []string
		expectedMeta      map[string]string
		minConfidence     float64
	}{
		{
			name: "Astro with Node adapter (SSR)",
			files: map[string]string{
				"astro.config.mjs": `import { defineConfig } from 'astro/config';
import node from '@astrojs/node';

export default defineConfig({
  output: 'server',
  adapter: node({
    mode: 'standalone'
  }),
});`,
				"package.json": `{
  "name": "astro-ssr",
  "dependencies": {
    "astro": "^4.0.0",
    "@astrojs/node": "^7.0.0"
  }
}`,
				"src/pages/api/data.ts": "export const get = () => ({ body: JSON.stringify({ data: 'dynamic' }) });",
			},
			expectedFramework: "Astro",
			expectedLanguage:  "JavaScript/TypeScript",
			expectedSignals:   []string{"astro.config", "package.json has astro"},
			expectedMeta: map[string]string{
				"adapter":  "node",
				"run_mode": "server",
			},
			minConfidence: 0.9,
		},
		{
			name: "Astro with Vercel adapter",
			files: map[string]string{
				"astro.config.mjs": `import { defineConfig } from 'astro/config';
import vercel from '@astrojs/vercel/serverless';

export default defineConfig({
  output: 'server',
  adapter: vercel(),
});`,
				"package.json": `{
  "dependencies": {
    "astro": "^4.0.0",
    "@astrojs/vercel": "^6.0.0"
  }
}`,
			},
			expectedFramework: "Astro",
			expectedLanguage:  "JavaScript/TypeScript",
			expectedMeta: map[string]string{
				"adapter":  "vercel",
				"run_mode": "server",
			},
			minConfidence: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != tt.expectedFramework {
				t.Errorf("Expected framework %s, got %s", tt.expectedFramework, detection.Framework)
			}

			// Verify meta fields
			for key, expectedValue := range tt.expectedMeta {
				actualValue, ok := detection.Meta[key]
				if !ok {
					t.Errorf("Expected meta key '%s' not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected meta['%s'] = '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Verify NO deployment_type for SSR (only static sites get this)
			if deploymentType, ok := detection.Meta["deployment_type"]; ok && deploymentType == "static" {
				t.Errorf("Expected no 'static' deployment_type for SSR app, got '%s'", deploymentType)
			}

			// Verify health check exists for SSR sites
			if detection.Healthcheck == nil {
				t.Errorf("Expected health check for SSR site, got nil")
			}

			// Verify run plan is NOT a static comment
			if len(detection.RunPlan) > 0 {
				firstRunCmd := detection.RunPlan[0]
				if firstRunCmd == "# Static site - serve dist/ with nginx" {
					t.Errorf("Expected SSR run plan, got static site comment")
				}
			}
		})
	}
}

func TestNextJSStaticExport(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedMeta      map[string]string
	}{
		{
			name: "Next.js with output: 'export' (static)",
			files: map[string]string{
				"next.config.js": `/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
}

module.exports = nextConfig`,
				"package.json": `{
  "name": "nextjs-static",
  "dependencies": {
    "next": "^14.0.0",
    "react": "^18.0.0",
    "react-dom": "^18.0.0"
  }
}`,
				"pages/index.js": "export default function Home() { return <h1>Hello</h1>; }",
			},
			expectedFramework: "Next.js",
			expectedMeta: map[string]string{
				"output_mode":     "export",
				"export":          "static",
				"deployment_type": "static",
			},
		},
		{
			name: "Next.js with output: 'standalone' (SSR)",
			files: map[string]string{
				"next.config.js": `module.exports = {
  output: 'standalone',
}`,
				"package.json": `{
  "dependencies": {
    "next": "^14.0.0",
    "react": "^18.0.0",
    "react-dom": "^18.0.0"
  }
}`,
			},
			expectedFramework: "Next.js",
			expectedMeta: map[string]string{
				"output_mode": "standalone",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != tt.expectedFramework {
				t.Errorf("Expected framework %s, got %s", tt.expectedFramework, detection.Framework)
			}

			// Verify meta fields
			for key, expectedValue := range tt.expectedMeta {
				actualValue, ok := detection.Meta[key]
				if !ok {
					t.Errorf("Expected meta key '%s' not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected meta['%s'] = '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Check health check based on deployment type
			isStatic := detection.Meta["deployment_type"] == "static"
			if isStatic && detection.Healthcheck != nil {
				t.Errorf("Expected nil health check for static export, got %v", detection.Healthcheck)
			} else if !isStatic && detection.Healthcheck == nil {
				t.Errorf("Expected health check for SSR mode, got nil")
			}
		})
	}
}

func TestSvelteKitStaticAdapter(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedMeta      map[string]string
	}{
		{
			name: "SvelteKit with static adapter",
			files: map[string]string{
				"svelte.config.js": `import adapter from '@sveltejs/adapter-static';

export default {
  kit: {
    adapter: adapter()
  }
};`,
				"package.json": `{
  "name": "sveltekit-static",
  "dependencies": {
    "@sveltejs/kit": "^2.0.0",
    "@sveltejs/adapter-static": "^3.0.0",
    "svelte": "^4.0.0"
  }
}`,
				"src/routes/+page.svelte": "<h1>Hello SvelteKit</h1>",
			},
			expectedFramework: "Svelte",
			expectedMeta: map[string]string{
				"adapter":         "static",
				"run_mode":        "static",
				"deployment_type": "static",
			},
		},
		{
			name: "SvelteKit with node adapter (SSR)",
			files: map[string]string{
				"svelte.config.js": `import adapter from '@sveltejs/adapter-node';

export default {
  kit: {
    adapter: adapter()
  }
};`,
				"package.json": `{
  "dependencies": {
    "@sveltejs/kit": "^2.0.0",
    "@sveltejs/adapter-node": "^2.0.0",
    "svelte": "^4.0.0"
  }
}`,
			},
			expectedFramework: "Svelte",
			expectedMeta: map[string]string{
				"adapter":  "node",
				"run_mode": "server",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != tt.expectedFramework {
				t.Errorf("Expected framework %s, got %s", tt.expectedFramework, detection.Framework)
			}

			// Verify meta fields
			for key, expectedValue := range tt.expectedMeta {
				actualValue, ok := detection.Meta[key]
				if !ok {
					t.Errorf("Expected meta key '%s' not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected meta['%s'] = '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Check health check based on deployment type
			isStatic := detection.Meta["deployment_type"] == "static"
			if isStatic && detection.Healthcheck != nil {
				t.Errorf("Expected nil health check for static adapter, got %v", detection.Healthcheck)
			} else if !isStatic && detection.Healthcheck == nil {
				t.Errorf("Expected health check for SSR mode, got nil")
			}
		})
	}
}
