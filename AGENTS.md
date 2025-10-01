# AGENTS.md - Context for Coding Agents

This document provides essential context and guidelines for AI coding agents working on the Lightfold CLI project.

## Project Overview

Lightfold CLI is a fast, intelligent project framework detector that automatically identifies web frameworks and generates optimal build and deployment plans. Beyond detection, it provides an interactive TUI for configuring deployments to DigitalOcean and S3, with animated progress bars and success animations. The core goals are accuracy, speed, comprehensive framework coverage, and seamless deployment workflows.

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
   - Cobra-based command structure
   - Clean JSON output formatting
   - Interactive TUI mode and non-interactive mode
   - Error handling and validation

5. **TUI Components** (`pkg/tui/`):
   - Bubbletea-based interactive menus
   - Deployment target selection (DigitalOcean, S3)
   - Configuration input forms with validation
   - Animated progress bars and success animations

6. **Configuration Management** (`pkg/config/`):
   - JSON-based project configuration storage
   - Per-project deployment settings
   - Secure credential handling

### Code Organization

```
lightfold/
├── cmd/
│   ├── lightfold/
│   │   └── main.go     # Entry point
│   ├── root.go         # Main command with TUI integration
│   └── deploy.go       # Deployment command
├── pkg/
│   ├── detector/       # Framework detection engine
│   │   ├── detector.go # Core detection logic
│   │   ├── plans.go    # Build/run plans
│   │   └── exports.go  # Test helpers
│   ├── tui/           # Terminal UI components
│   │   ├── menu.go    # Deployment target selection
│   │   ├── input.go   # Configuration forms
│   │   ├── progress.go # Progress bars
│   │   └── animation.go # Success animations
│   └── config/        # Configuration management
│       └── config.go  # JSON config storage
├── test/
│   └── detector/      # Comprehensive test suite
├── go.mod             # Go module dependencies
├── Makefile           # Build and test commands
├── README.md          # User-facing documentation
└── AGENTS.md          # This file
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

1. **Update constants** in `pkg/tui/menu.go`:
   ```go
   const TargetNewService = "newservice"
   ```

2. **Add to deployment targets list**:
   ```go
   var deploymentTargets = []choice{
       // existing targets...
       {name: "New Service", value: TargetNewService, description: "Deploy to new service"},
   }
   ```

3. **Create configuration struct** in `pkg/config/config.go`:
   ```go
   type NewServiceConfig struct {
       Endpoint string `json:"endpoint"`
       APIKey   string `json:"api_key"`
   }
   ```

4. **Add input form** in `pkg/tui/input.go`:
   ```go
   func ShowNewServiceInputs() (*config.NewServiceConfig, error) {
       // Implementation similar to existing input functions
   }
   ```

5. **Update deployment logic** in `cmd/deploy.go`

### TUI Component Guidelines

- **Keep it simple**: Use built-in bubbletea components when possible
- **Consistent styling**: Use the established color scheme (86 for primary, 243 for secondary)
- **Error handling**: Always provide clear error messages and graceful fallbacks
- **Non-interactive fallback**: Ensure all TUI features work in CI/automation mode

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

This context should help you understand the codebase structure, patterns, and development practices for contributing effectively to Lightfold CLI.