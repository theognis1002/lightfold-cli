# Releasing Lightfold CLI

This document outlines the release process for Lightfold CLI.

## Prerequisites

- Write access to the GitHub repository
- GoReleaser installed locally (for testing): `brew install goreleaser`
- **Homebrew Tap Token** configured (required for first release - see setup below)

### One-Time Setup: Homebrew Tap Token

The release workflow needs a Personal Access Token (PAT) to update the Homebrew tap repository:

1. **Create a Personal Access Token:**
   - Go to: https://github.com/settings/tokens/new
   - Token name: `lightfold-homebrew-tap`
   - Expiration: No expiration (or 1 year, then renew)
   - Scopes: ✅ `repo` (Full control of repositories)
   - Click "Generate token" and **copy it**

2. **Add to Repository Secrets:**
   - Go to: https://github.com/theognis1002/lightfold-cli/settings/secrets/actions
   - Click "New repository secret"
   - Name: `HOMEBREW_TAP_TOKEN`
   - Value: Paste your PAT
   - Click "Add secret"

3. **Verify Workflow Configuration:**
   - Check `.github/workflows/release.yml` includes:
     ```yaml
     env:
       GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
       HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
     ```

**Why is this needed?**
- `GITHUB_TOKEN` (auto-provided) can only write to the current repo (`lightfold-cli`)
- `HOMEBREW_TAP_TOKEN` (manual PAT) can write to the separate tap repo (`homebrew-lightfold`)
- Without this, the Homebrew formula won't auto-update on releases

This is only needed once. Future releases will automatically use this token.

## Release Process

### 1. Prepare the Release

1. **Ensure main branch is stable:**
   ```bash
   git checkout main
   git pull origin main
   make test
   ```

2. **Update version references** (if needed):
   - Check `CHANGELOG.md` is up to date (optional)
   - Update any hardcoded version strings

3. **Commit any final changes:**
   ```bash
   git add .
   git commit -m "chore: prepare for release vX.Y.Z"
   git push origin main
   ```

### 2. Create and Push a Tag

```bash
# Create an annotated tag (required for GoReleaser)
git tag -a v0.1.0 -m "Release v0.1.0: Initial public release"

# Push the tag to GitHub
git push origin v0.1.0
```

**Tag Naming Convention:**
- Use semantic versioning: `vMAJOR.MINOR.PATCH`
- Examples: `v0.1.0`, `v1.0.0`, `v1.2.3`
- Pre-releases: `v0.1.0-beta.1`, `v1.0.0-rc.1`

### 3. Automated Release Process

Once the tag is pushed, GitHub Actions will automatically:

1. Trigger the release workflow (`.github/workflows/release.yml`)
2. Build binaries for:
   - Linux (amd64, arm64)
   - macOS (amd64, arm64)
   - Windows (amd64)
3. Generate checksums for all binaries
4. Create a GitHub Release with:
   - Release notes from git commits
   - Downloadable archives for each platform
   - Installation instructions
5. Update Homebrew tap formula at `theognis1002/homebrew-lightfold`

### 4. Verify the Release

1. **Check GitHub Release:**
   - Visit: https://github.com/theognis1002/lightfold-cli/releases
   - Verify all binaries are present
   - Check release notes are correct

2. **Test Homebrew Installation:**
   ```bash
   brew update
   brew install theognis1002/lightfold/lightfold
   lightfold --version
   ```

3. **Test the binary:**
   ```bash
   cd /tmp
   mkdir test-project && cd test-project
   # Create a simple test app
   lightfold detect
   ```

### 5. Announce the Release

- Share on social media, Discord, etc.
- Update documentation if needed
- Notify users of breaking changes

## Testing Releases Locally

Before pushing a tag, you can test the release process locally:

```bash
# Dry run (no publishing)
goreleaser release --snapshot --clean

# Check the dist/ directory for built artifacts
ls -la dist/
```

## Homebrew Tap Setup

The Homebrew tap repository is managed automatically by GoReleaser:

- **Repository:** https://github.com/theognis1002/homebrew-lightfold
- **Formula location:** `Formula/lightfold.rb`
- GoReleaser creates/updates this automatically

**Manual Tap Creation (if needed):**
```bash
# Create the tap repository on GitHub first
# Then GoReleaser will populate it on first release
```

## Troubleshooting

### Release Workflow Fails

1. **Check GitHub Actions logs:**
   - Go to: https://github.com/theognis1002/lightfold-cli/actions
   - Find the failed workflow
   - Review error messages

2. **Common issues:**
   - Missing `GITHUB_TOKEN` (should be automatic)
   - Invalid `.goreleaser.yaml` syntax
   - Build errors (test locally first)

### Homebrew Formula Issues

1. **Formula not updating:**
   - Check if `homebrew-lightfold` repo exists
   - Verify GoReleaser has write permissions
   - Check workflow logs for Homebrew errors

2. **Manual formula update:**
   ```bash
   # Clone the tap
   git clone https://github.com/theognis1002/homebrew-lightfold.git
   cd homebrew-lightfold

   # Edit Formula/lightfold.rb manually
   # Update version, SHA256, etc.

   git add Formula/lightfold.rb
   git commit -m "Update formula for vX.Y.Z"
   git push origin main
   ```

### Binary Installation Fails

1. **Check binary permissions:**
   ```bash
   chmod +x lightfold
   ```

2. **Verify platform compatibility:**
   ```bash
   file lightfold
   uname -m  # Check architecture
   ```

## Rolling Back a Release

If a release has critical issues:

1. **Delete the GitHub release** (UI or API)
2. **Delete the tag:**
   ```bash
   git tag -d v0.1.0
   git push origin :refs/tags/v0.1.0
   ```
3. **Revert Homebrew formula** (if needed):
   - Manually update `homebrew-lightfold` to previous version
4. **Create a new patch release** with the fix

## Version Strategy

- **v0.x.x** - Pre-1.0 releases (API may change)
- **v1.0.0** - First stable release
- **Patch (v1.0.X)** - Bug fixes, no breaking changes
- **Minor (v1.X.0)** - New features, backwards compatible
- **Major (vX.0.0)** - Breaking changes

## Resources

- [GoReleaser Documentation](https://goreleaser.com/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook)
- [Semantic Versioning](https://semver.org/)
