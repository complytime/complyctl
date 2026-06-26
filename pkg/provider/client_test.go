// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

func TestAssessmentConfiguration_MatchID(t *testing.T) {
	assert.Equal(t, "plan-1", provider.AssessmentConfiguration{
		PlanID:        "plan-1",
		RequirementID: "req-1",
	}.MatchID())
	assert.Equal(t, "req-1", provider.AssessmentConfiguration{
		RequirementID: "req-1",
	}.MatchID())
}

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
			{PlanID: "plan-2", RequirementID: "req-2"},
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

	expectedIDs := []string{"plan-1", "plan-2"}
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

func TestClient_Generate_ComplypackPathPopulated(t *testing.T) {
	mock := newMockClient()

	complypackPath := "/home/user/.complytime/complypacks/io.complytime.opa/1.0.0/content.tar.gz"
	req := &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "plan-1", RequirementID: "req-1", Parameters: map[string]string{"key": "value"}},
		},
		ComplypackContentPath: complypackPath,
	}

	resp, err := mock.Generate(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Empty(t, resp.ErrorMessage)
	assert.Equal(t, complypackPath, mock.complypackContentPath,
		"ComplypackContentPath should propagate through GenerateRequest to the provider")
}

func TestClient_Generate_ComplypackPathEmpty(t *testing.T) {
	mock := newMockClient()

	req := &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "plan-1", RequirementID: "req-1", Parameters: map[string]string{"key": "value"}},
		},
		// ComplypackContentPath intentionally omitted — backward compatibility
	}

	resp, err := mock.Generate(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Empty(t, resp.ErrorMessage)
	assert.Empty(t, mock.complypackContentPath,
		"ComplypackContentPath should default to empty string for backward compatibility")
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
	assert.Equal(t, "plan-1", resp.Assessments[0].RequirementID)
	assert.Equal(t, "mock-check", resp.Assessments[0].Steps[0].Name)
}
