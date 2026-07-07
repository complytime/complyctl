// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complypack/pkg/complypack"
)

// --- ValidatePathComponent tests ---

func TestValidatePathComponent_Valid(t *testing.T) {
	valid := []string{
		"io.complytime.opa",
		"1.0.0",
		"my-pack",
	}
	for _, v := range valid {
		t.Run(v, func(t *testing.T) {
			err := cache.ValidatePathComponent(v)
			assert.NoError(t, err, "expected %q to be accepted", v)
		})
	}
}

func TestValidatePathComponent_PathTraversal(t *testing.T) {
	cases := []string{
		"../../etc",
		"foo/bar",
		`foo\bar`,
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			err := cache.ValidatePathComponent(c)
			require.Error(t, err, "expected %q to be rejected", c)
		})
	}
}

func TestValidatePathComponent_NullByte(t *testing.T) {
	err := cache.ValidatePathComponent("foo\x00bar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "null bytes")
}

func TestValidatePathComponent_Empty(t *testing.T) {
	err := cache.ValidatePathComponent("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// --- ComplypackCache tests ---

// newTestConfig returns a complypack.Config suitable for testing.
func newTestConfig(evaluatorID, version string) complypack.Config {
	return complypack.Config{
		ID:          "test-" + evaluatorID,
		EvaluatorID: evaluatorID,
		Version:     version,
	}
}

func TestComplypackCache_Store_CreatesDirectoryStructure(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	cfg := newTestConfig("io.complytime.opa", "1.0.0")
	contentPath, err := cc.Store(cfg, strings.NewReader("test content"))
	require.NoError(t, err)

	// Verify content.tar.gz exists at the returned path.
	assert.FileExists(t, contentPath)
	assert.True(t, strings.HasSuffix(contentPath, "content.tar.gz"),
		"returned path should end with content.tar.gz, got %s", contentPath)

	// Verify config.json exists alongside content.tar.gz.
	dir := filepath.Dir(contentPath)
	assert.FileExists(t, filepath.Join(dir, "config.json"))

	// Verify the directory structure: {cacheDir}/complypacks/{evaluator-id}/{version}/
	expectedDir := filepath.Join(cacheDir, "complypacks", "io.complytime.opa", "1.0.0")
	assert.Equal(t, expectedDir, dir)
}

func TestComplypackCache_Store_AtomicWrite(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	// First store succeeds — establishes a valid cache entry.
	cfg := newTestConfig("io.complytime.opa", "1.0.0")
	_, err := cc.Store(cfg, strings.NewReader("good content"))
	require.NoError(t, err)

	// Second store with an invalid evaluator-id should fail validation
	// before writing anything. This verifies that a failed Store call
	// does not leave partial artifacts at the final cache path.
	badCfg := newTestConfig("../../evil", "1.0.0")
	_, err = cc.Store(badCfg, strings.NewReader("bad content"))
	require.Error(t, err)

	// The original entry must still be intact — no partial overwrite.
	expectedDir := filepath.Join(cacheDir, "complypacks", "io.complytime.opa", "1.0.0")
	assert.FileExists(t, filepath.Join(expectedDir, "content.tar.gz"))
	assert.FileExists(t, filepath.Join(expectedDir, "config.json"))

	// The evil path must not exist at all.
	evilDir := filepath.Join(cacheDir, "complypacks", "../../evil", "1.0.0")
	assert.NoDirExists(t, evilDir)
}

func TestComplypackCache_Store_EvictsOldVersions(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	// Store version 1.0.0.
	cfg1 := newTestConfig("io.complytime.opa", "1.0.0")
	_, err := cc.Store(cfg1, strings.NewReader("v1 content"))
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(cacheDir, "complypacks", "io.complytime.opa", "1.0.0"))

	// Store version 2.0.0 — should evict 1.0.0.
	cfg2 := newTestConfig("io.complytime.opa", "2.0.0")
	path2, err := cc.Store(cfg2, strings.NewReader("v2 content"))
	require.NoError(t, err)

	// Old version must be removed.
	assert.NoDirExists(t, filepath.Join(cacheDir, "complypacks", "io.complytime.opa", "1.0.0"))
	// New version must exist with correct content.
	assert.FileExists(t, path2)
	data2, err := os.ReadFile(path2)
	require.NoError(t, err)
	assert.Equal(t, "v2 content", string(data2))
}

func TestComplypackCache_Store_EvictsMultipleOldVersions(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	// Pre-seed three versions by creating directories with content directly.
	evalDir := filepath.Join(cacheDir, "complypacks", "io.complytime.opa")
	for _, v := range []string{"0.1.0", "0.2.0", "0.3.0"} {
		dir := filepath.Join(evalDir, v)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "content.tar.gz"), []byte("old"), 0600))
	}

	// Store version 1.0.0 — should evict all three old versions.
	cfg := newTestConfig("io.complytime.opa", "1.0.0")
	_, err := cc.Store(cfg, strings.NewReader("new content"))
	require.NoError(t, err)

	// All old versions must be removed.
	assert.NoDirExists(t, filepath.Join(evalDir, "0.1.0"))
	assert.NoDirExists(t, filepath.Join(evalDir, "0.2.0"))
	assert.NoDirExists(t, filepath.Join(evalDir, "0.3.0"))
	// New version must exist.
	assert.DirExists(t, filepath.Join(evalDir, "1.0.0"))
}

func TestComplypackCache_Store_DoesNotAffectOtherEvaluators(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	// Store opa/1.0.0 and ampel/1.0.0.
	cfgOpa := newTestConfig("opa", "1.0.0")
	_, err := cc.Store(cfgOpa, strings.NewReader("opa v1"))
	require.NoError(t, err)

	cfgAmpel := newTestConfig("ampel", "1.0.0")
	ampelPath, err := cc.Store(cfgAmpel, strings.NewReader("ampel v1"))
	require.NoError(t, err)

	// Store opa/2.0.0 — should evict opa/1.0.0 but not touch ampel.
	cfgOpa2 := newTestConfig("opa", "2.0.0")
	_, err = cc.Store(cfgOpa2, strings.NewReader("opa v2"))
	require.NoError(t, err)

	// ampel/1.0.0 must be untouched.
	assert.FileExists(t, ampelPath)
	assert.DirExists(t, filepath.Join(cacheDir, "complypacks", "ampel", "1.0.0"))

	// opa/1.0.0 must be evicted.
	assert.NoDirExists(t, filepath.Join(cacheDir, "complypacks", "opa", "1.0.0"))
	assert.DirExists(t, filepath.Join(cacheDir, "complypacks", "opa", "2.0.0"))
}

func TestComplypackCache_Store_SameVersionIdempotent(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	cfg := newTestConfig("io.complytime.opa", "1.0.0")

	// Store the same version twice — should succeed without error.
	_, err := cc.Store(cfg, strings.NewReader("first write"))
	require.NoError(t, err)

	path, err := cc.Store(cfg, strings.NewReader("second write"))
	require.NoError(t, err)

	// Content should reflect the second write (atomic replace).
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "second write", string(data))
}

func TestComplypackCache_Store_NoExistingDir(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	// Store with no prior evaluator directory — should succeed.
	cfg := newTestConfig("brand-new-evaluator", "1.0.0")
	path, err := cc.Store(cfg, strings.NewReader("fresh content"))
	require.NoError(t, err)

	assert.FileExists(t, path)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "fresh content", string(data))
}

func TestComplypackCache_Lookup_Found(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	cfg := newTestConfig("io.complytime.opa", "1.0.0")
	storedPath, err := cc.Store(cfg, strings.NewReader("test content"))
	require.NoError(t, err)

	contentPath, returnedCfg, err := cc.Lookup("io.complytime.opa", "1.0.0")
	require.NoError(t, err)

	// Verify the returned path matches what Store returned.
	assert.Equal(t, storedPath, contentPath)

	// Verify the returned config matches what was stored.
	assert.Equal(t, "io.complytime.opa", returnedCfg.EvaluatorID)
	assert.Equal(t, "1.0.0", returnedCfg.Version)
}

func TestComplypackCache_Lookup_NotFound(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	_, _, err := cc.Lookup("io.complytime.opa", "9.9.9")
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist,
		"lookup for missing complypack should wrap os.ErrNotExist")
}

// --- LookupByEvaluatorID tests ---

func TestComplypackCache_LookupByEvaluatorID_NotFound(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	// Lookup an evaluator that was never stored — should return empty path,
	// nil config, nil error (non-error "not found" contract).
	contentPath, cfg, err := cc.LookupByEvaluatorID("io.complytime.nonexistent")
	require.NoError(t, err, "missing evaluator should not return an error")
	assert.Empty(t, contentPath, "content path should be empty for missing evaluator")
	assert.Nil(t, cfg, "config should be nil for missing evaluator")
}

func TestComplypackCache_LookupByEvaluatorID_SkipsHiddenDirs(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	// Create a hidden directory that simulates an in-progress atomic write.
	// This mimics the .complypack-tmp-xxx directories created by Store().
	evalDir := filepath.Join(cacheDir, "complypacks", "io.complytime.opa")
	hiddenDir := filepath.Join(evalDir, ".complypack-tmp-abc123")
	require.NoError(t, os.MkdirAll(hiddenDir, 0755))

	// Place a content.tar.gz inside the hidden dir so it would match if
	// the hidden-dir filter were missing.
	require.NoError(t, os.WriteFile(
		filepath.Join(hiddenDir, "content.tar.gz"),
		[]byte("partial content"),
		0600,
	))

	// LookupByEvaluatorID must skip the hidden directory and return empty
	// results since no real version directory exists.
	contentPath, cfg, err := cc.LookupByEvaluatorID("io.complytime.opa")
	require.NoError(t, err, "hidden dirs should be silently skipped")
	assert.Empty(t, contentPath, "content path should be empty when only hidden dirs exist")
	assert.Nil(t, cfg, "config should be nil when only hidden dirs exist")
}

func TestComplypackCache_LookupByEvaluatorID_InvalidInput(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	// Path traversal input must be rejected by ValidatePathComponent.
	_, _, err := cc.LookupByEvaluatorID("../../evil")
	require.Error(t, err, "path traversal evaluator-id must be rejected")
	assert.Contains(t, err.Error(), "invalid evaluator-id")
}

// --- State complypack round-trip tests ---

func TestState_ComplypackRoundTrip(t *testing.T) {
	stateDir := t.TempDir()

	// Create state with complypack entries.
	state, err := cache.LoadState(stateDir)
	require.NoError(t, err)

	state.UpdateComplypackStateWithVerification("io.complytime.opa", "1.0.0", "sha256:abc123", "opa", nil)
	state.UpdateComplypackStateWithVerification("io.complytime.kyverno", "2.0.0", "sha256:def456", "kyverno", nil)

	err = cache.SaveState(state, stateDir)
	require.NoError(t, err)

	// Reload and verify.
	loaded, err := cache.LoadState(stateDir)
	require.NoError(t, err)

	ps1, ok := loaded.GetComplypackState("io.complytime.opa")
	require.True(t, ok, "expected io.complytime.opa to be present in loaded state")
	assert.Equal(t, "1.0.0", ps1.Version)
	assert.Equal(t, "sha256:abc123", ps1.Digest)
	assert.WithinDuration(t, time.Now(), ps1.LastUpdated, 5*time.Second)

	ps2, ok := loaded.GetComplypackState("io.complytime.kyverno")
	require.True(t, ok, "expected io.complytime.kyverno to be present in loaded state")
	assert.Equal(t, "2.0.0", ps2.Version)
	assert.Equal(t, "sha256:def456", ps2.Digest)
	assert.WithinDuration(t, time.Now(), ps2.LastUpdated, 5*time.Second)
}

func TestState_ComplypackLoadMissing_ReturnsEmpty(t *testing.T) {
	stateDir := t.TempDir()

	// Load from a directory with no state.json — should return empty but not nil.
	state, err := cache.LoadState(stateDir)
	require.NoError(t, err)

	assert.NotNil(t, state.Complypacks, "Complypacks map must not be nil")
	assert.Empty(t, state.Complypacks, "Complypacks map must be empty")

	// Verify no complypack state is found.
	_, ok := state.GetComplypackState("io.complytime.opa")
	assert.False(t, ok, "expected no complypack state for missing file")
}

// TestState_ComplypackLoadLegacy_InitializesMap verifies that loading a
// state.json written before the Complypacks field existed still initializes
// the Complypacks map to non-nil. This covers the nil-guard in LoadState.
func TestState_ComplypackLoadLegacy_InitializesMap(t *testing.T) {
	stateDir := t.TempDir()

	// Write a legacy state.json without the "complypacks" key.
	legacyJSON := `{
  "last_sync": "2025-01-01T00:00:00Z",
  "policies": {
    "nist": {"version": "v1.0", "digest": "sha256:abc", "last_updated": "2025-01-01T00:00:00Z"}
  }
}`
	statePath := filepath.Join(stateDir, "state.json")
	require.NoError(t, os.WriteFile(statePath, []byte(legacyJSON), 0600))

	state, err := cache.LoadState(stateDir)
	require.NoError(t, err)

	// Policies should be loaded from the file.
	ps, ok := state.GetPolicyState("nist")
	require.True(t, ok)
	assert.Equal(t, "v1.0", ps.Version)

	// Complypacks must be initialized to non-nil even though the key was absent.
	assert.NotNil(t, state.Complypacks, "Complypacks map must be initialized for legacy state files")
	assert.Empty(t, state.Complypacks)
}
