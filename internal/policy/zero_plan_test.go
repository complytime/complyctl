// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"strings"
	"testing"

	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
)

// A governance Policy mandates guidance without inventing assessment plans.
const guidanceOnlyPolicyYAML = `title: Guidance governance
metadata:
  id: governance
  type: Policy
  gemara-version: 1.0.0
  version: "1.0"
  mapping-references:
    - id: governance-reference
      title: Governance reference
contacts:
  responsible:
    - name: Security team
  accountable:
    - name: Policy owner
scope:
  in:
    technologies: [linux]
imports:
  guidance:
    - reference-id: guidance
      constraints:
        - id: mandatory-guidance
          target-id: guide-1
          text: Follow the security guidance
implementation-plan:
  evaluation-timeline:
    start: "2026-01-01T00:00:00Z"
    notes: Review guidance adoption
  enforcement-timeline:
    start: "2026-02-01T00:00:00Z"
    notes: Guidance is mandatory
`

const governanceGuidanceYAML = `title: Security guidance
metadata:
  id: guidance
  type: GuidanceCatalog
  gemara-version: 1.0.0
guidelines:
  - id: guide-1
    title: Review changes
    objective: Review security implications before releasing changes
`

func TestResolvePolicyGraph_ZeroPlanRoot(t *testing.T) {
	for _, adherence := range []string{"", "adherence: {}\n", "adherence:\n  assessment-plans: []\n"} {
		for _, shape := range []string{"bundle", "split"} {
			t.Run(shape+"/"+adherence, func(t *testing.T) {
				data := []byte(guidanceOnlyPolicyYAML + adherence)
				guidance := []byte(governanceGuidanceYAML)
				catalog := []byte(testCatalogYAML("catalog"))
				var loader PolicyLoader
				if shape == "bundle" {
					loader = seedBundlePolicy(t, "governance", "v1", []bundle.File{
						{Name: "root.yaml", Type: "Policy", Data: data},
						{Name: "guidance.yaml", Type: "GuidanceCatalog", Data: guidance},
						{Name: "catalog.yaml", Type: "ControlCatalog", Data: catalog},
					})
					artifacts, err := loader.LoadBundleArtifacts("governance", "v1")
					require.NoError(t, err)
					assert.Equal(t, data, artifacts.Source.Data, "retain guidance imports/constraints")
				} else {
					ml := newMockLoader()
					ml.exists["governance/v1"] = true
					ml.layers["governance/v1/"+complytime.MediaTypePolicy] = data
					ml.layers["governance/v1/"+complytime.MediaTypeGuidance] = guidance
					ml.layers["governance/v1/"+complytime.MediaTypeCatalog] = catalog
					loader = ml
				}
				r := NewResolver(loader)
				// Metadata is independently usable before building a graph.
				meta, err := r.ExtractPolicyMetadata("governance", "v1")
				require.NoError(t, err)
				assert.Equal(t, PolicyMetadata{Title: "Guidance governance", ControlCount: 1}, meta)
				graph, err := r.ResolvePolicyGraph("governance", "v1")
				require.NoError(t, err)
				assert.Equal(t, "governance", graph.PolicyID)
				assert.Empty(t, graph.Assessments)
				assert.Empty(t, ExtractAssessmentConfigs(graph))
				assert.Empty(t, GroupByEvaluator(ExtractAssessmentConfigs(graph), graph))
				assert.Empty(t, graph.EvaluatorID)
				assert.Equal(t, []gemara.MappingReference{
					{Id: "governance-reference", Title: "Governance reference"},
				}, graph.MappingReferences)
				assert.Equal(t, &PolicyTimeline{
					EvaluationStart: "2026-01-01T00:00:00Z", EvaluationNotes: "Review guidance adoption",
					EnforcementStart: "2026-02-01T00:00:00Z", EnforcementNotes: "Guidance is mandatory",
				}, graph.Timeline)
				require.Len(t, graph.Guidelines, 1)
				assert.Equal(t, guidance, graph.Guidelines[0].Content)
				assert.Equal(t, "guidance", graph.Guidelines[0].Parsed.Metadata.Id)
				require.Len(t, graph.Guidelines[0].Parsed.Guidelines, 1)
				assert.Equal(t, "guide-1", graph.Guidelines[0].Parsed.Guidelines[0].Id)
				require.Len(t, graph.Controls, 1)
				assert.Equal(t, catalog, graph.Controls[0].Content)
				assert.Equal(t, "catalog-control", graph.Controls[0].Parsed.Controls[0].Id)
			})
		}
	}
}

func TestResolveBundleGraph_ZeroPlanImportedPolicy(t *testing.T) {
	for _, rootStart := range []string{"", "2027-01-01T00:00:00Z"} {
		t.Run(rootStart, func(t *testing.T) {
			parent := []byte(guidanceOnlyPolicyYAML)
			loader := seedBundlePolicy(t, "child", "v1", []bundle.File{
				{Name: "root.yaml", Type: "Policy",
					Data: []byte(testBundlePolicyYAML("child", "root-evaluator", rootStart))},
				{Name: "parent.yaml", Type: "Policy", Data: parent},
				{Name: "guidance.yaml", Type: "GuidanceCatalog", Data: []byte(governanceGuidanceYAML)},
				{Name: "a.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("a"))},
				{Name: "b.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("b"))},
			})
			r := NewResolver(loader)
			graph, err := r.ResolvePolicyGraph("child", "v1")
			require.NoError(t, err)
			assert.Equal(t, []Assessment{
				{ID: "child-plan", RequirementID: "child-req", EvaluatorID: "root-evaluator"},
			}, graph.Assessments)
			assert.Equal(t, "root-evaluator", graph.EvaluatorID)
			assert.Equal(t, []gemara.MappingReference{
				{Id: "child-mapping", Title: "child mapping"},
				{Id: "governance-reference", Title: "Governance reference"},
			}, graph.MappingReferences)
			require.NotNil(t, graph.Timeline)
			wantStart := rootStart
			if wantStart == "" {
				wantStart = "2026-01-01T00:00:00Z"
			}
			assert.Equal(t, wantStart, graph.Timeline.EvaluationStart)
			require.Len(t, graph.Controls, 2)
			assert.Equal(t, "a", graph.Controls[0].ID)
			assert.Equal(t, "b", graph.Controls[1].ID)
			require.Len(t, graph.Guidelines, 1)
			assert.Equal(t, "guidance", graph.Guidelines[0].ID)
			artifacts, err := loader.LoadBundleArtifacts("child", "v1")
			require.NoError(t, err)
			assert.Equal(t, parent, artifacts.ImportsByType("Policy")[0].Data)
			meta, err := r.ExtractPolicyMetadata("child", "v1")
			require.NoError(t, err)
			assert.Equal(t, PolicyMetadata{
				Title: "child", EvaluatorID: "root-evaluator", ControlCount: 2, AssessmentCount: 1,
			}, meta)
		})
	}
}

func TestResolveBundleGraph_ZeroPlanRootImportedDefaults(t *testing.T) {
	root := strings.Split(guidanceOnlyPolicyYAML, "implementation-plan:")[0]
	r := NewResolver(seedBundlePolicy(t, "governance", "v1", []bundle.File{
		{Name: "root.yaml", Type: "Policy", Data: []byte(root)},
		{Name: "parent.yaml", Type: "Policy",
			Data: []byte(testBundlePolicyYAML("parent", "parent-evaluator", "2025-01-01T00:00:00Z"))},
	}))
	graph, err := r.ResolvePolicyGraph("governance", "v1")
	require.NoError(t, err)
	assert.Equal(t, []Assessment{
		{ID: "parent-plan", RequirementID: "parent-req", EvaluatorID: "parent-evaluator"},
	}, graph.Assessments)
	assert.Empty(t, graph.EvaluatorID, "imports must not set whole-graph routing")
	require.NotNil(t, graph.Timeline)
	assert.Equal(t, "2025-01-01T00:00:00Z", graph.Timeline.EvaluationStart)
	meta, err := r.ExtractPolicyMetadata("governance", "v1")
	require.NoError(t, err)
	assert.Equal(t, PolicyMetadata{Title: "Guidance governance", AssessmentCount: 1}, meta)
}

func TestParsePolicyLayer_InvalidPolicyRoles(t *testing.T) {
	// Include invalid identity/type with plans: removing the old plan guard
	// without validating the artifact must not make these inputs acceptable.
	withPlans := testPolicyYAML("valid")
	cases := map[string]string{
		"empty": "", "null": "null", "empty mapping": "{}", "scalar": "hello", "sequence": "[]",
		"unrelated":          "hello: world\n",
		"malformed YAML":     "{not: valid: yaml: [}",
		"missing type":       strings.ReplaceAll(withPlans, "  type: Policy\n", ""),
		"wrong type":         strings.ReplaceAll(withPlans, "type: Policy", "type: ControlCatalog"),
		"unknown type":       strings.ReplaceAll(withPlans, "type: Policy", "type: Banana"),
		"missing ID":         strings.ReplaceAll(withPlans, "  id: valid\n", ""),
		"blank ID":           strings.ReplaceAll(withPlans, "  id: valid\n", "  id: '   '\n"),
		"mistyped metadata":  "metadata: []\n",
		"mistyped adherence": guidanceOnlyPolicyYAML + "adherence: wrong\n",
		"mistyped plans":     guidanceOnlyPolicyYAML + "adherence:\n  assessment-plans: wrong\n",
		"mistyped imports":   strings.ReplaceAll(guidanceOnlyPolicyYAML, "imports:\n  guidance:", "imports:\n  guidance: wrong\n  unused:"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parsePolicyLayer("invalid", []byte(data))
			require.ErrorContains(t, err, "policy invalid:")
			for _, role := range []string{"bundle root", "split root", "import"} {
				t.Run(role, func(t *testing.T) {
					ml := newMockLoader()
					ml.exists["invalid/v1"] = true
					if role == "split root" {
						ml.layers["invalid/v1/"+complytime.MediaTypePolicy] = []byte(data)
					} else {
						ml.bundleShape["invalid/v1"] = true
						artifacts := BundleArtifacts{Source: bundle.File{
							Name: "root.yaml", Type: "Policy", Data: []byte(data),
						}}
						if role == "import" {
							artifacts.Source.Data = []byte(withPlans)
							artifacts.Imports = []bundle.File{{Name: "bad-parent.yaml", Type: "Policy", Data: []byte(data)}}
						}
						ml.bundleArtifacts["invalid/v1"] = artifacts
					}
					r := NewResolver(ml)
					_, err := r.ResolvePolicyGraph("invalid", "v1")
					require.ErrorContains(t, err, "policy invalid:")
					if role == "import" {
						assert.Contains(t, err.Error(), "bad-parent.yaml")
					} else {
						_, err = r.ExtractPolicyMetadata("invalid", "v1")
						require.ErrorContains(t, err, "policy invalid:")
					}
				})
			}
		})
	}
}
