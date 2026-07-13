// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/complytime/complyctl/internal/complytime"
)

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

// LoadState reads and parses the state.json file from the given cache directory.
// Returns a fresh State with empty maps if the file does not exist.
func LoadState(cacheDir string) (*State, error) {
	statePath := filepath.Join(cacheDir, complytime.StateFileName)

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

	return &state, nil
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

// SaveState writes the state to state.json in the given cache directory.
func SaveState(state *State, cacheDir string) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	statePath := filepath.Join(cacheDir, complytime.StateFileName)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write state file %s: %w", statePath, err)
	}

	return nil
}

// UpdatePolicyStateWithVerification records version, digest, verification
// metadata, and current timestamp for a cached policy. When vr is nil,
// Verified is set to false (no verification was performed).
func (s *State) UpdatePolicyStateWithVerification(policyID, version, digest string, vr *VerificationResult) {
	if s.Policies == nil {
		s.Policies = make(map[string]PolicyState)
	}
	ps := PolicyState{
		Version:     version,
		Digest:      digest,
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
}

// UpdateComplypackStateWithVerification records version, digest, evaluator-id,
// verification metadata, and current timestamp for a cached complypack.
func (s *State) UpdateComplypackStateWithVerification(repository, version, digest, evaluatorID string, vr *VerificationResult) {
	if s.Complypacks == nil {
		s.Complypacks = make(map[string]PolicyState)
	}
	ps := PolicyState{
		Version:     version,
		Digest:      digest,
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
