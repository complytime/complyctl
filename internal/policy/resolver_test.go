// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"testing"

	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPolicyLoader satisfies PolicyLoader for resolver unit tests.
type mockPolicyLoader struct {
	layers          map[string][]byte            // key: "policyID/version/mediaType"
	exists          map[string]bool              // key: "policyID/version"
	versions        map[string]string            // key: "policyID/configVersion" → resolved version
	bundleFiles     map[string]map[string][]byte // key: "policyID/version" → type → data
	bundleArtifacts map[string]BundleArtifacts
	bundleShape     map[string]bool  // key: "policyID/version" → true if bundle
	bundleShapeErr  map[string]error // key: "policyID/version" → error for DetectManifestShape
}

func newMockLoader() *mockPolicyLoader {
	return &mockPolicyLoader{
		layers:          make(map[string][]byte),
		exists:          make(map[string]bool),
		versions:        make(map[string]string),
		bundleFiles:     make(map[string]map[string][]byte),
		bundleArtifacts: make(map[string]BundleArtifacts),
		bundleShape:     make(map[string]bool),
		bundleShapeErr:  make(map[string]error),
	}
}

func (m *mockPolicyLoader) LoadLayerByMediaType(policyID, version, mediaType string) ([]byte, error) {
	key := policyID + "/" + version + "/" + mediaType
	if data, ok := m.layers[key]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("layer %s not found", key)
}

func (m *mockPolicyLoader) LoadBundleFiles(policyID, version string) (map[string][]byte, error) {
	key := policyID + "/" + version
	if files, ok := m.bundleFiles[key]; ok {
		return files, nil
	}
	return nil, fmt.Errorf("bundle files not found for %s", key)
}

func (m *mockPolicyLoader) LoadBundleArtifacts(policyID, version string) (BundleArtifacts, error) {
	key := policyID + "/" + version
	if artifacts, ok := m.bundleArtifacts[key]; ok {
		return artifacts, nil
	}
	files, err := m.LoadBundleFiles(policyID, version)
	if err != nil {
		return BundleArtifacts{}, err
	}
	var artifacts BundleArtifacts
	seen := make(map[string]bool)
	for _, typ := range []string{"Policy", "ControlCatalog", "GuidanceCatalog"} {
		if data, ok := files[typ]; ok {
			file := bundle.File{Name: typ + ".yaml", Type: typ, Data: data}
			if artifacts.Source.Name == "" {
				artifacts.Source = file
			} else {
				artifacts.Imports = append(artifacts.Imports, file)
			}
			seen[typ] = true
		}
	}
	for typ, data := range files {
		if seen[typ] {
			continue
		}
		file := bundle.File{Name: typ + ".yaml", Type: typ, Data: data}
		if artifacts.Source.Name == "" {
			artifacts.Source = file
		} else {
			artifacts.Imports = append(artifacts.Imports, file)
		}
	}
	return artifacts, nil
}

func (m *mockPolicyLoader) DetectManifestShape(policyID, version string) (bool, error) {
	key := policyID + "/" + version
	if err, ok := m.bundleShapeErr[key]; ok {
		return false, err
	}
	return m.bundleShape[key], nil
}

func (m *mockPolicyLoader) PolicyExists(policyID, version string) bool {
	return m.exists[policyID+"/"+version]
}

func (m *mockPolicyLoader) ResolveVersion(policyID, configVersion string) (string, error) {
	key := policyID + "/" + configVersion
	if v, ok := m.versions[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("policy %s@%s not in cache", policyID, configVersion)
}

// --- T146: ResolvePolicyGraph tests ---

func TestResolvePolicyGraph_EmptyPolicyID(t *testing.T) {
	r := NewResolver(newMockLoader())
	_, err := r.ResolvePolicyGraph("", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy ID cannot be empty")
}

func TestResolvePolicyGraph_EmptyVersion(t *testing.T) {
	r := NewResolver(newMockLoader())
	_, err := r.ResolvePolicyGraph("test-policy", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version cannot be empty")
}

func TestResolvePolicyGraph_PolicyNotInCache(t *testing.T) {
	r := NewResolver(newMockLoader())
	_, err := r.ResolvePolicyGraph("missing-policy", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy not found")
}

func TestResolvePolicyGraph_AllThreeLayers(t *testing.T) {
	ml := newMockLoader()
	ml.exists["test-policy/v1"] = true

	ml.layers["test-policy/v1/application/vnd.gemara.catalog.v1+yaml"] = []byte(`
title: Test Catalog
metadata:
  type: ControlCatalog
  id: cat-1
  version: "1.0"
controls: []
`)

	ml.layers["test-policy/v1/application/vnd.gemara.guidance.v1+yaml"] = []byte(`
title: Test Guidance
metadata:
  type: GuidanceCatalog
  id: guide-1
  version: "1.0"
guidelines: []
`)

	ml.layers["test-policy/v1/application/vnd.gemara.policy.v1+yaml"] = []byte(`
title: Test Policy
metadata:
  id: pol-1
  type: Policy
  gemara-version: 1.0.0
  version: "1.0"
contacts:
  responsible:
    - name: team-a
  accountable:
    - name: team-b
scope:
  in:
    technologies:
      - linux
imports:
  catalogs:
    - reference-id: cat-1
adherence:
  assessment-plans:
    - id: ap-1
      requirement-id: req-1
      frequency: daily
      evaluation-methods:
        - id: openscap-eval
          type: Behavioral
          mode: Automated
          executor:
            id: openscap
`)

	r := NewResolver(ml)
	graph, err := r.ResolvePolicyGraph("test-policy", "v1")
	require.NoError(t, err)
	assert.Equal(t, "test-policy", graph.PolicyID)
	assert.Len(t, graph.Controls, 1)
	assert.Len(t, graph.Guidelines, 1)
	assert.Len(t, graph.Assessments, 1)
	assert.Equal(t, "openscap", graph.EvaluatorID)
	assert.Equal(t, "ap-1", graph.Assessments[0].ID)

	// Verify parsed content (matches bundle-path test pattern)
	require.NotNil(t, graph.Controls[0].Parsed)
	assert.Equal(t, "cat-1", graph.Controls[0].Parsed.Metadata.Id)
	require.NotNil(t, graph.Guidelines[0].Parsed)
	assert.Equal(t, "guide-1", graph.Guidelines[0].Parsed.Metadata.Id)
}

func TestResolvePolicyGraph_MissingOptionalLayers(t *testing.T) {
	ml := newMockLoader()
	ml.exists["minimal/v1"] = true

	ml.layers["minimal/v1/application/vnd.gemara.policy.v1+yaml"] = []byte(`
title: Minimal Policy
metadata:
  id: pol-min
  type: Policy
  gemara-version: 1.0.0
  version: "1.0"
contacts:
  responsible:
    - name: team-a
  accountable:
    - name: team-b
scope:
  in:
    technologies:
      - linux
imports:
  catalogs:
    - reference-id: external
adherence:
  assessment-plans:
    - id: ap-min
      requirement-id: req-min
      frequency: weekly
      evaluation-methods:
        - id: kube-eval-method
          type: Behavioral
          mode: Automated
          executor:
            id: kube-eval
`)

	r := NewResolver(ml)
	graph, err := r.ResolvePolicyGraph("minimal", "v1")
	require.NoError(t, err)
	assert.Empty(t, graph.Controls)
	assert.Empty(t, graph.Guidelines)
	assert.Len(t, graph.Assessments, 1)
	assert.Equal(t, "kube-eval", graph.EvaluatorID)
}

// --- T147: parsePolicyLayer tests ---

func TestParsePolicyLayer_InvalidYAML(t *testing.T) {
	_, err := parsePolicyLayer("bad", []byte("{not: valid: yaml: [}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid Gemara Policy YAML")
}

// Replaces the R39 rejection: the user clarified that roots may mandate
// guidance without any assessment plans. Zero plans are not invalid input.
func TestParsePolicyLayer_EmptyAdherence(t *testing.T) {
	yamlData := []byte(`
title: Empty Adherence
metadata:
  id: pol-empty
  type: Policy
  gemara-version: 1.0.0
  version: "1.0"
contacts:
  responsible:
    - name: team-a
  accountable:
    - name: team-b
scope:
  in:
    technologies:
      - linux
imports:
  catalogs:
    - reference-id: cat-1
adherence: {}
`)
	result, err := parsePolicyLayer("pol-empty", yamlData)
	require.NoError(t, err)
	assert.Equal(t, "Empty Adherence", result.Title)
	assert.Empty(t, result.Assessments)
	assert.Empty(t, result.EvaluatorID)
}

func TestParsePolicyLayer_SingleAssessmentPlan(t *testing.T) {
	yamlData := []byte(`
title: Single Plan
metadata:
  id: pol-single
  type: Policy
  gemara-version: 1.0.0
  version: "1.0"
contacts:
  responsible:
    - name: team-a
  accountable:
    - name: team-b
scope:
  in:
    technologies:
      - linux
imports:
  catalogs:
    - reference-id: cat-1
adherence:
  assessment-plans:
    - id: ap-1
      requirement-id: req-1
      frequency: daily
      evaluation-methods:
        - id: openscap-eval
          type: Behavioral
          mode: Automated
          executor:
            id: openscap
`)
	result, err := parsePolicyLayer("pol-single", yamlData)
	require.NoError(t, err)
	assert.Equal(t, "openscap", result.EvaluatorID)
	assert.Len(t, result.Assessments, 1)
	assert.Equal(t, "ap-1", result.Assessments[0].ID)
	assert.Equal(t, "req-1", result.Assessments[0].RequirementID)
	assert.Equal(t, "openscap", result.Assessments[0].EvaluatorID)
}

func TestParsePolicyLayer_MultiEvaluator(t *testing.T) {
	yamlData := []byte(`
title: Multi Evaluator
metadata:
  id: pol-multi
  type: Policy
  gemara-version: 1.0.0
  version: "1.0"
contacts:
  responsible:
    - name: team-a
  accountable:
    - name: team-b
scope:
  in:
    technologies:
      - linux
imports:
  catalogs:
    - reference-id: cat-1
adherence:
  assessment-plans:
    - id: ap-1
      requirement-id: req-1
      frequency: daily
      evaluation-methods:
        - id: openscap-eval
          type: Behavioral
          mode: Automated
          executor:
            id: openscap
    - id: ap-2
      requirement-id: req-2
      frequency: weekly
      evaluation-methods:
        - id: kube-eval-method
          type: Behavioral
          mode: Automated
          executor:
            id: kube-eval
`)
	result, err := parsePolicyLayer("pol-multi", yamlData)
	require.NoError(t, err)
	assert.Empty(t, result.EvaluatorID, "result-level EvaluatorID should be empty for mixed evaluators")
	assert.Len(t, result.Assessments, 2)
	assert.Equal(t, "openscap", result.Assessments[0].EvaluatorID)
	assert.Equal(t, "kube-eval", result.Assessments[1].EvaluatorID)
}

// --- T148: extractFromGemaraPolicy tests ---

func TestExtractFromGemaraPolicy_SingleEvaluator(t *testing.T) {
	p := &gemara.Policy{
		Adherence: gemara.Adherence{
			AssessmentPlans: []gemara.AssessmentPlan{
				{
					Id:            "ap-1",
					RequirementId: "req-1",
					Frequency:     "daily",
					EvaluationMethods: []gemara.AcceptedMethod{
						{Mode: gemara.ModeAutomated, Executor: gemara.Actor{Id: "openscap"}},
					},
				},
				{
					Id:            "ap-2",
					RequirementId: "req-2",
					Frequency:     "weekly",
					EvaluationMethods: []gemara.AcceptedMethod{
						{Mode: gemara.ModeAutomated, Executor: gemara.Actor{Id: "openscap"}},
					},
				},
			},
		},
	}

	result := extractFromGemaraPolicy(p)
	assert.Equal(t, "openscap", result.EvaluatorID)
	assert.Len(t, result.Assessments, 2)
	assert.Equal(t, "ap-1", result.Assessments[0].ID)
	assert.Equal(t, "req-1", result.Assessments[0].RequirementID)
	assert.Equal(t, "ap-2", result.Assessments[1].ID)
	assert.Equal(t, "req-2", result.Assessments[1].RequirementID)
}

func TestExtractFromGemaraPolicy_MixedEvaluators(t *testing.T) {
	p := &gemara.Policy{
		Adherence: gemara.Adherence{
			AssessmentPlans: []gemara.AssessmentPlan{
				{
					Id:            "ap-1",
					RequirementId: "req-1",
					Frequency:     "daily",
					EvaluationMethods: []gemara.AcceptedMethod{
						{Mode: gemara.ModeAutomated, Executor: gemara.Actor{Id: "openscap"}},
					},
				},
				{
					Id:            "ap-2",
					RequirementId: "req-2",
					Frequency:     "weekly",
					EvaluationMethods: []gemara.AcceptedMethod{
						{Mode: gemara.ModeAutomated, Executor: gemara.Actor{Id: "kube-eval"}},
					},
				},
			},
		},
	}

	result := extractFromGemaraPolicy(p)
	assert.Empty(t, result.EvaluatorID)
	assert.Equal(t, "openscap", result.Assessments[0].EvaluatorID)
	assert.Equal(t, "kube-eval", result.Assessments[1].EvaluatorID)
	assert.Equal(t, "req-1", result.Assessments[0].RequirementID)
	assert.Equal(t, "req-2", result.Assessments[1].RequirementID)
}

func TestExtractFromGemaraPolicy_Timeline(t *testing.T) {
	p := &gemara.Policy{
		ImplementationPlan: gemara.ImplementationPlan{
			EvaluationTimeline: gemara.ImplementationDetails{
				Start: "2026-01-01",
				End:   "2026-12-31",
				Notes: "eval notes",
			},
			EnforcementTimeline: gemara.ImplementationDetails{
				Start: "2026-06-01",
				Notes: "enforce notes",
			},
		},
		Adherence: gemara.Adherence{
			AssessmentPlans: []gemara.AssessmentPlan{
				{
					Id:        "ap-1",
					Frequency: "daily",
					EvaluationMethods: []gemara.AcceptedMethod{
						{Mode: gemara.ModeAutomated, Executor: gemara.Actor{Id: "test"}},
					},
				},
			},
		},
	}

	result := extractFromGemaraPolicy(p)
	require.NotNil(t, result.Timeline)
	assert.Equal(t, "2026-01-01", result.Timeline.EvaluationStart)
	assert.Equal(t, "2026-12-31", result.Timeline.EvaluationEnd)
	assert.Equal(t, "eval notes", result.Timeline.EvaluationNotes)
	assert.Equal(t, "2026-06-01", result.Timeline.EnforcementStart)
	assert.Equal(t, "enforce notes", result.Timeline.EnforcementNotes)
}

func TestExtractFromGemaraPolicy_NoTimeline(t *testing.T) {
	p := &gemara.Policy{
		Adherence: gemara.Adherence{
			AssessmentPlans: []gemara.AssessmentPlan{
				{
					Id:        "ap-1",
					Frequency: "daily",
					EvaluationMethods: []gemara.AcceptedMethod{
						{Mode: gemara.ModeAutomated, Executor: gemara.Actor{Id: "test"}},
					},
				},
			},
		},
	}

	result := extractFromGemaraPolicy(p)
	assert.Nil(t, result.Timeline)
}

func TestExtractFromGemaraPolicy_PolicyLevelFallback(t *testing.T) {
	p := &gemara.Policy{
		Adherence: gemara.Adherence{
			EvaluationMethods: []gemara.AcceptedMethod{
				{Mode: gemara.ModeAutomated, Executor: gemara.Actor{Id: "policy-level"}},
			},
			AssessmentPlans: []gemara.AssessmentPlan{
				{
					Id:                "ap-1",
					Frequency:         "daily",
					EvaluationMethods: nil,
				},
			},
		},
	}

	result := extractFromGemaraPolicy(p)
	assert.Equal(t, "policy-level", result.EvaluatorID)
	assert.Equal(t, "policy-level", result.Assessments[0].EvaluatorID)
}

// --- T235: Resolver error surfacing tests ---

func TestResolvePolicyGraph_InvalidCatalogYAML(t *testing.T) {
	ml := newMockLoader()
	ml.exists["broken-cat/v1"] = true

	ml.layers["broken-cat/v1/application/vnd.gemara.catalog.v1+yaml"] = []byte("{not: valid: yaml: [}")

	ml.layers["broken-cat/v1/application/vnd.gemara.policy.v1+yaml"] = validPolicyYAML()

	r := NewResolver(ml)
	_, err := r.ResolvePolicyGraph("broken-cat", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog layer is not valid Gemara")
}

func TestResolvePolicyGraph_InvalidGuidanceYAML(t *testing.T) {
	ml := newMockLoader()
	ml.exists["broken-guide/v1"] = true

	ml.layers["broken-guide/v1/application/vnd.gemara.guidance.v1+yaml"] = []byte("{not: valid: yaml: [}")

	ml.layers["broken-guide/v1/application/vnd.gemara.policy.v1+yaml"] = validPolicyYAML()

	r := NewResolver(ml)
	_, err := r.ResolvePolicyGraph("broken-guide", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "guidance layer is not valid Gemara")
}

func TestResolvePolicyGraph_CatalogLoadError_PartialGraph(t *testing.T) {
	ml := newMockLoader()
	ml.exists["partial/v1"] = true

	ml.layers["partial/v1/application/vnd.gemara.guidance.v1+yaml"] = []byte(`
title: Test Guidance
metadata:
  type: GuidanceCatalog
  id: guide-1
  version: "1.0"
guidelines: []
`)

	ml.layers["partial/v1/application/vnd.gemara.policy.v1+yaml"] = validPolicyYAML()

	r := NewResolver(ml)
	graph, err := r.ResolvePolicyGraph("partial", "v1")
	require.NoError(t, err)
	assert.Empty(t, graph.Controls, "catalog load failure should result in no controls")
	assert.Len(t, graph.Guidelines, 1)
	assert.Len(t, graph.Assessments, 1)
}

func TestResolvePolicyGraph_PolicyLayerLoadError(t *testing.T) {
	ml := newMockLoader()
	ml.exists["no-policy/v1"] = true

	ml.layers["no-policy/v1/application/vnd.gemara.catalog.v1+yaml"] = []byte(`
title: Test Catalog
metadata:
  type: ControlCatalog
  id: cat-1
  version: "1.0"
controls: []
`)

	r := NewResolver(ml)
	_, err := r.ResolvePolicyGraph("no-policy", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load policy layer")
}

// --- Bundle-shape resolver tests (US1: 005-bundle-resolver-alignment) ---

func testBundlePolicyYAML(id, evaluator, start string) string {
	return fmt.Sprintf(`metadata:
  id: %s
  type: Policy
  mapping-references:
    - id: %s-mapping
      title: %s mapping
title: %s
adherence:
  evaluation-methods:
    - executor:
        id: %q
  assessment-plans:
    - id: %s-plan
      requirement-id: %s-req
implementation-plan:
  evaluation-timeline:
    start: %q
`, id, id, id, id, evaluator, id, id, start)
}

func testGuidanceYAML(id string) string {
	return fmt.Sprintf("metadata:\n  id: %s\n  type: GuidanceCatalog\nguidelines: []\n", id)
}

func TestResolveBundleGraphMultipleCatalogsAndParentPolicy(t *testing.T) {
	files := []bundle.File{
		{Name: "root.yaml", Type: "Policy", Data: []byte(testBundlePolicyYAML(
			"root-policy", "root-evaluator", "2026-01-01T00:00:00Z"))},
		{Name: "catalog-a.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("catalog-a"))},
		{Name: "catalog-b.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("catalog-b") + "  - id: extra-control\n")},
		{Name: "guidance-a.yaml", Type: "GuidanceCatalog", Data: []byte(testGuidanceYAML("guidance-a"))},
		{Name: "parent.yaml", Type: "Policy", Data: []byte(testBundlePolicyYAML(
			"parent-policy", "parent-evaluator", "2025-01-01T00:00:00Z"))},
		{Name: "guidance-b.yaml", Type: "GuidanceCatalog", Data: []byte(testGuidanceYAML("guidance-b"))},
		{Name: "grandparent.yaml", Type: "Policy", Data: []byte(testBundlePolicyYAML(
			"grandparent-policy", "grandparent-evaluator", "2024-01-01T00:00:00Z"))},
	}
	r := NewResolver(seedBundlePolicy(t, "multi", "v1", files))
	graph, err := r.ResolvePolicyGraph("multi", "v1")
	require.NoError(t, err)
	assert.Equal(t, "multi", graph.PolicyID)
	require.Len(t, graph.Controls, 2)
	for i, tt := range []struct {
		id   string
		file bundle.File
	}{{"catalog-a", files[1]}, {"catalog-b", files[2]}} {
		assert.Equal(t, tt.id, graph.Controls[i].ID)
		assert.Equal(t, tt.file.Data, graph.Controls[i].Content)
		assert.Equal(t, tt.id, graph.Controls[i].Parsed.Metadata.Id)
		require.Len(t, graph.Controls[i].Parsed.Controls, i+1)
		assert.Equal(t, tt.id+"-control", graph.Controls[i].Parsed.Controls[0].Id)
	}
	require.Len(t, graph.Guidelines, 2)
	assert.Equal(t, "guidance-a", graph.Guidelines[0].ID)
	assert.Equal(t, "guidance-b", graph.Guidelines[1].ID)
	assert.Equal(t, files[3].Data, graph.Guidelines[0].Content)
	assert.Equal(t, files[5].Data, graph.Guidelines[1].Content)
	assert.Equal(t, "guidance-b", graph.Guidelines[1].Parsed.Metadata.Id)
	assert.Equal(t, []Assessment{
		{ID: "root-policy-plan", RequirementID: "root-policy-req", EvaluatorID: "root-evaluator"},
		{ID: "parent-policy-plan", RequirementID: "parent-policy-req", EvaluatorID: "parent-evaluator"},
		{ID: "grandparent-policy-plan", RequirementID: "grandparent-policy-req",
			EvaluatorID: "grandparent-evaluator"},
	}, graph.Assessments)
	assert.Equal(t, []gemara.MappingReference{
		{Id: "root-policy-mapping", Title: "root-policy mapping"},
		{Id: "parent-policy-mapping", Title: "parent-policy mapping"},
		{Id: "grandparent-policy-mapping", Title: "grandparent-policy mapping"},
	}, graph.MappingReferences)
	assert.Equal(t, "root-evaluator", graph.EvaluatorID)
	require.NotNil(t, graph.Timeline)
	assert.Equal(t, "2026-01-01T00:00:00Z", graph.Timeline.EvaluationStart)
	metadata, err := r.ExtractPolicyMetadata("multi", "v1")
	require.NoError(t, err)
	assert.Equal(t, PolicyMetadata{
		Title: "root-policy", EvaluatorID: "root-evaluator", ControlCount: 3, AssessmentCount: 1,
	}, metadata)
}

func TestResolveBundleGraph_ImportedDefaultsAndMissingCatalogIDs(t *testing.T) {
	loader := newMockLoader()
	loader.exists["multi/v1"] = true
	loader.bundleShape["multi/v1"] = true
	loader.bundleArtifacts["multi/v1"] = BundleArtifacts{
		Source: bundle.File{Name: "root.yaml", Type: "Policy", Data: []byte(testPolicyYAML("root"))},
		Imports: []bundle.File{
			{Name: "a.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML(""))},
			{Name: "b.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML(""))},
			{Name: "c.yaml", Type: "GuidanceCatalog", Data: []byte(testGuidanceYAML(""))},
			{Name: "d.yaml", Type: "GuidanceCatalog", Data: []byte(testGuidanceYAML(""))},
			{Name: "parent.yaml", Type: "Policy", Data: []byte(testBundlePolicyYAML(
				"parent", "parent-evaluator", "2025-01-01T00:00:00Z"))},
			{Name: "grandparent.yaml", Type: "Policy", Data: []byte(testBundlePolicyYAML(
				"grandparent", "grandparent-evaluator", "2024-01-01T00:00:00Z"))},
		},
	}
	graph, err := NewResolver(loader).ResolvePolicyGraph("multi", "v1")
	require.NoError(t, err)
	require.Len(t, graph.Controls, 2)
	assert.Equal(t, "multi-catalog-0", graph.Controls[0].ID)
	assert.Equal(t, "multi-catalog-1", graph.Controls[1].ID)
	assert.Empty(t, graph.Controls[0].Parsed.Metadata.Id)
	require.Len(t, graph.Guidelines, 2)
	assert.Equal(t, "multi-guidance-0", graph.Guidelines[0].ID)
	assert.Equal(t, "multi-guidance-1", graph.Guidelines[1].ID)
	assert.Empty(t, graph.EvaluatorID, "imports must not set whole-graph routing")
	require.NotNil(t, graph.Timeline)
	assert.Equal(t, "2025-01-01T00:00:00Z", graph.Timeline.EvaluationStart)
	require.Len(t, graph.Assessments, 3)
	assert.Empty(t, graph.Assessments[0].EvaluatorID)
	assert.Equal(t, "grandparent-plan", graph.Assessments[2].ID)
}

func TestExtractBundleMetadata_MultipleCatalogs(t *testing.T) {
	r := NewResolver(seedBundlePolicy(t, "multi", "v1", []bundle.File{
		{Name: "root.yaml", Type: "Policy", Data: []byte(testBundlePolicyYAML("root", "opa", ""))},
		{Name: "a.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("a"))},
		{Name: "b.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("b") + "  - id: b-extra\n  - id: b-third\n")},
		{Name: "parent.yaml", Type: "Policy", Data: []byte(testPolicyYAML("parent"))},
	}))
	metadata, err := r.ExtractPolicyMetadata("multi", "v1")
	require.NoError(t, err)
	assert.Equal(t, PolicyMetadata{
		Title: "root", EvaluatorID: "opa", ControlCount: 4, AssessmentCount: 1,
	}, metadata)
}

func TestResolveBundleGraph_RejectsNonPolicySource(t *testing.T) {
	loader := seedBundlePolicy(t, "wrong-root", "v1", []bundle.File{
		{Name: "root.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("root"))},
		{Name: "parent.yaml", Type: "Policy", Data: []byte(testPolicyYAML("parent"))},
	})
	r := NewResolver(loader)
	_, err := r.ResolvePolicyGraph("wrong-root", "v1")
	require.ErrorContains(t, err, `bundle source is "ControlCatalog", expected Policy`)
	_, err = r.ExtractPolicyMetadata("wrong-root", "v1")
	require.ErrorContains(t, err, `bundle source is "ControlCatalog", expected Policy`)
}

func TestResolveBundleGraph_MalformedSupportingArtifacts(t *testing.T) {
	for _, typ := range []string{"ControlCatalog", "GuidanceCatalog", "Policy"} {
		t.Run(typ, func(t *testing.T) {
			r := NewResolver(seedBundlePolicy(t, "malformed", "v1", []bundle.File{
				{Name: "root.yaml", Type: "Policy", Data: []byte(testPolicyYAML("root"))},
				{Name: "valid.yaml", Type: "ControlCatalog", Data: []byte(testCatalogYAML("valid"))},
				{Name: "invalid.yaml", Type: typ, Data: []byte("{invalid: yaml: [}")},
			}))
			_, err := r.ResolvePolicyGraph("malformed", "v1")
			require.ErrorContains(t, err, "invalid.yaml")
			if typ == "ControlCatalog" {
				_, err = r.ExtractPolicyMetadata("malformed", "v1")
				require.ErrorContains(t, err, "invalid.yaml")
			}
		})
	}
}

func TestResolvePolicyGraph_BundleAllThreeArtifacts(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bundle-policy/v1"] = true
	ml.bundleShape["bundle-policy/v1"] = true
	ml.bundleFiles["bundle-policy/v1"] = map[string][]byte{
		"ControlCatalog": []byte(`
title: Test Catalog
metadata:
  type: ControlCatalog
  id: cat-1
  version: "1.0"
controls: []
`),
		"GuidanceCatalog": []byte(`
title: Test Guidance
metadata:
  type: GuidanceCatalog
  id: guide-1
  version: "1.0"
guidelines: []
`),
		"Policy": validPolicyYAML(),
	}

	r := NewResolver(ml)
	graph, err := r.ResolvePolicyGraph("bundle-policy", "v1")
	require.NoError(t, err)
	assert.Equal(t, "bundle-policy", graph.PolicyID)
	assert.Len(t, graph.Controls, 1)
	assert.Len(t, graph.Guidelines, 1)
	assert.Len(t, graph.Assessments, 1)
	assert.Equal(t, "openscap", graph.EvaluatorID)
}

func TestResolvePolicyGraph_BundlePolicyOnly(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bundle-minimal/v1"] = true
	ml.bundleShape["bundle-minimal/v1"] = true
	ml.bundleFiles["bundle-minimal/v1"] = map[string][]byte{
		"Policy": validPolicyYAML(),
	}

	r := NewResolver(ml)
	graph, err := r.ResolvePolicyGraph("bundle-minimal", "v1")
	require.NoError(t, err)
	assert.Empty(t, graph.Controls)
	assert.Empty(t, graph.Guidelines)
	assert.Len(t, graph.Assessments, 1)
	assert.Equal(t, "openscap", graph.EvaluatorID)
}

func TestResolvePolicyGraph_BundleMissingPolicy(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bundle-no-policy/v1"] = true
	ml.bundleShape["bundle-no-policy/v1"] = true
	ml.bundleFiles["bundle-no-policy/v1"] = map[string][]byte{
		"ControlCatalog": []byte(`
title: Test Catalog
metadata:
  type: ControlCatalog
  id: cat-1
  version: "1.0"
controls: []
`),
	}

	r := NewResolver(ml)
	_, err := r.ResolvePolicyGraph("bundle-no-policy", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required Policy artifact")
}

func TestResolvePolicyGraph_BundleInvalidCatalogYAML(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bundle-bad-cat/v1"] = true
	ml.bundleShape["bundle-bad-cat/v1"] = true
	ml.bundleFiles["bundle-bad-cat/v1"] = map[string][]byte{
		"ControlCatalog": []byte("{not: valid: yaml: [}"),
		"Policy":         validPolicyYAML(),
	}

	r := NewResolver(ml)
	_, err := r.ResolvePolicyGraph("bundle-bad-cat", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog layer is not valid Gemara")
}

func TestResolvePolicyGraph_BundleUnpackError(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bundle-broken/v1"] = true
	ml.bundleShape["bundle-broken/v1"] = true
	// No bundleFiles entry → LoadBundleFiles will return error

	r := NewResolver(ml)
	_, err := r.ResolvePolicyGraph("bundle-broken", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bundle unpack failed")
}

func TestResolvePolicyGraph_DetectShapeError(t *testing.T) {
	ml := newMockLoader()
	ml.exists["corrupt/v1"] = true
	ml.bundleShapeErr["corrupt/v1"] = fmt.Errorf("corrupt manifest")

	r := NewResolver(ml)
	_, err := r.ResolvePolicyGraph("corrupt", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to detect manifest shape")
	assert.Contains(t, err.Error(), "corrupt@v1")
}

func TestResolvePolicyGraph_BundleInvalidGuidanceYAML(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bundle-bad-guide/v1"] = true
	ml.bundleShape["bundle-bad-guide/v1"] = true
	ml.bundleFiles["bundle-bad-guide/v1"] = map[string][]byte{
		"GuidanceCatalog": []byte("{not: valid: yaml: [}"),
		"Policy":          validPolicyYAML(),
	}

	r := NewResolver(ml)
	_, err := r.ResolvePolicyGraph("bundle-bad-guide", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "guidance layer is not valid Gemara")
}

func TestResolvePolicyGraph_BundleInvalidPolicyYAML(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bundle-bad-pol/v1"] = true
	ml.bundleShape["bundle-bad-pol/v1"] = true
	ml.bundleFiles["bundle-bad-pol/v1"] = map[string][]byte{
		"Policy": []byte("{not: valid: yaml: [}"),
	}

	r := NewResolver(ml)
	_, err := r.ResolvePolicyGraph("bundle-bad-pol", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid Gemara Policy YAML")
}

func TestResolvePolicyGraph_BundleVerifyParsedFields(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bundle-parsed/v1"] = true
	ml.bundleShape["bundle-parsed/v1"] = true
	ml.bundleFiles["bundle-parsed/v1"] = map[string][]byte{
		"ControlCatalog": []byte(`
title: Parsed Catalog
metadata:
  type: ControlCatalog
  id: cat-parsed
  version: "1.0"
controls: []
`),
		"GuidanceCatalog": []byte(`
title: Parsed Guidance
metadata:
  type: GuidanceCatalog
  id: guide-parsed
  version: "1.0"
guidelines: []
`),
		"Policy": validPolicyYAML(),
	}

	r := NewResolver(ml)
	graph, err := r.ResolvePolicyGraph("bundle-parsed", "v1")
	require.NoError(t, err)
	require.Len(t, graph.Controls, 1)
	require.NotNil(t, graph.Controls[0].Parsed)
	assert.Equal(t, "cat-parsed", graph.Controls[0].Parsed.Metadata.Id)
	require.Len(t, graph.Guidelines, 1)
	require.NotNil(t, graph.Guidelines[0].Parsed)
	assert.Equal(t, "guide-parsed", graph.Guidelines[0].Parsed.Metadata.Id)
}

// --- ExtractPolicyMetadata tests ---

func TestResolver_ExtractPolicyMetadata_EmptyPolicyID(t *testing.T) {
	r := NewResolver(newMockLoader())
	_, err := r.ExtractPolicyMetadata("", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy ID cannot be empty")
}

func TestResolver_ExtractPolicyMetadata_EmptyVersion(t *testing.T) {
	r := NewResolver(newMockLoader())
	_, err := r.ExtractPolicyMetadata("test-policy", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version cannot be empty")
}

func TestResolver_ExtractPolicyMetadata_PolicyNotInCache(t *testing.T) {
	r := NewResolver(newMockLoader())
	_, err := r.ExtractPolicyMetadata("missing-policy", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy not found")
}

func TestResolver_ExtractPolicyMetadata_DetectShapeError(t *testing.T) {
	ml := newMockLoader()
	ml.exists["shape-err/v1"] = true
	ml.bundleShapeErr["shape-err/v1"] = fmt.Errorf(
		"corrupt manifest")

	r := NewResolver(ml)
	_, err := r.ExtractPolicyMetadata("shape-err", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(),
		"failed to detect manifest shape")
	assert.Contains(t, err.Error(), "corrupt manifest")
}

func TestResolver_ExtractPolicyMetadata_BundleMissingPolicyArtifact(
	t *testing.T,
) {
	ml := newMockLoader()
	ml.exists["no-policy/v1"] = true
	ml.bundleShape["no-policy/v1"] = true
	ml.bundleFiles["no-policy/v1"] = map[string][]byte{
		"ControlCatalog": validCatalogYAML(),
	}

	r := NewResolver(ml)
	_, err := r.ExtractPolicyMetadata("no-policy", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(),
		"missing required Policy artifact")
}

func TestResolver_ExtractPolicyMetadata_BundleInvalidCatalog(
	t *testing.T,
) {
	ml := newMockLoader()
	ml.exists["bad-cat/v1"] = true
	ml.bundleShape["bad-cat/v1"] = true
	ml.bundleFiles["bad-cat/v1"] = map[string][]byte{
		"Policy":         validPolicyYAML(),
		"ControlCatalog": []byte("{not: valid: catalog: [}"),
	}

	r := NewResolver(ml)
	_, err := r.ExtractPolicyMetadata("bad-cat", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(),
		"catalog layer is not valid Gemara")
}

func TestResolver_ExtractPolicyMetadata_SplitPolicyLoadError(
	t *testing.T,
) {
	ml := newMockLoader()
	ml.exists["split-err/v1"] = true
	// bundleShape defaults to false (split-layer).
	// No policy layer registered -> LoadLayerByMediaType fails.

	r := NewResolver(ml)
	_, err := r.ExtractPolicyMetadata("split-err", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(),
		"failed to load policy layer")
}

func TestResolver_ExtractPolicyMetadata_SplitMissingCatalog(
	t *testing.T,
) {
	ml := newMockLoader()
	ml.exists["split-nocat/v1"] = true
	// bundleShape defaults to false (split-layer).
	policyMediaType := "application/vnd.gemara.policy.v1+yaml"
	ml.layers["split-nocat/v1/"+policyMediaType] = validPolicyYAML()
	// No catalog layer registered -> catalog load fails with a
	// warning log; ControlCount defaults to 0.

	r := NewResolver(ml)
	meta, err := r.ExtractPolicyMetadata("split-nocat", "v1")
	require.NoError(t, err)
	assert.Equal(t, "Test Policy", meta.Title)
	assert.Equal(t, "openscap", meta.EvaluatorID)
	assert.Equal(t, 0, meta.ControlCount,
		"missing catalog should yield zero controls")
	assert.Equal(t, 1, meta.AssessmentCount)
}

func TestResolver_ExtractPolicyMetadata_BundleFormat(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bundle-meta/v1"] = true
	ml.bundleShape["bundle-meta/v1"] = true
	ml.bundleFiles["bundle-meta/v1"] = map[string][]byte{
		"Policy":         validPolicyYAML(),
		"ControlCatalog": validCatalogYAML(),
	}

	r := NewResolver(ml)
	meta, err := r.ExtractPolicyMetadata("bundle-meta", "v1")
	require.NoError(t, err)
	assert.Equal(t, "Test Policy", meta.Title)
	assert.Equal(t, "openscap", meta.EvaluatorID)
	assert.Equal(t, 2, meta.ControlCount)
	assert.Equal(t, 1, meta.AssessmentCount)
}

func TestResolver_ExtractPolicyMetadata_SplitLayer(t *testing.T) {
	ml := newMockLoader()
	ml.exists["split-meta/v1"] = true
	// bundleShape defaults to false (split-layer)

	policyMediaType := "application/vnd.gemara.policy.v1+yaml"
	catalogMediaType := "application/vnd.gemara.catalog.v1+yaml"

	ml.layers["split-meta/v1/"+policyMediaType] = validPolicyYAML()
	ml.layers["split-meta/v1/"+catalogMediaType] = validCatalogYAML()

	r := NewResolver(ml)
	meta, err := r.ExtractPolicyMetadata("split-meta", "v1")
	require.NoError(t, err)
	assert.Equal(t, "Test Policy", meta.Title)
	assert.Equal(t, "openscap", meta.EvaluatorID)
	assert.Equal(t, 2, meta.ControlCount)
	assert.Equal(t, 1, meta.AssessmentCount)
}

func TestResolver_ExtractPolicyMetadata_MissingCatalog(t *testing.T) {
	ml := newMockLoader()
	ml.exists["no-catalog/v1"] = true
	ml.bundleShape["no-catalog/v1"] = true
	ml.bundleFiles["no-catalog/v1"] = map[string][]byte{
		"Policy": validPolicyYAML(),
	}

	r := NewResolver(ml)
	meta, err := r.ExtractPolicyMetadata("no-catalog", "v1")
	require.NoError(t, err)
	assert.Equal(t, "Test Policy", meta.Title)
	assert.Equal(t, "openscap", meta.EvaluatorID)
	assert.Equal(t, 0, meta.ControlCount)
	assert.Equal(t, 1, meta.AssessmentCount)
}

func TestResolver_ExtractPolicyMetadata_MultiEvaluator(t *testing.T) {
	multiEvalPolicy := []byte(`
title: Multi Evaluator Policy
metadata:
  id: pol-multi
  type: Policy
  version: "1.0"
contacts:
  responsible:
    - name: team-a
  accountable:
    - name: team-b
scope:
  in:
    technologies:
      - linux
imports:
  catalogs:
    - reference-id: cat-1
adherence:
  assessment-plans:
    - id: ap-1
      requirement-id: req-1
      frequency: daily
      evaluation-methods:
        - type: Behavioral
          executor:
            id: openscap
    - id: ap-2
      requirement-id: req-2
      frequency: weekly
      evaluation-methods:
        - type: Behavioral
          executor:
            id: opa
`)

	ml := newMockLoader()
	ml.exists["multi-eval/v1"] = true
	ml.bundleShape["multi-eval/v1"] = true
	ml.bundleFiles["multi-eval/v1"] = map[string][]byte{
		"Policy": multiEvalPolicy,
	}

	r := NewResolver(ml)
	meta, err := r.ExtractPolicyMetadata("multi-eval", "v1")
	require.NoError(t, err)
	assert.Equal(t, "Multi Evaluator Policy", meta.Title)
	assert.Empty(t, meta.EvaluatorID,
		"multi-evaluator policy should have empty EvaluatorID")
	assert.Equal(t, 2, meta.AssessmentCount)
}

func TestResolver_ExtractPolicyMetadata_EmptyTitle(t *testing.T) {
	emptyTitlePolicy := []byte(`
title: ""
metadata:
  id: pol-notitle
  type: Policy
  version: "1.0"
contacts:
  responsible:
    - name: team-a
  accountable:
    - name: team-b
scope:
  in:
    technologies:
      - linux
imports:
  catalogs:
    - reference-id: cat-1
adherence:
  assessment-plans:
    - id: ap-1
      requirement-id: req-1
      frequency: daily
      evaluation-methods:
        - type: Behavioral
          executor:
            id: ampel
`)

	ml := newMockLoader()
	ml.exists["empty-title/v1"] = true
	ml.bundleShape["empty-title/v1"] = true
	ml.bundleFiles["empty-title/v1"] = map[string][]byte{
		"Policy": emptyTitlePolicy,
	}

	r := NewResolver(ml)
	meta, err := r.ExtractPolicyMetadata("empty-title", "v1")
	require.NoError(t, err)
	assert.Empty(t, meta.Title)
	assert.Equal(t, "ampel", meta.EvaluatorID)
	assert.Equal(t, 1, meta.AssessmentCount)
}

func TestResolver_ExtractPolicyMetadata_ParseError(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bad-yaml/v1"] = true
	ml.bundleShape["bad-yaml/v1"] = true
	ml.bundleFiles["bad-yaml/v1"] = map[string][]byte{
		"Policy": []byte("{not: valid: yaml: [}"),
	}

	r := NewResolver(ml)
	_, err := r.ExtractPolicyMetadata("bad-yaml", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid Gemara Policy YAML")
}

func validCatalogYAML() []byte {
	return []byte(`
title: Test Catalog
metadata:
  type: ControlCatalog
  id: cat-1
  version: "1.0"
controls:
  - id: ctrl-1
    title: First Control
    objective: Test objective 1
    assessment-requirements:
      - id: ar-1
        description: Check first
  - id: ctrl-2
    title: Second Control
    objective: Test objective 2
    assessment-requirements:
      - id: ar-2
        description: Check second
`)
}

func validPolicyYAML() []byte {
	return []byte(`
title: Test Policy
metadata:
  id: pol-1
  type: Policy
  gemara-version: 1.0.0
  version: "1.0"
contacts:
  responsible:
    - name: team-a
  accountable:
    - name: team-b
scope:
  in:
    technologies:
      - linux
imports:
  catalogs:
    - reference-id: cat-1
adherence:
  assessment-plans:
    - id: ap-1
      requirement-id: req-1
      frequency: daily
      evaluation-methods:
        - id: openscap-eval
          type: Behavioral
          mode: Automated
          executor:
            id: openscap
`)
}

func validPolicyYAMLWithMappingRefs() []byte {
	return []byte(`
title: Test Policy With Refs
metadata:
  id: pol-1
  type: Policy
  gemara-version: 1.0.0
  version: "1.0"
  mapping-references:
    - id: nist-800-53
      title: NIST SP 800-53 Rev 5
      version: "5.0"
      url: https://csrc.nist.gov/publications/detail/sp/800-53/rev-5/final
    - id: cis-k8s
      title: CIS Kubernetes Benchmark
      version: "1.8"
contacts:
  responsible:
    - name: team-a
  accountable:
    - name: team-b
scope:
  in:
    technologies:
      - linux
imports:
  catalogs:
    - reference-id: cat-1
adherence:
  assessment-plans:
    - id: ap-1
      requirement-id: req-1
      frequency: daily
      evaluation-methods:
        - id: openscap-eval
          type: Behavioral
          mode: Automated
          executor:
            id: openscap
`)
}

// --- MappingReferences extraction tests ---

func TestExtractFromGemaraPolicy_MappingReferencesPreserved(t *testing.T) {
	var p gemara.Policy
	err := yaml.Unmarshal(validPolicyYAMLWithMappingRefs(), &p)
	require.NoError(t, err)

	result := extractFromGemaraPolicy(&p)
	require.Len(t, result.MappingReferences, 2)
	assert.Equal(t, "nist-800-53", result.MappingReferences[0].Id)
	assert.Equal(t, "NIST SP 800-53 Rev 5", result.MappingReferences[0].Title)
	assert.Equal(t, "5.0", result.MappingReferences[0].Version)
	assert.Equal(t, "https://csrc.nist.gov/publications/detail/sp/800-53/rev-5/final",
		result.MappingReferences[0].Url)
	assert.Equal(t, "cis-k8s", result.MappingReferences[1].Id)
}

func TestExtractFromGemaraPolicy_NoMappingReferences(t *testing.T) {
	var p gemara.Policy
	err := yaml.Unmarshal(validPolicyYAML(), &p)
	require.NoError(t, err)

	result := extractFromGemaraPolicy(&p)
	assert.Nil(t, result.MappingReferences)
}

func TestResolvePolicyGraph_SplitGraph_MappingReferencesPropagated(t *testing.T) {
	ml := newMockLoader()
	ml.exists["test-policy/v1"] = true

	ml.layers["test-policy/v1/application/vnd.gemara.policy.v1+yaml"] =
		validPolicyYAMLWithMappingRefs()

	r := NewResolver(ml)
	graph, err := r.ResolvePolicyGraph("test-policy", "v1")
	require.NoError(t, err)
	require.Len(t, graph.MappingReferences, 2)
	assert.Equal(t, "nist-800-53", graph.MappingReferences[0].Id)
	assert.Equal(t, "cis-k8s", graph.MappingReferences[1].Id)
}

func TestResolvePolicyGraph_BundleGraph_MappingReferencesPropagated(t *testing.T) {
	ml := newMockLoader()
	ml.exists["test-policy/v1"] = true
	ml.bundleShape["test-policy/v1"] = true
	ml.bundleFiles["test-policy/v1"] = map[string][]byte{
		"Policy": validPolicyYAMLWithMappingRefs(),
	}

	r := NewResolver(ml)
	graph, err := r.ResolvePolicyGraph("test-policy", "v1")
	require.NoError(t, err)
	require.Len(t, graph.MappingReferences, 2)
	assert.Equal(t, "nist-800-53", graph.MappingReferences[0].Id)
	assert.Equal(t, "cis-k8s", graph.MappingReferences[1].Id)
}

func TestResolvePolicyGraph_NoMappingReferences(t *testing.T) {
	ml := newMockLoader()
	ml.exists["test-policy/v1"] = true

	ml.layers["test-policy/v1/application/vnd.gemara.policy.v1+yaml"] =
		validPolicyYAML()

	r := NewResolver(ml)
	graph, err := r.ResolvePolicyGraph("test-policy", "v1")
	require.NoError(t, err)
	assert.Nil(t, graph.MappingReferences)
}
