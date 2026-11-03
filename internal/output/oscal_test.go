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
		Target: gemara.Resource{Id: "web-server", Name: "web-server", Type: gemara.Software},
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

func TestToOSCAL_EvidenceFreeReportValidatesAgainstSchema(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()
	log.Evaluations = log.Evaluations[:1]

	path, err := output.ToOSCAL(log, outDir)
	require.NoError(t, err)
	assert.FileExists(t, path)

	doc := readOSCALDocument(t, path)

	require.NotNil(t, doc.AssessmentResults, "assessment-results must be present")
	assert.Equal(t, oscalTypes.Version, doc.AssessmentResults.Metadata.OscalVersion)
	assert.NotEmpty(t, doc.AssessmentResults.UUID)
	assert.NotEmpty(t, doc.AssessmentResults.Results)

	assert.Contains(t, doc.AssessmentResults.ImportAp.Href, "test-policy")

	assertValidOSCALSchema(t, doc)
}

func TestToOSCAL_EvidencePreservesSourceProvenance(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()
	log.Evaluations = log.Evaluations[:1]
	log.Metadata.MappingReferences = []gemara.MappingReference{{
		Id:      "scanner-output",
		Title:   "Scanner output",
		Version: "1.0.0",
		Url:     "https://example.test/scanner-output.json",
	}}
	log.Evaluations[0].AssessmentLogs[0].Evidence = []gemara.Evidence{{
		Id:          "evidence-1",
		Type:        gemara.EvidenceType("command-output"),
		CollectedAt: gemara.Datetime("2026-01-01T00:00:00Z"),
		Description: "The scanner found the required configuration.",
		Source: gemara.EvidenceMapping{
			ReferenceId: "scanner-output",
			Coordinate:  "$.configuration.required",
			Digest:      "sha256:0123456789abcdef",
			Remarks:     "Collected during the compliance scan.",
		},
	}}

	path, err := output.ToOSCAL(log, outDir)
	require.NoError(t, err)

	doc := readOSCALDocument(t, path)
	assertValidOSCALSchema(t, doc)

	require.NotNil(t, doc.AssessmentResults)
	require.Len(t, doc.AssessmentResults.Results, 1)
	require.NotNil(t, doc.AssessmentResults.Results[0].Observations)
	observations := *doc.AssessmentResults.Results[0].Observations
	require.Len(t, observations, 1)
	require.NotNil(t, observations[0].RelevantEvidence)
	require.Len(t, *observations[0].RelevantEvidence, 1)

	evidence := (*observations[0].RelevantEvidence)[0]
	assert.Equal(t, "The scanner found the required configuration.", evidence.Description)
	assert.Equal(t, "Collected during the compliance scan.", evidence.Remarks)
	require.NotNil(t, evidence.Props)
	assert.Contains(t, *evidence.Props, oscalTypes.Property{
		Name:  "source-reference-id",
		Value: "scanner-output",
		Ns:    "https://github.com/gemaraproj/go-gemara/ns/oscal",
	})
	assert.Contains(t, *evidence.Props, oscalTypes.Property{
		Name:  "source-coordinate",
		Value: "$.configuration.required",
		Ns:    "https://github.com/gemaraproj/go-gemara/ns/oscal",
	})
	assert.Contains(t, *evidence.Props, oscalTypes.Property{
		Name:  "source-digest",
		Value: "sha256:0123456789abcdef",
		Ns:    "https://github.com/gemaraproj/go-gemara/ns/oscal",
	})
	require.NotNil(t, evidence.Links)
	assert.Len(t, *evidence.Links, 1)
	assert.Equal(t, "resource", (*evidence.Links)[0].Rel)
}

func TestToOSCAL_ControlFindingLinksEveryAssessmentObservation(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()
	log.Evaluations = log.Evaluations[:1]
	log.Evaluations[0].AssessmentLogs = append(log.Evaluations[0].AssessmentLogs, &gemara.AssessmentLog{
		Requirement:     gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "req-2"},
		Description:     "second assessment",
		Result:          gemara.Passed,
		Message:         "passed",
		Applicability:   []string{"default"},
		Start:           "2026-01-01T00:00:00Z",
		StepsExecuted:   1,
		ConfidenceLevel: gemara.High,
	})

	path, err := output.ToOSCAL(log, outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc oscalTypes.OscalModels
	require.NoError(t, json.Unmarshal(data, &doc))

	require.NotNil(t, doc.AssessmentResults)
	require.Len(t, doc.AssessmentResults.Results, 1)
	result := doc.AssessmentResults.Results[0]
	require.NotNil(t, result.Findings)
	require.Len(t, *result.Findings, 1)
	require.NotNil(t, result.Observations)
	require.Len(t, *result.Observations, 2)

	observationUUIDs := make(map[string]struct{}, len(*result.Observations))
	for _, observation := range *result.Observations {
		observationUUIDs[observation.UUID] = struct{}{}
	}

	relatedObservations := (*result.Findings)[0].RelatedObservations
	require.NotNil(t, relatedObservations)
	require.Len(t, *relatedObservations, len(observationUUIDs))
	for _, relatedObservation := range *relatedObservations {
		_, found := observationUUIDs[relatedObservation.ObservationUuid]
		assert.True(t, found, "finding must link to every assessment observation")
		delete(observationUUIDs, relatedObservation.ObservationUuid)
	}
	assert.Empty(t, observationUUIDs, "every assessment observation must be linked")
}

func TestToOSCAL_NilOrIncompleteLogReturnsErrorWithoutWritingReport(t *testing.T) {
	tests := []struct {
		name string
		log  *gemara.EvaluationLog
	}{
		{
			name: "nil log",
		},
		{
			name: "missing policy ID",
			log:  &gemara.EvaluationLog{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outDir := t.TempDir()

			path, err := output.ToOSCAL(tt.log, outDir)

			require.Error(t, err)
			assert.Empty(t, path)
			entries, readErr := os.ReadDir(outDir)
			require.NoError(t, readErr)
			assert.Empty(t, entries)
		})
	}
}

func TestToOSCAL_NotApplicableUsesGemaraConverterStatus(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()

	path, err := output.ToOSCAL(log, outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc oscalTypes.OscalModels
	require.NoError(t, json.Unmarshal(data, &doc))

	require.NotNil(t, doc.AssessmentResults)
	require.NotNil(t, doc.AssessmentResults.Results[0].Findings)
	findings := *doc.AssessmentResults.Results[0].Findings
	require.Len(t, findings, 2)
	require.NotNil(t, doc.AssessmentResults.Results[0].Observations)
	observations := *doc.AssessmentResults.Results[0].Observations
	require.Len(t, observations, 2)

	assert.Equal(t, "ctrl-1", findings[0].Target.TargetId)
	assert.Equal(t, "satisfied", findings[0].Target.Status.State)
	assert.Empty(t, findings[0].Target.Status.Reason)
	assert.Equal(t, "ok", findings[0].Description)
	assert.Equal(t, "passed", observations[0].Description)

	assert.Equal(t, "ctrl-2", findings[1].Target.TargetId)
	assert.Equal(t, "not-satisfied", findings[1].Target.Status.State)
	assert.Equal(t, "Not Applicable", findings[1].Target.Status.Reason)
	assert.Empty(t, findings[1].Remarks)
	assert.Equal(t, "tailored", findings[1].Description)
	assert.Equal(t, "tailored", observations[1].Description)
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
		{"not applicable", gemara.NotApplicable, "not-satisfied", "Not Applicable"},
		{"unknown", gemara.Unknown, "not-satisfied", "Unknown"},
		{"not run", gemara.NotRun, "not-satisfied", "Not Run"},
		{"needs review", gemara.NeedsReview, "not-satisfied", "Needs Review"},
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
	assert.Contains(t, filename, "web-server")
	assert.Contains(t, filename, ".json")
}

func TestMultiTargetDistinctFilenames(t *testing.T) {
	outDir := t.TempDir()

	makeLog := func(targetID, targetName string) *gemara.EvaluationLog {
		return &gemara.EvaluationLog{
			Metadata: gemara.Metadata{
				Id:            "test-policy",
				GemaraVersion: gemara.SchemaVersion,
				Author:        gemara.Actor{Id: "test", Name: "test", Type: gemara.Software},
			},
			Target: gemara.Resource{Id: targetID, Name: targetName, Type: gemara.Software},
			Evaluations: []*gemara.ControlEvaluation{{
				Name:    "ctrl-1",
				Result:  gemara.Passed,
				Message: "ok",
				Control: gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "ctrl-1"},
				AssessmentLogs: []*gemara.AssessmentLog{{
					Requirement: gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "req-1"},
					Result:      gemara.Passed,
					Message:     "passed",
				}},
			}},
		}
	}

	path1, err := output.ToOSCAL(makeLog("web-server", "web-server"), outDir)
	require.NoError(t, err)

	path2, err := output.ToOSCAL(makeLog("db-server", "db-server"), outDir)
	require.NoError(t, err)

	require.NotEqual(t, path1, path2, "multi-target scans must produce distinct file paths")

	assert.FileExists(t, path1)
	assert.FileExists(t, path2)

	assert.Contains(t, filepath.Base(path1), "web-server")
	assert.Contains(t, filepath.Base(path2), "db-server")
}

func readOSCALDocument(t *testing.T, path string) oscalTypes.OscalModels {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var doc oscalTypes.OscalModels
	require.NoError(t, json.Unmarshal(data, &doc))
	return doc
}

func assertValidOSCALSchema(t *testing.T, doc oscalTypes.OscalModels) {
	t.Helper()

	validator, err := oscalValidation.NewValidatorDesiredVersion(doc, oscalTypes.Version)
	require.NoError(t, err)
	assert.NoError(t, validator.Validate())
}
