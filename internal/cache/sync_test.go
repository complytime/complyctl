// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/cache/cachetest"
	"github.com/complytime/complyctl/internal/registry"
)

func TestSync_CopyOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	mock.SeedPolicy("test-policy", "v1.0.0", "sha256:abc123")

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")

	fetched, err := sync.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)
	assert.True(t, fetched, "first sync should report a fetch")

	// Verify state was updated
	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state2.GetPolicyState("test-policy")
	assert.True(t, ok)
	assert.NotEmpty(t, ps.Digest)
	assert.NotEmpty(t, ps.Version)

	// Verify OCI Layout store exists (oci-layout marker and index.json)
	storePath := cacheMgr.PolicyStorePath("test-policy")
	assert.FileExists(t, filepath.Join(storePath, "oci-layout"))
	assert.FileExists(t, filepath.Join(storePath, "index.json"))
	assert.DirExists(t, filepath.Join(storePath, "blobs", "sha256"))
}

func TestSync_CopyOnSuccess_PinnedVersion(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	mock.SeedPolicy("test-policy", "v1.0.0", "sha256:abc123")

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")

	fetched, err := sync.SyncPolicy(context.Background(), "test-policy", "v1.0.0")
	require.NoError(t, err)
	assert.True(t, fetched, "first sync should report a fetch")

	assert.Equal(t, "test-policy:v1.0.0", mock.LastLookupRef,
		"source should receive the versioned ref when a pinned version is provided")

	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state2.GetPolicyState("test-policy")
	assert.True(t, ok)
	assert.Equal(t, "v1.0.0", ps.Version)
	assert.Equal(t, "sha256:abc123", ps.Digest)
}

func TestSync_FailureOnMissingPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")

	fetched, err := sync.SyncPolicy(context.Background(), "nonexistent-policy", "latest")
	require.Error(t, err)
	assert.False(t, fetched, "failed sync should not report a fetch")
	assert.True(t, errors.Is(err, registry.ErrVersionNotFound),
		"missing policy must return ErrVersionNotFound, got: %v", err)
	assert.NotContains(t, err.Error(), "registry unreachable",
		"missing policy must hit the 'not found' branch, not 'registry unreachable'")
}

func TestSync_RegistryUnreachableError(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	mock.SetError("unreachable-policy", fmt.Errorf("connection refused"))

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")

	fetched, err := sync.SyncPolicy(context.Background(), "unreachable-policy", "latest")
	require.Error(t, err)
	assert.False(t, fetched, "failed sync should not report a fetch")
	assert.Contains(t, err.Error(), "registry unreachable",
		"non-ErrVersionNotFound errors must hit the 'registry unreachable' branch")
	assert.Contains(t, err.Error(), "connection refused",
		"original error must be wrapped")
	assert.False(t, errors.Is(err, registry.ErrVersionNotFound),
		"registry unreachable must not be ErrVersionNotFound")
}

func TestSync_IncrementalSkip(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	mock.SeedPolicy("test-policy", "v1.0.0", "sha256:abc123")

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")

	// First sync
	fetched, err := sync.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)
	assert.True(t, fetched, "first sync should report a fetch")

	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state2.GetPolicyState("test-policy")
	require.True(t, ok)
	originalDigest := ps.Digest

	// Second sync with same digest — should be no-op (FR-004)
	sync2 := cache.NewSync(cacheMgr, state2, mock, cacheDir, "")
	fetched2, err := sync2.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)
	assert.False(t, fetched2, "incremental skip should not report a fetch")

	state3, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps3, _ := state3.GetPolicyState("test-policy")
	assert.Equal(t, originalDigest, ps3.Digest, "digest should not change for incremental sync")
}

func TestSync_EmptyPolicyID(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	cacheMgr := cache.NewCache(cacheDir)
	state, _ := cache.LoadState(cacheDir)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")
	fetched, err := sync.SyncPolicy(context.Background(), "", "latest")
	require.Error(t, err)
	assert.False(t, fetched, "empty policy ID should not report a fetch")
	assert.Contains(t, err.Error(), "policy ID cannot be empty")
}

func TestSync_RedownloadAfterDeletion(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	mock.SeedPolicy("test-policy", "v1.0.0", "sha256:abc123")

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")

	fetched, err := sync.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)
	assert.True(t, fetched, "first sync should report a fetch")

	storePath := cacheMgr.PolicyStorePath("test-policy")
	assert.FileExists(t, filepath.Join(storePath, "oci-layout"))

	require.NoError(t, os.RemoveAll(storePath))
	assert.NoDirExists(t, storePath)

	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	sync2 := cache.NewSync(cacheMgr, state2, mock, cacheDir, "")

	fetched2, err := sync2.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)
	assert.True(t, fetched2, "re-download after deletion should report a fetch")

	assert.FileExists(t, filepath.Join(storePath, "oci-layout"))
	assert.DirExists(t, filepath.Join(storePath, "blobs", "sha256"))
}

// TestSync_ConcurrentDifferentPolicies launches concurrent goroutines each
// syncing a distinct policy, verifying thread safety of shared state and
// cache structures under the race detector (SC-008/FR-006).
//
// Each worker uses its own cache directory to avoid state.json file-level
// contention: os.WriteFile truncates before writing, so a concurrent
// LoadState can observe a truncated file and fail with unexpected EOF.
// Isolation is the correct model; production callers do not share a single
// cache directory across concurrent sync managers.
func TestSync_ConcurrentDifferentPolicies(t *testing.T) {
	tmpDir := t.TempDir()

	const workers = 10

	mock := cachetest.NewMockPolicySource()
	for i := 0; i < workers; i++ {
		mock.SeedPolicy(
			fmt.Sprintf("policy-%d", i),
			"v1.0.0",
			fmt.Sprintf("sha256:digest%d", i),
		)
	}

	type workerResult struct {
		cacheDir string
		cacheMgr *cache.Cache
	}
	results := make([]workerResult, workers)

	for i := 0; i < workers; i++ {
		workerCacheDir := filepath.Join(tmpDir, fmt.Sprintf("cache-%d", i))
		require.NoError(t, os.MkdirAll(workerCacheDir, 0755))
		results[i] = workerResult{cacheDir: workerCacheDir, cacheMgr: cache.NewCache(workerCacheDir)}
	}

	var wg gosync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			r := results[workerID]
			state, loadErr := cache.LoadState(r.cacheDir)
			if loadErr != nil {
				t.Errorf("worker %d: state load failed: %v", workerID, loadErr)
				return
			}

			syncMgr := cache.NewSync(r.cacheMgr, state, mock, r.cacheDir, "")
			policyID := fmt.Sprintf("policy-%d", workerID)

			_, syncErr := syncMgr.SyncPolicy(context.Background(), policyID, "latest")
			if syncErr != nil {
				t.Errorf("worker %d: sync of %s failed: %v", workerID, policyID, syncErr)
			}
		}(i)
	}

	wg.Wait()

	// Final: each worker's state must be loadable and contain its own policy.
	for i := 0; i < workers; i++ {
		workerID := i
		r := results[workerID]
		policyID := fmt.Sprintf("policy-%d", workerID)

		finalState, err := cache.LoadState(r.cacheDir)
		require.NoError(t, err, "worker %d: final state must load without error", workerID)

		ps, ok := finalState.GetPolicyState(policyID)
		require.True(t, ok, "worker %d: policy %s must be in state after successful sync", workerID, policyID)
		assert.NotEmpty(t, ps.Digest, "%s must have a digest", policyID)
		assert.NotEmpty(t, ps.Version, "%s must have a version", policyID)

		storePath := r.cacheMgr.PolicyStorePath(policyID)
		assert.FileExists(t, filepath.Join(storePath, "oci-layout"),
			"%s OCI layout marker must exist", policyID)
	}
}

// TestSync_ConcurrentMixedFailures launches concurrent goroutines with a
// mix of valid and invalid policy syncs, verifying the mock and sync code
// handle concurrent access without data races.
//
// Each worker uses its own cache directory to avoid state.json file-level
// contention (see TestSync_ConcurrentDifferentPolicies for rationale).
func TestSync_ConcurrentMixedFailures(t *testing.T) {
	tmpDir := t.TempDir()

	const workers = 10

	mock := cachetest.NewMockPolicySource()
	// Seed only even-numbered policies; odd workers will fail.
	for i := 0; i < workers; i += 2 {
		mock.SeedPolicy(
			fmt.Sprintf("policy-%d", i),
			"v1.0.0",
			fmt.Sprintf("sha256:digest%d", i),
		)
	}

	cacheDirs := make([]string, workers)
	for i := 0; i < workers; i++ {
		workerCacheDir := filepath.Join(tmpDir, fmt.Sprintf("cache-%d", i))
		require.NoError(t, os.MkdirAll(workerCacheDir, 0755))
		cacheDirs[i] = workerCacheDir
	}

	var wg gosync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			workerCacheDir := cacheDirs[workerID]
			cacheMgr := cache.NewCache(workerCacheDir)
			state, loadErr := cache.LoadState(workerCacheDir)
			if loadErr != nil {
				t.Errorf("worker %d: state load failed: %v", workerID, loadErr)
				return
			}

			syncMgr := cache.NewSync(cacheMgr, state, mock, workerCacheDir, "")
			policyID := fmt.Sprintf("policy-%d", workerID)

			_, syncErr := syncMgr.SyncPolicy(context.Background(), policyID, "latest")

			if workerID%2 == 0 {
				if syncErr != nil {
					t.Errorf("worker %d: seeded policy sync must succeed: %v", workerID, syncErr)
				}
			} else {
				if syncErr == nil {
					t.Errorf("worker %d: unseeded policy sync must fail", workerID)
				}
			}
		}(i)
	}

	wg.Wait()

	// State must remain loadable after mixed concurrent operations.
	// Even-numbered workers (seeded) should have their policy in state.
	for i := 0; i < workers; i += 2 {
		workerID := i
		policyID := fmt.Sprintf("policy-%d", workerID)

		finalState, err := cache.LoadState(cacheDirs[workerID])
		require.NoError(t, err, "worker %d: final state must load without error", workerID)

		ps, ok := finalState.GetPolicyState(policyID)
		require.True(t, ok, "worker %d: policy %s must be in state after successful sync", workerID, policyID)
		assert.NotEmpty(t, ps.Digest, "%s must have a digest", policyID)
	}
}

func TestBuildLookupRef(t *testing.T) {
	tests := []struct {
		name         string
		registryHost string
		repository   string
		tag          string
		digest       string
		want         string
	}{
		// Backward-compatible cases: empty registryHost preserves existing behavior.
		{"tag version", "", "org/policy", "v1.0", "", "org/policy:v1.0"},
		{"sha256 digest", "", "org/policy", "", "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", "org/policy@sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"},
		{"sha512 digest", "", "org/policy", "", "sha512:def456", "org/policy@sha512:def456"},
		{"empty version", "", "org/policy", "", "", "org/policy"},
		{"latest version", "", "org/policy", "latest", "", "org/policy"},
		// New cases: non-empty registryHost prepends host prefix.
		{"host with tag", "registry.example.com", "policies/verify-keyed", "v1.0.0", "", "registry.example.com/policies/verify-keyed:v1.0.0"},
		{"host with digest", "registry.example.com", "policies/verify-keyed", "", "sha256:abc123", "registry.example.com/policies/verify-keyed@sha256:abc123"},
		{"host with empty version", "registry.example.com", "org/policy", "", "", "registry.example.com/org/policy"},
		{"host with latest", "registry.example.com", "org/policy", "latest", "", "registry.example.com/org/policy"},
		// Edge cases: port-based registries and trailing slash normalization.
		{"host with port and tag", "myregistry.io:5000", "org/policy", "v1.0.0", "", "myregistry.io:5000/org/policy:v1.0.0"},
		{"host with port and digest", "localhost:5000", "org/policy", "", "sha256:abc123", "localhost:5000/org/policy@sha256:abc123"},
		{"host with trailing slash", "registry.example.com/", "org/policy", "v1.0.0", "", "registry.example.com/org/policy:v1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cache.BuildLookupRef(tt.registryHost, tt.repository, tt.tag, tt.digest)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestBuildLookupRef_DigestPrecedence verifies that when both tag and
// digest are provided, digest takes precedence (OCI convention).
func TestBuildLookupRef_DigestPrecedence(t *testing.T) {
	got := cache.BuildLookupRef("", "org/policy", "v1.0", "sha256:abc123")
	assert.Equal(t, "org/policy@sha256:abc123", got)
}

// TestBuildLookupRef_SHA384Digest verifies sha384 digests are handled
// correctly through the typed fields.
func TestBuildLookupRef_SHA384Digest(t *testing.T) {
	sha384Digest := "sha384:" + "a" + strings.Repeat("b", 95)
	got := cache.BuildLookupRef("", "org/policy", "", sha384Digest)
	assert.Equal(t, "org/policy@"+sha384Digest, got)
}

// TestSync_SHA384Digest verifies that a sha384 digest version string is
// correctly classified and used as a digest (not a tag) in the sync path.
func TestSync_SHA384Digest(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	sha384Digest := "sha384:" + "a" + strings.Repeat("b", 95)
	lookupRef := "test-policy@" + sha384Digest
	mock := cachetest.NewMockPolicySource()
	// Seed with the digest-based lookup ref so DefinitionVersion succeeds,
	// and with the bare policyID so CopyPolicy can find it.
	mock.SeedPolicy(lookupRef, "v1.0.0", "sha256:abc123")
	mock.SeedPolicy("test-policy", "v1.0.0", "sha256:abc123")

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")

	// Pass a sha384 digest as the version string. classifyVersion must
	// detect it as a digest and BuildLookupRef must use "@" separator.
	_, err = sync.SyncPolicy(context.Background(), "test-policy", sha384Digest)
	require.NoError(t, err, "sync with sha384 digest must succeed")

	assert.Contains(t, mock.LastLookupRef, "@"+sha384Digest,
		"sha384 digest must use @ separator, not : separator")
	assert.NotContains(t, mock.LastLookupRef, ":"+sha384Digest,
		"sha384 digest must not be treated as a tag")
}

// TestBuildLookupRef_Regression_NoDoubleTag verifies that the original
// bug (issue #594) is prevented: a :tag in the repository must not
// produce a double-tagged reference like "repo:v0.4.0:v0.4.0".
func TestBuildLookupRef_Regression_NoDoubleTag(t *testing.T) {
	// When ParsePolicyRef correctly extracts the tag, Repository
	// will be "complytime/complypack-ampel-bp" and Tag "v0.4.0".
	// BuildLookupRef should produce a single-tagged reference.
	lookupRef := cache.BuildLookupRef("", "complytime/complypack-ampel-bp", "v0.4.0", "")
	assert.Equal(t, "complytime/complypack-ampel-bp:v0.4.0", lookupRef)
	assert.NotContains(t, lookupRef, ":v0.4.0:v0.4.0")
}

func TestSync_VerificationFailure_AbortsCopy(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	mock.SeedPolicy("registry.example.com/test-policy", "v1.0.0", "sha256:abc123")
	mock.SeedPolicy("test-policy", "v1.0.0", "sha256:abc123")

	failVerifier := func(_ context.Context, _ string) (*cache.VerificationResult, error) {
		return nil, fmt.Errorf("signature verification failed: identity mismatch")
	}

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "registry.example.com", cache.WithVerifier(failVerifier))

	_, err = sync.SyncPolicy(context.Background(), "test-policy", "latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verification failed")

	// Verify cache is unchanged — policy store should not exist
	assert.False(t, cacheMgr.PolicyStoreExists("test-policy"),
		"policy store must not exist after verification failure")

	// Verify state was not updated
	_, exists := state.GetPolicyState("test-policy")
	assert.False(t, exists, "state must not record policy after verification failure")
}

func TestSync_VerificationSuccess_RecordsMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	mock.SeedPolicy("registry.example.com/test-policy", "v1.0.0", "sha256:abc123")
	mock.SeedPolicy("test-policy", "v1.0.0", "sha256:abc123")

	successVerifier := func(_ context.Context, _ string) (*cache.VerificationResult, error) {
		return &cache.VerificationResult{
			Verified:       true,
			SignerIdentity: "workflow@github.com",
			Issuer:         "https://token.actions.githubusercontent.com",
			VerifiedAt:     time.Now(),
		}, nil
	}

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "registry.example.com", cache.WithVerifier(successVerifier))

	_, err = sync.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)

	// Verify state records verification metadata
	ps, exists := state.GetPolicyState("test-policy")
	require.True(t, exists)
	assert.True(t, ps.Verified)
	assert.Equal(t, "workflow@github.com", ps.SignerIdentity)
	assert.Equal(t, "https://token.actions.githubusercontent.com", ps.Issuer)
	assert.False(t, ps.VerifiedAt.IsZero())
}

func TestSync_NilVerifier_SkipsVerification(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	mock.SeedPolicy("test-policy", "v1.0.0", "sha256:abc123")

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	// No WithVerifier option — verification disabled
	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "")

	_, err = sync.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)

	// Sync succeeds but Verified is false (no verification performed)
	ps, exists := state.GetPolicyState("test-policy")
	require.True(t, exists)
	assert.False(t, ps.Verified, "Verified must be false when no verifier is configured")
	assert.Empty(t, ps.SignerIdentity)
}

// TestSync_RegistryHost_PassedToVerifier verifies that when a registryHost
// is configured on the Sync struct, the VerifyFunc receives a reference
// prefixed with that host — not Docker Hub's index.docker.io (regression
// test for the registry-host-resolution security defect).
func TestSync_RegistryHost_PassedToVerifier(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	// Seed with host-qualified ref for DefinitionVersion and bare policyID for CopyPolicy.
	mock.SeedPolicy("registry.example.com/org/test-policy", "v1.0.0", "sha256:abc123")
	mock.SeedPolicy("org/test-policy", "v1.0.0", "sha256:abc123")

	var capturedRef string
	capturingVerifier := func(_ context.Context, registryRef string) (*cache.VerificationResult, error) {
		capturedRef = registryRef
		return &cache.VerificationResult{Verified: true}, nil
	}

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "registry.example.com",
		cache.WithVerifier(capturingVerifier))

	_, err = sync.SyncPolicy(context.Background(), "org/test-policy", "latest")
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(capturedRef, "registry.example.com/"),
		"verifier ref must have registry host prefix, got: %s", capturedRef)
	assert.NotContains(t, capturedRef, "index.docker.io",
		"verifier ref must NOT resolve to Docker Hub")
}

// TestSync_EmptyRegistryHost_FailClosed verifies that when a verifier is
// configured but registryHost is empty, SyncPolicy returns a fail-closed
// error rather than silently resolving against Docker Hub.
func TestSync_EmptyRegistryHost_FailClosed(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()
	mock.SeedPolicy("org/test-policy", "v1.0.0", "sha256:abc123")

	neverCalledVerifier := func(_ context.Context, _ string) (*cache.VerificationResult, error) {
		t.Fatal("verifier must not be called when registryHost is empty")
		return nil, nil
	}

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock, cacheDir, "",
		cache.WithVerifier(neverCalledVerifier))

	_, err = sync.SyncPolicy(context.Background(), "org/test-policy", "latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry host",
		"fail-closed error must mention 'registry host'")
}
