# Lightfold CLI

Framework detector and deployment tool for web applications.

## Features

- Detects 15+ frameworks: Next.js, Astro, Django, FastAPI, Express.js, Laravel, Rails, etc.
- Package manager detection: npm, yarn, pnpm, bun, pip, poetry, uv, pipenv
- Generates build and run commands
- Two-step deployment configuration: BYOS vs Auto-provision
- DigitalOcean auto-provisioning with API integration
- S3 static site deployment
- Interactive TUI and JSON output for automation

## Installation

```bash
make build
# or
go build -o lightfold ./cmd/lightfold
```

## Usage

```bash
# Detect framework and configure deployment
./lightfold .

# Deploy application
./lightfold deploy

# JSON output for automation
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

## Detection

Uses scoring system based on:
1. Framework config files (highest priority)
2. Package manager lockfiles and dependencies
3. Build scripts and directory structure

### Package Manager Priority
- **JavaScript/TypeScript**: bun → pnpm → yarn → npm
- **Python**: uv → poetry → pipenv → pip

## Supported Frameworks

**Frontend**: Next.js, Astro, Gatsby, Svelte/SvelteKit, Vue.js, Angular
**Backend**: Django, Flask, FastAPI, Express.js, NestJS, Laravel, Rails, Spring Boot, ASP.NET Core, Phoenix
**Languages**: JavaScript/TypeScript, Python, PHP, Ruby, Go, Java, C#, Elixir

## Commands

- `lightfold [PROJECT_PATH]` - Detect framework and configure deployment
- `lightfold detect [PROJECT_PATH]` - Run detection flow
- `lightfold deploy [PROJECT_PATH]` - Deploy using saved configuration
- `lightfold --version` - Show version

## Flags

- `--json` - Output JSON (for CI/automation)
- `--no-interactive` - Skip interactive prompts
- `-h, --help` - Help

## Examples

```bash
# Interactive detection and configuration
./lightfold ./my-app

# JSON output for CI
./lightfold ./my-app --json

# Deploy configured project
./lightfold deploy

# Deploy specific project
./lightfold deploy /path/to/project
```

## Configuration

Config stored in `~/.lightfold/config.json`:

```json
{
  "projects": {
    "/path/to/project": {
      "framework": "Next.js",
      "target": "digitalocean",
      "digitalocean": {
        "ip": "192.168.1.100",
        "ssh_key": "~/.ssh/id_rsa",
        "username": "deploy",
        "region": "nyc1",
        "size": "s-1vcpu-1gb",
        "provisioned": true
      }
    }
  }
}
```

API tokens stored securely in `~/.lightfold/tokens.json`.

## Development

```bash
make build && ./lightfold .
make test
```

See [AGENTS.md](AGENTS.md) for architecture details.