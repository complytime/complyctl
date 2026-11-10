// SPDX-License-Identifier: Apache-2.0

package cli

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
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/pkg/provider"
)

func TestEnsureGenerated_UpgradesLegacyProjectionOnce(t *testing.T) {
	baseDir, dataDir, cacheDir := t.TempDir(), t.TempDir(), t.TempDir()
	cpDigests := map[string]string{"opa": cachetest.DigestB}
	require.NoError(t, cache.SaveState(&cache.State{
		Policies: map[string]cache.PolicyState{"multi": {Digest: cachetest.DigestA}},
		Complypacks: map[string]cache.PolicyState{
			"packs/opa": {EvaluatorID: "opa", Digest: cachetest.DigestB},
		},
	}, dataDir))
	evalDir := filepath.Join(baseDir, complytime.WorkspaceDir, "opa")
	require.NoError(t, os.MkdirAll(evalDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "root-only.rego"),
		[]byte("package root"), 0600))
	genDir := filepath.Join(baseDir, complytime.WorkspaceDir, "generation")
	require.NoError(t, os.MkdirAll(genDir, 0750))
	legacyJSON := fmt.Sprintf(`{"policy_id":"multi","policy_digest":%q,
		"complypack_digests":{"opa":%q},"evaluator_ids":["opa"]}`,
		cachetest.DigestA, cachetest.DigestB)
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "multi.json"), []byte(legacyJSON), 0600))
	assert.True(t, evaluatorArtifactsExist(baseDir, []string{"opa"}))

	needs, digest, gotCP, err := checkGenerationFreshness(dataDir, baseDir, "multi", "multi")
	require.NoError(t, err)
	assert.Equal(t, cachetest.DigestA, digest)
	assert.Equal(t, cpDigests, gotCP)
	assert.True(t, needs, "unchanged digests and old artifacts must not bypass imported plans")

	// A missing provider makes attempted regeneration observable without a fake
	// generation implementation. Failure must leave the legacy state stale.
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)
	groups := map[string]policy.EvaluatorGroup{"opa": {EvaluatorID: "opa",
		Configs: []provider.AssessmentConfiguration{
			{PlanID: "root-plan", RequirementID: "root-req"},
			{PlanID: "imported-plan", RequirementID: "imported-req"},
		}}}
	targets := []complytime.TargetConfig{{ID: "target"}}
	err = ensureGenerated(context.Background(), cacheDir, dataDir, baseDir, mgr,
		groups, targets, nil, "multi", "multi", []string{"opa"})
	require.ErrorContains(t, err, `no provider registered for evaluator "opa"`)
	legacy, err := policy.LoadGenerationState(baseDir, "multi")
	require.NoError(t, err)
	assert.False(t, legacy.IsFresh(cachetest.DigestA, cpDigests))

	// Exercise the explicit-generate success persistence path, then the same
	// scan freshness/skip path using the on-disk state, not an in-memory stub.
	require.NoError(t, saveGenerationAndPrint(dataDir, baseDir, "multi", "multi",
		[]string{"opa"}, []string{"opa"}, nil))
	current, err := policy.LoadGenerationState(baseDir, "multi")
	require.NoError(t, err)
	assert.True(t, current.IsFresh(cachetest.DigestA, cpDigests))
	needs, _, _, err = checkGenerationFreshness(dataDir, baseDir, "multi", "multi")
	require.NoError(t, err)
	assert.False(t, needs, "persisted current projection must remain fresh")
	require.NoError(t, ensureGenerated(context.Background(), cacheDir, dataDir, baseDir, mgr,
		groups, targets, nil, "multi", "multi", []string{"opa"}))
}
