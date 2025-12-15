package runtime

// Runtime represents a language runtime installed on a server
type Runtime string

const (
	RuntimeNodeJS  Runtime = "nodejs" // JavaScript/TypeScript (includes npm, node, bun, pnpm, yarn)
	RuntimePython  Runtime = "python" // Python (includes pip, venv, poetry, uv, pipenv)
	RuntimeGo      Runtime = "go"     // Go
	RuntimePHP     Runtime = "php"    // PHP
	RuntimeRuby    Runtime = "ruby"   // Ruby
	RuntimeJava    Runtime = "java"   // Java
	RuntimeDocker  Runtime = "docker" // Docker + Docker Compose V2
	RuntimeUnknown Runtime = "unknown"
)

// RuntimeInfo contains information needed to clean up a runtime
type RuntimeInfo struct {
	Runtime     Runtime  // The runtime identifier
	Packages    []string // APT packages to remove
	Directories []string // Directories to delete (user-level installs)
	Commands    []string // Additional cleanup commands
}

// GetRuntimeFromLanguage maps a detected language to its runtime
func GetRuntimeFromLanguage(language string) Runtime {
	switch language {
	case "JavaScript/TypeScript":
		return RuntimeNodeJS
	case "Python":
		return RuntimePython
	case "Go":
		return RuntimeGo
	case "PHP":
		return RuntimePHP
	case "Ruby":
		return RuntimeRuby
	case "Java":
		return RuntimeJava
	case "Container":
		return RuntimeDocker
	default:
		return RuntimeUnknown
	}
}

// GetRuntimeInfo returns cleanup information for a specific runtime
func GetRuntimeInfo(rt Runtime) RuntimeInfo {
	switch rt {
	case RuntimeNodeJS:
		return RuntimeInfo{
			Runtime: RuntimeNodeJS,
			Packages: []string{
				"nodejs",
				"npm",
			},
			Directories: []string{
				"/usr/local/bin/node",
				"/usr/local/bin/npm",
				"/usr/local/bin/npx",
				"/usr/local/lib/node_modules",
				"/home/deploy/.bun",
				"/home/deploy/.npm",
				"/home/deploy/.local/share/pnpm",
			},
			Commands: []string{
				"rm -f /usr/bin/node /usr/bin/npm /usr/bin/npx",
			},
		}

	case RuntimePython:
		return RuntimeInfo{
			Runtime: RuntimePython,
			Packages: []string{
				"python3-pip",
				"python3-venv",
			},
			Directories: []string{
				"/home/deploy/.local/lib/python*",
				"/home/deploy/.local/bin/poetry",
				"/home/deploy/.local/bin/uv",
				"/home/deploy/.local/bin/pipenv",
			},
			Commands: []string{
				"pip3 uninstall -y poetry pipenv uv 2>/dev/null || true",
			},
		}

	case RuntimeGo:
		return RuntimeInfo{
			Runtime: RuntimeGo,
			Packages: []string{
				"golang-go",
			},
			Directories: []string{
				"/usr/lib/go-*",
				"/home/deploy/go",
			},
			Commands: []string{},
		}

	case RuntimePHP:
		return RuntimeInfo{
			Runtime: RuntimePHP,
			Packages: []string{
				"php",
				"php-fpm",
				"php-mysql",
				"php-xml",
				"php-mbstring",
			},
			Directories: []string{},
			Commands:    []string{},
		}

	case RuntimeRuby:
		return RuntimeInfo{
			Runtime: RuntimeRuby,
			Packages: []string{
				"ruby-full",
			},
			Directories: []string{
				"/home/deploy/.gem",
			},
			Commands: []string{},
		}

	case RuntimeJava:
		return RuntimeInfo{
			Runtime: RuntimeJava,
			Packages: []string{
				"default-jre",
				"default-jdk",
			},
			Directories: []string{},
			Commands:    []string{},
		}

	case RuntimeDocker:
		return RuntimeInfo{
			Runtime: RuntimeDocker,
			Packages: []string{
				"docker-ce",
				"docker-ce-cli",
				"containerd.io",
				"docker-buildx-plugin",
				"docker-compose-plugin",
			},
			Directories: []string{},
			Commands:    []string{},
		}

	default:
		return RuntimeInfo{
			Runtime:     RuntimeUnknown,
			Packages:    []string{},
			Directories: []string{},
			Commands:    []string{},
		}
	}
}
