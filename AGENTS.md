# AGENTS.md - Context for Coding Agents

This document provides essential context and guidelines for AI coding agents working on the Lightfold CLI project.

## Project Overview

Lightfold CLI is a fast, intelligent project framework detector that automatically identifies web frameworks and generates optimal build and deployment plans. Beyond detection, it provides an interactive TUI with a two-step deployment flow: users first choose between BYOS (Bring Your Own Server) or auto-provisioning, then select specific providers. Features secure API token storage, DigitalOcean droplet provisioning, S3 deployment, animated progress bars, and success animations. The core goals are accuracy, speed, comprehensive framework coverage, and seamless deployment workflows.

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
   - Cobra-based command structure with root and deploy commands
   - Clean JSON output formatting with `--json` flag
   - Interactive TUI mode with terminal detection
   - Non-interactive mode for CI/automation with `--no-interactive`
   - Error handling and graceful validation

5. **TUI Components** (`cmd/ui/`):
   - Bubbletea-based interactive menus and sequential flows
   - Two-step deployment configuration: BYOS vs Provision selection
   - Animated spinner during framework detection
   - Deployment target selection with enhanced styling
   - Step-by-step configuration forms with validation
   - DigitalOcean API token collection and secure storage
   - Region/size selection for auto-provisioning
   - Colorful gradient progress bars for deployment
   - SSH key generation and management
   - Secure input handling for sensitive data

6. **Configuration Management** (`pkg/config/`):
   - JSON-based project configuration storage
   - Per-project deployment settings
   - Secure API token storage in separate tokens.json file
   - Support for both BYOS and provisioned configurations

### Code Organization

```
lightfold/
├── cmd/
│   ├── lightfold/
│   │   └── main.go     # Entry point
│   ├── root.go         # Main command with detection and TUI integration
│   ├── deploy.go       # Deployment command with progress animation
│   ├── detect.go       # Explicit detection command
│   ├── flags/          # Command flags and deployment targets
│   ├── steps/          # Two-step deployment configuration definitions
│   ├── ui/             # TUI components
│   │   ├── detection/  # Detection results display
│   │   ├── multiInput/ # Menu selection components
│   │   ├── sequential/ # Step-by-step configuration flows (BYOS + Provision)
│   │   ├── spinner/    # Loading animations
│   │   └── progress.go # Colorful gradient progress bars
│   └── utils/          # Utility functions
├── pkg/
│   ├── detector/       # Framework detection engine
│   │   ├── detector.go # Core detection logic
│   │   ├── plans.go    # Build/run plans
│   │   └── exports.go  # Test helpers
│   └── config/         # Configuration management
│       └── config.go   # JSON config storage and validation
├── test/
│   └── detector/       # Comprehensive test suite
├── go-blueprint/       # External framework examples
├── go.mod              # Go module dependencies
├── Makefile            # Build and test commands
├── README.md           # User-facing documentation
└── AGENTS.md           # This file
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

## Deployment Flow Architecture

### Two-Step Configuration

**Step 1: Deployment Type Selection**
- **BYOS (Bring Your Own Server)**: User provides existing server details
- **Provision for me**: Auto-provision new infrastructure

**Step 2: Provider Selection**
- **BYOS Options**: DigitalOcean (existing droplet), Custom Server
- **Provision Options**: DigitalOcean (new droplet), S3 (static sites)

### BYOS vs Provision Flows

**BYOS Flow (existing behavior):**
- Collects server IP address
- Requests SSH key path or content
- Asks for username (default: root)
- Stores configuration for manual deployment

**Provision Flow (new functionality):**
- Collects DigitalOcean API token (stored securely)
- Presents region selection (nyc1, ams3, sfo3, etc.)
- Offers droplet size options (s-1vcpu-1gb, s-2vcpu-2gb, etc.)
- Auto-generates SSH keypairs locally
- Stores configuration for automatic provisioning

### Security Architecture

**API Token Storage:**
- Tokens stored in `~/.lightfold/tokens.json`
- File permissions: 0600 (owner read/write only)
- Separate from main configuration for security
- Never included in project configs or logs

**SSH Key Management:**
- Auto-generation of Ed25519 keypairs for provisioned droplets
- Keys stored in `~/.lightfold/keys/` with project-specific names
- Automatic public key upload to cloud providers
- Secure private key permissions (0600)

## Development Guidelines

### Code Style

- Use Go conventions (gofmt, golint)
- Clear, descriptive function names
- Minimal external dependencies
- Error handling at appropriate levels

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

## TUI Development

### Adding New Deployment Targets

**For BYOS targets:**

1. **Update steps configuration** in `cmd/steps/steps.go`:
   ```go
   "byos_target": {
       StepName: "BYOS Target",
       Options: []Item{
           // existing options...
           {
               Title: "New Service",
               Desc:  "Deploy to existing new service infrastructure",
           },
       },
   }
   ```

2. **Add sequential flow** in `cmd/ui/sequential/flows.go`:
   ```go
   func CreateNewServiceFlow(projectName string) *FlowModel {
       steps := []Step{
           CreateIPStep("ip", "203.0.113.1"),
           CreateSSHKeyStep("ssh_key"),
           CreateUsernameStep("username", "deploy"),
       }
       return NewFlow("Configure New Service Deployment", steps)
   }
   ```

**For Provision targets:**

1. **Update provision steps** in `cmd/steps/steps.go`:
   ```go
   "provision_target": {
       Options: []Item{
           // existing options...
           {
               Title: "New Service",
               Desc:  "Auto-provision new service infrastructure",
           },
       },
   }
   ```

2. **Add provision flow** in `cmd/ui/sequential/flows.go`:
   ```go
   func RunProvisionNewServiceFlow(projectName string) (*config.NewServiceConfig, error) {
       // Collect API credentials, configure auto-provisioning
   }
   ```

3. **Update configuration handling** in `cmd/root.go`

### TUI Component Guidelines

- **Keep it simple**: Use built-in bubbletea components when possible
- **Consistent styling**: Use the established color scheme (#01FAC6 for primary/focus, 86/170 for highlights)
- **Error handling**: Always provide clear error messages and graceful fallbacks
- **Non-interactive fallback**: Ensure all TUI features work in CI/automation mode
- **Terminal detection**: Check for proper TTY, TERM environment, and CI detection
- **Graceful degradation**: Fall back to text-based output if TUI fails
- **Enhanced visual feedback**: Use colorful gradient progress bars, spinners, and selection indicators
- **Sequential flows**: Break complex configuration into manageable step-by-step processes

### Deployment Command Behavior

**Smart Project Detection:**
- `lightfold deploy` - Auto-detects single configured project
- `lightfold deploy <path>` - Deploys specific project
- Shows helpful error messages with available projects list

**Fallback Logic:**
- Attempts TUI mode in interactive terminals
- Falls back to non-interactive mode if TUI fails (e.g., TTY issues)
- Respects `--no-interactive` flag
- Detects CI environments automatically

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

### CLI & TUI
- [ ] CLI argument parsing
- [ ] JSON vs interactive mode switching
- [ ] TUI menu navigation
- [ ] Form input validation
- [ ] Configuration saving/loading
- [ ] Deploy command functionality
- [ ] Error message clarity
- [ ] Non-interactive/CI mode compatibility

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

## Notes & Considerations

- Keep `README.md` straightforward, lean, and no-bs. Do not add excessive info there - only the most important information for the end user.


This context should help you understand the codebase structure, patterns, and development practices for contributing effectively to Lightfold CLI.