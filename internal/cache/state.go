// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/opencontainers/go-digest"

	"github.com/complytime/complyctl/internal/complytime"
)

// ValidateDigest checks that s is a well-formed OCI content digest
// (algorithm:hex, supported algorithm, correct hex length). Empty
// strings are allowed and return nil — they represent entries that
// have not yet been synced.
func ValidateDigest(s string) error {
	if s == "" {
		return nil
	}
	_, err := digest.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid digest %q: %w", s, err)
	}
	return nil
}

// State tracks sync metadata for all cached policies and complypacks,
// persisted as state.json.
type State struct {
	LastSync    time.Time              `json:"last_sync"`
	Policies    map[string]PolicyState `json:"policies"`
	Complypacks map[string]PolicyState `json:"complypacks,omitempty"`
}

// PolicyState holds version, digest, verification status, timestamp, and
// display-oriented metadata for a single cached policy or complypack.
// The metadata fields (PolicyTitle, PolicyEvaluator, ControlCount,
// AssessmentCount) are populated at sync time by ExtractPolicyMetadata
// and are used by the list and get commands for display purposes.
type PolicyState struct {
	Version        string    `json:"version"`
	Digest         string    `json:"digest"`
	EvaluatorID    string    `json:"evaluator_id,omitempty"`
	LastUpdated    time.Time `json:"last_updated"`
	Verified       bool      `json:"verified,omitempty"`
	SignerIdentity string    `json:"signer_identity,omitempty"`
	Issuer         string    `json:"issuer,omitempty"`
	VerifiedAt     time.Time `json:"verified_at,omitempty"`

	// Display-oriented metadata extracted from Gemara policy YAML.
	PolicyTitle     string `json:"policy_title,omitempty"`
	PolicyEvaluator string `json:"policy_evaluator,omitempty"`
	ControlCount    int    `json:"control_count"`
	AssessmentCount int    `json:"assessment_count"`
}

// LoadState reads and parses the state.json file from the given base directory.
// Returns a fresh State with empty maps if the file does not exist.
func LoadState(baseDir string) (*State, error) {
	statePath := filepath.Join(baseDir, complytime.StateFileName)

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{
				LastSync:    time.Time{},
				Policies:    make(map[string]PolicyState),
				Complypacks: make(map[string]PolicyState),
			}, nil
		}
		return nil, fmt.Errorf("failed to read state file %s: %w", statePath, err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file %s: %w", statePath, err)
	}

	initStateMaps(&state)
	excludeMalformedDigests(&state)

	return &state, nil
}

// excludeMalformedDigests removes entries with non-empty, malformed digest
// fields from the Policies and Complypacks maps, logging a warning for each.
// Empty digest fields are preserved for backward compatibility with
// pre-digest state entries. This provides defense-in-depth against corrupted
// or hand-edited state files.
func excludeMalformedDigests(s *State) {
	for key, ps := range s.Policies {
		if err := ValidateDigest(ps.Digest); err != nil {
			log.Warn("excluding policy with malformed digest, run complyctl get to re-fetch",
				"policy", key, "error", err)
			delete(s.Policies, key)
		}
	}
	for key, ps := range s.Complypacks {
		if err := ValidateDigest(ps.Digest); err != nil {
			log.Warn("excluding complypack with malformed digest, run complyctl get to re-fetch",
				"repository", key, "error", err)
			delete(s.Complypacks, key)
		}
	}
}

// initStateMaps ensures Policies and Complypacks maps are non-nil.
// Extracted to keep LoadState's cyclomatic complexity stable when new
// map fields are added to State.
func initStateMaps(s *State) {
	if s.Policies == nil {
		s.Policies = make(map[string]PolicyState)
	}
	if s.Complypacks == nil {
		s.Complypacks = make(map[string]PolicyState)
	}
}

// SaveState atomically writes the state to state.json in the given base
// directory. It marshals to a sibling temp file then renames it into place so
// concurrent readers never observe a truncated or partial write. The directory
// is created with 0700 permissions (user-only access) since state.json resides
// in the XDG data directory.
func SaveState(state *State, baseDir string) error {
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	statePath := filepath.Join(baseDir, complytime.StateFileName)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to a temp file in the same directory so os.Rename is atomic
	// (same filesystem, POSIX guarantee). This prevents concurrent readers
	// from observing a truncated file mid-write.
	tmp, err := os.CreateTemp(baseDir, ".state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp state file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp state file %s: %w", tmpPath, err)
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions on temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, statePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write state file %s: %w", statePath, err)
	}

	return nil
}

// UpdatePolicyStateWithVerification records version, digest, verification
// metadata, and current timestamp for a cached policy. When vr is nil,
// Verified is set to false (no verification was performed). Returns an
// error if the digest is non-empty and not a valid OCI content digest.
func (s *State) UpdatePolicyStateWithVerification(policyID, version, dgst string, vr *VerificationResult) error {
	if err := ValidateDigest(dgst); err != nil {
		return fmt.Errorf("policy %s: %w", policyID, err)
	}
	if s.Policies == nil {
		s.Policies = make(map[string]PolicyState)
	}
	ps := PolicyState{
		Version:     version,
		Digest:      dgst,
		LastUpdated: time.Now(),
	}
	if vr != nil {
		ps.Verified = vr.Verified
		ps.SignerIdentity = vr.SignerIdentity
		ps.Issuer = vr.Issuer
		ps.VerifiedAt = vr.VerifiedAt
	}
	s.Policies[policyID] = ps
	s.LastSync = time.Now()
	return nil
}

// UpdateComplypackStateWithVerification records version, digest, evaluator-id,
// verification metadata, and current timestamp for a cached complypack.
// Returns an error if the digest is non-empty and not a valid OCI content
// digest.
func (s *State) UpdateComplypackStateWithVerification(repository, version, dgst, evaluatorID string, vr *VerificationResult) error {
	if err := ValidateDigest(dgst); err != nil {
		return fmt.Errorf("complypack %s: %w", repository, err)
	}
	if s.Complypacks == nil {
		s.Complypacks = make(map[string]PolicyState)
	}
	ps := PolicyState{
		Version:     version,
		Digest:      dgst,
		EvaluatorID: evaluatorID,
		LastUpdated: time.Now(),
	}
	if vr != nil {
		ps.Verified = vr.Verified
		ps.SignerIdentity = vr.SignerIdentity
		ps.Issuer = vr.Issuer
		ps.VerifiedAt = vr.VerifiedAt
	}
	s.Complypacks[repository] = ps
	s.LastSync = time.Now()
	return nil
}

// GetPolicyState returns the cached state for a policy identified by policyID.
func (s *State) GetPolicyState(policyID string) (PolicyState, bool) {
	if s.Policies == nil {
		return PolicyState{}, false
	}
	state, exists := s.Policies[policyID]
	return state, exists
}

// GetComplypackState returns the cached state for a complypack, keyed by
// repository (e.g., "example.com/complypacks/opa-bundle").
func (s *State) GetComplypackState(repository string) (PolicyState, bool) {
	if s.Complypacks == nil {
		return PolicyState{}, false
	}
	state, exists := s.Complypacks[repository]
	return state, exists
}

// EvaluatorIDToVersion performs a reverse lookup on the Complypacks map,
// returning the active version for the given evaluator-id. State is keyed
// by repository, so this iterates all entries to find the matching
// evaluator-id. Returns ("", false, nil) when the evaluator-id is not
// found, or when the receiver or Complypacks map is nil/empty.
//
// Returns an error if multiple repositories reference the same
// evaluator-id, since Go map iteration order is non-deterministic and
// the result would be undefined. This invariant is currently enforced
// upstream by complyctl get's duplicate evaluator-id rejection.
func (s *State) EvaluatorIDToVersion(evaluatorID string) (string, bool, error) {
	if s == nil || len(s.Complypacks) == 0 {
		return "", false, nil
	}
	var found bool
	var version string
	for _, ps := range s.Complypacks {
		if ps.EvaluatorID == evaluatorID {
			if found {
				return "", false, fmt.Errorf(
					"duplicate evaluator-id %q in state: "+
						"multiple repositories reference the same evaluator",
					evaluatorID,
				)
			}
			version = ps.Version
			found = true
		}
	}
	return version, found, nil
}

// SetPolicyMetadata updates display-oriented metadata fields on an
// existing PolicyState entry without overwriting sync fields. No-ops
// when the repository key does not exist in the Policies map.
func (s *State) SetPolicyMetadata(
	repository, title, evaluator string,
	controls, assessments int,
) {
	ps, exists := s.Policies[repository]
	if !exists {
		return
	}
	ps.PolicyTitle = title
	ps.PolicyEvaluator = evaluator
	ps.ControlCount = controls
	ps.AssessmentCount = assessments
	s.Policies[repository] = ps
}
