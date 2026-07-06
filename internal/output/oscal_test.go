// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	oscalValidation "github.com/defenseunicorns/go-oscal/src/pkg/validation"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/output"
)

func mockGemaraEvalLog() *gemara.EvaluationLog {
	return &gemara.EvaluationLog{
		Metadata: gemara.Metadata{
			Id:            "test-policy",
			GemaraVersion: gemara.SchemaVersion,
			Description:   "Test evaluation log",
			Author: gemara.Actor{
				Id:   "complytime",
				Name: "complytime",
				Type: gemara.Software,
			},
		},
		Evaluations: []*gemara.ControlEvaluation{
			{
				Name:    "ctrl-1",
				Result:  gemara.Passed,
				Message: "ok",
				Control: gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "ctrl-1"},
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement:     gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "req-1"},
						Description:     "passed",
						Result:          gemara.Passed,
						Message:         "passed",
						Applicability:   []string{"default"},
						Start:           "2026-01-01T00:00:00Z",
						StepsExecuted:   1,
						ConfidenceLevel: gemara.High,
					},
				},
			},
			{
				Name:    "ctrl-2",
				Result:  gemara.NotApplicable,
				Message: "tailored",
				Control: gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "ctrl-2"},
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement:     gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "req-2"},
						Description:     "tailored",
						Result:          gemara.NotApplicable,
						Message:         "tailored",
						Applicability:   []string{"default"},
						Start:           "2026-01-01T00:00:00Z",
						StepsExecuted:   1,
						ConfidenceLevel: gemara.Undetermined,
					},
				},
			},
		},
	}
}

func TestToOSCAL_ProducesValidJSON(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()

	path, err := output.ToOSCAL(log, outDir)
	require.NoError(t, err)
	assert.FileExists(t, path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var doc oscalTypes.OscalModels
	require.NoError(t, json.Unmarshal(data, &doc))

	require.NotNil(t, doc.AssessmentResults, "assessment-results must be present")
	assert.Equal(t, oscalTypes.Version, doc.AssessmentResults.Metadata.OscalVersion)
	assert.NotEmpty(t, doc.AssessmentResults.UUID)
	assert.NotEmpty(t, doc.AssessmentResults.Results)

	assert.Contains(t, doc.AssessmentResults.ImportAp.Href, "test-policy")

	validator, err := oscalValidation.NewValidatorDesiredVersion(doc, oscalTypes.Version)
	require.NoError(t, err)
	assert.NoError(t, validator.Validate())
}

func TestToOSCAL_SkippedMapsToNotApplicable(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()

	path, err := output.ToOSCAL(log, outDir)
	require.NoError(t, err)

	data, _ := os.ReadFile(path)
	var doc oscalTypes.OscalModels
	require.NoError(t, json.Unmarshal(data, &doc))

	require.NotNil(t, doc.AssessmentResults)
	require.NotNil(t, doc.AssessmentResults.Results[0].Findings)
	findings := *doc.AssessmentResults.Results[0].Findings
	require.Len(t, findings, 2)

	assert.Equal(t, "satisfied", findings[0].Target.Status.State)
	assert.Empty(t, findings[0].Target.Status.Reason)
	assert.Equal(t, "satisfied", findings[1].Target.Status.State)
	assert.Equal(t, "not-applicable", findings[1].Target.Status.Reason)
	assert.Contains(t, findings[1].Remarks, "Skipped due to Tailoring")
}

func TestToOSCAL_ProperUUIDs(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()

	path, err := output.ToOSCAL(log, outDir)
	require.NoError(t, err)

	data, _ := os.ReadFile(path)
	var doc oscalTypes.OscalModels
	require.NoError(t, json.Unmarshal(data, &doc))

	require.NotNil(t, doc.AssessmentResults)
	assert.Len(t, doc.AssessmentResults.UUID, 36)
	for _, result := range doc.AssessmentResults.Results {
		assert.Len(t, result.UUID, 36)
		if result.Findings != nil {
			for _, finding := range *result.Findings {
				assert.Len(t, finding.UUID, 36)
			}
		}
	}
}

func TestToOSCAL_AllResultMappings(t *testing.T) {
	tests := []struct {
		name           string
		result         gemara.Result
		expectedState  string
		expectedReason string
	}{
		{"passed", gemara.Passed, "satisfied", ""},
		{"failed", gemara.Failed, "not-satisfied", ""},
		{"not applicable", gemara.NotApplicable, "satisfied", "not-applicable"},
		{"unknown", gemara.Unknown, "not-satisfied", "unknown"},
		{"not run (default)", gemara.NotRun, "not-satisfied", "unknown"},
		{"needs review (default)", gemara.NeedsReview, "not-satisfied", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outDir := t.TempDir()
			log := &gemara.EvaluationLog{
				Metadata: gemara.Metadata{
					Id:            "test-policy",
					GemaraVersion: gemara.SchemaVersion,
					Author:        gemara.Actor{Id: "test", Name: "test", Type: gemara.Software},
				},
				Evaluations: []*gemara.ControlEvaluation{{
					Name:    "ctrl-1",
					Result:  tt.result,
					Message: "test",
					Control: gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "ctrl-1"},
					AssessmentLogs: []*gemara.AssessmentLog{{
						Requirement: gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "req-1"},
						Result:      tt.result,
						Message:     "test message",
					}},
				}},
			}

			path, err := output.ToOSCAL(log, outDir)
			require.NoError(t, err)

			data, err := os.ReadFile(path)
			require.NoError(t, err)

			var doc oscalTypes.OscalModels
			require.NoError(t, json.Unmarshal(data, &doc))

			require.NotNil(t, doc.AssessmentResults)
			require.NotNil(t, doc.AssessmentResults.Results[0].Findings)
			findings := *doc.AssessmentResults.Results[0].Findings
			require.Len(t, findings, 1)

			assert.Equal(t, tt.expectedState, findings[0].Target.Status.State, "state mismatch")
			assert.Equal(t, tt.expectedReason, findings[0].Target.Status.Reason, "reason mismatch")
		})
	}
}

func TestToOSCAL_OutputFileNaming(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()

	path, err := output.ToOSCAL(log, outDir)
	require.NoError(t, err)

	filename := filepath.Base(path)
	assert.Contains(t, filename, "assessment-results-test-policy-")
	assert.Contains(t, filename, ".json")
}
