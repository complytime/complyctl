// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/opencontainers/go-digest"

	"github.com/complytime/complyctl/internal/registry"
)

// BuildLookupRef constructs an OCI lookup reference from a repository with
// separate tag and digest fields. When digest is non-empty it uses "@" as
// the separator; when tag is non-empty (and not "latest") it uses ":".
// If both are empty or tag is "latest", the bare repository is returned so
// oras resolves the default tag.
func BuildLookupRef(repository, tag, digest string) string {
	if digest != "" {
		return repository + "@" + digest
	}
	if tag == "" || tag == "latest" {
		return repository
	}
	return repository + ":" + tag
}

// classifyVersion splits a version string into tag and digest components.
// It delegates to go-digest's Parse to determine whether the string is a
// valid OCI digest (sha256, sha384, sha512). This is a convenience for
// callers that receive an untyped version string and need to call
// BuildLookupRef.
func classifyVersion(version string) (tag string, dgst string) {
	if _, err := digest.Parse(version); err == nil {
		return "", version
	}
	return version, ""
}

// SyncOption configures optional behavior for sync operations.
type SyncOption func(*syncConfig)

type syncConfig struct {
	verifier VerifyFunc
}

// WithVerifier sets a signature verification function that is called before
// copying content from the registry. If verification fails, the sync is
// aborted and the local cache remains unchanged.
func WithVerifier(vf VerifyFunc) SyncOption {
	return func(c *syncConfig) {
		c.verifier = vf
	}
}

// Sync provides incremental sync using oras.Copy() for remote-to-local transfer.
type Sync struct {
	cache    *Cache
	state    *State
	source   PolicySource
	verifier VerifyFunc
	dataDir  string
}

// NewSync creates a Sync that orchestrates remote-to-local policy transfer.
// dataDir is the XDG data directory where state.json is persisted.
// SyncOption args configure optional behavior such as signature verification.
func NewSync(cache *Cache, state *State, source PolicySource, dataDir string, opts ...SyncOption) *Sync {
	cfg := &syncConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Sync{
		cache:    cache,
		state:    state,
		source:   source,
		verifier: cfg.verifier,
		dataDir:  dataDir,
	}
}

// SyncPolicy performs incremental synchronization of a policy. Compares local
// digest against remote manifest digest; if they match, sync is skipped. On
// failure, the OCI Layout store retains its previous state.
//
// Returns (true, nil) when a fetch occurred, (false, nil) when the local cache
// was already up-to-date (incremental skip), or (false, err) on failure.
func (s *Sync) SyncPolicy(ctx context.Context, policyID, version string) (bool, error) {
	if policyID == "" {
		return false, fmt.Errorf("policy ID cannot be empty")
	}

	tag, digest := classifyVersion(version)
	lookupRef := BuildLookupRef(policyID, tag, digest)

	remoteDigest, remoteVersion, err := s.source.DefinitionVersion(ctx, lookupRef)
	if err != nil {
		if errors.Is(err, registry.ErrVersionNotFound) {
			return false, fmt.Errorf("policy %s: %w", policyID, err)
		}
		return false, fmt.Errorf("policy %s: registry unreachable: %w", policyID, err)
	}

	if version == "" || version == "latest" {
		version = remoteVersion
	}

	localState, exists := s.state.GetPolicyState(policyID)
	if exists && localState.Digest == remoteDigest && s.cache.PolicyStoreExists(policyID) {
		return false, nil
	}

	// Pre-copy verification: verify signature via registry API before copying
	// content to disk. If verification fails, the local cache is unchanged.
	var verifyResult *VerificationResult
	if s.verifier != nil {
		registryRef := BuildLookupRef(policyID, tag, digest)
		vr, verifyErr := s.verifier(ctx, registryRef)
		if verifyErr != nil {
			return false, fmt.Errorf("policy %s: verification failed: %w", policyID, verifyErr)
		}
		verifyResult = vr
	}

	localStore, err := s.cache.NewPolicyStore(policyID)
	if err != nil {
		return false, fmt.Errorf("failed to open local store for policy %s: %w", policyID, err)
	}

	_, err = s.source.CopyPolicy(ctx, policyID, version, localStore)
	if err != nil {
		return false, fmt.Errorf("policy %s@%s: copy failed: %w", policyID, version, err)
	}

	s.state.UpdatePolicyStateWithVerification(policyID, version, remoteDigest, verifyResult)
	if err := SaveState(s.state, s.dataDir); err != nil {
		return false, fmt.Errorf("failed to save state after sync: %w (policy blobs are valid)", err)
	}

	return true, nil
}
