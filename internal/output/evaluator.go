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

// Evaluator accumulates per-target provider assessments and produces a
// gemara.EvaluationLog grouped by control.
type Evaluator struct {
	policyID     string
	reqToControl map[string]string
	controlEvals map[string]*gemara.ControlEvaluation
	controlOrder []string
}

// NewEvaluator creates an Evaluator. reqToControl maps requirement IDs to
// control IDs; pass nil when the catalog is unavailable.
func NewEvaluator(policyID string, reqToControl map[string]string) *Evaluator {
	if reqToControl == nil {
		reqToControl = make(map[string]string)
	}
	return &Evaluator{
		policyID:     policyID,
		reqToControl: reqToControl,
		controlEvals: make(map[string]*gemara.ControlEvaluation),
	}
}

// AddTarget converts provider assessment results for one target into
// gemara ControlEvaluations, grouping assessments by control.
func (e *Evaluator) AddTarget(assessments []provider.AssessmentLog) {
	for i := range assessments {
		a := &assessments[i]
		controlID := e.resolveControl(a.RequirementID)

		gemaraAssessment := e.providerToGemaraAssessment(a)

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
		ce.Result = gemara.UpdateAggregateResult(ce.Result, gemaraAssessment.Result)
		ce.Message = gemaraAssessment.Message
	}
}

// GemaraLog returns the assembled gemara.EvaluationLog.
func (e *Evaluator) GemaraLog() *gemara.EvaluationLog {
	evals := make([]*gemara.ControlEvaluation, 0, len(e.controlEvals))
	for _, id := range e.controlOrder {
		evals = append(evals, e.controlEvals[id])
	}

	return &gemara.EvaluationLog{
		Evaluations: evals,
		Metadata: gemara.Metadata{
			Id:          e.policyID,
			Description: "Compliance scan evaluation log",
			Author: gemara.Actor{
				Id:   "complytime",
				Name: "complytime",
				Type: gemara.Software,
				Uri:  "https://github.com/complytime/complyctl",
			},
		},
	}
}

// Write serializes the evaluation log as YAML to outDir and returns the path.
func (e *Evaluator) Write(outDir string) (string, error) {
	evalLog := e.GemaraLog()

	data, err := yaml.Marshal(evalLog)
	if err != nil {
		return "", fmt.Errorf("failed to marshal evaluation log: %w", err)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("evaluation-log-%s-%s.yaml",
		complytime.FilenameSafe(e.policyID), time.Now().Format("20060102-150405"))
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

func (e *Evaluator) providerToGemaraAssessment(a *provider.AssessmentLog) *gemara.AssessmentLog {
	result := aggregateResultFromSteps(a.Steps)
	desc := a.Message
	if desc == "" && len(a.Steps) > 0 {
		desc = a.Steps[0].Name
	}
	if desc == "" {
		desc = a.RequirementID
	}

	return &gemara.AssessmentLog{
		Requirement: gemara.EntryMapping{
			ReferenceId: e.policyID,
			EntryId:     a.RequirementID,
		},
		Description:     desc,
		Result:          result,
		Message:         a.Message,
		Applicability:   []string{"default"},
		Start:           gemara.Datetime(time.Now().Format(time.RFC3339)),
		StepsExecuted:   int64(len(a.Steps)),
		ConfidenceLevel: providerConfidenceToGemara(a.Confidence),
	}
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
