# crossbench

A friendly CLI tool for testing and rendering Crossplane compositions locally. Think of it as your local playground for Crossplane - you can test your compositions without needing a running cluster or dealing with complex setups.

`crossbench` uses the same battle-tested code from Crossplane's official `render` command, so you get identical behavior and results. Perfect for CI/CD pipelines, local development, or debugging compositions before deploying them.

## What this actually differs from actual `crossplane render`?

- **Auto-discover functions** - It automatically figures out which functions your composition needs and fetches the right pkg versions from crossplane AND/OR upbound repositories.
- More to come...

## Getting Started

### Install via Homebrew

Install `crossbench` using Homebrew:

```bash
brew tap gjbravi/crossbench https://github.com/gjbravi/crossbench.git
brew install crossbench
```

Or install directly:

```bash
brew install gjbravi/crossbench/crossbench
```

### Install via Go

You can also install directly using Go:

```bash
go install github.com/gjbravi/crossbench@latest
```

This will install `crossbench` to your `$GOPATH/bin` (or `~/go/bin` by default). Make sure that's in your PATH!

### Build from Source

If you prefer to build it yourself:

```bash
git clone https://github.com/gjbravi/crossbench.git
cd crossbench
go build -o crossbench .
```

Then move the `crossbench` binary somewhere in your PATH, or use it directly with `./crossbench`.

## Configuration (Optional)

Out of the box, `crossbench` works great with sensible defaults. But if you want to customize things (like cache behavior, package registries, or GitHub settings), you can use environment variables.

### Quick Setup

If you want to customize anything, copy the example config file:

```bash
cp .env.example .env
# Then edit .env with your preferences
```

### What Can You Configure?

**Cache Settings**:
- `CROSSBENCH_CACHE_EXPIRATION` - How long cached versions stay valid (default: `24h`)
- `CROSSBENCH_CACHE_DIR` - Where to store the cache (default: `~/.crossbench`)
- `CROSSBENCH_CACHE_FILENAME` - Cache filename (default: `function-versions.json`)

**GitHub API Settings**:
- `CROSSBENCH_GITHUB_API_URL` - GitHub API URL (default: `https://api.github.com`)
- `CROSSBENCH_GITHUB_API_TIMEOUT` - Request timeout (default: `10s`)
- `CROSSBENCH_GITHUB_TOKEN` - Your GitHub token (optional - it'll use `gh auth token` automatically if you're logged in)

**Package Registry Settings**:
- `CROSSBENCH_DEFAULT_GITHUB_OWNER` - Default GitHub org (default: `crossplane-contrib`)
- `CROSSBENCH_DEFAULT_PACKAGE_REGISTRY` - Where functions are published (default: `xpkg.crossplane.io`)
- `CROSSBENCH_UPBOUND_PACKAGE_REGISTRY` - Upbound registry URL (default: `xpkg.upbound.io`)
- `CROSSBENCH_UPBOUND_FUNCTIONS` - Functions using Upbound registry (default: `function-unit-test`)

Check out `.env.example` for all the details and examples!

## Usage

### The Basics

At its core, `crossbench` needs two things: your composite resource (XR) and your composition:

```bash
crossbench render <composite-resource> <composition> [functions]
```

The `functions` argument is **optional** - if you don't provide it, `crossbench` will automatically:
1. Look at your composition's pipeline
2. Figure out which functions you're using
3. Fetch the latest versions from GitHub
4. Use them to render your composition

Pretty neat, right? You can also provide a `functions.yaml` file if you want specific versions or need to work offline.

### Try It Out

We've included some example files to get you started. Try this:

```bash
cd testdata

# Let crossbench figure out the functions automatically
crossbench render xr.yaml composition.yaml

# Or be explicit about which functions to use
crossbench render xr.yaml composition.yaml functions.yaml
```

Both commands do the same thing, but the first one is easier - `crossbench` handles the function discovery for you!

### Common Use Cases

**Just render a composition:**
```bash
crossbench render xr.yaml composition.yaml
```

**Test with existing resources** (simulate an update scenario):
```bash
crossbench render xr.yaml composition.yaml \
  --observed-resources=existing-resources.yaml
```

**Pass environment context to functions:**
```bash
crossbench render xr.yaml composition.yaml \
  --context-values=apiextensions.crossplane.io/environment='{"env": "production"}'
```

**Include extra resources** that functions might need:
```bash
crossbench render xr.yaml composition.yaml \
  --extra-resources=extra-resources.yaml
```

**Provide credentials** for functions that need them:
```bash
crossbench render xr.yaml composition.yaml \
  --function-credentials=credentials.yaml
```

**Force refresh** cached function versions:
```bash
crossbench render xr.yaml composition.yaml --refresh-cache
```

**Pro tip:** Run `crossbench render --help` to see all options with descriptions!

## Smart Caching (How We Avoid Rate Limits)

Nobody likes hitting API rate limits. That's why `crossbench` caches function versions.

### How It Works

**Simple and straightforward:**
- **First time**: Fetches from GitHub and saves to cache
- **Within 24 hours**: Uses cached versions
- **After 24 hours**: Fetches fresh version from GitHub and updates cache
- **If rate limited**: Falls back to cached version (even if expired) so you can keep working

That's it! One check per day per function.

### Cache Management

**Force refresh everything:**
```bash
crossbench render xr.yaml composition.yaml --refresh-cache
```

**Clear cache manually:**
```bash
rm ~/.crossbench/function-versions.json
```

**Customize cache location:**
Set `CROSSBENCH_CACHE_DIR` in your `.env` file to store cache somewhere else.

**Change cache expiration:**
Set `CROSSBENCH_CACHE_EXPIRATION` in your `.env` file (default: `24h`).

## License

This project uses packages from the Crossplane project, which is licensed under the Apache License 2.0.
