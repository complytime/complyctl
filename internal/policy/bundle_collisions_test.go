// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"strings"
	"testing"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSplitGraph_RejectsAmbiguousIdentities(t *testing.T) {
	for _, tc := range []struct{ name, catalog, policy, want string }{
		{"control", testCatalogYAML("a") + "  - id: a-control\n", testPolicyYAML("root"), "control"},
		{"requirement", testCatalogYAML("a") + "    assessment-requirements: [{id: shared}, {id: shared}]\n", testPolicyYAML("root"), "requirement"},
		{"reference", testCatalogYAML("a"), strings.Replace(testBundlePolicyYAML("root", "opa", ""),
			"title: root mapping", "title: root mapping\n    - id: root-mapping\n      title: conflicting", 1), "mapping reference"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loader := newMockLoader()
			loader.exists["split/v1"] = true
			loader.layers["split/v1/"+complytime.MediaTypeCatalog] = []byte(tc.catalog)
			loader.layers["split/v1/"+complytime.MediaTypePolicy] = []byte(tc.policy)
			graph, err := NewResolver(loader).ResolvePolicyGraph("split", "v1")
			require.ErrorContains(t, err, tc.want)
			assert.Nil(t, graph)
		})
	}
}

func TestResolveBundleGraph_RejectsAmbiguousCatalogIdentities(t *testing.T) {
	for _, tc := range []struct {
		name, first, second, kind string
	}{
		{"catalog", testCatalogYAML("same"), testCatalogYAML("same"), "catalog"},
		{"control", testCatalogYAML("a"), strings.ReplaceAll(testCatalogYAML("b"), "b-control", "a-control"), "control"},
		{"requirement", testCatalogYAML("a") + "    assessment-requirements: [{id: shared}]\n",
			testCatalogYAML("b") + "    assessment-requirements: [{id: shared}]\n", "requirement"},
	} {
		for _, reverse := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/forward", true: "/reverse"}[reverse], func(t *testing.T) {
				files := []bundle.File{
					{Name: "first.yaml", Type: "ControlCatalog", Data: []byte(tc.first)},
					{Name: "second.yaml", Type: "ControlCatalog", Data: []byte(tc.second)},
				}
				if reverse {
					files[0], files[1] = files[1], files[0]
				}
				r := NewResolver(seedBundlePolicy(t, "collision", "v1", append([]bundle.File{
					{Name: "root.yaml", Type: "Policy", Data: []byte(testPolicyYAML("root"))},
				}, files...)))
				graph, err := r.ResolvePolicyGraph("collision", "v1")
				require.ErrorContains(t, err, tc.kind)
				assert.Nil(t, graph)
				assert.Contains(t, err.Error(), "first.yaml")
				assert.Contains(t, err.Error(), "second.yaml")
				_, err = r.ExtractPolicyMetadata("collision", "v1")
				require.Error(t, err, "metadata must validate the same inventory")
			})
		}
	}
}

func TestResolveBundleGraph_RejectsIntraCatalogRequirementCollision(t *testing.T) {
	catalog := testCatalogYAML("a") + "    assessment-requirements: [{id: shared}, {id: shared}]\n"
	r := NewResolver(seedBundlePolicy(t, "collision", "v1", []bundle.File{
		{Name: "root.yaml", Type: "Policy", Data: []byte(testPolicyYAML("root"))},
		{Name: "catalog.yaml", Type: "ControlCatalog", Data: []byte(catalog)},
	}))
	_, err := r.ResolvePolicyGraph("collision", "v1")
	require.ErrorContains(t, err, "shared")
	assert.Contains(t, err.Error(), "catalog.yaml")
}

func TestResolveBundleGraph_MappingReferenceCollisions(t *testing.T) {
	for _, conflicting := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			t.Run(map[bool]string{false: "equal", true: "conflicting"}[conflicting]+map[bool]string{false: "/forward", true: "/reverse"}[reverse], func(t *testing.T) {
				first := testBundlePolicyYAML("root", "opa", "")
				second := testBundlePolicyYAML("parent", "opa", "")
				second = strings.ReplaceAll(second, "parent-mapping", "root-mapping")
				second = strings.ReplaceAll(second, "parent mapping", "root mapping")
				if conflicting {
					second = strings.ReplaceAll(second, "root mapping", "different resource")
				}
				files := []bundle.File{
					{Name: "root.yaml", Type: "Policy", Data: []byte(first)},
					{Name: "parent.yaml", Type: "Policy", Data: []byte(second)},
				}
				if reverse {
					files[0], files[1] = files[1], files[0]
				}
				r := NewResolver(seedBundlePolicy(t, "refs", "v1", files))
				graph, err := r.ResolvePolicyGraph("refs", "v1")
				if conflicting {
					require.ErrorContains(t, err, "root-mapping")
					assert.Contains(t, err.Error(), "root.yaml")
					assert.Contains(t, err.Error(), "parent.yaml")
					_, err = r.ExtractPolicyMetadata("refs", "v1")
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.Len(t, graph.MappingReferences, 1)
				assert.Equal(t, "root mapping", graph.MappingReferences[0].Title)
			})
		}
	}
}
