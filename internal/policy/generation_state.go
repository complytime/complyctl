// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
)

// GenerationState tracks the policy cache digest at generation time for
// freshness detection. Persisted per policy at
// {workspace}/{WorkspaceDir}/generation/{policy-id}.json
// See R37: specs/001-gemara-native-workflow/research.md
type GenerationState struct {
	PolicyID          string            `json:"policy_id"`
	PolicyDigest      string            `json:"policy_digest"`
	ComplypackDigests map[string]string `json:"complypack_digests,omitempty"`
	GeneratedAt       string            `json:"generated_at"`
	EvaluatorIDs      []string          `json:"evaluator_ids"`
}

// SaveGenerationState persists a GenerationState to the generation directory.
// Creates the full directory path, including any subdirectories from nested
// policy IDs (e.g. "policies/cis-fedora-l1-workstation").
func SaveGenerationState(baseDir, policyID string, state *GenerationState) error {
	path := filepath.Join(baseDir, complytime.WorkspaceDir, "generation", policyID+".json")
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create generation state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal generation state: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write generation state: %w", err)
	}
	return nil
}

// LoadGenerationState reads a persisted GenerationState for the given policy.
// Returns nil (no error) when no state file exists.
func LoadGenerationState(baseDir, policyID string) (*GenerationState, error) {
	path := filepath.Join(baseDir, complytime.WorkspaceDir, "generation", policyID+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read generation state: %w", err)
	}

	var state GenerationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse generation state: %w", err)
	}
	return &state, nil
}

// IsFresh returns true when the persisted policy digest and complypack digests
// both match their current values from the cache.
func (s *GenerationState) IsFresh(currentDigest string, currentComplypackDigests map[string]string) bool {
	if s.PolicyDigest != currentDigest {
		return false
	}
	return maps.Equal(normalizeNilMap(s.ComplypackDigests), normalizeNilMap(currentComplypackDigests))
}

func normalizeNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// NewGenerationState creates a GenerationState with the current timestamp.
func NewGenerationState(policyID, digest string, evaluatorIDs []string, complypackDigests map[string]string) *GenerationState {
	return &GenerationState{
		PolicyID:          policyID,
		PolicyDigest:      digest,
		ComplypackDigests: complypackDigests,
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		EvaluatorIDs:      evaluatorIDs,
	}
}

// InvalidateForEvaluator removes generation state files that reference the
// given evaluator-id. This forces the next scan to trigger a fresh Generate
// cycle for any policy that used that evaluator. Returns nil if the generation
// directory does not exist. Walks subdirectories since policy IDs may contain
// path separators (e.g. "policies/cis-fedora-l1-workstation").
//
// Files that cannot be read or contain malformed JSON are skipped and reported
// in the returned warnings slice so the caller can log them for diagnostics.
func InvalidateForEvaluator(baseDir, evaluatorID string) (warnings []string, _ error) {
	genDir := filepath.Join(baseDir, complytime.WorkspaceDir, "generation")

	root, rootErr := os.OpenRoot(genDir)
	if rootErr != nil {
		if os.IsNotExist(rootErr) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open generation directory: %w", rootErr)
	}
	defer root.Close()

	err := filepath.WalkDir(genDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		relPath, relErr := filepath.Rel(genDir, path)
		if relErr != nil {
			return fmt.Errorf("failed to compute relative path for %s: %w", path, relErr)
		}
		data, readErr := root.ReadFile(relPath)
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("skipped unreadable file %s: %v", path, readErr))
			return nil
		}
		var state GenerationState
		if jsonErr := json.Unmarshal(data, &state); jsonErr != nil {
			warnings = append(warnings, fmt.Sprintf("skipped malformed JSON %s: %v", path, jsonErr))
			return nil
		}
		if slices.Contains(state.EvaluatorIDs, evaluatorID) {
			if rmErr := root.Remove(relPath); rmErr != nil && !os.IsNotExist(rmErr) {
				return fmt.Errorf("failed to remove generation state %s: %w", path, rmErr)
			}
		}
		return nil
	})
	return warnings, err
}

// RemoveEvaluatorArtifacts removes the workspace evaluator artifact directory
// ({baseDir}/.complytime/{evaluatorID}/). Returns nil if the directory does
// not exist. Validates evaluatorID as a safe path component before constructing
// the target path.
func RemoveEvaluatorArtifacts(baseDir, evaluatorID string) error {
	if err := cache.ValidatePathComponent(evaluatorID); err != nil {
		return fmt.Errorf("invalid evaluator ID: %w", err)
	}
	dir := filepath.Join(baseDir, complytime.WorkspaceDir, evaluatorID)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove evaluator artifacts %s: %w", dir, err)
	}
	return nil
}
