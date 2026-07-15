// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complypack/pkg/complypack"
)

// validPathComponent matches evaluator-id and version values that contain only
// safe characters: alphanumerics, dots, hyphens, and underscores.
var validPathComponent = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

const (
	// complypackContentFile is the filename for cached complypack content.
	complypackContentFile = "content.tar.gz"
	// complypackConfigFile is the filename for cached complypack configuration.
	complypackConfigFile = "config.json"
)

// ComplypackCache manages cached complypack artifacts under
// {cacheDir}/complypacks/{evaluator-id}/{version}/.
type ComplypackCache struct {
	cacheDir       string
	state          *State
	retentionCount int
}

// NewComplypackCache creates a ComplypackCache rooted at the given base cache
// directory (e.g., ~/.cache/complytime). The optional state parameter enables
// state-driven lookup and retention-aware eviction. When state is nil,
// both features fall back to filesystem-only behavior (D4).
//
// The retention count is read from COMPLYTIME_CACHE_VERSIONS once at
// construction time to avoid repeated os.Getenv calls and ensure
// deterministic behavior during the cache's lifetime.
func NewComplypackCache(cacheDir string, state *State) *ComplypackCache {
	return &ComplypackCache{
		cacheDir:       cacheDir,
		state:          state,
		retentionCount: CacheRetentionCount(),
	}
}

// ValidatePathComponent rejects evaluator-id and version values that would be
// unsafe as directory names. It rejects empty strings, path separators (/ and \),
// parent directory references (..), null bytes, and any character outside
// [a-zA-Z0-9._-].
func ValidatePathComponent(value string) error {
	if value == "" {
		return fmt.Errorf("path component must not be empty")
	}
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("path component must not contain null bytes: %q", value)
	}
	if strings.Contains(value, "/") || strings.Contains(value, `\`) {
		return fmt.Errorf("path component must not contain path separators: %q", value)
	}
	if strings.Contains(value, "..") {
		return fmt.Errorf("path component must not contain parent directory reference: %q", value)
	}
	if !validPathComponent.MatchString(value) {
		return fmt.Errorf("path component contains invalid characters (allowed: a-zA-Z0-9._-): %q", value)
	}
	return nil
}

// complypackDir returns the path to a specific complypack's cache directory:
// {cacheDir}/complypacks/{evaluatorID}/{version}/
func (c *ComplypackCache) complypackDir(evaluatorID, version string) string {
	return filepath.Join(c.cacheDir, complytime.ComplypacksSubdir, evaluatorID, version)
}

// Store writes a complypack's content and configuration to the cache using
// atomic rename. It validates evaluator-id and version via ValidatePathComponent,
// writes content.tar.gz and config.json to a temporary directory first, then
// atomically renames to the final cache path. Returns the path to content.tar.gz.
func (c *ComplypackCache) Store(config complypack.Config, content io.Reader) (string, error) {
	if err := ValidatePathComponent(config.EvaluatorID); err != nil {
		return "", fmt.Errorf("invalid evaluator-id: %w", err)
	}
	if err := ValidatePathComponent(config.Version); err != nil {
		return "", fmt.Errorf("invalid version: %w", err)
	}

	finalDir := c.complypackDir(config.EvaluatorID, config.Version)
	parentDir := filepath.Dir(finalDir)

	// Ensure the parent directory exists for both the temp dir and the final path.
	if err := os.MkdirAll(parentDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create complypack parent directory %s: %w", parentDir, err)
	}

	// Create a temporary directory under the parent so os.Rename is atomic
	// (same filesystem).
	tmpDir, err := os.MkdirTemp(parentDir, ".complypack-tmp-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	// Clean up the temp dir on any error path.
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(tmpDir)
		}
	}()

	// Write content.tar.gz
	contentPath := filepath.Join(tmpDir, complypackContentFile)
	contentFile, err := os.OpenFile(contentPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create content file: %w", err)
	}
	if _, err := io.Copy(contentFile, content); err != nil {
		contentFile.Close()
		return "", fmt.Errorf("failed to write content file: %w", err)
	}
	if err := contentFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close content file: %w", err)
	}

	// Write config.json
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal complypack config: %w", err)
	}
	configPath := filepath.Join(tmpDir, complypackConfigFile)
	if err := os.WriteFile(configPath, configData, 0600); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	// Remove any existing final directory before the atomic rename.
	if err := os.RemoveAll(finalDir); err != nil {
		return "", fmt.Errorf("failed to remove existing cache directory %s: %w", finalDir, err)
	}

	// Atomic rename from temp to final path.
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return "", fmt.Errorf("failed to rename temporary directory to %s: %w", finalDir, err)
	}
	cleanup = false // Rename succeeded; don't remove the final directory.

	// Evict old version directories for the same evaluator-id, keeping
	// up to N versions per the configured retention count (FR-001).
	// Eviction runs after the successful Rename to prevent data loss:
	// if Rename fails, old versions are preserved and the temp dir is
	// cleaned up by the deferred cleanup.
	evictOldVersions(parentDir, config.Version, c.retentionCount, c.state)

	return filepath.Join(finalDir, complypackContentFile), nil
}

// evictOldVersions removes version directories under evaluatorDir to enforce
// the retention count. When state is non-nil, orphaned directories (not
// tracked in state for any evaluator) are evicted first, then the oldest
// versions by LastUpdated timestamp, keeping up to retentionCount versions
// (including the target version). When state is nil, all versions except
// the target are removed (preserving current single-version behavior).
//
// evaluatorDir must be a path within the complypack cache root
// (e.g. {cacheDir}/complypacks/{evaluator-id}). Callers are responsible
// for validating the path before calling this function.
func evictOldVersions(
	evaluatorDir, targetVersion string,
	retentionCount int,
	state *State,
) {
	entries, err := os.ReadDir(evaluatorDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Warn("failed to list complypack versions for eviction",
			"dir", evaluatorDir, "error", err)
		return
	}

	// Collect non-hidden, non-target version directories.
	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if name == targetVersion {
			continue
		}
		candidates = append(candidates, name)
	}

	// When state is nil, fall back to current behavior: remove all
	// except the target version (FR-002 default).
	if state == nil {
		for _, name := range candidates {
			oldDir := filepath.Join(evaluatorDir, name)
			if removeErr := os.RemoveAll(oldDir); removeErr != nil {
				log.Warn("failed to evict old complypack version",
					"dir", oldDir, "error", removeErr)
			}
		}
		return
	}

	// Build a set of versions tracked in state for this evaluator-id.
	// State is keyed by repository, so we iterate all complypack entries
	// and match by evaluator-id to scope eviction correctly (FR-008).
	evaluatorID := filepath.Base(evaluatorDir)
	trackedVersions := buildTrackedVersionSet(state, evaluatorID)

	// Partition candidates into orphaned (not in state) and tracked.
	var orphaned, tracked []string
	for _, name := range candidates {
		if _, ok := trackedVersions[name]; ok {
			tracked = append(tracked, name)
		} else {
			orphaned = append(orphaned, name)
		}
	}

	// Evict orphaned directories first (D5: least valuable).
	for _, name := range orphaned {
		oldDir := filepath.Join(evaluatorDir, name)
		if removeErr := os.RemoveAll(oldDir); removeErr != nil {
			log.Warn("failed to evict orphaned complypack version",
				"dir", oldDir, "error", removeErr)
		}
	}

	// Count how many versions we can keep (target is always kept).
	// retentionCount includes the target version.
	slotsForOld := retentionCount - 1
	if slotsForOld < 0 {
		slotsForOld = 0
	}

	if len(tracked) <= slotsForOld {
		return
	}

	// Sort tracked versions by LastUpdated ascending (oldest first)
	// so we evict the oldest ones beyond the retention limit.
	sortByLastUpdated(tracked, state, evaluatorID)

	evictCount := len(tracked) - slotsForOld
	for i := 0; i < evictCount; i++ {
		oldDir := filepath.Join(evaluatorDir, tracked[i])
		if removeErr := os.RemoveAll(oldDir); removeErr != nil {
			log.Warn("failed to evict old complypack version",
				"dir", oldDir, "error", removeErr)
		}
	}
}

// buildTrackedVersionSet returns a set of version strings that are tracked
// in state.json for a specific evaluator-id. Scoping by evaluator-id
// prevents version strings from other evaluators from being incorrectly
// classified as tracked (FR-008 cross-evaluator isolation).
func buildTrackedVersionSet(state *State, evaluatorID string) map[string]bool {
	versions := make(map[string]bool)
	if state == nil || state.Complypacks == nil {
		return versions
	}
	for _, ps := range state.Complypacks {
		if ps.EvaluatorID == evaluatorID && ps.Version != "" {
			versions[ps.Version] = true
		}
	}
	return versions
}

// sortByLastUpdated sorts version names by their LastUpdated timestamp
// from state, ascending (oldest first). Versions not found in state are
// sorted to the front (evicted first). Only considers state entries
// matching the given evaluator-id for correct cross-evaluator isolation.
func sortByLastUpdated(versions []string, state *State, evaluatorID string) {
	// Build version → timestamp map from state complypack entries.
	timestamps := make(map[string]time.Time)
	if state != nil && state.Complypacks != nil {
		for _, ps := range state.Complypacks {
			if ps.EvaluatorID == evaluatorID && ps.Version != "" {
				timestamps[ps.Version] = ps.LastUpdated
			}
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		ti := timestamps[versions[i]]
		tj := timestamps[versions[j]]
		return ti.Before(tj)
	})
}

// Lookup finds the cached complypack content path and config for a specific
// evaluator-id and version. Returns the path to content.tar.gz and the parsed
// config. Returns an error wrapping os.ErrNotExist if the cache entry does not
// exist.
func (c *ComplypackCache) Lookup(evaluatorID, version string) (string, *complypack.Config, error) {
	if err := ValidatePathComponent(evaluatorID); err != nil {
		return "", nil, fmt.Errorf("invalid evaluator-id: %w", err)
	}
	if err := ValidatePathComponent(version); err != nil {
		return "", nil, fmt.Errorf("invalid version: %w", err)
	}

	dir := c.complypackDir(evaluatorID, version)

	// Check that the content file exists.
	contentPath := filepath.Join(dir, complypackContentFile)
	if _, err := os.Stat(contentPath); err != nil {
		return "", nil, fmt.Errorf("complypack content not found for %s@%s: %w", evaluatorID, version, err)
	}

	// Read and parse config.json.
	configPath := filepath.Join(dir, complypackConfigFile)
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read complypack config for %s@%s: %w", evaluatorID, version, err)
	}

	var cfg complypack.Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return "", nil, fmt.Errorf("failed to parse complypack config for %s@%s: %w", evaluatorID, version, err)
	}

	return contentPath, &cfg, nil
}

// LookupByEvaluatorID finds the cached complypack content path and parsed
// config for a given evaluator-id without requiring a specific version.
//
// When state is available (injected via NewComplypackCache), the active
// version is resolved from state.json via EvaluatorIDToVersion and
// delegated to Lookup. This is deterministic and avoids directory scanning
// (FR-005). If the state-based lookup fails, it falls back to directory
// scanning for backward compatibility.
//
// When state is nil, it scans the evaluator-id directory under
// {cacheDir}/complypacks/{evaluatorID}/ and returns the content path and
// config for the first version found.
//
// Returns ("", nil, nil) if no cached complypack exists for the evaluator-id,
// allowing callers to treat a missing complypack as a non-error (backward
// compatible).
func (c *ComplypackCache) LookupByEvaluatorID(evaluatorID string) (string, *complypack.Config, error) {
	if err := ValidatePathComponent(evaluatorID); err != nil {
		return "", nil, fmt.Errorf("invalid evaluator-id: %w", err)
	}

	// State-driven lookup: resolve the active version from state and
	// delegate to Lookup for deterministic resolution (D4, FR-005).
	if c.state != nil {
		version, ok, lookupErr := c.state.EvaluatorIDToVersion(evaluatorID)
		if lookupErr != nil {
			return "", nil, fmt.Errorf(
				"state lookup failed for evaluator %s: %w",
				evaluatorID, lookupErr,
			)
		}
		if ok {
			contentPath, cfg, err := c.Lookup(evaluatorID, version)
			if err == nil && contentPath != "" {
				return contentPath, cfg, nil
			}
			// State references a version that is not on disk — fall
			// through to directory scan for backward compatibility.
		}
	}

	// Fallback: directory scan (original behavior when state is nil or
	// state-based lookup fails).
	return c.lookupByDirScan(evaluatorID)
}

// lookupByDirScan scans the evaluator-id directory for the first valid
// version directory containing content.tar.gz and config.json. This is
// the original LookupByEvaluatorID behavior, extracted for use as a
// fallback when state-driven lookup is unavailable or fails.
func (c *ComplypackCache) lookupByDirScan(evaluatorID string) (string, *complypack.Config, error) {
	evalDir := filepath.Join(c.cacheDir, complytime.ComplypacksSubdir, evaluatorID)
	entries, err := os.ReadDir(evalDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, nil
		}
		return "", nil, fmt.Errorf("failed to read complypack cache directory for %s: %w", evaluatorID, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden/temporary directories left by atomic writes.
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		contentPath := filepath.Join(evalDir, entry.Name(), complypackContentFile)
		if _, statErr := os.Stat(contentPath); statErr == nil {
			// Read and parse config.json from the same directory.
			configPath := filepath.Join(evalDir, entry.Name(), complypackConfigFile)
			configData, readErr := os.ReadFile(configPath)
			if readErr != nil {
				return "", nil, fmt.Errorf("failed to read complypack config for %s: %w", evaluatorID, readErr)
			}
			var cfg complypack.Config
			if jsonErr := json.Unmarshal(configData, &cfg); jsonErr != nil {
				return "", nil, fmt.Errorf("failed to parse complypack config for %s: %w", evaluatorID, jsonErr)
			}
			return contentPath, &cfg, nil
		}
	}

	return "", nil, nil
}

// Dir returns the base cache directory.
func (c *ComplypackCache) Dir() string {
	return c.cacheDir
}
