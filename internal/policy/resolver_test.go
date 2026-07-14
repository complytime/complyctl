// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"testing"

	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPolicyLoader satisfies PolicyLoader for resolver unit tests.
type mockPolicyLoader struct {
	layers         map[string][]byte            // key: "policyID/version/mediaType"
	exists         map[string]bool              // key: "policyID/version"
	versions       map[string]string            // key: "policyID/configVersion" → resolved version
	bundleFiles    map[string]map[string][]byte // key: "policyID/version" → type → data
	bundleShape    map[string]bool              // key: "policyID/version" → true if bundle
	bundleShapeErr map[string]error             // key: "policyID/version" → error for DetectManifestShape
}

func newMockLoader() *mockPolicyLoader {
	return &mockPolicyLoader{
		layers:         make(map[string][]byte),
		exists:         make(map[string]bool),
		versions:       make(map[string]string),
		bundleFiles:    make(map[string]map[string][]byte),
		bundleShape:    make(map[string]bool),
		bundleShapeErr: make(map[string]error),
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
  id: cat-1
  version: "1.0"
controls: []
`)

	ml.layers["test-policy/v1/application/vnd.gemara.guidance.v1+yaml"] = []byte(`
title: Test Guidance
metadata:
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

func TestParsePolicyLayer_MissingAssessmentPlans(t *testing.T) {
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
	_, err := parsePolicyLayer("pol-empty", yamlData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no adherence.assessment-plans")
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

func TestResolvePolicyGraph_BundleAllThreeArtifacts(t *testing.T) {
	ml := newMockLoader()
	ml.exists["bundle-policy/v1"] = true
	ml.bundleShape["bundle-policy/v1"] = true
	ml.bundleFiles["bundle-policy/v1"] = map[string][]byte{
		"ControlCatalog": []byte(`
title: Test Catalog
metadata:
  id: cat-1
  version: "1.0"
controls: []
`),
		"GuidanceCatalog": []byte(`
title: Test Guidance
metadata:
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
  id: cat-parsed
  version: "1.0"
controls: []
`),
		"GuidanceCatalog": []byte(`
title: Parsed Guidance
metadata:
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
