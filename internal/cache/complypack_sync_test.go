// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	ocistore "oras.land/oras-go/v2/content/oci"

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

// TestComplypackSync_FetchAndStore verifies the full fetch-unpack-store pipeline:
// a valid complypack artifact is created via complypack.Pack() in the mock source,
// synced through ComplypackSync, and the resulting cache contains the expected
// content.tar.gz and config.json files.
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

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)

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

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)

	// First sync — should fetch and store.
	fetched, err := syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/opa-bundle", "1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched, "first sync should report a fetch occurred")
	assert.Equal(t, 1, mock.getCopyCount(), "first sync should call CopyComplypack once")

	// Reload state from disk (as production code does between syncs).
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr2 := cache.NewComplypackSync(complypackCache, state2, mock)

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

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)

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

	syncMgr2 := cache.NewComplypackSync(complypackCache, state2, mock)

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

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)

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

	syncMgr2 := cache.NewComplypackSync(complypackCache, state2, mock)

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

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, maliciousMock)

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
// complypack.Pack() validates Config, so we pack with a safe ID and then
// re-pack with the malicious ID by manipulating the OCI store content.
// Instead, we use a simpler approach: pack with a safe config, then the
// Unpack result will have the safe config. We override CopyComplypack to
// pack an artifact whose config has the malicious evaluator-id.
//
// Since complypack.Config.Validate() rejects empty evaluator-ids but does
// NOT reject path traversal characters (that's complyctl's responsibility),
// we can pack with "../../evil" directly.
type maliciousEvaluatorMock struct {
	base *mockComplypackSource
}

func (m *maliciousEvaluatorMock) DefinitionVersion(ctx context.Context, repository string) (string, string, error) {
	return m.base.DefinitionVersion(ctx, repository)
}

func (m *maliciousEvaluatorMock) CopyComplypack(ctx context.Context, repository, tag string, dst *ocistore.Store) (ocispec.Descriptor, error) {
	// Pack with the malicious evaluator-id. complypack.Config.Validate()
	// checks for empty fields but does not enforce path safety — that's
	// the consumer's (complyctl's) responsibility.
	cfg := complypack.Config{
		ID:          "evil-pack",
		EvaluatorID: "../../evil",
		Version:     "1.0.0",
	}

	desc, err := complypack.Pack(ctx, dst, cfg, strings.NewReader("evil content"))
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	if err := dst.Tag(ctx, desc, tag); err != nil {
		return ocispec.Descriptor{}, err
	}

	return desc, nil
}

// TestComplypackSync_UnsignedWarning verifies that the sync pipeline completes
// successfully for an unsigned artifact. Signature verification warnings are
// logged in get.go (the CLI layer), not in the sync pipeline itself. This test
// confirms the end-to-end sync works without signature verification options.
func TestComplypackSync_UnsignedWarning(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := newMockComplypackSource()
	// No signing options — artifact is unsigned.
	mock.seedComplypack(
		"example.com/complypacks/unsigned-pack",
		"io.complytime.unsigned",
		"2.0.0",
		"sha256:unsigned999",
		"unsigned policy content",
	)

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)

	// Sync should succeed — no signature verification in the sync layer.
	fetched, err := syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/unsigned-pack", "2.0.0")
	require.NoError(t, err, "unsigned complypack should sync successfully")
	assert.True(t, fetched, "first sync should report a fetch occurred")

	// Verify the artifact was cached correctly.
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state2.GetComplypackState("example.com/complypacks/unsigned-pack")
	assert.True(t, ok, "complypack state should exist")
	assert.Equal(t, "sha256:unsigned999", ps.Digest)
	assert.Equal(t, "2.0.0", ps.Version)

	// Verify cache files exist.
	contentPath, cfg, err := complypackCache.Lookup("io.complytime.unsigned", "2.0.0")
	require.NoError(t, err)
	assert.FileExists(t, contentPath)
	assert.Equal(t, "io.complytime.unsigned", cfg.EvaluatorID)
	assert.Equal(t, "2.0.0", cfg.Version)
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

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)

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

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)

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
	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)

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
	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, brokenMock)

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

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)

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
	syncMgr2 := cache.NewComplypackSync(complypackCache, state3, mock)

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
