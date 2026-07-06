// SPDX-License-Identifier: Apache-2.0

package output

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/complytime/complyctl/internal/complytime"
)

func TestFormatExecutionPlan_SingleRow(t *testing.T) {
	rows := []ExecutionPlanRow{
		{
			TargetID:         "my-app",
			ProviderID:       "ampel",
			RequirementCount: 5,
			Status:           "healthy",
		},
	}

	result := FormatExecutionPlan("test-policy", rows)

	assert.Contains(t, result, "Execution Plan: test-policy")
	assert.Contains(t, result, "Target: my-app")
	assert.Contains(t, result, "Provider: ampel")
	assert.Contains(t, result, complytime.StatusPassed)
	assert.Contains(t, result, "healthy")
	assert.Contains(t, result, "Requirements: 5")
	assert.Contains(t, result, "Generation completed.")
}

func TestFormatExecutionPlan_MultipleRows(t *testing.T) {
	rows := []ExecutionPlanRow{
		{
			TargetID:         "app-1",
			ProviderID:       "ampel",
			RequirementCount: 3,
			Status:           "healthy",
		},
		{
			TargetID:         "app-2",
			ProviderID:       "opa",
			RequirementCount: 7,
			Status:           "unavailable",
		},
	}

	result := FormatExecutionPlan("multi-policy", rows)

	assert.Contains(t, result, "Execution Plan: multi-policy")
	assert.Contains(t, result, "Target: app-1")
	assert.Contains(t, result, "Provider: ampel")
	assert.Contains(t, result, "Requirements: 3")
	assert.Contains(t, result, "Target: app-2")
	assert.Contains(t, result, "Provider: opa")
	assert.Contains(t, result, "Requirements: 7")
	assert.Contains(t, result, complytime.StatusPassed, "healthy provider should get passed emoji")
	assert.Contains(t, result, complytime.StatusFailed, "unavailable provider should get failed emoji")
}

func TestFormatExecutionPlan_EmptyRows(t *testing.T) {
	result := FormatExecutionPlan("empty-policy", nil)

	assert.Contains(t, result, "Execution Plan: empty-policy")
	assert.Contains(t, result, "Generation completed.")
	assert.NotContains(t, result, "Target:")
	assert.NotContains(t, result, "Provider:")
}

func TestFormatExecutionPlan_UnhealthyStatus(t *testing.T) {
	rows := []ExecutionPlanRow{
		{
			TargetID:         "target-1",
			ProviderID:       "broken-provider",
			RequirementCount: 2,
			Status:           "error",
		},
	}

	result := FormatExecutionPlan("pol", rows)

	assert.Contains(t, result, complytime.StatusFailed, "non-healthy status should get failed emoji")
	assert.Contains(t, result, "error")
}

func TestFormatPreScanSummary_WithProvidersAndTargets(t *testing.T) {
	result := FormatPreScanSummary(12, []string{"ampel", "opa"}, []string{"app-1", "app-2"})

	assert.Contains(t, result, "Scanning 12 requirements")
	assert.Contains(t, result, "ampel, opa")
	assert.Contains(t, result, "app-1, app-2")
	assert.Contains(t, result, "...")
}

func TestFormatPreScanSummary_SingleProviderAndTarget(t *testing.T) {
	result := FormatPreScanSummary(5, []string{"ampel"}, []string{"my-target"})

	assert.Equal(t, "Scanning 5 requirements via ampel for target(s): my-target...", result)
}

func TestFormatPreScanSummary_EmptyInputs(t *testing.T) {
	result := FormatPreScanSummary(0, nil, nil)

	assert.Equal(t, "Scanning 0 requirements via  for target(s): ...", result)
}
