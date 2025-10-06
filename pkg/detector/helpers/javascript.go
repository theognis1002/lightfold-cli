package helpers

import (
	"encoding/json"
	"strings"
)

// FSReader provides filesystem operations for helper functions
type FSReader interface {
	Has(path string) bool
	Read(path string) string
	DirExists(path string) bool
}

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
func ParseNextConfig(fs FSReader) NextConfig {
	config := NextConfig{
		OutputMode:  "default",
		Router:      "",
		BuildOutput: ".next/",
	}

	if dirExists(fs, "app") {
		config.Router = "app"
	} else if dirExists(fs, "pages") {
		config.Router = "pages"
	}

	for _, configFile := range []string{"next.config.js", "next.config.ts", "next.config.mjs"} {
		if content := readFile(fs, configFile); content != "" {
			if strings.Contains(content, "output: 'standalone'") ||
				strings.Contains(content, `output: "standalone"`) ||
				strings.Contains(content, "output:'standalone'") {
				config.OutputMode = "standalone"
				config.BuildOutput = ".next/standalone/"
			}
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

func ParsePackageJSON(fs FSReader) PackageJSON {
	content := readFile(fs, "package.json")
	if content == "" {
		return PackageJSON{}
	}

	var pkg PackageJSON
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		pkg.Scripts = make(map[string]string)
		pkg.Dependencies = make(map[string]string)
		pkg.DevDeps = make(map[string]string)
	}

	return pkg
}

func DetectFrameworkAdapter(pkg PackageJSON, framework string) FrameworkAdapter {
	adapter := FrameworkAdapter{
		Type:    "unknown",
		RunMode: "server",
	}

	allDeps := mergeDeps(pkg.Dependencies, pkg.DevDeps)

	switch framework {
	case "remix":
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
		if adapter.Type == "unknown" && allDeps["@remix-run/react"] != "" {
			adapter.Type = "node"
			adapter.RunMode = "server"
		}

	case "svelte", "sveltekit":
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
		if adapter.Type == "unknown" && allDeps["@sveltejs/kit"] != "" {
			adapter.Type = "node"
			adapter.RunMode = "server"
		}

	case "astro":
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
		if adapter.Type == "unknown" {
			adapter.Type = "static"
			adapter.RunMode = "static"
		}
	}

	return adapter
}

func GetProductionStartScript(pkg PackageJSON) string {
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

	return "start"
}

func GetProductionBuildScript(pkg PackageJSON) string {
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

	return "build"
}

func DetectMonorepoType(fs FSReader) string {
	switch {
	case fileExists(fs, "turbo.json"):
		return "turborepo"
	case fileExists(fs, "nx.json"):
		return "nx"
	case fileExists(fs, "lerna.json"):
		return "lerna"
	case hasWorkspacesInPackageJSON(fs):
		return "npm-workspaces"
	case fileExists(fs, "pnpm-workspace.yaml"):
		return "pnpm-workspaces"
	default:
		return "none"
	}
}

func fileExists(fs FSReader, rel string) bool {
	return fs.Has(rel)
}

func dirExists(fs FSReader, rel string) bool {
	return fs.DirExists(rel)
}

func readFile(fs FSReader, rel string) string {
	return fs.Read(rel)
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

func hasWorkspacesInPackageJSON(fs FSReader) bool {
	content := readFile(fs, "package.json")
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
