# Lightfold CLI

Framework detector and deployment tool for web applications with composable, idempotent commands.

## Features

- **Framework Detection**: Detects 16+ frameworks (Next.js, Astro, Django, FastAPI, Express.js, tRPC, NestJS, Laravel, Rails, etc.)
- **Package Manager Detection**: npm, yarn, pnpm, bun, pip, poetry, uv, pipenv
- **Pluggable Builders**: Native (traditional), Nixpacks (auto-detected), or Dockerfile (reserved)
- **Composable Commands**: Run deployment steps independently or orchestrated together
- **Idempotent Operations**: Safe to rerun commands - skips already-completed steps
- **Multi-Provider Support**: DigitalOcean, Vultr, Hetzner Cloud, S3, BYOS (Bring Your Own Server)
- **State Tracking**: Remembers what's been done, skips unnecessary work
- **Blue/Green Deployments**: Zero-downtime releases with automatic rollback

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap theognis1002/lightfold
brew install lightfold
```

### Manual Installation

**Download Pre-built Binary:**
Visit the [releases page](https://github.com/theognis1002/lightfold-cli/releases) and download the binary for your platform.

**Build from Source:**
```bash
git clone https://github.com/theognis1002/lightfold-cli.git
cd lightfold-cli
make build
sudo make install
```

## Quick Start

1. **Install Lightfold:**
   ```bash
   brew install theognis1002/lightfold/lightfold
   ```

2. **Navigate to your project:**
   ```bash
   cd ~/Projects/myapp
   ```

3. **Deploy:**
   ```bash
   lightfold deploy
   ```

That's it! On first run, you'll be prompted to:
- Select a cloud provider (DigitalOcean, Vultr, Hetzner Cloud, BYOS, etc.)
- Provide credentials (API tokens, SSH keys)
- Choose region and server size

Then Lightfold automatically:
1. Detects your framework
2. Selects optimal builder (Dockerfile → Nixpacks → Native)
3. Provisions infrastructure
4. Configures the server
5. Deploys your code

For subsequent deployments, just run the same command - it intelligently skips completed steps and only redeploys code changes.

## Commands

### Primary Command

**`lightfold deploy`** - Full deployment (recommended)

### Advanced Commands

For granular control over deployment steps:

- **`lightfold create`** - Create infrastructure only
- **`lightfold configure`** - Configure server only
- **`lightfold push`** - Deploy code changes only

### Management Commands

- **`lightfold status`** - View deployment status
- **`lightfold logs`** - View application logs
- **`lightfold rollback`** - Rollback to previous release
- **`lightfold ssh`** - SSH into deployment target
- **`lightfold destroy`** - Destroy VM and remove local config

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
      "builder": "nixpacks",
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
    "vultr": "...",
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
  "provisioned_id": "123456789",
  "builder": "nixpacks"
}
```


## Detection

### Framework Detection
Uses scoring system based on:
1. Framework config files (highest priority)
2. Package manager lockfiles and dependencies
3. Build scripts and directory structure

### Package Manager Priority
- **JavaScript/TypeScript**: bun → pnpm → yarn → npm
- **Python**: uv → poetry → pipenv → pip

### Builder Selection
Auto-selection priority:
1. **Dockerfile exists** → use `dockerfile` builder
2. **Node/Python + nixpacks available** → use `nixpacks` builder
3. **Fallback** → use `native` builder

Override with `--builder` flag.

## Supported Frameworks

**Frontend**: Next.js, Astro, Gatsby, Svelte/SvelteKit, Vue.js, Angular
**Backend**: Django, Flask, FastAPI, Express.js, NestJS, tRPC, Laravel, Rails, Spring Boot, ASP.NET Core, Phoenix
**Languages**: JavaScript/TypeScript, Python, PHP, Ruby, Go, Java, C#, Elixir

## Supported Providers

### Available
- **DigitalOcean** - Full provisioning support
- **Vultr** - Full provisioning support
- **Hetzner Cloud** - Full provisioning support
- **BYOS (Bring Your Own Server)** - Use any existing server

### Coming Soon
- [ ] Linode/Akamai
- [ ] AWS EC2
- [ ] Google Cloud Compute
- [ ] Azure Virtual Machines

## Development

### Building Locally

```bash
make build && ./lightfold .
make test
```

### Creating a Release

1. **Tag a new version:**
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

2. **GitHub Actions will automatically:**
   - Build binaries for all platforms
   - Create a GitHub Release
   - Update the Homebrew tap formula
   - Generate checksums and changelog

3. **Users can then install via:**
   ```bash
   brew update
   brew install theognis1002/lightfold/lightfold
   ```

See [AGENTS.md](AGENTS.md) for architecture details and [docs/RELEASING.md](docs/RELEASING.md) for release instructions.

## Key Design Principles

1. **Composable** - Each command works standalone
2. **Idempotent** - Safe to rerun without side effects
3. **Stateful** - Tracks progress, skips completed work
4. **Provider-Agnostic** - Unified interface across clouds
5. **Release-Based** - Timestamped releases, easy rollback


## TODO
[ ] `sync` command for syncing state with current VM provider
[ ] separate `configure` command from `push` - right now we build/deploy app in `configure`
[ ] review previous releases cleanup + consolidate where possible
[ ] add 100% unit test coverage in `detector/` + expand detector scenarios