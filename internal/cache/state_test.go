// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
