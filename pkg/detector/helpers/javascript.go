package helpers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// NextConfig represents parsed Next.js configuration
type NextConfig struct {
	OutputMode   string // "standalone", "export", or "default"
	Router       string // "app", "pages", or "unknown"
	BuildOutput  string // build output directory
}

// PackageJSON represents parsed package.json
type PackageJSON struct {
	Scripts      map[string]string      `json:"scripts"`
	Dependencies map[string]string      `json:"dependencies"`
	DevDeps      map[string]string      `json:"devDependencies"`
}

// FrameworkAdapter represents detected adapter information
type FrameworkAdapter struct {
	Type     string // e.g., "node", "static", "cloudflare", "vercel"
	Package  string // e.g., "@sveltejs/adapter-node"
	RunMode  string // "server" or "static"
}

// ParseNextConfig parses Next.js configuration files
func ParseNextConfig(root string) NextConfig {
	config := NextConfig{
		OutputMode:  "default",
		Router:      "", // empty means no router detected
		BuildOutput: ".next/",
	}

	// Detect router type
	if dirExists(root, "app") {
		config.Router = "app"
	} else if dirExists(root, "pages") {
		config.Router = "pages"
	}

	// Parse config files for output mode
	for _, configFile := range []string{"next.config.js", "next.config.ts", "next.config.mjs"} {
		if content := readFile(root, configFile); content != "" {
			// Check for standalone output
			if strings.Contains(content, "output: 'standalone'") ||
				strings.Contains(content, `output: "standalone"`) ||
				strings.Contains(content, "output:'standalone'") {
				config.OutputMode = "standalone"
				config.BuildOutput = ".next/standalone/"
			}
			// Check for static export
			if strings.Contains(content, "output: 'export'") ||
				strings.Contains(content, `output: "export"`) ||
				strings.Contains(content, "output:'export'") {
				config.OutputMode = "export"
				config.BuildOutput = "out/"
			}
			break
		}
	}

	return config
}

// ParsePackageJSON parses package.json and returns structured data
func ParsePackageJSON(root string) PackageJSON {
	var pkg PackageJSON

	content := readFile(root, "package.json")
	if content == "" {
		return pkg
	}

	// Try to unmarshal JSON
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		// If JSON parsing fails, initialize empty maps
		pkg.Scripts = make(map[string]string)
		pkg.Dependencies = make(map[string]string)
		pkg.DevDeps = make(map[string]string)
	}

	return pkg
}

// DetectFrameworkAdapter detects the deployment adapter for various frameworks
func DetectFrameworkAdapter(pkg PackageJSON, framework string) FrameworkAdapter {
	adapter := FrameworkAdapter{
		Type:    "unknown",
		RunMode: "server",
	}

	allDeps := mergeDeps(pkg.Dependencies, pkg.DevDeps)

	switch framework {
	case "remix":
		// Check for Remix adapters
		for dep := range allDeps {
			switch {
			case strings.Contains(dep, "@remix-run/cloudflare"):
				adapter.Type = "cloudflare"
				adapter.Package = dep
				adapter.RunMode = "server"
			case strings.Contains(dep, "@remix-run/deno"):
				adapter.Type = "deno"
				adapter.Package = dep
				adapter.RunMode = "server"
			case strings.Contains(dep, "@remix-run/node"):
				adapter.Type = "node"
				adapter.Package = dep
				adapter.RunMode = "server"
			}
		}
		// Default to node if no specific adapter found but @remix-run/react exists
		if adapter.Type == "unknown" && allDeps["@remix-run/react"] != "" {
			adapter.Type = "node"
			adapter.RunMode = "server"
		}

	case "svelte", "sveltekit":
		// Check for SvelteKit adapters
		for dep := range allDeps {
			switch {
			case strings.Contains(dep, "@sveltejs/adapter-static"):
				adapter.Type = "static"
				adapter.Package = dep
				adapter.RunMode = "static"
			case strings.Contains(dep, "@sveltejs/adapter-node"):
				adapter.Type = "node"
				adapter.Package = dep
				adapter.RunMode = "server"
			case strings.Contains(dep, "@sveltejs/adapter-vercel"):
				adapter.Type = "vercel"
				adapter.Package = dep
				adapter.RunMode = "server"
			case strings.Contains(dep, "@sveltejs/adapter-netlify"):
				adapter.Type = "netlify"
				adapter.Package = dep
				adapter.RunMode = "server"
			case strings.Contains(dep, "@sveltejs/adapter-cloudflare"):
				adapter.Type = "cloudflare"
				adapter.Package = dep
				adapter.RunMode = "server"
			}
		}
		// Default to node for SvelteKit
		if adapter.Type == "unknown" && allDeps["@sveltejs/kit"] != "" {
			adapter.Type = "node"
			adapter.RunMode = "server"
		}

	case "astro":
		// Check for Astro adapters
		for dep := range allDeps {
			switch {
			case strings.Contains(dep, "@astrojs/node"):
				adapter.Type = "node"
				adapter.Package = dep
				adapter.RunMode = "server"
			case strings.Contains(dep, "@astrojs/vercel"):
				adapter.Type = "vercel"
				adapter.Package = dep
				adapter.RunMode = "server"
			case strings.Contains(dep, "@astrojs/netlify"):
				adapter.Type = "netlify"
				adapter.Package = dep
				adapter.RunMode = "server"
			case strings.Contains(dep, "@astrojs/cloudflare"):
				adapter.Type = "cloudflare"
				adapter.Package = dep
				adapter.RunMode = "server"
			}
		}
		// Default to static for Astro (SSG is default)
		if adapter.Type == "unknown" {
			adapter.Type = "static"
			adapter.RunMode = "static"
		}
	}

	return adapter
}

// GetProductionStartScript finds the best production start script from package.json
func GetProductionStartScript(pkg PackageJSON) string {
	// Priority order for production scripts
	priorities := []string{
		"start:prod",
		"start:production",
		"serve",
		"preview",
		"start",
	}

	for _, scriptName := range priorities {
		if script, exists := pkg.Scripts[scriptName]; exists && script != "" {
			return scriptName
		}
	}

	return "start" // fallback
}

// GetProductionBuildScript finds the build script from package.json
func GetProductionBuildScript(pkg PackageJSON) string {
	// Check for build scripts
	priorities := []string{
		"build:prod",
		"build:production",
		"build",
	}

	for _, scriptName := range priorities {
		if script, exists := pkg.Scripts[scriptName]; exists && script != "" {
			return scriptName
		}
	}

	return "build" // fallback
}

// DetectMonorepoType detects the monorepo tool being used
func DetectMonorepoType(root string) string {
	switch {
	case fileExists(root, "turbo.json"):
		return "turborepo"
	case fileExists(root, "nx.json"):
		return "nx"
	case fileExists(root, "lerna.json"):
		return "lerna"
	case hasWorkspacesInPackageJSON(root):
		return "npm-workspaces"
	case fileExists(root, "pnpm-workspace.yaml"):
		return "pnpm-workspaces"
	default:
		return "none"
	}
}

// Helper functions

func fileExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

func dirExists(root, rel string) bool {
	info, err := os.Stat(filepath.Join(root, rel))
	return err == nil && info.IsDir()
}

func readFile(root, rel string) string {
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return ""
	}
	return string(b)
}

func mergeDeps(deps ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, d := range deps {
		for k, v := range d {
			merged[k] = v
		}
	}
	return merged
}

func hasWorkspacesInPackageJSON(root string) bool {
	content := readFile(root, "package.json")
	if content == "" {
		return false
	}

	var pkg struct {
		Workspaces interface{} `json:"workspaces"`
	}

	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return false
	}

	return pkg.Workspaces != nil
}
