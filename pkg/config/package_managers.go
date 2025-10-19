package config

// PackageManagerPaths defines standard system paths for package managers and runtimes.
// These are the default installation locations used across the deployment system.
type PackageManagerPaths struct {
	Bun     string
	Pnpm    string
	Npm     string
	Yarn    string
	Node    string
	Python3 string
	Pip3    string
}

// DefaultPackageManagerPaths returns the standard system paths for package managers.
// These paths assume standard installation locations on Ubuntu/Debian systems.
func DefaultPackageManagerPaths() PackageManagerPaths {
	return PackageManagerPaths{
		Bun:     "/home/deploy/.bun/bin/bun",
		Pnpm:    "/home/deploy/.local/share/pnpm/pnpm",
		Npm:     "/usr/bin/npm",
		Yarn:    "/usr/bin/yarn",
		Node:    "/usr/bin/node",
		Python3: "/usr/bin/python3",
		Pip3:    "/usr/bin/pip3",
	}
}

// GetPackageManagerPath returns the path for a specific package manager.
// Falls back to just the binary name if not found in defaults.
func GetPackageManagerPath(name string) string {
	paths := DefaultPackageManagerPaths()

	switch name {
	case "bun":
		return paths.Bun
	case "pnpm":
		return paths.Pnpm
	case "npm":
		return paths.Npm
	case "yarn":
		return paths.Yarn
	case "node":
		return paths.Node
	case "python3":
		return paths.Python3
	case "pip3":
		return paths.Pip3
	default:
		// Fallback to binary name for unknown package managers
		return name
	}
}
