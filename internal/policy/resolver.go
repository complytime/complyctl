// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/gemaraproj/go-gemara"
	"github.com/goccy/go-yaml"
)

// DependencyGraph represents a resolved set of Controls, Guidelines, and Assessments
type DependencyGraph struct {
	PolicyID    string
	Controls    []Control
	Guidelines  []Guideline
	Assessments []Assessment
	EvaluatorID string
	Timeline    *PolicyTimeline
}

// Control pairs raw OCI layer content with a parsed Gemara ControlCatalog.
type Control struct {
	ID      string
	Content []byte
	Parsed  *gemara.ControlCatalog
}

// Guideline pairs raw OCI layer content with a parsed Gemara GuidanceCatalog.
type Guideline struct {
	ID      string
	Content []byte
	Parsed  *gemara.GuidanceCatalog
}

// Assessment holds a single assessment entry with its optional evaluator binding.
type Assessment struct {
	ID          string
	EvaluatorID string
	Parameters  map[string]string
}

// PolicyTimeline captures the evaluation and enforcement periods from a
// Gemara Policy's implementation-plan. Nil when the policy defines no
// implementation-plan. Datetime strings follow ISO 8601.
type PolicyTimeline struct {
	EvaluationStart  string
	EvaluationEnd    string
	EvaluationNotes  string
	EnforcementStart string
	EnforcementEnd   string
	EnforcementNotes string
}

// PolicyLoader abstracts the Loader methods used by Resolver, enabling
// mock injection for unit tests without coupling to OCI store internals.
type PolicyLoader interface {
	LoadLayerByMediaType(policyID, version, mediaType string) ([]byte, error)
	LoadBundleFiles(policyID, version string) (map[string][]byte, error)
	DetectManifestShape(policyID, version string) (isBundleShape bool, err error)
	PolicyExists(policyID, version string) bool
	ResolveVersion(policyID, configVersion string) (string, error)
}

// Resolver builds a DependencyGraph from cached OCI layers for a given policy.
type Resolver struct {
	loader PolicyLoader
}

// NewResolver creates a Resolver that uses the given PolicyLoader to load policy artifacts.
func NewResolver(loader PolicyLoader) *Resolver {
	return &Resolver{
		loader: loader,
	}
}

// ResolveVersion delegates to the underlying Loader to resolve a policy
// version. Empty configVersion resolves to the latest cached tag.
func (r *Resolver) ResolveVersion(policyID, configVersion string) (string, error) {
	return r.loader.ResolveVersion(policyID, configVersion)
}

// ResolvePolicyGraph builds a DependencyGraph from cached OCI layers.
// It detects the manifest shape (bundle vs split-layer) and delegates
// to the appropriate loading path.
func (r *Resolver) ResolvePolicyGraph(policyID, version string) (*DependencyGraph, error) {
	if policyID == "" {
		return nil, fmt.Errorf("policy ID cannot be empty")
	}

	if version == "" {
		return nil, fmt.Errorf("version cannot be empty")
	}

	if !r.loader.PolicyExists(policyID, version) {
		return nil, fmt.Errorf("policy not found: %s@%s", policyID, version)
	}

	isBundle, detectErr := r.loader.DetectManifestShape(policyID, version)
	if detectErr != nil {
		return nil, fmt.Errorf("failed to detect manifest shape for %s@%s: %w", policyID, version, detectErr)
	}

	if isBundle {
		return r.resolveBundleGraph(policyID, version)
	}
	return r.resolveSplitGraph(policyID, version)
}

// resolveBundleGraph loads artifacts from a Gemara bundle using bundle.Unpack.
func (r *Resolver) resolveBundleGraph(policyID, version string) (*DependencyGraph, error) {
	files, err := r.loader.LoadBundleFiles(policyID, version)
	if err != nil {
		return nil, fmt.Errorf("bundle unpack failed for %s@%s: %w", policyID, version, err)
	}

	graph := &DependencyGraph{
		PolicyID:    policyID,
		Controls:    []Control{},
		Guidelines:  []Guideline{},
		Assessments: []Assessment{},
	}

	if catalogData, ok := files["ControlCatalog"]; ok {
		ctrl := Control{
			ID:      policyID + "-catalog",
			Content: catalogData,
		}
		parsed, parseErr := parseControlCatalog(catalogData)
		if parseErr != nil {
			return nil, fmt.Errorf("policy %s: catalog layer is not valid Gemara: %w", policyID, parseErr)
		}
		ctrl.Parsed = parsed
		graph.Controls = append(graph.Controls, ctrl)
	}

	if guidanceData, ok := files["GuidanceCatalog"]; ok {
		gl := Guideline{
			ID:      policyID + "-guidance",
			Content: guidanceData,
		}
		parsed, parseErr := parseGuidanceCatalog(guidanceData)
		if parseErr != nil {
			return nil, fmt.Errorf("policy %s: guidance layer is not valid Gemara: %w", policyID, parseErr)
		}
		gl.Parsed = parsed
		graph.Guidelines = append(graph.Guidelines, gl)
	}

	policyData, ok := files["Policy"]
	if !ok {
		return nil, fmt.Errorf("bundle for %s@%s: missing required Policy artifact", policyID, version)
	}

	policyLayer, parseErr := parsePolicyLayer(policyID, policyData)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse policy layer for %s: %w", policyID, parseErr)
	}
	graph.EvaluatorID = policyLayer.EvaluatorID
	graph.Assessments = append(graph.Assessments, policyLayer.Assessments...)
	graph.Timeline = policyLayer.Timeline

	return graph, nil
}

// resolveSplitGraph loads artifacts by matching distinct OCI layer media types.
func (r *Resolver) resolveSplitGraph(policyID, version string) (*DependencyGraph, error) {
	graph := &DependencyGraph{
		PolicyID:    policyID,
		Controls:    []Control{},
		Guidelines:  []Guideline{},
		Assessments: []Assessment{},
	}

	controlsData, catalogLoadErr := r.loader.LoadLayerByMediaType(policyID, version, complytime.MediaTypeCatalog)
	if catalogLoadErr == nil {
		ctrl := Control{
			ID:      policyID + "-catalog",
			Content: controlsData,
		}
		parsed, parseErr := parseControlCatalog(controlsData)
		if parseErr != nil {
			return nil, fmt.Errorf("policy %s: catalog layer is not valid Gemara: %w", policyID, parseErr)
		}
		ctrl.Parsed = parsed
		graph.Controls = append(graph.Controls, ctrl)
	}

	guidelinesData, guidanceLoadErr := r.loader.LoadLayerByMediaType(policyID, version, complytime.MediaTypeGuidance)
	if guidanceLoadErr == nil {
		gl := Guideline{
			ID:      policyID + "-guidance",
			Content: guidelinesData,
		}
		parsed, parseErr := parseGuidanceCatalog(guidelinesData)
		if parseErr != nil {
			return nil, fmt.Errorf("policy %s: guidance layer is not valid Gemara: %w", policyID, parseErr)
		}
		gl.Parsed = parsed
		graph.Guidelines = append(graph.Guidelines, gl)
	}

	policyData, policyLoadErr := r.loader.LoadLayerByMediaType(policyID, version, complytime.MediaTypePolicy)
	if policyLoadErr != nil {
		return nil, fmt.Errorf("failed to load policy layer for %s@%s: %w", policyID, version, policyLoadErr)
	}

	policyLayer, parseErr := parsePolicyLayer(policyID, policyData)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse policy layer for %s: %w", policyID, parseErr)
	}
	graph.EvaluatorID = policyLayer.EvaluatorID
	graph.Assessments = append(graph.Assessments, policyLayer.Assessments...)
	graph.Timeline = policyLayer.Timeline

	return graph, nil
}

func parseControlCatalog(data []byte) (*gemara.ControlCatalog, error) {
	var catalog gemara.ControlCatalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse control catalog YAML: %w", err)
	}
	return &catalog, nil
}

func parseGuidanceCatalog(data []byte) (*gemara.GuidanceCatalog, error) {
	var doc gemara.GuidanceCatalog
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse guidance catalog YAML: %w", err)
	}
	return &doc, nil
}

type policyLayerResult struct {
	EvaluatorID string
	Assessments []Assessment
	Timeline    *PolicyTimeline
}

// parsePolicyLayer accepts only gemara.Policy with adherence.assessment-plans (R39).
// Returns an error for invalid YAML or missing assessment plans — no fallback.
func parsePolicyLayer(policyID string, data []byte) (policyLayerResult, error) {
	var gemaraPolicy gemara.Policy
	if err := yaml.Unmarshal(data, &gemaraPolicy); err != nil {
		return policyLayerResult{}, fmt.Errorf(
			"policy %s: layer is not valid Gemara Policy YAML: %w", policyID, err)
	}
	if len(gemaraPolicy.Adherence.AssessmentPlans) == 0 {
		return policyLayerResult{}, fmt.Errorf(
			"policy %s: Gemara Policy has no adherence.assessment-plans", policyID)
	}
	return extractFromGemaraPolicy(&gemaraPolicy), nil
}

// extractFromGemaraPolicy converts a Gemara Policy into a policyLayerResult.
// Per R32: each assessment-plan carries its own evaluation-methods[].executor.id
// for per-plan evaluator routing. Falls back to policy-level evaluation-methods
// when a plan defines none. EvaluatorID at the result level is set only when all
// plans share the same evaluator (single-evaluator shortcut).
func extractFromGemaraPolicy(p *gemara.Policy) policyLayerResult {
	var policyLevelEvalID string
	if len(p.Adherence.EvaluationMethods) > 0 {
		policyLevelEvalID = p.Adherence.EvaluationMethods[0].Executor.Id
	}

	assessments := make([]Assessment, 0, len(p.Adherence.AssessmentPlans))
	evalIDSet := make(map[string]bool)
	for _, plan := range p.Adherence.AssessmentPlans {
		evalID := policyLevelEvalID
		if len(plan.EvaluationMethods) > 0 && plan.EvaluationMethods[0].Executor.Id != "" {
			evalID = plan.EvaluationMethods[0].Executor.Id
		}
		evalIDSet[evalID] = true

		assessments = append(assessments, Assessment{
			ID:          plan.Id,
			EvaluatorID: evalID,
		})
	}

	resultEvalID := ""
	if len(evalIDSet) == 1 {
		for id := range evalIDSet {
			resultEvalID = id
		}
	}

	var timeline *PolicyTimeline
	ip := p.ImplementationPlan
	if ip.EvaluationTimeline.Start != "" || ip.EnforcementTimeline.Start != "" {
		timeline = &PolicyTimeline{
			EvaluationStart:  string(ip.EvaluationTimeline.Start),
			EvaluationEnd:    string(ip.EvaluationTimeline.End),
			EvaluationNotes:  ip.EvaluationTimeline.Notes,
			EnforcementStart: string(ip.EnforcementTimeline.Start),
			EnforcementEnd:   string(ip.EnforcementTimeline.End),
			EnforcementNotes: ip.EnforcementTimeline.Notes,
		}
	}

	return policyLayerResult{
		EvaluatorID: resultEvalID,
		Assessments: assessments,
		Timeline:    timeline,
	}
}
