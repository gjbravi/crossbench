# Test Data for crossbench render

This directory contains example files that can be used to test the `crossbench render` command, including testing the automatic function extraction feature.

## Files Overview

### Composite Resources (XR)
- **xr.yaml**: Basic Bucket composite resource

### Compositions
- **composition.yaml**: Single function composition (patch-and-transform)
- **composition-multi-step.yaml**: Multi-step pipeline with multiple functions (comprehensive test)

### Functions
- **functions.yaml**: Single function definition (patch-and-transform)

### Additional Resources
- **observed-resources.yaml**: Example observed resources for testing updates to existing XRs
- **extra-resources.yaml**: Example extra resources that can be passed to the function pipeline

## Testing Scenarios

### 1. Basic Single Function (with explicit functions.yaml)

```bash
crossbench render xr.yaml composition.yaml functions.yaml
```

### 2. Auto-extract Single Function (tests GitHub API)

```bash
crossbench render xr.yaml composition.yaml
```

This will:
- Extract `crossplane-contrib-function-patch-and-transform` from the composition
- Fetch the latest version from GitHub releases API
- Use it to render the composition

### 3. Multi-Step Pipeline with Multiple Functions

```bash
crossbench render xr.yaml composition-multi-step.yaml
```

This comprehensive test:
- Extracts multiple different functions from multiple pipeline steps
- Handles duplicate function references (same function used in multiple steps)
- Fetches latest versions for each unique function from GitHub
- Executes functions in the correct pipeline order

### 4. Render with Observed Resources (simulating an update)

```bash
crossbench render xr.yaml composition.yaml functions.yaml \
  --observed-resources=observed-resources.yaml
```

### 5. Render with Extra Resources

```bash
crossbench render xr.yaml composition.yaml functions.yaml \
  --extra-resources=extra-resources.yaml
```

### 6. Render with Context Values

```bash
crossbench render xr.yaml composition.yaml functions.yaml \
  --context-values=apiextensions.crossplane.io/environment='{"key": "value"}'
```

### 7. Include Function Results in Output

```bash
crossbench render xr.yaml composition.yaml functions.yaml \
  --include-function-results
```

### 8. Include Full XR in Output

```bash
crossbench render xr.yaml composition.yaml functions.yaml \
  --include-full-xr
```

## Testing Auto-Extraction Logic

The following tests verify that the auto-extraction feature works correctly:

### Test 1: Single Function Extraction
```bash
crossbench render xr.yaml composition.yaml
```
**Expected**: Should extract `crossplane-contrib-function-patch-and-transform` and fetch latest version from GitHub.

### Test 2: Multi-Step Pipeline (Multiple Functions)
```bash
crossbench render xr.yaml composition-multi-step.yaml
```
**Expected**: Should extract all 4 unique functions (function-python, function-kcl, function-auto-ready, function-unit-test) from different steps, fetch their latest versions from GitHub, and execute them in sequence. This tests:
- Multiple different functions in one composition (4 functions)
- Sequential execution of pipeline steps
- GitHub API version fetching for multiple functions

### Test 3: Verify Version Fetching
When running auto-extraction, check the INFO messages:
```
INFO: Extracted N function(s) from composition pipeline
INFO: Using function "function-name" with package "xpkg.crossplane.io/owner/repo:vX.Y.Z"
```
**Expected**: Versions should be fetched from GitHub releases API (not `:latest`).

## Prerequisites

- Docker must be running (functions are executed in Docker containers)
- Internet connection for GitHub API calls (when using auto-extraction)
- Function packages will be pulled automatically on first use

## Function Mapping

The auto-extraction feature maps function names to GitHub repositories:

- `crossplane-contrib-function-patch-and-transform` → `crossplane-contrib/function-patch-and-transform`
- `function-python` → `crossplane-contrib/function-python`
- `function-auto-ready` → `crossplane-contrib/function-auto-ready`
- `function-kcl` → `crossplane-contrib/function-kcl`
- `function-unit-test` → `crossplane-contrib/function-unit-test`

## Notes

- Auto-extraction queries GitHub releases API: `https://api.github.com/repos/{owner}/{repo}/releases/latest`
- If GitHub API is unavailable, provide explicit `functions.yaml` files
- Function packages are downloaded from `xpkg.crossplane.io` registry
- Make sure Docker has access to pull images from `xpkg.crossplane.io`
- GitHub API has rate limits; multiple function extractions may hit limits if done frequently

## Troubleshooting

### GitHub API Rate Limits
If you hit rate limits, use explicit `functions.yaml` files instead of auto-extraction.

### Function Not Found
If a function name doesn't map correctly, check the function name pattern and update `mapFunctionNameToGitHubRepo()` in `cmd/functions.go`.

### Network Issues
If GitHub API is unreachable, use explicit `functions.yaml` files with specific versions.
