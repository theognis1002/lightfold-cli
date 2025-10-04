# AGENTS.md - Context for Coding Agents

This document provides essential context and guidelines for AI coding agents working on the Lightfold CLI project.

## Project Overview

Lightfold CLI is a framework detector and deployment tool with composable, idempotent commands. It automatically identifies 15+ web frameworks, then provides standalone commands for each deployment step (create, configure, push) that can be run independently or orchestrated together. Features include multi-provider support (DigitalOcean, Hetzner, BYOS), state tracking for smart skipping, blue/green deployments with rollback, and secure API token storage. The core design principles are composability, idempotency, and provider-agnostic deployment.

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

4. **CLI Interface** (`cmd/`):
   - **Composable Commands**: Independent commands for each deployment step
     - `detect` - Framework detection only (JSON output, standalone use)
     - `create` - Infrastructure creation (BYOS or auto-provision)
     - `configure` - Server configuration with idempotency checks
     - `push` - Release deployment with health checks
     - `deploy` - Orchestrator that chains all steps with smart skipping
     - `status` - View deployment state and server status
     - `config` - Manage targets and API tokens
     - `keygen` - Generate SSH keypairs
     - `ssh` - Interactive SSH sessions to deployment targets
     - `destroy` - Destroy VM and remove local configuration
   - Clean JSON output with `--json` flag
   - Target-based architecture (named targets, not paths)
   - Dry-run support (`--dry-run`) for preview
   - Force flags to override idempotency

5. **State Tracking** (`pkg/state/`):
   - Local state files per target: `~/.lightfold/state/<target>.json`
   - Remote state markers on servers: `/etc/lightfold/{created,configured}`
   - Git commit tracking to skip unchanged deployments
   - Tracks: last commit, last deploy time, last release ID, provision ID
   - Enables idempotent operations and intelligent step skipping

6. **Configuration Management** (`pkg/config/`):
   - **Target-Based Config**: Named deployment targets (not path-based)
   - Structure: `~/.lightfold/config.json` with `targets` map
   - Each target stores: project path, framework, provider, provider config
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
│   ├── status.go         # Deployment status viewer
│   ├── config.go         # Config/token management
│   ├── keygen.go         # SSH key generation
│   ├── ssh.go            # Interactive SSH sessions
│   ├── destroy.go        # VM destruction and cleanup
│   ├── common.go         # Shared helper functions
│   └── ui/               # TUI components
│       ├── detection/    # Detection results display
│       ├── sequential/   # Token collection flows
│       ├── spinner/      # Loading animations
│       ├── progress.go   # Deployment progress bars
│       └── animation.go  # Shared animations
├── pkg/
│   ├── detector/         # Framework detection engine
│   │   ├── detector.go   # Core detection logic
│   │   ├── plans.go      # Build/run plans
│   │   └── exports.go    # Test helpers
│   ├── config/           # Configuration management
│   │   ├── config.go     # Target-based config + tokens
│   │   └── deployment.go # Deployment options processing
│   ├── state/            # State tracking
│   │   └── state.go      # Local/remote state management
│   ├── deploy/           # Deployment logic
│   │   ├── orchestrator.go # Multi-provider orchestration
│   │   ├── executor.go   # Blue/green deployment executor
│   │   └── templates/    # Deployment templates
│   ├── ssh/              # SSH operations
│   │   ├── executor.go   # SSH command execution
│   │   └── keygen.go     # SSH key generation
│   ├── util/             # Shared utilities
│   │   ├── env.go        # .env file parsing
│   │   └── project.go    # Project path validation
│   └── providers/        # Cloud provider registry
│       ├── provider.go   # Provider interface
│       ├── registry.go   # Provider factory
│       ├── digitalocean/ # DigitalOcean implementation
│       ├── hetzner/      # Hetzner implementation
│       └── cloudinit/    # Cloud-init template generation
├── test/
│   ├── detector/         # Detection test suite
│   ├── deploy/           # Deployment tests
│   ├── ssh/              # SSH tests
│   ├── providers/        # Provider tests
│   ├── cloudinit/        # Cloud-init tests
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

Lightfold uses a provider registry pattern that makes adding new cloud providers trivial. All providers implement the `Provider` interface and auto-register themselves at init time.

**Key Components:**
1. **Provider Interface** (`pkg/providers/provider.go`): Defines standard methods all providers must implement
2. **Provider Registry** (`pkg/providers/registry.go`): Central factory for creating provider instances
3. **Provider Implementations**: Self-contained packages (e.g., `pkg/providers/digitalocean/`, `pkg/providers/hetzner/`)

**Adding a New Provider:**
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

That's it! The provider is now available throughout the application with zero changes to orchestrator, CLI, or config layers.

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
- **Provision Mode** (`--provider do|hetzner`): Auto-provisions new server
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

**5. Full Orchestration** (`lightfold deploy --target <name>`)
- **Primary user-facing command** (recommended workflow)
- First run: Interactively prompts for provider selection if target doesn't exist
- Auto-calls `createTarget()` and `configureTarget()` which handle idempotency internally
- Chains all steps: detect → create → configure → push
- Intelligently skips completed steps based on state (idempotency is automatic)
- `--force` flag reruns all steps
- `--dry-run` shows execution plan without running
- **Result**: True one-command deployment - users never need to run individual commands manually

### Configuration Architecture

**Target-Based Configuration (`~/.lightfold/config.json`):**
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
  "provisioned_id": "123456789"
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
- ✅ DigitalOcean (fully implemented with IP recovery)
- ✅ Hetzner Cloud (fully implemented with IP recovery)
- ✅ BYOS (Bring Your Own Server - no provisioning, just deployment)
- 🔜 Linode, Fly.io, AWS EC2, Google Cloud, Azure (trivial to add with IP recovery pattern)

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

Commands should be usable standalone or as part of the orchestrator:

**Standalone Usage:**
```bash
lightfold create --target myapp --provider byos --ip 1.2.3.4 --ssh-key ~/.ssh/id_rsa
lightfold configure --target myapp
lightfold push --target myapp
```

**Orchestrated Usage:**
```bash
lightfold deploy --target myapp  # Runs create → configure → push with smart skipping
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

## Testing Checklist

### Framework Detection
- [ ] Framework detection accuracy
- [ ] Package manager detection
- [ ] Build command generation
- [ ] JSON output format
- [ ] Edge case handling
- [ ] Performance with large projects

### Composable Commands
- [ ] `create` - BYOS and provision modes
- [ ] `configure` - Idempotency and force flag
- [ ] `push` - Git commit tracking, rollback
- [ ] `deploy` - Step skipping, dry-run, force
- [ ] `status` - Config, state, server status display
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

## Quick Reference

**Command Flow:**
```bash
# Full deployment (orchestrated)
lightfold deploy --target myapp

# Individual steps (composable)
lightfold create --target myapp --provider byos --ip 1.2.3.4 --ssh-key ~/.ssh/id_rsa
lightfold configure --target myapp
lightfold push --target myapp --env-file .env.production

# Management
lightfold status --target myapp
lightfold config list
lightfold config set-token digitalocean

# Utilities
lightfold ssh --target myapp           # SSH into server
lightfold destroy --target myapp       # Destroy VM and cleanup
```

**File Locations:**
- Config: `~/.lightfold/config.json` (targets)
- Tokens: `~/.lightfold/tokens.json` (API tokens, 0600)
- State: `~/.lightfold/state/<target>.json` (per-target state)
- SSH Keys: `~/.lightfold/keys/` (generated keypairs)
- Remote markers: `/etc/lightfold/{created,configured}` (on server)
- Releases: `/srv/<app>/releases/<timestamp>/` (on server)

## Notes & Considerations

- Keep `README.md` straightforward, lean, and no-bs. Only the most important information for the end user.
- All commands use `--target` flag to reference named targets, not paths
- State tracking is dual: local JSON files + remote markers on servers
- Idempotency is critical - commands check state first, then execute, then update state
- **Code reuse pattern**: Core logic extracted to `cmd/common.go` as `createTarget()` and `configureTarget()`
  - Cobra commands (`create`, `configure`) are thin wrappers
  - `deploy` orchestrator calls these reusable functions directly
  - Single source of truth for all deployment logic
- **Provider configuration storage**:
  - BYOS targets MUST store config under `provider: "byos"` key
  - DigitalOcean targets use `provider: "digitalocean"` key
  - Hetzner targets use `provider: "hetzner"` key
  - NEVER hardcode one provider's key when storing another provider's config!
- TUI components (`cmd/ui/`) used for interactive flows during initial setup
- Cloud-init templates (`pkg/providers/cloudinit/`) handle server bootstrapping during provisioning
- Utility packages (`pkg/util/`) provide shared helpers for env parsing and project validation

This context should help you understand the codebase structure, patterns, and development practices for contributing effectively to Lightfold CLI.