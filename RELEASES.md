# Release Guide

This document explains how to create releases for `crossbench` using GoReleaser and make them available via Homebrew.

## Prerequisites

1. **GoReleaser** - Install it locally (optional, for testing):
   ```bash
   brew install goreleaser
   # or
   go install github.com/goreleaser/goreleaser@latest
   ```

2. **GitHub Token** - You'll need a GitHub Personal Access Token (PAT) with the following permissions:
   - `repo` (full control of private repositories)
   - `write:packages` (if publishing packages)

3. **Homebrew Tap Repository** - Create a new GitHub repository named `homebrew-crossbench`:
   ```bash
   # Create the repository on GitHub first, then:
   git clone https://github.com/gjbravi/homebrew-crossbench.git
   cd homebrew-crossbench
   # The repository will be automatically populated by GoReleaser
   ```

## Release Process

### 1. Create a Release Tag

Use semantic versioning (e.g., `v1.0.0`, `v1.1.0`, `v2.0.0`):

```bash
# Make sure you're on the main branch and everything is committed
git checkout main
git pull origin main

# Create and push a new tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

### 2. GitHub Actions Will Automatically:

- Build binaries for Linux (amd64, arm64), macOS (amd64, arm64), and Windows (amd64, arm64)
- Create a GitHub release with the binaries and checksums
- Generate and push a Homebrew formula to the `homebrew-crossbench` tap repository

### 3. Set Up GitHub Secrets

In your GitHub repository settings (`Settings` → `Secrets and variables` → `Actions`), add:

- `HOMEBREW_TAP_TOKEN`: A GitHub Personal Access Token with `repo` permissions for the `homebrew-crossbench` repository

**Note:** The `GITHUB_TOKEN` is automatically provided by GitHub Actions and doesn't need to be set manually.

### 4. Test Locally (Optional)

Before pushing a tag, you can test GoReleaser locally:

```bash
# Dry run (won't create releases or push)
goreleaser release --snapshot --skip-publish --rm-dist

# Or test a full release (requires GITHUB_TOKEN env var)
export GITHUB_TOKEN=your_token_here
goreleaser release --rm-dist
```

## Semantic Versioning

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR** version (v2.0.0): Incompatible API changes
- **MINOR** version (v1.1.0): New functionality in a backwards compatible manner
- **PATCH** version (v1.0.1): Backwards compatible bug fixes

## Homebrew Installation

Once a release is created, users can install `crossbench` via Homebrew:

```bash
brew tap gjbravi/crossbench
brew install crossbench
```

## Troubleshooting

### Release Failed

1. Check GitHub Actions logs for errors
2. Verify `HOMEBREW_TAP_TOKEN` is set correctly
3. Ensure the `homebrew-crossbench` repository exists and is accessible
4. Check that the tag follows semantic versioning format (`v*.*.*`)

### Homebrew Formula Not Updated

1. Verify `HOMEBREW_TAP_TOKEN` has write access to `homebrew-crossbench` repository
2. Check that the repository exists and is not empty (GoReleaser needs to push to it)
3. Review GitHub Actions logs for Homebrew-related errors

### Testing Releases

You can create a pre-release by using a tag with a suffix:

```bash
git tag -a v1.0.0-rc1 -m "Release candidate v1.0.0-rc1"
git push origin v1.0.0-rc1
```

GoReleaser will automatically mark these as pre-releases.

## Manual Release (Alternative)

If you prefer to release manually without GitHub Actions:

```bash
export GITHUB_TOKEN=your_token_here
export HOMEBREW_TAP_TOKEN=your_token_here
goreleaser release --rm-dist
```

