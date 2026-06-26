// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"context"

	"github.com/complytime/complyctl/pkg/provider"
)

// Compile-time check: mockClient must implement Provider
var _ provider.Provider = (*mockClient)(nil)

// mockClient provides an in-memory mock provider.Provider for testing only.
// Like real providers, it stores match IDs during Generate and uses
// them during Scan (R47).
type mockClient struct {
	matchIDs              []string
	complypackContentPath string
}

// newMockClient creates a new mockClient instance for tests.
func newMockClient() *mockClient {
	return &mockClient{}
}

func (m *mockClient) Describe(_ context.Context, _ *provider.DescribeRequest) (*provider.DescribeResponse, error) {
	return &provider.DescribeResponse{
		Healthy: true,
		Version: "mock-v1",
	}, nil
}

func (m *mockClient) Generate(_ context.Context, req *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	m.matchIDs = make([]string, 0, len(req.Configuration))
	for _, cfg := range req.Configuration {
		m.matchIDs = append(m.matchIDs, cfg.MatchID())
	}
	m.complypackContentPath = req.ComplypackContentPath
	return &provider.GenerateResponse{Success: true}, nil
}

func (m *mockClient) Scan(_ context.Context, _ *provider.ScanRequest) (*provider.ScanResponse, error) {
	assessments := make([]provider.AssessmentLog, 0, len(m.matchIDs))
	for _, matchID := range m.matchIDs {
		assessments = append(assessments, provider.AssessmentLog{
			RequirementID: matchID,
			Steps: []provider.Step{
				{Name: "mock-check", Result: provider.ResultPassed, Message: "mock check passed"},
			},
			Message:    "mock passed",
			Confidence: provider.ConfidenceLevelHigh,
		})
	}
	return &provider.ScanResponse{Assessments: assessments}, nil
}
