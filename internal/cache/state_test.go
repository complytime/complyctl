// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/cache/cachetest"
)

// --- EvaluatorIDToVersion tests ---

func TestEvaluatorIDToVersion_Found(t *testing.T) {
	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{
			"repo/opa": {
				EvaluatorID: "io.complytime.opa",
				Version:     "2.0.0",
			},
			"repo/ampel": {
				EvaluatorID: "io.complytime.ampel",
				Version:     "1.0.0",
			},
		},
	}

	version, ok, err := state.EvaluatorIDToVersion("io.complytime.opa")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "2.0.0", version)
}

func TestEvaluatorIDToVersion_NotFound(t *testing.T) {
	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{
			"repo/opa": {
				EvaluatorID: "io.complytime.opa",
				Version:     "2.0.0",
			},
		},
	}

	version, ok, err := state.EvaluatorIDToVersion("io.complytime.nonexistent")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, version)
}

func TestEvaluatorIDToVersion_EmptyComplypacks(t *testing.T) {
	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{},
	}

	version, ok, err := state.EvaluatorIDToVersion("io.complytime.opa")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, version)
}

func TestEvaluatorIDToVersion_NilComplypacks(t *testing.T) {
	state := &cache.State{
		Complypacks: nil,
	}

	version, ok, err := state.EvaluatorIDToVersion("io.complytime.opa")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, version)
}

func TestEvaluatorIDToVersion_NilReceiver(t *testing.T) {
	var state *cache.State

	version, ok, err := state.EvaluatorIDToVersion("io.complytime.opa")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, version)
}

func TestEvaluatorIDToVersion_DuplicateEvaluatorID(t *testing.T) {
	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{
			"repo/opa-a": {
				EvaluatorID: "io.complytime.opa",
				Version:     "1.0.0",
			},
			"repo/opa-b": {
				EvaluatorID: "io.complytime.opa",
				Version:     "2.0.0",
			},
		},
	}

	_, _, err := state.EvaluatorIDToVersion("io.complytime.opa")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate evaluator-id")
}

func TestPolicyState_BackwardCompatibility_NoVerifiedField(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a state.json without the "verified" field (simulating a
	// file from a previous version of complyctl).
	legacyState := map[string]interface{}{
		"last_sync": "2025-01-01T00:00:00Z",
		"policies": map[string]interface{}{
			"legacy-policy": map[string]interface{}{
				"version":      "v1.0.0",
				"digest":       "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
				"last_updated": "2025-01-01T00:00:00Z",
			},
		},
	}
	data, err := json.MarshalIndent(legacyState, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0600)
	require.NoError(t, err)

	loaded, err := cache.LoadState(tmpDir)
	require.NoError(t, err)
	ps, ok := loaded.GetPolicyState("legacy-policy")
	require.True(t, ok)
	assert.Equal(t, "v1.0.0", ps.Version)
	assert.Equal(t, "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", ps.Digest)
}

func TestPolicyState_JSONRoundTrip_WithMetadata(t *testing.T) {
	original := cache.PolicyState{
		Version:         "v2.0.0",
		Digest:          cachetest.DigestA,
		EvaluatorID:     "openscap",
		LastUpdated:     time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
		Verified:        true,
		SignerIdentity:  "user@example.com",
		Issuer:          "https://accounts.google.com",
		VerifiedAt:      time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
		PolicyTitle:     "CIS Fedora Linux - Level 1",
		PolicyEvaluator: "openscap",
		ControlCount:    42,
		AssessmentCount: 5,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var roundTripped cache.PolicyState
	err = json.Unmarshal(data, &roundTripped)
	require.NoError(t, err)

	assert.Equal(t, original, roundTripped)
}

func TestPolicyState_JSONBackwardCompatibility(t *testing.T) {
	// JSON from an older complyctl version that lacks metadata fields.
	oldJSON := `{
		"version": "v1.0.0",
		"digest": "sha256:3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d",
		"last_updated": "2025-06-01T00:00:00Z"
	}`

	var ps cache.PolicyState
	err := json.Unmarshal([]byte(oldJSON), &ps)
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", ps.Version)
	assert.Equal(t, "sha256:3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d", ps.Digest)
	// Metadata fields default to zero values.
	assert.Equal(t, "", ps.PolicyTitle)
	assert.Equal(t, "", ps.PolicyEvaluator)
	assert.Equal(t, 0, ps.ControlCount)
	assert.Equal(t, 0, ps.AssessmentCount)
}

func TestPolicyState_JSONRoundTrip_ZeroControlCount(t *testing.T) {
	// Verify that ControlCount:0 survives a JSON round-trip.
	// Without omitempty the field is explicitly serialized as 0,
	// so "metadata extracted with 0 controls" is distinguishable
	// from "metadata never extracted" after deserialization.
	original := cache.PolicyState{
		Version:         "v1.0.0",
		Digest:          cachetest.DigestC,
		PolicyTitle:     "Zero Controls Policy",
		PolicyEvaluator: "opa",
		ControlCount:    0,
		AssessmentCount: 1,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Confirm control_count is present in JSON (explicit zero).
	assert.Contains(t, string(data), `"control_count":0`)

	var roundTripped cache.PolicyState
	err = json.Unmarshal(data, &roundTripped)
	require.NoError(t, err)

	assert.Equal(t, 0, roundTripped.ControlCount)
	assert.Equal(t, "opa", roundTripped.PolicyEvaluator)
	assert.Equal(t, 1, roundTripped.AssessmentCount)
}

func TestState_SetPolicyMetadata_PreservesSyncFields(t *testing.T) {
	verifiedAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	lastUpdated := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)

	state := &cache.State{
		Policies: map[string]cache.PolicyState{
			"example.com/policies/cis-fedora": {
				Version:        "v3.1.0",
				Digest:         cachetest.DigestD,
				EvaluatorID:    "complypack-eval",
				LastUpdated:    lastUpdated,
				Verified:       true,
				SignerIdentity: "signer@example.com",
				Issuer:         "https://issuer.example.com",
				VerifiedAt:     verifiedAt,
			},
		},
	}

	state.SetPolicyMetadata(
		"example.com/policies/cis-fedora",
		"CIS Fedora Linux",
		"openscap",
		42,
		5,
	)

	ps, exists := state.GetPolicyState(
		"example.com/policies/cis-fedora",
	)
	require.True(t, exists)

	// Sync fields MUST be unchanged.
	assert.Equal(t, "v3.1.0", ps.Version)
	assert.Equal(t, cachetest.DigestD, ps.Digest)
	assert.Equal(t, "complypack-eval", ps.EvaluatorID)
	assert.Equal(t, lastUpdated, ps.LastUpdated)
	assert.True(t, ps.Verified)
	assert.Equal(t, "signer@example.com", ps.SignerIdentity)
	assert.Equal(t, "https://issuer.example.com", ps.Issuer)
	assert.Equal(t, verifiedAt, ps.VerifiedAt)

	// Metadata fields MUST be set.
	assert.Equal(t, "CIS Fedora Linux", ps.PolicyTitle)
	assert.Equal(t, "openscap", ps.PolicyEvaluator)
	assert.Equal(t, 42, ps.ControlCount)
	assert.Equal(t, 5, ps.AssessmentCount)
}

// TestSaveState_StateSeparation verifies that SaveState writes state.json
// to the data directory and not to the cache directory. This is regression
// protection for FR-004 "State survives cache clear": clearing the cache
// directory must not destroy state.json.
func TestSaveState_StateSeparation(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	state := &cache.State{
		Policies: map[string]cache.PolicyState{
			"example.com/policies/test": {
				Version: "v1.0.0",
				Digest:  cachetest.DigestE,
			},
		},
		Complypacks: make(map[string]cache.PolicyState),
	}

	// Save state to the data directory (not the cache directory).
	err := cache.SaveState(state, dataDir)
	require.NoError(t, err)

	// state.json MUST exist in the data directory.
	dataStatePath := filepath.Join(dataDir, "state.json")
	_, err = os.Stat(dataStatePath)
	require.NoError(t, err, "state.json must exist in the data directory")

	// state.json MUST NOT exist in the cache directory.
	cacheStatePath := filepath.Join(cacheDir, "state.json")
	_, err = os.Stat(cacheStatePath)
	require.True(t, os.IsNotExist(err), "state.json must not exist in the cache directory")

	// Verify the saved state can be loaded back from the data directory.
	loaded, err := cache.LoadState(dataDir)
	require.NoError(t, err)
	ps, exists := loaded.GetPolicyState("example.com/policies/test")
	require.True(t, exists)
	assert.Equal(t, "v1.0.0", ps.Version)
	assert.Equal(t, cachetest.DigestE, ps.Digest)
}

// TestSaveState_DirectoryPermissions verifies that SaveState creates the
// data directory with 0700 permissions (user-only access) per XDG Base
// Directory security requirements.
func TestSaveState_DirectoryPermissions(t *testing.T) {
	baseDir := t.TempDir()
	nestedDir := filepath.Join(baseDir, "xdg-data", "complytime")

	state := &cache.State{
		Policies:    map[string]cache.PolicyState{},
		Complypacks: map[string]cache.PolicyState{},
	}

	err := cache.SaveState(state, nestedDir)
	require.NoError(t, err)

	// Verify the created directory has 0700 permissions.
	info, err := os.Stat(nestedDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), info.Mode().Perm(),
		"data directory must have 0700 permissions")

	// Also verify state.json was written with 0600 permissions.
	stateInfo, err := os.Stat(filepath.Join(nestedDir, "state.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), stateInfo.Mode().Perm(),
		"state.json must have 0600 permissions")
}

func TestState_SetPolicyMetadata_NoOpForMissingKey(t *testing.T) {
	state := &cache.State{
		Policies: map[string]cache.PolicyState{
			"existing-policy": {
				Version: "v1.0.0",
				Digest:  cachetest.DigestF,
			},
		},
	}

	// Call with a key that does not exist — must not panic.
	state.SetPolicyMetadata(
		"nonexistent-policy",
		"Some Title",
		"ampel",
		10,
		3,
	)

	// Map must be unchanged: only the original entry exists.
	assert.Len(t, state.Policies, 1)
	_, exists := state.GetPolicyState("nonexistent-policy")
	assert.False(t, exists)

	// Original entry must be untouched.
	ps, exists := state.GetPolicyState("existing-policy")
	require.True(t, exists)
	assert.Equal(t, "v1.0.0", ps.Version)
	assert.Equal(t, cachetest.DigestF, ps.Digest)
}

// --- ValidateDigest tests (D4) ---

func TestValidateDigest(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid sha256", "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", false},
		{"valid sha384", "sha384:768412320f7b0aa5812fce428dc4706b3cae50e02a64caa16a782249bfe8efc4b7ef1ccb126255d196047dfedf17a0a9", false},
		{"valid sha512", "sha512:ee26b0dd4af7e749aa1a8ee3c10ae9923f618980772e473f8819a5d4940e0db27ac185f8a0e1d5f84f88bc887fd67b143732c304cc5fa9ad8e6f57f50028a8ff", false},
		{"empty string is allowed", "", false},
		{"missing colon", "sha256abc123", true},
		{"wrong hex length for sha256", "sha256:abc123", true},
		{"unsupported algorithm", "md5:d41d8cd98f00b204e9800998ecf8427e", true},
		{"no hex after colon", "sha256:", true},
		{"bare algorithm name", "sha256", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cache.ValidateDigest(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- UpdatePolicyStateWithVerification rejects malformed digest (D2) ---

func TestUpdatePolicyStateWithVerification_RejectsMalformedDigest(t *testing.T) {
	state := &cache.State{
		Policies: make(map[string]cache.PolicyState),
	}

	err := state.UpdatePolicyStateWithVerification("test-policy", "v1.0", "sha256:bad", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid digest")
	assert.Contains(t, err.Error(), "test-policy")

	// State must not be modified.
	_, exists := state.GetPolicyState("test-policy")
	assert.False(t, exists, "malformed digest must not be stored")
}

// --- UpdateComplypackStateWithVerification rejects malformed digest (D2) ---

func TestUpdateComplypackStateWithVerification_RejectsMalformedDigest(t *testing.T) {
	state := &cache.State{
		Complypacks: make(map[string]cache.PolicyState),
	}

	err := state.UpdateComplypackStateWithVerification("repo/pack", "v1.0", "sha256:bad", "opa", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid digest")
	assert.Contains(t, err.Error(), "repo/pack")

	// State must not be modified.
	_, exists := state.GetComplypackState("repo/pack")
	assert.False(t, exists, "malformed digest must not be stored")
}

// --- LoadState warns and excludes malformed digests (D1) ---

func TestLoadState_ExcludesMalformedDigests(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a state.json with a mix of valid, malformed, and empty digests.
	stateData := map[string]interface{}{
		"last_sync": time.Now().Format(time.RFC3339),
		"policies": map[string]interface{}{
			"valid-policy": map[string]interface{}{
				"version":      "v1.0",
				"digest":       "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
				"last_updated": time.Now().Format(time.RFC3339),
			},
			"malformed-policy": map[string]interface{}{
				"version":      "v2.0",
				"digest":       "sha256:bad",
				"last_updated": time.Now().Format(time.RFC3339),
			},
			"empty-digest-policy": map[string]interface{}{
				"version":      "v3.0",
				"digest":       "",
				"last_updated": time.Now().Format(time.RFC3339),
			},
		},
		"complypacks": map[string]interface{}{
			"valid-pack": map[string]interface{}{
				"version":      "v1.0",
				"digest":       "sha256:3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d",
				"evaluator_id": "opa",
				"last_updated": time.Now().Format(time.RFC3339),
			},
			"malformed-pack": map[string]interface{}{
				"version":      "v2.0",
				"digest":       "sha256:tooshort",
				"evaluator_id": "ampel",
				"last_updated": time.Now().Format(time.RFC3339),
			},
		},
	}

	data, err := json.MarshalIndent(stateData, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "state.json"), data, 0600))

	state, err := cache.LoadState(tmpDir)
	require.NoError(t, err, "LoadState must return nil error even with malformed entries")

	// Valid policy must be preserved.
	ps, exists := state.GetPolicyState("valid-policy")
	require.True(t, exists, "valid-policy must be preserved")
	assert.Equal(t, "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", ps.Digest)

	// Malformed policy must be excluded.
	_, exists = state.GetPolicyState("malformed-policy")
	assert.False(t, exists, "malformed-policy must be excluded")

	// Empty-digest policy must be preserved.
	ps, exists = state.GetPolicyState("empty-digest-policy")
	require.True(t, exists, "empty-digest-policy must be preserved")
	assert.Empty(t, ps.Digest)

	// Valid complypack must be preserved.
	ps, exists = state.GetComplypackState("valid-pack")
	require.True(t, exists, "valid-pack must be preserved")
	assert.Equal(t, "sha256:3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d", ps.Digest)

	// Malformed complypack must be excluded.
	_, exists = state.GetComplypackState("malformed-pack")
	assert.False(t, exists, "malformed-pack must be excluded")
}
