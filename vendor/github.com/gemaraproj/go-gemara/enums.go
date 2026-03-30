package gemara

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gemaraproj/go-gemara/internal/loaders"
)

// Result represents the result of a control evaluation
type Result int

// ArtifactType identifies the kind of Gemara artifact for unambiguous parsing
type ArtifactType int

// EntityType specifies the type of entity (human or tool) interacting in the workflow.
type EntityType int

// Lifecycle represents the lifecycle state of a guideline, control, or assessment requirement
type Lifecycle int

// EntryType enumerates the atomic units within Gemara artifacts that can participate in mappings
type EntryType int

// ConfidenceLevel indicates the evaluator's confidence level in an assessment result.
type ConfidenceLevel int

// RelationshipType enumerates the nature of the mapping between entries.
type RelationshipType int

// MethodType enumerates the category of evaluation or enforcement method.
type MethodType int

// Severity defines the allowed impact levels for a risk.
type Severity int

// GuidanceType restricts the possible types that a catalog may be listed as.
type GuidanceType int

// RiskAppetite defines the acceptable level of exposure for a risk category.
type RiskAppetite int

// ModType defines the type of modification to the assessment requirement.
type ModType int

const (
	NotRun Result = iota
	Passed
	Failed
	NeedsReview
	NotApplicable
	Unknown
)

const (
	ControlCatalogArtifact ArtifactType = iota
	EvaluationLogArtifact
	GuidanceCatalogArtifact
	MappingDocumentArtifact
	PolicyArtifact
	ThreatCatalogArtifact
	VectorCatalogArtifact
)

const (
	Human EntityType = iota
	Software
	SoftwareAssisted
)

const (
	LifecycleActive Lifecycle = iota
	LifecycleDraft
	LifecycleDeprecated
	LifecycleRetired
)

const (
	EntryTypeGuideline EntryType = iota
	EntryTypeStatement
	EntryTypeControl
	EntryTypeAssessmentRequirement
	EntryTypeVector
)

const (
	Undetermined ConfidenceLevel = iota
	Low
	Medium
	High
)

const (
	RelImplements RelationshipType = iota
	RelImplementedBy
	RelSupports
	RelSupportedBy
	RelEquivalent
	RelSubsumes
	RelNoMatch
	RelRelatesTo
)

const (
	MethodManual MethodType = iota
	MethodBehavioral
	MethodAutomated
	MethodAutoremediation
	MethodGate
)

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

const (
	GuidanceStandard GuidanceType = iota
	GuidanceRegulation
	GuidanceBestPractice
	GuidanceFramework
)

const (
	RiskAppetiteZero RiskAppetite = iota
	RiskAppetiteLow
	RiskAppetiteModerate
	RiskAppetiteHigh
)

const (
	ModAdd ModType = iota
	ModModify
	ModRemove
	ModReplace
	ModOverride
)

var (
	toString = map[Result]string{
		NotRun:        "Not Run",
		Passed:        "Passed",
		Failed:        "Failed",
		NeedsReview:   "Needs Review",
		NotApplicable: "Not Applicable",
		Unknown:       "Unknown",
	}

	stringToResult = map[string]Result{
		"Not Run":        NotRun,
		"Passed":         Passed,
		"Failed":         Failed,
		"Needs Review":   NeedsReview,
		"Not Applicable": NotApplicable,
		"Unknown":        Unknown,
	}

	lifecycleToString = map[Lifecycle]string{
		LifecycleActive:     "Active",
		LifecycleDraft:      "Draft",
		LifecycleDeprecated: "Deprecated",
		LifecycleRetired:    "Retired",
	}

	stringToLifecycle = map[string]Lifecycle{
		"Active":     LifecycleActive,
		"Draft":      LifecycleDraft,
		"Deprecated": LifecycleDeprecated,
		"Retired":    LifecycleRetired,
	}

	artifactTypeToString = map[ArtifactType]string{
		ControlCatalogArtifact:  "ControlCatalog",
		EvaluationLogArtifact:   "EvaluationLog",
		GuidanceCatalogArtifact: "GuidanceCatalog",
		MappingDocumentArtifact: "MappingDocument",
		PolicyArtifact:          "Policy",
		ThreatCatalogArtifact:   "ThreatCatalog",
		VectorCatalogArtifact:   "VectorCatalog",
	}

	stringToArtifactType = map[string]ArtifactType{
		"ControlCatalog":  ControlCatalogArtifact,
		"EvaluationLog":   EvaluationLogArtifact,
		"GuidanceCatalog": GuidanceCatalogArtifact,
		"MappingDocument": MappingDocumentArtifact,
		"Policy":          PolicyArtifact,
		"ThreatCatalog":   ThreatCatalogArtifact,
		"VectorCatalog":   VectorCatalogArtifact,
	}

	entityTypeToString = map[EntityType]string{
		Human:            "Human",
		Software:         "Software",
		SoftwareAssisted: "Software Assisted",
	}

	stringToEntityType = map[string]EntityType{
		"Human":             Human,
		"Software":          Software,
		"Software Assisted": SoftwareAssisted,
	}

	entryTypeToString = map[EntryType]string{
		EntryTypeGuideline:             "Guideline",
		EntryTypeStatement:             "Statement",
		EntryTypeControl:               "Control",
		EntryTypeAssessmentRequirement: "AssessmentRequirement",
		EntryTypeVector:                "Vector",
	}

	stringToEntryType = map[string]EntryType{
		"Guideline":             EntryTypeGuideline,
		"Statement":             EntryTypeStatement,
		"Control":               EntryTypeControl,
		"AssessmentRequirement": EntryTypeAssessmentRequirement,
		"Vector":                EntryTypeVector,
	}

	confidenceLevelToString = map[ConfidenceLevel]string{
		Undetermined: "Undetermined",
		Low:          "Low",
		Medium:       "Medium",
		High:         "High",
	}

	stringToConfidenceLevel = map[string]ConfidenceLevel{
		"Undetermined": Undetermined,
		"Low":          Low,
		"Medium":       Medium,
		"High":         High,
	}

	relationshipTypeToString = map[RelationshipType]string{
		RelImplements:    "implements",
		RelImplementedBy: "implemented-by",
		RelSupports:      "supports",
		RelSupportedBy:   "supported-by",
		RelEquivalent:    "equivalent",
		RelSubsumes:      "subsumes",
		RelNoMatch:       "no-match",
		RelRelatesTo:     "relates-to",
	}

	stringToRelationshipType = map[string]RelationshipType{
		"implements":     RelImplements,
		"implemented-by": RelImplementedBy,
		"supports":       RelSupports,
		"supported-by":   RelSupportedBy,
		"equivalent":     RelEquivalent,
		"subsumes":       RelSubsumes,
		"no-match":       RelNoMatch,
		"relates-to":     RelRelatesTo,
	}

	methodTypeToString = map[MethodType]string{
		MethodManual:          "Manual",
		MethodBehavioral:      "Behavioral",
		MethodAutomated:       "Automated",
		MethodAutoremediation: "Autoremediation",
		MethodGate:            "Gate",
	}

	stringToMethodType = map[string]MethodType{
		"Manual":          MethodManual,
		"Behavioral":      MethodBehavioral,
		"Automated":       MethodAutomated,
		"Autoremediation": MethodAutoremediation,
		"Gate":            MethodGate,
	}

	severityToString = map[Severity]string{
		SeverityLow:      "Low",
		SeverityMedium:   "Medium",
		SeverityHigh:     "High",
		SeverityCritical: "Critical",
	}

	stringToSeverity = map[string]Severity{
		"Low":      SeverityLow,
		"Medium":   SeverityMedium,
		"High":     SeverityHigh,
		"Critical": SeverityCritical,
	}

	guidanceTypeToString = map[GuidanceType]string{
		GuidanceStandard:     "Standard",
		GuidanceRegulation:   "Regulation",
		GuidanceBestPractice: "Best Practice",
		GuidanceFramework:    "Framework",
	}

	stringToGuidanceType = map[string]GuidanceType{
		"Standard":      GuidanceStandard,
		"Regulation":    GuidanceRegulation,
		"Best Practice": GuidanceBestPractice,
		"Framework":     GuidanceFramework,
	}

	riskAppetiteToString = map[RiskAppetite]string{
		RiskAppetiteZero:     "Zero",
		RiskAppetiteLow:      "Low",
		RiskAppetiteModerate: "Moderate",
		RiskAppetiteHigh:     "High",
	}

	stringToRiskAppetite = map[string]RiskAppetite{
		"Zero":     RiskAppetiteZero,
		"Low":      RiskAppetiteLow,
		"Moderate": RiskAppetiteModerate,
		"High":     RiskAppetiteHigh,
	}

	modTypeToString = map[ModType]string{
		ModAdd:      "Add",
		ModModify:   "Modify",
		ModRemove:   "Remove",
		ModReplace:  "Replace",
		ModOverride: "Override",
	}

	stringToModType = map[string]ModType{
		"Add":      ModAdd,
		"Modify":   ModModify,
		"Remove":   ModRemove,
		"Replace":  ModReplace,
		"Override": ModOverride,
	}
)

// enumStringer is used by marshal helpers. Implemented by all string-backed enums.
type enumStringer interface {
	String() string
}

func marshalYAMLString(s enumStringer) (interface{}, error) {
	return s.String(), nil
}

func marshalJSONString(s enumStringer) ([]byte, error) {
	return json.Marshal(s.String())
}

func unmarshalYAMLEnum[T any](data []byte, m map[string]T, name string, dest *T) error {
	var s string
	if err := loaders.UnmarshalYAML(data, &s); err != nil {
		return err
	}
	if val, ok := m[s]; ok {
		*dest = val
		return nil
	}
	return unknownEnumStringError(name, s, m)
}

func unmarshalJSONEnum[T any](data []byte, m map[string]T, name string, dest *T) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if val, ok := m[s]; ok {
		*dest = val
		return nil
	}
	return unknownEnumStringError(name, s, m)
}

// unknownEnumStringError builds an error for an invalid enum string, including valid values.
func unknownEnumStringError[T any](name, got string, validMap map[string]T) error {
	valid := make([]string, 0, len(validMap))
	for k := range validMap {
		valid = append(valid, k)
	}
	sort.Strings(valid)
	return fmt.Errorf("invalid %s: %q (valid: %s)", name, got, strings.Join(valid, ", "))
}

func (r Result) String() string {
	if s, ok := toString[r]; ok {
		return s
	}
	return fmt.Sprintf("Result(%d)", r)
}

// MarshalYAML ensures that Result is serialized as a string in YAML
func (r Result) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(r)
}

// MarshalJSON ensures that Result is serialized as a string in JSON
func (r Result) MarshalJSON() ([]byte, error) {
	return marshalJSONString(r)
}

// UnmarshalYAML ensures that Result can be deserialized from a YAML string
func (r *Result) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToResult, "Result", r)
}

// UnmarshalJSON ensures that Result can be deserialized from a JSON string
func (r *Result) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToResult, "Result", r)
}

func (a ArtifactType) String() string {
	if s, ok := artifactTypeToString[a]; ok {
		return s
	}
	return fmt.Sprintf("ArtifactType(%d)", a)
}

// MarshalYAML ensures that ArtifactType is serialized as a string in YAML
func (a ArtifactType) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(a)
}

// MarshalJSON ensures that ArtifactType is serialized as a string in JSON
func (a ArtifactType) MarshalJSON() ([]byte, error) {
	return marshalJSONString(a)
}

// UnmarshalYAML ensures that ArtifactType can be deserialized from a YAML string
func (a *ArtifactType) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToArtifactType, "ArtifactType", a)
}

// UnmarshalJSON ensures that ArtifactType can be deserialized from a JSON string
func (a *ArtifactType) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToArtifactType, "ArtifactType", a)
}

func (e EntityType) String() string {
	if s, ok := entityTypeToString[e]; ok {
		return s
	}
	return fmt.Sprintf("EntityType(%d)", e)
}

// MarshalYAML ensures that EntityType is serialized as a string in YAML
func (e EntityType) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(e)
}

// MarshalJSON ensures that EntityType is serialized as a string in JSON
func (e EntityType) MarshalJSON() ([]byte, error) {
	return marshalJSONString(e)
}

// UnmarshalYAML ensures that EntityType can be deserialized from a YAML string
func (e *EntityType) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToEntityType, "EntityType", e)
}

// UnmarshalJSON ensures that EntityType can be deserialized from a JSON string
func (e *EntityType) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToEntityType, "EntityType", e)
}

func (l Lifecycle) String() string {
	if s, ok := lifecycleToString[l]; ok {
		return s
	}
	return fmt.Sprintf("Lifecycle(%d)", l)
}

// MarshalYAML ensures that Lifecycle is serialized as a string in YAML
func (l Lifecycle) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(l)
}

// MarshalJSON ensures that Lifecycle is serialized as a string in JSON
func (l Lifecycle) MarshalJSON() ([]byte, error) {
	return marshalJSONString(l)
}

// UnmarshalYAML ensures that Lifecycle can be deserialized from a YAML string
func (l *Lifecycle) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToLifecycle, "Lifecycle", l)
}

// UnmarshalJSON ensures that Lifecycle can be deserialized from a JSON string
func (l *Lifecycle) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToLifecycle, "Lifecycle", l)
}

func (e EntryType) String() string {
	if s, ok := entryTypeToString[e]; ok {
		return s
	}
	return fmt.Sprintf("EntryType(%d)", e)
}

// MarshalYAML ensures that EntryType is serialized as a string in YAML
func (e EntryType) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(e)
}

// MarshalJSON ensures that EntryType is serialized as a string in JSON
func (e EntryType) MarshalJSON() ([]byte, error) {
	return marshalJSONString(e)
}

// UnmarshalYAML ensures that EntryType can be deserialized from a YAML string
func (e *EntryType) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToEntryType, "EntryType", e)
}

// UnmarshalJSON ensures that EntryType can be deserialized from a JSON string
func (e *EntryType) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToEntryType, "EntryType", e)
}

func (c ConfidenceLevel) String() string {
	if s, ok := confidenceLevelToString[c]; ok {
		return s
	}
	return fmt.Sprintf("ConfidenceLevel(%d)", c)
}

// MarshalYAML ensures that ConfidenceLevel is serialized as a string in YAML
func (c ConfidenceLevel) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(c)
}

// MarshalJSON ensures that ConfidenceLevel is serialized as a string in JSON
func (c ConfidenceLevel) MarshalJSON() ([]byte, error) {
	return marshalJSONString(c)
}

// UnmarshalYAML ensures that ConfidenceLevel can be deserialized from a YAML string
func (c *ConfidenceLevel) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToConfidenceLevel, "ConfidenceLevel", c)
}

// UnmarshalJSON ensures that ConfidenceLevel can be deserialized from a JSON string
func (c *ConfidenceLevel) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToConfidenceLevel, "ConfidenceLevel", c)
}

func (r RelationshipType) String() string {
	if s, ok := relationshipTypeToString[r]; ok {
		return s
	}
	return fmt.Sprintf("RelationshipType(%d)", r)
}

// MarshalYAML ensures that RelationshipType is serialized as a string in YAML
func (r RelationshipType) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(r)
}

// MarshalJSON ensures that RelationshipType is serialized as a string in JSON
func (r RelationshipType) MarshalJSON() ([]byte, error) {
	return marshalJSONString(r)
}

// UnmarshalYAML ensures that RelationshipType can be deserialized from a YAML string
func (r *RelationshipType) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToRelationshipType, "RelationshipType", r)
}

// UnmarshalJSON ensures that RelationshipType can be deserialized from a JSON string
func (r *RelationshipType) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToRelationshipType, "RelationshipType", r)
}

func (m MethodType) String() string {
	if s, ok := methodTypeToString[m]; ok {
		return s
	}
	return fmt.Sprintf("MethodType(%d)", m)
}

// MarshalYAML ensures that MethodType is serialized as a string in YAML
func (m MethodType) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(m)
}

// MarshalJSON ensures that MethodType is serialized as a string in JSON
func (m MethodType) MarshalJSON() ([]byte, error) {
	return marshalJSONString(m)
}

// UnmarshalYAML ensures that MethodType can be deserialized from a YAML string
func (m *MethodType) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToMethodType, "MethodType", m)
}

// UnmarshalJSON ensures that MethodType can be deserialized from a JSON string
func (m *MethodType) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToMethodType, "MethodType", m)
}

func (s Severity) String() string {
	if str, ok := severityToString[s]; ok {
		return str
	}
	return fmt.Sprintf("Severity(%d)", s)
}

// MarshalYAML ensures that Severity is serialized as a string in YAML
func (s Severity) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(s)
}

// MarshalJSON ensures that Severity is serialized as a string in JSON
func (s Severity) MarshalJSON() ([]byte, error) {
	return marshalJSONString(s)
}

// UnmarshalYAML ensures that Severity can be deserialized from a YAML string
func (s *Severity) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToSeverity, "Severity", s)
}

// UnmarshalJSON ensures that Severity can be deserialized from a JSON string
func (s *Severity) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToSeverity, "Severity", s)
}

func (g GuidanceType) String() string {
	if s, ok := guidanceTypeToString[g]; ok {
		return s
	}
	return fmt.Sprintf("GuidanceType(%d)", g)
}

// MarshalYAML ensures that GuidanceType is serialized as a string in YAML
func (g GuidanceType) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(g)
}

// MarshalJSON ensures that GuidanceType is serialized as a string in JSON
func (g GuidanceType) MarshalJSON() ([]byte, error) {
	return marshalJSONString(g)
}

// UnmarshalYAML ensures that GuidanceType can be deserialized from a YAML string
func (g *GuidanceType) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToGuidanceType, "GuidanceType", g)
}

// UnmarshalJSON ensures that GuidanceType can be deserialized from a JSON string
func (g *GuidanceType) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToGuidanceType, "GuidanceType", g)
}

func (r RiskAppetite) String() string {
	if s, ok := riskAppetiteToString[r]; ok {
		return s
	}
	return fmt.Sprintf("RiskAppetite(%d)", r)
}

// MarshalYAML ensures that RiskAppetite is serialized as a string in YAML
func (r RiskAppetite) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(r)
}

// MarshalJSON ensures that RiskAppetite is serialized as a string in JSON
func (r RiskAppetite) MarshalJSON() ([]byte, error) {
	return marshalJSONString(r)
}

// UnmarshalYAML ensures that RiskAppetite can be deserialized from a YAML string
func (r *RiskAppetite) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToRiskAppetite, "RiskAppetite", r)
}

// UnmarshalJSON ensures that RiskAppetite can be deserialized from a JSON string
func (r *RiskAppetite) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToRiskAppetite, "RiskAppetite", r)
}

func (m ModType) String() string {
	if s, ok := modTypeToString[m]; ok {
		return s
	}
	return fmt.Sprintf("ModType(%d)", m)
}

// MarshalYAML ensures that ModType is serialized as a string in YAML
func (m ModType) MarshalYAML() (interface{}, error) {
	return marshalYAMLString(m)
}

// MarshalJSON ensures that ModType is serialized as a string in JSON
func (m ModType) MarshalJSON() ([]byte, error) {
	return marshalJSONString(m)
}

// UnmarshalYAML ensures that ModType can be deserialized from a YAML string
func (m *ModType) UnmarshalYAML(data []byte) error {
	return unmarshalYAMLEnum(data, stringToModType, "ModType", m)
}

// UnmarshalJSON ensures that ModType can be deserialized from a JSON string
func (m *ModType) UnmarshalJSON(data []byte) error {
	return unmarshalJSONEnum(data, stringToModType, "ModType", m)
}

// UpdateAggregateResult compares the current result with the new result and returns the most severe of the two.
func UpdateAggregateResult(previous Result, new Result) Result {
	if new == NotRun {
		// Not Run should not overwrite anything
		// Failed should not be overwritten by anything
		// Failed should overwrite anything
		return previous
	}

	if previous == Failed || new == Failed {
		// Failed should not be overwritten by anything
		// Failed should overwrite anything
		return Failed
	}

	if previous == Unknown || new == Unknown {
		// If the current or past result is Unknown, it should not be overwritten by NeedsReview or Passed.
		return Unknown
	}

	if previous == NeedsReview || new == NeedsReview {
		// NeedsReview should not be overwritten by Passed
		// NeedsReview should overwrite Passed
		return NeedsReview
	}
	return Passed
}
