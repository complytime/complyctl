// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache/cachetest"
	"github.com/complytime/complyctl/internal/complytime"
)

func TestGenerationState_ResolvedTreeFreshness(t *testing.T) {
	for _, tc := range []struct {
		name, persisted, current string
		wantFresh                bool
	}{
		{"same tree different OCI", `,"resolved_tree_digest":"sha256:tree"`, "sha256:tree", true},
		{"different tree same OCI", `,"resolved_tree_digest":"sha256:old-tree"`, cachetest.DigestA, false},
		{"former projection version", `,"projection_version":1`, cachetest.DigestA, false},
		{"missing identifier", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var state GenerationState
			require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(
				`{"policy_digest":%q%s}`, cachetest.DigestA, tc.persisted)), &state))
			assert.Equal(t, tc.wantFresh, state.IsFresh(tc.current, nil))
		})
	}
}

func TestGenerationState_ResolvedTreeUpgrade(t *testing.T) {
	for _, versionField := range []string{"", `,"projection_version":-1`, `,"projection_version":1`} {
		t.Run(versionField, func(t *testing.T) {
			baseDir := t.TempDir()
			dir := filepath.Join(baseDir, complytime.WorkspaceDir, "generation")
			require.NoError(t, os.MkdirAll(dir, 0750))
			data := fmt.Sprintf(`{"policy_digest":%q,"evaluator_ids":["opa"]%s}`,
				cachetest.DigestA, versionField)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "multi.json"), []byte(data), 0600))
			state, err := LoadGenerationState(baseDir, "multi")
			require.NoError(t, err)
			assert.False(t, state.IsFresh(cachetest.DigestA, nil))
			require.NoError(t, SaveGenerationState(baseDir, "multi", state))
			state, err = LoadGenerationState(baseDir, "multi")
			require.NoError(t, err)
			assert.False(t, state.IsFresh(cachetest.DigestA, nil),
				"saving old state must not certify successful current generation")
			require.NoError(t, SaveGenerationState(baseDir, "multi",
				NewGenerationState("multi", cachetest.DigestB, cachetest.DigestA, []string{"opa"}, nil)))
			state, err = LoadGenerationState(baseDir, "multi")
			require.NoError(t, err)
			assert.True(t, state.IsFresh(cachetest.DigestA, nil))
		})
	}
}
