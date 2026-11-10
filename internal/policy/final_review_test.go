// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"testing"

	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
)

func TestResolvePolicyGraph_ImportingEvaluatorPrecedence(t *testing.T) {
	zero := "metadata: {id: root, type: Policy}\ntitle: Governance\n"
	multi := zero + `adherence:
  assessment-plans:
    - id: root-opa
      evaluation-methods: [{executor: {id: opa}}]
    - id: root-ampel
      evaluation-methods: [{executor: {id: ampel}}]
`
	for _, tc := range []struct {
		name string
		root string
		want map[string][]string
	}{
		{"root override", testBundlePolicyYAML("root", "opa", ""),
			map[string][]string{"opa": {"root-plan", "parent-plan", "other-plan"}}},
		{"multi root no shortcut", multi,
			map[string][]string{"opa": {"root-opa"}, "ampel": {"root-ampel", "parent-plan"}, "other": {"other-plan"}}},
		{"zero root no default", zero,
			map[string][]string{"ampel": {"parent-plan"}, "other": {"other-plan"}}},
		{"unbound root plan", testPolicyYAML("root"),
			map[string][]string{"": {"root-plan"}, "ampel": {"parent-plan"}, "other": {"other-plan"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewResolver(seedBundlePolicy(t, "routing", "v1", []bundle.File{
				{Name: "root.yaml", Type: "Policy", Data: []byte(tc.root)},
				{Name: "parent.yaml", Type: "Policy", Data: []byte(testBundlePolicyYAML("parent", "ampel", ""))},
				{Name: "other.yaml", Type: "Policy", Data: []byte(testBundlePolicyYAML("other", "other", ""))},
			}))
			graph, err := r.ResolvePolicyGraph("routing", "v1")
			require.NoError(t, err)
			got := map[string][]string{}
			for id, group := range GroupByEvaluator(ExtractAssessmentConfigs(graph), graph) {
				for _, cfg := range group.Configs {
					got[id] = append(got[id], cfg.PlanID)
				}
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPolicyParser_OneDocumentAllRoles(t *testing.T) {
	for _, suffix := range []string{testPolicyYAML("hidden"), "", "null\n", "[unterminated\n"} {
		for _, zero := range []bool{false, true} {
			t.Run(fmt.Sprintf("%q/zero=%v", suffix, zero), func(t *testing.T) {
				first := testPolicyYAML("root")
				if zero {
					first = "metadata: {id: root, type: Policy}\ntitle: Governance\n"
				}
				bad := []byte(first + "\n---\n" + suffix)
				for _, role := range []string{"bundle", "split", "import"} {
					t.Run(role, func(t *testing.T) {
						ml := newMockLoader()
						ml.exists["probe/v1"] = true
						if role == "split" {
							ml.layers["probe/v1/"+complytime.MediaTypePolicy] = bad
						} else {
							ml.bundleShape["probe/v1"] = true
							ml.bundleArtifacts["probe/v1"] = BundleArtifacts{Source: bundle.File{Name: "root.yaml", Type: "Policy", Data: bad}}
							if role == "import" {
								a := ml.bundleArtifacts["probe/v1"]
								a.Source.Data = []byte(testPolicyYAML("root"))
								a.Imports = []bundle.File{{Name: "extra.yaml", Type: "Policy", Data: bad}}
								ml.bundleArtifacts["probe/v1"] = a
							}
						}
						r := NewResolver(ml)
						_, err := r.ResolvePolicyGraph("probe", "v1")
						require.ErrorContains(t, err, "probe")
						if role == "import" {
							assert.Contains(t, err.Error(), "extra.yaml")
						} else {
							if role == "bundle" {
								assert.Contains(t, err.Error(), "root.yaml")
							}
							_, err = r.ExtractPolicyMetadata("probe", "v1")
							require.ErrorContains(t, err, "probe")
							if role == "bundle" {
								assert.Contains(t, err.Error(), "root.yaml")
							}
						}
					})
				}
			})
		}
	}
}

func TestSupportingCatalogParser_TypeBoundary(t *testing.T) {
	for _, typ := range []string{"ControlCatalog", "GuidanceCatalog"} {
		for name, raw := range map[string]string{
			"wrong type":   testPolicyYAML("wrong"),
			"null":         "null\n",
			"not artifact": "hello: world\n",
			"wrong field":  "metadata: {type: " + typ + "}\ncontrols: invalid\nguidelines: invalid\n",
		} {
			t.Run(typ+"/"+name, func(t *testing.T) {
				r := NewResolver(seedBundlePolicy(t, "probe", "v1", []bundle.File{
					{Name: "root.yaml", Type: "Policy", Data: []byte(testPolicyYAML("root"))},
					{Name: "wrong.yaml", Type: typ, Data: []byte(raw)},
				}))
				_, err := r.ResolvePolicyGraph("probe", "v1")
				require.ErrorContains(t, err, "wrong.yaml")
				assert.Contains(t, err.Error(), "probe")
				if typ == "ControlCatalog" {
					_, err = r.ExtractPolicyMetadata("probe", "v1")
					require.ErrorContains(t, err, "wrong.yaml")
				}
			})
		}
	}
}
