// SPDX-License-Identifier: Apache-2.0

package output

import (
	"testing"

	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

func TestProviderStepToGemara_SingleStep(t *testing.T) {
	step := provider.Step{
		Name:    "check-permissions",
		Result:  provider.ResultPassed,
		Message: "all permissions valid",
	}

	closure := providerStepToGemara(step, provider.ConfidenceLevelHigh)
	require.NotNil(t, closure)

	result, msg, conf := closure(nil)
	assert.Equal(t, gemara.Passed, result)
	assert.Equal(t, "all permissions valid", msg)
	assert.Equal(t, gemara.High, conf)
}

func TestProviderStepsToGemara_MultipleSteps(t *testing.T) {
	steps := []provider.Step{
		{Name: "step-1", Result: provider.ResultPassed, Message: "passed"},
		{Name: "step-2", Result: provider.ResultFailed, Message: "failed check"},
		{Name: "step-3", Result: provider.ResultSkipped, Message: "skipped"},
	}

	closures := providerStepsToGemara(steps, provider.ConfidenceLevelMedium)
	require.Len(t, closures, 3)

	r1, m1, c1 := closures[0](nil)
	assert.Equal(t, gemara.Passed, r1)
	assert.Equal(t, "passed", m1)
	assert.Equal(t, gemara.Medium, c1)

	r2, m2, c2 := closures[1](nil)
	assert.Equal(t, gemara.Failed, r2)
	assert.Equal(t, "failed check", m2)
	assert.Equal(t, gemara.Medium, c2)

	r3, m3, c3 := closures[2](nil)
	assert.Equal(t, gemara.NotApplicable, r3)
	assert.Equal(t, "skipped", m3)
	assert.Equal(t, gemara.Medium, c3)
}

func TestProviderStepsToGemara_EmptySteps(t *testing.T) {
	closures := providerStepsToGemara(nil, provider.ConfidenceLevelHigh)
	assert.Nil(t, closures)

	closures = providerStepsToGemara([]provider.Step{}, provider.ConfidenceLevelHigh)
	assert.Nil(t, closures)
}

func TestProviderStepToGemara_ResultMapping(t *testing.T) {
	tests := []struct {
		name     string
		input    provider.Result
		expected gemara.Result
	}{
		{"Passed", provider.ResultPassed, gemara.Passed},
		{"Failed", provider.ResultFailed, gemara.Failed},
		{"Skipped", provider.ResultSkipped, gemara.NotApplicable},
		{"Error", provider.ResultError, gemara.Unknown},
		{"Unrecognized", provider.Result(99), gemara.NotRun},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := provider.Step{
				Name:    "test-step",
				Result:  tt.input,
				Message: "test",
			}
			closure := providerStepToGemara(step, provider.ConfidenceLevelUndetermined)
			result, _, _ := closure(nil)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Regression test for issue #573: before this fix, providerToGemaraAssessment
// did not populate the Steps field, causing evaluation log YAML to always
// contain steps: [] even when providers sent populated step data.
func TestProviderToGemaraAssessment_StepsPopulated(t *testing.T) {
	e := NewEvaluator("test-policy", "target-1", nil, nil, nil)

	assessment := &provider.AssessmentLog{
		RequirementID: "req-1",
		Message:       "assessment complete",
		Confidence:    provider.ConfidenceLevelHigh,
		Steps: []provider.Step{
			{Name: "check-a", Result: provider.ResultPassed, Message: "ok"},
			{Name: "check-b", Result: provider.ResultPassed, Message: "ok too"},
		},
	}

	result, _ := e.providerToGemaraAssessment(assessment)

	require.NotNil(t, result)
	require.Len(t, result.Steps, 2)
	assert.Equal(t, int64(2), result.StepsExecuted)

	r1, m1, c1 := result.Steps[0](nil)
	assert.Equal(t, gemara.Passed, r1)
	assert.Equal(t, "ok", m1)
	assert.Equal(t, gemara.High, c1)

	r2, m2, c2 := result.Steps[1](nil)
	assert.Equal(t, gemara.Passed, r2)
	assert.Equal(t, "ok too", m2)
	assert.Equal(t, gemara.High, c2)
}
