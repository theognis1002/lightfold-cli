package helpers

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// DetectPortFromPackageJSON attempts to detect the port from package.json scripts
func DetectPortFromPackageJSON(fs FSReader) string {
	if !fs.Has("package.json") {
		return ""
	}

	content := fs.Read("package.json")
	if content == "" {
		return ""
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}

	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return ""
	}

	// Check common script names
	scriptNames := []string{"start", "dev", "serve", "prod", "production"}
	for _, scriptName := range scriptNames {
		if script, exists := pkg.Scripts[scriptName]; exists {
			if port := extractPortFromCommand(script); port != "" {
				return port
			}
		}
	}

	return ""
}

// DetectPortFromEnvFile attempts to detect PORT from .env files
func DetectPortFromEnvFile(fs FSReader) string {
	envFiles := []string{".env", ".env.local", ".env.example", ".env.development"}

	for _, envFile := range envFiles {
		if !fs.Has(envFile) {
			continue
		}

		content := fs.Read(envFile)
		lines := strings.Split(content, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				continue
			}

			if strings.HasPrefix(line, "PORT=") {
				portStr := strings.TrimPrefix(line, "PORT=")
				portStr = strings.Trim(portStr, `"'`)
				if isValidPort(portStr) {
					return portStr
				}
			}
		}
	}

	return ""
}

// DetectPortFromViteConfig attempts to detect port from vite.config.js/ts
func DetectPortFromViteConfig(fs FSReader) string {
	configs := []string{"vite.config.js", "vite.config.ts", "vite.config.mjs"}

	for _, config := range configs {
		if !fs.Has(config) {
			continue
		}

		content := fs.Read(config)
		// Look for server.port or server: { port: xxxx }
		portRegex := regexp.MustCompile(`port\s*:\s*(\d+)`)
		if matches := portRegex.FindStringSubmatch(content); len(matches) > 1 {
			if isValidPort(matches[1]) {
				return matches[1]
			}
		}
	}

	return ""
}

// DetectPortFromNextConfig attempts to detect port from next.config.js
func DetectPortFromNextConfig(fs FSReader) string {
	// Next.js doesn't typically define port in config, it's in package.json scripts
	// But check anyway for custom setups
	configs := []string{"next.config.js", "next.config.mjs"}

	for _, config := range configs {
		if !fs.Has(config) {
			continue
		}

		content := fs.Read(config)
		// Look for port references (rare but possible)
		portRegex := regexp.MustCompile(`port\s*:\s*(\d+)`)
		if matches := portRegex.FindStringSubmatch(content); len(matches) > 1 {
			if isValidPort(matches[1]) {
				return matches[1]
			}
		}
	}

	return ""
}

// DetectPortFromDjangoSettings attempts to detect port from Django settings
func DetectPortFromDjangoSettings(fs FSReader) string {
	// Django typically uses runserver command, check manage.py or settings for custom ports
	settingsPaths := []string{"settings.py", "config/settings.py", "core/settings.py"}

	for _, settingsPath := range settingsPaths {
		if !fs.Has(settingsPath) {
			continue
		}

		content := fs.Read(settingsPath)
		// Look for PORT or BIND settings
		portRegex := regexp.MustCompile(`PORT\s*=\s*['"]?(\d+)['"]?`)
		if matches := portRegex.FindStringSubmatch(content); len(matches) > 1 {
			if isValidPort(matches[1]) {
				return matches[1]
			}
		}
	}

	return ""
}

// DetectPortFromGoCode attempts to detect port from Go main files
func DetectPortFromGoCode(fs FSReader) string {
	goFiles := []string{"main.go", "cmd/server/main.go", "cmd/api/main.go"}

	for _, goFile := range goFiles {
		if !fs.Has(goFile) {
			continue
		}

		content := fs.Read(goFile)
		// Look for :PORT patterns or port constants
		portRegex := regexp.MustCompile(`:(\d{4,5})[^0-9]`)
		if matches := portRegex.FindStringSubmatch(content); len(matches) > 1 {
			if isValidPort(matches[1]) {
				return matches[1]
			}
		}

		// Look for port variable assignments
		portRegex2 := regexp.MustCompile(`port\s*:?=\s*['"]?(\d+)['"]?`)
		if matches := portRegex2.FindStringSubmatch(content); len(matches) > 1 {
			if isValidPort(matches[1]) {
				return matches[1]
			}
		}
	}

	return ""
}

// DetectPortFromPhoenixConfig attempts to detect port from Phoenix config
func DetectPortFromPhoenixConfig(fs FSReader) string {
	configs := []string{"config/config.exs", "config/dev.exs", "config/prod.exs"}

	for _, config := range configs {
		if !fs.Has(config) {
			continue
		}

		content := fs.Read(config)
		// Look for http: [port: xxxx]
		portRegex := regexp.MustCompile(`port:\s*(\d+)`)
		if matches := portRegex.FindStringSubmatch(content); len(matches) > 1 {
			if isValidPort(matches[1]) {
				return matches[1]
			}
		}
	}

	return ""
}

// DetectPortFromRubyConfig attempts to detect port from Ruby/Rails config
func DetectPortFromRubyConfig(fs FSReader) string {
	configs := []string{"config/puma.rb", "config.ru"}

	for _, config := range configs {
		if !fs.Has(config) {
			continue
		}

		content := fs.Read(config)
		// Look for port xxxx
		portRegex := regexp.MustCompile(`port\s+(\d+)`)
		if matches := portRegex.FindStringSubmatch(content); len(matches) > 1 {
			if isValidPort(matches[1]) {
				return matches[1]
			}
		}
	}

	return ""
}

// DetectPortFromSpringBoot attempts to detect port from Spring Boot application.properties
func DetectPortFromSpringBoot(fs FSReader) string {
	configs := []string{
		"src/main/resources/application.properties",
		"src/main/resources/application.yml",
		"application.properties",
		"application.yml",
	}

	for _, config := range configs {
		if !fs.Has(config) {
			continue
		}

		content := fs.Read(config)
		// Look for server.port
		portRegex := regexp.MustCompile(`server\.port\s*[=:]\s*(\d+)`)
		if matches := portRegex.FindStringSubmatch(content); len(matches) > 1 {
			if isValidPort(matches[1]) {
				return matches[1]
			}
		}
	}

	return ""
}

// DetectPortFromASPNet attempts to detect port from ASP.NET launchSettings.json
func DetectPortFromASPNet(fs FSReader) string {
	if !fs.Has("Properties/launchSettings.json") {
		return ""
	}

	content := fs.Read("Properties/launchSettings.json")
	if content == "" {
		return ""
	}

	// Look for applicationUrl with port
	portRegex := regexp.MustCompile(`http://[^:]+:(\d+)`)
	if matches := portRegex.FindStringSubmatch(content); len(matches) > 1 {
		if isValidPort(matches[1]) {
			return matches[1]
		}
	}

	return ""
}

// extractPortFromCommand extracts port from command line arguments
func extractPortFromCommand(command string) string {
	// Pattern 1: -p 8080 or --port 8080
	portFlagRegex := regexp.MustCompile(`(?:-p|--port)\s+(\d+)`)
	if matches := portFlagRegex.FindStringSubmatch(command); len(matches) > 1 {
		if isValidPort(matches[1]) {
			return matches[1]
		}
	}

	// Pattern 2: -p=8080 or --port=8080
	portFlagEqualRegex := regexp.MustCompile(`(?:-p|--port)=(\d+)`)
	if matches := portFlagEqualRegex.FindStringSubmatch(command); len(matches) > 1 {
		if isValidPort(matches[1]) {
			return matches[1]
		}
	}

	// Pattern 3: --bind 0.0.0.0:8080 or --listen :8080
	bindRegex := regexp.MustCompile(`(?:--bind|--listen)\s+[^:]*:(\d+)`)
	if matches := bindRegex.FindStringSubmatch(command); len(matches) > 1 {
		if isValidPort(matches[1]) {
			return matches[1]
		}
	}

	// Pattern 4: PORT=8080 in command
	envRegex := regexp.MustCompile(`PORT=(\d+)`)
	if matches := envRegex.FindStringSubmatch(command); len(matches) > 1 {
		if isValidPort(matches[1]) {
			return matches[1]
		}
	}

	return ""
}

// isValidPort checks if a port string is valid (1-65535)
func isValidPort(portStr string) bool {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return port > 0 && port <= 65535
}

// DetectPort is a unified port detection function that tries multiple methods
func DetectPort(fs FSReader, framework string) string {
	// Try package.json first (most common for JS/TS)
	if port := DetectPortFromPackageJSON(fs); port != "" {
		return port
	}

	// Try .env files
	if port := DetectPortFromEnvFile(fs); port != "" {
		return port
	}

	// Framework-specific detection
	switch framework {
	case "Next.js", "Remix", "SvelteKit":
		if port := DetectPortFromNextConfig(fs); port != "" {
			return port
		}
	case "Vue.js", "Vite":
		if port := DetectPortFromViteConfig(fs); port != "" {
			return port
		}
	case "Django", "Flask", "FastAPI":
		if port := DetectPortFromDjangoSettings(fs); port != "" {
			return port
		}
	case "Gin", "Echo", "Fiber", "Go":
		if port := DetectPortFromGoCode(fs); port != "" {
			return port
		}
	case "Phoenix":
		if port := DetectPortFromPhoenixConfig(fs); port != "" {
			return port
		}
	case "Rails", "Ruby":
		if port := DetectPortFromRubyConfig(fs); port != "" {
			return port
		}
	case "Spring Boot":
		if port := DetectPortFromSpringBoot(fs); port != "" {
			return port
		}
	case "ASP.NET":
		if port := DetectPortFromASPNet(fs); port != "" {
			return port
		}
	}

	return ""
}

// GetDefaultPortForFramework returns the conventional default port for a framework
func GetDefaultPortForFramework(framework string) string {
	defaults := map[string]string{
		// JavaScript/TypeScript
		"Next.js":    "3000",
		"Remix":      "3000",
		"Nuxt.js":    "3000",
		"SvelteKit":  "5173", // Vite default
		"Astro":      "4321", // Astro 4.x default
		"Gatsby":     "9000",
		"Vue.js":     "8080", // Vue CLI default
		"Angular":    "4200",
		"Express.js": "3000",
		"Fastify":    "3000",
		"NestJS":     "3000",
		"tRPC":       "3000",
		"Eleventy":   "8080",
		"Docusaurus": "3000",

		// Python
		"Django":  "8000",
		"FastAPI": "8000",
		"Flask":   "5000",

		// Go
		"Gin":   "8080",
		"Echo":  "8080",
		"Fiber": "3000",
		"Go":    "8080",
		"Hugo":  "1313",

		// PHP
		"Laravel": "8000",
		"Symfony": "8000",

		// Ruby
		"Rails":  "3000",
		"Jekyll": "4000",

		// Rust
		"Actix": "8080",
		"Axum":  "3000",

		// Java
		"Spring Boot": "8080",

		// C#
		"ASP.NET": "5000",

		// Elixir
		"Phoenix": "4000",
	}

	if port, exists := defaults[framework]; exists {
		return port
	}

	return "3000" // Ultimate fallback
}
