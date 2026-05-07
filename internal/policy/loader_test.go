// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gemaraproj/go-gemara/bundle"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/cache/cachetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T155: PolicyExists, ResolveVersion, GetCachedVersions ---

func TestPolicyExists_NonexistentPolicy(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	assert.False(t, loader.PolicyExists("nonexistent-policy", "v1"))
}

func TestPolicyExists_EmptyInputs(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	assert.False(t, loader.PolicyExists("", "v1"))
	assert.False(t, loader.PolicyExists("test", ""))
	assert.False(t, loader.PolicyExists("", ""))
}

func TestResolveVersion_EmptyCache(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	_, err := loader.ResolveVersion("nonexistent", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in cache")
}

func TestResolveVersion_LatestFallbackEmptyCache(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	_, err := loader.ResolveVersion("nonexistent", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in cache")
}

func TestGetCachedVersions_NonexistentPolicy(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	versions, err := loader.GetCachedVersions("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, versions)
}

// --- T156: LoadLayerByMediaType ---

func TestLoadLayerByMediaType_EmptyPolicyID(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	_, err := loader.LoadLayerByMediaType("", "v1", "application/vnd.gemara.policy.v1+yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy ID cannot be empty")
}

func TestLoadLayerByMediaType_EmptyVersion(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	_, err := loader.LoadLayerByMediaType("test", "", "application/vnd.gemara.policy.v1+yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version cannot be empty")
}

func TestLoadLayerByMediaType_EmptyMediaType(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	_, err := loader.LoadLayerByMediaType("test", "v1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "media type cannot be empty")
}

func TestLoadLayerByMediaType_NonexistentPolicy(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	_, err := loader.LoadLayerByMediaType("missing", "v1", "application/vnd.gemara.policy.v1+yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in cache")
}

// seedTestPolicy syncs a mock policy into the cache and returns the loader.
func seedTestPolicy(t *testing.T, policyID, version string) *Loader {
	t.Helper()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	mock.SeedPolicy(policyID, version, "sha256:test-digest")

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock)
	require.NoError(t, sync.SyncPolicy(context.Background(), policyID, "latest"))

	return NewLoader(cacheMgr)
}

func TestLoadLayerByMediaType_HappyPath(t *testing.T) {
	loader := seedTestPolicy(t, "test-policy", "v1.0.0")

	data, err := loader.LoadLayerByMediaType("test-policy", "v1.0.0", "application/vnd.gemara.policy.v1+yaml")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "test-policy")
}

func TestLoadLayerByMediaType_WrongMediaType(t *testing.T) {
	loader := seedTestPolicy(t, "test-policy", "v1.0.0")

	_, err := loader.LoadLayerByMediaType("test-policy", "v1.0.0", "application/vnd.gemara.catalog.v1+yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLoadLayerByMediaType_WrongVersion(t *testing.T) {
	loader := seedTestPolicy(t, "test-policy", "v1.0.0")

	_, err := loader.LoadLayerByMediaType("test-policy", "v2.0.0", "application/vnd.gemara.policy.v1+yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in cache")
}

// --- Bundle-shape loader integration tests (US1: 005-bundle-resolver-alignment) ---

// seedBundlePolicy creates a local OCI store with bundle.Pack-produced content.
func seedBundlePolicy(t *testing.T, policyID, version string, files []bundle.File) *Loader {
	t.Helper()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockBundlePolicySource()
	mock.SeedBundlePolicy(policyID, version, "sha256:bundle-digest", files)

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock)
	require.NoError(t, sync.SyncPolicy(context.Background(), policyID, "latest"))

	return NewLoader(cacheMgr)
}

func TestDetectManifestShape_SplitLayer(t *testing.T) {
	loader := seedTestPolicy(t, "split-policy", "v1.0.0")

	isBundle, err := loader.DetectManifestShape("split-policy", "v1.0.0")
	require.NoError(t, err)
	assert.False(t, isBundle, "split-layer manifest should not be detected as bundle")
}

func TestDetectManifestShape_Bundle(t *testing.T) {
	loader := seedBundlePolicy(t, "bundle-policy", "v1.0.0", []bundle.File{
		{Name: "policy.yaml", Type: "Policy", Data: []byte("metadata:\n  type: Policy\n")},
	})

	isBundle, err := loader.DetectManifestShape("bundle-policy", "v1.0.0")
	require.NoError(t, err)
	assert.True(t, isBundle, "bundle manifest should be detected as bundle")
}

func TestDetectManifestShape_EmptyInputs(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	_, err := loader.DetectManifestShape("", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy ID and version are required")

	_, err = loader.DetectManifestShape("test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy ID and version are required")
}

func TestLoadBundleFiles_HappyPath(t *testing.T) {
	policyData := []byte("metadata:\n  type: Policy\n  id: pol-1\n")
	catalogData := []byte("metadata:\n  type: ControlCatalog\n  id: cat-1\n")

	loader := seedBundlePolicy(t, "bundle-multi", "v1.0.0", []bundle.File{
		{Name: "policy.yaml", Type: "Policy", Data: policyData},
		{Name: "catalog.yaml", Type: "ControlCatalog", Data: catalogData},
	})

	files, err := loader.LoadBundleFiles("bundle-multi", "v1.0.0")
	require.NoError(t, err)
	assert.Len(t, files, 2, "should contain exactly Policy and ControlCatalog")
	assert.Contains(t, files, "Policy")
	assert.Contains(t, files, "ControlCatalog")
	assert.Equal(t, policyData, files["Policy"])
	assert.Equal(t, catalogData, files["ControlCatalog"])
}

func TestLoadBundleFiles_EmptyPolicyID(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	_, err := loader.LoadBundleFiles("", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy ID cannot be empty")
}

func TestLoadBundleFiles_EmptyVersion(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	_, err := loader.LoadBundleFiles("test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version cannot be empty")
}
