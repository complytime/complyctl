// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ocistore "oras.land/oras-go/v2/content/oci"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
)

func TestSyncPolicies_RefreshesPopulatedMetadataOnCacheHit(t *testing.T) {
	ctx := context.Background()
	layout := t.TempDir()
	store, err := ocistore.New(layout)
	require.NoError(t, err)
	b := &bundle.Bundle{
		Source: bundle.File{Name: "root.yaml", Type: "Policy", Data: []byte(`metadata: {id: root, type: Policy}
title: Root display title
adherence:
  assessment-plans: [{id: root-plan, requirement-id: root-req}]
`)},
		Imports: []bundle.File{{Name: "parent.yaml", Type: "Policy", Data: []byte(`metadata: {id: parent, type: Policy}
title: Parent
adherence:
  assessment-plans: [{id: parent-plan, requirement-id: parent-req}]
`)}},
	}
	desc, err := bundle.Pack(ctx, store, b)
	require.NoError(t, err)
	options := &getOptions{cacheDir: t.TempDir(), dataDir: t.TempDir(), fromOCILayout: layout}
	cfg := &complytime.WorkspaceConfig{Policies: []complytime.PolicyEntry{{
		ID: "test", URL: "registry.invalid/policies/test@" + desc.Digest.String(),
	}}}
	require.NoError(t, options.syncPolicies(ctx, cfg, nil, nil))
	state, err := cache.LoadState(options.dataDir)
	require.NoError(t, err)
	state.SetPolicyMetadata("policies/test", "Root display title", "", 0, 1)
	require.NoError(t, cache.SaveState(state, options.dataDir))
	before, found := state.GetPolicyState("policies/test")
	require.True(t, found)
	require.NoError(t, options.syncPolicies(ctx, cfg, nil, nil))
	state, err = cache.LoadState(options.dataDir)
	require.NoError(t, err)
	after, found := state.GetPolicyState("policies/test")
	require.True(t, found)
	assert.Equal(t, 2, after.AssessmentCount)
	assert.Equal(t, "Root display title", after.PolicyTitle)
	assert.Equal(t, before.Digest, after.Digest)
	assert.Equal(t, before.LastUpdated, after.LastUpdated)
	statePath := filepath.Join(options.dataDir, complytime.StateFileName)
	unchanged, err := os.ReadFile(statePath)
	require.NoError(t, err)
	require.NoError(t, options.syncPolicies(ctx, cfg, nil, nil))
	repeated, err := os.ReadFile(statePath)
	require.NoError(t, err)
	assert.Equal(t, unchanged, repeated, "unchanged metadata must not rewrite sync state")
}
