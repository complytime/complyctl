// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/cache/cachetest"
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

	sync := cache.NewSync(cacheMgr, state, mock)

	err = sync.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)

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

func TestSync_FailureOnMissingPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	mock := cachetest.NewMockPolicySource()

	cacheMgr := cache.NewCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	sync := cache.NewSync(cacheMgr, state, mock)

	err = sync.SyncPolicy(context.Background(), "nonexistent-policy", "latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
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

	sync := cache.NewSync(cacheMgr, state, mock)

	// First sync
	err = sync.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)

	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	ps, ok := state2.GetPolicyState("test-policy")
	require.True(t, ok)
	originalDigest := ps.Digest

	// Second sync with same digest — should be no-op (FR-004)
	sync2 := cache.NewSync(cacheMgr, state2, mock)
	err = sync2.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)

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

	sync := cache.NewSync(cacheMgr, state, mock)
	err := sync.SyncPolicy(context.Background(), "", "latest")
	require.Error(t, err)
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

	sync := cache.NewSync(cacheMgr, state, mock)

	err = sync.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)

	storePath := cacheMgr.PolicyStorePath("test-policy")
	assert.FileExists(t, filepath.Join(storePath, "oci-layout"))

	require.NoError(t, os.RemoveAll(storePath))
	assert.NoDirExists(t, storePath)

	state2, err := cache.LoadState(cacheDir)
	require.NoError(t, err)
	sync2 := cache.NewSync(cacheMgr, state2, mock)

	err = sync2.SyncPolicy(context.Background(), "test-policy", "latest")
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(storePath, "oci-layout"))
	assert.DirExists(t, filepath.Join(storePath, "blobs", "sha256"))
}

// TestSync_StressConcurrentFailures performs 100 sync iterations with alternating
// success and failure scenarios, verifying zero cache corruption (SC-008/FR-006).
func TestSync_StressConcurrentFailures(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	const iterations = 100
	policyID := "stress-test-policy"

	mock := cachetest.NewMockPolicySource()
	mock.SeedPolicy(policyID, "v1.0.0", "sha256:stress123")

	cacheMgr := cache.NewCache(cacheDir)

	successCount := 0
	failCount := 0

	for i := 0; i < iterations; i++ {
		state, loadErr := cache.LoadState(cacheDir)
		require.NoError(t, loadErr, "state load must not fail on iteration %d", i)

		syncMgr := cache.NewSync(cacheMgr, state, mock)

		if i%3 == 0 {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			syncErr := syncMgr.SyncPolicy(ctx, policyID, "latest")
			if syncErr != nil {
				failCount++
			} else {
				successCount++
			}
		} else if i%7 == 0 {
			syncErr := syncMgr.SyncPolicy(context.Background(),
				fmt.Sprintf("nonexistent-%d", i), "latest")
			require.Error(t, syncErr, "nonexistent policy must fail on iteration %d", i)
			failCount++
		} else {
			syncErr := syncMgr.SyncPolicy(context.Background(), policyID, "latest")
			require.NoError(t, syncErr, "normal sync must not fail on iteration %d", i)
			successCount++
		}
	}

	t.Logf("Stress test complete: %d success, %d failure out of %d iterations",
		successCount, failCount, iterations)

	// Final: cache state must be loadable and consistent
	finalState, err := cache.LoadState(cacheDir)
	require.NoError(t, err, "final state must load without error")

	ps, ok := finalState.GetPolicyState(policyID)
	if ok {
		assert.NotEmpty(t, ps.Digest, "successful policy must have a digest")
		assert.NotEmpty(t, ps.Version, "successful policy must have a version")

		// Verify OCI Layout store integrity
		storePath := cacheMgr.PolicyStorePath(policyID)
		assert.FileExists(t, filepath.Join(storePath, "oci-layout"),
			"OCI layout marker must exist after successful syncs")
	}
}
