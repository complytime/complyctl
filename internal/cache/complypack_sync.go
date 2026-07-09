// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	ocistore "oras.land/oras-go/v2/content/oci"

	"github.com/complytime/complypack/pkg/complypack"
)

// ComplypackSync provides incremental sync for complypack artifacts.
// Mirrors the Sync/SyncPolicy pattern: compares remote manifest digest
// against local state and skips fetch when unchanged.
type ComplypackSync struct {
	complypackCache *ComplypackCache
	state           *State
	source          ComplypackSource
	verifier        VerifyFunc
}

// NewComplypackSync creates a ComplypackSync that orchestrates the
// fetch-unpack-store pipeline for complypack artifacts. SyncOption args
// configure optional behavior such as signature verification.
func NewComplypackSync(complypackCache *ComplypackCache, state *State, source ComplypackSource, opts ...SyncOption) *ComplypackSync {
	cfg := &syncConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return &ComplypackSync{
		complypackCache: complypackCache,
		state:           state,
		source:          source,
		verifier:        cfg.verifier,
	}
}

// cacheExistsForState checks whether the cache directory for a previously
// synced complypack still exists on disk. Uses the evaluator-id recorded in
// state to locate the cache via LookupByEvaluatorID. Returns false if the
// evaluator-id is empty (state predates evaluator-id tracking) or if the
// cache files are missing.
func (s *ComplypackSync) cacheExistsForState(ps PolicyState) bool {
	if ps.EvaluatorID == "" {
		return false
	}
	contentPath, _, err := s.complypackCache.LookupByEvaluatorID(ps.EvaluatorID)
	return err == nil && contentPath != ""
}

// SyncComplypack performs incremental synchronization of a complypack artifact.
// Compares the local cached digest against the remote manifest digest; if they
// match, sync is skipped. On change, the artifact is fetched into a temporary
// OCI Layout store, unpacked via complypack.Unpack(), and stored via
// ComplypackCache.Store(). State is updated and persisted on success.
//
// Returns (true, nil) when a fetch occurred, (false, nil) when the local cache
// was already up-to-date (incremental skip), or (false, err) on failure.
func (s *ComplypackSync) SyncComplypack(ctx context.Context, repository, version string) (bool, error) {
	if repository == "" {
		return false, fmt.Errorf("complypack repository cannot be empty")
	}

	tag, digest := classifyVersion(version)
	lookupRef := BuildLookupRef(repository, tag, digest)

	remoteDigest, remoteVersion, err := s.source.DefinitionVersion(ctx, lookupRef)
	if err != nil {
		// Registry unreachable: attempt to serve from local cache
		// before failing. If the repository has a cached entry with
		// existing content on disk, the user can continue working
		// offline with previously fetched data (FR-004).
		localState, exists := s.state.GetComplypackState(repository)
		if exists && s.cacheExistsForState(localState) {
			log.Warn("registry unreachable, serving from local cache",
				"repository", repository, "error", err)
			return false, nil
		}
		return false, fmt.Errorf(
			"complypack %s: registry unreachable: %w",
			repository, err,
		)
	}

	if version == "" || version == "latest" {
		version = remoteVersion
	}

	// Incremental sync check: skip only if local digest matches remote AND
	// the cache directory still exists on disk. The state records the
	// evaluator-id from the previous unpack, so we can verify cache presence
	// without re-unpacking. This mirrors the policy sync pattern where
	// PolicyStoreExists() gates the skip (see sync.go line 85).
	localState, exists := s.state.GetComplypackState(repository)
	if exists && localState.Digest == remoteDigest && s.cacheExistsForState(localState) {
		return false, nil
	}

	// Local cache hit check (FR-004, D3): when switching to a different
	// version that already exists in the local cache, serve it without a
	// network fetch. This avoids re-downloads when switching between
	// previously cached versions. The evaluator-id is resolved from the
	// existing state entry for the repository. First-time syncs (no state
	// entry) always fetch.
	//
	// The localState.Version != version guard ensures this path is only
	// entered when the user changed the version tag in complytime.yaml.
	// Same-version-different-digest (mutable tag re-push) falls through
	// to the standard fetch path: the incremental skip at line 88 fails
	// (digest mismatch), this guard fails (version matches), and the code
	// continues to the full OCI fetch below.
	if exists && localState.EvaluatorID != "" && localState.Version != version {
		cacheHit, cacheErr := s.tryLocalCacheHit(
			ctx, repository, version, remoteDigest,
			localState, tag, digest,
		)
		if cacheErr == nil && cacheHit {
			return true, nil
		}
		// Cache miss or verification error — fall through to standard
		// fetch path which will re-download and re-verify. Log
		// verification errors at WARN since they may indicate a
		// security-relevant signal (e.g., signature revoked).
		if cacheErr != nil {
			log.Warn("local cache hit rejected, falling back to fetch",
				"repository", repository, "version", version,
				"error", cacheErr)
		}
	}

	// Pre-copy verification: verify signature via registry API before copying
	// content to disk. If verification fails, the local cache is unchanged.
	var verifyResult *VerificationResult
	if s.verifier != nil {
		registryRef := BuildLookupRef(repository, tag, digest)
		vr, verifyErr := s.verifier(ctx, registryRef)
		if verifyErr != nil {
			return false, fmt.Errorf("complypack %s: verification failed: %w", repository, verifyErr)
		}
		verifyResult = vr
	}

	// Create a temporary OCI Layout store for the oras.Copy() transfer.
	// This is discarded after unpacking — the final cache uses the
	// ComplypackCache directory structure, not an OCI Layout.
	tmpDir, err := os.MkdirTemp("", "complypack-oci-*")
	if err != nil {
		return false, fmt.Errorf("failed to create temporary OCI store directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpStore, err := ocistore.New(tmpDir)
	if err != nil {
		return false, fmt.Errorf("failed to open temporary OCI store: %w", err)
	}

	desc, err := s.source.CopyComplypack(ctx, repository, version, tmpStore)
	if err != nil {
		return false, fmt.Errorf(
			"complypack %s@%s: registry unreachable: %w (local cache unchanged)",
			repository, version, err,
		)
	}

	// Unpack the complypack artifact from the temporary OCI store.
	result, err := complypack.Unpack(ctx, tmpStore, desc)
	if err != nil {
		return false, fmt.Errorf("failed to unpack complypack %s@%s: %w", repository, version, err)
	}
	defer result.Content.Close()

	// Store the unpacked config and content into the ComplypackCache.
	_, err = s.complypackCache.Store(result.Config, result.Content)
	if err != nil {
		return false, fmt.Errorf("failed to store complypack %s@%s: %w", repository, version, err)
	}

	// Use result.Config.Version (from the complypack's embedded config.json)
	// instead of the OCI tag `version` for state tracking. Store() creates
	// on-disk directories using config.Version (e.g., "1.0.0"), so state
	// must record the same value to keep state ↔ filesystem consistent.
	// See: https://github.com/complytime/complyctl/issues/694
	s.state.UpdateComplypackStateWithVerification(repository, result.Config.Version, remoteDigest, result.Config.EvaluatorID, verifyResult)
	if err := SaveState(s.state, s.complypackCache.Dir()); err != nil {
		return false, fmt.Errorf("failed to save state after complypack sync: %w (complypack blobs are valid)", err)
	}

	return true, nil
}

// tryLocalCacheHit checks whether the target version already exists in the
// local cache for the given evaluator-id. If found, it optionally re-verifies
// via the registry API (D6, FR-010), updates state, and persists it. Returns
// (true, nil) on a successful cache hit, or (false, err) to signal the caller
// should fall through to the standard fetch path.
func (s *ComplypackSync) tryLocalCacheHit(
	ctx context.Context,
	repository, version, remoteDigest string,
	localState PolicyState,
	tag, digest string,
) (bool, error) {
	contentPath, _, lookupErr := s.complypackCache.Lookup(
		localState.EvaluatorID, version,
	)
	if lookupErr != nil || contentPath == "" {
		return false, lookupErr
	}

	// Re-verify via registry API when a verifier is configured (D6).
	// If verification fails, reject the cache hit — the caller will
	// proceed to the standard fetch path.
	var verifyResult *VerificationResult
	if s.verifier != nil {
		registryRef := BuildLookupRef(repository, tag, digest)
		vr, verifyErr := s.verifier(ctx, registryRef)
		if verifyErr != nil {
			return false, verifyErr
		}
		verifyResult = vr
	}

	// Update state to reflect the version switch and persist.
	// remoteDigest (not local content digest) is intentional: state
	// tracks the registry manifest digest so that the incremental sync
	// check (localState.Digest == remoteDigest) can skip redundant
	// fetches on subsequent runs. Using a local content digest would
	// cause a permanent mismatch and force re-fetches every time.
	s.state.UpdateComplypackStateWithVerification(
		repository, version, remoteDigest,
		localState.EvaluatorID, verifyResult,
	)
	if err := SaveState(s.state, s.complypackCache.Dir()); err != nil {
		return false, err
	}

	// Return true to trigger generation invalidation in callers (FR-009).
	return true, nil
}
