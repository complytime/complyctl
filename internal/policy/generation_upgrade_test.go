// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache/cachetest"
	"github.com/complytime/complyctl/internal/complytime"
)

func TestGenerationState_ProjectionUpgrade(t *testing.T) {
	for _, versionField := range []string{"", `,"projection_version":-1`} {
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
				NewGenerationState("multi", cachetest.DigestA, []string{"opa"}, nil)))
			state, err = LoadGenerationState(baseDir, "multi")
			require.NoError(t, err)
			assert.True(t, state.IsFresh(cachetest.DigestA, nil))
		})
	}
}
