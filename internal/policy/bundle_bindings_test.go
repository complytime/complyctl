// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"strings"
	"testing"

	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

func TestResolveBundleGraph_RoutesDeclaringPolicyEvaluators(t *testing.T) {
	r := NewResolver(seedBundlePolicy(t, "routing", "v1", []bundle.File{
		{Name: "root.yaml", Type: "Policy", Data: []byte(testBundlePolicyYAML("root", "opa", ""))},
		{Name: "parent.yaml", Type: "Policy", Data: []byte(testBundlePolicyYAML("parent", "openscap", ""))},
	}))
	graph, err := r.ResolvePolicyGraph("routing", "v1")
	require.NoError(t, err)
	groups := GroupByEvaluator(ExtractAssessmentConfigs(graph), graph)
	require.Len(t, groups, 2)
	assert.Equal(t, []provider.AssessmentConfiguration{{
		PlanID: "root-plan", RequirementID: "root-req", EvaluatorID: "opa",
	}}, groups["opa"].Configs)
	assert.Equal(t, []provider.AssessmentConfiguration{{
		PlanID: "parent-plan", RequirementID: "parent-req", EvaluatorID: "openscap",
	}}, groups["openscap"].Configs)
}

func TestGroupByEvaluator_UsesGraphOnlyForUnboundConfigs(t *testing.T) {
	configs := []provider.AssessmentConfiguration{
		{PlanID: "bound", EvaluatorID: "openscap"}, {PlanID: "unbound"},
	}
	groups := GroupByEvaluator(configs, &DependencyGraph{EvaluatorID: "opa"})
	require.Len(t, groups, 2)
	assert.Equal(t, []provider.AssessmentConfiguration{configs[0]}, groups["openscap"].Configs)
	assert.Equal(t, []provider.AssessmentConfiguration{configs[1]}, groups["opa"].Configs)
}

func TestResolvedTreeDigest_AuthoredBundleParameters(t *testing.T) {
	const parameters = `      parameters:
        - id: severity
          label: Severity threshold
          description: Minimum severity
          accepted-values: [high, critical]
`
	parent := testPolicyYAML("parent") + parameters
	resolve := func(t *testing.T, content string) *DependencyGraph {
		t.Helper()
		r := NewResolver(seedBundlePolicy(t, "parameters", "v1", []bundle.File{
			{Name: "root.yaml", Type: "Policy", Data: []byte(testPolicyYAML("root"))},
			{Name: "parent.yaml", Type: "Policy", Data: []byte(content)},
		}))
		graph, err := r.ResolvePolicyGraph("parameters", "v1")
		require.NoError(t, err)
		return graph
	}
	graph := resolve(t, parent)
	before, err := ResolvedTreeDigest(graph)
	require.NoError(t, err)
	for _, tt := range []struct{ name, old, replacement string }{
		{"id", "id: severity", "id: priority"},
		{"label", "label: Severity threshold", "label: Priority threshold"},
		{"description", "description: Minimum severity", "description: Maximum severity"},
		{"accepted values", "[high, critical]", "[medium, high, critical]"},
		{"removed", parameters, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			changed := resolve(t, strings.Replace(parent, tt.old, tt.replacement, 1))
			after, err := ResolvedTreeDigest(changed)
			require.NoError(t, err)
			assert.NotEqual(t, before, after, "authored parameter changes must invalidate generation")
		})
	}
	for _, cfg := range ExtractAssessmentConfigs(graph) {
		assert.Empty(t, cfg.Parameters, "accepted values are declarations, not runtime overrides")
	}
}
