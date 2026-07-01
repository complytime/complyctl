// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	cacheDir string
}

// NewComplypackCache creates a ComplypackCache rooted at the given base cache
// directory (e.g., ~/.complytime). Complypack artifacts are stored under
// {cacheDir}/complypacks/{evaluator-id}/{version}/.
func NewComplypackCache(cacheDir string) *ComplypackCache {
	return &ComplypackCache{
		cacheDir: cacheDir,
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
	if err := os.MkdirAll(parentDir, 0755); err != nil {
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

	// Evict old version directories for the same evaluator-id.
	// This ensures at most one version exists per evaluator-id after Store().
	evictOldVersions(parentDir, config.Version)

	// Remove any existing final directory before the atomic rename.
	if err := os.RemoveAll(finalDir); err != nil {
		return "", fmt.Errorf("failed to remove existing cache directory %s: %w", finalDir, err)
	}

	// Atomic rename from temp to final path.
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return "", fmt.Errorf("failed to rename temporary directory to %s: %w", finalDir, err)
	}
	cleanup = false // Rename succeeded; don't remove the final directory.

	return filepath.Join(finalDir, complypackContentFile), nil
}

// evictOldVersions removes all version directories under evaluatorDir that are
// not the target version and not hidden (dot-prefixed). Eviction errors are
// non-fatal: a warning is logged but the store operation continues.
func evictOldVersions(evaluatorDir, targetVersion string) {
	entries, err := os.ReadDir(evaluatorDir)
	if err != nil {
		// If the directory doesn't exist yet, nothing to evict.
		if os.IsNotExist(err) {
			return
		}
		log.Warn("failed to list complypack versions for eviction",
			"dir", evaluatorDir, "error", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden/temporary directories (e.g. .complypack-tmp-*).
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Skip the version being stored.
		if name == targetVersion {
			continue
		}
		oldDir := filepath.Join(evaluatorDir, name)
		if removeErr := os.RemoveAll(oldDir); removeErr != nil {
			log.Warn("failed to evict old complypack version",
				"dir", oldDir, "error", removeErr)
		}
	}
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
// config for a given evaluator-id without requiring a specific version. It
// scans the evaluator-id directory under {cacheDir}/complypacks/{evaluatorID}/
// and returns the content path and config for the first version found. This is
// used during the scan pipeline where the exact version is not known — only
// the evaluator-id from the policy graph.
//
// Returns ("", nil, nil) if no cached complypack exists for the evaluator-id,
// allowing callers to treat a missing complypack as a non-error (backward
// compatible).
func (c *ComplypackCache) LookupByEvaluatorID(evaluatorID string) (string, *complypack.Config, error) {
	if err := ValidatePathComponent(evaluatorID); err != nil {
		return "", nil, fmt.Errorf("invalid evaluator-id: %w", err)
	}

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
