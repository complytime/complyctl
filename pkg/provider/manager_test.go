// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

func TestManager_GetProvider(t *testing.T) {
	mgr, err := provider.NewManager("/nonexistent", nil)
	require.NoError(t, err)

	mgr.RegisterProviderForTest("mock", &provider.LoadedProvider{
		Info: provider.ProviderInfo{
			ProviderID:     "mock",
			EvaluatorID:    "mock",
			ExecutablePath: "(test)",
		},
	})

	p, err := mgr.GetProvider("mock")
	require.NoError(t, err)
	assert.Equal(t, "mock", p.Info.ProviderID)
}

func TestManager_GetProvider_UnknownID(t *testing.T) {
	mgr, err := provider.NewManager("/nonexistent", nil)
	require.NoError(t, err)

	mgr.RegisterProviderForTest("mock", &provider.LoadedProvider{
		Info: provider.ProviderInfo{
			ProviderID:     "mock",
			EvaluatorID:    "mock",
			ExecutablePath: "(test)",
		},
	})

	_, err = mgr.GetProvider("unknown-evaluator")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider not found for evaluator ID")
}

func TestManager_ListProviders(t *testing.T) {
	mgr, err := provider.NewManager("/nonexistent", nil)
	require.NoError(t, err)

	mgr.RegisterProviderForTest("mock", &provider.LoadedProvider{
		Info: provider.ProviderInfo{
			ProviderID:     "mock",
			EvaluatorID:    "mock",
			ExecutablePath: "(test)",
		},
	})

	providers := mgr.ListProviders()
	assert.Len(t, providers, 1)
	assert.Equal(t, "mock", providers[0].Info.ProviderID)
}

func TestManager_EmptyProviderDir(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := provider.NewManager(tmpDir, nil)
	require.NoError(t, err)
	require.NoError(t, mgr.LoadProviders())

	providers := mgr.ListProviders()
	assert.Empty(t, providers)
}

type failingMockClient struct {
	mockClient
	failGenerate bool
	failScan     bool
}

func (f *failingMockClient) Generate(_ context.Context, _ *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	if f.failGenerate {
		return nil, fmt.Errorf("generate RPC failed")
	}
	return &provider.GenerateResponse{Success: true}, nil
}

func (f *failingMockClient) Scan(_ context.Context, _ *provider.ScanRequest) (*provider.ScanResponse, error) {
	if f.failScan {
		return nil, fmt.Errorf("scan RPC failed")
	}
	return &provider.ScanResponse{Assessments: []provider.AssessmentLog{{RequirementID: "test-req"}}}, nil
}

func TestManager_RouteGenerate_SpecificProvider(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	mock := newMockClient()
	lp := provider.NewMockLoadedProvider("test-provider", "test-eval", mock)
	mgr.RegisterProviderForTest("test-eval", lp)

	err = mgr.RouteGenerate(context.Background(), "test-eval", nil, nil, []provider.AssessmentConfiguration{
		{RequirementID: "req-1"},
	}, "")
	require.NoError(t, err)
}

func TestManager_RouteGenerate_UnknownEvaluator(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	err = mgr.RouteGenerate(context.Background(), "nonexistent", nil, nil, nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no provider registered")
}

func TestManager_RouteGenerate_Broadcast(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	mock := newMockClient()
	lp := provider.NewMockLoadedProvider("test-provider", "test-eval", mock)
	mgr.RegisterProviderForTest("test-eval", lp)

	err = mgr.RouteGenerate(context.Background(), "", nil, nil, nil, "")
	require.NoError(t, err)
}

func TestManager_RouteGenerate_ProviderReturnsFailure(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	fm := &failingMockClient{failGenerate: true}
	lp := provider.NewMockLoadedProvider("fail-provider", "fail-eval", fm)
	mgr.RegisterProviderForTest("fail-eval", lp)

	err = mgr.RouteGenerate(context.Background(), "fail-eval", nil, nil, nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generate RPC failed")
}

func TestManager_RouteGenerate_WithComplypackPath(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	mock := newMockClient()
	lp := provider.NewMockLoadedProvider("test-provider", "test-eval", mock)
	mgr.RegisterProviderForTest("test-eval", lp)

	complypackPath := "/cache/complypacks/test-eval/1.0.0/content.tar.gz"
	err = mgr.RouteGenerate(context.Background(), "test-eval", nil, nil, []provider.AssessmentConfiguration{
		{RequirementID: "req-1"},
	}, complypackPath)
	require.NoError(t, err)

	// Verify the mock client received the complypack content path.
	assert.Equal(t, complypackPath, mock.complypackContentPath,
		"ComplypackContentPath should be forwarded to the provider")
}

func TestManager_RouteScan_SpecificProvider(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	mock := &failingMockClient{}
	lp := provider.NewMockLoadedProvider("test-provider", "test-eval", mock)
	mgr.RegisterProviderForTest("test-eval", lp)

	results, err := mgr.RouteScan(context.Background(), "test-eval", []provider.Target{{TargetID: "t1"}})
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestManager_RouteScan_UnknownEvaluator(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	_, err = mgr.RouteScan(context.Background(), "nonexistent", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no provider registered")
}

func TestManager_RouteScan_Broadcast(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	mock := &failingMockClient{}
	lp := provider.NewMockLoadedProvider("test-provider", "test-eval", mock)
	mgr.RegisterProviderForTest("test-eval", lp)

	results, err := mgr.RouteScan(context.Background(), "", []provider.Target{{TargetID: "t1"}})
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestManager_RouteScan_ProviderError_ReturnsErrorAssessment(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	fm := &failingMockClient{failScan: true}
	lp := provider.NewMockLoadedProvider("fail-provider", "fail-eval", fm)
	mgr.RegisterProviderForTest("fail-eval", lp)

	results, err := mgr.RouteScan(context.Background(), "fail-eval", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Message, "scan RPC failed")
}

func TestManager_RouteScanResult_ReturnsErrors(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	fm := &failingMockClient{failScan: true}
	lp := provider.NewMockLoadedProvider("fail-provider", "fail-eval", fm)
	mgr.RegisterProviderForTest("fail-eval", lp)

	result, err := mgr.RouteScanResult(context.Background(), "fail-eval", nil)
	require.NoError(t, err)
	assert.True(t, result.HasErrors())
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "scan RPC failed")
	assert.Empty(t, result.Assessments, "RPC failures must not inject synthetic assessments (D3)")
}

type errorEmbeddingMockClient struct {
	mockClient
}

func (e *errorEmbeddingMockClient) Scan(_ context.Context, _ *provider.ScanRequest) (*provider.ScanResponse, error) {
	return &provider.ScanResponse{
		Assessments: []provider.AssessmentLog{{
			RequirementID: "req-1",
			Steps:         []provider.Step{{Name: "check", Result: provider.ResultPassed}},
			Message:       "evaluated",
		}},
		Errors: []string{"target 'staging': clone failed: auth denied"},
	}, nil
}

func TestManager_RouteScanResult_PartialResults(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	mock := &errorEmbeddingMockClient{}
	lp := provider.NewMockLoadedProvider("partial-provider", "partial-eval", mock)
	mgr.RegisterProviderForTest("partial-eval", lp)

	result, err := mgr.RouteScanResult(context.Background(), "partial-eval",
		[]provider.Target{{TargetID: "prod"}, {TargetID: "staging"}})
	require.NoError(t, err)

	assert.True(t, result.HasErrors())
	assert.Contains(t, result.Errors[0], "clone failed")
	require.Len(t, result.Assessments, 1)
	assert.Equal(t, "req-1", result.Assessments[0].RequirementID)
}

func TestManager_RouteScanResult_NoErrors(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	mock := newMockClient()
	lp := provider.NewMockLoadedProvider("ok-provider", "ok-eval", mock)
	mgr.RegisterProviderForTest("ok-eval", lp)

	result, err := mgr.RouteScanResult(context.Background(), "ok-eval",
		[]provider.Target{{TargetID: "t1"}})
	require.NoError(t, err)

	assert.False(t, result.HasErrors())
	assert.Empty(t, result.Errors)
}

func TestManager_RouteScanResult_Broadcast_AggregatesErrors(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	// Provider that returns partial results with errors
	partial := &errorEmbeddingMockClient{}
	lpPartial := provider.NewMockLoadedProvider("partial-provider", "partial-eval", partial)
	mgr.RegisterProviderForTest("partial-eval", lpPartial)

	// Provider that succeeds cleanly
	clean := newMockClient()
	lpClean := provider.NewMockLoadedProvider("clean-provider", "clean-eval", clean)
	mgr.RegisterProviderForTest("clean-eval", lpClean)

	// Broadcast mode: empty evaluatorID scans all providers
	result, err := mgr.RouteScanResult(context.Background(), "",
		[]provider.Target{{TargetID: "prod"}})
	require.NoError(t, err)

	assert.True(t, result.HasErrors())
	assert.Contains(t, result.Errors[0], "clone failed")
	// Should have assessments from both providers
	assert.GreaterOrEqual(t, len(result.Assessments), 1)
}

func TestManager_RouteScanResult_Broadcast_RPCFailure(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	// Provider that fails at the RPC level
	failing := &failingMockClient{failScan: true}
	lpFailing := provider.NewMockLoadedProvider("fail-provider", "fail-eval", failing)
	mgr.RegisterProviderForTest("fail-eval", lpFailing)

	// Provider that succeeds cleanly — must Generate first so Scan returns results
	clean := newMockClient()
	lpClean := provider.NewMockLoadedProvider("clean-provider", "clean-eval", clean)
	mgr.RegisterProviderForTest("clean-eval", lpClean)
	_, _ = clean.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{{RequirementID: "req-1"}},
	})

	// Broadcast: both providers scanned, one fails
	result, err := mgr.RouteScanResult(context.Background(), "",
		[]provider.Target{{TargetID: "t1"}})
	require.NoError(t, err)

	assert.True(t, result.HasErrors())
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "scan RPC failed")
	// Assessments come only from the clean provider — RPC failures do not
	// inject synthetic assessments (D3).
	require.Len(t, result.Assessments, 1)
	assert.Equal(t, "req-1", result.Assessments[0].PlanID)
}

func TestManager_RouteScan_DropsProviderErrors(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	// Provider returns both assessments and errors
	mock := &errorEmbeddingMockClient{}
	lp := provider.NewMockLoadedProvider("partial-provider", "partial-eval", mock)
	mgr.RegisterProviderForTest("partial-eval", lp)

	// RouteScan returns only assessments, not errors
	results, err := mgr.RouteScan(context.Background(), "partial-eval",
		[]provider.Target{{TargetID: "prod"}})
	require.NoError(t, err)

	// Assessments are returned
	require.Len(t, results, 1)
	assert.Equal(t, "req-1", results[0].RequirementID)
	// The error is silently dropped (backwards-compatible behavior)
}

func TestScanResult_HasErrors_EdgeCases(t *testing.T) {
	// nil Errors slice
	nilResult := &provider.ScanResult{Errors: nil}
	assert.False(t, nilResult.HasErrors())

	// empty Errors slice
	emptyResult := &provider.ScanResult{Errors: []string{}}
	assert.False(t, emptyResult.HasErrors())

	// populated Errors slice
	populatedResult := &provider.ScanResult{Errors: []string{"some error"}}
	assert.True(t, populatedResult.HasErrors())
}
