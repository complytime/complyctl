// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2"
	ocistore "oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/errdef"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complypack/pkg/complypack"
)

// mockComplypackSource implements cache.ComplypackSource for testing.
// Uses complypack.Pack() to create real complypack artifacts in the
// destination OCI store, so complypack.Unpack() works downstream.
type mockComplypackSource struct {
	mu        sync.RWMutex
	packs     map[string]*mockComplypackData
	copyCount int // tracks how many times CopyComplypack was called
}

type mockComplypackData struct {
	digest      string
	version     string
	evaluatorID string
	id          string
	content     string
}

func newMockComplypackSource() *mockComplypackSource {
	return &mockComplypackSource{
		packs: make(map[string]*mockComplypackData),
	}
}

// seedComplypack registers a complypack under both the bare repository key
// and the versioned key (repository:version) so lookups resolve either way.
func (m *mockComplypackSource) seedComplypack(repository, evaluatorID, version, digestStr, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data := &mockComplypackData{
		digest:      digestStr,
		version:     version,
		evaluatorID: evaluatorID,
		id:          "test-" + evaluatorID,
		content:     content,
	}
	m.packs[repository] = data
	m.packs[repository+":"+version] = data
}

// seedComplypackWithTag registers a complypack where the OCI tag differs from
// the config version (e.g., tag "v1.0.0" vs config version "1.0.0"). The
// DefinitionVersion lookup uses the tag for the versioned key, while
// CopyComplypack packs the artifact with the config version.
func (m *mockComplypackSource) seedComplypackWithTag(
	repository, evaluatorID, tag, configVersion, digestStr, content string,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data := &mockComplypackData{
		digest:      digestStr,
		version:     configVersion,
		evaluatorID: evaluatorID,
		id:          "test-" + evaluatorID,
		content:     content,
	}
	m.packs[repository] = data
	m.packs[repository+":"+tag] = data
}

func (m *mockComplypackSource) DefinitionVersion(_ context.Context, lookupRef string) (string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.packs[lookupRef]
	if !ok {
		return "", "", &complypackNotFoundError{ref: lookupRef}
	}
	return p.digest, p.version, nil
}

// CopyComplypack uses complypack.Pack() to create a real complypack artifact
// in the destination OCI store. This mirrors what oras.Copy() does in
// production — the artifact is fully formed and can be unpacked by
// complypack.Unpack().
func (m *mockComplypackSource) CopyComplypack(ctx context.Context, repository, tag string, dst *ocistore.Store) (ocispec.Descriptor, error) {
	m.mu.Lock()
	m.copyCount++
	m.mu.Unlock()

	m.mu.RLock()
	p, ok := m.packs[repository]
	m.mu.RUnlock()
	if !ok {
		return ocispec.Descriptor{}, &complypackNotFoundError{ref: repository}
	}

	cfg := complypack.Config{
		ID:          p.id,
		EvaluatorID: p.evaluatorID,
		Version:     p.version,
	}

	desc, err := complypack.Pack(ctx, dst, cfg, strings.NewReader(p.content))
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	if err := dst.Tag(ctx, desc, tag); err != nil {
		return ocispec.Descriptor{}, err
	}

	return desc, nil
}

func (m *mockComplypackSource) getCopyCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.copyCount
}

type complypackNotFoundError struct {
	ref string
}

func (e *complypackNotFoundError) Error() string {
	return "complypack " + e.ref + " not found"
}

// --- Local cache hit tests ---

// TestComplypackSync_LocalCacheHit verifies FR-004: when switching to a version
// that already exists in the local cache, SyncComplypack returns (true, nil)
// without calling CopyComplypack, and state.json is updated to reflect the
// switched version.
func TestComplypackSync_LocalCacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	t.Setenv("COMPLYTIME_CACHE_VERSIONS", "3")

	mock := newMockComplypackSource()
	repository := "example.com/complypacks/opa-bundle"
	evalID := "io.complytime.opa"

	// Seed v1.0.0 via a real sync.
	mock.seedComplypack(repository, evalID, "1.0.0", "sha256:v1digest", "v1 content")
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	cpCache := cache.NewComplypackCache(cacheDir, state)
	syncMgr := cache.NewComplypackSync(cpCache, state, mock, cacheDir)

	fetched, err := syncMgr.SyncComplypack(context.Background(), repository, "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched)
	assert.Equal(t, 1, mock.getCopyCount())

	// Seed v2.0.0 via a real sync (simulates switching to a new version).
	mock.seedComplypack(repository, evalID, "2.0.0", "sha256:v2digest", "v2 content")
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	cpCache2 := cache.NewComplypackCache(cacheDir, state2)
	syncMgr2 := cache.NewComplypackSync(cpCache2, state2, mock, cacheDir)

	fetched2, err := syncMgr2.SyncComplypack(context.Background(), repository, "2.0.0")
	require.NoError(t, err)
	assert.True(t, fetched2)
	assert.Equal(t, 2, mock.getCopyCount())

	// Now switch back to v1.0.0 — should be a local cache hit.
	// Update mock to return v1.0.0 as the remote version.
	mock.seedComplypack(repository, evalID, "1.0.0", "sha256:v1digest", "v1 content")
	state3, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	cpCache3 := cache.NewComplypackCache(cacheDir, state3)
	syncMgr3 := cache.NewComplypackSync(cpCache3, state3, mock, cacheDir)

	fetched3, err := syncMgr3.SyncComplypack(context.Background(), repository, "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched3, "local cache hit should return true for generation invalidation")
	assert.Equal(t, 2, mock.getCopyCount(),
		"CopyComplypack should NOT be called for local cache hit")

	// Verify state.json was updated to reflect v1.0.0 as active.
	state4, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state4.GetComplypackState(repository)
	require.True(t, ok)
	assert.Equal(t, "1.0.0", ps.Version,
		"state should reflect switched version")
}

// TestComplypackSync_LocalCacheMiss_CorruptedContent verifies that when a
// version directory exists but content files are missing, the sync falls
// through to remote fetch (CopyComplypack IS called).
func TestComplypackSync_LocalCacheMiss_CorruptedContent(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	t.Setenv("COMPLYTIME_CACHE_VERSIONS", "3")

	mock := newMockComplypackSource()
	repository := "example.com/complypacks/opa-bundle"
	evalID := "io.complytime.opa"

	// First sync v2.0.0 to establish state.
	mock.seedComplypack(repository, evalID, "2.0.0", "sha256:v2digest", "v2 content")
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	cpCache := cache.NewComplypackCache(cacheDir, state)
	syncMgr := cache.NewComplypackSync(cpCache, state, mock, cacheDir)

	_, err = syncMgr.SyncComplypack(context.Background(), repository, "2.0.0")
	require.NoError(t, err)
	assert.Equal(t, 1, mock.getCopyCount())

	// Create a v1.0.0 directory but with missing content (corrupted).
	corruptDir := filepath.Join(cacheDir, "complypacks", evalID, "1.0.0")
	require.NoError(t, os.MkdirAll(corruptDir, 0755))
	// No content.tar.gz or config.json — Lookup will fail.

	// Now try to sync v1.0.0 — should fall through to remote fetch.
	mock.seedComplypack(repository, evalID, "1.0.0", "sha256:v1digest", "v1 content")
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	cpCache2 := cache.NewComplypackCache(cacheDir, state2)
	syncMgr2 := cache.NewComplypackSync(cpCache2, state2, mock, cacheDir)

	fetched, err := syncMgr2.SyncComplypack(context.Background(), repository, "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched)
	assert.Equal(t, 2, mock.getCopyCount(),
		"CopyComplypack should be called when local cache is corrupted")
}

// TestComplypackSync_LocalCacheHit_WithVerifier verifies FR-010: when a
// verifier is configured and a local cache hit occurs, re-verification
// occurs before accepting the cache hit.
func TestComplypackSync_LocalCacheHit_WithVerifier(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	t.Setenv("COMPLYTIME_CACHE_VERSIONS", "3")

	mock := newMockComplypackSource()
	repository := "example.com/complypacks/opa-bundle"
	evalID := "io.complytime.opa"

	// Sync v1.0.0 first (no verifier).
	mock.seedComplypack(repository, evalID, "1.0.0", "sha256:v1digest", "v1 content")
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	cpCache := cache.NewComplypackCache(cacheDir, state)
	syncMgr := cache.NewComplypackSync(cpCache, state, mock, cacheDir)

	_, err = syncMgr.SyncComplypack(context.Background(), repository, "1.0.0")
	require.NoError(t, err)

	// Sync v2.0.0 to switch active version.
	mock.seedComplypack(repository, evalID, "2.0.0", "sha256:v2digest", "v2 content")
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	cpCache2 := cache.NewComplypackCache(cacheDir, state2)
	syncMgr2 := cache.NewComplypackSync(cpCache2, state2, mock, cacheDir)

	_, err = syncMgr2.SyncComplypack(context.Background(), repository, "2.0.0")
	require.NoError(t, err)

	// Now switch back to v1.0.0 WITH a verifier — verify it's called.
	mock.seedComplypack(repository, evalID, "1.0.0", "sha256:v1digest", "v1 content")
	state3, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	cpCache3 := cache.NewComplypackCache(cacheDir, state3)

	verifyCalled := false
	mockVerifier := func(_ context.Context, _ string) (*cache.VerificationResult, error) {
		verifyCalled = true
		return &cache.VerificationResult{Verified: true}, nil
	}

	syncMgr3 := cache.NewComplypackSync(
		cpCache3, state3, mock, cacheDir,
		cache.WithVerifier(mockVerifier),
	)

	fetched, err := syncMgr3.SyncComplypack(context.Background(), repository, "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched, "local cache hit should return true")
	assert.True(t, verifyCalled, "verifier should be called on local cache hit")
}

// TestComplypackSync_FetchAndStore verifies the full fetch-unpack-store pipeline:
// a valid complypack artifact is created via complypack.Pack() in the mock source,
// synced through ComplypackSync, and the resulting cache contains the expected
// content.tar.gz and config.json files.
//
// Note: the sync layer does not perform signature verification. Unsigned-artifact
// warnings are the CLI layer's responsibility (see get.go).
func TestComplypackSync_FetchAndStore(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := newMockComplypackSource()
	mock.seedComplypack(
		"example.com/complypacks/opa-bundle",
		"io.complytime.opa",
		"1.0.0",
		"sha256:fetchandstore111",
		"test policy content for opa",
	)

	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	complypackCache := cache.NewComplypackCache(cacheDir, state)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock, cacheDir)

	fetched, err := syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/opa-bundle", "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched, "first sync should report a fetch occurred")

	// Verify state was updated and persisted.
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state2.GetComplypackState("example.com/complypacks/opa-bundle")
	assert.True(t, ok, "complypack state should exist after sync")
	assert.Equal(t, "sha256:fetchandstore111", ps.Digest)
	assert.Equal(t, "1.0.0", ps.Version)

	// Verify cache files exist via Lookup.
	contentPath, cfg, err := complypackCache.Lookup("io.complytime.opa", "1.0.0")
	require.NoError(t, err)
	assert.FileExists(t, contentPath)
	assert.Equal(t, "io.complytime.opa", cfg.EvaluatorID)
	assert.Equal(t, "1.0.0", cfg.Version)

	// Verify content.tar.gz has non-zero size (complypack.Pack wrote real content).
	info, err := os.Stat(contentPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "content.tar.gz should not be empty")

	// Verify config.json exists alongside content.tar.gz.
	configPath := filepath.Join(filepath.Dir(contentPath), "config.json")
	assert.FileExists(t, configPath)
}

// TestComplypackSync_IncrementalSkip verifies that a second sync with the same
// remote digest is a no-op: CopyComplypack is not called again.
func TestComplypackSync_IncrementalSkip(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := newMockComplypackSource()
	mock.seedComplypack(
		"example.com/complypacks/opa-bundle",
		"io.complytime.opa",
		"1.0.0",
		"sha256:incremental111",
		"opa bundle content",
	)

	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	complypackCache := cache.NewComplypackCache(cacheDir, state)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock, cacheDir)

	// First sync — should fetch and store.
	fetched, err := syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/opa-bundle", "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched, "first sync should report a fetch occurred")
	assert.Equal(t, 1, mock.getCopyCount(), "first sync should call CopyComplypack once")

	// Reload state from disk (as production code does between syncs).
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	complypackCache2 := cache.NewComplypackCache(cacheDir, state2)
	syncMgr2 := cache.NewComplypackSync(complypackCache2, state2, mock, cacheDir)

	// Second sync with same digest — should be a no-op.
	fetched2, err := syncMgr2.SyncComplypack(context.Background(), "example.com/complypacks/opa-bundle", "1.0.0")
	require.NoError(t, err)
	assert.False(t, fetched2, "second sync with same digest should report no fetch")
	assert.Equal(t, 1, mock.getCopyCount(),
		"second sync with same digest should not call CopyComplypack again")

	// Verify state is unchanged.
	state3, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state3.GetComplypackState("example.com/complypacks/opa-bundle")
	require.True(t, ok)
	assert.Equal(t, "sha256:incremental111", ps.Digest)
}

// TestComplypackSync_CacheMissing_RefetchesDespiteMatchingDigest verifies that
// when the cache directory has been deleted but state.json still records a
// matching digest, SyncComplypack re-fetches the artifact instead of skipping.
// This is the fix for #649: state/filesystem desynchronization.
func TestComplypackSync_CacheMissing_RefetchesDespiteMatchingDigest(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := newMockComplypackSource()
	mock.seedComplypack(
		"example.com/complypacks/opa-bundle",
		"io.complytime.opa",
		"1.0.0",
		"sha256:cache-missing-test",
		"opa bundle content for cache-missing test",
	)

	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	complypackCache := cache.NewComplypackCache(cacheDir, state)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock, cacheDir)

	// First sync — should fetch and store.
	fetched, err := syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/opa-bundle", "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched, "first sync should fetch")
	assert.Equal(t, 1, mock.getCopyCount())

	// Verify cache exists.
	contentPath, _, err := complypackCache.LookupByEvaluatorID("io.complytime.opa")
	require.NoError(t, err)
	assert.NotEmpty(t, contentPath, "cache should exist after first sync")

	// Delete the cache directory to simulate user deletion.
	complypacksDir := filepath.Join(cacheDir, "complypacks", "io.complytime.opa")
	require.NoError(t, os.RemoveAll(complypacksDir))

	// Verify cache is gone.
	contentPath2, _, err := complypackCache.LookupByEvaluatorID("io.complytime.opa")
	require.NoError(t, err)
	assert.Empty(t, contentPath2, "cache should be gone after deletion")

	// Reload state — it still records the old digest.
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state2.GetComplypackState("example.com/complypacks/opa-bundle")
	require.True(t, ok, "state should still record the complypack")
	assert.Equal(t, "sha256:cache-missing-test", ps.Digest,
		"state should still have the old digest")

	complypackCache2 := cache.NewComplypackCache(cacheDir, state2)
	syncMgr2 := cache.NewComplypackSync(complypackCache2, state2, mock, cacheDir)

	// Second sync — same digest, but cache is missing. Should re-fetch.
	fetched2, err := syncMgr2.SyncComplypack(context.Background(), "example.com/complypacks/opa-bundle", "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched2,
		"sync should re-fetch when cache is missing despite matching digest")
	assert.Equal(t, 2, mock.getCopyCount(),
		"CopyComplypack should be called again when cache is missing")

	// Verify cache is restored.
	contentPath3, _, err := complypackCache.LookupByEvaluatorID("io.complytime.opa")
	require.NoError(t, err)
	assert.NotEmpty(t, contentPath3, "cache should be restored after re-fetch")
	assert.FileExists(t, contentPath3, "content.tar.gz should exist after re-fetch")
}

// TestComplypackSync_DigestChanged verifies that when the remote digest changes,
// a re-fetch is triggered and the cache is updated with the new content.
func TestComplypackSync_DigestChanged(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := newMockComplypackSource()
	mock.seedComplypack(
		"example.com/complypacks/opa-bundle",
		"io.complytime.opa",
		"1.0.0",
		"sha256:digest_v1",
		"opa bundle content v1",
	)

	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	complypackCache := cache.NewComplypackCache(cacheDir, state)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock, cacheDir)

	// First sync.
	fetched, err := syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/opa-bundle", "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched, "first sync should report a fetch occurred")
	assert.Equal(t, 1, mock.getCopyCount())

	// Simulate a remote update: same repository, new digest and content.
	mock.seedComplypack(
		"example.com/complypacks/opa-bundle",
		"io.complytime.opa",
		"1.0.0",
		"sha256:digest_v2",
		"opa bundle content v2 — updated",
	)

	// Reload state from disk.
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	complypackCache2 := cache.NewComplypackCache(cacheDir, state2)
	syncMgr2 := cache.NewComplypackSync(complypackCache2, state2, mock, cacheDir)

	// Second sync — digest changed, should re-fetch.
	fetched2, err := syncMgr2.SyncComplypack(context.Background(), "example.com/complypacks/opa-bundle", "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched2, "digest change should report a fetch occurred")
	assert.Equal(t, 2, mock.getCopyCount(),
		"digest change should trigger a second CopyComplypack call")

	// Verify state reflects the new digest.
	state3, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state3.GetComplypackState("example.com/complypacks/opa-bundle")
	require.True(t, ok)
	assert.Equal(t, "sha256:digest_v2", ps.Digest, "state should reflect the updated digest")
}

// TestComplypackSync_InvalidEvaluatorID verifies that a complypack with a
// malicious evaluator-id (e.g., "../../evil") is rejected during the Store
// step. The path traversal attempt must not escape the cache directory.
func TestComplypackSync_InvalidEvaluatorID(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := newMockComplypackSource()

	// Seed with a safe evaluator-id so Pack() succeeds in the mock,
	// but override the evaluator-id in CopyComplypack to inject the
	// malicious value. We use a custom mock for this test case.
	maliciousMock := &maliciousEvaluatorMock{
		base: mock,
	}
	mock.seedComplypack(
		"example.com/complypacks/evil",
		"io.complytime.opa", // safe ID for Pack() validation
		"1.0.0",
		"sha256:evil123",
		"evil content",
	)

	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	complypackCache := cache.NewComplypackCache(cacheDir, state)

	syncMgr := cache.NewComplypackSync(complypackCache, state, maliciousMock, cacheDir)

	_, err = syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/evil", "1.0.0")
	require.Error(t, err, "sync should fail for malicious evaluator-id")
	assert.Contains(t, err.Error(), "invalid evaluator-id",
		"error should indicate the evaluator-id is invalid")

	// Verify no directory was created outside the cache.
	evilPath := filepath.Join(cacheDir, "..", "evil")
	assert.NoDirExists(t, evilPath, "path traversal must not create directories outside cache")
}

// maliciousEvaluatorMock wraps a mockComplypackSource but overrides the
// evaluator-id in the packed artifact config to inject a path traversal value.
// complypack v0.0.8 validates Config at Pack() time, so we bypass Pack()
// entirely and construct the OCI artifact manually to simulate receiving
// a maliciously crafted artifact from an untrusted registry.
type maliciousEvaluatorMock struct {
	base *mockComplypackSource
}

func (m *maliciousEvaluatorMock) DefinitionVersion(ctx context.Context, repository string) (string, string, error) {
	return m.base.DefinitionVersion(ctx, repository)
}

func (m *maliciousEvaluatorMock) CopyComplypack(ctx context.Context, _, tag string, dst *ocistore.Store) (ocispec.Descriptor, error) {
	// Bypass complypack.Pack() validation by constructing the OCI artifact
	// manually. This simulates a malicious registry serving a complypack
	// with a path traversal evaluator-id.
	cfg := complypack.Config{
		ID:          "io.complytime.evil-pack",
		EvaluatorID: "../../evil",
		Version:     "1.0.0",
	}
	configData, err := json.Marshal(cfg)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshaling config: %w", err)
	}
	configDesc := ocispec.Descriptor{
		MediaType: complypack.MediaTypeConfig,
		Digest:    digest.FromBytes(configData),
		Size:      int64(len(configData)),
	}
	if err := dst.Push(ctx, configDesc, bytes.NewReader(configData)); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("pushing config: %w", err)
	}

	contentData := []byte("evil content")
	contentDesc := ocispec.Descriptor{
		MediaType: complypack.MediaTypeContent,
		Digest:    digest.FromBytes(contentData),
		Size:      int64(len(contentData)),
	}
	if err := dst.Push(ctx, contentDesc, bytes.NewReader(contentData)); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("pushing content: %w", err)
	}

	manifestDesc, err := oras.PackManifest(ctx, dst,
		oras.PackManifestVersion1_1,
		complypack.MediaTypeArtifact,
		oras.PackManifestOptions{
			ConfigDescriptor: &configDesc,
			Layers:           []ocispec.Descriptor{contentDesc},
		})
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("packing manifest: %w", err)
	}

	if err := dst.Tag(ctx, manifestDesc, tag); err != nil {
		return ocispec.Descriptor{}, err
	}

	return manifestDesc, nil
}

// TestComplypackSync_EmptyVersion_ResolvesToRemote verifies that when version
// is empty (""), the remote version from DefinitionVersion is used for state
// storage and cache directory naming — not the empty string.
func TestComplypackSync_EmptyVersion_ResolvesToRemote(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := newMockComplypackSource()
	mock.seedComplypack(
		"example.com/complypacks/resolve-empty",
		"io.complytime.resolve.empty",
		"3.2.1",
		"sha256:resolve-empty-digest",
		"content for empty version resolution",
	)

	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	complypackCache := cache.NewComplypackCache(cacheDir, state)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock, cacheDir)

	// Pass empty version — should resolve to "3.2.1" from the remote.
	fetched, err := syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/resolve-empty", "")
	require.NoError(t, err)
	assert.True(t, fetched, "sync with empty version should fetch")

	// Verify state records the resolved remote version, not empty string.
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state2.GetComplypackState("example.com/complypacks/resolve-empty")
	require.True(t, ok, "complypack state should exist after sync")
	assert.Equal(t, "3.2.1", ps.Version,
		"state version should be the resolved remote version, not empty")
	assert.Equal(t, "sha256:resolve-empty-digest", ps.Digest)

	// Verify cache has content at the evaluator-id path.
	contentPath, cfg, err := complypackCache.Lookup("io.complytime.resolve.empty", "3.2.1")
	require.NoError(t, err)
	assert.FileExists(t, contentPath,
		"cache should have content at the resolved version path")
	assert.Equal(t, "io.complytime.resolve.empty", cfg.EvaluatorID)
	assert.Equal(t, "3.2.1", cfg.Version)
}

// TestComplypackSync_LatestVersion_ResolvesToRemote verifies that when version
// is "latest", the remote version from DefinitionVersion is used for state
// storage and cache directory naming — not the literal string "latest".
func TestComplypackSync_LatestVersion_ResolvesToRemote(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := newMockComplypackSource()
	mock.seedComplypack(
		"example.com/complypacks/resolve-latest",
		"io.complytime.resolve.latest",
		"5.0.0",
		"sha256:resolve-latest-digest",
		"content for latest version resolution",
	)

	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	complypackCache := cache.NewComplypackCache(cacheDir, state)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock, cacheDir)

	// Pass "latest" — should resolve to "5.0.0" from the remote.
	fetched, err := syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/resolve-latest", "latest")
	require.NoError(t, err)
	assert.True(t, fetched, "sync with 'latest' version should fetch")

	// Verify state records the resolved remote version, not "latest".
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state2.GetComplypackState("example.com/complypacks/resolve-latest")
	require.True(t, ok, "complypack state should exist after sync")
	assert.Equal(t, "5.0.0", ps.Version,
		"state version should be the resolved remote version, not 'latest'")
	assert.Equal(t, "sha256:resolve-latest-digest", ps.Digest)

	// Verify cache has content at the evaluator-id path.
	contentPath, cfg, err := complypackCache.Lookup("io.complytime.resolve.latest", "5.0.0")
	require.NoError(t, err)
	assert.FileExists(t, contentPath,
		"cache should have content at the resolved version path")
	assert.Equal(t, "io.complytime.resolve.latest", cfg.EvaluatorID)
	assert.Equal(t, "5.0.0", cfg.Version)
}

// TestComplypackSync_EmptyRepository verifies that an empty repository string
// returns an error immediately without attempting any registry operations.
func TestComplypackSync_EmptyRepository(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := newMockComplypackSource()
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	complypackCache := cache.NewComplypackCache(cacheDir, state)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock, cacheDir)

	_, err = syncMgr.SyncComplypack(context.Background(), "", "1.0.0")
	require.Error(t, err, "empty repository should return an error")
	assert.Contains(t, err.Error(), "cannot be empty",
		"error should indicate the repository is empty")

	// Verify no state was written.
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	assert.Empty(t, state2.Complypacks, "no complypack state should exist after empty repository error")

	// Verify CopyComplypack was never called.
	assert.Equal(t, 0, mock.getCopyCount(),
		"CopyComplypack should not be called for empty repository")
}

// TestComplypackSync_UnpackFailure verifies that when CopyComplypack returns a
// valid descriptor but the OCI store has no actual content (so Unpack fails),
// the error wraps appropriately and state is not updated.
func TestComplypackSync_UnpackFailure(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	brokenMock := &brokenUnpackMock{}
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	complypackCache := cache.NewComplypackCache(cacheDir, state)

	syncMgr := cache.NewComplypackSync(complypackCache, state, brokenMock, cacheDir)

	_, err = syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/broken", "1.0.0")
	require.Error(t, err, "unpack failure should return an error")
	assert.Contains(t, err.Error(), "unpack",
		"error should indicate an unpack failure")

	// Verify state was NOT updated — a failed unpack must not record success.
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	_, exists := state2.GetComplypackState("example.com/complypacks/broken")
	assert.False(t, exists, "complypack state should not exist after unpack failure")
}

// brokenUnpackMock implements cache.ComplypackSource but returns a descriptor
// that points to non-existent content in the OCI store, causing Unpack to fail.
type brokenUnpackMock struct{}

func (m *brokenUnpackMock) DefinitionVersion(_ context.Context, _ string) (string, string, error) {
	return "sha256:broken-digest", "1.0.0", nil
}

func (m *brokenUnpackMock) CopyComplypack(_ context.Context, _, tag string, dst *ocistore.Store) (ocispec.Descriptor, error) {
	// Return a descriptor that references content not present in the store.
	// This simulates a corrupted or incomplete copy where the manifest
	// descriptor exists but the underlying blobs are missing.
	return ocispec.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Size:      0,
	}, nil
}

// TestComplypackSync_VPrefixedTag_StateMatchesDisk verifies that when the OCI
// tag uses a "v" prefix (e.g., "v1.0.0") but the complypack's embedded
// config.json uses bare semver (e.g., "1.0.0"), the state version matches the
// on-disk directory name (config.Version), not the OCI tag.
// Regression test for https://github.com/complytime/complyctl/issues/694.
func TestComplypackSync_VPrefixedTag_StateMatchesDisk(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := newMockComplypackSource()
	// OCI tag is "v1.0.0" but config.Version is "1.0.0" (no "v" prefix).
	// seedComplypackWithTag(repo, evaluatorID, tag, configVersion, digest, content)
	mock.seedComplypackWithTag(
		"example.com/complypacks/opa-bundle",
		"io.complytime.opa",
		"v1.0.0",
		"1.0.0",
		"sha256:vprefixed111",
		"opa bundle with v-prefixed tag",
	)

	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	complypackCache := cache.NewComplypackCache(cacheDir, state)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock, cacheDir)

	// Sync with the v-prefixed OCI tag.
	fetched, err := syncMgr.SyncComplypack(
		context.Background(),
		"example.com/complypacks/opa-bundle",
		"v1.0.0",
	)
	require.NoError(t, err)
	assert.True(t, fetched, "first sync should report a fetch occurred")

	// Verify state records config.Version ("1.0.0"), NOT the OCI tag ("v1.0.0").
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state2.GetComplypackState("example.com/complypacks/opa-bundle")
	require.True(t, ok, "complypack state should exist after sync")
	assert.Equal(t, "1.0.0", ps.Version,
		"state version should match config.Version (on-disk dir), not OCI tag")
	assert.Equal(t, "sha256:vprefixed111", ps.Digest)

	// Verify on-disk directory uses config.Version ("1.0.0").
	contentPath, cfg, err := complypackCache.Lookup("io.complytime.opa", "1.0.0")
	require.NoError(t, err)
	assert.FileExists(t, contentPath,
		"cache should exist at config.Version path (1.0.0)")
	assert.Equal(t, "io.complytime.opa", cfg.EvaluatorID)
	assert.Equal(t, "1.0.0", cfg.Version)

	// Verify NO directory exists at the OCI tag version ("v1.0.0").
	vPrefixedDir := filepath.Join(
		cacheDir, "complypacks", "io.complytime.opa", "v1.0.0",
	)
	assert.NoDirExists(t, vPrefixedDir,
		"directory should NOT exist at OCI tag path (v1.0.0)")

	// Verify incremental skip works: a second sync with the same digest
	// should be a no-op, confirming state ↔ filesystem consistency.
	state3, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	syncMgr2 := cache.NewComplypackSync(complypackCache, state3, mock, cacheDir)

	fetched2, err := syncMgr2.SyncComplypack(
		context.Background(),
		"example.com/complypacks/opa-bundle",
		"v1.0.0",
	)
	require.NoError(t, err)
	assert.False(t, fetched2,
		"second sync with same digest should skip (incremental)")
	assert.Equal(t, 1, mock.getCopyCount(),
		"CopyComplypack should only be called once across both syncs")
}
