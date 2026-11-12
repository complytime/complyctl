// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/gemaraproj/go-gemara"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"

	"github.com/complytime/complyctl/internal/complytime"
)

// DependencyGraph represents a resolved set of Controls, Guidelines, and Assessments
type DependencyGraph struct {
	PolicyID          string
	Controls          []Control
	Guidelines        []Guideline
	Assessments       []Assessment
	EvaluatorID       string
	Timeline          *PolicyTimeline
	MappingReferences []gemara.MappingReference
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
	ID string
	// RequirementID is the requirement ID from the Gemara assessment plan's
	// requirement-id field. Used post-scan to resolve plan IDs in provider
	// results to actual requirement IDs for report output.
	RequirementID      string
	EvaluatorID        string
	Parameters         map[string]string
	AuthoredParameters []gemara.Parameter
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

// PolicyMetadata holds display-oriented metadata extracted from a
// cached Gemara policy, suitable for CLI summary and list output.
// EvaluatorID maps to PolicyState.PolicyEvaluator when persisted
// (not PolicyState.EvaluatorID, which is reserved for complypacks).
type PolicyMetadata struct {
	Title           string
	EvaluatorID     string
	ControlCount    int
	AssessmentCount int
}

// PolicyLoader abstracts the Loader methods used by Resolver, enabling
// mock injection for unit tests without coupling to OCI store internals.
type PolicyLoader interface {
	LoadLayerByMediaType(policyID, version, mediaType string) ([]byte, error)
	LoadBundleFiles(policyID, version string) (map[string][]byte, error)
	LoadBundleArtifacts(policyID, version string) (BundleArtifacts, error)
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

// ExtractPolicyMetadata extracts display-oriented metadata from a
// cached policy without building a full DependencyGraph. The policyID
// parameter is the OCI repository path (cache key), following the
// same convention as ResolvePolicyGraph.
func (r *Resolver) ExtractPolicyMetadata(
	policyID, version string,
) (PolicyMetadata, error) {
	if policyID == "" {
		return PolicyMetadata{}, fmt.Errorf(
			"policy ID cannot be empty")
	}
	if version == "" {
		return PolicyMetadata{}, fmt.Errorf(
			"version cannot be empty")
	}
	if !r.loader.PolicyExists(policyID, version) {
		return PolicyMetadata{}, fmt.Errorf(
			"policy not found: %s@%s", policyID, version)
	}

	isBundle, detectErr := r.loader.DetectManifestShape(
		policyID, version)
	if detectErr != nil {
		return PolicyMetadata{}, fmt.Errorf(
			"failed to detect manifest shape for %s@%s: %w",
			policyID, version, detectErr)
	}

	if isBundle {
		return r.extractBundleMetadata(policyID, version)
	}
	return r.extractSplitMetadata(policyID, version)
}

// extractBundleMetadata extracts metadata from a Gemara bundle-format
// policy using the same validated inventory as execution.
func (r *Resolver) extractBundleMetadata(
	policyID, version string,
) (PolicyMetadata, error) {
	artifacts, err := r.loader.LoadBundleArtifacts(policyID, version)
	if err != nil {
		return PolicyMetadata{}, fmt.Errorf(
			"bundle unpack failed for %s@%s: %w",
			policyID, version, err)
	}

	graph, err := resolveBundleArtifacts(policyID, version, artifacts)
	if err != nil {
		return PolicyMetadata{}, err
	}

	policyResult, parseErr := parsePolicyLayer(
		policyID, artifacts.Source.Data)
	if parseErr != nil {
		return PolicyMetadata{}, fmt.Errorf(
			"failed to parse policy layer for %s (%q): %w",
			policyID, artifacts.Source.Name, parseErr)
	}

	meta := PolicyMetadata{
		Title:           policyResult.Title,
		EvaluatorID:     policyResult.EvaluatorID,
		AssessmentCount: len(graph.Assessments),
	}

	for _, catalog := range graph.Controls {
		meta.ControlCount += len(catalog.Parsed.Controls)
	}

	return meta, nil
}

// extractSplitMetadata extracts metadata from a split-layer format
// policy. Parses only the Policy and ControlCatalog layers —
// guidance is not needed for display metadata.
func (r *Resolver) extractSplitMetadata(
	policyID, version string,
) (PolicyMetadata, error) {
	policyData, policyLoadErr := r.loader.LoadLayerByMediaType(
		policyID, version, complytime.MediaTypePolicy)
	if policyLoadErr != nil {
		return PolicyMetadata{}, fmt.Errorf(
			"failed to load policy layer for %s@%s: %w",
			policyID, version, policyLoadErr)
	}

	policyResult, parseErr := parsePolicyLayer(
		policyID, policyData)
	if parseErr != nil {
		return PolicyMetadata{}, fmt.Errorf(
			"failed to parse policy layer for %s: %w",
			policyID, parseErr)
	}

	meta := PolicyMetadata{
		Title:           policyResult.Title,
		EvaluatorID:     policyResult.EvaluatorID,
		AssessmentCount: len(policyResult.Assessments),
	}

	catalogData, catalogLoadErr := r.loader.LoadLayerByMediaType(
		policyID, version, complytime.MediaTypeCatalog)
	if catalogLoadErr != nil {
		// Catalog layer unavailable — could be genuinely absent or
		// corrupted after sync. Log a warning so the user knows the
		// control count may be inaccurate rather than silently
		// defaulting to zero.
		log.Warn("catalog layer unavailable, control count will be 0",
			"policy", policyID, "version", version,
			"error", catalogLoadErr)
	} else {
		catalog, catErr := parseControlCatalog(catalogData)
		if catErr != nil {
			return PolicyMetadata{}, fmt.Errorf(
				"policy %s: catalog layer is not valid Gemara: %w",
				policyID, catErr)
		}
		meta.ControlCount = len(catalog.Controls)
	}

	return meta, nil
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
	artifacts, err := r.loader.LoadBundleArtifacts(policyID, version)
	if err != nil {
		return nil, fmt.Errorf("bundle unpack failed for %s@%s: %w", policyID, version, err)
	}
	return resolveBundleArtifacts(policyID, version, artifacts)
}

func resolveBundleArtifacts(policyID, version string, artifacts BundleArtifacts) (*DependencyGraph, error) {
	if artifacts.Source.Type != artifactTypePolicy {
		return nil, fmt.Errorf(
			"bundle for %s@%s: missing required Policy artifact: bundle source is %q, expected Policy",
			policyID, version, artifacts.Source.Type)
	}
	rootPolicy, err := parsePolicyLayer(policyID, artifacts.Source.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse policy layer for %s (%q): %w", policyID, artifacts.Source.Name, err)
	}

	graph := &DependencyGraph{
		PolicyID:          policyID,
		Controls:          []Control{},
		Guidelines:        []Guideline{},
		Assessments:       append([]Assessment{}, rootPolicy.Assessments...),
		EvaluatorID:       rootPolicy.EvaluatorID,
		Timeline:          rootPolicy.Timeline,
		MappingReferences: append([]gemara.MappingReference{}, rootPolicy.MappingReferences...),
	}
	if err := appendBundleCatalogs(graph, artifacts); err != nil {
		return nil, err
	}
	if err := appendBundlePolicies(graph, artifacts); err != nil {
		return nil, err
	}
	return graph, nil
}

func appendBundleCatalogs(graph *DependencyGraph, artifacts BundleArtifacts) error {
	identities := newCatalogIdentities()
	for index, file := range artifacts.FilesByType(artifactTypeControl) {
		catalog, parseErr := parseControlCatalog(file.Data)
		if parseErr != nil {
			return fmt.Errorf("policy %s: catalog layer is not valid Gemara (%q): %w",
				graph.PolicyID, file.Name, parseErr)
		}
		id := catalog.Metadata.Id
		if id == "" {
			id = fmt.Sprintf("%s-catalog-%d", graph.PolicyID, index)
		}
		if err := identities.registerControlCatalog(id, catalog, file.Name); err != nil {
			return err
		}
		graph.Controls = append(graph.Controls, Control{ID: id, Content: file.Data, Parsed: catalog})
	}

	for index, file := range artifacts.FilesByType(artifactTypeGuidance) {
		guidance, parseErr := parseGuidanceCatalog(file.Data)
		if parseErr != nil {
			return fmt.Errorf("policy %s: guidance layer is not valid Gemara (%q): %w",
				graph.PolicyID, file.Name, parseErr)
		}
		id := guidance.Metadata.Id
		if id == "" {
			id = fmt.Sprintf("%s-guidance-%d", graph.PolicyID, index)
		}
		if err := registerBundleIdentity(identities.catalogs, "catalog", id, file.Name); err != nil {
			return err
		}
		graph.Guidelines = append(graph.Guidelines, Guideline{ID: id, Content: file.Data, Parsed: guidance})
	}
	return nil
}

func appendBundlePolicies(graph *DependencyGraph, artifacts BundleArtifacts) error {
	declarations := make(map[string]assessmentPlanDeclaration)
	references := make(map[string]mappingReferenceDeclaration)
	rootRefs := graph.MappingReferences
	graph.MappingReferences = nil
	if err := appendMappingReferences(graph, references, rootRefs, artifacts.Source.Name); err != nil {
		return err
	}
	if err := registerAssessmentPlans(declarations, graph.Assessments, artifacts.Source.Name); err != nil {
		return err
	}
	for _, file := range artifacts.ImportsByType(artifactTypePolicy) {
		imported, parseErr := parsePolicyLayer(graph.PolicyID, file.Data)
		if parseErr != nil {
			return fmt.Errorf("parse imported policy %q: %w", file.Name, parseErr)
		}
		if err := registerAssessmentPlans(declarations, imported.Assessments, file.Name); err != nil {
			return err
		}
		graph.Assessments = append(graph.Assessments, imported.Assessments...)
		if err := appendMappingReferences(graph, references, imported.MappingReferences, file.Name); err != nil {
			return err
		}
		if graph.Timeline == nil {
			graph.Timeline = imported.Timeline
		}
	}

	return nil
}

type assessmentPlanDeclaration struct {
	artifact      string
	requirementID string
}

// Plan IDs share one namespace in provider dispatch and scan result mapping.
// Even equal requirement IDs cannot make duplicate configurations unambiguous.
func registerAssessmentPlans(
	declarations map[string]assessmentPlanDeclaration, assessments []Assessment, artifact string,
) error {
	for _, assessment := range assessments {
		if previous, exists := declarations[assessment.ID]; exists {
			return fmt.Errorf(
				"duplicate assessment plan ID %q: %q (requirement %q) and %q (requirement %q)",
				assessment.ID, previous.artifact, previous.requirementID,
				artifact, assessment.RequirementID)
		}
		declarations[assessment.ID] = assessmentPlanDeclaration{
			artifact: artifact, requirementID: assessment.RequirementID,
		}
	}
	return nil
}

// resolveSplitGraph loads artifacts by matching distinct OCI layer media types.
func (r *Resolver) resolveSplitGraph(policyID, version string) (*DependencyGraph, error) {
	identities := newCatalogIdentities()
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
		id := parsed.Metadata.Id
		if id == "" {
			id = ctrl.ID
		}
		if err := identities.registerControlCatalog(id, parsed, complytime.MediaTypeCatalog); err != nil {
			return nil, err
		}
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
		id := parsed.Metadata.Id
		if id == "" {
			id = gl.ID
		}
		if err := registerBundleIdentity(identities.catalogs, "catalog", id, complytime.MediaTypeGuidance); err != nil {
			return nil, err
		}
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
	if err := registerAssessmentPlans(
		make(map[string]assessmentPlanDeclaration), policyLayer.Assessments, policyID,
	); err != nil {
		return nil, err
	}
	graph.EvaluatorID = policyLayer.EvaluatorID
	graph.Assessments = append(graph.Assessments, policyLayer.Assessments...)
	graph.Timeline = policyLayer.Timeline
	if err := appendMappingReferences(graph, make(map[string]mappingReferenceDeclaration),
		policyLayer.MappingReferences, complytime.MediaTypePolicy); err != nil {
		return nil, err
	}

	return graph, nil
}

func parseControlCatalog(data []byte) (*gemara.ControlCatalog, error) {
	if err := validateArtifactType(data, gemara.ControlCatalogArtifact); err != nil {
		return nil, err
	}
	var catalog gemara.ControlCatalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse control catalog YAML: %w", err)
	}
	return &catalog, nil
}

func parseGuidanceCatalog(data []byte) (*gemara.GuidanceCatalog, error) {
	if err := validateArtifactType(data, gemara.GuidanceCatalogArtifact); err != nil {
		return nil, err
	}
	var doc gemara.GuidanceCatalog
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse guidance catalog YAML: %w", err)
	}
	return &doc, nil
}

type policyLayerResult struct {
	Title             string
	EvaluatorID       string
	Assessments       []Assessment
	Timeline          *PolicyTimeline
	MappingReferences []gemara.MappingReference
}

// parsePolicyLayer decodes roots and imports through the same Gemara boundary.
// A Policy may mandate only guidance; plan count does not establish validity.
func parsePolicyLayer(policyID string, data []byte) (policyLayerResult, error) {
	if err := validateArtifactType(data, gemara.PolicyArtifact); err != nil {
		return policyLayerResult{}, fmt.Errorf(
			"policy %s: layer is not valid Gemara Policy YAML: %w", policyID, err)
	}
	var gemaraPolicy gemara.Policy
	if err := yaml.Unmarshal(data, &gemaraPolicy); err != nil {
		return policyLayerResult{}, fmt.Errorf(
			"policy %s: layer is not valid Gemara Policy YAML: %w", policyID, err)
	}
	if strings.TrimSpace(gemaraPolicy.Metadata.Id) == "" {
		return policyLayerResult{}, fmt.Errorf(
			"policy %s: Gemara Policy metadata.id cannot be empty", policyID)
	}
	return extractFromGemaraPolicy(&gemaraPolicy), nil
}

func validateArtifactType(data []byte, expected gemara.ArtifactType) error {
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return err
	}
	if len(file.Docs) != 1 {
		return fmt.Errorf("expected exactly one %s document, got %d", expected, len(file.Docs))
	}
	typ, err := gemara.DetectType(data)
	if err != nil {
		return err
	}
	if typ != expected {
		return fmt.Errorf("metadata.type is %q, expected %s", typ, expected)
	}
	return nil
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
			ID:                 plan.Id,
			RequirementID:      plan.RequirementId,
			EvaluatorID:        evalID,
			AuthoredParameters: plan.Parameters,
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
		Title:             p.Title,
		EvaluatorID:       resultEvalID,
		Assessments:       assessments,
		Timeline:          timeline,
		MappingReferences: p.Metadata.MappingReferences,
	}
}
