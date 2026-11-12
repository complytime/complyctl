// SPDX-License-Identifier: Apache-2.0

//go:build acceptance

package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newOSCALRepeatedRequirementFixture links two schema-valid observations for
// the same requirement, with complete fixture evidence on only the first.
func newOSCALRepeatedRequirementFixture(t *testing.T) (
	map[string]interface{}, map[string]interface{}, oscalEvidenceExpectation,
) {
	t.Helper()
	document := newOSCALSchemaFixture(t)
	assessment := document["assessment-results"].(map[string]interface{})
	results, err := oscalObjects(assessment, "results")
	require.NoError(t, err)
	result := results[0]
	observations, err := oscalObjects(result, "observations")
	require.NoError(t, err)
	withEvidence := observations[0]
	fixture := newOSCALEvidenceFixture(t)
	withEvidence["props"] = fixture.observation["props"]
	withEvidence["relevant-evidence"] = []interface{}{fixture.evidence}
	withoutEvidence := make(map[string]interface{}, len(withEvidence))
	for key, value := range withEvidence {
		withoutEvidence[key] = value
	}
	withoutEvidence["uuid"] = "00000000-0000-4000-8000-000000000004"
	delete(withoutEvidence, "relevant-evidence")
	result["observations"] = []interface{}{withEvidence, withoutEvidence}
	result["findings"] = []interface{}{map[string]interface{}{
		"uuid":        "00000000-0000-4000-8000-000000000005",
		"title":       "Repeated requirement finding",
		"description": "Two assessment observations for the same requirement",
		"target": map[string]interface{}{
			"type": "objective-id", "target-id": fixture.expected.ControlID,
			"status": map[string]interface{}{"state": "satisfied"},
		},
		"related-observations": []interface{}{
			map[string]interface{}{"observation-uuid": withEvidence["uuid"]},
			map[string]interface{}{"observation-uuid": withoutEvidence["uuid"]},
		},
	}}
	return document, result, fixture.expected
}

// TestOSCALEvidence_RepeatedRequirement checks existential evidence matching
// across every link/observation ordering without combining partial provenance.
func TestOSCALEvidence_RepeatedRequirement(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, map[string]interface{}, map[string]interface{})
		want   string
	}{
		{"complete_evidence", func(*testing.T, map[string]interface{}, map[string]interface{}) {}, ""},
		{"absent_evidence", func(_ *testing.T, first, _ map[string]interface{}) {
			delete(first, "relevant-evidence")
		}, "relevant-evidence"},
		{"split_provenance", func(t *testing.T, first, second map[string]interface{}) {
			evidence, err := oscalObjects(first, "relevant-evidence")
			require.NoError(t, err)
			props, err := oscalObjects(evidence[0], "props")
			require.NoError(t, err)
			evidence[0]["props"] = []interface{}{props[0]}
			second["relevant-evidence"] = []interface{}{map[string]interface{}{
				"description": evidence[0]["description"],
				"props":       []interface{}{props[1], props[2]},
			}}
		}, "no matching relevant-evidence"},
	}
	orders := []struct {
		name                string
		reverseLinks        bool
		reverseObservations bool
	}{
		{"evidence_link_first/observation_first", false, false},
		{"evidence_link_first/observation_last", false, true},
		{"evidence_link_last/observation_first", true, false},
		{"evidence_link_last/observation_last", true, true},
	}
	for _, tt := range cases {
		for _, order := range orders {
			t.Run(tt.name+"/"+order.name, func(t *testing.T) {
				document, result, expected := newOSCALRepeatedRequirementFixture(t)
				observations, err := oscalObjects(result, "observations")
				require.NoError(t, err)
				tt.mutate(t, observations[0], observations[1])
				if order.reverseObservations {
					result["observations"] = []interface{}{observations[1], observations[0]}
				}
				findings, err := oscalObjects(result, "findings")
				require.NoError(t, err)
				links, err := oscalObjects(findings[0], "related-observations")
				require.NoError(t, err)
				if order.reverseLinks {
					findings[0]["related-observations"] = []interface{}{links[1], links[0]}
				}
				require.NoError(t, checkOSCALSchema(writeOSCALSchemaFixture(t, document)),
					"each positive and negative permutation must remain schema-valid")
				err = checkOSCALEvidence(result, expected)
				if tt.want == "" {
					assert.NoError(t, err, "any linked matching observation may carry the evidence")
				} else {
					assert.ErrorContains(t, err, tt.want)
				}
			})
		}
	}
}
