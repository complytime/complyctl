// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package provider_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/pkg/provider"
)

const lifecycleHelperEnv = "COMPLYCTL_PROVIDER_LIFECYCLE_TEST"

// TestMain lets lifecycle tests launch this binary using the existing provider
// executable contract, without a compiler, registry, or modified wire protocol.
func TestMain(m *testing.M) {
	if os.Getenv(lifecycleHelperEnv) != "" {
		provider.Serve(lifecycleProvider{})
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type lifecycleProvider struct{}

func (lifecycleProvider) Describe(context.Context, *provider.DescribeRequest) (*provider.DescribeResponse, error) {
	if os.Getenv(lifecycleHelperEnv) == "describe-rpc" {
		return nil, status.Error(codes.Unavailable, "Describe transport failure")
	}
	return &provider.DescribeResponse{Healthy: true, Version: "lifecycle-test"}, nil
}

func (lifecycleProvider) Generate(context.Context, *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	if os.Getenv(lifecycleHelperEnv) == "generate-rpc" {
		return nil, status.Error(codes.Unavailable, "Generate transport failure")
	}
	return &provider.GenerateResponse{ErrorMessage: "handled failure"}, nil
}

func (lifecycleProvider) Scan(context.Context, *provider.ScanRequest) (*provider.ScanResponse, error) {
	if os.Getenv(lifecycleHelperEnv) == "scan-rpc" {
		return nil, status.Error(codes.Unavailable, "Scan transport failure")
	}
	return &provider.ScanResponse{Errors: []string{"handled failure"}}, nil
}

func TestManager_CleanupNormal_TracksRPCCompletion(t *testing.T) {
	for _, mode := range []string{"handled", "describe-rpc", "generate-rpc", "scan-rpc"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(lifecycleHelperEnv, mode)
			t.Setenv(complytime.SystemProviderDirEnvVar, t.TempDir())
			dir := t.TempDir()
			executable, err := os.Executable()
			require.NoError(t, err)
			require.NoError(t, os.Symlink(executable, filepath.Join(dir, "complyctl-provider-lifecycle")))
			mgr, err := provider.NewManager(dir, nil)
			require.NoError(t, err)
			// Safe fallback if an assertion stops the test before explicit cleanup.
			defer func() { _ = mgr.CleanupNormal() }()
			require.NoError(t, mgr.LoadProviders())
			if mode == "describe-rpc" {
				assert.Empty(t, mgr.ListProviders(), "skipped clients still belong to cleanup")
			} else {
				loaded, err := mgr.GetProvider("lifecycle")
				require.NoError(t, err)
				gen, err := loaded.Client.Generate(context.Background(), &provider.GenerateRequest{})
				if mode == "generate-rpc" {
					require.ErrorContains(t, err, "Generate transport failure")
				} else {
					require.NoError(t, err)
					assert.False(t, gen.Success)
					assert.Equal(t, "handled failure", gen.ErrorMessage)
				}
				scan, err := loaded.Client.Scan(context.Background(), &provider.ScanRequest{})
				if mode == "scan-rpc" {
					require.ErrorContains(t, err, "Scan transport failure")
				} else {
					require.NoError(t, err)
					assert.Equal(t, []string{"handled failure"}, scan.Errors)
				}
			}
			err = mgr.CleanupNormal()
			if mode == "handled" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "completion or shutdown is uncertain")
			}
			assert.Equal(t, err, mgr.CleanupNormal(), "later cleanup cannot erase uncertainty")
		})
	}
}

func TestManager_CleanupNormal_StartupFailure(t *testing.T) {
	t.Setenv(complytime.SystemProviderDirEnvVar, t.TempDir())
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "complyctl-provider-broken"),
		[]byte("not an executable format"), 0o600))
	require.NoError(t, os.Chmod(filepath.Join(dir, "complyctl-provider-broken"), 0o700))
	mgr, err := provider.NewManager(dir, nil)
	require.NoError(t, err)
	require.ErrorContains(t, mgr.LoadProviders(), "failed to create client")
	require.ErrorContains(t, mgr.CleanupNormal(), "startup did not complete normally")
	assert.Empty(t, mgr.ListProviders())
}

func TestClient_Close_ZeroValue(t *testing.T) {
	client := &provider.Client{}
	assert.NotPanics(t, func() { client.Close() })
	assert.NotPanics(t, func() { client.Close() })
}
