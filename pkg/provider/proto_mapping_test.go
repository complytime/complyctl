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

	t.Run("fully populated evidence with source maps all fields", func(t *testing.T) {
		input := []*pluginv2.Evidence{
			{
				Id:          "ev-src-001",
				Type:        "config-file",
				Description: "TLS config",
				Payload:     []byte("tls-data"),
				CollectedAt: "2026-07-01T10:00:00Z",
				Source: &pluginv2.EvidenceMapping{
					ReferenceId: "policy-ref",
					Coordinate:  "/etc/tls.conf",
					EntryId:     "entry-42",
					Digest:      "sha256:abc123def456",
					Remarks:     "collected from production",
				},
			},
		}

		got := protoEvidenceToInternal(input)
		require.Len(t, got, 1)
		assert.Equal(t, "ev-src-001", got[0].ID)
		assert.Equal(t, "config-file", got[0].Type)
		assert.Equal(t, "TLS config", got[0].Description)
		assert.Equal(t, []byte("tls-data"), got[0].Payload)
		assert.Equal(t, "2026-07-01T10:00:00Z", got[0].CollectedAt)
		require.NotNil(t, got[0].Source)
		assert.Equal(t, "policy-ref", got[0].Source.ReferenceID)
		assert.Equal(t, "/etc/tls.conf", got[0].Source.Coordinate)
		assert.Equal(t, "entry-42", got[0].Source.EntryID)
		assert.Equal(t, "sha256:abc123def456", got[0].Source.Digest)
		assert.Equal(t, "collected from production", got[0].Source.Remarks)
	})

	t.Run("evidence without source preserves nil", func(t *testing.T) {
		input := []*pluginv2.Evidence{
			{
				Id:   "ev-no-src",
				Type: "artifact",
			},
		}

		got := protoEvidenceToInternal(input)
		require.Len(t, got, 1)
		assert.Equal(t, "ev-no-src", got[0].ID)
		assert.Nil(t, got[0].Source,
			"backward compat: nil proto source must produce nil internal source")
	})

	t.Run("evidence with partial source fields", func(t *testing.T) {
		input := []*pluginv2.Evidence{
			{
				Id: "ev-partial",
				Source: &pluginv2.EvidenceMapping{
					ReferenceId: "partial-ref",
					Coordinate:  "/app/config.yaml",
				},
			},
		}

		got := protoEvidenceToInternal(input)
		require.Len(t, got, 1)
		require.NotNil(t, got[0].Source)
		assert.Equal(t, "partial-ref", got[0].Source.ReferenceID)
		assert.Equal(t, "/app/config.yaml", got[0].Source.Coordinate)
		assert.Empty(t, got[0].Source.EntryID)
		assert.Empty(t, got[0].Source.Digest)
		assert.Empty(t, got[0].Source.Remarks)
	})
}

func TestInternalEvidenceToProto(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		got := internalEvidenceToProto(nil)
		assert.Nil(t, got)
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		got := internalEvidenceToProto([]Evidence{})
		assert.Nil(t, got)
	})

	t.Run("fully populated evidence without source", func(t *testing.T) {
		input := []Evidence{
			{
				ID:          "ev-001",
				Type:        "artifact",
				Description: "scan output log",
				Payload:     []byte(`{"result": "pass"}`),
				CollectedAt: "2026-06-30T12:00:00Z",
			},
		}

		got := internalEvidenceToProto(input)
		require.Len(t, got, 1)
		assert.Equal(t, "ev-001", got[0].GetId())
		assert.Equal(t, "artifact", got[0].GetType())
		assert.Equal(t, "scan output log", got[0].GetDescription())
		assert.Equal(t, []byte(`{"result": "pass"}`), got[0].GetPayload())
		assert.Equal(t, "2026-06-30T12:00:00Z", got[0].GetCollectedAt())
		assert.Nil(t, got[0].GetSource(),
			"nil internal source must produce nil proto source")
	})

	t.Run("fully populated evidence with source", func(t *testing.T) {
		input := []Evidence{
			{
				ID:          "ev-src-001",
				Type:        "config-file",
				Description: "TLS config",
				Payload:     []byte("tls-data"),
				CollectedAt: "2026-07-01T10:00:00Z",
				Source: &EvidenceSource{
					ReferenceID: "policy-ref",
					Coordinate:  "/etc/tls.conf",
					EntryID:     "entry-42",
					Digest:      "sha256:abc123def456",
					Remarks:     "collected from production",
				},
			},
		}

		got := internalEvidenceToProto(input)
		require.Len(t, got, 1)
		assert.Equal(t, "ev-src-001", got[0].GetId())
		assert.Equal(t, "config-file", got[0].GetType())
		assert.Equal(t, "TLS config", got[0].GetDescription())
		assert.Equal(t, []byte("tls-data"), got[0].GetPayload())
		assert.Equal(t, "2026-07-01T10:00:00Z", got[0].GetCollectedAt())

		src := got[0].GetSource()
		require.NotNil(t, src)
		assert.Equal(t, "policy-ref", src.GetReferenceId())
		assert.Equal(t, "/etc/tls.conf", src.GetCoordinate())
		assert.Equal(t, "entry-42", src.GetEntryId())
		assert.Equal(t, "sha256:abc123def456", src.GetDigest())
		assert.Equal(t, "collected from production", src.GetRemarks())
	})

	t.Run("evidence with partial source fields", func(t *testing.T) {
		input := []Evidence{
			{
				ID: "ev-partial",
				Source: &EvidenceSource{
					ReferenceID: "partial-ref",
					Coordinate:  "/app/config.yaml",
				},
			},
		}

		got := internalEvidenceToProto(input)
		require.Len(t, got, 1)

		src := got[0].GetSource()
		require.NotNil(t, src)
		assert.Equal(t, "partial-ref", src.GetReferenceId())
		assert.Equal(t, "/app/config.yaml", src.GetCoordinate())
		assert.Empty(t, src.GetEntryId())
		assert.Empty(t, src.GetDigest())
		assert.Empty(t, src.GetRemarks())
	})
}
