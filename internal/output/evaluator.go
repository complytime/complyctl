// SPDX-License-Identifier: Apache-2.0

package output

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gemaraproj/go-gemara"
	"github.com/goccy/go-yaml"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/pkg/provider"
)

// Evaluator accumulates provider assessments for a single target and produces
// a gemara.EvaluationLog grouped by control.
type Evaluator struct {
	policyID           string
	targetID           string
	reqToControl       map[string]string
	reqToPlan          map[string]string
	reqToComplypackRef map[string]string
	controlEvals       map[string]*gemara.ControlEvaluation
	controlOrder       []string
	// controlStepNames tracks step name strings parallel to each control's
	// AssessmentLogs slice. Keyed by control ID, each value is a slice of
	// step name slices (one per assessment log under that control).
	controlStepNames map[string][][]string
}

// defaultMap returns m if non-nil, otherwise a new empty map.
func defaultMap(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	return m
}

// NewEvaluator creates an Evaluator scoped to a single target. reqToControl
// maps requirement IDs to control IDs; pass nil when the catalog is unavailable.
// reqToPlan maps requirement IDs to assessment plan IDs for populating the Plan
// field; pass nil when unavailable. reqToComplypackRef maps requirement IDs
// directly to OCI references (repository@digest) for step identity; pass nil
// when no complypacks are configured.
func NewEvaluator(policyID, targetID string, reqToControl, reqToPlan, reqToComplypackRef map[string]string) *Evaluator {
	return &Evaluator{
		policyID:           policyID,
		targetID:           targetID,
		reqToControl:       defaultMap(reqToControl),
		reqToPlan:          defaultMap(reqToPlan),
		reqToComplypackRef: defaultMap(reqToComplypackRef),
		controlEvals:       make(map[string]*gemara.ControlEvaluation),
		controlStepNames:   make(map[string][][]string),
	}
}

// TargetID returns the target identifier for this evaluator.
func (e *Evaluator) TargetID() string {
	return e.targetID
}

// AddTarget converts provider assessment results for one target into
// gemara ControlEvaluations, grouping assessments by control.
func (e *Evaluator) AddTarget(assessments []provider.AssessmentLog) {
	for i := range assessments {
		a := &assessments[i]
		controlID := e.resolveControl(a.RequirementID)

		gemaraAssessment, stepNames := e.providerToGemaraAssessment(a)

		ce, exists := e.controlEvals[controlID]
		if !exists {
			ce = &gemara.ControlEvaluation{
				Name:   controlID,
				Result: gemara.NotRun,
				Control: gemara.EntryMapping{
					ReferenceId: e.policyID,
					EntryId:     controlID,
				},
			}
			e.controlEvals[controlID] = ce
			e.controlOrder = append(e.controlOrder, controlID)
		}

		ce.AssessmentLogs = append(ce.AssessmentLogs, gemaraAssessment)
		e.controlStepNames[controlID] = append(e.controlStepNames[controlID], stepNames)
		ce.Result = gemara.UpdateAggregateResult(ce.Result, gemaraAssessment.Result)
		ce.Message = gemaraAssessment.Message
	}
}

// GemaraLog returns the assembled gemara.EvaluationLog with fully populated
// metadata, aggregated result, and target resource.
func (e *Evaluator) GemaraLog() *gemara.EvaluationLog {
	evals := make([]*gemara.ControlEvaluation, 0, len(e.controlEvals))
	for _, id := range e.controlOrder {
		evals = append(evals, e.controlEvals[id])
	}

	result := gemara.NotRun
	for _, ce := range evals {
		result = gemara.UpdateAggregateResult(result, ce.Result)
	}

	return &gemara.EvaluationLog{
		Evaluations: evals,
		Result:      result,
		Metadata: gemara.Metadata{
			Id:            e.policyID,
			Type:          gemara.EvaluationLogArtifact,
			GemaraVersion: gemara.SchemaVersion,
			Description:   "Compliance scan evaluation log",
			Author: gemara.Actor{
				Id:   "complytime",
				Name: "complytime",
				Type: gemara.Software,
				Uri:  "https://github.com/complytime/complyctl",
			},
		},
		Target: gemara.Resource{
			Id:   e.targetID,
			Name: e.targetID,
			Type: gemara.Software,
		},
	}
}

// Write serializes the evaluation log as YAML to outDir and returns the path.
// It uses a shadow struct to replace gemara.AssessmentStep function closures
// with human-readable step identity strings.
func (e *Evaluator) Write(outDir string) (string, error) {
	evalLog := e.GemaraLog()
	serializable := e.toSerializable(evalLog)

	data, err := yaml.Marshal(serializable)
	if err != nil {
		return "", fmt.Errorf("failed to marshal evaluation log: %w", err)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("evaluation-log-%s-%s-%s.yaml",
		complytime.FilenameSafe(e.policyID), complytime.FilenameSafe(e.targetID), time.Now().Format("20060102-150405"))
	path := filepath.Join(outDir, filename)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write evaluation log: %w", err)
	}

	return path, nil
}

func (e *Evaluator) resolveControl(requirementID string) string {
	if ctrlID, ok := e.reqToControl[requirementID]; ok {
		return ctrlID
	}
	return requirementID
}

func (e *Evaluator) providerToGemaraAssessment(a *provider.AssessmentLog) (*gemara.AssessmentLog, []string) {
	result := aggregateResultFromSteps(a.Steps)

	msg := a.Message
	if result != gemara.Passed {
		if stepMsg := matchingStepMessage(a.Steps, result); stepMsg != "" {
			msg = stepMsg
		}
	}

	desc := msg
	if desc == "" && len(a.Steps) > 0 {
		desc = a.Steps[0].Name
	}
	if desc == "" {
		desc = a.RequirementID
	}

	gemaraLog := &gemara.AssessmentLog{
		Requirement: gemara.EntryMapping{
			ReferenceId: e.policyID,
			EntryId:     a.RequirementID,
		},
		Description:     desc,
		Result:          result,
		Message:         msg,
		Applicability:   []string{"default"},
		Steps:           providerStepsToGemara(a.Steps, a.Confidence),
		Start:           gemara.Datetime(time.Now().Format(time.RFC3339)),
		StepsExecuted:   int64(len(a.Steps)),
		ConfidenceLevel: providerConfidenceToGemara(a.Confidence),
	}

	// Populate the Plan field when a plan ID mapping exists for this requirement.
	if planID, ok := e.reqToPlan[a.RequirementID]; ok {
		gemaraLog.Plan = &gemara.EntryMapping{
			ReferenceId: e.policyID,
			EntryId:     planID,
		}
	}

	// Collect step names so the shadow struct serialization can produce
	// human-readable step identity strings instead of function pointer names.
	names := make([]string, len(a.Steps))
	for i, s := range a.Steps {
		names[i] = s.Name
	}

	return gemaraLog, names
}

// providerStepToGemara wraps a provider.Step into a gemara.AssessmentStep closure.
// The closure ignores its payload argument and returns the step's pre-computed
// result, message, and the assessment-level confidence.
func providerStepToGemara(step provider.Step, confidence provider.ConfidenceLevel) gemara.AssessmentStep {
	result := providerResultToGemara(step.Result)
	conf := providerConfidenceToGemara(confidence)
	return func(_ interface{}) (gemara.Result, string, gemara.ConfidenceLevel) {
		return result, step.Message, conf
	}
}

// providerStepsToGemara converts a slice of provider steps into gemara AssessmentStep closures.
// Returns nil when the input slice is empty.
func providerStepsToGemara(steps []provider.Step, confidence provider.ConfidenceLevel) []gemara.AssessmentStep {
	if len(steps) == 0 {
		return nil
	}
	gemaraSteps := make([]gemara.AssessmentStep, len(steps))
	for i, s := range steps {
		gemaraSteps[i] = providerStepToGemara(s, confidence)
	}
	return gemaraSteps
}

func providerConfidenceToGemara(c provider.ConfidenceLevel) gemara.ConfidenceLevel {
	switch c {
	case provider.ConfidenceLevelHigh:
		return gemara.High
	case provider.ConfidenceLevelMedium:
		return gemara.Medium
	case provider.ConfidenceLevelLow:
		return gemara.Low
	case provider.ConfidenceLevelUndetermined:
		return gemara.Undetermined
	default:
		// Unknown provider values map to Undetermined as the most conservative confidence level.
		return gemara.Undetermined
	}
}

// Shadow struct types mirror gemara types but replace Steps []AssessmentStep
// (function type) with Steps []string (identity strings) for YAML serialization.

type serializableEvaluationLog struct {
	Metadata    gemara.Metadata                  `yaml:"metadata"`
	Result      gemara.Result                    `yaml:"result"`
	Evaluations []*serializableControlEvaluation `yaml:"evaluations"`
	Target      gemara.Resource                  `yaml:"target"`
}

type serializableControlEvaluation struct {
	Name           string                       `yaml:"name"`
	Result         gemara.Result                `yaml:"result"`
	Message        string                       `yaml:"message"`
	Control        gemara.EntryMapping          `yaml:"control"`
	AssessmentLogs []*serializableAssessmentLog `yaml:"assessment-logs"`
}

type serializableAssessmentLog struct {
	Requirement     gemara.EntryMapping    `yaml:"requirement"`
	Plan            *gemara.EntryMapping   `yaml:"plan,omitempty"`
	Description     string                 `yaml:"description"`
	Result          gemara.Result          `yaml:"result"`
	Message         string                 `yaml:"message"`
	Applicability   []string               `yaml:"applicability"`
	Steps           []string               `yaml:"steps"`
	StepsExecuted   int64                  `yaml:"steps-executed,omitempty"`
	Start           gemara.Datetime        `yaml:"start"`
	End             gemara.Datetime        `yaml:"end,omitempty"`
	Recommendation  string                 `yaml:"recommendation,omitempty"`
	ConfidenceLevel gemara.ConfidenceLevel `yaml:"confidence-level,omitempty"`
}

// formatStepIdentity produces a step identity string. When complypackRef is
// non-empty, the format is "{complypackRef}#{stepName}". Otherwise, the bare
// stepName is returned.
func formatStepIdentity(complypackRef, stepName string) string {
	if complypackRef != "" && stepName != "" {
		return complypackRef + "#" + stepName
	}
	return stepName
}

// resolveComplypackRef returns the pre-resolved OCI reference for the
// complypack associated with the given requirement ID.
func (e *Evaluator) resolveComplypackRef(requirementID string) string {
	return e.reqToComplypackRef[requirementID]
}

// toSerializable converts a gemara.EvaluationLog into the shadow struct,
// replacing AssessmentStep closures with formatted step identity strings.
func (e *Evaluator) toSerializable(log *gemara.EvaluationLog) *serializableEvaluationLog {
	sEvals := make([]*serializableControlEvaluation, len(log.Evaluations))
	for i, ce := range log.Evaluations {
		controlID := ce.Control.EntryId
		controlNames := e.controlStepNames[controlID]
		sLogs := make([]*serializableAssessmentLog, len(ce.AssessmentLogs))
		for j, al := range ce.AssessmentLogs {
			complypackRef := e.resolveComplypackRef(al.Requirement.EntryId)
			var names []string
			if j < len(controlNames) {
				names = controlNames[j]
			}
			steps := make([]string, len(names))
			for k, name := range names {
				steps[k] = formatStepIdentity(complypackRef, name)
			}
			sLogs[j] = &serializableAssessmentLog{
				Requirement:     al.Requirement,
				Plan:            al.Plan,
				Description:     al.Description,
				Result:          al.Result,
				Message:         al.Message,
				Applicability:   al.Applicability,
				Steps:           steps,
				StepsExecuted:   al.StepsExecuted,
				Start:           al.Start,
				End:             al.End,
				Recommendation:  al.Recommendation,
				ConfidenceLevel: al.ConfidenceLevel,
			}
		}
		sEvals[i] = &serializableControlEvaluation{
			Name:           ce.Name,
			Result:         ce.Result,
			Message:        ce.Message,
			Control:        ce.Control,
			AssessmentLogs: sLogs,
		}
	}
	return &serializableEvaluationLog{
		Metadata:    log.Metadata,
		Result:      log.Result,
		Evaluations: sEvals,
		Target:      log.Target,
	}
}
