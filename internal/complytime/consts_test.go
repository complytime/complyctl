// SPDX-License-Identifier: Apache-2.0

package complytime

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithWorkspaceVar_NilMap(t *testing.T) {
	result := WithWorkspaceVar(nil, "/home/user/project")
	assert.Equal(t, map[string]string{WorkspaceVarKey: "/home/user/project"}, result)
}

func TestWithWorkspaceVar_EmptyMap(t *testing.T) {
	result := WithWorkspaceVar(map[string]string{}, "/home/user/project")
	assert.Equal(t, map[string]string{WorkspaceVarKey: "/home/user/project"}, result)
}

func TestWithWorkspaceVar_PreservesExistingVars(t *testing.T) {
	input := map[string]string{"output_dir": "/tmp", "scan_target": "host"}
	result := WithWorkspaceVar(input, "/workspace")

	assert.Equal(t, "/workspace", result[WorkspaceVarKey])
	assert.Equal(t, "/tmp", result["output_dir"])
	assert.Equal(t, "host", result["scan_target"])
	assert.Len(t, result, 3)
}

func TestWithWorkspaceVar_OverridesUserDefined(t *testing.T) {
	input := map[string]string{WorkspaceVarKey: "."}
	result := WithWorkspaceVar(input, "/resolved/path")

	assert.Equal(t, "/resolved/path", result[WorkspaceVarKey])
}

func TestWithWorkspaceVar_DoesNotMutateOriginal(t *testing.T) {
	input := map[string]string{"key": "value"}
	_ = WithWorkspaceVar(input, "/workspace")

	_, hasWorkspace := input[WorkspaceVarKey]
	assert.False(t, hasWorkspace, "original map must not be mutated")
	assert.Len(t, input, 1)
}

func TestResolveDataDir_DefaultPath(t *testing.T) {
	// Unset XDG_DATA_HOME to exercise the platform fallback.
	t.Setenv("XDG_DATA_HOME", "")

	dir, err := ResolveDataDir()
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	switch runtime.GOOS {
	case "darwin":
		assert.Equal(t, filepath.Join(homeDir, "Library", "Application Support", xdgAppName), dir)
	default:
		// Linux and other Unix-like systems.
		assert.Equal(t, filepath.Join(homeDir, ".local", "share", xdgAppName), dir)
	}
}

func TestResolveDataDir_CustomXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")

	dir, err := ResolveDataDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/custom/data", xdgAppName), dir)
}

func TestResolveDataDir_EmptyXDGDataHomeTreatedAsUnset(t *testing.T) {
	// An empty $XDG_DATA_HOME MUST be treated as unset per the XDG spec.
	t.Setenv("XDG_DATA_HOME", "")

	dir, err := ResolveDataDir()
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	// Should fall back to the platform default, not produce an empty-prefix path.
	assert.Contains(t, dir, homeDir)
	assert.Contains(t, dir, xdgAppName)
}

func TestResolveDataDir_RelativeXDGDataHomeIgnored(t *testing.T) {
	// A relative $XDG_DATA_HOME MUST be ignored per the XDG spec:
	// "If $XDG_DATA_HOME is either not set or empty, a default equal to
	// $HOME/.local/share should be used." The spec also states all paths
	// MUST be absolute.
	t.Setenv("XDG_DATA_HOME", "relative/path")

	dir, err := ResolveDataDir()
	require.NoError(t, err)

	// Should fall back to the platform default, not use the relative path.
	assert.True(t, filepath.IsAbs(dir), "resolved path must be absolute, got: %s", dir)
	assert.NotContains(t, dir, "relative/path")
}

func TestResolveDataDir_ErrorWhenHomeUnset(t *testing.T) {
	// When $XDG_DATA_HOME is unset and $HOME is also unset, the fallback
	// should propagate the os.UserHomeDir() error.
	if runtime.GOOS == "windows" {
		t.Skip("unsetting HOME does not affect os.UserHomeDir on Windows")
	}
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")

	_, err := ResolveDataDir()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resolve data dir")
}

func TestResolveCacheDir_DefaultPath(t *testing.T) {
	dir, err := ResolveCacheDir()
	require.NoError(t, err)

	cacheBase, err := os.UserCacheDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(cacheBase, xdgAppName), dir)
}

func TestResolveCacheDir_CustomXDGCacheHome(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CACHE_HOME only affects os.UserCacheDir on Linux")
	}
	t.Setenv("XDG_CACHE_HOME", "/custom/cache")

	dir, err := ResolveCacheDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/custom/cache", xdgAppName), dir)
}

func TestResolveProviderDir_DefaultPath(t *testing.T) {
	// Unset XDG_DATA_HOME to exercise the platform fallback.
	t.Setenv("XDG_DATA_HOME", "")

	dir, err := ResolveProviderDir()
	require.NoError(t, err)

	dataDir, err := ResolveDataDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(dataDir, providerSubdir), dir)
}

func TestResolveProviderDir_CustomXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")

	dir, err := ResolveProviderDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/custom/data", xdgAppName, providerSubdir), dir)
}
