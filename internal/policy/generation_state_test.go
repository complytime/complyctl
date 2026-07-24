// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T152: SaveGenerationState + LoadGenerationState round-trip ---

func TestGenerationState_SaveLoadRoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	state := &GenerationState{
		PolicyID:          "nist-800-53-r5",
		PolicyDigest:      "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		ComplypackDigests: map[string]string{"opa": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "ampel": "sha256:2222222222222222222222222222222222222222222222222222222222222222"},
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		EvaluatorIDs:      []string{"openscap", "kube-eval"},
	}

	err := SaveGenerationState(baseDir, "nist-800-53-r5", state)
	require.NoError(t, err)

	loaded, err := LoadGenerationState(baseDir, "nist-800-53-r5")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, state.PolicyID, loaded.PolicyID)
	assert.Equal(t, state.PolicyDigest, loaded.PolicyDigest)
	assert.Equal(t, state.ComplypackDigests, loaded.ComplypackDigests)
	assert.Equal(t, state.GeneratedAt, loaded.GeneratedAt)
	assert.Equal(t, state.EvaluatorIDs, loaded.EvaluatorIDs)
}

func TestGenerationState_SaveLoadRoundTrip_NoComplypackDigests(t *testing.T) {
	baseDir := t.TempDir()
	state := &GenerationState{
		PolicyID:     "test-policy",
		PolicyDigest: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
	}

	require.NoError(t, SaveGenerationState(baseDir, "test-policy", state))

	loaded, err := LoadGenerationState(baseDir, "test-policy")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Nil(t, loaded.ComplypackDigests, "omitempty field should be nil when absent")
}

func TestGenerationState_SaveCreatesNestedDirs(t *testing.T) {
	baseDir := t.TempDir()
	state := &GenerationState{PolicyID: "policies/nested/deep"}

	err := SaveGenerationState(baseDir, "policies/nested/deep", state)
	require.NoError(t, err)

	path := filepath.Join(baseDir, complytime.WorkspaceDir, "generation", "policies/nested/deep.json")
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestLoadGenerationState_MissingFile(t *testing.T) {
	baseDir := t.TempDir()
	loaded, err := LoadGenerationState(baseDir, "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestLoadGenerationState_CorruptJSON(t *testing.T) {
	baseDir := t.TempDir()
	dir := filepath.Join(baseDir, complytime.WorkspaceDir, "generation")
	require.NoError(t, os.MkdirAll(dir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{not valid json"), 0600))

	_, err := LoadGenerationState(baseDir, "corrupt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse generation state")
}

// --- T153: IsFresh tests ---

func TestIsFresh_MatchingDigest(t *testing.T) {
	s := &GenerationState{PolicyDigest: "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"}
	assert.True(t, s.IsFresh("sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", nil))
}

func TestIsFresh_MismatchedDigest(t *testing.T) {
	s := &GenerationState{PolicyDigest: "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"}
	assert.False(t, s.IsFresh("sha256:d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5", nil))
}

func TestIsFresh_EmptyDigest(t *testing.T) {
	s := &GenerationState{PolicyDigest: ""}
	assert.False(t, s.IsFresh("sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", nil))
}

func TestIsFresh_BothEmpty(t *testing.T) {
	s := &GenerationState{PolicyDigest: ""}
	assert.True(t, s.IsFresh("", nil))
}

func TestIsFresh_ComplypackDigestChanged(t *testing.T) {
	s := &GenerationState{
		PolicyDigest:      "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		ComplypackDigests: map[string]string{"opa": "sha256:4444444444444444444444444444444444444444444444444444444444444444"},
	}
	assert.False(t, s.IsFresh("sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", map[string]string{"opa": "sha256:3333333333333333333333333333333333333333333333333333333333333333"}))
}

func TestIsFresh_ComplypackDigestAdded(t *testing.T) {
	s := &GenerationState{PolicyDigest: "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"}
	assert.False(t, s.IsFresh("sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", map[string]string{"opa": "sha256:1111111111111111111111111111111111111111111111111111111111111111"}))
}

func TestIsFresh_NilVsEmptyComplypackDigests(t *testing.T) {
	s := &GenerationState{PolicyDigest: "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"}
	assert.True(t, s.IsFresh("sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", map[string]string{}),
		"nil and empty map both mean no complypacks")
}

func TestIsFresh_ComplypackDigestRemoved(t *testing.T) {
	s := &GenerationState{
		PolicyDigest:      "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		ComplypackDigests: map[string]string{"opa": "sha256:1111111111111111111111111111111111111111111111111111111111111111"},
	}
	assert.False(t, s.IsFresh("sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", nil),
		"removing a complypack should trigger regeneration")
}

func TestIsFresh_MatchingComplypackDigests(t *testing.T) {
	cpDigests := map[string]string{"opa": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "ampel": "sha256:2222222222222222222222222222222222222222222222222222222222222222"}
	s := &GenerationState{
		PolicyDigest:      "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		ComplypackDigests: cpDigests,
	}
	assert.True(t, s.IsFresh("sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", map[string]string{"opa": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "ampel": "sha256:2222222222222222222222222222222222222222222222222222222222222222"}))
}

// --- T154: NewGenerationState tests ---

func TestNewGenerationState(t *testing.T) {
	evalIDs := []string{"openscap", "kube-eval"}
	cpDigests := map[string]string{"opa": "sha256:1111111111111111111111111111111111111111111111111111111111111111"}
	state := NewGenerationState("test-policy", "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", evalIDs, cpDigests)

	assert.Equal(t, "test-policy", state.PolicyID)
	assert.Equal(t, "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", state.PolicyDigest)
	assert.Equal(t, cpDigests, state.ComplypackDigests)
	assert.Equal(t, evalIDs, state.EvaluatorIDs)

	_, err := time.Parse(time.RFC3339, state.GeneratedAt)
	assert.NoError(t, err, "GeneratedAt should be valid RFC3339")
}

func TestNewGenerationState_NilEvaluatorIDs(t *testing.T) {
	state := NewGenerationState("test", "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", nil, nil)
	assert.Nil(t, state.EvaluatorIDs)
	assert.Nil(t, state.ComplypackDigests)
}

// --- InvalidateForEvaluator tests ---

func TestInvalidateForEvaluator_RemovesMatchingState(t *testing.T) {
	baseDir := t.TempDir()
	state := &GenerationState{
		PolicyID:     "test-policy",
		PolicyDigest: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		EvaluatorIDs: []string{"opa", "ampel"},
	}
	require.NoError(t, SaveGenerationState(baseDir, "test-policy", state))

	warnings, err := InvalidateForEvaluator(baseDir, "opa")
	require.NoError(t, err)
	assert.Empty(t, warnings)

	loaded, err := LoadGenerationState(baseDir, "test-policy")
	assert.NoError(t, err)
	assert.Nil(t, loaded, "generation state referencing 'opa' should be deleted")
}

func TestInvalidateForEvaluator_PreservesUnrelatedState(t *testing.T) {
	baseDir := t.TempDir()

	opaState := &GenerationState{
		PolicyID:     "policy-a",
		PolicyDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		EvaluatorIDs: []string{"opa"},
	}
	ampelState := &GenerationState{
		PolicyID:     "policy-b",
		PolicyDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		EvaluatorIDs: []string{"ampel"},
	}
	require.NoError(t, SaveGenerationState(baseDir, "policy-a", opaState))
	require.NoError(t, SaveGenerationState(baseDir, "policy-b", ampelState))

	warnings, err := InvalidateForEvaluator(baseDir, "opa")
	require.NoError(t, err)
	assert.Empty(t, warnings)

	loaded, err := LoadGenerationState(baseDir, "policy-a")
	assert.NoError(t, err)
	assert.Nil(t, loaded, "policy-a should be removed (references opa)")

	loaded, err = LoadGenerationState(baseDir, "policy-b")
	assert.NoError(t, err)
	assert.NotNil(t, loaded, "policy-b should be preserved (only references ampel)")
}

func TestInvalidateForEvaluator_NoGenerationDir(t *testing.T) {
	baseDir := t.TempDir()
	warnings, err := InvalidateForEvaluator(baseDir, "opa")
	assert.NoError(t, err, "should not error when generation/ does not exist")
	assert.Empty(t, warnings)
}

func TestInvalidateForEvaluator_MalformedJSON(t *testing.T) {
	baseDir := t.TempDir()
	genDir := filepath.Join(baseDir, complytime.WorkspaceDir, "generation")
	require.NoError(t, os.MkdirAll(genDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "bad.json"), []byte("{invalid"), 0600))

	warnings, err := InvalidateForEvaluator(baseDir, "opa")
	assert.NoError(t, err, "malformed JSON files should be skipped, not cause an error")
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "skipped malformed JSON")
	assert.Contains(t, warnings[0], "bad.json")

	_, statErr := os.Stat(filepath.Join(genDir, "bad.json"))
	assert.NoError(t, statErr, "malformed file should not be removed")
}

func TestInvalidateForEvaluator_UnreadableFile(t *testing.T) {
	baseDir := t.TempDir()
	genDir := filepath.Join(baseDir, complytime.WorkspaceDir, "generation")
	require.NoError(t, os.MkdirAll(genDir, 0750))

	unreadable := filepath.Join(genDir, "locked.json")
	require.NoError(t, os.WriteFile(unreadable, []byte(`{"evaluator_ids":["opa"]}`), 0000))
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0600) })

	warnings, err := InvalidateForEvaluator(baseDir, "opa")
	assert.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "skipped unreadable file")
	assert.Contains(t, warnings[0], "locked.json")
}

func TestInvalidateForEvaluator_NestedPolicyID(t *testing.T) {
	baseDir := t.TempDir()
	state := &GenerationState{
		PolicyID:     "policies/nested-policy",
		PolicyDigest: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		EvaluatorIDs: []string{"opa"},
	}
	require.NoError(t, SaveGenerationState(baseDir, "policies/nested-policy", state))

	warnings, err := InvalidateForEvaluator(baseDir, "opa")
	require.NoError(t, err)
	assert.Empty(t, warnings)

	loaded, err := LoadGenerationState(baseDir, "policies/nested-policy")
	assert.NoError(t, err)
	assert.Nil(t, loaded, "nested generation state referencing 'opa' should be deleted")
}

func TestInvalidateForEvaluator_NestedNonJSONPreserved(t *testing.T) {
	baseDir := t.TempDir()
	genDir := filepath.Join(baseDir, complytime.WorkspaceDir, "generation", "policies")
	require.NoError(t, os.MkdirAll(genDir, 0750))

	require.NoError(t, os.WriteFile(filepath.Join(genDir, "README.md"), []byte("# keep"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "metadata.yaml"), []byte("keep: true"), 0600))

	state := &GenerationState{
		PolicyID:     "policies/target",
		PolicyDigest: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		EvaluatorIDs: []string{"opa"},
	}
	require.NoError(t, SaveGenerationState(baseDir, "policies/target", state))

	warnings, err := InvalidateForEvaluator(baseDir, "opa")
	require.NoError(t, err)
	assert.Empty(t, warnings)

	loaded, err := LoadGenerationState(baseDir, "policies/target")
	assert.NoError(t, err)
	assert.Nil(t, loaded, "matching state should be deleted")

	_, statErr := os.Stat(filepath.Join(genDir, "README.md"))
	assert.NoError(t, statErr, "nested non-JSON file should be preserved")

	_, statErr = os.Stat(filepath.Join(genDir, "metadata.yaml"))
	assert.NoError(t, statErr, "nested non-JSON file should be preserved")
}

// --- RemoveEvaluatorArtifacts tests ---

func TestRemoveEvaluatorArtifacts_RemovesDir(t *testing.T) {
	baseDir := t.TempDir()
	evalDir := filepath.Join(baseDir, complytime.WorkspaceDir, "opa")
	require.NoError(t, os.MkdirAll(evalDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "scan-config.json"), []byte("{}"), 0600))

	err := RemoveEvaluatorArtifacts(baseDir, "opa")
	require.NoError(t, err)

	_, statErr := os.Stat(evalDir)
	assert.True(t, os.IsNotExist(statErr), "evaluator artifact dir should be removed")
}

func TestRemoveEvaluatorArtifacts_DirNotExist(t *testing.T) {
	baseDir := t.TempDir()
	err := RemoveEvaluatorArtifacts(baseDir, "nonexistent")
	assert.NoError(t, err, "should not error when dir does not exist")
}

func TestInvalidateForEvaluator_IgnoresNonJSONFiles(t *testing.T) {
	baseDir := t.TempDir()
	genDir := filepath.Join(baseDir, complytime.WorkspaceDir, "generation")
	require.NoError(t, os.MkdirAll(genDir, 0750))

	// Place a non-JSON file alongside a valid JSON state file.
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "notes.txt"), []byte("keep me"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "backup.bak"), []byte("keep me too"), 0600))

	state := &GenerationState{
		PolicyID:     "policy-x",
		PolicyDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		EvaluatorIDs: []string{"opa"},
	}
	require.NoError(t, SaveGenerationState(baseDir, "policy-x", state))

	warnings, err := InvalidateForEvaluator(baseDir, "opa")
	require.NoError(t, err)
	assert.Empty(t, warnings)

	// JSON state file referencing 'opa' should be deleted.
	loaded, err := LoadGenerationState(baseDir, "policy-x")
	assert.NoError(t, err)
	assert.Nil(t, loaded, "JSON state file should be deleted")

	// Non-JSON files should be preserved.
	_, statErr := os.Stat(filepath.Join(genDir, "notes.txt"))
	assert.NoError(t, statErr, "non-JSON .txt file should be preserved")

	_, statErr = os.Stat(filepath.Join(genDir, "backup.bak"))
	assert.NoError(t, statErr, "non-JSON .bak file should be preserved")
}

func TestRemoveEvaluatorArtifacts_RejectsPathTraversal(t *testing.T) {
	baseDir := t.TempDir()
	err := RemoveEvaluatorArtifacts(baseDir, "../../etc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid evaluator ID")
}
