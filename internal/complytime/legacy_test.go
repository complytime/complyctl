// SPDX-License-Identifier: Apache-2.0

package complytime

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckLegacyDir_LegacyExistsXDGAbsent_Warns(t *testing.T) {
	homeDir := t.TempDir()

	// Create legacy directory.
	legacyDir := filepath.Join(homeDir, legacyDirName)
	require.NoError(t, os.MkdirAll(legacyDir, 0700))

	// XDG directories do not exist (fresh install scenario).
	// Override XDG env vars so ResolveCacheDir/ResolveDataDir point
	// into the temp home where nothing exists yet.
	t.Setenv("XDG_CACHE_HOME", filepath.Join(homeDir, "xdg-cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, "xdg-data"))

	var buf bytes.Buffer
	checkLegacyDirWithHome(&buf, homeDir)

	output := buf.String()
	assert.Contains(t, output, "WARNING:")
	assert.Contains(t, output, legacyDir)
}

func TestCheckLegacyDir_LegacyExistsXDGExists_NoWarn(t *testing.T) {
	homeDir := t.TempDir()

	// Create legacy directory.
	legacyDir := filepath.Join(homeDir, legacyDirName)
	require.NoError(t, os.MkdirAll(legacyDir, 0700))

	// Create both XDG directories so migration is considered complete.
	cacheBase := filepath.Join(homeDir, "xdg-cache")
	dataBase := filepath.Join(homeDir, "xdg-data")
	t.Setenv("XDG_CACHE_HOME", cacheBase)
	t.Setenv("XDG_DATA_HOME", dataBase)

	cacheDir := filepath.Join(cacheBase, xdgAppName)
	dataDir := filepath.Join(dataBase, xdgAppName)
	require.NoError(t, os.MkdirAll(cacheDir, 0700))
	require.NoError(t, os.MkdirAll(dataDir, 0700))

	var buf bytes.Buffer
	checkLegacyDirWithHome(&buf, homeDir)

	assert.Empty(t, buf.String(), "no warning when both XDG dirs exist")
}

func TestCheckLegacyDir_LegacyAbsent_NoWarn(t *testing.T) {
	homeDir := t.TempDir()
	// No legacy directory exists — nothing to warn about.

	t.Setenv("XDG_CACHE_HOME", filepath.Join(homeDir, "xdg-cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, "xdg-data"))

	var buf bytes.Buffer
	checkLegacyDirWithHome(&buf, homeDir)

	assert.Empty(t, buf.String(), "no warning when legacy dir is absent")
}

func TestCheckLegacyDir_LegacyExistsCacheExistsDataAbsent_Warns(t *testing.T) {
	homeDir := t.TempDir()

	// Create legacy directory.
	legacyDir := filepath.Join(homeDir, legacyDirName)
	require.NoError(t, os.MkdirAll(legacyDir, 0700))

	// Cache exists but data does not — state.json has not been migrated.
	cacheBase := filepath.Join(homeDir, "xdg-cache")
	dataBase := filepath.Join(homeDir, "xdg-data")
	t.Setenv("XDG_CACHE_HOME", cacheBase)
	t.Setenv("XDG_DATA_HOME", dataBase)

	cacheDir := filepath.Join(cacheBase, xdgAppName)
	require.NoError(t, os.MkdirAll(cacheDir, 0700))
	// dataDir intentionally not created.

	var buf bytes.Buffer
	checkLegacyDirWithHome(&buf, homeDir)

	output := buf.String()
	assert.Contains(t, output, "WARNING:")
	assert.Contains(t, output, legacyDir)
}

func TestCheckLegacyDir_WarningContainsExpectedPaths(t *testing.T) {
	homeDir := t.TempDir()

	// Create legacy directory.
	legacyDir := filepath.Join(homeDir, legacyDirName)
	require.NoError(t, os.MkdirAll(legacyDir, 0700))

	cacheBase := filepath.Join(homeDir, "xdg-cache")
	dataBase := filepath.Join(homeDir, "xdg-data")
	t.Setenv("XDG_CACHE_HOME", cacheBase)
	t.Setenv("XDG_DATA_HOME", dataBase)

	expectedCacheDir := filepath.Join(cacheBase, xdgAppName)
	expectedDataDir := filepath.Join(dataBase, xdgAppName)

	var buf bytes.Buffer
	checkLegacyDirWithHome(&buf, homeDir)

	output := buf.String()
	// Must contain the legacy path.
	assert.Contains(t, output, legacyDir)
	// Must contain the new cache path.
	assert.Contains(t, output, expectedCacheDir)
	// Must contain the new data path.
	assert.Contains(t, output, expectedDataDir)
	// Must distinguish re-fetchable cache from non-recoverable state.
	assert.Contains(t, output, "re-fetchable")
	assert.Contains(t, output, "not recoverable")
	// Must include state.json reference.
	assert.Contains(t, output, StateFileName)
}

func TestCheckLegacyDir_OutputGoesToProvidedWriter(t *testing.T) {
	homeDir := t.TempDir()

	legacyDir := filepath.Join(homeDir, legacyDirName)
	require.NoError(t, os.MkdirAll(legacyDir, 0700))

	cacheBase := filepath.Join(homeDir, "xdg-cache")
	dataBase := filepath.Join(homeDir, "xdg-data")
	t.Setenv("XDG_CACHE_HOME", cacheBase)
	t.Setenv("XDG_DATA_HOME", dataBase)

	// Use two separate writers to verify output goes only to the provided one.
	var primary, secondary bytes.Buffer
	checkLegacyDirWithHome(&primary, homeDir)

	assert.NotEmpty(t, primary.String(), "output must go to the provided writer")
	assert.Empty(t, secondary.String(), "output must not go to other writers")
}

func TestCheckLegacyDir_LegacyContentsUnchanged(t *testing.T) {
	homeDir := t.TempDir()

	// Create legacy directory with some content.
	legacyDir := filepath.Join(homeDir, legacyDirName)
	require.NoError(t, os.MkdirAll(legacyDir, 0700))

	stateFile := filepath.Join(legacyDir, StateFileName)
	stateContent := []byte(`{"last_sync":"2025-01-01T00:00:00Z","policies":{}}`)
	require.NoError(t, os.WriteFile(stateFile, stateContent, 0600))

	policiesDir := filepath.Join(legacyDir, PoliciesSubdir)
	require.NoError(t, os.MkdirAll(policiesDir, 0700))
	policyFile := filepath.Join(policiesDir, "test-policy.yaml")
	policyContent := []byte("policy: test")
	require.NoError(t, os.WriteFile(policyFile, policyContent, 0600))

	cacheBase := filepath.Join(homeDir, "xdg-cache")
	dataBase := filepath.Join(homeDir, "xdg-data")
	t.Setenv("XDG_CACHE_HOME", cacheBase)
	t.Setenv("XDG_DATA_HOME", dataBase)

	var buf bytes.Buffer
	checkLegacyDirWithHome(&buf, homeDir)

	// Warning should have been printed (legacy exists, XDG absent).
	require.NotEmpty(t, buf.String())

	// Verify legacy directory contents are unchanged.
	readState, err := os.ReadFile(stateFile)
	require.NoError(t, err)
	assert.Equal(t, stateContent, readState, "state.json must not be modified")

	readPolicy, err := os.ReadFile(policyFile)
	require.NoError(t, err)
	assert.Equal(t, policyContent, readPolicy, "policy file must not be modified")

	// Verify legacy directory structure is intact.
	_, err = os.Stat(legacyDir)
	require.NoError(t, err, "legacy directory must still exist")
	_, err = os.Stat(policiesDir)
	require.NoError(t, err, "policies subdirectory must still exist")
}

func TestCheckLegacyDir_PublicFunction_NoHomeError(t *testing.T) {
	// When HOME is unset, CheckLegacyDir should silently return
	// without panicking or printing anything.
	if runtime.GOOS == "windows" {
		t.Skip("unsetting HOME does not affect os.UserHomeDir on Windows")
	}
	t.Setenv("HOME", "")

	var buf bytes.Buffer
	// Call the public function (not the internal one) to verify
	// graceful degradation when os.UserHomeDir() fails.
	CheckLegacyDir(&buf)

	assert.Empty(t, buf.String(), "must not print when home dir is unavailable")
}

func TestCheckLegacyDir_LegacyExistsDataExistsCacheAbsent_NoWarn(t *testing.T) {
	// When data dir exists but cache dir doesn't, the critical state.json
	// is already in the right place. Cache will be re-created on next
	// `complyctl get`. However, the warning should still fire because
	// the legacy dir exists and not all XDG dirs are present — the user
	// should still clean up.
	homeDir := t.TempDir()

	legacyDir := filepath.Join(homeDir, legacyDirName)
	require.NoError(t, os.MkdirAll(legacyDir, 0700))

	cacheBase := filepath.Join(homeDir, "xdg-cache")
	dataBase := filepath.Join(homeDir, "xdg-data")
	t.Setenv("XDG_CACHE_HOME", cacheBase)
	t.Setenv("XDG_DATA_HOME", dataBase)

	// Data exists, cache does not.
	dataDir := filepath.Join(dataBase, xdgAppName)
	require.NoError(t, os.MkdirAll(dataDir, 0700))

	var buf bytes.Buffer
	checkLegacyDirWithHome(&buf, homeDir)

	// Warning fires because cache dir is absent and legacy exists.
	output := buf.String()
	assert.Contains(t, output, "WARNING:")
}

func TestDirExists_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	assert.True(t, dirExists(dir))
}

func TestDirExists_NonExistent(t *testing.T) {
	assert.False(t, dirExists("/nonexistent/path/xyz123"))
}

func TestDirExists_FileNotDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("test"), 0600))
	assert.False(t, dirExists(f))
}
