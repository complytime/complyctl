// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
)

// --- resolveVerifier tests ---

// mockVerifyFunc returns a no-op VerifyFunc for testing. Each call
// returns a distinct func value so callers can compare identity.
func mockVerifyFunc() cache.VerifyFunc {
	return func(_ context.Context, _ string) (
		*cache.VerificationResult, error,
	) {
		return &cache.VerificationResult{Verified: true}, nil
	}
}

// installMockBuilder replaces defaultVerifierBuilder with a mock
// that records calls and returns a fresh VerifyFunc per config.
// Returns a pointer to the call count. Restores the original
// builder on test cleanup.
func installMockBuilder(t *testing.T) *int {
	t.Helper()
	orig := defaultVerifierBuilder
	t.Cleanup(func() { defaultVerifierBuilder = orig })

	callCount := 0
	defaultVerifierBuilder = func(
		_ complytime.VerificationConfig,
	) (cache.VerifyFunc, error) {
		callCount++
		return mockVerifyFunc(), nil
	}
	return &callCount
}

// installFailingBuilder replaces defaultVerifierBuilder with a mock
// that always returns an error. Restores the original on cleanup.
func installFailingBuilder(t *testing.T, errMsg string) {
	t.Helper()
	orig := defaultVerifierBuilder
	t.Cleanup(func() { defaultVerifierBuilder = orig })

	defaultVerifierBuilder = func(
		_ complytime.VerificationConfig,
	) (cache.VerifyFunc, error) {
		return nil, fmt.Errorf("%s", errMsg)
	}
}

func TestResolveVerifier_SkipVerify(t *testing.T) {
	calls := installMockBuilder(t)

	entry := complytime.PolicyEntry{
		URL:        "registry.example.com/policies/test:v1",
		SkipVerify: true,
	}
	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	opts, err := resolveVerifier(entry, wsCfg, vfCache)
	require.NoError(t, err)
	assert.Nil(t, opts,
		"skip_verify=true should return nil opts")
	assert.Equal(t, 0, *calls,
		"verifier constructor should not be called")
}

func TestResolveVerifier_EntryConfig(t *testing.T) {
	calls := installMockBuilder(t)

	entryCfg := &complytime.VerificationConfig{
		Key: "/path/to/entry.pub",
	}
	entry := complytime.PolicyEntry{
		URL:          "registry.example.com/policies/test:v1",
		Verification: entryCfg,
	}
	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	opts, err := resolveVerifier(entry, wsCfg, vfCache)
	require.NoError(t, err)
	require.NotNil(t, opts,
		"entry-level config should return sync opts")
	assert.Len(t, opts, 1)
	assert.Equal(t, 1, *calls,
		"constructor should be called once for entry config")

	// Verify the cache was populated with the entry config.
	_, cached := vfCache[*entryCfg]
	assert.True(t, cached,
		"entry config should be cached")
}

func TestResolveVerifier_WorkspaceConfig(t *testing.T) {
	calls := installMockBuilder(t)

	entry := complytime.PolicyEntry{
		URL: "registry.example.com/policies/test:v1",
		// No entry-level verification.
	}
	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	opts, err := resolveVerifier(entry, wsCfg, vfCache)
	require.NoError(t, err)
	require.NotNil(t, opts,
		"workspace config should return sync opts")
	assert.Len(t, opts, 1)
	assert.Equal(t, 1, *calls,
		"constructor should be called once for workspace config")

	// Verify the cache was populated with the workspace config.
	_, cached := vfCache[*wsCfg]
	assert.True(t, cached,
		"workspace config should be cached")
}

func TestResolveVerifier_NoConfig(t *testing.T) {
	calls := installMockBuilder(t)

	entry := complytime.PolicyEntry{
		URL: "registry.example.com/policies/test:v1",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	// nil workspace config, no entry config.
	opts, err := resolveVerifier(entry, nil, vfCache)
	require.NoError(t, err)
	assert.Nil(t, opts,
		"no config should return nil opts")
	assert.Equal(t, 0, *calls,
		"constructor should not be called")
}

func TestResolveVerifier_CacheHit(t *testing.T) {
	calls := installMockBuilder(t)

	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	entryA := complytime.PolicyEntry{
		URL: "registry.example.com/policies/a:v1",
	}
	entryB := complytime.PolicyEntry{
		URL: "registry.example.com/policies/b:v1",
	}

	// First call: cache miss → constructs verifier.
	optsA, err := resolveVerifier(entryA, wsCfg, vfCache)
	require.NoError(t, err)
	require.NotNil(t, optsA)
	assert.Equal(t, 1, *calls,
		"first call should invoke constructor")

	// Second call: cache hit → reuses verifier.
	optsB, err := resolveVerifier(entryB, wsCfg, vfCache)
	require.NoError(t, err)
	require.NotNil(t, optsB)
	assert.Equal(t, 1, *calls,
		"second call should reuse cached verifier (no new call)")
}

func TestResolveVerifier_CacheMiss(t *testing.T) {
	calls := installMockBuilder(t)

	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	// Entry with its own config (different from workspace).
	entryWithOwnCfg := complytime.PolicyEntry{
		URL: "registry.example.com/policies/vendor:v1",
		Verification: &complytime.VerificationConfig{
			Key: "/path/to/vendor.pub",
		},
	}
	// Entry inheriting workspace config.
	entryInheriting := complytime.PolicyEntry{
		URL: "registry.example.com/policies/standard:v1",
	}

	optsA, err := resolveVerifier(
		entryWithOwnCfg, wsCfg, vfCache,
	)
	require.NoError(t, err)
	require.NotNil(t, optsA)
	assert.Equal(t, 1, *calls)

	optsB, err := resolveVerifier(
		entryInheriting, wsCfg, vfCache,
	)
	require.NoError(t, err)
	require.NotNil(t, optsB)
	assert.Equal(t, 2, *calls,
		"different config should trigger a second constructor call")
}

func TestResolveVerifier_SkipVerifyFalseExplicit(t *testing.T) {
	calls := installMockBuilder(t)

	entry := complytime.PolicyEntry{
		URL:        "registry.example.com/policies/test:v1",
		SkipVerify: false, // explicit false = same as omitted
	}
	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	opts, err := resolveVerifier(entry, wsCfg, vfCache)
	require.NoError(t, err)
	require.NotNil(t, opts,
		"explicit false should behave like omitted "+
			"(inherit workspace)")
	assert.Equal(t, 1, *calls)
}

func TestResolveVerifier_EmptyVerification(t *testing.T) {
	calls := installMockBuilder(t)

	// Entry has a non-nil but unconfigured verification block.
	// IsConfigured() returns false → falls through to workspace.
	entry := complytime.PolicyEntry{
		URL:          "registry.example.com/policies/test:v1",
		Verification: &complytime.VerificationConfig{},
	}
	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	opts, err := resolveVerifier(entry, wsCfg, vfCache)
	require.NoError(t, err)
	require.NotNil(t, opts,
		"empty entry verification should fall through "+
			"to workspace config")
	assert.Equal(t, 1, *calls)

	// Verify it cached the workspace config, not the empty one.
	_, cached := vfCache[*wsCfg]
	assert.True(t, cached,
		"workspace config should be cached when entry is empty")
}

func TestResolveVerifier_BuilderError(t *testing.T) {
	installFailingBuilder(t, "TUF root unavailable")

	entry := complytime.PolicyEntry{
		URL: "registry.example.com/policies/test:v1",
	}
	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	opts, err := resolveVerifier(entry, wsCfg, vfCache)
	require.Error(t, err)
	assert.Nil(t, opts)
	assert.Contains(t, err.Error(), "TUF root unavailable")
	assert.Contains(t, err.Error(), "--skip-verify to bypass")
}

func TestResolveVerifier_NilCache(t *testing.T) {
	calls := installMockBuilder(t)

	// When cache is nil (--skip-verify path), even if entry has
	// its own verification config, resolveVerifier returns nil.
	// This tests the defensive guard.
	entry := complytime.PolicyEntry{
		URL: "registry.example.com/policies/test:v1",
		Verification: &complytime.VerificationConfig{
			Key: "/path/to/key.pub",
		},
	}

	opts, err := resolveVerifier(entry, nil, nil)
	require.NoError(t, err)
	assert.Nil(t, opts)
	assert.Equal(t, 0, *calls)
}

// --- Error collection tests ---

// stubCredFunc is a no-op credential function for testing.
func stubCredFunc(_ context.Context, _ string) (
	auth.Credential, error,
) {
	return auth.EmptyCredential, nil
}

func TestSyncAllPolicies_AllSucceed(t *testing.T) {
	// With no real registry, every sync will fail. To test the
	// "all succeed" path, we'd need to mock the entire cache layer.
	// Instead, verify the contract: with zero policies, no error.
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	state := &cache.State{}

	err := syncAllPolicies(
		context.Background(), cacheMgr, state,
		stubCredFunc, nil, cacheDir, nil, vfCache,
	)
	assert.NoError(t, err)
}

func TestSyncAllPolicies_ErrorCollection(t *testing.T) {
	installMockBuilder(t)
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	state := &cache.State{}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	// Three policies — all will fail because there's no real
	// registry. The key assertion: all three errors are collected,
	// not just the first.
	policies := []complytime.PolicyEntry{
		{URL: "registry.example.com/policies/a:v1", ID: "a"},
		{URL: "registry.example.com/policies/b:v1", ID: "b"},
		{URL: "registry.example.com/policies/c:v1", ID: "c"},
	}

	err := syncAllPolicies(
		context.Background(), cacheMgr, state,
		stubCredFunc, policies, cacheDir, nil, vfCache,
	)
	require.Error(t, err)

	// errors.Join produces a multi-error; verify all three
	// policy IDs appear in the error message.
	errMsg := err.Error()
	assert.Contains(t, errMsg, "policies/a",
		"error for policy a should be collected")
	assert.Contains(t, errMsg, "policies/b",
		"error for policy b should be collected")
	assert.Contains(t, errMsg, "policies/c",
		"error for policy c should be collected")
}

func TestSyncAllPolicies_PartialFailure(t *testing.T) {
	installMockBuilder(t)
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	state := &cache.State{}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	// Two policies — both will fail at registry, but we verify
	// that the second is attempted (not short-circuited).
	policies := []complytime.PolicyEntry{
		{URL: "registry.example.com/policies/fail1:v1", ID: "f1"},
		{URL: "registry.example.com/policies/fail2:v1", ID: "f2"},
	}

	err := syncAllPolicies(
		context.Background(), cacheMgr, state,
		stubCredFunc, policies, cacheDir, nil, vfCache,
	)
	require.Error(t, err)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "policies/fail1")
	assert.Contains(t, errMsg, "policies/fail2")
}

func TestSyncAllPolicies_ErrorsUnwrappable(t *testing.T) {
	installMockBuilder(t)
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	state := &cache.State{}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	policies := []complytime.PolicyEntry{
		{URL: "registry.example.com/policies/x:v1", ID: "x"},
	}

	err := syncAllPolicies(
		context.Background(), cacheMgr, state,
		stubCredFunc, policies, cacheDir, nil, vfCache,
	)
	require.Error(t, err)

	// errors.Join returns an error implementing Unwrap() []error.
	// Verify individual errors are accessible.
	var joined interface{ Unwrap() []error }
	require.True(t, errors.As(err, &joined),
		"errors.Join should return unwrappable error")
	unwrapped := joined.Unwrap()
	assert.Len(t, unwrapped, 1,
		"should contain exactly one wrapped error")
	assert.Contains(t, err.Error(), "policies/x")
}

func TestSyncAllComplypacks_ErrorCollection(t *testing.T) {
	installMockBuilder(t)
	cacheDir := t.TempDir()
	state := &cache.State{}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	complypacks := []complytime.PolicyEntry{
		{
			URL: "registry.example.com/packs/a:v1",
			ID:  "cp-a",
		},
		{
			URL: "registry.example.com/packs/b:v1",
			ID:  "cp-b",
		},
	}

	err := syncAllComplypacks(
		context.Background(), state,
		stubCredFunc, complypacks, cacheDir, cacheDir, t.TempDir(),
		nil, vfCache,
	)
	require.Error(t, err)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "packs/a",
		"error for complypack a should be collected")
	assert.Contains(t, errMsg, "packs/b",
		"error for complypack b should be collected")
}

func TestSyncAllComplypacks_AllSucceed_Empty(t *testing.T) {
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)
	state := &cache.State{}

	// Empty complypack list → no errors.
	// Note: syncAllComplypacks is only called when len > 0,
	// but we test the function directly for completeness.
	err := syncAllComplypacks(
		context.Background(), state,
		stubCredFunc, nil, t.TempDir(), t.TempDir(), t.TempDir(),
		nil, vfCache,
	)
	assert.NoError(t, err)
}

func TestSyncAllComplypacks_ErrorsContainPolicyID(t *testing.T) {
	installMockBuilder(t)
	cacheDir := t.TempDir()
	state := &cache.State{}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	complypacks := []complytime.PolicyEntry{
		{
			URL: "registry.example.com/packs/vendor:v2",
			ID:  "vendor-pack",
		},
	}

	err := syncAllComplypacks(
		context.Background(), state,
		stubCredFunc, complypacks, cacheDir, cacheDir, t.TempDir(),
		nil, vfCache,
	)
	require.Error(t, err)

	// FR-006: error message MUST identify the failed entry.
	assert.Contains(t, err.Error(), "packs/vendor",
		"error should contain the repository identifier")
}

// TestResolveVerifier_EntryOverridesWorkspace verifies that when
// both entry and workspace configs are set, the entry config wins
// (D1: standalone blocks, no merging).
func TestResolveVerifier_EntryOverridesWorkspace(t *testing.T) {
	// Track which config was used by the builder.
	orig := defaultVerifierBuilder
	t.Cleanup(func() { defaultVerifierBuilder = orig })

	var receivedCfg complytime.VerificationConfig
	defaultVerifierBuilder = func(
		cfg complytime.VerificationConfig,
	) (cache.VerifyFunc, error) {
		receivedCfg = cfg
		return mockVerifyFunc(), nil
	}

	entryCfg := &complytime.VerificationConfig{
		Key: "/path/to/entry.pub",
	}
	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	entry := complytime.PolicyEntry{
		URL:          "registry.example.com/policies/test:v1",
		Verification: entryCfg,
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	opts, err := resolveVerifier(entry, wsCfg, vfCache)
	require.NoError(t, err)
	require.NotNil(t, opts)

	// Verify the builder received the entry config, not workspace.
	assert.Equal(t, "/path/to/entry.pub", receivedCfg.Key)
	assert.Empty(t, receivedCfg.Issuer)
	assert.Empty(t, receivedCfg.Identity)
}

// TestSyncAllPolicies_VerificationError verifies that a verifier
// construction error for one entry is collected alongside sync
// errors from other entries.
func TestSyncAllPolicies_VerificationError(t *testing.T) {
	installFailingBuilder(t, "mock TUF error")
	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	state := &cache.State{}
	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	policies := []complytime.PolicyEntry{
		{URL: "registry.example.com/policies/a:v1", ID: "a"},
		{
			URL:        "registry.example.com/policies/b:v1",
			ID:         "b",
			SkipVerify: true, // This one should not fail
		},
	}

	err := syncAllPolicies(
		context.Background(), cacheMgr, state,
		stubCredFunc, policies, cacheDir, wsCfg, vfCache,
	)
	require.Error(t, err)

	errMsg := err.Error()
	// Policy "a" should fail on verifier construction.
	assert.Contains(t, errMsg, "mock TUF error")
	// Policy "b" skips verification but still fails at registry.
	// Both errors should be collected.
	assert.Contains(t, errMsg, "policies/b",
		"policy b sync error should be collected")
}

// --- Integration tests (Phase 4, Tasks 4.1–4.4) ---

// installSelectiveBuilder replaces defaultVerifierBuilder with a mock
// that fails for a specific config and succeeds for all others.
// Returns a pointer to the call count. Restores the original on
// test cleanup.
func installSelectiveBuilder(
	t *testing.T,
	failCfg complytime.VerificationConfig,
	failErr string,
) *int {
	t.Helper()
	orig := defaultVerifierBuilder
	t.Cleanup(func() { defaultVerifierBuilder = orig })

	callCount := 0
	defaultVerifierBuilder = func(
		cfg complytime.VerificationConfig,
	) (cache.VerifyFunc, error) {
		callCount++
		if cfg == failCfg {
			return nil, fmt.Errorf("%s", failErr)
		}
		return mockVerifyFunc(), nil
	}
	return &callCount
}

// TestResolveVerifier_MixedConfigs verifies that entries with
// different verification configs (keyed, keyless entry-level,
// workspace-inherited) all resolve correctly with the expected
// number of constructor calls and cache entries within a single
// invocation.
func TestResolveVerifier_MixedConfigs(t *testing.T) {
	calls := installMockBuilder(t)

	keyedCfg := &complytime.VerificationConfig{
		Key: "/path/to/vendor.pub",
	}
	keylessCfg := &complytime.VerificationConfig{
		Issuer:   "https://entry-issuer.example.com",
		Identity: "entry@example.com",
	}
	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://ws-issuer.example.com",
		Identity: "workspace@example.com",
	}

	entries := []complytime.PolicyEntry{
		{
			URL:          "registry.example.com/policies/vendor:v1",
			ID:           "vendor",
			Verification: keyedCfg,
		},
		{
			URL:          "registry.example.com/policies/partner:v1",
			ID:           "partner",
			Verification: keylessCfg,
		},
		{
			URL: "registry.example.com/policies/internal:v1",
			ID:  "internal",
			// No entry-level config → inherits workspace.
		},
	}

	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	for _, entry := range entries {
		opts, err := resolveVerifier(entry, wsCfg, vfCache)
		require.NoError(t, err,
			"entry %s should resolve without error",
			entry.EffectiveID())
		require.NotNil(t, opts,
			"entry %s should return sync opts",
			entry.EffectiveID())
		assert.Len(t, opts, 1,
			"entry %s: exactly one sync option expected",
			entry.EffectiveID())
	}

	// Three distinct configs → three constructor calls.
	assert.Equal(t, 3, *calls,
		"constructor called once per distinct config")
	assert.Len(t, vfCache, 3,
		"cache should contain three distinct entries")

	// Verify each config is in the cache.
	_, hasKeyed := vfCache[*keyedCfg]
	assert.True(t, hasKeyed,
		"keyed config should be cached")
	_, hasKeyless := vfCache[*keylessCfg]
	assert.True(t, hasKeyless,
		"entry-level keyless config should be cached")
	_, hasWs := vfCache[*wsCfg]
	assert.True(t, hasWs,
		"workspace config should be cached")
}

// TestResolveVerifier_SkipVerifyWithVerifiedEntries verifies that
// a skip_verify entry returns nil opts while other entries in the
// same invocation get their verifiers resolved. The constructor
// is called only for the non-skipped entries.
func TestResolveVerifier_SkipVerifyWithVerifiedEntries(
	t *testing.T,
) {
	calls := installMockBuilder(t)

	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	entrySkip := complytime.PolicyEntry{
		URL:        "registry.example.com/policies/skip:v1",
		ID:         "skip",
		SkipVerify: true,
	}
	entryInherit := complytime.PolicyEntry{
		URL: "registry.example.com/policies/inherit:v1",
		ID:  "inherit",
	}
	entryKeyed := complytime.PolicyEntry{
		URL: "registry.example.com/policies/keyed:v1",
		ID:  "keyed",
		Verification: &complytime.VerificationConfig{
			Key: "/path/to/key.pub",
		},
	}

	// Resolve skip_verify entry — should return nil.
	optsSkip, err := resolveVerifier(
		entrySkip, wsCfg, vfCache,
	)
	require.NoError(t, err)
	assert.Nil(t, optsSkip,
		"skip_verify entry should return nil opts")

	// Resolve workspace-inheriting entry.
	optsInherit, err := resolveVerifier(
		entryInherit, wsCfg, vfCache,
	)
	require.NoError(t, err)
	require.NotNil(t, optsInherit,
		"inheriting entry should return sync opts")

	// Resolve entry-level keyed entry.
	optsKeyed, err := resolveVerifier(
		entryKeyed, wsCfg, vfCache,
	)
	require.NoError(t, err)
	require.NotNil(t, optsKeyed,
		"keyed entry should return sync opts")

	// Constructor called twice: workspace + keyed.
	// skip_verify does not trigger the constructor.
	assert.Equal(t, 2, *calls,
		"constructor called for inherited and keyed only")
	assert.Len(t, vfCache, 2,
		"cache should have two entries (skip excluded)")
}

// TestSyncAllPolicies_SelectiveVerificationError verifies that
// when one entry's verifier resolution fails (e.g., invalid key
// path), other entries still proceed to sync. All errors from
// both verification failures and sync failures are collected
// together.
func TestSyncAllPolicies_SelectiveVerificationError(
	t *testing.T,
) {
	failCfg := complytime.VerificationConfig{
		Key: "/invalid/path/bad.pub",
	}
	calls := installSelectiveBuilder(
		t, failCfg, "key file not found",
	)

	cacheDir := t.TempDir()
	cacheMgr := cache.NewCache(cacheDir)
	state := &cache.State{}

	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	policies := []complytime.PolicyEntry{
		{
			URL: "registry.example.com/policies/bad:v1",
			ID:  "bad",
			Verification: &complytime.VerificationConfig{
				Key: "/invalid/path/bad.pub",
			},
		},
		{
			URL: "registry.example.com/policies/good:v1",
			ID:  "good",
			// Inherits workspace keyless → succeeds at
			// verifier construction but fails at registry.
		},
		{
			URL:        "registry.example.com/policies/skip:v1",
			ID:         "skip",
			SkipVerify: true,
		},
	}

	err := syncAllPolicies(
		context.Background(), cacheMgr, state,
		stubCredFunc, policies, cacheDir, wsCfg, vfCache,
	)
	require.Error(t, err)

	errMsg := err.Error()

	// Entry "bad": verifier construction fails.
	assert.Contains(t, errMsg, "key file not found",
		"verifier error for policy 'bad' should be collected")
	assert.Contains(t, errMsg, "bad",
		"error should identify failing policy")

	// Entry "good": passes verification but fails at registry.
	assert.Contains(t, errMsg, "policies/good",
		"policy 'good' should be attempted and its sync "+
			"error collected")

	// Constructor called twice: once for failing keyed config,
	// once for workspace keyless config.
	assert.Equal(t, 2, *calls,
		"constructor called for bad (fails) and "+
			"good (succeeds)")

	// Verify errors are unwrappable (errors.Join).
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		unwrapped := joined.Unwrap()
		assert.GreaterOrEqual(t, len(unwrapped), 2,
			"should collect errors from multiple entries")
	}
}

// TestResolveVerifier_BackwardCompat_WithWorkspace verifies that
// entries without per-entry fields (no Verification, no
// SkipVerify) all receive the workspace-level verifier via a
// single constructor call — matching pre-feature behavior.
func TestResolveVerifier_BackwardCompat_WithWorkspace(
	t *testing.T,
) {
	calls := installMockBuilder(t)

	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	// Three entries — none have Verification or SkipVerify.
	entries := []complytime.PolicyEntry{
		{
			URL: "registry.example.com/policies/a:v1",
			ID:  "a",
		},
		{
			URL: "registry.example.com/policies/b:v1",
			ID:  "b",
		},
		{
			URL: "registry.example.com/policies/c:v1",
			ID:  "c",
		},
	}

	for _, entry := range entries {
		opts, err := resolveVerifier(entry, wsCfg, vfCache)
		require.NoError(t, err)
		require.NotNil(t, opts,
			"entry %s should get workspace verifier",
			entry.EffectiveID())
		assert.Len(t, opts, 1)
	}

	// All three share workspace config → one constructor call.
	assert.Equal(t, 1, *calls,
		"constructor called once for shared workspace config")
	assert.Len(t, vfCache, 1,
		"cache should contain one entry (workspace config)")
}

// TestResolveVerifier_BackwardCompat_NoWorkspace verifies that
// entries without per-entry fields and no workspace-level
// verification produce nil opts for all entries — matching
// pre-feature behavior when no verification was configured.
func TestResolveVerifier_BackwardCompat_NoWorkspace(
	t *testing.T,
) {
	calls := installMockBuilder(t)
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	// Three entries — no per-entry config, no workspace config.
	entries := []complytime.PolicyEntry{
		{
			URL: "registry.example.com/policies/a:v1",
			ID:  "a",
		},
		{
			URL: "registry.example.com/policies/b:v1",
			ID:  "b",
		},
		{
			URL: "registry.example.com/policies/c:v1",
			ID:  "c",
		},
	}

	for _, entry := range entries {
		opts, err := resolveVerifier(entry, nil, vfCache)
		require.NoError(t, err)
		assert.Nil(t, opts,
			"entry %s should get nil opts without config",
			entry.EffectiveID())
	}

	assert.Equal(t, 0, *calls,
		"constructor should not be called")
	assert.Empty(t, vfCache,
		"cache should remain empty")
}

// --- Builder Passthrough Tests ---

// TestBuildVerifierFromConfig_PassesTrustedRoot verifies that
// buildVerifierFromConfig passes cfg.TrustedRoot to
// NewKeylessVerifier when keyless verification is configured.
func TestResolveVerifier_PassesTrustedRootToBuilder(
	t *testing.T,
) {
	// Capture the config received by the builder to verify
	// TrustedRoot is passed through.
	orig := defaultVerifierBuilder
	t.Cleanup(func() { defaultVerifierBuilder = orig })

	var receivedCfg complytime.VerificationConfig
	defaultVerifierBuilder = func(
		cfg complytime.VerificationConfig,
	) (cache.VerifyFunc, error) {
		receivedCfg = cfg
		return mockVerifyFunc(), nil
	}

	entry := complytime.PolicyEntry{
		URL: "registry.example.com/policies/test:v1",
		Verification: &complytime.VerificationConfig{
			Issuer:      "https://issuer.example.com",
			Identity:    "user@example.com",
			TrustedRoot: "/path/to/trusted_root.json",
		},
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	opts, err := resolveVerifier(entry, nil, vfCache)
	require.NoError(t, err)
	require.NotNil(t, opts)

	// Verify the builder received the TrustedRoot field.
	assert.Equal(t, "/path/to/trusted_root.json",
		receivedCfg.TrustedRoot,
		"TrustedRoot should be passed through to builder")
	assert.Equal(t, "https://issuer.example.com",
		receivedCfg.Issuer)
	assert.Equal(t, "user@example.com",
		receivedCfg.Identity)
	assert.Empty(t, receivedCfg.Key,
		"Key should be empty for keyless config")
}

// TestBuildVerifierFromConfig_EmptyTrustedRoot verifies that
// buildVerifierFromConfig passes empty TrustedRoot to
// NewKeylessVerifier when only issuer + identity are set.
func TestResolveVerifier_EmptyTrustedRootPassedToBuilder(
	t *testing.T,
) {
	orig := defaultVerifierBuilder
	t.Cleanup(func() { defaultVerifierBuilder = orig })

	var receivedCfg complytime.VerificationConfig
	defaultVerifierBuilder = func(
		cfg complytime.VerificationConfig,
	) (cache.VerifyFunc, error) {
		receivedCfg = cfg
		return mockVerifyFunc(), nil
	}

	entry := complytime.PolicyEntry{
		URL: "registry.example.com/policies/test:v1",
		Verification: &complytime.VerificationConfig{
			Issuer:   "https://issuer.example.com",
			Identity: "user@example.com",
			// No TrustedRoot — empty string.
		},
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	opts, err := resolveVerifier(entry, nil, vfCache)
	require.NoError(t, err)
	require.NotNil(t, opts)

	assert.Empty(t, receivedCfg.TrustedRoot,
		"TrustedRoot should be empty when not configured")
}

// TestSyncAllComplypacks_SelectiveVerificationError verifies
// error collection through the complypack sync layer when one
// entry's verifier resolution fails selectively.
func TestSyncAllComplypacks_SelectiveVerificationError(
	t *testing.T,
) {
	failCfg := complytime.VerificationConfig{
		Key: "/invalid/complypack.pub",
	}
	calls := installSelectiveBuilder(
		t, failCfg, "complypack key not found",
	)

	cacheDir := t.TempDir()
	state := &cache.State{}

	wsCfg := &complytime.VerificationConfig{
		Issuer:   "https://issuer.example.com",
		Identity: "user@example.com",
	}
	vfCache := make(
		map[complytime.VerificationConfig]cache.VerifyFunc,
	)

	complypacks := []complytime.PolicyEntry{
		{
			URL: "registry.example.com/packs/bad:v1",
			ID:  "cp-bad",
			Verification: &complytime.VerificationConfig{
				Key: "/invalid/complypack.pub",
			},
		},
		{
			URL: "registry.example.com/packs/good:v1",
			ID:  "cp-good",
			// Inherits workspace keyless → succeeds at
			// verifier construction but fails at registry.
		},
	}

	err := syncAllComplypacks(
		context.Background(), state,
		stubCredFunc, complypacks, cacheDir, cacheDir, t.TempDir(),
		wsCfg, vfCache,
	)
	require.Error(t, err)

	errMsg := err.Error()

	// Complypack "cp-bad": verifier construction fails.
	assert.Contains(t, errMsg, "complypack key not found",
		"verifier error for complypack should be collected")

	// Complypack "cp-good": passes verification, fails sync.
	assert.Contains(t, errMsg, "packs/good",
		"complypack 'good' should be attempted")

	// Constructor called twice.
	assert.Equal(t, 2, *calls,
		"constructor called for both configs")
}
