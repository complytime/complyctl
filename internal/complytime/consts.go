// SPDX-License-Identifier: Apache-2.0

package complytime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const xdgAppName = "complytime"
const providerSubdir = "providers"

// WorkspaceDir is the workspace-local directory for all complyctl artifacts
// (generation state, scan output). Separate from the user-level XDG cache and data directories.
const WorkspaceDir = ".complytime"

const StateFileName = "state.json"
const PoliciesSubdir = "policies"
const ComplypacksSubdir = "complypacks"
const WorkspaceConfigFile = "complytime.yaml"

const CurrentWorkspaceVersion = 1

const (
	OutputFormatOSCAL  = "oscal"
	OutputFormatPretty = "pretty"
	OutputFormatSARIF  = "sarif"
)

// ShowPassingEnvVar is the environment variable that controls whether
// passing controls appear in the scan summary table. When set, it
// overrides the default (true) unless --show-passing is explicitly
// provided on the command line.
const ShowPassingEnvVar = "COMPLYTIME_SHOW_PASSING"

const ScanOutputDir = "scan"

// LogFileName is the log file name written to {WorkspaceDir}/{LogFileName}.
// See FR-038, R57: specs/001-gemara-native-workflow/research.md
const LogFileName = "complyctl.log"

// DefaultCommandTimeout is the default deadline for scan and generate operations.
// This flows through gRPC to the provider subprocess without additional capping.
const DefaultCommandTimeout = 5 * time.Minute

const ProviderExecutablePrefix = "complyctl-provider-"

// SystemProviderDir is the system-wide provider directory where
// package managers (e.g., RPM) install provider binaries.
// Discovery checks this path as a fallback after the user directory.
const SystemProviderDir = "/usr/libexec/complytime/providers"

// Gemara OCI layer media types for identifying layer content within multi-layer OCI manifests.
const (
	MediaTypeCatalog  = "application/vnd.gemara.catalog.v1+yaml"
	MediaTypeGuidance = "application/vnd.gemara.guidance.v1+yaml"
	MediaTypePolicy   = "application/vnd.gemara.policy.v1+yaml"
)

const OCIEmptyConfig = "application/vnd.oci.empty.v1+json"

// CacheVersionsEnvVar is the environment variable that controls how many
// complypack versions are retained per evaluator-id in the local cache.
// Values less than 1 are treated as 1. Default is 1 (single-version).
const CacheVersionsEnvVar = "COMPLYTIME_CACHE_VERSIONS"

// WorkspaceEnvVar is the environment variable name for workspace directory resolution.
const WorkspaceEnvVar = "COMPLYTIME_WORKSPACE"

// WorkspaceVarKey is the variable key used to inject the resolved workspace
// directory into provider variable maps. Providers receive this as a global
// variable during generation and as a target variable during scan.
const WorkspaceVarKey = "workspace"

// WithWorkspaceVar returns a copy of vars with the workspace key set to the
// resolved workspace directory. User-defined values for the same key are
// overridden so providers always receive the absolute resolved path.
func WithWorkspaceVar(vars map[string]string, workspace string) map[string]string {
	merged := make(map[string]string, len(vars)+1)
	for k, v := range vars {
		merged[k] = v
	}
	merged[WorkspaceVarKey] = workspace
	return merged
}

// Scan result status emoji indicators for terminal summary table (FR-037).
const (
	StatusPassed  = "✅"
	StatusFailed  = "❌"
	StatusSkipped = "⏭️"
	StatusError   = "⚠️"
)

// FilenameSafe replaces characters unsafe for filenames (e.g., path separators)
// so that policy IDs like "policies/nist-800-53-r5" produce flat filenames.
func FilenameSafe(s string) string {
	return strings.ReplaceAll(s, "/", "-")
}

// ExpandPath resolves a leading ~/ to the user's home directory.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// ResolveDataDir returns the absolute path to the user data directory
// following the XDG Base Directory Specification. It checks $XDG_DATA_HOME
// first (ignoring empty or relative values per the spec), then falls back
// to platform-specific defaults: ~/.local/share on Linux,
// ~/Library/Application Support on macOS, %LocalAppData% on Windows.
func ResolveDataDir() (string, error) {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" && filepath.IsAbs(xdgData) {
		return filepath.Join(xdgData, xdgAppName), nil
	}

	// Platform-specific fallback for the data base directory.
	switch runtime.GOOS {
	case "darwin":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve data dir: %w", err)
		}
		return filepath.Join(homeDir, "Library", "Application Support", xdgAppName), nil
	case "windows":
		// Windows does not distinguish cache from data under %LocalAppData%.
		// os.UserCacheDir() returns %LocalAppData%, which serves as the
		// conventional base for both cache and persistent application data.
		base, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("resolve data dir: %w", err)
		}
		return filepath.Join(base, xdgAppName), nil
	default:
		// Linux and other Unix-like systems: $HOME/.local/share
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve data dir: %w", err)
		}
		return filepath.Join(homeDir, ".local", "share", xdgAppName), nil
	}
}

// ResolveCacheDir returns the absolute path to the cache directory
// using os.UserCacheDir (which respects $XDG_CACHE_HOME on Linux,
// ~/Library/Caches on macOS, and %LocalAppData% on Windows).
func ResolveCacheDir() (string, error) {
	cacheBase, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve cache dir: %w", err)
	}
	return filepath.Join(cacheBase, xdgAppName), nil
}

// ResolveProviderDir returns the absolute path to the provider directory
// within the user data directory.
func ResolveProviderDir() (string, error) {
	dataDir, err := ResolveDataDir()
	if err != nil {
		return "", fmt.Errorf("resolve provider dir: %w", err)
	}
	return filepath.Join(dataDir, providerSubdir), nil
}
