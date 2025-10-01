# Security Policy

## Supported Versions

We provide security updates for the following versions of Lightfold CLI:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security vulnerability in Lightfold CLI, please report it responsibly.

### How to Report

1. **Do NOT** create a public GitHub issue for security vulnerabilities
2. Email security issues to: [Create a private issue or contact maintainers]
3. Include the following information:
   - Description of the vulnerability
   - Steps to reproduce the issue
   - Potential impact assessment
   - Suggested fix (if available)

### Response Timeline

- **Initial Response**: Within 48 hours of report
- **Confirmation**: Within 7 days
- **Fix Timeline**: Varies based on severity and complexity
- **Public Disclosure**: After fix is released and users have time to update

## Security Considerations

### Input Validation

Lightfold CLI processes file system paths and project files. We implement several security measures:

- **Path Sanitization**: All input paths are cleaned and validated
- **Directory Traversal Prevention**: Restricted to specified project directories
- **File Size Limits**: Reasonable limits on file reading operations
- **Safe File Operations**: Read-only operations with proper error handling

### File System Access

The tool operates with the following file system constraints:

- **Read-Only**: Never modifies user files
- **Scope Limitation**: Only reads files within the specified project directory
- **Skip Sensitive Directories**: Automatically skips `.git`, `.env`, and other sensitive paths
- **No Execution**: Never executes discovered commands or scripts

### Data Handling

- **No Network Requests**: Tool operates entirely offline
- **No Data Collection**: No telemetry or analytics collection
- **Local Processing**: All analysis happens locally
- **No Persistence**: No data stored beyond the JSON output

### Dependencies

We maintain security by:

- **Minimal Dependencies**: Using only essential, well-maintained libraries
- **Regular Updates**: Keeping dependencies current with security patches
- **Dependency Scanning**: Regular security audits of dependencies
- **Trusted Sources**: Only using dependencies from trusted sources

## Security Best Practices for Users

### Safe Usage

1. **Verify Downloads**: Always download from official sources
2. **Check Checksums**: Verify file integrity when available
3. **Run in Containers**: Consider running in isolated environments for untrusted projects
4. **Update Regularly**: Keep Lightfold CLI updated to the latest version

### Project Analysis

When analyzing projects:

1. **Trusted Projects**: Only run on projects you trust
2. **Review Output**: Always review generated build plans before execution
3. **Sandbox Testing**: Test in isolated environments first
4. **Validate Dependencies**: Review detected dependencies and package managers

### Build Plan Execution

⚠️ **Important**: Lightfold CLI only detects and suggests build commands - it never executes them.

Before running suggested commands:

1. **Review All Commands**: Understand what each command does
2. **Check Package Managers**: Verify package manager installations
3. **Validate Dependencies**: Review package.json, requirements.txt, etc.
4. **Use Virtual Environments**: Isolate package installations when possible

## Known Security Considerations

### File Content Analysis

The tool reads file contents for detection purposes:

- **Limited Scope**: Only reads configuration and manifest files
- **No Sensitive Files**: Skips .env, credentials, and key files
- **Content Parsing**: Only performs pattern matching, no code execution
- **Memory Limits**: Reasonable limits to prevent memory exhaustion

### Framework Detection

Detection logic is designed to be safe:

- **Pattern Matching**: Uses simple string matching, not parsing
- **No Code Execution**: Never executes or evaluates discovered code
- **Static Analysis**: All analysis is static, no dynamic evaluation
- **Fail-Safe Design**: Defaults to safe, conservative detection

### Package Manager Detection

When detecting package managers:

- **Lockfile Analysis**: Only reads lockfiles, never modifies them
- **No Installation**: Never installs or updates packages
- **Command Generation**: Only generates commands, never executes them
- **Safe Defaults**: Falls back to well-known, safe defaults

## Vulnerability Disclosure Timeline

When a vulnerability is reported and confirmed:

1. **Day 0**: Vulnerability reported
2. **Days 1-7**: Initial triage and confirmation
3. **Days 7-30**: Fix development and testing
4. **Day 30+**: Coordinated disclosure and release
5. **Day 45+**: Public security advisory (if applicable)

## Security Resources

- [Go Security Best Practices](https://go.dev/doc/security/)
- [OWASP Secure Coding Practices](https://owasp.org/www-project-secure-coding-practices-quick-reference-guide/)
- [GitHub Security Advisories](https://docs.github.com/en/code-security/security-advisories)

## Acknowledgments

We appreciate security researchers and users who help keep Lightfold CLI secure. Responsible disclosure helps protect all users of the project.

---

*This security policy is subject to updates as the project evolves. Please check this document regularly for the latest security information.*