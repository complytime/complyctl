// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/output"
)

func mockGemaraEvalLogWithFindings() *gemara.EvaluationLog {
	return &gemara.EvaluationLog{
		Metadata: gemara.Metadata{
			Id:            "test-policy",
			GemaraVersion: gemara.SchemaVersion,
			Description:   "Test evaluation log with findings",
			Author: gemara.Actor{
				Id:   "complytime",
				Name: "complytime",
				Type: gemara.Software,
			},
		},
		Result: gemara.Failed,
		Target: gemara.Resource{Id: "my-target", Name: "my-target", Type: gemara.Software},
		Evaluations: []*gemara.ControlEvaluation{
			{
				Name:    "ctrl-1",
				Result:  gemara.Passed,
				Message: "ok",
				Control: gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "ctrl-1"},
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement:     gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "req-1"},
						Result:          gemara.Passed,
						Message:         "check passed",
						Applicability:   []string{"default"},
						Start:           "2026-01-01T00:00:00Z",
						StepsExecuted:   1,
						ConfidenceLevel: gemara.High,
					},
				},
			},
			{
				Name:    "ctrl-2",
				Result:  gemara.Failed,
				Message: "violation found",
				Control: gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "ctrl-2"},
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement:     gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "req-2"},
						Result:          gemara.Failed,
						Message:         "cert validity exceeds 397 days",
						Applicability:   []string{"default"},
						Start:           "2026-01-01T00:00:00Z",
						StepsExecuted:   1,
						ConfidenceLevel: gemara.High,
						Recommendation:  "Reduce certificate validity to 397 days or fewer",
						Evidence: []gemara.Evidence{
							{
								Id:          "ev-1",
								Type:        "config-file",
								Description: "TLS certificate config",
								CollectedAt: "2026-01-01T00:00:00Z",
							},
						},
					},
				},
			},
			{
				Name:    "ctrl-3",
				Result:  gemara.NotApplicable,
				Message: "tailored",
				Control: gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "ctrl-3"},
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement:     gemara.EntryMapping{ReferenceId: "test-policy", EntryId: "req-3"},
						Result:          gemara.NotApplicable,
						Message:         "tailored out",
						Applicability:   []string{"default"},
						Start:           "2026-01-01T00:00:00Z",
						StepsExecuted:   0,
						ConfidenceLevel: gemara.Undetermined,
					},
				},
			},
		},
	}
}

func TestMarkdown_Write(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()
	md := output.NewMarkdown("test-policy", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)
	assert.FileExists(t, path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "# Compliance Scan Report: test-policy")
	assert.Contains(t, content, "| Policy | test-policy |")
	assert.Contains(t, content, "| Target | - |")
	assert.Contains(t, content, "| Tool |")
	assert.Contains(t, content, "| Result |")
	assert.Contains(t, content, "| Date |")
	assert.Contains(t, content, "pass rate")
	assert.Contains(t, content, "applicable")
	assert.Contains(t, content, "## Controls")
	assert.Contains(t, content, "**ctrl-1**")
	assert.Contains(t, content, "**ctrl-2**")
	assert.Contains(t, content, "req-1")
	assert.Contains(t, content, "req-2")
	assert.Contains(t, content, "## Findings")
}

func TestMarkdown_TargetIdFallback(t *testing.T) {
	outDir := t.TempDir()
	log := &gemara.EvaluationLog{
		Metadata: gemara.Metadata{Id: "pol"},
		Result:   gemara.Passed,
		Target:   gemara.Resource{Id: "my-target", Name: "my-target"},
		Evaluations: []*gemara.ControlEvaluation{
			{
				Name:   "ctrl-1",
				Result: gemara.Passed,
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement: gemara.EntryMapping{EntryId: "req-1"},
						Result:      gemara.Passed,
						Message:     "ok",
					},
				},
			},
		},
	}
	md := output.NewMarkdown("pol", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "| Target | my-target |")
	assert.NotContains(t, content, "| Target | - |")
}

func TestMarkdown_SkippedDueToTailoring(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLogWithFindings()
	md := output.NewMarkdown("test-policy", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.True(t, strings.Contains(content, "Not Applicable"),
		"expected 'Not Applicable' in findings for tailored assessments")
}

func TestMarkdown_OutputFileNaming(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()
	md := output.NewMarkdown("test-policy", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	filename := filepath.Base(path)
	assert.Contains(t, filename, "report-test-policy-")
	assert.Contains(t, filename, ".md")
}

func TestMarkdown_EmbedEvaluationLog(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()

	evalPath := filepath.Join(outDir, "eval.yaml")
	require.NoError(t, os.WriteFile(evalPath, []byte("test: true\n"), 0600))

	md := output.NewMarkdown("test-policy", log)
	md.SetEmbedEvaluationLog(evalPath)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "<details>")
	assert.Contains(t, content, "<summary>Evaluation Log</summary>")
	assert.Contains(t, content, "test: true")
}

func TestMarkdown_FindingsWithRecommendationAndEvidence(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLogWithFindings()
	md := output.NewMarkdown("test-policy", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "#### req-2 -- Failed")
	assert.Contains(t, content, "**Control**: ctrl-2")
	assert.Contains(t, content, "cert validity exceeds 397 days")
	assert.Contains(t, content, "**Recommendation**: Reduce certificate validity")
	assert.Contains(t, content, "Evidence (1 items)")
	assert.Contains(t, content, "TLS certificate config")
	assert.Contains(t, content, "config-file")
	assert.Contains(t, content, "collected: 2026-01-01T00:00:00Z")
}

func TestMarkdown_NoFindingsWhenAllPassed(t *testing.T) {
	outDir := t.TempDir()
	log := &gemara.EvaluationLog{
		Metadata: gemara.Metadata{Id: "pol"},
		Result:   gemara.Passed,
		Evaluations: []*gemara.ControlEvaluation{
			{
				Name:   "ctrl-1",
				Result: gemara.Passed,
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement: gemara.EntryMapping{EntryId: "req-1"},
						Result:      gemara.Passed,
						Message:     "ok",
					},
				},
			},
		},
	}
	md := output.NewMarkdown("pol", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "No findings.")
}

func TestMarkdown_FindingsSortOrder(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLogWithFindings()
	md := output.NewMarkdown("test-policy", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	failedIdx := strings.Index(content, "### Failed")
	notApplicableIdx := strings.Index(content, "### Not Applicable")
	require.Greater(t, failedIdx, 0, "expected Failed heading in findings")
	require.Greater(t, notApplicableIdx, 0, "expected Not Applicable heading in findings")
	assert.Less(t, failedIdx, notApplicableIdx,
		"Failed findings should appear before Not Applicable")
}

func TestMarkdown_PassRate(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLogWithFindings()
	md := output.NewMarkdown("test-policy", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "50% pass rate (1/2 applicable)")
}

func TestMarkdown_EmbedEvaluationLog_MissingFile(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()

	md := output.NewMarkdown("test-policy", log)
	md.SetEmbedEvaluationLog(filepath.Join(outDir, "nonexistent.yaml"))

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.NotContains(t, content, "<summary>Evaluation Log</summary>")
}

func TestMarkdown_EvidenceEmptyDescriptionOmitted(t *testing.T) {
	outDir := t.TempDir()
	log := &gemara.EvaluationLog{
		Metadata: gemara.Metadata{Id: "pol"},
		Result:   gemara.Failed,
		Evaluations: []*gemara.ControlEvaluation{
			{
				Name:   "ctrl-1",
				Result: gemara.Failed,
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement: gemara.EntryMapping{EntryId: "req-1"},
						Result:      gemara.Failed,
						Message:     "bad",
						Evidence: []gemara.Evidence{
							{Id: "ev-1", Description: "visible"},
							{Id: "ev-2", Description: ""},
						},
					},
				},
			},
		},
	}
	md := output.NewMarkdown("pol", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "Evidence (2 items)")
	assert.Contains(t, content, "- visible")
	assert.Contains(t, content, "- ev-2",
		"evidence with empty description should fall back to ID")
}

func TestMarkdown_ZeroEvaluations(t *testing.T) {
	outDir := t.TempDir()
	log := &gemara.EvaluationLog{
		Metadata:    gemara.Metadata{Id: "pol"},
		Result:      gemara.NotRun,
		Evaluations: nil,
	}
	md := output.NewMarkdown("pol", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "0% pass rate (0/0 applicable)")
	assert.Contains(t, content, "No findings.")
}

func TestMarkdown_ConfidenceLevelShown(t *testing.T) {
	outDir := t.TempDir()
	log := &gemara.EvaluationLog{
		Metadata: gemara.Metadata{Id: "pol"},
		Result:   gemara.Failed,
		Evaluations: []*gemara.ControlEvaluation{
			{
				Name:   "ctrl-1",
				Result: gemara.Failed,
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement:     gemara.EntryMapping{EntryId: "req-low"},
						Result:          gemara.Failed,
						Message:         "failed with low confidence",
						ConfidenceLevel: gemara.Low,
					},
					{
						Requirement:     gemara.EntryMapping{EntryId: "req-high"},
						Result:          gemara.Failed,
						Message:         "failed with high confidence",
						ConfidenceLevel: gemara.High,
					},
					{
						Requirement:     gemara.EntryMapping{EntryId: "req-undetermined"},
						Result:          gemara.Failed,
						Message:         "failed with undetermined confidence",
						ConfidenceLevel: gemara.Undetermined,
					},
				},
			},
		},
	}
	md := output.NewMarkdown("pol", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "**Confidence**: Low")
	assert.Contains(t, content, "**Confidence**: High")
	assert.NotContains(t, content, "**Confidence**: Undetermined")
}

func TestMarkdown_ToolAttribution(t *testing.T) {
	outDir := t.TempDir()
	log := &gemara.EvaluationLog{
		Metadata: gemara.Metadata{
			Id: "pol",
			Author: gemara.Actor{
				Name:    "complytime",
				Version: "v1.0.0",
			},
		},
		Result: gemara.Passed,
		Evaluations: []*gemara.ControlEvaluation{
			{
				Name:   "ctrl-1",
				Result: gemara.Passed,
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement: gemara.EntryMapping{EntryId: "req-1"},
						Result:      gemara.Passed,
						Message:     "ok",
					},
				},
			},
		},
	}
	md := output.NewMarkdown("pol", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "| Tool | complytime v1.0.0 |")
}

func TestMarkdown_ControlsTableShowsAllControls(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLogWithFindings()
	md := output.NewMarkdown("test-policy", log)

	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "**ctrl-1**")
	assert.Contains(t, content, "**ctrl-2**")
	assert.Contains(t, content, "**ctrl-3**")
	assert.Contains(t, content, "&nbsp;&nbsp;req-1")
	assert.Contains(t, content, "&nbsp;&nbsp;req-2")
	assert.Contains(t, content, "&nbsp;&nbsp;req-3")
}

func TestMarkdown_NeedsReviewInFindings(t *testing.T) {
	outDir := t.TempDir()
	evalLog := &gemara.EvaluationLog{
		Metadata: gemara.Metadata{
			Id:            "nr-policy",
			GemaraVersion: gemara.SchemaVersion,
			Author: gemara.Actor{
				Name: "complytime",
			},
		},
		Result: gemara.NeedsReview,
		Target: gemara.Resource{Id: "target-nr", Name: "target-nr", Type: gemara.Software},
		Evaluations: []*gemara.ControlEvaluation{
			{
				Name:    "ctrl-review",
				Result:  gemara.NeedsReview,
				Message: "human review required",
				Control: gemara.EntryMapping{ReferenceId: "nr-policy", EntryId: "ctrl-review"},
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement:     gemara.EntryMapping{ReferenceId: "nr-policy", EntryId: "req-review"},
						Result:          gemara.NeedsReview,
						Message:         "automated check inconclusive",
						ConfidenceLevel: gemara.Medium,
						Recommendation:  "Manual verification needed",
					},
				},
			},
			{
				Name:    "ctrl-pass",
				Result:  gemara.Passed,
				Message: "ok",
				Control: gemara.EntryMapping{ReferenceId: "nr-policy", EntryId: "ctrl-pass"},
				AssessmentLogs: []*gemara.AssessmentLog{
					{
						Requirement: gemara.EntryMapping{ReferenceId: "nr-policy", EntryId: "req-pass"},
						Result:      gemara.Passed,
						Message:     "check passed",
					},
				},
			},
		},
	}

	md := output.NewMarkdown("nr-policy", evalLog)
	path, err := md.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	// Summary counts table should include Needs Review column with value 1
	assert.Contains(t, content, "Needs Review")
	assert.Contains(t, content, "| 1 | 0 | 1 |", "should show 1 passed, 0 failed, 1 needs review")

	// Controls table should show NeedsReview result
	assert.Contains(t, content, "**ctrl-review**")
	assert.Contains(t, content, "Needs Review")

	// Findings section should include the NeedsReview finding
	assert.Contains(t, content, "### Needs Review")
	assert.Contains(t, content, "#### req-review -- Needs Review")
	assert.Contains(t, content, "**Control**: ctrl-review")
	assert.Contains(t, content, "automated check inconclusive")
	assert.Contains(t, content, "**Confidence**: Medium")
	assert.Contains(t, content, "**Recommendation**: Manual verification needed")

	// Passed controls should NOT appear in findings
	assert.NotContains(t, content, "#### req-pass")
}
