// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginv2 "github.com/complytime/complyctl/api/plugin"
)

func TestProtoResultToInternal(t *testing.T) {
	tests := []struct {
		name     string
		input    pluginv2.Result
		expected Result
	}{
		{
			name:     "RESULT_PASSED maps to ResultPassed",
			input:    pluginv2.Result_RESULT_PASSED,
			expected: ResultPassed,
		},
		{
			name:     "RESULT_FAILED maps to ResultFailed",
			input:    pluginv2.Result_RESULT_FAILED,
			expected: ResultFailed,
		},
		{
			name:     "RESULT_ERROR maps to ResultError",
			input:    pluginv2.Result_RESULT_ERROR,
			expected: ResultError,
		},
		{
			name:     "RESULT_SKIPPED maps to ResultSkipped",
			input:    pluginv2.Result_RESULT_SKIPPED,
			expected: ResultSkipped,
		},
		{
			name:     "RESULT_UNSPECIFIED maps to ResultUnspecified",
			input:    pluginv2.Result_RESULT_UNSPECIFIED,
			expected: ResultUnspecified,
		},
		{
			name:     "unknown future value falls back to ResultUnspecified",
			input:    pluginv2.Result(99),
			expected: ResultUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := protoResultToInternal(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestProtoConfidenceToInternal(t *testing.T) {
	tests := []struct {
		name     string
		input    pluginv2.ConfidenceLevel
		expected ConfidenceLevel
	}{
		{
			name:     "NOT_SET maps to ConfidenceLevelNotSet",
			input:    pluginv2.ConfidenceLevel_CONFIDENCE_LEVEL_NOT_SET,
			expected: ConfidenceLevelNotSet,
		},
		{
			name:     "UNDETERMINED maps to ConfidenceLevelUndetermined",
			input:    pluginv2.ConfidenceLevel_CONFIDENCE_LEVEL_UNDETERMINED,
			expected: ConfidenceLevelUndetermined,
		},
		{
			name:     "LOW maps to ConfidenceLevelLow",
			input:    pluginv2.ConfidenceLevel_CONFIDENCE_LEVEL_LOW,
			expected: ConfidenceLevelLow,
		},
		{
			name:     "MEDIUM maps to ConfidenceLevelMedium",
			input:    pluginv2.ConfidenceLevel_CONFIDENCE_LEVEL_MEDIUM,
			expected: ConfidenceLevelMedium,
		},
		{
			name:     "HIGH maps to ConfidenceLevelHigh",
			input:    pluginv2.ConfidenceLevel_CONFIDENCE_LEVEL_HIGH,
			expected: ConfidenceLevelHigh,
		},
		{
			name:     "unknown future value falls back to ConfidenceLevelNotSet",
			input:    pluginv2.ConfidenceLevel(99),
			expected: ConfidenceLevelNotSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := protoConfidenceToInternal(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestProtoEvidenceToInternal(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		got := protoEvidenceToInternal(nil)
		assert.Nil(t, got)
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		got := protoEvidenceToInternal([]*pluginv2.Evidence{})
		assert.Nil(t, got)
	})

	t.Run("fully populated evidence maps all fields", func(t *testing.T) {
		input := []*pluginv2.Evidence{
			{
				Id:          "ev-001",
				Type:        "artifact",
				Description: "scan output log",
				Payload:     []byte(`{"result": "pass"}`),
				CollectedAt: "2026-06-30T12:00:00Z",
			},
		}

		got := protoEvidenceToInternal(input)
		require.Len(t, got, 1)
		assert.Equal(t, "ev-001", got[0].ID)
		assert.Equal(t, "artifact", got[0].Type)
		assert.Equal(t, "scan output log", got[0].Description)
		assert.Equal(t, []byte(`{"result": "pass"}`), got[0].Payload)
		assert.Equal(t, "2026-06-30T12:00:00Z", got[0].CollectedAt)
	})

	t.Run("multiple evidence items preserve order", func(t *testing.T) {
		input := []*pluginv2.Evidence{
			{Id: "ev-001", Type: "first"},
			{Id: "ev-002", Type: "second"},
		}

		got := protoEvidenceToInternal(input)
		require.Len(t, got, 2)
		assert.Equal(t, "ev-001", got[0].ID)
		assert.Equal(t, "ev-002", got[1].ID)
	})
}
