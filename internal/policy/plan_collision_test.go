// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
)

func collisionPolicyFile(name, evaluator string, plans ...[2]string) bundle.File {
	var data strings.Builder
	fmt.Fprintf(&data, "metadata: {id: %s, type: Policy}\nadherence:\n", name)
	fmt.Fprintf(&data, "  evaluation-methods: [{executor: {id: %s}}]\n", evaluator)
	data.WriteString("  assessment-plans:\n")
	for _, plan := range plans {
		fmt.Fprintf(&data, "    - id: %s\n      requirement-id: %s\n", plan[0], plan[1])
	}
	return bundle.File{Name: name + ".yaml", Type: "Policy", Data: []byte(data.String())}
}

func TestResolvePolicyGraph_RejectsPlanIDCollisions(t *testing.T) {
	root := collisionPolicyFile("root", "opa", [2]string{"check-1", "root-req"})
	parent := collisionPolicyFile("parent", "ampel", [2]string{"check-1", "parent-req"})
	other := collisionPolicyFile("other", "other", [2]string{"check-1", "other-req"})
	zero := bundle.File{Name: "zero.yaml", Type: "Policy",
		Data: []byte("metadata: {id: zero, type: Policy}\n")}
	for _, tc := range []struct {
		name  string
		files []bundle.File
		first string
		last  string
	}{
		{"root import", []bundle.File{root, parent},
			`"root.yaml" (requirement "root-req")`, `"parent.yaml" (requirement "parent-req")`},
		{"import import", []bundle.File{zero, parent, other},
			`"parent.yaml" (requirement "parent-req")`, `"other.yaml" (requirement "other-req")`},
		{"equal requirements different evaluators", []bundle.File{root,
			collisionPolicyFile("parent", "ampel", [2]string{"check-1", "root-req"})},
			`"root.yaml" (requirement "root-req")`, `"parent.yaml" (requirement "root-req")`},
		{"root internal", []bundle.File{collisionPolicyFile("root", "opa",
			[2]string{"check-1", "first-req"}, [2]string{"check-1", "last-req"})},
			`"root.yaml" (requirement "first-req")`, `"root.yaml" (requirement "last-req")`},
		{"root internal equal requirements", []bundle.File{collisionPolicyFile("root", "opa",
			[2]string{"check-1", "same-req"}, [2]string{"check-1", "same-req"})},
			`"root.yaml" (requirement "same-req")`, `"root.yaml" (requirement "same-req")`},
		{"import internal", []bundle.File{zero, collisionPolicyFile("parent", "opa",
			[2]string{"check-1", "first-req"}, [2]string{"check-1", "last-req"})},
			`"parent.yaml" (requirement "first-req")`, `"parent.yaml" (requirement "last-req")`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewResolver(seedBundlePolicy(t, "collision", "v1", tc.files))
			graph, err := r.ResolvePolicyGraph("collision", "v1")
			require.Error(t, err, "ambiguous plan IDs must fail before provider dispatch")
			assert.Nil(t, graph, "a partial graph must not escape")
			assert.Contains(t, err.Error(), `duplicate assessment plan ID "check-1"`)
			assert.Contains(t, err.Error(), tc.first+" and "+tc.last)
		})
	}
}

func TestResolvePolicyGraph_SplitRejectsInternalPlanIDCollisions(t *testing.T) {
	for _, lastReq := range []string{"last-req", "first-req"} {
		t.Run(lastReq, func(t *testing.T) {
			loader := newMockLoader()
			loader.exists["split/v1"] = true
			file := collisionPolicyFile("root", "opa",
				[2]string{"check-1", "first-req"}, [2]string{"check-1", lastReq})
			loader.layers["split/v1/"+complytime.MediaTypePolicy] = file.Data
			graph, err := NewResolver(loader).ResolvePolicyGraph("split", "v1")
			require.Error(t, err)
			assert.Nil(t, graph)
			assert.Contains(t, err.Error(), `duplicate assessment plan ID "check-1"`)
			assert.Contains(t, err.Error(), fmt.Sprintf(
				`"split" (requirement "first-req") and "split" (requirement %q)`, lastReq))
		})
	}
}

func TestResolvePolicyGraph_UniqueSinglePolicyPreservesPlans(t *testing.T) {
	file := collisionPolicyFile("root", "opa",
		[2]string{"check-1", "first-req"}, [2]string{"check-2", "last-req"})
	graph, err := NewResolver(seedBundlePolicy(t, "single", "v1", []bundle.File{file})).
		ResolvePolicyGraph("single", "v1")
	require.NoError(t, err)
	assert.Equal(t, []Assessment{
		{ID: "check-1", RequirementID: "first-req", EvaluatorID: "opa"},
		{ID: "check-2", RequirementID: "last-req", EvaluatorID: "opa"},
	}, graph.Assessments)
	groups := GroupByEvaluator(ExtractAssessmentConfigs(graph), graph)
	require.Len(t, groups, 1)
	require.Len(t, groups["opa"].Configs, 2)
	assert.Equal(t, "check-1", groups["opa"].Configs[0].PlanID)
	assert.Equal(t, "first-req", groups["opa"].Configs[0].RequirementID)
	assert.Equal(t, "check-2", groups["opa"].Configs[1].PlanID)
	assert.Equal(t, "last-req", groups["opa"].Configs[1].RequirementID)
}
