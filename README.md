# Lightfold CLI

Framework detector and deployment tool for web applications with composable, idempotent commands.

## Features

- **Framework Detection**: Detects 15+ frameworks (Next.js, Astro, Django, FastAPI, Express.js, Laravel, Rails, etc.)
- **Package Manager Detection**: npm, yarn, pnpm, bun, pip, poetry, uv, pipenv
- **Composable Commands**: Run deployment steps independently or orchestrated together
- **Idempotent Operations**: Safe to rerun commands - skips already-completed steps
- **Multi-Provider Support**: DigitalOcean, Hetzner Cloud, S3, BYOS (Bring Your Own Server)
- **State Tracking**: Remembers what's been done, skips unnecessary work
- **Blue/Green Deployments**: Zero-downtime releases with automatic rollback

## Installation

```bash
make build
# or
go build -o lightfold ./cmd/lightfold
```

## Quick Start

Deploy your application in one command:

```bash
lightfold deploy --target myapp-prod
```

This will:
1. Detect your framework automatically
2. Create and provision infrastructure
3. Configure the server
4. Deploy your code

For subsequent deployments, just run the same command - it intelligently skips completed steps.

## Commands

### Core Commands

- **`lightfold deploy --target <name>`** - Full deployment orchestration
- **`lightfold create --target <name> --provider <provider>`** - Create infrastructure
- **`lightfold configure --target <name>`** - Configure server
- **`lightfold push --target <name>`** - Deploy new release
- **`lightfold status --target <name>`** - View deployment status
- **`lightfold ssh --target <name>`** - SSH into deployment target
- **`lightfold deploy --target <name> --rollback`** - Rollback to previous release

## Configuration

### Target-Based Config

Config stored in `~/.lightfold/config.json`:

```json
{
  "targets": {
    "myapp-prod": {
      "project_path": "/path/to/project",
      "framework": "Next.js",
      "provider": "digitalocean",
      "provider_config": {
        "digitalocean": {
          "ip": "192.168.1.100",
          "ssh_key": "~/.ssh/id_rsa",
          "username": "deploy",
          "region": "nyc1",
          "size": "s-1vcpu-1gb",
          "provisioned": true,
          "droplet_id": "123456789"
        }
      }
    }
  }
}
```

### API Tokens

Tokens stored securely in `~/.lightfold/tokens.json` (0600 permissions):

```json
{
  "tokens": {
    "digitalocean": "dop_v1_...",
    "hetzner": "..."
  }
}
```

### State Tracking

State per target in `~/.lightfold/state/<target>.json`:

```json
{
  "created": true,
  "configured": true,
  "last_commit": "abc123...",
  "last_deploy": "2025-10-03T10:30:00Z",
  "last_release": "20251003103000",
  "provisioned_id": "123456789"
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

## Supported Providers

### Available
- **DigitalOcean** - Full provisioning support
- **BYOS (Bring Your Own Server)** - Use any existing server

### Coming Soon
- [ ] Hetzner Cloud
- [ ] Vultr
- [ ] Linode/Akamai
- [ ] AWS EC2
- [ ] Google Cloud Compute
- [ ] Azure Virtual Machines

## Development

```bash
make build && ./lightfold .
make test
```

See [AGENTS.md](AGENTS.md) for architecture details.

## Key Design Principles

1. **Composable** - Each command works standalone
2. **Idempotent** - Safe to rerun without side effects
3. **Stateful** - Tracks progress, skips completed work
4. **Provider-Agnostic** - Unified interface across clouds
5. **Release-Based** - Timestamped releases, easy rollback
