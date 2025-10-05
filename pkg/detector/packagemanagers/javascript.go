package packagemanagers

import (
	"os"
	"path/filepath"
)

// DetectJS detects the JavaScript package manager used in a project
func DetectJS(root string) string {
	fileExists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, rel))
		return err == nil
	}

	switch {
	case fileExists("bun.lockb") || fileExists("bun.lock"):
		return "bun"
	case fileExists(".yarnrc.yml"):
		return "yarn-berry"
	case fileExists("pnpm-lock.yaml"):
		return "pnpm"
	case fileExists("yarn.lock"):
		return "yarn"
	default:
		return "npm"
	}
}

// GetJSInstallCommand returns the install command for the given package manager
func GetJSInstallCommand(pm string) string {
	switch pm {
	case "bun":
		return "bun install"
	case "pnpm":
		return "pnpm install"
	case "yarn", "yarn-berry":
		return "yarn install"
	default:
		return "npm install"
	}
}

// GetJSBuildCommand returns the build command for the given package manager
func GetJSBuildCommand(pm string) string {
	switch pm {
	case "bun":
		return "bun run build"
	case "pnpm":
		return "pnpm run build"
	case "yarn", "yarn-berry":
		return "yarn build"
	default:
		return "npm run build"
	}
}

// GetJSStartCommand returns the start command for the given package manager
func GetJSStartCommand(pm string) string {
	switch pm {
	case "bun":
		return "bun run start"
	case "pnpm":
		return "pnpm start"
	case "yarn", "yarn-berry":
		return "yarn start"
	default:
		return "npm start"
	}
}

// GetPreviewCommand returns the preview command for the given package manager
func GetPreviewCommand(pm string) string {
	switch pm {
	case "npm":
		return "npm run preview"
	default:
		return pm + " run preview"
	}
}

// GetRunCommand returns a custom run command for the given package manager and script
func GetRunCommand(pm string, script string) string {
	switch pm {
	case "npm":
		return "npm run " + script
	default:
		return pm + " run " + script
	}
}

// DetectDenoRuntime checks if the project uses Deno runtime
func DetectDenoRuntime(root string) bool {
	fileExists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, rel))
		return err == nil
	}
	return fileExists("deno.json") || fileExists("deno.jsonc")
}
