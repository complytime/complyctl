// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"fmt"
)

// Sync provides incremental sync using oras.Copy() for remote-to-local transfer.
type Sync struct {
	cache  *Cache
	state  *State
	source PolicySource
}

func NewSync(cache *Cache, state *State, source PolicySource) *Sync {
	return &Sync{
		cache:  cache,
		state:  state,
		source: source,
	}
}

// SyncPolicy performs incremental synchronization of a policy. Compares local
// digest against remote manifest digest; if they match, sync is skipped. On
// failure, the OCI Layout store retains its previous state.
func (s *Sync) SyncPolicy(ctx context.Context, policyID, version string) error {
	if policyID == "" {
		return fmt.Errorf("policy ID cannot be empty")
	}

	remoteDigest, remoteVersion, err := s.source.DefinitionVersion(ctx, policyID)
	if err != nil {
		return fmt.Errorf(
			"policy %s: registry unreachable: %w (cached data may still be available)",
			policyID, err,
		)
	}

	if version == "" || version == "latest" {
		version = remoteVersion
	}

	localState, exists := s.state.GetPolicyState(policyID)
	if exists && localState.Digest == remoteDigest && s.cache.PolicyStoreExists(policyID) {
		return nil
	}

	localStore, err := s.cache.NewPolicyStore(policyID)
	if err != nil {
		return fmt.Errorf("failed to open local store for policy %s: %w", policyID, err)
	}

	_, err = s.source.CopyPolicy(ctx, policyID, version, localStore)
	if err != nil {
		return fmt.Errorf(
			"policy %s@%s: registry unreachable: %w (local cache unchanged)",
			policyID, version, err,
		)
	}

	s.state.UpdatePolicyState(policyID, version, remoteDigest)
	if err := SaveState(s.state, s.cache.Dir()); err != nil {
		return fmt.Errorf("failed to save state after sync: %w (policy blobs are valid)", err)
	}

	return nil
}
