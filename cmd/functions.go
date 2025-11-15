package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
	pkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	apiextensionsv1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
)

// FunctionVersionCache represents the cache structure for function versions
type FunctionVersionCache struct {
	Versions map[string]CacheEntry `json:"versions"`
}

// CacheEntry represents a cached version entry with timestamp
type CacheEntry struct {
	Version   string    `json:"version"`
	FetchedAt time.Time `json:"fetched_at"`
}

// getCacheExpiration returns how long cached versions are considered valid
// Default: 24 hours, configurable via CROSSBENCH_CACHE_EXPIRATION env var
// Cache is checked once per day - if cache is valid, use it; otherwise fetch from GitHub
func getCacheExpiration() time.Duration {
	if val := os.Getenv("CROSSBENCH_CACHE_EXPIRATION"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return 24 * time.Hour
}

// getCachePath returns the path to the cache file
func getCachePath(fs afero.Fs) (string, error) {
	// Get cache directory from env or use default
	cacheDirName := os.Getenv("CROSSBENCH_CACHE_DIR")
	if cacheDirName == "" {
		cacheDirName = ".crossbench"
	}
	
	// Get cache filename from env or use default
	cacheFileName := os.Getenv("CROSSBENCH_CACHE_FILENAME")
	if cacheFileName == "" {
		cacheFileName = "function-versions.json"
	}
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		return ".crossbench-cache.json", nil
	}
	
	// If cacheDirName is absolute, use it directly; otherwise join with homeDir
	var cacheDir string
	if filepath.IsAbs(cacheDirName) {
		cacheDir = cacheDirName
	} else {
		cacheDir = filepath.Join(homeDir, cacheDirName)
	}
	
	// Ensure cache directory exists
	if err := fs.MkdirAll(cacheDir, 0755); err != nil {
		return ".crossbench-cache.json", nil
	}
	return filepath.Join(cacheDir, cacheFileName), nil
}

// loadCache loads the function version cache from disk
func loadCache(fs afero.Fs) (*FunctionVersionCache, error) {
	cachePath, err := getCachePath(fs)
	if err != nil {
		return &FunctionVersionCache{Versions: make(map[string]CacheEntry)}, nil
	}

	data, err := afero.ReadFile(fs, cachePath)
	if err != nil {
		// Cache file doesn't exist yet, return empty cache
		if os.IsNotExist(err) {
			return &FunctionVersionCache{Versions: make(map[string]CacheEntry)}, nil
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache FunctionVersionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		// Invalid cache file, return empty cache
		return &FunctionVersionCache{Versions: make(map[string]CacheEntry)}, nil
	}

	if cache.Versions == nil {
		cache.Versions = make(map[string]CacheEntry)
	}

	return &cache, nil
}

// saveCache saves the function version cache to disk
func saveCache(fs afero.Fs, cache *FunctionVersionCache) error {
	cachePath, err := getCachePath(fs)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := afero.WriteFile(fs, cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// getCachedVersion retrieves a cached version if it exists and is still valid.
// Returns the version and whether it was found in cache.
func getCachedVersion(cache *FunctionVersionCache, cacheKey string) (version string, found bool) {
	entry, exists := cache.Versions[cacheKey]
	if !exists {
		return "", false
	}

	age := time.Since(entry.FetchedAt)

	// Check if cache entry has expired (older than expiration time)
	if age > getCacheExpiration() {
		return "", false
	}

	return entry.Version, true
}

// setCachedVersion stores a version in the cache
func setCachedVersion(cache *FunctionVersionCache, cacheKey, version string) {
	cache.Versions[cacheKey] = CacheEntry{
		Version:   version,
		FetchedAt: time.Now(),
	}
}

// ExtractFunctionsFromComposition extracts function references from a Composition's pipeline
// and creates Function resources for them. It fetches the latest version from GitHub releases.
// If forceRefresh is true, it will bypass cache and fetch fresh versions.
func ExtractFunctionsFromComposition(comp *apiextensionsv1.Composition, fs afero.Fs, forceRefresh bool) ([]pkgv1.Function, error) {
	if comp.Spec.Mode != apiextensionsv1.CompositionModePipeline {
		return nil, fmt.Errorf("composition must use Pipeline mode to extract functions")
	}

	if comp.Spec.Pipeline == nil || len(comp.Spec.Pipeline) == 0 {
		return nil, fmt.Errorf("composition pipeline is empty")
	}

	// Map to track unique function names
	functionMap := make(map[string]bool)
	var functions []pkgv1.Function

	// Create a context with timeout for GitHub API calls
	timeout := getGitHubAPITimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, step := range comp.Spec.Pipeline {
		functionName := step.FunctionRef.Name
		if functionName == "" {
			continue
		}

		// Skip if we've already added this function
		if functionMap[functionName] {
			continue
		}

		functionMap[functionName] = true

		// Create a Function resource from the function reference
		// Fetch the latest version from GitHub releases (with caching)
		packageName, err := inferPackageFromFunctionName(ctx, functionName, fs, forceRefresh)
		if err != nil {
			return nil, fmt.Errorf("cannot determine package for function %q: %w", functionName, err)
		}

		fn := pkgv1.Function{
			ObjectMeta: comp.ObjectMeta,
		}
		fn.SetName(functionName)
		fn.Spec.Package = packageName

		functions = append(functions, fn)
	}

	if len(functions) == 0 {
		return nil, fmt.Errorf("no function references found in composition pipeline")
	}

	return functions, nil
}

// inferPackageFromFunctionName attempts to infer the package name from a function name
// and fetches the latest version from GitHub releases (with caching).
// Common patterns:
// - crossplane-contrib-function-patch-and-transform -> xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform:v0.9.2
// - function-name -> xpkg.crossplane.io/crossplane-contrib/function-name:vX.Y.Z
func inferPackageFromFunctionName(ctx context.Context, name string, fs afero.Fs, forceRefresh bool) (string, error) {
	// Common prefix patterns
	if len(name) == 0 {
		return "", fmt.Errorf("function name cannot be empty")
	}

	// If it already looks like a package reference, return as-is
	if strings.Contains(name, "/") || strings.Contains(name, ":") {
		return name, nil
	}

	// Map function name to GitHub repository
	owner, repo, err := mapFunctionNameToGitHubRepo(name)
	if err != nil {
		return "", fmt.Errorf("cannot map function name to GitHub repository: %w", err)
	}

	// Create cache key from owner/repo
	cacheKey := fmt.Sprintf("%s/%s", owner, repo)

	// Load cache
	cache, err := loadCache(fs)
	if err != nil {
		// If cache loading fails, continue without cache
		cache = &FunctionVersionCache{Versions: make(map[string]CacheEntry)}
	}

	// Get cache file path for logging
	cachePath, _ := getCachePath(fs)
	
	// Check cache first (unless force refresh is requested)
	version, found := getCachedVersion(cache, cacheKey)
	if found && !forceRefresh {
		// Cache hit - use cached version immediately
		_, _ = fmt.Fprintf(os.Stderr, "INFO: Using cached function version %s:%s from %s\n", cacheKey, version, cachePath)
	} else {
		// Cache miss or expired - fetch latest release version from GitHub
		version, err = fetchLatestReleaseVersion(ctx, owner, repo)
		if err != nil {
			// If rate limited, try to use stale cache if available
			if rateLimitErr, ok := err.(*RateLimitError); ok {
				// Check if we have any cached version (even if expired)
				if staleEntry, hasStale := cache.Versions[cacheKey]; hasStale {
					// Use stale cache as fallback when rate limited
					version = staleEntry.Version
					_, _ = fmt.Fprintf(os.Stderr, "WARN: Received rate limit from GitHub for %s, falling back to cached version %s from %s\n", cacheKey, version, cachePath)
					// Don't update cache timestamp, keep it as stale
				} else {
					return "", fmt.Errorf("cannot fetch latest version for %s/%s: %w (no cached version available)", owner, repo, rateLimitErr)
				}
			} else {
				return "", fmt.Errorf("cannot fetch latest version for %s/%s: %w", owner, repo, err)
			}
		} else {
			// Store in cache only if fetch succeeded
			setCachedVersion(cache, cacheKey, version)
			if err := saveCache(fs, cache); err != nil {
				// Log but don't fail if cache save fails
				_, _ = fmt.Fprintf(os.Stderr, "WARN: Failed to save cache to %s: %v\n", cachePath, err)
			}
		}
	}

	// Determine the package registry based on function name
	registry := getPackageRegistry(name)

	// Construct the package reference
	packageName := fmt.Sprintf("%s/%s/%s:%s", registry, owner, repo, version)
	return packageName, nil
}

// getDefaultGitHubOwner returns the default GitHub owner/organization for functions
// Default: crossplane-contrib, configurable via CROSSBENCH_DEFAULT_GITHUB_OWNER env var
func getDefaultGitHubOwner() string {
	if owner := os.Getenv("CROSSBENCH_DEFAULT_GITHUB_OWNER"); owner != "" {
		return owner
	}
	return "crossplane-contrib"
}

// mapFunctionNameToGitHubRepo maps a function name to its GitHub repository owner and name.
// For crossplane-contrib functions, the pattern is:
// - crossplane-contrib-function-patch-and-transform -> crossplane-contrib, function-patch-and-transform
// - function-name -> crossplane-contrib, function-name
func mapFunctionNameToGitHubRepo(functionName string) (owner, repo string, err error) {
	// Default owner for crossplane-contrib functions
	owner = getDefaultGitHubOwner()

	// Remove common prefixes to get the base function name
	repo = functionName
	if strings.HasPrefix(repo, "crossplane-contrib-function-") {
		repo = strings.TrimPrefix(repo, "crossplane-contrib-function-")
		repo = fmt.Sprintf("function-%s", repo)
	} else if strings.HasPrefix(repo, "function-") {
		// Already has function- prefix, use as-is
	} else {
		// No prefix, assume it needs the function- prefix
		repo = fmt.Sprintf("function-%s", repo)
	}

	return owner, repo, nil
}

// getDefaultPackageRegistry returns the default package registry URL
// Default: xpkg.crossplane.io, configurable via CROSSBENCH_DEFAULT_PACKAGE_REGISTRY env var
func getDefaultPackageRegistry() string {
	if registry := os.Getenv("CROSSBENCH_DEFAULT_PACKAGE_REGISTRY"); registry != "" {
		return registry
	}
	return "xpkg.crossplane.io"
}

// getUpboundPackageRegistry returns the Upbound package registry URL
// Default: xpkg.upbound.io, configurable via CROSSBENCH_UPBOUND_PACKAGE_REGISTRY env var
func getUpboundPackageRegistry() string {
	if registry := os.Getenv("CROSSBENCH_UPBOUND_PACKAGE_REGISTRY"); registry != "" {
		return registry
	}
	return "xpkg.upbound.io"
}

// getUpboundFunctionNames returns a comma-separated list of function names that use Upbound registry
// Default: function-unit-test, configurable via CROSSBENCH_UPBOUND_FUNCTIONS env var
func getUpboundFunctionNames() []string {
	if functions := os.Getenv("CROSSBENCH_UPBOUND_FUNCTIONS"); functions != "" {
		return strings.Split(functions, ",")
	}
	return []string{"function-unit-test"}
}

// getPackageRegistry returns the appropriate package registry for a given function name.
// Most functions use the default registry, but some use Upbound registry
func getPackageRegistry(functionName string) string {
	// Check if this function should use Upbound registry
	upboundFunctions := getUpboundFunctionNames()
	for _, fn := range upboundFunctions {
		if strings.TrimSpace(fn) == functionName {
			return getUpboundPackageRegistry()
		}
	}
	// Default to crossplane registry
	return getDefaultPackageRegistry()
}

// RateLimitError represents a GitHub API rate limit error
type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string {
	return e.Message
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

// getGitHubAPITimeout returns the GitHub API request timeout
// Default: 10 seconds, configurable via CROSSBENCH_GITHUB_API_TIMEOUT env var
func getGitHubAPITimeout() time.Duration {
	if val := os.Getenv("CROSSBENCH_GITHUB_API_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return 10 * time.Second
}

// getGitHubAPIURL returns the GitHub API base URL
// Default: https://api.github.com, configurable via CROSSBENCH_GITHUB_API_URL env var
func getGitHubAPIURL() string {
	if url := os.Getenv("CROSSBENCH_GITHUB_API_URL"); url != "" {
		return url
	}
	return "https://api.github.com"
}

// getGitHubToken retrieves a GitHub token from environment or gh CLI
func getGitHubToken() string {
	// Check CROSSBENCH_GITHUB_TOKEN first (project-specific)
	if token := os.Getenv("CROSSBENCH_GITHUB_TOKEN"); token != "" {
		return token
	}
	
	// Check standard GITHUB_TOKEN environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	
	// Try to get token from gh CLI
	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		// Remove trailing newline
		token := strings.TrimSpace(string(output))
		if token != "" {
			return token
		}
	}
	
	return ""
}

// fetchLatestReleaseVersion fetches the latest release version from GitHub API.
func fetchLatestReleaseVersion(ctx context.Context, owner, repo string) (string, error) {
	baseURL := getGitHubAPIURL()
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", baseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for GitHub API
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	// Add authentication token if available
	if token := getGitHubToken(); token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	client := &http.Client{
		Timeout: getGitHubAPITimeout(),
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		// Check if this is a rate limit error
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "rate limit") || strings.Contains(string(body), "API rate limit") {
			return "", &RateLimitError{Message: fmt.Sprintf("GitHub API rate limit exceeded: %s", string(body))}
		}
		return "", fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no tag name found in release")
	}

	// Return the tag name as-is (e.g., v0.9.2)
	// Package references use the tag name directly
	return release.TagName, nil
}

