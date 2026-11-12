// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/cache/cachetest"
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

// --- ResolveVersion happy-path tests ---

func TestResolveVersion_DirectTagMatch(t *testing.T) {
	loader := seedTestPolicy(t, "resolve-direct", "v1.0.0")

	version, err := loader.ResolveVersion("resolve-direct", "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", version)
}

func TestResolveVersion_LatestFallbackToCachedVersion(t *testing.T) {
	loader := seedTestPolicy(t, "resolve-latest", "v1.0.0")

	version, err := loader.ResolveVersion("resolve-latest", "")
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", version, "should fall back to the last cached version when 'latest' tag doesn't exist")
}

// --- ListCachedPolicies tests ---

func TestListCachedPolicies_EmptyCache(t *testing.T) {
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	loader := NewLoader(cacheMgr)

	result, err := loader.ListCachedPolicies()
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListCachedPolicies_SeededCache(t *testing.T) {
	loader := seedTestPolicy(t, "policy-a", "v1.0.0")

	result, err := loader.ListCachedPolicies()
	require.NoError(t, err)
	require.Contains(t, result, "policy-a")
	assert.Equal(t, []string{"v1.0.0"}, result["policy-a"])
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
	mock.SeedPolicy(policyID, version, cachetest.DigestA)

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")
	_, err = sync.SyncPolicy(context.Background(), policyID, "latest")
	require.NoError(t, err)

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
	mock.SeedBundlePolicy(policyID, version, cachetest.DigestB, files)

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")
	_, err = sync.SyncPolicy(context.Background(), policyID, "latest")
	require.NoError(t, err)

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

func testPolicyYAML(id string) string {
	return fmt.Sprintf(`metadata:
  id: %s
  type: Policy
title: %s
adherence:
  assessment-plans:
    - id: %s-plan
      requirement-id: %s-req
`, id, id, id, id)
}

func testCatalogYAML(id string) string {
	return fmt.Sprintf(`metadata:
  id: %s
  type: ControlCatalog
controls:
  - id: %s-control
    title: Example control
`, id, id)
}

func TestLoadBundleArtifactsPreservesMultipleCatalogsAndPolicies(t *testing.T) {
	files := []bundle.File{
		{Name: "root.yaml", Type: "Policy", Data: []byte(testPolicyYAML("root-policy"))},
		{Name: "catalog-a.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("catalog-a"))},
		{Name: "catalog-b.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("catalog-b"))},
		{Name: "parent.yaml", Type: "Policy", Data: []byte(testPolicyYAML("parent-policy"))},
	}
	loader := seedBundlePolicy(t, "multi", "v1", files)
	artifacts, err := loader.LoadBundleArtifacts("multi", "v1")
	require.NoError(t, err)
	assert.Equal(t, files[0], artifacts.Source)
	assert.Equal(t, files[1:], artifacts.Imports)
	assert.Equal(t, files[1:3], artifacts.FilesByType("ControlCatalog"))
	assert.Equal(t, []bundle.File{files[0], files[3]}, artifacts.FilesByType("Policy"))
	assert.Equal(t, files[3:], artifacts.ImportsByType("Policy"))
	assert.Empty(t, artifacts.FilesByType("GuidanceCatalog"))
	assert.Empty(t, artifacts.ImportsByType("GuidanceCatalog"))

	collapsed, err := loader.LoadBundleFiles("multi", "v1")
	require.NoError(t, err)
	assert.Equal(t, map[string][]byte{
		"Policy": files[0].Data, "ControlCatalog": files[1].Data,
	}, collapsed)
}

func TestLoadBundleArtifacts_NormalizesManifestTypes(t *testing.T) {
	cacheMgr := cache.NewCache(t.TempDir())
	store, err := cacheMgr.NewPolicyStore("manifest-types")
	require.NoError(t, err)
	ctx := context.Background()
	source := bundle.File{Name: "policies/root.yaml", Data: []byte(testPolicyYAML("root"))}
	imports := []bundle.File{
		{Name: "catalogs/a.yaml", Data: []byte(testCatalogYAML("a"))},
		{Name: "catalogs/b.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("b"))},
		{Name: "extra.yaml", Data: []byte("opaque: content\n")},
	}
	desc, err := bundle.Pack(ctx, store, &bundle.Bundle{
		Source: source, Imports: imports,
		Manifest: bundle.Manifest{Artifacts: []bundle.Artifact{
			{Name: source.Name, Type: "Policy"},
			{Name: imports[0].Name, Type: "ControlCatalog"},
			{Name: imports[1].Name, Type: "GuidanceCatalog"},
		}},
	})
	require.NoError(t, err)
	require.NoError(t, store.Tag(ctx, desc, "v1"))
	loader := NewLoader(cacheMgr)
	artifacts, err := loader.LoadBundleArtifacts("manifest-types", "v1")
	require.NoError(t, err)
	source.Type = "Policy"
	imports[0].Type = "ControlCatalog"
	assert.Equal(t, source, artifacts.Source)
	assert.Equal(t, imports, artifacts.Imports)
	assert.Equal(t, imports[:2], artifacts.FilesByType("ControlCatalog"))
	files, err := loader.LoadBundleFiles("manifest-types", "v1")
	require.NoError(t, err)
	assert.Equal(t, map[string][]byte{"Policy": source.Data, "ControlCatalog": imports[0].Data}, files)
}

func TestLoadBundleArtifacts_InvalidInputsAndMissingBundle(t *testing.T) {
	for _, tt := range []struct {
		name, policyID, version, wantError string
	}{
		{"empty policy", "", "v1", "policy ID cannot be empty"},
		{"empty version", "test", "", "version cannot be empty"},
		{"missing bundle", "test", "v1", "failed to unpack bundle"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoader(cache.NewCache(t.TempDir()))
			artifacts, err := loader.LoadBundleArtifacts(tt.policyID, tt.version)
			require.ErrorContains(t, err, tt.wantError)
			assert.Empty(t, artifacts.Source)
			assert.Empty(t, artifacts.Imports)
		})
	}
}
