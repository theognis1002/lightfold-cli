# Contributing to Lightfold CLI

Thank you for your interest in contributing to Lightfold CLI! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Contributing Process](#contributing-process)
- [Framework Detection](#framework-detection)
- [Testing](#testing)
- [Documentation](#documentation)
- [Release Process](#release-process)

## Code of Conduct

By participating in this project, you agree to:

- Be respectful and inclusive
- Focus on constructive feedback
- Help maintain a welcoming environment
- Follow community guidelines

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git
- Basic understanding of web frameworks and build tools

### Development Environment

1. **Fork and Clone**:
   ```bash
   git clone https://github.com/your-username/lightfold.git
   cd lightfold
   ```

2. **Install Dependencies**:
   ```bash
   go mod download
   ```

3. **Build and Test**:
   ```bash
   go build -o lightfold .
   ./lightfold --help
   ```

## Development Setup

### Project Structure

```
lightfold/
├── main.go              # Core detection logic
├── cmd/                 # CLI commands
│   └── root.go         # Root command definition
├── go.mod              # Go dependencies
├── README.md           # User documentation
├── AGENTS.md           # Developer context
└── test-projects/      # Test frameworks (local)
```

### Building

```bash
# Development build
go build -o lightfold .

# Cross-compilation examples
GOOS=linux GOARCH=amd64 go build -o lightfold-linux .
GOOS=windows GOARCH=amd64 go build -o lightfold-windows.exe .
GOOS=darwin GOARCH=arm64 go build -o lightfold-macos-arm64 .
```

## Contributing Process

### 1. Issue Creation

Before contributing:

- Check existing issues and PRs
- Create an issue for bugs or feature requests
- Discuss significant changes in issues first

### 2. Development Workflow

```bash
# Create feature branch
git checkout -b feature/framework-name

# Make changes and test
go build -o lightfold .
./lightfold path/to/test/project

# Commit with clear messages
git commit -m "Add detection for Framework X

- Add scoring logic for framework config files
- Implement build plan with package manager detection
- Add tests for npm/yarn/pnpm/bun variants"
```

### 3. Pull Request Guidelines

- **Clear Title**: Descriptive title explaining the change
- **Description**: Explain what, why, and how
- **Testing**: Include test results and sample projects
- **Documentation**: Update README if needed
- **Small PRs**: Keep changes focused and reviewable

## Framework Detection

### Adding New Frameworks

1. **Research Phase**:
   ```bash
   # Create test project
   mkdir test-projects/new-framework
   # Set up minimal project structure
   # Document unique identifiers
   ```

2. **Implementation**:

   Add detection block in `main.go`:
   ```go
   // NewFramework
   {
       score := 0.0
       signals := []string{}

       // High-priority indicators (3+ points)
       if has("framework.config.js") {
           score += 3
           signals = append(signals, "framework config")
       }

       // Medium-priority indicators (2-2.5 points)
       if has("package.json") {
           pj := strings.ToLower(read("package.json"))
           if strings.Contains(pj, `"new-framework"`) {
               score += 2.5
               signals = append(signals, "package.json has new-framework")
           }
       }

       // Low-priority indicators (0.5-1 points)
       if dirExists(root, "src/pages") {
           score += 0.5
           signals = append(signals, "framework structure")
       }

       if score > 0 {
           cands = append(cands, candidate{
               name:     "NewFramework",
               score:    score,
               language: "JavaScript/TypeScript",
               signals:  signals,
               plan:     newFrameworkPlan,
           })
       }
   }
   ```

3. **Plan Function**:
   ```go
   func newFrameworkPlan(root string) ([]string, []string, map[string]any, []string) {
       pm := detectPackageManager(root)

       build := []string{
           getJSInstallCommand(pm),
           "framework build",
       }

       run := []string{
           "framework start --port 3000",
       }

       health := map[string]any{
           "path": "/",
           "expect": 200,
           "timeout_seconds": 30,
       }

       env := []string{"FRAMEWORK_API_KEY", "NODE_ENV"}

       return build, run, health, env
   }
   ```

### Framework Detection Guidelines

- **Unique Identifiers**: Focus on framework-specific files
- **Scoring Logic**: Higher scores for more specific indicators
- **Package Manager Integration**: Always use detection functions
- **Fallback Safety**: Graceful degradation for ambiguous cases

### Package Manager Support

When adding new package managers:

1. **Detection Logic**:
   ```go
   func detectPackageManager(root string) string {
       switch {
       case fileExists(root, "new-pm.lock"):
           return "new-pm"
       // ... existing cases
       }
   }
   ```

2. **Command Generation**:
   ```go
   func getJSInstallCommand(pm string) string {
       switch pm {
       case "new-pm":
           return "new-pm install"
       // ... existing cases
       }
   }
   ```

## Testing

### Test Structure

1. **Create Test Projects**:
   ```bash
   mkdir -p test-projects/framework-name
   cd test-projects/framework-name

   # Create minimal project structure
   echo '{"name": "test", "dependencies": {"framework": "^1.0.0"}}' > package.json
   echo 'export default {}' > framework.config.js
   mkdir src
   ```

2. **Test Detection**:
   ```bash
   ./lightfold test-projects/framework-name
   ```

3. **Test Package Managers**:
   ```bash
   # Test with different package managers
   touch test-projects/framework-name/yarn.lock
   ./lightfold test-projects/framework-name

   rm test-projects/framework-name/yarn.lock
   touch test-projects/framework-name/pnpm-lock.yaml
   ./lightfold test-projects/framework-name
   ```

### Test Checklist

- [ ] Framework correctly detected
- [ ] Appropriate confidence score
- [ ] Correct package manager detection
- [ ] Build commands use right package manager
- [ ] JSON output validates
- [ ] Edge cases handled (multiple frameworks, etc.)

### Manual Testing

```bash
# Test various scenarios
./lightfold .                    # Self-detection (Go project)
./lightfold /path/to/nextjs      # Next.js project
./lightfold /path/to/django      # Django project
./lightfold /path/to/mixed       # Multiple frameworks
./lightfold /path/to/empty       # Empty directory
```

## Documentation

### README Updates

When adding frameworks, update:

- Supported frameworks list
- Detection examples
- Package manager compatibility

### Code Documentation

- Clear function names and comments
- Document complex detection logic
- Explain scoring rationale

## Release Process

### Version Numbering

- **Major** (1.0.0): Breaking changes or major new features
- **Minor** (1.1.0): New frameworks, package managers, or features
- **Patch** (1.1.1): Bug fixes, minor improvements

### Release Checklist

- [ ] All tests passing
- [ ] Documentation updated
- [ ] Version updated in `cmd/root.go`
- [ ] CHANGELOG.md updated
- [ ] Git tag created
- [ ] Binaries built for all platforms

## Common Patterns

### Error Handling

```go
// Prefer graceful degradation
if err != nil {
    // Log warning, continue with defaults
    continue
}
```

### File Operations

```go
// Always use helper functions
if fileExists(root, "config.js") {
    content := read("config.js")
    // Process content
}
```

### String Matching

```go
// Case-insensitive, safe matching
if strings.Contains(strings.ToLower(content), "framework") {
    // Framework detected
}
```

## Getting Help

- **Issues**: GitHub issues for bugs and features
- **Discussions**: GitHub discussions for questions
- **Documentation**: See AGENTS.md for detailed context

## Recognition

Contributors will be:

- Listed in release notes
- Credited in documentation
- Welcomed into the community

Thank you for contributing to Lightfold CLI! 🚀
