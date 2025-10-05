package detector_test

import (
	"testing"
)

// Framework-Specific Nuances Tests (Next.js)

func TestNextJSAppRouterDetection(t *testing.T) {
	tests := []struct {
		name           string
		files          map[string]string
		expectedRouter string
		expectedOutput string
	}{
		{
			name: "Next.js with App Router (app directory)",
			files: map[string]string{
				"next.config.js": "module.exports = {}",
				"package.json":   `{"dependencies": {"next": "^14.0.0"}}`,
				"app/page.tsx":   "export default function Page() { return <div>App Router</div> }",
				"app/layout.tsx": "export default function Layout({ children }) { return children }",
			},
			expectedRouter: "app",
			expectedOutput: ".next/",
		},
		{
			name: "Next.js with Pages Router (pages directory)",
			files: map[string]string{
				"next.config.js":   "module.exports = {}",
				"package.json":     `{"dependencies": {"next": "^13.0.0"}}`,
				"pages/index.tsx":  "export default function Home() { return <div>Pages Router</div> }",
				"pages/_app.tsx":   "export default function App({ Component, pageProps }) { return <Component {...pageProps} /> }",
			},
			expectedRouter: "pages",
			expectedOutput: ".next/",
		},
		{
			name: "Next.js with both app and pages (app takes priority)",
			files: map[string]string{
				"next.config.js":  "module.exports = {}",
				"package.json":    `{"dependencies": {"next": "^14.0.0"}}`,
				"app/page.tsx":    "export default function Page() { return <div>App Router</div> }",
				"pages/index.tsx": "export default function Home() { return <div>Pages Router</div> }",
			},
			expectedRouter: "app",
			expectedOutput: ".next/",
		},
		{
			name: "Next.js without router directories",
			files: map[string]string{
				"next.config.js": "module.exports = {}",
				"package.json":   `{"dependencies": {"next": "^13.0.0"}}`,
				"public/favicon.ico": "binary",
			},
			expectedRouter: "", // No router detected
			expectedOutput: ".next/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Next.js" {
				t.Errorf("Expected framework Next.js, got %s", detection.Framework)
			}

			if tt.expectedRouter != "" {
				if detection.Meta["router"] != tt.expectedRouter {
					t.Errorf("Expected router %s, got %s", tt.expectedRouter, detection.Meta["router"])
				}
			} else {
				if _, exists := detection.Meta["router"]; exists {
					t.Errorf("Expected no router metadata, but got %s", detection.Meta["router"])
				}
			}

			if detection.Meta["build_output"] != tt.expectedOutput {
				t.Errorf("Expected build_output %s, got %s", tt.expectedOutput, detection.Meta["build_output"])
			}
		})
	}
}

func TestNextJSStaticExportDetection(t *testing.T) {
	tests := []struct {
		name           string
		files          map[string]string
		expectedExport string
		expectedOutput string
	}{
		{
			name: "Next.js static export with single quotes",
			files: map[string]string{
				"next.config.js": `module.exports = {
  output: 'export',
  images: { unoptimized: true }
}`,
				"package.json":  `{"dependencies": {"next": "^14.0.0"}}`,
				"pages/index.js": "export default function Home() { return <div>Static Export</div> }",
			},
			expectedExport: "static",
			expectedOutput: "out/",
		},
		{
			name: "Next.js static export with double quotes",
			files: map[string]string{
				"next.config.js": `module.exports = {
  output: "export",
  trailingSlash: true
}`,
				"package.json":  `{"dependencies": {"next": "^14.0.0"}}`,
				"pages/index.js": "export default function Home() { return <div>Static Export</div> }",
			},
			expectedExport: "static",
			expectedOutput: "out/",
		},
		{
			name: "Next.js static export with TypeScript config",
			files: map[string]string{
				"next.config.ts": `import type { NextConfig } from 'next'

const config: NextConfig = {
  output: 'export',
  distDir: 'dist'
}

export default config`,
				"package.json":  `{"dependencies": {"next": "^14.0.0"}}`,
				"pages/index.tsx": "export default function Home() { return <div>Static Export</div> }",
			},
			expectedExport: "static",
			expectedOutput: "out/",
		},
		{
			name: "Next.js without static export (SSR mode)",
			files: map[string]string{
				"next.config.js": `module.exports = {
  reactStrictMode: true,
  swcMinify: true
}`,
				"package.json":  `{"dependencies": {"next": "^14.0.0"}}`,
				"pages/index.js": "export default function Home() { return <div>SSR</div> }",
			},
			expectedExport: "", // No export mode
			expectedOutput: ".next/",
		},
		{
			name: "Next.js with output: standalone",
			files: map[string]string{
				"next.config.js": `module.exports = {
  output: 'standalone'
}`,
				"package.json":  `{"dependencies": {"next": "^14.0.0"}}`,
				"pages/index.js": "export default function Home() { return <div>Standalone</div> }",
			},
			expectedExport: "", // Not static export
			expectedOutput: ".next/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Next.js" {
				t.Errorf("Expected framework Next.js, got %s", detection.Framework)
			}

			if tt.expectedExport != "" {
				if detection.Meta["export"] != tt.expectedExport {
					t.Errorf("Expected export mode %s, got %s", tt.expectedExport, detection.Meta["export"])
				}
			} else {
				if _, exists := detection.Meta["export"]; exists {
					t.Errorf("Expected no export metadata, but got %s", detection.Meta["export"])
				}
			}

			if detection.Meta["build_output"] != tt.expectedOutput {
				t.Errorf("Expected build_output %s, got %s", tt.expectedOutput, detection.Meta["build_output"])
			}
		})
	}
}

func TestNextJSCombinedNuances(t *testing.T) {
	tests := []struct {
		name           string
		files          map[string]string
		expectedRouter string
		expectedExport string
		expectedOutput string
	}{
		{
			name: "Next.js App Router with static export",
			files: map[string]string{
				"next.config.js": `module.exports = {
  output: 'export'
}`,
				"package.json": `{"dependencies": {"next": "^14.0.0"}}`,
				"app/page.tsx":   "export default function Page() { return <div>App Router + Static</div> }",
				"app/layout.tsx": "export default function Layout({ children }) { return children }",
			},
			expectedRouter: "app",
			expectedExport: "static",
			expectedOutput: "out/",
		},
		{
			name: "Next.js Pages Router with static export",
			files: map[string]string{
				"next.config.js": `module.exports = {
  output: 'export',
  trailingSlash: true
}`,
				"package.json":    `{"dependencies": {"next": "^14.0.0"}}`,
				"pages/index.tsx": "export default function Home() { return <div>Pages + Static</div> }",
				"pages/_app.tsx":  "export default function App({ Component, pageProps }) { return <Component {...pageProps} /> }",
			},
			expectedRouter: "pages",
			expectedExport: "static",
			expectedOutput: "out/",
		},
		{
			name: "Next.js App Router without static export (SSR)",
			files: map[string]string{
				"next.config.js": `module.exports = {
  reactStrictMode: true
}`,
				"package.json": `{"dependencies": {"next": "^14.0.0"}}`,
				"app/page.tsx":   "export default function Page() { return <div>App Router SSR</div> }",
				"app/layout.tsx": "export default function Layout({ children }) { return children }",
			},
			expectedRouter: "app",
			expectedExport: "",
			expectedOutput: ".next/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Next.js" {
				t.Errorf("Expected framework Next.js, got %s", detection.Framework)
			}

			// Check router
			if detection.Meta["router"] != tt.expectedRouter {
				t.Errorf("Expected router %s, got %s", tt.expectedRouter, detection.Meta["router"])
			}

			// Check export mode
			if tt.expectedExport != "" {
				if detection.Meta["export"] != tt.expectedExport {
					t.Errorf("Expected export mode %s, got %s", tt.expectedExport, detection.Meta["export"])
				}
			} else {
				if _, exists := detection.Meta["export"]; exists {
					t.Errorf("Expected no export metadata, but got %s", detection.Meta["export"])
				}
			}

			// Check build output
			if detection.Meta["build_output"] != tt.expectedOutput {
				t.Errorf("Expected build_output %s, got %s", tt.expectedOutput, detection.Meta["build_output"])
			}
		})
	}
}

func TestNextJSPackageManagerWithNuances(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedPM      string
		expectedRouter  string
		expectedOutput  string
	}{
		{
			name: "Next.js App Router with pnpm",
			files: map[string]string{
				"next.config.js":   "module.exports = {}",
				"package.json":     `{"dependencies": {"next": "^14.0.0"}}`,
				"pnpm-lock.yaml":   "lockfileVersion: '6.0'",
				"app/page.tsx":     "export default function Page() { return <div>App</div> }",
			},
			expectedPM:     "pnpm",
			expectedRouter: "app",
			expectedOutput: ".next/",
		},
		{
			name: "Next.js Pages Router with bun",
			files: map[string]string{
				"next.config.js":  "module.exports = {}",
				"package.json":    `{"dependencies": {"next": "^13.0.0"}}`,
				"bun.lockb":       "binary lock file",
				"pages/index.tsx": "export default function Home() { return <div>Pages</div> }",
			},
			expectedPM:     "bun",
			expectedRouter: "pages",
			expectedOutput: ".next/",
		},
		{
			name: "Next.js with Yarn and static export",
			files: map[string]string{
				"next.config.js": `module.exports = { output: 'export' }`,
				"package.json":   `{"dependencies": {"next": "^14.0.0"}}`,
				"yarn.lock":      "# THIS IS AN AUTOGENERATED FILE",
				"pages/index.js": "export default function Home() { return <div>Static</div> }",
			},
			expectedPM:     "yarn",
			expectedRouter: "pages",
			expectedOutput: "out/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Next.js" {
				t.Errorf("Expected framework Next.js, got %s", detection.Framework)
			}

			if detection.Meta["package_manager"] != tt.expectedPM {
				t.Errorf("Expected package_manager %s, got %s", tt.expectedPM, detection.Meta["package_manager"])
			}

			if detection.Meta["router"] != tt.expectedRouter {
				t.Errorf("Expected router %s, got %s", tt.expectedRouter, detection.Meta["router"])
			}

			if detection.Meta["build_output"] != tt.expectedOutput {
				t.Errorf("Expected build_output %s, got %s", tt.expectedOutput, detection.Meta["build_output"])
			}
		})
	}
}

// Vue.js Version Detection Tests

func TestVue2Detection(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedVersion string
	}{
		{
			name: "Vue 2 with caret version",
			files: map[string]string{
				"package.json": `{
					"name": "my-vue-app",
					"dependencies": {
						"vue": "^2.6.14"
					}
				}`,
				"src/App.vue":  "<template><div>App</div></template>",
				"src/main.js":  "import Vue from 'vue'\nnew Vue({ render: h => h(App) }).$mount('#app')",
			},
			expectedVersion: "2",
		},
		{
			name: "Vue 2 with tilde version",
			files: map[string]string{
				"package.json": `{
					"name": "my-vue-app",
					"dependencies": {
						"vue": "~2.7.0"
					}
				}`,
				"src/App.vue": "<template><div>App</div></template>",
			},
			expectedVersion: "2",
		},
		{
			name: "Vue 2 with exact version",
			files: map[string]string{
				"package.json": `{
					"name": "my-vue-app",
					"dependencies": {
						"vue": "2.6.14"
					}
				}`,
				"src/App.vue": "<template><div>App</div></template>",
			},
			expectedVersion: "2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Vue.js" {
				t.Errorf("Expected framework Vue.js, got %s", detection.Framework)
			}

			if detection.Meta["vue_version"] != tt.expectedVersion {
				t.Errorf("Expected vue_version %s, got %s", tt.expectedVersion, detection.Meta["vue_version"])
			}
		})
	}
}

func TestVue3Detection(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedVersion string
	}{
		{
			name: "Vue 3 with caret version",
			files: map[string]string{
				"package.json": `{
					"name": "my-vue-app",
					"dependencies": {
						"vue": "^3.3.0"
					}
				}`,
				"src/App.vue":  "<template><div>App</div></template>",
				"src/main.js":  "import { createApp } from 'vue'\ncreateApp(App).mount('#app')",
			},
			expectedVersion: "3",
		},
		{
			name: "Vue 3 with tilde version",
			files: map[string]string{
				"package.json": `{
					"name": "my-vue-app",
					"dependencies": {
						"vue": "~3.2.0"
					}
				}`,
				"src/App.vue": "<template><div>App</div></template>",
			},
			expectedVersion: "3",
		},
		{
			name: "Vue 3 with exact version",
			files: map[string]string{
				"package.json": `{
					"name": "my-vue-app",
					"dependencies": {
						"vue": "3.3.4"
					}
				}`,
				"src/App.vue": "<template><div>App</div></template>",
			},
			expectedVersion: "3",
		},
		{
			name: "Vue 3 with Vite",
			files: map[string]string{
				"vite.config.js": `import { defineConfig } from 'vite'\nexport default defineConfig({})`,
				"package.json": `{
					"name": "my-vue-app",
					"dependencies": {
						"vue": "^3.3.0"
					},
					"devDependencies": {
						"vite": "^4.0.0",
						"@vitejs/plugin-vue": "^4.0.0"
					}
				}`,
				"src/App.vue":  "<template><div>App</div></template>",
			},
			expectedVersion: "3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Vue.js" {
				t.Errorf("Expected framework Vue.js, got %s", detection.Framework)
			}

			if detection.Meta["vue_version"] != tt.expectedVersion {
				t.Errorf("Expected vue_version %s, got %s", tt.expectedVersion, detection.Meta["vue_version"])
			}
		})
	}
}

func TestVueVersionNotDetected(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{
			name: "Vue without version in package.json",
			files: map[string]string{
				"vue.config.js": "module.exports = {}",
				"src/App.vue":   "<template><div>App</div></template>",
			},
		},
		{
			name: "Vue with non-standard version format",
			files: map[string]string{
				"package.json": `{
					"name": "my-vue-app",
					"dependencies": {
						"vue": "next"
					}
				}`,
				"src/App.vue": "<template><div>App</div></template>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Vue.js" {
				t.Errorf("Expected framework Vue.js, got %s", detection.Framework)
			}

			// Should not have vue_version when it can't be determined
			if _, exists := detection.Meta["vue_version"]; exists {
				t.Errorf("Expected no vue_version, but got %s", detection.Meta["vue_version"])
			}
		})
	}
}
