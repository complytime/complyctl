// SPDX-License-Identifier: Apache-2.0

// test-provider is a standalone gRPC provider for E2E testing.
// It implements the provider.Provider interface with predefined responses
// and registers via provider.Serve().
//
// Build:
//
//	go build -o complyctl-provider-test ./cmd/test-provider
//
// The resulting binary follows the complyctl-provider-* naming convention
// so the provider discovery system finds it automatically when placed in
// the providers directory (~/.complytime/providers/ or a test-specific path).
//
// This binary is NOT referenced by any production code.
package main

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/pkg/provider"
)

// Compile-time checks
var (
	_ provider.Provider = (*testEvaluator)(nil)
)

// testEvaluator returns predefined responses for all RPCs.
type testEvaluator struct {
	matchIDs []string
}

func (t *testEvaluator) Describe(_ context.Context, _ *provider.DescribeRequest) (*provider.DescribeResponse, error) {
	return &provider.DescribeResponse{
		Healthy:                 true,
		Version:                 "test-v0.1.0",
		RequiredGlobalVariables: []string{"workspace"},
	}, nil
}

func (t *testEvaluator) Generate(_ context.Context, req *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	t.matchIDs = make([]string, 0, len(req.Configuration))
	for _, cfg := range req.Configuration {
		t.matchIDs = append(t.matchIDs, cfg.MatchID())
	}

	// Log complypack content path receipt for E2E test observability.
	logComplypackPath(req.ComplypackContentPath)

	return &provider.GenerateResponse{Success: true}, nil
}

// logComplypackPath logs the complypack content path when non-empty.
// Extracted to keep Generate's cyclomatic complexity low.
func logComplypackPath(path string) {
	if path != "" {
		hclog.Default().Info("complypack content path received",
			"complypack_content_path", path)
	}
}

func (t *testEvaluator) Scan(_ context.Context, req *provider.ScanRequest) (*provider.ScanResponse, error) {
	assessments := make([]provider.AssessmentLog, 0, len(t.matchIDs))
	for _, matchID := range t.matchIDs {
		assessments = append(assessments, provider.AssessmentLog{
			PlanID: matchID,
			Steps: []provider.Step{
				{
					Name:    "test-check",
					Result:  provider.ResultPassed,
					Message: "predefined pass from test-provider",
				},
			},
			Message:    "all checks passed",
			Confidence: provider.ConfidenceLevelHigh,
			Evidence: []provider.Evidence{
				{
					ID:          "ev-" + matchID,
					Type:        "log",
					Description: "test evidence for " + matchID,
					Payload:     []byte("sample payload"),
					CollectedAt: time.Now().Format(time.RFC3339),
				},
			},
			Recommendation: "No action needed",
		})
	}
	return &provider.ScanResponse{Assessments: assessments}, nil
}

func main() {
	provider.Serve(&testEvaluator{})
}
