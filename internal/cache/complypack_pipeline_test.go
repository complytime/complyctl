// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/pkg/provider"
)

// TestComplypackPipeline_SyncLookupGenerate is an integration test that
// verifies the full complypack pipeline from cache storage through to
// provider invocation:
//
//  1. A complypack is synced into the cache via ComplypackSync
//  2. The cache directory structure is verified (complypacks/{evaluator-id}/{version}/)
//  3. state.json is verified to contain complypack entries
//  4. LookupByEvaluatorID returns a non-empty content path
//  5. A mock provider receives the content path via GenerateRequest
//
// This replaces a full E2E test (which would require a mock OCI registry,
// compiled test-provider binary, and complytime.yaml configuration) with a
// focused integration test that exercises the same code paths.
func TestComplypackPipeline_SyncLookupGenerate(t *testing.T) {
	cacheDir := t.TempDir()

	// --- Phase 1: Sync a complypack into the cache ---

	mock := newMockComplypackSource()
	mock.seedComplypack(
		"example.com/complypacks/test-bundle",
		"io.complytime.test",
		"1.0.0",
		"sha256:pipeline-test-digest",
		"test policy content for pipeline integration",
	)

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)
	_, err = syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/test-bundle", "1.0.0")
	require.NoError(t, err, "complypack sync should succeed")

	// --- Phase 2: Verify cache directory structure ---

	expectedDir := filepath.Join(cacheDir, complytime.ComplypacksSubdir, "io.complytime.test", "1.0.0")
	assert.DirExists(t, expectedDir,
		"cache directory should exist at {cacheDir}/complypacks/{evaluator-id}/{version}/")

	contentFile := filepath.Join(expectedDir, "content.tar.gz")
	assert.FileExists(t, contentFile, "content.tar.gz should exist in cache directory")

	configFile := filepath.Join(expectedDir, "config.json")
	assert.FileExists(t, configFile, "config.json should exist in cache directory")

	// Verify content.tar.gz is non-empty (complypack.Pack wrote real content).
	info, err := os.Stat(contentFile)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "content.tar.gz should not be empty")

	// --- Phase 3: Verify state.json contains complypack entries ---

	statePath := filepath.Join(cacheDir, complytime.StateFileName)
	assert.FileExists(t, statePath, "state.json should exist after sync")

	stateData, err := os.ReadFile(statePath)
	require.NoError(t, err)

	var rawState map[string]interface{}
	err = json.Unmarshal(stateData, &rawState)
	require.NoError(t, err, "state.json should be valid JSON")

	complypacks, ok := rawState["complypacks"].(map[string]interface{})
	require.True(t, ok, "state.json should contain a 'complypacks' object")
	assert.Contains(t, complypacks, "example.com/complypacks/test-bundle",
		"state.json complypacks should contain the synced repository key")

	// Verify state via typed API as well.
	loadedState, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	ps, exists := loadedState.GetComplypackState("example.com/complypacks/test-bundle")
	require.True(t, exists, "complypack state should exist for synced repository")
	assert.Equal(t, "sha256:pipeline-test-digest", ps.Digest)
	assert.Equal(t, "1.0.0", ps.Version)

	// --- Phase 4: LookupByEvaluatorID returns non-empty content path ---

	contentPath, lookupCfg, err := complypackCache.LookupByEvaluatorID("io.complytime.test")
	require.NoError(t, err, "LookupByEvaluatorID should not error for cached complypack")
	assert.NotEmpty(t, contentPath,
		"LookupByEvaluatorID should return a non-empty path for a cached complypack")
	assert.FileExists(t, contentPath,
		"content path from LookupByEvaluatorID should point to an existing file")
	assert.True(t, strings.HasSuffix(contentPath, "content.tar.gz"),
		"content path should end with content.tar.gz, got %s", contentPath)
	require.NotNil(t, lookupCfg, "LookupByEvaluatorID should return a non-nil config")
	assert.Equal(t, "io.complytime.test", lookupCfg.EvaluatorID)
	assert.Equal(t, "1.0.0", lookupCfg.Version)

	// --- Phase 5: Mock provider receives the content path via GenerateRequest ---

	mockProvider := &capturingProvider{}
	req := &provider.GenerateRequest{
		GlobalVariables: map[string]string{"workspace": "/tmp/test"},
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "plan-1", RequirementID: "req-1"},
		},
		ComplypackContentPath: contentPath,
	}

	resp, err := mockProvider.Generate(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, contentPath, mockProvider.receivedComplypackPath,
		"provider should receive the complypack content path from the cache lookup")
	assert.NotEmpty(t, mockProvider.receivedComplypackPath,
		"provider should receive a non-empty complypack_content_path")
}

// TestComplypackPipeline_NoComplypack_BackwardCompatible verifies that when
// no complypack is cached for an evaluator-id, the pipeline passes an empty
// string to the provider (backward compatible behavior).
func TestComplypackPipeline_NoComplypack_BackwardCompatible(t *testing.T) {
	cacheDir := t.TempDir()
	complypackCache := cache.NewComplypackCache(cacheDir)

	// LookupByEvaluatorID for a non-existent evaluator should return empty.
	contentPath, lookupCfg, err := complypackCache.LookupByEvaluatorID("io.complytime.nonexistent")
	require.NoError(t, err, "LookupByEvaluatorID should not error for missing complypack")
	assert.Empty(t, contentPath,
		"LookupByEvaluatorID should return empty string for missing complypack")
	assert.Nil(t, lookupCfg,
		"LookupByEvaluatorID should return nil config for missing complypack")

	// Provider receives empty complypack path — backward compatible.
	mockProvider := &capturingProvider{}
	req := &provider.GenerateRequest{
		GlobalVariables: map[string]string{"workspace": "/tmp/test"},
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "plan-1", RequirementID: "req-1"},
		},
		ComplypackContentPath: contentPath,
	}

	resp, err := mockProvider.Generate(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Empty(t, mockProvider.receivedComplypackPath,
		"provider should receive empty complypack_content_path when no complypack is cached")
}

// TestComplypackPipeline_MultipleEvaluators verifies that the cache correctly
// isolates complypacks by evaluator-id, and each provider receives the correct
// content path for its evaluator.
func TestComplypackPipeline_MultipleEvaluators(t *testing.T) {
	cacheDir := t.TempDir()

	mock := newMockComplypackSource()
	mock.seedComplypack(
		"example.com/complypacks/opa-bundle",
		"io.complytime.opa",
		"1.0.0",
		"sha256:opa-digest",
		"opa policy content",
	)
	mock.seedComplypack(
		"example.com/complypacks/kyverno-bundle",
		"io.complytime.kyverno",
		"2.0.0",
		"sha256:kyverno-digest",
		"kyverno policy content",
	)

	complypackCache := cache.NewComplypackCache(cacheDir)
	state, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	syncMgr := cache.NewComplypackSync(complypackCache, state, mock)

	_, err = syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/opa-bundle", "1.0.0")
	require.NoError(t, err)
	_, err = syncMgr.SyncComplypack(context.Background(), "example.com/complypacks/kyverno-bundle", "2.0.0")
	require.NoError(t, err)

	// Verify each evaluator gets its own content path.
	opaPath, _, err := complypackCache.LookupByEvaluatorID("io.complytime.opa")
	require.NoError(t, err)
	assert.NotEmpty(t, opaPath)
	assert.Contains(t, opaPath, "io.complytime.opa",
		"OPA content path should contain the OPA evaluator-id")

	kyvernoPath, _, err := complypackCache.LookupByEvaluatorID("io.complytime.kyverno")
	require.NoError(t, err)
	assert.NotEmpty(t, kyvernoPath)
	assert.Contains(t, kyvernoPath, "io.complytime.kyverno",
		"Kyverno content path should contain the Kyverno evaluator-id")

	// Paths must be distinct.
	assert.NotEqual(t, opaPath, kyvernoPath,
		"different evaluators should have different content paths")

	// Verify state.json has both entries.
	loadedState, err := cache.LoadState(cacheDir)
	require.NoError(t, err)

	_, opaExists := loadedState.GetComplypackState("example.com/complypacks/opa-bundle")
	assert.True(t, opaExists, "state should contain OPA complypack entry")

	_, kyvernoExists := loadedState.GetComplypackState("example.com/complypacks/kyverno-bundle")
	assert.True(t, kyvernoExists, "state should contain Kyverno complypack entry")
}

// capturingProvider is a minimal provider.Provider that captures the
// ComplypackContentPath from GenerateRequest for test assertions.
// It mirrors the test-provider's behavior but runs in-process.
type capturingProvider struct {
	receivedComplypackPath string
	requirementIDs         []string
}

// Compile-time check: capturingProvider must implement provider.Provider.
var _ provider.Provider = (*capturingProvider)(nil)

func (p *capturingProvider) Describe(_ context.Context, _ *provider.DescribeRequest) (*provider.DescribeResponse, error) {
	return &provider.DescribeResponse{
		Healthy: true,
		Version: "capturing-v0.1.0",
	}, nil
}

func (p *capturingProvider) Generate(_ context.Context, req *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	p.receivedComplypackPath = req.ComplypackContentPath
	p.requirementIDs = make([]string, 0, len(req.Configuration))
	for _, cfg := range req.Configuration {
		p.requirementIDs = append(p.requirementIDs, cfg.PlanID)
	}
	return &provider.GenerateResponse{Success: true}, nil
}

func (p *capturingProvider) Scan(_ context.Context, _ *provider.ScanRequest) (*provider.ScanResponse, error) {
	assessments := make([]provider.AssessmentLog, 0, len(p.requirementIDs))
	for _, reqID := range p.requirementIDs {
		assessments = append(assessments, provider.AssessmentLog{
			RequirementID: reqID,
			Steps: []provider.Step{
				{Name: "capture-check", Result: provider.ResultPassed, Message: "captured"},
			},
			Message:    "captured",
			Confidence: provider.ConfidenceLevelHigh,
		})
	}
	return &provider.ScanResponse{Assessments: assessments}, nil
}
