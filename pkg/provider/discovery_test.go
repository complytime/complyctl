// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T157: DiscoverProviders tests ---

func TestDiscoverProviders_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	d := NewDiscovery(dir)
	providers, err := d.DiscoverProviders()
	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestDiscoverProviders_NonPrefixedExecutables(t *testing.T) {
	dir := t.TempDir()
	createExecutable(t, dir, "some-other-tool")

	d := NewDiscovery(dir)
	providers, err := d.DiscoverProviders()
	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestDiscoverProviders_ValidProvider(t *testing.T) {
	dir := t.TempDir()
	createExecutable(t, dir, complytime.ProviderExecutablePrefix+"mock")

	d := NewDiscovery(dir)
	providers, err := d.DiscoverProviders()
	require.NoError(t, err)
	require.Len(t, providers, 1)
	assert.Equal(t, "mock", providers[0].EvaluatorID)
	assert.Equal(t, filepath.Join(dir, complytime.ProviderExecutablePrefix+"mock"), providers[0].ExecutablePath)
}

// --- T158: scanDir tests ---

func TestScanDir_NonexistentDirectory(t *testing.T) {
	providers, err := scanDir("/nonexistent/path/that/does/not/exist")
	assert.NoError(t, err)
	assert.Nil(t, providers)
}

func TestScanDir_NonExecutableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits not meaningful on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, complytime.ProviderExecutablePrefix+"noexec")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh"), 0644)) // #nosec

	providers, err := scanDir(dir)
	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestScanDir_DirectoryEntriesSkipped(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, complytime.ProviderExecutablePrefix+"subdir")
	require.NoError(t, os.Mkdir(subdir, 0755)) // #nosec

	providers, err := scanDir(dir)
	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestScanDir_MultipleValidExecutables(t *testing.T) {
	dir := t.TempDir()
	createExecutable(t, dir, complytime.ProviderExecutablePrefix+"openscap")
	createExecutable(t, dir, complytime.ProviderExecutablePrefix+"kube-eval")
	createExecutable(t, dir, complytime.ProviderExecutablePrefix+"trivy")

	providers, err := scanDir(dir)
	require.NoError(t, err)
	require.Len(t, providers, 3)

	ids := make(map[string]bool)
	for _, p := range providers {
		ids[p.EvaluatorID] = true
	}
	assert.True(t, ids["openscap"])
	assert.True(t, ids["kube-eval"])
	assert.True(t, ids["trivy"])
}

// --- T159: expandPath tests ---

func TestExpandPath_TildePrefix(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	result := expandPath("~/foo")
	assert.Equal(t, filepath.Join(home, "foo"), result)
}

func TestExpandPath_AbsolutePath(t *testing.T) {
	assert.Equal(t, "/absolute/path", expandPath("/absolute/path"))
}

func TestExpandPath_RelativePath(t *testing.T) {
	assert.Equal(t, "relative/path", expandPath("relative/path"))
}

// --- T160: user-dir precedence test ---

func TestScanDir_UserDirPrecedence(t *testing.T) {
	userDir := t.TempDir()
	sysDir := t.TempDir()
	createExecutable(t, userDir, complytime.ProviderExecutablePrefix+"openscap")
	createExecutable(t, sysDir, complytime.ProviderExecutablePrefix+"openscap")
	createExecutable(t, sysDir, complytime.ProviderExecutablePrefix+"sys-only")

	seen := make(map[string]bool)
	var providers []ProviderInfo

	userProviders, err := scanDir(userDir)
	require.NoError(t, err)
	for _, p := range userProviders {
		seen[p.EvaluatorID] = true
		providers = append(providers, p)
	}

	sysProviders, err := scanDir(sysDir)
	require.NoError(t, err)
	for _, p := range sysProviders {
		if !seen[p.EvaluatorID] {
			providers = append(providers, p)
		}
	}

	require.Len(t, providers, 2)

	idPaths := make(map[string]string)
	for _, p := range providers {
		idPaths[p.EvaluatorID] = p.ExecutablePath
	}

	assert.Equal(t, filepath.Join(userDir, complytime.ProviderExecutablePrefix+"openscap"), idPaths["openscap"],
		"user-dir openscap should take precedence over system-dir")
	assert.Equal(t, filepath.Join(sysDir, complytime.ProviderExecutablePrefix+"sys-only"), idPaths["sys-only"])
}

func createExecutable(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), 0755)) // #nosec
}
