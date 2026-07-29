// SPDX-License-Identifier: Apache-2.0

package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gemaraproj/go-gemara"

	oscalUUID "github.com/defenseunicorns/go-oscal/src/pkg/uuid"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
)

// FIXME(jpower432): This would probably make more sense in go-gemara/gemaraconv

// ToOSCAL converts a gemara.EvaluationLog to OSCAL assessment-results format.
func ToOSCAL(log *gemara.EvaluationLog, outDir string) (string, error) {
	now := time.Now().UTC()
	policyID := log.Metadata.Id

	findings := make([]oscalTypes.Finding, 0)
	for _, ce := range log.Evaluations {
		for _, al := range ce.AssessmentLogs {
			mapped := gemaraResultToOSCAL(al.Result)
			remarks := al.Message
			if mapped.Reason == "not-applicable" {
				remarks = fmt.Sprintf("Skipped due to Tailoring: %s", al.Message)
			}
			findings = append(findings, oscalTypes.Finding{
				UUID:        newUUID(),
				Title:       fmt.Sprintf("Assessment: %s", al.Requirement.EntryId),
				Description: al.Message,
				Target: oscalTypes.FindingTarget{
					Type:     "objective-id",
					TargetId: al.Requirement.EntryId,
					Status: oscalTypes.ObjectiveStatus{
						State:  mapped.State,
						Reason: mapped.Reason,
					},
				},
				Remarks: remarks,
			})
		}
	}

	result := oscalTypes.Result{
		UUID:        newUUID(),
		Title:       fmt.Sprintf("Policy: %s", policyID),
		Description: fmt.Sprintf("Assessment results for policy %s", policyID),
		Start:       now,
		ReviewedControls: oscalTypes.ReviewedControls{
			ControlSelections: []oscalTypes.AssessedControls{
				{Description: "All controls assessed for policy " + policyID},
			},
		},
	}
	if len(findings) > 0 {
		result.Findings = &findings
	}

	doc := oscalTypes.OscalModels{
		AssessmentResults: &oscalTypes.AssessmentResults{
			UUID: newUUID(),
			Metadata: oscalTypes.Metadata{
				Title:        fmt.Sprintf("Compliance scan: %s", policyID),
				LastModified: now,
				Version:      "1.0.0",
				OscalVersion: oscalTypes.Version,
			},
			ImportAp: oscalTypes.ImportAp{
				Href: fmt.Sprintf("file://assessment-plan-%s.json", policyID),
			},
			Results: []oscalTypes.Result{result},
		},
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

func newUUID() string {
	return oscalUUID.NewUUID()
}

type oscalState struct {
	State  string
	Reason string
}

func gemaraResultToOSCAL(r gemara.Result) oscalState {
	switch r {
	case gemara.Passed:
		return oscalState{State: "satisfied"}
	case gemara.Failed:
		return oscalState{State: "not-satisfied"}
	case gemara.NotApplicable:
		return oscalState{State: "satisfied", Reason: "not-applicable"}
	case gemara.Unknown:
		return oscalState{State: "not-satisfied", Reason: "unknown"}
	default:
		return oscalState{State: "not-satisfied", Reason: "unknown"}
	}
}
