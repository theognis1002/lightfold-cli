# Lightfold CLI

A fast, intelligent project framework detector that automatically identifies web frameworks and generates optimal build and deployment plans. Features an interactive TUI for seamless deployment configuration to DigitalOcean and S3.

## Features

- **Smart Framework Detection**: Automatically detects 15+ popular web frameworks including:
  - **Frontend**: Next.js, Astro, Gatsby, Svelte/SvelteKit, Vue.js, Angular
  - **Backend**: Django, Flask, FastAPI, Express.js, NestJS, Laravel, Rails, Spring Boot, ASP.NET Core, Phoenix
  - **Generic**: Go services, Docker-based projects

- **Advanced Package Manager Support**:
  - **JavaScript/TypeScript**: npm, yarn, pnpm, bun
  - **Python**: pip, poetry, uv, pipenv
  - **Others**: composer (PHP), bundle (Ruby), mix (Elixir), dotnet, maven/gradle

- **Comprehensive Build Plans**: Generates framework-specific build commands with proper package manager detection
- **Production-Ready Run Commands**: Provides optimized runtime configurations
- **Health Check Configuration**: Sets up appropriate health check endpoints
- **Environment Variable Schemas**: Documents expected environment variables per framework

### 🚀 Deployment Features

- **Interactive TUI**: Beautiful terminal interface for deployment configuration
- **Multiple Deployment Targets**:
  - **DigitalOcean**: SSH-based deployment to droplets
  - **S3**: Static site deployment to AWS S3
- **Configuration Management**: Secure storage in `~/.lightfold/config.json`
- **Animated Progress**: Real-time deployment progress with animated bars
- **Success Animations**: Celebratory ASCII rocket on successful deployment
- **CI/Automation Support**: JSON output mode and `--no-interactive` flag

## Installation

```bash
make build
# or
go build -o lightfold ./cmd/lightfold
```

## Quick Start

### 1. Detect Framework & Configure Deployment
```bash
./lightfold .
```
This will:
- Detect your project's framework
- Show detection results with confidence score
- Launch interactive TUI for deployment configuration (if in terminal)
- Save configuration to `~/.lightfold/config.json`

### 2. Deploy Your Application
```bash
./lightfold deploy
```
This will:
- Auto-detect the configured project if only one exists
- Show animated progress bar (or fall back to non-interactive mode)
- Deploy using your saved configuration
- Celebrate with ASCII rocket animation! 🚀

```bash
# Deploy specific project
./lightfold deploy /path/to/project
```

### 3. JSON Output (for CI/Automation)
```bash
./lightfold . --json
./lightfold deploy --no-interactive
```

The tool outputs a JSON structure containing:

```json
{
  "framework": "Next.js",
  "language": "JavaScript/TypeScript",
  "confidence": 0.95,
  "signals": [
    "next.config",
    "package.json has next",
    "package.json scripts for next"
  ],
  "build_plan": [
    "pnpm install",
    "next build"
  ],
  "run_plan": [
    "next start -p 3000"
  ],
  "healthcheck": {
    "path": "/",
    "expect": 200,
    "timeout_seconds": 30
  },
  "env_schema": [
    "NEXT_PUBLIC_*, any server-only envs"
  ]
}
```

## Detection Logic

Lightfold uses a sophisticated scoring system that evaluates:

1. **Framework-specific configuration files** (highest weight)
2. **Package manager lockfiles and dependencies**
3. **Build scripts and CLI tool presence**
4. **Directory structure patterns**
5. **File extensions and content analysis**

### Package Manager Detection Priority

**JavaScript/TypeScript:**
1. `bun.lockb` → bun
2. `pnpm-lock.yaml` → pnpm
3. `yarn.lock` → yarn
4. Default → npm

**Python:**
1. `uv.lock` → uv
2. `poetry.lock` → poetry
3. `Pipfile.lock` → pipenv
4. Default → pip

## Supported Frameworks

### Frontend Frameworks
- **Next.js**: Detects config files, dependencies, and app/pages structure
- **Astro**: Identifies config files and src/public structure
- **Gatsby**: Looks for gatsby-config and src/pages
- **Svelte/SvelteKit**: Detects svelte.config and src/routes
- **Vue.js**: Identifies Vue CLI, Nuxt, or Vite setups
- **Angular**: Detects angular.json and @angular/core

### Backend Frameworks
- **Django**: Detects manage.py and Django dependencies
- **Flask**: Identifies Flask imports and app files
- **FastAPI**: Detects FastAPI imports and uvicorn setup
- **Express.js**: Identifies Express dependencies and server files
- **NestJS**: Detects nest-cli.json and NestJS structure
- **Laravel**: Identifies artisan and composer.lock
- **Ruby on Rails**: Detects bin/rails and Gemfile.lock
- **Spring Boot**: Identifies Spring Boot in pom.xml/build.gradle
- **ASP.NET Core**: Detects .csproj and Program.cs
- **Phoenix**: Identifies mix.exs with Phoenix dependencies

### Language Support
- JavaScript/TypeScript
- Python
- PHP
- Ruby
- Go
- Java
- C#
- Elixir

## CLI Options

### Commands
- `lightfold [PROJECT_PATH]` - Detect framework and optionally configure deployment
- `lightfold deploy [PROJECT_PATH]` - Deploy using saved configuration
- `lightfold --help` - Show help information
- `lightfold --version` - Show version

### Flags
- `--json` - Output JSON instead of interactive mode (for CI/automation)
- `--no-interactive` - Skip interactive prompts
- `-h, --help` - Help for any command
- `-v, --version` - Version information

## Examples

### Interactive Mode (Default)
```bash
./lightfold ./my-nextjs-app
# Shows framework detection results
# Prompts for deployment configuration
# Launches TUI for DigitalOcean/S3 setup
```

### CI/Automation Mode
```bash
./lightfold ./my-fastapi-app --json
# {"framework": "FastAPI", "language": "Python", ...}
```

### Deployment Examples
```bash
# Configure project for deployment
./lightfold /path/to/my-project

# Deploy from anywhere (auto-detects if only one project configured)
./lightfold deploy

# Deploy specific project by path
./lightfold deploy /path/to/my-project

# List configured projects if multiple exist
./lightfold deploy /nonexistent/path  # Shows available projects
```

## Configuration Storage

Lightfold stores deployment configurations in `~/.lightfold/config.json`:

```json
{
  "projects": {
    "/path/to/project": {
      "framework": "Next.js",
      "target": "digitalocean",
      "digitalocean": {
        "ip": "192.168.1.100",
        "ssh_key": "~/.ssh/id_rsa",
        "username": "root"
      }
    }
  }
}
```

## Testing

```bash
# Run all tests
make test

# Run with coverage
make test-cover

# Run with verbose output
make test-verbose

# Build and test
make build && ./lightfold .
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. For new frameworks: Add detection logic in `pkg/detector/detector.go`
4. For new deployment targets: Add TUI components in `pkg/tui/`
5. Add corresponding plan functions and tests
6. Test with sample projects using `make test`
7. Submit a pull request

See [AGENTS.md](AGENTS.md) for detailed development guidelines.

## License

MIT License - see LICENSE file for details.