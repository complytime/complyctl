// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"os"
	"testing"

	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/output"
	"github.com/complytime/complyctl/pkg/provider"
)

func TestNewEvaluator_NilMapsInitialized(t *testing.T) {
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	// Should not panic when adding targets — nil maps are initialized internally.
	eval.AddTarget([]provider.AssessmentLog{
		{RequirementID: "R1", Steps: []provider.Step{{Result: provider.ResultPassed, Message: "ok"}}},
	})
	assert.Equal(t, gemara.Passed, eval.GemaraLog().Result)
}

func TestNewEvaluator_NonNilMapsPreserved(t *testing.T) {
	reqToControl := map[string]string{"req-1": "ctrl-1"}
	reqToPlan := map[string]string{"req-1": "plan-1"}
	reqToComplypackRef := map[string]string{"req-1": "registry.example.com/complypacks/opa@sha256:abc"}
	eval := output.NewEvaluator("pol", "tgt", reqToControl, reqToPlan, reqToComplypackRef)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "req-1",
			Steps:         []provider.Step{{Name: "check", Result: provider.ResultPassed, Message: "ok"}},
		},
	})
	log := eval.GemaraLog()
	require.Len(t, log.Evaluations, 1)
	assert.Equal(t, "ctrl-1", log.Evaluations[0].Name)
}

func TestGemaraLog_MetadataType(t *testing.T) {
	eval := output.NewEvaluator("test-policy", "target-1", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "REQ-1",
			Steps:         []provider.Step{{Result: provider.ResultPassed, Message: "ok"}},
		},
	})

	log := eval.GemaraLog()
	assert.Equal(t, gemara.EvaluationLogArtifact, log.Metadata.Type)
	assert.Equal(t, gemara.SchemaVersion, log.Metadata.GemaraVersion)
}

func TestGemaraLog_AggregatesResult(t *testing.T) {
	tests := []struct {
		name     string
		steps    []provider.Step
		expected gemara.Result
	}{
		{
			name:     "all passing yields Passed",
			steps:    []provider.Step{{Result: provider.ResultPassed, Message: "ok"}},
			expected: gemara.Passed,
		},
		{
			name:     "any failure yields Failed",
			steps:    []provider.Step{{Result: provider.ResultFailed, Message: "bad"}},
			expected: gemara.Failed,
		},
		{
			name:     "no steps yields NotRun",
			steps:    nil,
			expected: gemara.NotRun,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
			eval.AddTarget([]provider.AssessmentLog{
				{RequirementID: "R1", Steps: tt.steps},
			})
			assert.Equal(t, tt.expected, eval.GemaraLog().Result)
		})
	}
}

func TestGemaraLog_PopulatesTarget(t *testing.T) {
	eval := output.NewEvaluator("policy-id", "my-target", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{RequirementID: "R1", Steps: []provider.Step{{Result: provider.ResultPassed, Message: "ok"}}},
	})

	log := eval.GemaraLog()
	assert.Equal(t, "my-target", log.Target.Id)
	assert.Equal(t, "my-target", log.Target.Name)
	assert.Equal(t, gemara.Software, log.Target.Type)
}

func TestGemaraLog_AssessmentMessageUsesStepViolation(t *testing.T) {
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "REQ-1",
			Steps: []provider.Step{
				{Result: provider.ResultFailed, Message: "cert validity exceeds 397 days"},
			},
			Message: "0 of 1 targets passed",
		},
	})

	log := eval.GemaraLog()
	require.Len(t, log.Evaluations, 1)
	require.Len(t, log.Evaluations[0].AssessmentLogs, 1)
	assert.Equal(t, "cert validity exceeds 397 days", log.Evaluations[0].AssessmentLogs[0].Message)
}

func TestGemaraLog_PassingAssessmentKeepsProviderMessage(t *testing.T) {
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "REQ-1",
			Steps: []provider.Step{
				{Result: provider.ResultPassed, Message: "check ok"},
			},
			Message: "1 of 1 targets passed",
		},
	})

	log := eval.GemaraLog()
	require.Len(t, log.Evaluations, 1)
	require.Len(t, log.Evaluations[0].AssessmentLogs, 1)
	assert.Equal(t, "1 of 1 targets passed", log.Evaluations[0].AssessmentLogs[0].Message)
}

func TestGemaraLog_PlanFieldPopulated(t *testing.T) {
	reqToPlan := map[string]string{"req-1": "plan-1"}
	eval := output.NewEvaluator("pol", "tgt", nil, reqToPlan, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{RequirementID: "req-1", Steps: []provider.Step{{Result: provider.ResultPassed, Message: "ok"}}},
	})

	log := eval.GemaraLog()
	require.Len(t, log.Evaluations, 1)
	require.Len(t, log.Evaluations[0].AssessmentLogs, 1)
	al := log.Evaluations[0].AssessmentLogs[0]
	require.NotNil(t, al.Plan)
	assert.Equal(t, "pol", al.Plan.ReferenceId)
	assert.Equal(t, "plan-1", al.Plan.EntryId)
}

func TestGemaraLog_PlanFieldOmittedWhenNoMapping(t *testing.T) {
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{RequirementID: "req-1", Steps: []provider.Step{{Result: provider.ResultPassed, Message: "ok"}}},
	})

	log := eval.GemaraLog()
	require.Len(t, log.Evaluations, 1)
	require.Len(t, log.Evaluations[0].AssessmentLogs, 1)
	assert.Nil(t, log.Evaluations[0].AssessmentLogs[0].Plan)
}

func TestGemaraLog_PlanFieldOmittedForUnmappedRequirement(t *testing.T) {
	reqToPlan := map[string]string{"other-req": "plan-99"}
	eval := output.NewEvaluator("pol", "tgt", nil, reqToPlan, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{RequirementID: "req-1", Steps: []provider.Step{{Result: provider.ResultPassed, Message: "ok"}}},
	})

	log := eval.GemaraLog()
	require.Len(t, log.Evaluations, 1)
	require.Len(t, log.Evaluations[0].AssessmentLogs, 1)
	assert.Nil(t, log.Evaluations[0].AssessmentLogs[0].Plan)
}

func TestEvaluator_Write(t *testing.T) {
	outDir := t.TempDir()
	eval := output.NewEvaluator("test-policy", "target-1", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{RequirementID: "R1", Steps: []provider.Step{{Result: provider.ResultPassed, Message: "ok"}}},
	})

	path, err := eval.Write(outDir)
	require.NoError(t, err)
	assert.FileExists(t, path)
	assert.Contains(t, path, "evaluation-log-test-policy-target-1-")
}

func TestEvaluator_Write_StepIdentityWithComplypackRef(t *testing.T) {
	outDir := t.TempDir()
	reqToPlan := map[string]string{"req-1": "plan-1"}
	reqToComplypackRef := map[string]string{"req-1": "registry.example.com/complypacks/opa@sha256:abc123"}
	eval := output.NewEvaluator("pol", "tgt", nil, reqToPlan, reqToComplypackRef)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "req-1",
			Steps: []provider.Step{
				{Name: "kubernetes.run_as_nonroot", Result: provider.ResultPassed, Message: "ok"},
			},
		},
	})

	path, err := eval.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "registry.example.com/complypacks/opa@sha256:abc123#kubernetes.run_as_nonroot")
	assert.NotContains(t, content, "providerStepToGemara")
	assert.Contains(t, content, "plan:")
	assert.Contains(t, content, "entry-id: plan-1")
}

func TestEvaluator_Write_StepIdentityWithoutComplypack(t *testing.T) {
	outDir := t.TempDir()
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "req-1",
			Steps: []provider.Step{
				{Name: "my-check", Result: provider.ResultPassed, Message: "ok"},
			},
		},
	})

	path, err := eval.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "- my-check")
	assert.NotContains(t, content, "providerStepToGemara")
}

func TestEvaluator_Write_StepIdentityEmptyName(t *testing.T) {
	outDir := t.TempDir()
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "req-1",
			Steps: []provider.Step{
				{Name: "", Result: provider.ResultPassed, Message: "ok"},
			},
		},
	})

	path, err := eval.Write(outDir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.NotContains(t, content, "providerStepToGemara")
}
