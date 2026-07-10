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
				"digest":       "sha256:legacy",
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
	assert.Equal(t, "sha256:legacy", ps.Digest)
}

func TestPolicyState_JSONRoundTrip_WithMetadata(t *testing.T) {
	original := cache.PolicyState{
		Version:         "v2.0.0",
		Digest:          "sha256:abc123",
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
		"digest": "sha256:old",
		"last_updated": "2025-06-01T00:00:00Z"
	}`

	var ps cache.PolicyState
	err := json.Unmarshal([]byte(oldJSON), &ps)
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", ps.Version)
	assert.Equal(t, "sha256:old", ps.Digest)
	// Metadata fields default to zero values.
	assert.Equal(t, "", ps.PolicyTitle)
	assert.Equal(t, "", ps.PolicyEvaluator)
	assert.Equal(t, 0, ps.ControlCount)
	assert.Equal(t, 0, ps.AssessmentCount)
}

func TestPolicyState_JSONRoundTrip_ZeroControlCount(t *testing.T) {
	// Verify that ControlCount:0 with omitempty survives a
	// JSON round-trip. Since omitempty omits zero-value ints,
	// the field will be absent from the JSON, and unmarshalling
	// will produce 0 (Go zero value). This confirms the display
	// logic can safely treat 0 as "no controls" when other
	// metadata is present.
	original := cache.PolicyState{
		Version:         "v1.0.0",
		Digest:          "sha256:abc",
		PolicyTitle:     "Zero Controls Policy",
		PolicyEvaluator: "opa",
		ControlCount:    0,
		AssessmentCount: 1,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Confirm control_count is omitted from JSON.
	assert.NotContains(t, string(data), "control_count")

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
				Digest:         "sha256:syncdigest",
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
	assert.Equal(t, "sha256:syncdigest", ps.Digest)
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

func TestState_SetPolicyMetadata_NoOpForMissingKey(t *testing.T) {
	state := &cache.State{
		Policies: map[string]cache.PolicyState{
			"existing-policy": {
				Version: "v1.0.0",
				Digest:  "sha256:exists",
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
	assert.Equal(t, "sha256:exists", ps.Digest)
}
