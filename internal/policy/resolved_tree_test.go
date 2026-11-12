// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"slices"
	"testing"

	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resolvedTreeFixture() *DependencyGraph {
	return &DependencyGraph{
		PolicyID: "multi", EvaluatorID: "opa",
		Controls: []Control{
			{ID: "catalog-b", Content: []byte("# original formatting"),
				Parsed: &gemara.ControlCatalog{Title: "B"}},
			{ID: "catalog-a", Parsed: &gemara.ControlCatalog{Title: "A",
				Controls: []gemara.Control{{Id: "C1", Objective: "Protect data"}}}},
		},
		Guidelines: []Guideline{
			{ID: "guidance-b", Parsed: &gemara.GuidanceCatalog{Title: "B"}},
			{ID: "guidance-a", Parsed: &gemara.GuidanceCatalog{Title: "A"}},
		},
		Assessments: []Assessment{
			{ID: "root-plan", RequirementID: "root-req", EvaluatorID: "opa"},
			{ID: "imported-plan", RequirementID: "imported-req", EvaluatorID: "opa",
				Parameters: map[string]string{"severity": "high", "mode": "strict"}},
		},
		Timeline:          &PolicyTimeline{EvaluationStart: "2026-01-01"},
		MappingReferences: []gemara.MappingReference{{Id: "ref", Version: "1"}},
	}
}

// Inventory order and YAML presentation must not invalidate equivalent generation inputs.
func TestResolvedTreeDigest_CanonicalWithoutMutation(t *testing.T) {
	graph := resolvedTreeFixture()
	digest, err := ResolvedTreeDigest(graph)
	require.NoError(t, err)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, digest)
	assert.Equal(t, "catalog-b", graph.Controls[0].ID)
	assert.Equal(t, "guidance-b", graph.Guidelines[0].ID)

	equivalent := resolvedTreeFixture()
	slices.Reverse(equivalent.Controls)
	slices.Reverse(equivalent.Guidelines)
	equivalent.Controls[1].Content = []byte("different: YAML\n# comments and whitespace")
	equivalent.Assessments[1].Parameters = map[string]string{"mode": "strict", "severity": "high"}
	equivalent.Assessments[0].Parameters = map[string]string{}
	for range 10 {
		got, err := ResolvedTreeDigest(equivalent)
		require.NoError(t, err)
		assert.Equal(t, digest, got)
	}
}

func TestResolvedTreeDigest_ResolvedChangesInvalidate(t *testing.T) {
	mutations := map[string]func(*DependencyGraph){
		"policy ID":             func(g *DependencyGraph) { g.PolicyID = "other" },
		"control ID":            func(g *DependencyGraph) { g.Controls[0].ID = "other" },
		"control content":       func(g *DependencyGraph) { g.Controls[1].Parsed.Controls[0].Objective = "Changed" },
		"guidance ID":           func(g *DependencyGraph) { g.Guidelines[0].ID = "other" },
		"guidance content":      func(g *DependencyGraph) { g.Guidelines[0].Parsed.Title = "Changed" },
		"imported plan added":   func(g *DependencyGraph) { g.Assessments = append(g.Assessments, Assessment{ID: "new-plan"}) },
		"imported plan removed": func(g *DependencyGraph) { g.Assessments = g.Assessments[:1] },
		"imported plan ID":      func(g *DependencyGraph) { g.Assessments[1].ID = "changed-plan" },
		"imported requirement":  func(g *DependencyGraph) { g.Assessments[1].RequirementID = "changed-req" },
		"imported evaluator":    func(g *DependencyGraph) { g.Assessments[1].EvaluatorID = "other" },
		"imported parameters":   func(g *DependencyGraph) { g.Assessments[1].Parameters["severity"] = "low" },
		"assessment precedence": func(g *DependencyGraph) { slices.Reverse(g.Assessments) },
		"effective evaluator":   func(g *DependencyGraph) { g.EvaluatorID = "other" },
		"timeline":              func(g *DependencyGraph) { g.Timeline.EnforcementNotes = "Changed" },
		"mapping references":    func(g *DependencyGraph) { g.MappingReferences[0].Version = "2" },
	}
	want, err := ResolvedTreeDigest(resolvedTreeFixture())
	require.NoError(t, err)
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			graph := resolvedTreeFixture()
			mutate(graph)
			got, err := ResolvedTreeDigest(graph)
			require.NoError(t, err)
			assert.NotEqual(t, want, got)
		})
	}
}

func TestResolvedTreeDigest_IncompleteGraph(t *testing.T) {
	for name, graph := range map[string]*DependencyGraph{
		"nil graph":         nil,
		"unparsed control":  {Controls: []Control{{ID: "missing"}}},
		"unparsed guidance": {Guidelines: []Guideline{{ID: "missing"}}},
	} {
		t.Run(name, func(t *testing.T) {
			digest, err := ResolvedTreeDigest(graph)
			require.Error(t, err)
			assert.Empty(t, digest)
		})
	}
}
