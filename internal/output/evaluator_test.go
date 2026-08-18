// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"encoding/json"
	"os"
	"strings"
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
	reqToComplypackRef := map[string]string{"req-1": "registry.example.com/complypacks/opa@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"}
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
		{
			name:     "all skipped yields NotApplicable",
			steps:    []provider.Step{{Result: provider.ResultSkipped, Message: "n/a"}},
			expected: gemara.NotApplicable,
		},
		{
			name: "mixed passed and skipped yields Passed",
			steps: []provider.Step{
				{Result: provider.ResultPassed, Message: "ok"},
				{Result: provider.ResultSkipped, Message: "n/a"},
			},
			expected: gemara.Passed,
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

func TestGemaraLog_AllSkippedControlsYieldNotApplicable(t *testing.T) {
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{RequirementID: "R1", Steps: []provider.Step{{Result: provider.ResultSkipped, Message: "n/a"}}},
		{RequirementID: "R2", Steps: []provider.Step{{Result: provider.ResultSkipped, Message: "n/a"}}},
	})
	log := eval.GemaraLog()
	assert.Equal(t, gemara.NotApplicable, log.Result,
		"overall result should be NotApplicable when all controls are skipped")
	for _, ce := range log.Evaluations {
		assert.Equal(t, gemara.NotApplicable, ce.Result,
			"each control evaluation should be NotApplicable")
	}
}

func TestGemaraLog_NoAssessmentsYieldsNotApplicable(t *testing.T) {
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget(nil)
	log := eval.GemaraLog()
	assert.Equal(t, gemara.NotApplicable, log.Result)
	assert.Empty(t, log.Evaluations)
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

	path, err := eval.Write(outDir, "yaml")
	require.NoError(t, err)
	assert.FileExists(t, path)
	assert.Contains(t, path, "evaluation-log-test-policy-target-1-")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "result: Passed", "evaluation log should contain the result field")
	assert.Contains(t, content, "id: test-policy", "evaluation log should contain the policy ID in metadata")
	assert.Contains(t, content, "id: target-1", "evaluation log should contain the target ID")
	assert.Contains(t, content, "name: target-1", "evaluation log should contain the target name")
	assert.Contains(t, content, "entry-id: R1", "evaluation log should contain the requirement entry ID")
	assert.Contains(t, content, "type: EvaluationLog", "evaluation log should contain the artifact type")
	assert.Contains(t, content, "steps-executed: 1", "evaluation log should record steps executed")
}

func TestEvaluator_Write_StepIdentityWithComplypackRef(t *testing.T) {
	outDir := t.TempDir()
	reqToPlan := map[string]string{"req-1": "plan-1"}
	reqToComplypackRef := map[string]string{"req-1": "registry.example.com/complypacks/opa@sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"}
	eval := output.NewEvaluator("pol", "tgt", nil, reqToPlan, reqToComplypackRef)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "req-1",
			Steps: []provider.Step{
				{Name: "kubernetes.run_as_nonroot", Result: provider.ResultPassed, Message: "ok"},
			},
		},
	})

	path, err := eval.Write(outDir, "yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "registry.example.com/complypacks/opa@sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2#kubernetes.run_as_nonroot")
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

	path, err := eval.Write(outDir, "yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "- my-check")
	assert.NotContains(t, content, "providerStepToGemara")
}

func TestGemaraLog_EvidencePopulated(t *testing.T) {
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "req-1",
			Steps:         []provider.Step{{Result: provider.ResultFailed, Message: "bad"}},
			Evidence: []provider.Evidence{
				{
					ID:          "ev-1",
					Type:        "config-file",
					Description: "TLS config snippet",
					Payload:     []byte(`{"tls": "1.0"}`),
					CollectedAt: "2026-06-23T14:00:00Z",
				},
			},
			Recommendation: "Upgrade to TLS 1.2",
		},
	})

	log := eval.GemaraLog()
	require.Len(t, log.Evaluations, 1)
	require.Len(t, log.Evaluations[0].AssessmentLogs, 1)
	al := log.Evaluations[0].AssessmentLogs[0]
	require.Len(t, al.Evidence, 1)
	assert.Equal(t, "ev-1", al.Evidence[0].Id)
	assert.Equal(t, gemara.EvidenceType("config-file"), al.Evidence[0].Type)
	assert.Equal(t, "TLS config snippet", al.Evidence[0].Description)
	assert.Equal(t, `{"tls": "1.0"}`, al.Evidence[0].Payload)
	assert.Equal(t, gemara.Datetime("2026-06-23T14:00:00Z"), al.Evidence[0].CollectedAt)
	assert.Equal(t, "Upgrade to TLS 1.2", al.Recommendation)
}

func TestGemaraLog_EvidenceEmptyWhenNotProvided(t *testing.T) {
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{RequirementID: "req-1", Steps: []provider.Step{{Result: provider.ResultPassed, Message: "ok"}}},
	})

	log := eval.GemaraLog()
	require.Len(t, log.Evaluations, 1)
	require.Len(t, log.Evaluations[0].AssessmentLogs, 1)
	assert.Empty(t, log.Evaluations[0].AssessmentLogs[0].Evidence)
	assert.Empty(t, log.Evaluations[0].AssessmentLogs[0].Recommendation)
}

func TestEvaluator_Write_EvidenceSerialized(t *testing.T) {
	outDir := t.TempDir()
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "req-1",
			Steps:         []provider.Step{{Result: provider.ResultFailed, Message: "bad"}},
			Evidence: []provider.Evidence{
				{
					ID:          "ev-1",
					Type:        "config-file",
					Description: "TLS config",
					Payload:     []byte("sample"),
					CollectedAt: "2026-06-23T14:00:00Z",
				},
			},
			Recommendation: "Fix TLS",
		},
	})

	path, err := eval.Write(outDir, "yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "evidence:")
	assert.Contains(t, content, "id: ev-1")
	assert.Contains(t, content, "type: config-file")
	assert.Contains(t, content, "description: TLS config")
	assert.Contains(t, content, "recommendation: Fix TLS")
}

func TestGemaraLog_EvidenceBinaryPayloadBase64(t *testing.T) {
	eval := output.NewEvaluator("pol", "tgt", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "req-1",
			Steps:         []provider.Step{{Result: provider.ResultFailed, Message: "bad"}},
			Evidence: []provider.Evidence{
				{
					ID:      "ev-bin",
					Type:    "binary",
					Payload: []byte{0xff, 0xfe, 0x00, 0x01},
				},
			},
		},
	})

	log := eval.GemaraLog()
	require.Len(t, log.Evaluations, 1)
	require.Len(t, log.Evaluations[0].AssessmentLogs, 1)
	al := log.Evaluations[0].AssessmentLogs[0]
	require.Len(t, al.Evidence, 1)
	assert.Equal(t, "//4AAQ==", al.Evidence[0].Payload,
		"binary payload should be base64-encoded")
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

	path, err := eval.Write(outDir, "yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.NotContains(t, content, "providerStepToGemara")
	assert.Contains(t, content, "steps:\n    - \"\"",
		"empty step name should produce an empty-string step identity")
}

func TestEvaluator_Write_JSON(t *testing.T) {
	outDir := t.TempDir()
	eval := output.NewEvaluator("test-policy", "target-1", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{RequirementID: "R1", Steps: []provider.Step{{Result: provider.ResultPassed, Message: "ok"}}},
	})

	path, err := eval.Write(outDir, "json")
	require.NoError(t, err)
	assert.FileExists(t, path)
	assert.True(t, strings.HasSuffix(path, ".json"), "file should have .json extension")

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	// Verify the output is valid JSON.
	assert.True(t, json.Valid(data), "output should be valid JSON")

	// Verify structural content by unmarshalling with value assertions.
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Contains(t, parsed, "metadata")
	assert.Contains(t, parsed, "result")
	assert.Contains(t, parsed, "evaluations")
	assert.Contains(t, parsed, "target")

	// Verify specific values round-trip correctly.
	assert.Equal(t, "Passed", parsed["result"])
	metadata, ok := parsed["metadata"].(map[string]interface{})
	require.True(t, ok, "metadata should be a map")
	assert.Equal(t, "EvaluationLog", metadata["type"])
	assert.Equal(t, "test-policy", metadata["id"])
	evaluations, ok := parsed["evaluations"].([]interface{})
	require.True(t, ok, "evaluations should be an array")
	assert.NotEmpty(t, evaluations)
}

func TestEvaluator_Write_JSON_FieldNames(t *testing.T) {
	outDir := t.TempDir()
	reqToPlan := map[string]string{"req-1": "plan-1"}
	eval := output.NewEvaluator("pol", "tgt", nil, reqToPlan, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{
			RequirementID: "req-1",
			Steps:         []provider.Step{{Name: "check", Result: provider.ResultFailed, Message: "bad"}},
			Evidence: []provider.Evidence{
				{ID: "ev-1", Type: "config-file", Description: "snippet", Payload: []byte("data")},
			},
			Recommendation: "Fix it",
		},
	})

	path, err := eval.Write(outDir, "json")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	// Verify kebab-case field names from shadow structs.
	assert.Contains(t, content, `"assessment-logs"`)
	assert.Contains(t, content, `"steps-executed"`)

	// Verify embedded gemara types use their own kebab-case tags.
	assert.Contains(t, content, `"gemara-version"`)
	assert.Contains(t, content, `"reference-id"`)
	assert.Contains(t, content, `"entry-id"`)

	// Verify no camelCase or PascalCase field names leak through.
	assert.NotContains(t, content, `"AssessmentLogs"`)
	assert.NotContains(t, content, `"assessmentLogs"`)
	assert.NotContains(t, content, `"StepsExecuted"`)
	assert.NotContains(t, content, `"GemaraVersion"`)
}

func TestEvaluator_Write_YAML_Default(t *testing.T) {
	outDir := t.TempDir()
	eval := output.NewEvaluator("test-policy", "target-1", nil, nil, nil)
	eval.AddTarget([]provider.AssessmentLog{
		{RequirementID: "R1", Steps: []provider.Step{{Result: provider.ResultPassed, Message: "ok"}}},
	})

	path, err := eval.Write(outDir, "yaml")
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(path, ".yaml"), "file should have .yaml extension")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	// YAML output should contain YAML-style fields, not JSON braces.
	assert.Contains(t, content, "result: Passed")
	assert.Contains(t, content, "type: EvaluationLog")
	assert.NotContains(t, content, `"result":`, "YAML output should not contain JSON-style fields")
}
