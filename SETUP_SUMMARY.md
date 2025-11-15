# Homebrew Release Setup Summary

## ✅ What Was Set Up

### 1. Version Command
- Added `crossbench version` command that displays:
  - Version number
  - Git commit hash
  - Build date
  - Go version
  - OS/Architecture

### 2. GoReleaser Configuration (`.goreleaser.yml`)
- Builds binaries for:
  - Linux (amd64, arm64)
  - macOS (darwin) (amd64, arm64)
  - Windows (amd64, arm64)
- Creates GitHub releases automatically
- Generates checksums for all binaries
- Configures Homebrew tap integration

### 3. GitHub Actions Workflows
- **`.github/workflows/release.yml`**: Automatically runs on semantic version tags (`v*.*.*`)
  - Builds all binaries
  - Creates GitHub release
  - Updates Homebrew formula
- **`.github/workflows/test.yml`**: Runs tests on push/PR

### 4. Documentation
- Updated `README.md` with Homebrew installation instructions
- Created `RELEASES.md` with detailed release process guide
- Added Apache 2.0 LICENSE file

## 🚀 Next Steps

### 1. Create Homebrew Tap Repository
```bash
# On GitHub, create a new repository named: homebrew-crossbench
# It can be empty - GoReleaser will populate it
```

### 2. Set Up GitHub Secrets
In your repository settings (`Settings` → `Secrets and variables` → `Actions`):
- Add `HOMEBREW_TAP_TOKEN` with a GitHub PAT that has `repo` permissions for `homebrew-crossbench`

### 3. Create Your First Release
```bash
# Make sure everything is committed
git add .
git commit -m "Prepare for v1.0.0 release"

# Create and push a semantic version tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

GitHub Actions will automatically:
1. Build all binaries
2. Create a GitHub release
3. Update the Homebrew formula

### 4. Users Can Then Install
```bash
brew tap gjbravi/crossbench
brew install crossbench
```

## 📝 Notes

- **GoReleaser is highly recommended** - It's the industry standard for Go CLI releases
- **Semantic versioning** is enforced via tag pattern (`v*.*.*`)
- **Homebrew tap** must exist before the first release
- The `HOMEBREW_TAP_TOKEN` is only needed if you want automatic Homebrew updates

## 🔍 Testing Locally

Before your first release, you can test GoReleaser:

```bash
# Install GoReleaser
brew install goreleaser

# Dry run (won't publish anything)
goreleaser release --snapshot --skip-publish --rm-dist
```

This will create a `dist/` directory with all the artifacts that would be published.
