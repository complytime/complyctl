// SPDX-License-Identifier: Apache-2.0

package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	oscalValidation "github.com/defenseunicorns/go-oscal/src/pkg/validation"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/gemaraconv"
)

const oscalReportVersion = "2.0.0"

// ToOSCAL converts a gemara.EvaluationLog to OSCAL assessment-results format.
func ToOSCAL(log *gemara.EvaluationLog, outDir string) (string, error) {
	if log == nil {
		return "", fmt.Errorf("convert EvaluationLog to OSCAL assessment results: evaluation log is required")
	}
	if log.Metadata.Id == "" {
		return "", fmt.Errorf("convert EvaluationLog to OSCAL assessment results: evaluation log policy ID is required")
	}
	if len(log.Evaluations) == 0 {
		return "", fmt.Errorf("convert EvaluationLog to OSCAL assessment results: evaluation log has no evaluations")
	}
	for _, evaluation := range log.Evaluations {
		if evaluation == nil {
			return "", fmt.Errorf("convert EvaluationLog to OSCAL assessment results: evaluation log contains an incomplete evaluation")
		}
		for _, assessment := range evaluation.AssessmentLogs {
			if assessment == nil {
				return "", fmt.Errorf("convert EvaluationLog to OSCAL assessment results: evaluation log contains an incomplete assessment")
			}
		}
	}
	policyID := log.Metadata.Id
	evaluationLog := *log
	evaluationLog.Metadata.Version = oscalReportVersion

	assessmentResults, err := gemaraconv.EvaluationLogToOSCALAssessmentResults(
		evaluationLog,
		gemaraconv.WithImportApHref(fmt.Sprintf("file://assessment-plan-%s.json", policyID)),
	)
	if err != nil {
		return "", fmt.Errorf("convert EvaluationLog to OSCAL assessment results: %w", err)
	}
	doc := oscalTypes.OscalModels{
		AssessmentResults: &assessmentResults,
	}
	validator, err := oscalValidation.NewValidatorDesiredVersion(doc, oscalTypes.Version)
	if err != nil {
		return "", fmt.Errorf("create OSCAL schema validator: %w", err)
	}
	if err := validator.Validate(); err != nil {
		return "", fmt.Errorf("validate OSCAL assessment results: %w", err)
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal OSCAL: %w", err)
	}

	if outDir == "" {
		outDir = "."
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := BuildReportFilename("assessment-results", policyID, log.Target.Id, "json")
	path := filepath.Join(outDir, filename)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write OSCAL file: %w", err)
	}
	return path, nil
}
