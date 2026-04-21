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
	})
	require.NoError(t, err)
}

func TestManager_RouteGenerate_UnknownEvaluator(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	err = mgr.RouteGenerate(context.Background(), "nonexistent", nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no provider registered")
}

func TestManager_RouteGenerate_Broadcast(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	mock := newMockClient()
	lp := provider.NewMockLoadedProvider("test-provider", "test-eval", mock)
	mgr.RegisterProviderForTest("test-eval", lp)

	err = mgr.RouteGenerate(context.Background(), "", nil, nil, nil)
	require.NoError(t, err)
}

func TestManager_RouteGenerate_ProviderReturnsFailure(t *testing.T) {
	mgr, err := provider.NewManager(t.TempDir(), nil)
	require.NoError(t, err)

	fm := &failingMockClient{failGenerate: true}
	lp := provider.NewMockLoadedProvider("fail-provider", "fail-eval", fm)
	mgr.RegisterProviderForTest("fail-eval", lp)

	err = mgr.RouteGenerate(context.Background(), "fail-eval", nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generate RPC failed")
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
