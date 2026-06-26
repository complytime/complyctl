// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T149: ExtractAssessmentConfigs tests ---

func TestExtractAssessmentConfigs_EmptyGraph(t *testing.T) {
	graph := &DependencyGraph{
		PolicyID:    "test",
		Assessments: []Assessment{},
	}
	configs := ExtractAssessmentConfigs(graph)
	assert.Empty(t, configs)
}

func TestExtractAssessmentConfigs_ThreeAssessments(t *testing.T) {
	graph := &DependencyGraph{
		PolicyID: "nist",
		Assessments: []Assessment{
			{ID: "ap-1", RequirementID: "req-1", EvaluatorID: "openscap", Parameters: map[string]string{"profile": "xccdf_ssg"}},
			{ID: "ap-2", RequirementID: "req-2", EvaluatorID: "openscap"},
			{ID: "ap-3", RequirementID: "req-3", EvaluatorID: "kube-eval"},
		},
	}
	configs := ExtractAssessmentConfigs(graph)
	require.Len(t, configs, 3)

	assert.Equal(t, "ap-1", configs[0].PlanID)
	assert.Equal(t, "req-1", configs[0].RequirementID)
	assert.Equal(t, "openscap", configs[0].EvaluatorID)
	assert.Equal(t, "xccdf_ssg", configs[0].Parameters["profile"])

	assert.Equal(t, "ap-2", configs[1].PlanID)
	assert.Equal(t, "req-2", configs[1].RequirementID)
	assert.Equal(t, "kube-eval", configs[2].EvaluatorID)
}

// --- T150: GroupByEvaluator tests ---

func TestGroupByEvaluator_SingleEvaluatorShortcut(t *testing.T) {
	graph := &DependencyGraph{EvaluatorID: "openscap"}
	configs := []provider.AssessmentConfiguration{
		{PlanID: "p1", RequirementID: "r1", EvaluatorID: "openscap"},
		{PlanID: "p1", RequirementID: "r2", EvaluatorID: "openscap"},
	}

	groups := GroupByEvaluator(configs, graph)
	require.Len(t, groups, 1)
	assert.Equal(t, "openscap", groups["openscap"].EvaluatorID)
	assert.Len(t, groups["openscap"].Configs, 2)
}

func TestGroupByEvaluator_MultiEvaluator(t *testing.T) {
	graph := &DependencyGraph{EvaluatorID: ""}
	configs := []provider.AssessmentConfiguration{
		{PlanID: "p1", RequirementID: "r1", EvaluatorID: "openscap"},
		{PlanID: "p1", RequirementID: "r2", EvaluatorID: "kube-eval"},
		{PlanID: "p1", RequirementID: "r3", EvaluatorID: "openscap"},
	}

	groups := GroupByEvaluator(configs, graph)
	require.Len(t, groups, 2)
	assert.Len(t, groups["openscap"].Configs, 2)
	assert.Len(t, groups["kube-eval"].Configs, 1)
}

func TestGroupByEvaluator_EmptyConfigs(t *testing.T) {
	graph := &DependencyGraph{}
	groups := GroupByEvaluator(nil, graph)
	assert.Empty(t, groups)
}
