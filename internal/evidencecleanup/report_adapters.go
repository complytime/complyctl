// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	oscalValidation "github.com/defenseunicorns/go-oscal/src/pkg/validation"
	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/gemaraconv"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
)

const (
	maxReportNesting = 64
	markdownLogStart = "<details>\n<summary>Evaluation Log</summary>\n\n```yaml\n"
	markdownLogEnd   = "\n```\n\n</details>\n"
	gemaraNamespace  = "https://github.com/gemaraproj/go-gemara/ns/oscal"
)

type parsedReport struct {
	policy     string
	target     string
	evidence   []gemara.Evidence
	references []gemara.MappingReference
}

// The writer replaces function-valued steps with strings. Do not decode into
// gemara.EvaluationLog: that would reject real reports or lose their assessments.
type inventoryLog struct {
	Metadata    gemara.Metadata     `json:"metadata"`
	Result      *gemara.Result      `json:"result"`
	Target      gemara.Resource     `json:"target"`
	Evaluations []*inventoryControl `json:"evaluations"`
}

type inventoryControl struct {
	Name        string                 `json:"name"`
	Result      *gemara.Result         `json:"result"`
	Message     string                 `json:"message"`
	Control     gemara.EntryMapping    `json:"control"`
	Assessments []*inventoryAssessment `json:"assessment-logs"`
}

type inventoryAssessment struct {
	Requirement    gemara.EntryMapping    `json:"requirement"`
	Plan           *gemara.EntryMapping   `json:"plan,omitempty"`
	Description    string                 `json:"description"`
	Result         *gemara.Result         `json:"result"`
	Message        string                 `json:"message"`
	Applicability  []string               `json:"applicability"`
	Steps          []string               `json:"steps"`
	StepsExecuted  int64                  `json:"steps-executed,omitempty"`
	Start          gemara.Datetime        `json:"start"`
	End            gemara.Datetime        `json:"end,omitempty"`
	Recommendation string                 `json:"recommendation,omitempty"`
	Confidence     gemara.ConfidenceLevel `json:"confidence-level,omitempty"`
	Evidence       []gemara.Evidence      `json:"evidence,omitempty"`
}

func parseManagedReport(format string, data []byte) (*parsedReport, error) {
	switch format {
	case formatEvaluation:
		return parseInventoryLog(data)
	case formatMarkdown:
		return parseInventoryMarkdown(data)
	case formatOSCAL:
		return parseInventoryOSCAL(data)
	case formatSARIF:
		return parseInventorySARIF(data)
	default:
		return nil, fmt.Errorf("unsupported report adapter %q", format)
	}
}

func parseInventoryLog(data []byte) (*parsedReport, error) {
	var log inventoryLog
	if err := decodeReport(data, &log); err != nil {
		return nil, fmt.Errorf("decode EvaluationLog: %w", err)
	}
	if log.Metadata.Id == "" || log.Metadata.Type != gemara.EvaluationLogArtifact {
		return nil, fmt.Errorf("EvaluationLog policy ID or artifact type is missing/invalid")
	}
	if log.Evaluations == nil || !validReportResult(log.Result) {
		return nil, fmt.Errorf("EvaluationLog evaluations or result is missing/invalid")
	}
	result := &parsedReport{policy: log.Metadata.Id, target: log.Target.Id,
		references: log.Metadata.MappingReferences}
	for _, control := range log.Evaluations {
		if err := collectControlEvidence(control, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func validReportResult(result *gemara.Result) bool {
	return result != nil && *result >= gemara.NotRun && *result <= gemara.Unknown
}

func collectControlEvidence(control *inventoryControl, report *parsedReport) error {
	if control == nil || control.Control.EntryId == "" || !validReportResult(control.Result) {
		return fmt.Errorf("incomplete EvaluationLog control")
	}
	for _, assessment := range control.Assessments {
		if assessment == nil || assessment.Requirement.EntryId == "" {
			return fmt.Errorf("incomplete EvaluationLog assessment")
		}
		if !validReportResult(assessment.Result) {
			return fmt.Errorf("invalid EvaluationLog assessment result")
		}
		report.evidence = append(report.evidence, assessment.Evidence...)
	}
	return nil
}

func parseInventoryMarkdown(data []byte) (*parsedReport, error) {
	if bytes.Count(data, []byte(markdownLogStart)) != 1 {
		return nil, fmt.Errorf("markdown lacks one unambiguous embedded EvaluationLog")
	}
	_, remainder, _ := bytes.Cut(data, []byte(markdownLogStart))
	if !bytes.HasSuffix(remainder, []byte(markdownLogEnd)) {
		return nil, fmt.Errorf("markdown embedded EvaluationLog is incomplete")
	}
	log := bytes.TrimSuffix(remainder, []byte(markdownLogEnd))
	if bytes.Contains(log, []byte("\n```")) {
		return nil, fmt.Errorf("markdown embedded EvaluationLog has an ambiguous fence")
	}
	return parseInventoryLog(log)
}

// decodeReport rejects unknown fields, duplicate keys, aliases, extra documents,
// and excessive nesting before typed decoding. Unknown schema variants must not
// silently erase reference fields. Reuse the vendored YAML parser for JSON too.
func decodeReport(data []byte, target any) error {
	if err := validateReportSyntax(data); err != nil {
		return err
	}
	converted, err := yaml.YAMLToJSON(data)
	if err != nil {
		return fmt.Errorf("normalize report serialization: %w", err)
	}
	if err := validateReportFields(converted); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(converted))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode typed report: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing report content: %v", err)
	}
	return nil
}

func validateReportSyntax(data []byte) error {
	if len(data) > maxReportBytes {
		return fmt.Errorf("report byte limit exceeded")
	}
	depth := 0
	for _, item := range lexer.Tokenize(string(data)) {
		if item.Position.IndentLevel > maxReportNesting {
			return fmt.Errorf("report indentation nesting limit exceeded")
		}
		switch item.Type {
		case token.AnchorType, token.AliasType:
			return fmt.Errorf("report aliases/anchors are unsupported")
		case token.MappingStartType, token.SequenceStartType:
			depth++
		case token.MappingEndType, token.SequenceEndType:
			depth--
		}
		if depth > maxReportNesting {
			return fmt.Errorf("report nesting limit exceeded")
		}
	}
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return fmt.Errorf("parse report syntax: %w", err)
	}
	if len(file.Docs) != 1 {
		return fmt.Errorf("expected exactly one report document")
	}
	return nil
}

func parseInventorySARIF(data []byte) (*parsedReport, error) {
	var report gemaraconv.SarifReport
	if err := decodeReport(data, &report); err != nil {
		return nil, fmt.Errorf("decode SARIF: %w", err)
	}
	if report.Version != "2.1.0" || len(report.Runs) != 1 {
		return nil, fmt.Errorf("unsupported SARIF version or runs")
	}
	// ToSARIF omits policy, target, and every evidence mapping, and uses the
	// constant file:///scan URI. Even a unique alert projection is not provenance:
	// passed assessments are omitted and another log may have been overwritten.
	// Per FR-015, reject unbounded scope instead of guessing from alert/filename
	// similarity. This also applies to empty (all-passing) SARIF reports.
	return nil, fmt.Errorf("SARIF has no originating EvaluationLog/evidence provenance; " +
		"possible relationships are unbounded; workspace cleanup is blocked")
}

func parseInventoryOSCAL(data []byte) (*parsedReport, error) {
	var document oscal.OscalModels
	if err := decodeReport(data, &document); err != nil {
		return nil, fmt.Errorf("decode OSCAL: %w", err)
	}
	validator, err := oscalValidation.NewValidatorDesiredVersion(document, oscal.Version)
	if err != nil {
		return nil, fmt.Errorf("create OSCAL report validator: %w", err)
	}
	if err := validator.Validate(); err != nil {
		return nil, fmt.Errorf("validate OSCAL report: %w", err)
	}
	assessment := document.AssessmentResults
	if assessment == nil || len(assessment.Results) != 1 {
		return nil, fmt.Errorf("OSCAL requires one complete assessment-results document")
	}
	result := assessment.Results[0]
	policy, ok := strings.CutPrefix(result.Title, "Evaluation: ")
	if !ok || policy == "" || result.Description != "Results from Gemara EvaluationLog "+policy {
		return nil, fmt.Errorf("OSCAL lacks recognized EvaluationLog identity")
	}
	if assessment.Metadata.Title != "Assessment Results: "+policy {
		return nil, fmt.Errorf("OSCAL policy identities disagree")
	}
	parsed := &parsedReport{policy: policy}
	if err := collectOSCALReferences(assessment.BackMatter, parsed); err != nil {
		return nil, err
	}
	if err := collectOSCALEvidence(result, parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func collectOSCALReferences(backMatter *oscal.BackMatter, parsed *parsedReport) error {
	if backMatter == nil || backMatter.Resources == nil {
		return nil
	}
	for _, resource := range *backMatter.Resources {
		props, err := oscalProperties(resource.Props)
		if err != nil {
			return err
		}
		reference := gemara.MappingReference{Id: props["id"], Title: resource.Title,
			Description: resource.Description}
		if resource.Rlinks != nil {
			links := *resource.Rlinks
			if len(links) != 1 {
				return fmt.Errorf("OSCAL mapping-reference links are ambiguous")
			}
			reference.Url = links[0].Href
		}
		parsed.references = append(parsed.references, reference)
	}
	return nil
}

func collectOSCALEvidence(result oscal.Result, parsed *parsedReport) error {
	if result.LocalDefinitions == nil || result.LocalDefinitions.InventoryItems == nil {
		return fmt.Errorf("OSCAL target inventory is missing")
	}
	items := *result.LocalDefinitions.InventoryItems
	if len(items) != 1 {
		return fmt.Errorf("OSCAL target inventory is ambiguous")
	}
	target, err := oscalProperties(items[0].Props)
	if err != nil {
		return err
	}
	var found bool
	parsed.target, found = target["gemara-resource-id"]
	if !found {
		return fmt.Errorf("OSCAL target identity is missing")
	}
	if result.Observations == nil {
		return fmt.Errorf("OSCAL observation inventory is missing")
	}
	for _, observation := range *result.Observations {
		if err := collectOSCALObservation(observation, parsed); err != nil {
			return err
		}
	}
	return nil
}

func collectOSCALObservation(observation oscal.Observation, parsed *parsedReport) error {
	props, err := oscalProperties(observation.Props)
	if err != nil {
		return err
	}
	if props["control-id"] == "" || props["requirement-id"] == "" || props["result"] == "" {
		return fmt.Errorf("OSCAL observation lacks typed assessment identity")
	}
	if observation.RelevantEvidence == nil {
		return nil
	}
	for _, item := range *observation.RelevantEvidence {
		if item.Href != "" {
			return fmt.Errorf("OSCAL evidence href is unsupported; possible scope is unbounded")
		}
		props, err := oscalProperties(item.Props)
		if err != nil {
			return err
		}
		if err := validateOSCALEvidenceProperties(props); err != nil {
			return err
		}
		parsed.evidence = append(parsed.evidence, gemara.Evidence{
			Id: props["id"], Type: gemara.EvidenceType(props["type"]),
			Source: gemara.EvidenceMapping{ReferenceId: props["source-reference-id"],
				Coordinate: props["source-coordinate"], EntryId: props["source-entry-id"],
				Digest: props["source-digest"]},
		})
	}
	return nil
}

func validateOSCALEvidenceProperties(properties map[string]string) error {
	for name := range properties {
		switch name {
		case "id", "type", "collected-at", "source-reference-id", "source-coordinate",
			"source-entry-id", "source-digest":
		default:
			return fmt.Errorf("unsupported OSCAL evidence property %q", name)
		}
	}
	return nil
}

func oscalProperties(properties *[]oscal.Property) (map[string]string, error) {
	result := make(map[string]string)
	if properties == nil {
		return result, nil
	}
	for _, property := range *properties {
		if property.Ns != gemaraNamespace {
			return nil, fmt.Errorf("unsupported OSCAL property namespace %q", property.Ns)
		}
		if _, duplicate := result[property.Name]; duplicate {
			return nil, fmt.Errorf("duplicate OSCAL property %q", property.Name)
		}
		result[property.Name] = property.Value
	}
	return result, nil
}
