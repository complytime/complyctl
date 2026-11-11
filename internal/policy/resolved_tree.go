// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/gemaraproj/go-gemara"
)

// These snapshots exclude raw layer bytes and runtime inputs. Catalog inventories
// are unordered; assessment and mapping sequences retain resolution precedence.
type resolvedTree struct {
	PolicyID          string                    `json:"policy_id"`
	Controls          []resolvedCatalog         `json:"controls,omitempty"`
	Guidelines        []resolvedCatalog         `json:"guidelines,omitempty"`
	Assessments       []resolvedAssessment      `json:"assessments,omitempty"`
	EvaluatorID       string                    `json:"evaluator_id"`
	Timeline          *PolicyTimeline           `json:"timeline,omitempty"`
	MappingReferences []gemara.MappingReference `json:"mapping_references,omitempty"`
}

type resolvedCatalog struct {
	ID      string          `json:"id"`
	Catalog json.RawMessage `json:"catalog"`
}

type resolvedAssessment struct {
	ID            string            `json:"id"`
	RequirementID string            `json:"requirement_id"`
	EvaluatorID   string            `json:"evaluator_id"`
	Parameters    map[string]string `json:"parameters,omitempty"`
}

// ResolvedTreeDigest identifies resolved Gemara content independently of OCI
// packaging, YAML presentation, and workspace inputs. It does not mutate graph.
func ResolvedTreeDigest(graph *DependencyGraph) (string, error) {
	if graph == nil {
		return "", fmt.Errorf("cannot identify a nil resolved policy graph")
	}
	tree := resolvedTree{
		PolicyID: graph.PolicyID, EvaluatorID: graph.EvaluatorID,
		Timeline: graph.Timeline, MappingReferences: graph.MappingReferences,
	}
	for _, control := range graph.Controls {
		if control.Parsed == nil {
			return "", fmt.Errorf("control catalog %q is not parsed", control.ID)
		}
		catalog, err := snapshotCatalog(control.ID, control.Parsed)
		if err != nil {
			return "", err
		}
		tree.Controls = append(tree.Controls, catalog)
	}
	for _, guideline := range graph.Guidelines {
		if guideline.Parsed == nil {
			return "", fmt.Errorf("guidance catalog %q is not parsed", guideline.ID)
		}
		catalog, err := snapshotCatalog(guideline.ID, guideline.Parsed)
		if err != nil {
			return "", err
		}
		tree.Guidelines = append(tree.Guidelines, catalog)
	}
	sortCatalogs(tree.Controls)
	sortCatalogs(tree.Guidelines)
	for _, assessment := range graph.Assessments {
		tree.Assessments = append(tree.Assessments, resolvedAssessment(assessment))
	}
	// encoding/json sorts map keys, including assessment parameter maps.
	data, err := json.Marshal(tree)
	if err != nil {
		return "", fmt.Errorf("failed to encode resolved policy tree: %w", err)
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data)), nil
}

func snapshotCatalog(id string, parsed any) (resolvedCatalog, error) {
	data, err := json.Marshal(parsed)
	if err != nil {
		return resolvedCatalog{}, fmt.Errorf("failed to encode catalog %q: %w", id, err)
	}
	return resolvedCatalog{ID: id, Catalog: data}, nil
}

func sortCatalogs(catalogs []resolvedCatalog) {
	slices.SortFunc(catalogs, func(a, b resolvedCatalog) int {
		if order := cmp.Compare(a.ID, b.ID); order != 0 {
			return order
		}
		return bytes.Compare(a.Catalog, b.Catalog)
	})
}
