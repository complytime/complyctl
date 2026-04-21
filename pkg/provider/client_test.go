// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

func TestMockClient_Describe(t *testing.T) {
	mock := newMockClient()

	resp, err := mock.Describe(context.Background(), &provider.DescribeRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Healthy)
	assert.Equal(t, "mock-v1", resp.Version)
	assert.Empty(t, resp.ErrorMessage)
}

func TestMockClient_Generate(t *testing.T) {
	mock := newMockClient()

	req := &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "plan-1", RequirementID: "req-1", Parameters: map[string]string{"key": "value"}},
		},
	}

	resp, err := mock.Generate(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Empty(t, resp.ErrorMessage)
}

func TestMockClient_Scan(t *testing.T) {
	mock := newMockClient()

	genReq := &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "plan-1", RequirementID: "req-1"},
			{PlanID: "plan-1", RequirementID: "req-2"},
		},
	}
	_, err := mock.Generate(context.Background(), genReq)
	require.NoError(t, err)

	scanReq := &provider.ScanRequest{
		Targets: []provider.Target{{TargetID: "target-1", Variables: map[string]string{}}},
	}

	resp, err := mock.Scan(context.Background(), scanReq)
	require.NoError(t, err)
	require.Len(t, resp.Assessments, 2)

	expectedIDs := []string{"req-1", "req-2"}
	for i, a := range resp.Assessments {
		assert.Equal(t, expectedIDs[i], a.RequirementID)
		assert.Equal(t, "mock passed", a.Message)
		assert.Equal(t, provider.ConfidenceLevelHigh, a.Confidence)
		require.Len(t, a.Steps, 1)
		assert.Equal(t, provider.ResultPassed, a.Steps[0].Result)
	}
}

func TestMockClient_Scan_NoGenerate(t *testing.T) {
	mock := newMockClient()

	scanReq := &provider.ScanRequest{
		Targets: []provider.Target{{TargetID: "t1"}},
	}

	resp, err := mock.Scan(context.Background(), scanReq)
	require.NoError(t, err)
	assert.Empty(t, resp.Assessments)
}

func TestMockClient_Scan_ResponseMapping(t *testing.T) {
	mock := newMockClient()

	genReq := &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "plan-1", RequirementID: "single-req"},
		},
	}
	_, err := mock.Generate(context.Background(), genReq)
	require.NoError(t, err)

	scanReq := &provider.ScanRequest{
		Targets: []provider.Target{{TargetID: "t1"}},
	}

	resp, err := mock.Scan(context.Background(), scanReq)
	require.NoError(t, err)
	require.Len(t, resp.Assessments, 1)
	assert.Equal(t, "single-req", resp.Assessments[0].RequirementID)
	assert.Equal(t, "mock-check", resp.Assessments[0].Steps[0].Name)
}
