// SPDX-License-Identifier: Apache-2.0

//go:build acceptance

package acceptance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This is an inspection fixture, not a schema-validation fixture. Values match
// testdata/policy.yaml and cmd/test-provider/main.go; UUIDs are only local links.
const oscalEvidenceResultJSON = `{
  "findings": [
    {"target": {"target-id": "unrelated-control"}},
    {"target": {"target-id": "force-push-protection"},
     "related-observations": [{"observation-uuid": "assessment-observation"}]}
  ],
  "observations": [
    {"uuid": "unrelated-observation", "props": [
      {"ns": "https://github.com/gemaraproj/go-gemara/ns/oscal",
       "name": "requirement-id", "value": "unrelated-requirement"}]},
    {"uuid": "assessment-observation", "props": [
      {"ns": "https://github.com/gemaraproj/go-gemara/ns/oscal",
       "name": "requirement-id", "value": "block-force-push"}],
     "relevant-evidence": [{
       "description": "test evidence for block-force-push",
       "props": [
         {"ns": "https://github.com/gemaraproj/go-gemara/ns/oscal",
          "name": "source-reference-id", "value": "test-policy"},
         {"ns": "https://github.com/gemaraproj/go-gemara/ns/oscal",
          "name": "source-coordinate", "value": "/checks/block-force-push"},
         {"ns": "https://github.com/gemaraproj/go-gemara/ns/oscal",
          "name": "source-digest",
          "value": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}
       ]
     }]}
  ]
}`

type oscalEvidenceFixture struct {
	result      map[string]interface{}
	finding     map[string]interface{}
	observation map[string]interface{}
	unrelated   map[string]interface{}
	evidence    map[string]interface{}
	expected    oscalEvidenceExpectation
}

func newOSCALEvidenceFixture(t *testing.T) oscalEvidenceFixture {
	t.Helper()
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(oscalEvidenceResultJSON), &result))
	findings, err := oscalObjects(result, "findings")
	require.NoError(t, err)
	observations, err := oscalObjects(result, "observations")
	require.NoError(t, err)
	evidence, err := oscalObjects(observations[1], "relevant-evidence")
	require.NoError(t, err)
	return oscalEvidenceFixture{
		result: result, finding: findings[1], observation: observations[1],
		unrelated: observations[0], evidence: evidence[0],
		expected: oscalEvidenceExpectation{
			ControlID: "force-push-protection", RequirementID: "block-force-push",
			Description: "test evidence for block-force-push", ReferenceID: "test-policy",
			Coordinate: "/checks/block-force-push",
			Digest:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}
}

// TestOSCALEvidence_Valid verifies linkage does not depend on order or UUID values.
func TestOSCALEvidence_Valid(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(oscalEvidenceFixture)
	}{
		{"fixture", func(oscalEvidenceFixture) {}},
		{"reordered_observations", func(f oscalEvidenceFixture) {
			f.result["observations"] = []interface{}{f.observation, f.unrelated}
		}},
		{"different_UUID", func(f oscalEvidenceFixture) {
			f.observation["uuid"] = "another-generated-uuid"
			f.finding["related-observations"] = []interface{}{
				map[string]interface{}{"observation-uuid": "another-generated-uuid"},
			}
		}},
		{"unrelated_link_first", func(f oscalEvidenceFixture) {
			f.finding["related-observations"] = []interface{}{
				map[string]interface{}{"observation-uuid": "unrelated-observation"},
				map[string]interface{}{"observation-uuid": "assessment-observation"},
			}
		}},
		{"unrelated_evidence_first", func(f oscalEvidenceFixture) {
			f.observation["relevant-evidence"] = []interface{}{
				map[string]interface{}{"description": "another evidence item"}, f.evidence,
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newOSCALEvidenceFixture(t)
			tt.mutate(f)
			assert.NoError(t, checkOSCALEvidence(f.result, f.expected))
		})
	}
}

// TestOSCALEvidence_RejectsLoss covers FR-001 and FR-002 without the CLI or registry.
func TestOSCALEvidence_RejectsLoss(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(oscalEvidenceFixture)
		want   string
	}{
		{"absent_evidence", func(f oscalEvidenceFixture) {
			delete(f.observation, "relevant-evidence")
		}, "relevant-evidence must be an array"},
		{"empty_evidence", func(f oscalEvidenceFixture) {
			f.observation["relevant-evidence"] = []interface{}{}
		}, "relevant-evidence must not be empty"},
		{"null_evidence", func(f oscalEvidenceFixture) {
			f.observation["relevant-evidence"] = nil
		}, "relevant-evidence must be an array"},
		{"malformed_evidence", func(f oscalEvidenceFixture) {
			f.observation["relevant-evidence"] = []interface{}{"not an object"}
		}, "relevant-evidence[0] must be an object"},
		{"mismatched_description", func(f oscalEvidenceFixture) {
			f.evidence["description"] = "evidence from another assessment"
		}, "evidence description"},
		{"absent_links", func(f oscalEvidenceFixture) {
			delete(f.finding, "related-observations")
		}, "related-observations must be an array"},
		{"empty_links", func(f oscalEvidenceFixture) {
			f.finding["related-observations"] = []interface{}{}
		}, "no related observation for requirement"},
		{"dangling_link", func(f oscalEvidenceFixture) {
			f.finding["related-observations"] = []interface{}{
				map[string]interface{}{"observation-uuid": "missing-observation"},
			}
		}, `unresolved observation UUID "missing-observation"`},
		{"missing_link_UUID", func(f oscalEvidenceFixture) {
			f.finding["related-observations"] = []interface{}{map[string]interface{}{}}
		}, `unresolved observation UUID ""`},
		{"redirected_link", func(f oscalEvidenceFixture) {
			f.finding["related-observations"] = []interface{}{
				map[string]interface{}{"observation-uuid": "unrelated-observation"},
			}
			f.unrelated["relevant-evidence"] = f.observation["relevant-evidence"]
		}, "no related observation for requirement"},
		{"evidence_only_on_unrelated_assessment", func(f oscalEvidenceFixture) {
			f.unrelated["relevant-evidence"] = f.observation["relevant-evidence"]
			delete(f.observation, "relevant-evidence")
		}, "relevant-evidence must be an array"},
		{"wrong_finding", func(f oscalEvidenceFixture) {
			f.finding["target"] = map[string]interface{}{"target-id": "another-control"}
		}, `finding for control "force-push-protection" not found`},
		{"missing_requirement", func(f oscalEvidenceFixture) {
			delete(f.observation, "props")
		}, "no related observation for requirement"},
		{"duplicate_observation_UUID", func(f oscalEvidenceFixture) {
			f.unrelated["uuid"] = f.observation["uuid"]
		}, "empty or duplicated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newOSCALEvidenceFixture(t)
			tt.mutate(f)
			assert.ErrorContains(t, checkOSCALEvidence(f.result, f.expected), tt.want)
		})
	}
}

// TestOSCALEvidence_RejectsProvenanceLoss checks each populated source field independently.
func TestOSCALEvidence_RejectsProvenanceLoss(t *testing.T) {
	fields := []string{"source-reference-id", "source-coordinate", "source-digest"}
	mutations := []string{"absent", "empty", "mismatched", "wrong_namespace"}
	for i, field := range fields {
		for _, mutation := range mutations {
			t.Run(field+"/"+mutation, func(t *testing.T) {
				f := newOSCALEvidenceFixture(t)
				props, err := oscalObjects(f.evidence, "props")
				require.NoError(t, err)
				switch mutation {
				case "absent":
					delete(props[i], "name")
				case "empty":
					props[i]["value"] = ""
				case "mismatched":
					props[i]["value"] = "unexpected-value"
				case "wrong_namespace":
					props[i]["ns"] = "https://example.test/unrelated"
				}
				assert.ErrorContains(t, checkOSCALEvidence(f.result, f.expected), field)
			})
		}
	}
}

// TestOSCALEvidence_RejectsSplitProvenance prevents combining fields across evidence items.
func TestOSCALEvidence_RejectsSplitProvenance(t *testing.T) {
	f := newOSCALEvidenceFixture(t)
	props, err := oscalObjects(f.evidence, "props")
	require.NoError(t, err)
	f.evidence["props"] = []interface{}{props[0]}
	f.observation["relevant-evidence"] = []interface{}{
		f.evidence,
		map[string]interface{}{
			"description": f.expected.Description,
			"props":       []interface{}{props[1], props[2]},
		},
	}
	assert.ErrorContains(t, checkOSCALEvidence(f.result, f.expected), "no matching relevant-evidence")
}
