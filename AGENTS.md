# AGENTS.md - Context for Coding Agents

This document provides essential context and guidelines for AI coding agents working on the Lightfold CLI project.

## Project Overview

Lightfold CLI is a framework detector and deployment tool with composable, idempotent commands. It automatically identifies 15+ web frameworks, then provides standalone commands for each deployment step (create, configure, push) that can be run independently or orchestrated together. Features include multi-provider support (DigitalOcean, Vultr, Hetzner, BYOS), state tracking for smart skipping, blue/green deployments with rollback, and secure API token storage. The core design principles are composability, idempotency, and provider-agnostic deployment.

## Architecture

### Core Components

1. **Detection Engine** (`pkg/detector/`):
   - Scoring-based framework detection system
   - File pattern matching and content analysis
   - Configurable scoring thresholds per framework
   - Returns Detection struct for programmatic use

2. **Package Manager Detection**:
   - JavaScript/TypeScript: bun, pnpm, yarn, npm
   - Python: uv, poetry, pipenv, pip
   - Automatic command generation based on detected package manager

3. **Framework Plans**:
   - Build command sequences
   - Runtime configurations
   - Health check endpoints
   - Environment variable schemas

3.5. **Builder System** (`pkg/builders/`):
   - Pluggable build strategies with registry pattern
   - **Native Builder**: Traditional approach using framework detection + nginx
   - **Nixpacks Builder**: Railway's Nixpacks for auto-detected builds
   - **Dockerfile Builder**: Reserved for future Docker-based builds
   - Auto-selection priority: Dockerfile exists → `dockerfile`, Node/Python + nixpacks available → `nixpacks`, else → `native`
   - Builder choice persisted in config and state for retry resilience
   - Interface: `Name()`, `IsAvailable()`, `Build()`, `NeedsNginx()`

4. **CLI Interface** (`cmd/`):
   - **Unified Command Pattern**: All commands support three invocation methods:
     1. No arguments → current directory (`lightfold deploy`)
     2. Path argument → specific project (`lightfold deploy ~/path/to/project`)
     3. `--target` flag → named target (`lightfold deploy --target myapp`)
   - **Composable Commands**: Independent commands for each deployment step
     - `detect` - Framework detection only (JSON output, standalone use)
     - `create` - Infrastructure creation (BYOS or auto-provision)
     - `configure` - Server configuration with idempotency checks
     - `push` - Release deployment with health checks
     - `deploy` - Orchestrator that chains all steps with smart skipping (supports `--builder`, `--server-ip` flags)
     - `status` - View deployment state and server status (supports `--json`, shows multi-app context)
     - `server` - Manage servers and multi-app deployments (`list`, `show <ip>`)
     - `logs` - Fetch and display application logs (supports `--tail` and `--lines`)
     - `rollback` - Instant rollback to previous release (with confirmation)
     - `sync` - Sync local state/config with actual server state (drift recovery)
     - `config` - Manage targets and API tokens
     - `domain` - Manage custom domains and SSL (add, remove, show)
     - `keygen` - Generate SSH keypairs
     - `ssh` - Interactive SSH sessions to deployment targets
     - `destroy` - Destroy VM and remove local configuration (unregisters from server state)
   - Target resolution via `resolveTarget()` helper in `cmd/common.go`
   - Builder resolution via `resolveBuilder()` helper with 3-layer priority (flag > config > auto-detect)
   - Clean JSON output with `--json` flag (status command)
   - Target-based architecture (named targets, not paths)
   - Dry-run support (`--dry-run`) for preview
   - Force flags to override idempotency
   - Builder selection with `--builder` flag (native, nixpacks, dockerfile)

5. **State Tracking** (`pkg/state/`):
   - **Target State**: Per-target deployment state `~/.lightfold/state/<target>.json`
   - **Server State**: Multi-app tracking `~/.lightfold/servers/<server-ip>.json`
   - Remote state markers on servers: `/etc/lightfold/{created,configured}`
   - Git commit tracking to skip unchanged deployments
   - Tracks: last commit, last deploy time, last release ID, provision ID, builder, SSL status
   - Port allocation system (3000-9000 range) with conflict detection
   - Port selection UI shows used ports: "Port range: 3000-9000 | Used: 3000 (app1), 5000 (app2)"
   - Port output after SSH validation: "✓ Allocated to port 3001" (cmd/common.go:210-212)
   - Enables idempotent operations and intelligent step skipping
   - **Runtime Tracking**: `InstalledRuntimes []Runtime` in ServerState tracks installed language runtimes

5.5. **Runtime Cleanup System** (`pkg/runtime/`):
   - **Automatic runtime cleanup** when destroying apps on multi-app servers
   - **Smart dependency analysis**: Only removes runtimes not needed by remaining apps
   - **Registry pattern**: Consistent with SSL/proxy/builder architecture
   - **Runtime Types**: nodejs, python, go, php, ruby, java
   - **Key Components**:
     - `GetRuntimeFromLanguage()` - Maps language to runtime (JavaScript/TypeScript → nodejs)
     - `GetRuntimeInfo()` - Returns cleanup info (packages, directories, commands)
     - `GetLanguageFromFramework()` - Maps framework to language (Next.js → JavaScript/TypeScript)
     - `GetRequiredRuntimesForApps()` - Analyzes remaining apps, returns required runtimes
     - `CleanupUnusedRuntimes()` - Orchestrates cleanup (non-blocking, graceful failure)
   - **Integration Points**:
     - `pkg/deploy/orchestrator.go`: Registers runtimes after `InstallBasePackages()`
     - `cmd/destroy.go`: Calls cleanup after unregistering app from server state
   - **Cleanup Operations**:
     - Remove APT packages (e.g., `apt-get remove -y nodejs npm`)
     - Delete user directories (e.g., `/home/deploy/.bun`, `/home/deploy/.npm`)
     - Run additional commands (e.g., cleanup symlinks)
     - Update server state to reflect removed runtimes
   - **Safety**: Never removes nginx or system infrastructure, only language runtimes
   - **Non-blocking**: Cleanup failures log warnings but don't fail destroy operation
   - **Example Flow**:
     - Server has: JS app + Python app + Go app
     - Destroy Python app
     - System analyzes: JS still needed, Go still needed, Python NOT needed
     - Removes: python3-pip, python3-venv, /home/deploy/.local/lib/python*, poetry, uv, pipenv
     - Keeps: nodejs, npm, golang-go
   - **State Updates**: `state.RegisterRuntime()` during deploy, automatic cleanup during destroy

5.6. **SSL Management** (`pkg/ssl/`):
   - Pluggable SSL manager system with registry pattern
   - **Certbot Manager**: Let's Encrypt integration via certbot
   - Interface: `IsAvailable()`, `IssueCertificate(domain, email)`, `RenewCertificate(domain)`, `EnableAutoRenewal()`
   - Auto-renewal setup via systemd timer
   - Certificate paths: `/etc/letsencrypt/live/{domain}/`

5.7. **Reverse Proxy Management** (`pkg/proxy/`):
   - Pluggable proxy manager system with registry pattern
   - **Nginx Manager**: HTTP and HTTPS configuration with auto-redirect
   - Interface: `Configure(ProxyConfig)`, `Reload()`, `Remove(appName)`, `IsAvailable()`
   - Template-based config generation (HTTP-only and HTTPS with SSL)
   - Support for static file serving and security headers

6. **Configuration Management** (`pkg/config/`):
   - **Target-Based Config**: Named deployment targets (not path-based)
   - Structure: `~/.lightfold/config.json` with `targets` map
   - Each target stores: project path, framework, provider, builder, provider config, domain config
   - **Domain Config**: Optional domain, SSL settings, proxy type, SSL manager
   - Secure API token storage in `~/.lightfold/tokens.json` (0600 permissions)
   - Provider-agnostic config interface with `ProviderConfig` methods
   - Support for BYOS and provisioned configurations per target

### Code Organization

```
lightfold/
├── cmd/
│   ├── lightfold/
│   │   └── main.go       # Entry point
│   ├── root.go           # Root command (framework detection + next steps)
│   ├── detect.go         # Standalone detection command
│   ├── create.go         # Infrastructure creation (BYOS/provision)
│   ├── configure.go      # Server configuration (idempotent)
│   ├── push.go           # Release deployment
│   ├── deploy.go         # Orchestrator (chains all steps)
│   ├── autodeploy.go     # Auto-deployment workflow
│   ├── status.go         # Deployment status viewer (with health checks)
│   ├── logs.go           # Application log viewer
│   ├── rollback.go       # Release rollback
│   ├── sync.go           # State synchronization
│   ├── config.go         # Config/token management
│   ├── keygen.go         # SSH key generation
│   ├── ssh.go            # Interactive SSH sessions
│   ├── destroy.go        # VM destruction and cleanup
│   ├── server.go         # Multi-app server management
│   ├── domain.go         # Custom domain and SSL management
│   ├── common.go         # Shared command helpers and wrapper functions
│   ├── provider_bootstrap.go  # Provider registry and configuration
│   ├── provider_state.go      # Provider state handlers for IP recovery
│   ├── utils/            # Refactored shared utilities
│   │   ├── target_resolution.go   # Target resolution logic
│   │   ├── server_setup.go        # Server creation and configuration helpers
│   │   ├── provider_recovery.go   # Generic IP recovery from providers
│   │   └── port_allocation.go     # Multi-app port allocation
│   ├── templates/        # Command templates
│   └── ui/               # TUI components
│       ├── detection/    # Detection results display
│       ├── deployment/   # Deployment UI components
│       ├── sequential/   # Token collection flows
│       ├── spinner/      # Loading animations
│       ├── progress.go   # Deployment progress bars
│       └── animation.go  # Shared animations
├── pkg/
│   ├── detector/         # Framework detection engine
│   │   ├── detector.go   # Core detection orchestrator
│   │   ├── fsreader.go   # Filesystem reader abstraction
│   │   ├── helpers.go    # Shared detection helpers
│   │   ├── types.go      # Core detection types
│   │   ├── exports.go    # Test helpers
│   │   ├── detectors/    # Framework-specific detectors
│   │   │   ├── types.go        # Detector types and interfaces
│   │   │   ├── scoring.go      # Scoring system
│   │   │   ├── builder.go      # Detector registry builder
│   │   │   ├── javascript.go   # JS/TS frameworks (Next.js, Astro, SvelteKit, etc.)
│   │   │   ├── python.go       # Python frameworks (Django, Flask, FastAPI)
│   │   │   ├── go.go           # Go frameworks
│   │   │   ├── php.go          # PHP frameworks
│   │   │   ├── ruby.go         # Ruby frameworks
│   │   │   ├── rust.go         # Rust frameworks
│   │   │   ├── java.go         # Java frameworks
│   │   │   ├── csharp.go       # C# frameworks
│   │   │   ├── elixir.go       # Elixir frameworks
│   │   │   └── docker.go       # Dockerfile detection
│   │   ├── packagemanagers/  # Package manager detection
│   │   │   ├── javascript.go # JS package managers (npm, yarn, pnpm, bun)
│   │   │   └── python.go     # Python package managers (pip, poetry, uv, pipenv)
│   │   ├── helpers/      # Framework-specific helper utilities
│   │   │   └── javascript.go # JS framework detection helpers
│   │   └── plans/        # Build/run plan generators
│   │       ├── types.go        # Plan function type
│   │       ├── common.go       # Shared plan helpers
│   │       ├── helpers.go      # Plan utility functions
│   │       ├── javascript.go   # JS framework plans
│   │       ├── python.go       # Python framework plans
│   │       ├── go.go           # Go framework plans
│   │       ├── php.go          # PHP framework plans
│   │       ├── ruby.go         # Ruby framework plans
│   │       ├── rust.go         # Rust framework plans
│   │       ├── java.go         # Java framework plans
│   │       ├── csharp.go       # C# framework plans
│   │       ├── elixir.go       # Elixir framework plans
│   │       └── docker.go       # Docker-based plans
│   ├── builders/         # Build system registry
│   │   ├── builder.go    # Builder interface
│   │   ├── registry.go   # Builder factory + auto-selection
│   │   ├── native/       # Native builder implementation
│   │   ├── nixpacks/     # Nixpacks builder implementation
│   │   └── dockerfile/   # Dockerfile builder (stub)
│   ├── config/           # Configuration management
│   │   ├── config.go     # Target-based config + tokens
│   │   └── deployment.go # Deployment options processing
│   ├── state/            # State tracking
│   │   ├── state.go      # Local/remote state management
│   │   ├── server.go     # Multi-app server state
│   │   └── ports.go      # Port allocation and conflict detection
│   ├── runtime/          # Runtime management system
│   │   ├── types.go      # Runtime types and info
│   │   ├── cleaner.go    # Cleanup orchestration
│   │   └── installers/   # Runtime installer registry
│   │       ├── registry.go  # Installer interface and registry
│   │       ├── helpers.go   # Shared installer utilities
│   │       ├── node.go      # Node.js runtime installer
│   │       ├── python.go    # Python runtime installer
│   │       ├── go.go        # Go runtime installer
│   │       ├── php.go       # PHP runtime installer
│   │       └── ruby.go      # Ruby runtime installer
│   ├── deploy/           # Deployment logic
│   │   ├── orchestrator.go # Multi-provider orchestration
│   │   ├── executor.go   # Blue/green deployment executor
│   │   └── templates/    # Deployment templates
│   ├── ssh/              # SSH operations
│   │   ├── executor.go   # SSH command execution
│   │   └── keygen.go     # SSH key generation
│   ├── ssl/              # SSL certificate management
│   │   ├── manager.go    # SSL manager interface + registry
│   │   ├── certbot/      # Certbot (Let's Encrypt) manager
│   │   └── caddy/        # Caddy manager (future)
│   ├── proxy/            # Reverse proxy management
│   │   ├── proxy.go      # Proxy manager interface + registry
│   │   ├── nginx/        # Nginx manager
│   │   └── caddy/        # Caddy manager (future)
│   ├── util/             # Shared utilities
│   │   ├── env.go        # .env file parsing
│   │   └── git.go        # Git operations
│   ├── flyctl/           # Fly.io CLI wrapper
│   │   └── flyctl.go     # flyctl command execution
│   └── providers/        # Cloud provider registry
│       ├── provider.go   # Provider interface
│       ├── registry.go   # Provider factory
│       ├── digitalocean/ # DigitalOcean implementation
│       ├── vultr/        # Vultr implementation
│       ├── hetzner/      # Hetzner implementation
│       ├── flyio/        # Fly.io implementation
│       └── cloudinit/    # Cloud-init template generation
├── test/
│   ├── detector/         # Detection test suite
│   ├── deploy/           # Deployment tests
│   ├── ssh/              # SSH tests
│   ├── providers/        # Provider tests
│   ├── cloudinit/        # Cloud-init tests
│   ├── runtime/          # Runtime cleanup tests
│   └── fixtures/         # Test fixtures for frameworks
├── go.mod                # Go dependencies
├── Makefile              # Build targets
├── README.md             # User documentation
└── AGENTS.md             # This file
```

## Framework Detection Logic

### Detection Scoring System

Each framework uses a weighted scoring system:

- **High Priority** (3+ points): Framework-specific config files
- **Medium Priority** (2-2.5 points): Package dependencies, build tools
- **Low Priority** (0.5-1 points): Directory structure, file patterns

### Adding New Frameworks

When adding framework detection:

1. **Research Phase**: Identify unique indicators
   - Config files (highest priority)
   - Package.json/requirements.txt dependencies
   - Directory structures
   - File extensions and naming patterns

2. **Implementation**:
   - Add detection block in `main.go` candidates section
   - Create corresponding plan function
   - Update language mapping if needed

3. **Testing**: Create sample projects with various package managers

### Package Manager Priority

**JavaScript/TypeScript Detection Order:**
1. `bun.lockb` → bun
2. `pnpm-lock.yaml` → pnpm
3. `yarn.lock` → yarn
4. Default → npm

**Python Detection Order:**
1. `uv.lock` → uv
2. `poetry.lock` → poetry
3. `Pipfile.lock` → pipenv
4. Default → pip

## Multi-Provider Architecture

### Provider Registry System

Lightfold uses a provider registry pattern that makes adding new cloud providers straightforward. All providers implement the `Provider` interface and auto-register themselves at init time.

**Key Components:**
1. **Provider Interface** (`pkg/providers/provider.go`): Defines standard methods all providers must implement
2. **Provider Registry** (`pkg/providers/registry.go`): Central factory for creating provider instances
3. **Provider Implementations**: Self-contained packages (e.g., `pkg/providers/digitalocean/`, `pkg/providers/hetzner/`)

**Adding a New Provider:**

See **[docs/ADDING_NEW_PROVIDERS.md](docs/ADDING_NEW_PROVIDERS.md)** for a complete step-by-step integration guide.

Quick overview:
```go
// 1. Create pkg/providers/newprovider/client.go
package newprovider

func init() {
    providers.Register("newprovider", func(token string) providers.Provider {
        return NewClient(token)
    })
}

// 2. Implement the Provider interface
type Client struct { token string }
func (c *Client) Name() string { return "newprovider" }
func (c *Client) DisplayName() string { return "New Provider" }
// ... implement remaining interface methods
```

**Critical Integration Points (Complete E2E Checklist):**

See **[docs/ADDING_NEW_PROVIDERS.md](docs/ADDING_NEW_PROVIDERS.md)** for detailed guide. Essential steps:

1. **Provider Package** (`pkg/providers/newprovider/client.go`):
   - Implement `Provider` interface
   - Register in `init()` with `providers.Register()`
   - Add blank import in `pkg/deploy/orchestrator.go`

2. **Config** (`pkg/config/config.go`):
   - Add `NewProviderConfig` struct with `GetIP()`, `GetUsername()`, `GetSSHKey()`, `IsProvisioned()`, `GetServerID()`
   - Add `GetNewProviderConfig()` helper method
   - Add case to `GetSSHProviderConfig()` and `GetAnyProviderConfig()` switches

3. **Orchestrator** (`pkg/deploy/orchestrator.go`):
   - Add case to `getProvisioningParams()` - extracts region, size, SSH keys from config
   - Add case to `updateProviderConfigWithServerInfo()` - stores IP, server ID, metadata from provisioned server

4. **Provider Bootstrap** (`cmd/provider_bootstrap.go`):
   - Add entry to `providerBootstraps` array with canonical name, aliases, config key, default username, fallback flow

5. **Provider State** (`cmd/provider_state.go`):
   - Add entry to `providerStateHandlers` map with display name, config accessor, IP recovery function

6. **UI Flow** (`cmd/ui/sequential/provision_newprovider.go`):
   - Create `RunProvisionNewProviderFlow()` function for interactive provisioning prompts

7. **IP Recovery** (`cmd/utils/provider_recovery.go` or inline in `cmd/common.go`):
   - Implement recovery logic when IP missing but server ID exists (calls provider's `GetServer()`)

**Testing checklist:**
- Provision new server, verify IP stored
- Deploy app, verify SSH connection
- Destroy server, verify cleanup
- Test IP recovery (delete IP from config, run configure)
- Test multi-app deployment to same server

### Deployment Flow Architecture

**Composable Command Pattern with Auto-Creation:**

Lightfold uses a composable architecture where each deployment step is an independent, idempotent command. Commands are also exposed as reusable functions in `cmd/common.go` for maximum code reuse.

**Core Reusable Functions:**
- `createTarget(targetName, projectPath, cfg)` - Creates or returns existing target
- `configureTarget(target, targetName, force)` - Configures or skips if already configured
- Both functions contain built-in idempotency checks

**1. Framework Detection** (`lightfold detect` or `lightfold .`)
- Analyzes project structure to identify framework
- Outputs JSON for automation or shows interactive results
- Can be run standalone for framework verification

**2. Infrastructure Creation** (`lightfold create --target <name>`)
- **BYOS Mode** (`--provider byos`): Validates existing server SSH access
  - Requires: `--ip`, `--ssh-key`, `--user`
  - Writes `/etc/lightfold/created` marker on server
  - Stores config under `provider: "byos"` key (NOT under digitalocean!)
- **Provision Mode** (`--provider do|vultr|hetzner`): Auto-provisions new server
  - Requires: `--region`, `--size`, API token
  - Generates SSH keys, provisions via cloud API
  - Stores server ID and IP in config
- Marks target as "created" in local state
- Idempotent: Skips if already created (returns existing config)
- **Reusable**: `createTarget()` function in `cmd/common.go`

**3. Server Configuration** (`lightfold configure --target <name>`)
- Checks for `/etc/lightfold/configured` marker
- Installs: runtime dependencies, systemd services, nginx, firewall rules
- Sets up deployment directory structure: `/srv/<app>/releases/`
- Writes marker on success, updates local state
- Idempotent: Skips if marker exists (unless `--force`)
- **Reusable**: `configureTarget()` function in `cmd/common.go`

**4. Release Deployment** (`lightfold push --target <name>`)
- Checks current git commit vs. last deployed commit
- Creates timestamped release: `/srv/<app>/releases/<timestamp>/`
- Uploads tarball, builds project, deploys with health checks
- Blue/green deployment: symlink swap with rollback on failure
- Updates state with commit hash and release ID
- Idempotent: Skips if commit unchanged

**5. Full Orchestration** (`lightfold deploy [PATH]`)
- **Primary user-facing command** (recommended workflow)
- Flexible invocation:
  - `lightfold deploy` - Deploy current directory
  - `lightfold deploy ~/path/to/project` - Deploy specific path
  - `lightfold deploy --target myapp` - Deploy named target
- First run: Interactively prompts for provider selection if target doesn't exist
- Auto-calls `createTarget()` and `configureTarget()` which handle idempotency internally
- Chains all steps: detect → create → configure → push
- Intelligently skips completed steps based on state (idempotency is automatic)
- `--force` flag reruns all steps
- `--dry-run` shows execution plan without running
- **Result**: True one-command deployment - users never need to run individual commands manually

**6. State Synchronization** (`lightfold sync [PATH]`)
- **Drift recovery command** for syncing local state/config with actual server state
- Flexible invocation:
  - `lightfold sync` - Sync current directory
  - `lightfold sync ~/path/to/project` - Sync specific path
  - `lightfold sync --target myapp` - Sync named target
- Use cases:
  - Local state files corrupted or deleted
  - Server IP changed (e.g., rebuilt server)
  - Recovering from bad state or manual server changes
  - After manually modifying server deployment
- Sync operations:
  - Recovers server IP from provider API (DigitalOcean, Vultr, Hetzner)
  - Verifies SSH connectivity
  - Syncs remote markers (`/etc/lightfold/created`, `/etc/lightfold/configured`)
  - Updates deployment info (current release, git commit, last deploy time)
  - Checks service status (informational)
- Preserves user-supplied data:
  - Domain configuration (never deleted)
  - Environment variables
  - Build/run command overrides
- **Reusable**: `syncTarget()` function in `cmd/common.go`

### Configuration Architecture

**Target-Based Configuration (`~/.lightfold/config.json`):**
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
          "username": "deploy",
          "ssh_key": "~/.lightfold/keys/lightfold_ed25519",
          "region": "nyc1",
          "size": "s-1vcpu-1gb",
          "provisioned": true,
          "droplet_id": "123456789"
        }
      },
      "deploy": {
        "env_vars": {"NODE_ENV": "production"},
        "skip_build": false
      }
    }
  }
}
```

**State Tracking (`~/.lightfold/state/<target>.json`):**
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

**Generic Provider Config Access:**
- All providers implement `ProviderConfig` interface with methods: `GetIP()`, `GetUsername()`, `GetSSHKey()`, `IsProvisioned()`
- Config stored as `map[string]json.RawMessage` to support any provider
- Helper methods like `GetDigitalOceanConfig()`, `GetHetznerConfig()` for convenience
- `SetProviderConfig(provider, config)` marshals and stores provider-specific configuration

### Security Architecture

**API Token Storage:**
- Tokens stored in `~/.lightfold/tokens.json` as `map[string]string` (provider → token)
- File permissions: 0600 (owner read/write only)
- Separate from main configuration for security
- Never included in project configs or logs
- Generic methods: `SetToken(provider, token)`, `GetToken(provider)`, `HasToken(provider)`

**SSH Key Management:**
- Auto-generation of Ed25519 keypairs for provisioned servers
- Keys stored in `~/.lightfold/keys/` with project-specific names
- Automatic public key upload to cloud providers
- Secure private key permissions (0600)

### Idempotency Patterns

**Local State Tracking:**
- File: `~/.lightfold/state/<target>.json`
- Functions: `LoadState()`, `MarkCreated()`, `MarkConfigured()`, `UpdateDeployment()`
- Usage: Commands check state before executing, skip if already done

**Remote State Markers:**
- `/etc/lightfold/created` - Written by `create` command
- `/etc/lightfold/configured` - Written by `configure` command
- Checked via SSH: `test -f /etc/lightfold/configured && echo 'configured'`

**Git Commit Tracking:**
- `push` command compares current commit with `last_commit` in state
- Skips deployment if commit unchanged (unless `--force`)
- Updates state with new commit hash on successful deploy

**Force Flags:**
- `--force` on individual commands: Reruns that specific step
- `--force` on `deploy`: Reruns all steps regardless of state

### Deployment Orchestrator Architecture

**Provider-Agnostic Design:**

The deployment orchestrator (`pkg/deploy/orchestrator.go`) handles auto-provisioning for cloud providers. The deployment executor (`pkg/deploy/executor.go`) handles SSH-based deployment logic that works identically across all providers.

**Provisioning Flow (orchestrator.go):**
1. **Registry Lookup**: `provider := providers.GetProvider(config.Provider, token)`
2. **Validation**: Provider validates credentials via its API
3. **Provisioning**: Provider creates server, uploads SSH keys, configures cloud-init
4. **Config Update**: Stores server IP, ID, and credentials in target config

**Deployment Flow (executor.go):**
1. **SSH Connection**: Connect using IP, username, SSH key from config
2. **Release Creation**: Create timestamped directory `/srv/<app>/releases/<timestamp>/`
3. **Upload & Build**: Upload tarball, extract, run build commands
4. **Environment Setup**: Write `.env` file with user-provided variables
5. **Blue/Green Deploy**: Swap symlink `/srv/<app>/current` with health checks
6. **Auto Rollback**: Revert to previous release if health checks fail
7. **Cleanup**: Keep last 5 releases, remove older ones

**Key Insight**: Once a server has an IP, username, and SSH key, deployment is identical across all providers. Only the provisioning step is provider-specific.

**IP Recovery Logic (cmd/common.go:configureTarget):**

When a deployment gets into a bad state (e.g., server provisioned but IP missing from config), Lightfold automatically recovers:

1. **Detection**: Before validating config, checks if `IP == ""` but `DropletID/ServerID != ""`
2. **Fallback to State**: If provider config doesn't have server ID, looks it up from state file (`provisioned_id`)
3. **API Fetch**: Calls provider's `GetServer(dropletID)` to fetch current server info
4. **Config Update**: Updates target config with fetched IP and server ID, saves to disk
5. **Continue**: Configuration proceeds normally with recovered IP

**Adding IP Recovery for New Providers:**
```go
// In configureTarget(), add recovery logic for your provider:
if target.Provider == "newprovider" {
    if npConfig, ok := target.ProviderConfig["newprovider"].(*config.NewProviderConfig); ok {
        serverID := npConfig.ServerID
        // Fallback to state if needed
        if serverID == "" {
            if targetState, err := state.GetTargetState(targetName); err == nil {
                serverID = targetState.ProvisionedID
            }
        }
        if npConfig.IP == "" && serverID != "" {
            fmt.Println("Recovering server IP from NewProvider...")
            if err := recoverIPFromNewProvider(&target, targetName, serverID); err != nil {
                return fmt.Errorf("failed to recover IP: %w", err)
            }
        }
    }
}
```

This ensures robustness and prevents "stuck in bad state" scenarios.

**Supported Providers:**
- ✅ AWS EC2 (fully implemented with IP recovery, Elastic IP support, security group management)
- ✅ DigitalOcean (fully implemented with IP recovery)
- ✅ Vultr (fully implemented with IP recovery)
- ✅ Hetzner Cloud (fully implemented with IP recovery)
- ✅ Linode (fully implemented with IP recovery)
- ✅ BYOS (Bring Your Own Server - no provisioning, just deployment)
- ✅ Fly.io (fully implemented with flyctl + nixpacks, container-based deployment)
- 🔜 Google Cloud, Azure (trivial to add with IP recovery pattern)

### Fly.io Provider: Container-Based Deployment

**Architecture:** Unlike SSH-based VPS providers, Fly.io uses **container deployments** with flyctl + native nixpacks.

**Deployment Flow:**

1. **Create (Step 2)**: Fly.io SDK creates app shell via GraphQL API
   - App name: auto-generated with timestamp suffix (e.g., `my-project-72829-142`)
   - Stores: `app_name`, `organization_id`, `ip` in config
   - Config key: `provider: "flyio"`

2. **Configure (Step 3)**: Skipped (no SSH needed)

3. **Deploy (Step 4)**: `flyctl` with remote builder
   - Generates `fly.toml` with health checks and VM size from user's machine selection
   - Sets secrets: `flyctl secrets set KEY=VALUE --app <app-name>`
   - Deploys: `flyctl deploy --nixpacks --remote-only --primary-region <region> --app <app-name>`
   - Authentication: `FLY_ACCESS_TOKEN` environment variable (not `FLY_API_TOKEN`)
   - Fly.io's remote builder auto-detects framework and builds on their infrastructure

**Key Implementation Details:**

- **No Dockerfile needed**: `--nixpacks` flag uses Fly.io's native nixpacks support
- **Remote builds**: `--remote-only` builds on Fly.io infrastructure (no local Docker)
- **VM size**: `fly.toml` `[[vm]]` section respects user's machine size choice (256MB-16GB)
- **App names**: Generated by SDK, stored in config, passed to all flyctl commands
- **Destroy flow**: SDK deletes app via GraphQL (includes machines, IPs automatically)

**Files:**
- `pkg/providers/flyio/client.go` - Fly.io SDK (app creation, destroy)
- `pkg/flyctl/flyctl.go` - flyctl CLI wrapper (deploy, secrets, logs, status)
- `pkg/deploy/flyio_deployer.go` - Deployment orchestrator
- `pkg/providers/flyio/toml.go` - fly.toml generator with dynamic VM sizing

**Quota Limits:** Free tier may have CPU/memory limits. Smallest size: `shared-cpu-1x` (256MB, 1 vCPU).

### AWS EC2 Provider: Traditional VPS with Advanced Networking

**Architecture:** AWS EC2 uses traditional SSH-based VPS deployment with automatic security group and optional Elastic IP management.

**Key Features:**
- **Authentication**: AWS Access Key/Secret Key OR AWS profile support
- **Security Groups**: Auto-created and cleaned up on destroy
- **Elastic IP**: Optional static IP allocation (user prompted during provisioning)
- **AMI Lookup**: Dynamic Ubuntu 22.04 AMI via SSM Parameter Store
- **IP Recovery**: Automatic IP recovery when config incomplete
- **Resource Cleanup**: Complete cleanup on destroy (instance, Elastic IP, security group)

**Files:**
- `pkg/providers/aws/client.go` - AWS SDK client and provider interface
- `pkg/providers/aws/auth.go` - Credential parsing (access key + profile)
- `pkg/providers/aws/security_group.go` - Security group lifecycle management
- `pkg/providers/aws/elastic_ip.go` - Elastic IP allocation and cleanup

**Setup and Examples:**
- **Setup Guide**: [docs/AWS_SETUP.md](docs/AWS_SETUP.md) - IAM permissions, authentication, troubleshooting
- **Examples**: [docs/AWS_EXAMPLES.md](docs/AWS_EXAMPLES.md) - Real-world deployment scenarios

### Domain & SSL Architecture

**Overview:**
Lightfold supports optional custom domains with automatic Let's Encrypt SSL certificates. The architecture uses pluggable SSL and proxy managers for extensibility.

**Key Components:**

1. **SSL Manager System** (`pkg/ssl/`):
   - Registry pattern for pluggable SSL backends
   - **Certbot Manager**: Uses Let's Encrypt via certbot for automatic certificate issuance
   - Interface methods: `IsAvailable()`, `IssueCertificate(domain, email)`, `RenewCertificate(domain)`, `EnableAutoRenewal()`
   - Auto-renewal via systemd timer (`certbot.timer`)
   - Certificate storage: `/etc/letsencrypt/live/{domain}/`

2. **Proxy Manager System** (`pkg/proxy/`):
   - Registry pattern for pluggable reverse proxies
   - **Nginx Manager**: Template-based HTTP/HTTPS configuration
   - Interface methods: `Configure(ProxyConfig)`, `Reload()`, `Remove(appName)`, `GetConfigPath(appName)`, `IsAvailable()`
   - HTTP-only config: Basic proxy_pass to app port
   - HTTPS config: SSL certificates + HTTP→HTTPS redirect + security headers

3. **Domain Configuration** (`pkg/config/DomainConfig`):
   ```go
   type DomainConfig struct {
       Domain     string // example.com
       SSLEnabled bool   // true if Let's Encrypt enabled
       SSLManager string // "certbot"
       ProxyType  string // "nginx"
   }
   ```

4. **Domain Commands** (`cmd/domain.go`):
   - `lightfold domain add --domain example.com` - Configure domain + SSL
   - `lightfold domain remove` - Revert to IP-based access
   - `lightfold domain show` - Display current domain config
   - All commands support 3 invocation patterns (current dir, path arg, --target flag)

**Domain Configuration Flow:**

1. User runs `lightfold domain add --domain example.com`
2. Target validation: ensures target is created and configured
3. SSH connection test to server
4. User prompted for SSL enable (default: yes)
5. If SSL enabled:
   - Check if certbot is installed
   - Install certbot if needed: `apt-get install -y certbot python3-certbot-nginx`
   - Issue certificate: `certbot --nginx -d example.com --non-interactive --agree-tos --email noreply@example.com`
   - Enable auto-renewal: `systemctl enable certbot.timer`
6. Configure nginx with domain (HTTP or HTTPS based on SSL choice)
7. Reload nginx: `systemctl reload nginx`
8. Update target config with domain settings
9. Save config to `~/.lightfold/config.json`

**Domain Removal Flow:**

1. User runs `lightfold domain remove`
2. Confirmation prompt with current domain info
3. Revert nginx to IP-based configuration (no domain, no SSL)
4. Reload nginx
5. Clear domain config from target
6. Save config

**Optional Integration:**

- During `lightfold configure` or `lightfold deploy`, user is prompted:
  ```
  Want to add a custom domain? (y/N):
  Domain: example.com
  Enable SSL with Let's Encrypt? (Y/n):
  ```
- If declined, helpful hint is shown after deployment
- Domain configuration is completely optional and never blocks deployments

**Design Principles:**

- **DRY**: Reusable SSL/proxy managers via interfaces and registry pattern
- **Extensible**: Easy to add Caddy, custom SSL providers, or other proxies
- **Optional**: Never blocks deployments, graceful degradation on failures
- **Idempotent**: Safe to rerun domain commands
- **Simple**: Sensible defaults, minimal user input required

## Development Guidelines

### Code Style

- Use Go conventions (gofmt, golint)
- Clear, descriptive function names
- Minimal external dependencies
- Error handling at appropriate levels

### UI/UX Guidelines

- **Prefer Bubbletea progress UI** (`cmd/ui/progress.go`) for long-running operations and progress tracking
- **Use lipgloss for all styled output** - Never use plain `fmt.Printf` for user-facing messages
- Maintain consistent styling across all output:
  - Success checkmarks: Color "82" (green) with "✓"
  - Description text: Color "245" (darker gray)
  - In-progress: Color "226" (yellow) with spinner
  - Error: Color "196" (red) with "✗"
  - Step headers: Color "86" (cyan), bold
- Pattern: `fmt.Printf("%s\n", lipgloss.NewStyle().Foreground(...).Render("text"))`
- Deployment steps should flow through progress callbacks to Bubbletea UI when possible
- For simple output, use `fmt.Printf` with lipgloss styles to maintain visual consistency

### Testing Strategy

- Create minimal test projects for each framework
- Test all package manager combinations
- Verify JSON output format consistency
- Test edge cases (multiple frameworks, ambiguous projects)

### Performance Considerations

- Minimize file system operations
- Cache file reads where possible
- Skip unnecessary directories (.git, node_modules, etc.)
- Use efficient string matching

## Common Patterns

### Framework Detection Template

```go
// FrameworkName
{
    score := 0.0
    signals := []string{}

    // High-priority indicators
    if has("framework.config.js") {
        score += 3
        signals = append(signals, "framework config")
    }

    // Medium-priority indicators
    if has("package.json") {
        pj := strings.ToLower(read("package.json"))
        if strings.Contains(pj, `"framework"`) {
            score += 2.5
            signals = append(signals, "package.json has framework")
        }
    }

    // Low-priority indicators
    if dirExists(root, "src/framework") {
        score += 0.5
        signals = append(signals, "framework structure")
    }

    if score > 0 {
        cands = append(cands, candidate{
            name:     "FrameworkName",
            score:    score,
            language: "Language",
            signals:  signals,
            plan:     frameworkPlan,
        })
    }
}
```

### Plan Function Template

```go
func frameworkPlan(root string) ([]string, []string, map[string]any, []string) {
    pm := detectPackageManager(root)  // or detectPythonPackageManager(root)

    build := []string{
        getJSInstallCommand(pm),  // or getPythonInstallCommand(pm)
        "framework build command",
    }

    run := []string{
        "framework run command",
    }

    health := map[string]any{
        "path": "/health",
        "expect": 200,
        "timeout_seconds": 30,
    }

    env := []string{"ENV_VAR1", "ENV_VAR2"}

    return build, run, health, env
}
```

## Command Development Guidelines

### Adding New Commands

When adding new composable commands, follow these patterns:

**0. Resolve Target (Unified Pattern):**
```go
// Use resolveTarget() helper for consistent target resolution
// Supports: current dir, path arg, or --target flag
var pathArg string
if len(args) > 0 {
    pathArg = args[0]
}

cfg := loadConfigOrExit()
target, targetName := resolveTarget(cfg, targetFlag, pathArg)
// Now target and targetName are resolved and ready to use
```

**1. Check State First (Idempotency):**
```go
// Check local state
if state.IsCreated(targetName) {
    fmt.Printf("Target '%s' is already created.\n", targetName)
    os.Exit(0)
}

// Or check remote marker
result := sshExecutor.Execute("test -f /etc/lightfold/configured && echo 'configured'")
if result.ExitCode == 0 && strings.TrimSpace(result.Stdout) == "configured" {
    fmt.Println("Already configured. Use --force to reconfigure.")
    os.Exit(0)
}
```

**2. Execute Operation:**
```go
// Perform the actual work
if err := doWork(); err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)
}
```

**3. Update State:**
```go
// Update local state
if err := state.MarkCreated(targetName, serverID); err != nil {
    return fmt.Errorf("failed to update state: %w", err)
}

// And/or write remote marker
sshExecutor.Execute("echo 'created' | sudo tee /etc/lightfold/created > /dev/null")
```

### Target-Based Config Pattern

**Loading Config:**
```go
cfg, err := config.LoadConfig()
target, exists := cfg.GetTarget(targetName)
if !exists {
    fmt.Printf("Target '%s' not found\n", targetName)
    os.Exit(1)
}
```

**Updating Config:**
```go
target.Provider = "digitalocean"
target.SetProviderConfig("digitalocean", doConfig)
cfg.SetTarget(targetName, target)
cfg.SaveConfig()
```

### Command Composability

Commands should be usable standalone or as part of the orchestrator.

**All commands support three invocation patterns:**

```bash
# Pattern 1: Current directory (default)
lightfold configure
lightfold push
lightfold status

# Pattern 2: Specific path
lightfold configure ~/Projects/myapp
lightfold push ~/Projects/myapp
lightfold status ~/Projects/myapp

# Pattern 3: Named target
lightfold configure --target myapp
lightfold push --target myapp
lightfold status --target myapp
```

**Orchestrated Usage:**
```bash
# Deploy current directory
lightfold deploy

# Deploy specific path
lightfold deploy ~/Projects/myapp

# Deploy named target
lightfold deploy --target myapp
```

**New Management Commands:**
```bash
# View logs
lightfold logs                  # Current directory
lightfold logs --tail           # Stream in real-time
lightfold logs --lines 200      # Last 200 lines

# Rollback
lightfold rollback              # Current directory (with confirmation)
lightfold rollback --force      # Skip confirmation

# Enhanced status
lightfold status                # List all targets
lightfold status .              # Current directory details
lightfold status --json         # JSON output for automation
```

### Utility Packages (`pkg/util/`)

Shared utility functions for common operations:

- **Environment parsing** (`env.go`): `.env` file loading and parsing
- **Project validation** (`project.go`): Project path validation and cleaning

### Cloud-Init Templates (`pkg/providers/cloudinit/`)

Generates cloud-init user data for server provisioning:

- Template-based configuration generation
- Package installation (nginx, docker, nodejs, python)
- UFW firewall rules
- Systemd service generation
- User setup with SSH keys

## Extension Points

### Adding New Package Managers

1. Update detection functions (`detectPackageManager`, `detectPythonPackageManager`)
2. Add command generation functions (`getJSInstallCommand`, etc.)
3. Update all relevant plan functions
4. Test with sample projects

### Adding New Languages

1. Update `mapExt()` function with new file extensions
2. Create language-specific package manager detection
3. Add appropriate plan functions
4. Update documentation

### Composable Commands
- [ ] `create` - BYOS and provision modes, 3 invocation patterns
- [ ] `configure` - Idempotency and force flag, 3 invocation patterns
- [ ] `push` - Git commit tracking, 3 invocation patterns
- [ ] `deploy` - Step skipping, dry-run, force, 3 invocation patterns
- [ ] `status` - Config, state, server status, health checks, uptime, --json
- [ ] `logs` - Fetch logs, --tail streaming, --lines customization
- [ ] `rollback` - Previous release rollback with confirmation
- [ ] `sync` - State/config sync, IP recovery, remote marker sync, 3 invocation patterns
- [ ] `config` - Token storage, target management
- [ ] `ssh` - Interactive SSH sessions
- [ ] `destroy` - VM destruction, local cleanup

### Idempotency
- [ ] State file creation and updates
- [ ] Remote marker checks (created, configured)
- [ ] Git commit comparison
- [ ] Force flag overrides
- [ ] Orchestrator skip logic

### Multi-Provider
- [ ] DigitalOcean provisioning
- [ ] Hetzner provisioning
- [ ] BYOS (no provisioning)
- [ ] Provider config serialization
- [ ] SSH key generation and upload

### Testing Approach
Our tests use **dynamic test project creation** instead of static fixtures:
```go
projectPath := createTestProject(t, map[string]string{
    "package.json": `{"dependencies": {"next": "^13.0.0"}}`,
    "next.config.js": "module.exports = {}",
})
```
This approach provides better test isolation and easier maintenance.

## Debugging Tips

- Use `go build -o lightfold . && ./lightfold path/to/test/project` for quick testing
- Check scoring with debug prints in detection blocks
- Verify file system operations with `ls -la` in test directories
- Test JSON parsing with `jq` or similar tools

## Security Considerations

- Never execute detected commands automatically
- Sanitize file paths and prevent directory traversal
- Avoid reading sensitive files outside project scope
- Validate all user inputs (file paths, etc.)

## Key Design Principles

1. **Composable** - Each command works standalone and can be chained
2. **Idempotent** - Safe to rerun without side effects, checks state before executing
3. **Stateful** - Tracks progress locally and remotely, skips completed work
4. **Provider-Agnostic** - Unified interface across clouds and BYOS
5. **Release-Based** - Timestamped releases with blue/green deployment and rollback
6. **Target-Based** - Named deployment targets, not path-based projects
7. **Multi-App Ready** - Deploy multiple apps to one server with automatic port allocation

## Quick Reference

**Command Flow:**
```bash
# Full deployment (orchestrated) - 3 invocation patterns + builder flag
lightfold deploy                       # Deploy current directory (auto-detects builder)
lightfold deploy ~/Projects/myapp      # Deploy specific path (auto-detects builder)
lightfold deploy --target myapp        # Deploy named target (auto-detects builder)
lightfold deploy --builder nixpacks    # Force nixpacks builder

# Individual steps (composable) - all support 3 patterns
lightfold create                       # Current directory
lightfold configure ~/Projects/myapp   # Specific path
lightfold push --target myapp          # Named target

# Management commands - all support 3 patterns
lightfold status                       # List all targets
lightfold status .                     # Current directory
lightfold status --target myapp        # Named target
lightfold status --json                # JSON output

lightfold logs                         # Current directory logs
lightfold logs --tail                  # Stream logs in real-time
lightfold logs --lines 200             # Last 200 lines

lightfold rollback                     # Rollback current directory
lightfold rollback --force             # Skip confirmation

lightfold sync                         # Sync current directory state
lightfold sync --target myapp          # Sync named target state

# Configuration
lightfold config list
lightfold config set-token digitalocean

# Domain & SSL Management - all support 3 patterns
lightfold domain add --domain example.com    # Add domain to current directory
lightfold domain add --domain app.com --target myapp  # Add to named target
lightfold domain remove                # Remove domain from current directory
lightfold domain show --target myapp   # Show domain config for target

# Multi-App Server Management
lightfold server list                  # List all servers and their apps
lightfold server show 192.168.1.100    # Show server details and all deployed apps
lightfold deploy --server-ip 192.168.1.100  # Deploy new app to existing server

# Utilities
lightfold ssh --target myapp           # SSH into server
lightfold destroy --target myapp       # Destroy VM and cleanup
```

**File Locations:**
- Config: `~/.lightfold/config.json` (targets)
- Tokens: `~/.lightfold/tokens.json` (API tokens, 0600)
- State: `~/.lightfold/state/<target>.json` (per-target state)
- Server State: `~/.lightfold/servers/<server-ip>.json` (multi-app tracking)
- SSH Keys: `~/.lightfold/keys/` (generated keypairs)
- Remote markers: `/etc/lightfold/{created,configured}` (on server)
- Releases: `/srv/<app>/releases/<timestamp>/` (on server)

## Notes & Considerations

- Keep `README.md` straightforward, lean, and no-bs. Only the most important information for the end user.
- **Unified command pattern**: All commands support 3 invocation methods (current dir, path arg, --target flag)
- State tracking is dual: local JSON files + remote markers on servers
- Idempotency is critical - commands check state first, then execute, then update state
- **Code reuse pattern**: Core logic extracted to `cmd/common.go` and refactored utilities
  - `resolveTarget()` - Unified target resolution from 3 input patterns (DRY principle)
  - `createTarget()` and `configureTarget()` - Reusable deployment logic
  - Cobra commands are thin wrappers that call these shared functions
  - `deploy` orchestrator calls these reusable functions directly
  - Single source of truth for all deployment logic
- **Provider-agnostic refactoring**: Registry patterns for extensibility
  - `cmd/provider_bootstrap.go` - Provider registration with O(1) lookup, SSH key generation
  - `cmd/provider_state.go` - Unified IP recovery handlers for all providers
  - `pkg/runtime/installers/` - Pluggable runtime installer system with auto-registration
  - Adding new providers/runtimes requires only one new entry, zero changes to core logic
- **New convenience commands**:
  - `logs` - View application logs via journalctl (supports --tail and --lines)
  - `rollback` - Standalone rollback command (removed from deploy --rollback)
  - `sync` - Sync local state/config with actual server state (drift recovery)
  - Enhanced `status` - Now includes app uptime, health checks, commit info, --json support
- **Provider configuration storage**:
  - BYOS targets MUST store config under `provider: "byos"` key
  - DigitalOcean targets use `provider: "digitalocean"` key
  - Vultr targets use `provider: "vultr"` key
  - Hetzner targets use `provider: "hetzner"` key
  - NEVER hardcode one provider's key when storing another provider's config!
- TUI components (`cmd/ui/`) used for interactive flows during initial setup
- Cloud-init templates (`pkg/providers/cloudinit/`) handle server bootstrapping during provisioning
- Utility packages (`pkg/util/`) provide shared helpers for env parsing and project validation

## Release & Distribution

See [docs/RELEASING.md](docs/RELEASING.md) for detailed release process and troubleshooting.
