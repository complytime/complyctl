// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/internal/registry"
)

// --- Mock VersionResolver ---

type mockVersionResolver struct {
	versions       map[string]string // "registry|repo" -> latest version
	pinnedVersions map[string]string // "registry|repo|version" -> resolved version
	unreachable    map[string]bool   // registry -> true
	errOnResolve   map[string]error  // "registry|repo" -> error
	latestMissing  map[string]bool   // registry -> true (reachable but no latest tag)
}

func newMockVersionResolver() *mockVersionResolver {
	return &mockVersionResolver{
		versions:       make(map[string]string),
		pinnedVersions: make(map[string]string),
		unreachable:    make(map[string]bool),
		errOnResolve:   make(map[string]error),
		latestMissing:  make(map[string]bool),
	}
}

func (m *mockVersionResolver) ResolveLatestVersion(reg, repository string) (string, error) {
	if m.unreachable[reg] {
		return "", fmt.Errorf("connection refused")
	}
	if m.latestMissing[reg] {
		return "", fmt.Errorf("%w: %s/%s tag %q", registry.ErrVersionNotFound, reg, repository, "latest")
	}
	key := reg + "|" + repository
	if err, ok := m.errOnResolve[key]; ok {
		return "", err
	}
	if v, ok := m.versions[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("not found: %s/%s", reg, repository)
}

func (m *mockVersionResolver) ResolveVersion(reg, repository, version string) (string, error) {
	if m.unreachable[reg] {
		return "", fmt.Errorf("connection refused")
	}
	key := reg + "|" + repository + "|" + version
	if v, ok := m.pinnedVersions[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("not found: %s/%s:%s", reg, repository, version)
}

// --- Mock PolicyGraphResolver ---

type mockPolicyGraphResolver struct {
	versions map[string]string
	graphs   map[string]*policy.DependencyGraph
}

func newMockPolicyGraphResolver() *mockPolicyGraphResolver {
	return &mockPolicyGraphResolver{
		versions: make(map[string]string),
		graphs:   make(map[string]*policy.DependencyGraph),
	}
}

func (m *mockPolicyGraphResolver) ResolveVersion(policyID, configVersion string) (string, error) {
	key := policyID + "@" + configVersion
	if v, ok := m.versions[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("version not found: %s", key)
}

func (m *mockPolicyGraphResolver) ResolvePolicyGraph(policyID, version string) (*policy.DependencyGraph, error) {
	key := policyID + "@" + version
	if g, ok := m.graphs[key]; ok {
		return g, nil
	}
	return nil, fmt.Errorf("graph not found: %s", key)
}

// --- CheckPolicyVersions Tests ---

func TestCheckPolicyVersions_NilConfig(t *testing.T) {
	results := CheckPolicyVersions(nil, "/tmp", newMockVersionResolver())
	assert.Nil(t, results)
}

func TestCheckPolicyVersions_NoPolicies(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{}
	results := CheckPolicyVersions(cfg, "/tmp", newMockVersionResolver())
	assert.Nil(t, results)
}

func TestCheckPolicyVersions_NilResolver(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	results := CheckPolicyVersions(cfg, "/tmp", nil)
	assert.Nil(t, results)
}

func TestCheckPolicyVersions_PolicyAtLatest(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v1.0.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results[0].Status)
	assert.Contains(t, results[0].Message, "(pinned)")
}

func TestCheckPolicyVersions_UnpinnedAtLatest(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v1.0.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results[0].Status)
	assert.Contains(t, results[0].Message, "(latest)")
}

func TestCheckPolicyVersions_PinnedMatchesCached_LatestDiffers(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v1.1.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results[0].Status)
	assert.Contains(t, results[0].Message, "(pinned")
	assert.Contains(t, results[0].Message, "latest available: v1.1.0")
}

func TestCheckPolicyVersions_UnpinnedStale(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v1.1.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 1)
	assert.Equal(t, StatusWarn, results[0].Status)
	assert.Contains(t, results[0].Message, "cached v1.0.0")
	assert.Contains(t, results[0].Message, "available v1.1.0")
}

func TestCheckPolicyVersions_PinnedMismatchCached(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v2.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v2.0.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 1)
	assert.Equal(t, StatusWarn, results[0].Status)
	assert.Contains(t, results[0].Message, "does not match configured pin")
	assert.Contains(t, results[0].Message, "@v2.0.0")
}

func TestCheckPolicyVersions_NotCached(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v1.0.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 1)
	assert.Equal(t, StatusWarn, results[0].Status)
	assert.Contains(t, results[0].Message, "not cached")
}

func TestCheckPolicyVersions_RegistryUnreachable(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "unreachable.io/policies/nist:v1.0.0"},
			{URL: "unreachable.io/policies/cis:v2.0.0", ID: "cis"},
		},
	}

	vr := newMockVersionResolver()
	vr.unreachable["unreachable.io"] = true

	require.NoError(t, state.UpdatePolicyStateWithVerification("policies/cis", "v2.0.0", "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", nil))
	require.NoError(t, cache.SaveState(state, tmpDir))

	results := CheckPolicyVersions(cfg, tmpDir, vr)

	require.Len(t, results, 2)
	assert.Equal(t, "policy/nist", results[0].Name)
	assert.Equal(t, StatusWarn, results[0].Status)
	assert.Contains(t, results[0].Message, "registry unreachable")
	assert.Equal(t, "policy/cis", results[1].Name)
	assert.Equal(t, StatusWarn, results[1].Status)
}

func TestCheckPolicyVersions_LatestMissing_PinnedResolves(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.latestMissing["reg.io"] = true
	vr.pinnedVersions["reg.io|policies/nist|v1.0.0"] = "v1.0.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results[0].Status)
	assert.Contains(t, results[0].Message, "pinned")
	assert.Contains(t, results[0].Message, "latest tag unavailable")
}

func TestCheckPolicyVersions_LatestMissing_NoPinnedVersion(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist"},
		},
	}

	vr := newMockVersionResolver()
	vr.latestMissing["reg.io"] = true

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 1)
	assert.Equal(t, StatusWarn, results[0].Status)
	assert.Contains(t, results[0].Message, "latest tag not found")
	assert.Contains(t, results[0].Message, "pin a specific version")
}

func TestCheckPolicyVersions_LatestMissing_DoesNotPoisonSameRegistry(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/alpha": {Version: "v1.0.0"},
		"policies/beta":  {Version: "v2.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/alpha"},
			{URL: "reg.io/policies/beta"},
		},
	}

	vr := newMockVersionResolver()
	vr.latestMissing["reg.io"] = true

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, StatusWarn, r.Status, "expected warn for %s", r.Name)
		assert.Contains(t, r.Message, "latest tag not found")
	}
}

func TestCheckPolicyVersions_PinnedBoth404(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v2.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.latestMissing["reg.io"] = true

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 1)
	assert.Equal(t, StatusWarn, results[0].Status)
	assert.Contains(t, results[0].Message, "not found in registry")
}

func TestCheckPolicyVersions_MixedRegistries_Unreachable_And_404(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/alpha": {Version: "v1.0.0"},
		"policies/beta":  {Version: "v2.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "down.io/policies/alpha"},
			{URL: "up.io/policies/beta"},
		},
	}

	vr := newMockVersionResolver()
	vr.unreachable["down.io"] = true
	vr.latestMissing["up.io"] = true

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 2)
	assert.Equal(t, "policy/alpha", results[0].Name)
	assert.Contains(t, results[0].Message, "registry unreachable")
	assert.Contains(t, results[1].Message, "latest tag not found")
}

func TestCheckPolicyVersions_PinnedNetworkFailure_BothFail(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
		"policies/cis":  {Version: "v1.0.0"},
	}}
	require.NoError(t, cache.SaveState(state, tmpDir))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "flaky.io/policies/nist:v1.0.0"},
			{URL: "flaky.io/policies/cis:v1.0.0", ID: "cis"},
		},
	}

	vr := newMockVersionResolver()
	vr.unreachable["flaky.io"] = true

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	require.Len(t, results, 2)
	assert.Equal(t, "policy/nist", results[0].Name)
	assert.Contains(t, results[0].Message, "registry unreachable")
	assert.Equal(t, "policy/cis", results[1].Name)
	assert.Contains(t, results[1].Message, "registry unreachable")
}

func TestCheckPolicyVersions_BadCacheState(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, complytime.StateFileName), []byte("{bad json"), 0600))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}

	results := CheckPolicyVersions(cfg, tmpDir, newMockVersionResolver())
	require.Len(t, results, 1)
	assert.Equal(t, StatusWarn, results[0].Status)
}

// --- CheckVariables Tests (refactored with summary + verbose) ---

func TestCheckVariables_NoHealthData(t *testing.T) {
	results := CheckVariables(nil, nil, nil, false)
	assert.Nil(t, results)
}

func TestCheckVariables_NilConfig(t *testing.T) {
	health := []ProviderHealth{{EvaluatorID: "openscap"}}
	results := CheckVariables(nil, health, nil, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusFail, results[0].Status)
}

func TestCheckVariables_AllPresent_DefaultMode(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{"output_dir": "/tmp"},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredGlobalVariables: []string{"output_dir"},
	}}

	results := CheckVariables(cfg, health, nil, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results[0].Status)
	assert.Contains(t, results[0].Message, "1/1 global vars")
}

func TestCheckVariables_MissingGlobal_DefaultMode(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredGlobalVariables: []string{"output_dir", "scan_target"},
	}}

	results := CheckVariables(cfg, health, nil, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusFail, results[0].Status)
	assert.Contains(t, results[0].Message, "0/2 global vars")
	assert.Contains(t, results[0].Message, "output_dir")
}

func TestCheckVariables_VerboseMode_ExpandsDetail(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{"output_dir": "/tmp"},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredGlobalVariables: []string{"output_dir", "scan_target"},
	}}

	results := CheckVariables(cfg, health, nil, true)

	if len(results) != 1 {
		t.Fatalf("expected 1 summary result, got %d", len(results))
	}
	summary := results[0]
	if len(summary.Children) != 2 {
		t.Fatalf("expected 2 detail children (one per global var), got %d", len(summary.Children))
	}

	foundOutputDir := false
	foundScanTarget := false
	for _, child := range summary.Children {
		if strings.Contains(child.Message, "output_dir") {
			foundOutputDir = true
			assert.Equal(t, StatusPass, child.Status, "expected StatusPass for output_dir")
		}
		if strings.Contains(child.Message, "scan_target") {
			foundScanTarget = true
			assert.Equal(t, StatusFail, child.Status, "expected StatusFail for scan_target")
		}
	}
	assert.True(t, foundOutputDir, "expected verbose detail showing output_dir")
	assert.True(t, foundScanTarget, "expected verbose detail showing scan_target")
}

func TestCheckVariables_NoVerbose_NoDetail(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{"output_dir": "/tmp"},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredGlobalVariables: []string{"output_dir"},
	}}

	results := CheckVariables(cfg, health, nil, false)
	for _, r := range results {
		assert.False(t, strings.HasSuffix(r.Name, "/detail"),
			"did not expect detail results in non-verbose mode, got %q", r.Name)
		assert.Empty(t, r.Children,
			"did not expect Children in non-verbose mode for %q", r.Name)
	}
}

func TestCheckVariables_UnmappedTargetVars_NilResolver(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{"output_dir": "/tmp"},
		Policies:  []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
		Targets:   []complytime.TargetConfig{{ID: "host1", Policies: []string{"nist"}}},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredGlobalVariables: []string{"output_dir"},
		RequiredTargetVariables: []string{"profile"},
	}}

	results := CheckVariables(cfg, health, nil, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusFail, results[0].Status)
	assert.Contains(t, results[0].Message, "target vars not validated")
	assert.Contains(t, results[0].Message, "no policy resolver")
}

func TestCheckVariables_UnmappedTargetVars_ResolverFails(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
		Policies:  []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
		Targets: []complytime.TargetConfig{{
			ID:       "host1",
			Policies: []string{"nist"},
		}},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredTargetVariables: []string{"profile"},
	}}

	resolver := newMockPolicyGraphResolver()

	results := CheckVariables(cfg, health, resolver, false)
	require.GreaterOrEqual(t, len(results), 2)

	foundResolveWarn := false
	for _, r := range results {
		if r.Status == StatusWarn && strings.Contains(r.Name, "variables/resolve/") {
			foundResolveWarn = true
		}
	}
	assert.True(t, foundResolveWarn, "expected a resolve failure warning result")

	last := results[len(results)-1]
	assert.Equal(t, StatusFail, last.Status)
	assert.Contains(t, last.Message, "target vars not validated")
	assert.Contains(t, last.Message, "policy graph unresolved")
}

func TestCheckVariables_UnmappedTargetVars_EvaluatorNotInGraph(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
		Policies:  []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
		Targets: []complytime.TargetConfig{{
			ID:       "host1",
			Policies: []string{"nist"},
		}},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "unused-evaluator",
		RequiredTargetVariables: []string{"profile"},
	}}

	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID:    "policies/nist",
		EvaluatorID: "openscap",
	}

	results := CheckVariables(cfg, health, resolver, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results[0].Status)
	assert.Contains(t, results[0].Message, "no target mapping")
}

func TestCheckVariables_MappedTargetVars_MissingProfile(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
		Policies:  []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
		Targets: []complytime.TargetConfig{{
			ID:        "host1",
			Policies:  []string{"nist"},
			Variables: map[string]string{},
		}},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredTargetVariables: []string{"profile"},
	}}

	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID:    "policies/nist",
		EvaluatorID: "openscap",
	}

	results := CheckVariables(cfg, health, resolver, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusFail, results[0].Status)
	assert.Contains(t, results[0].Message, "profile")
}

func TestCheckVariables_Verbose_UnmappedTargetVars(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
		Policies:  []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
		Targets: []complytime.TargetConfig{{
			ID:       "host1",
			Policies: []string{"nist"},
		}},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredTargetVariables: []string{"profile"},
	}}

	results := CheckVariables(cfg, health, nil, true)

	if len(results) < 1 {
		t.Fatalf("expected at least 1 result, got %d", len(results))
	}

	// Find the summary result and check its children for the "not validated" detail.
	foundNotValidated := false
	for _, r := range results {
		for _, child := range r.Children {
			if strings.Contains(child.Message, "profile") && strings.Contains(child.Message, "not validated") {
				foundNotValidated = true
			}
		}
	}
	assert.True(t, foundNotValidated, "expected verbose detail showing profile as not validated")
}

func TestCheckVariables_WorkspaceAutoInjected_NotInConfig(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "test",
		RequiredGlobalVariables: []string{complytime.WorkspaceVarKey},
	}}

	results := CheckVariables(cfg, health, nil, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results[0].Status)
	assert.Contains(t, results[0].Message, "1/1 global vars")
}

func TestCheckVariables_WorkspaceAutoInjected_Verbose(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "test",
		RequiredGlobalVariables: []string{complytime.WorkspaceVarKey, "output_dir"},
	}}

	results := CheckVariables(cfg, health, nil, true)

	if len(results) < 1 {
		t.Fatalf("expected at least 1 result, got %d", len(results))
	}

	// D9: detail is now nested as Children with Status reflecting presence.
	foundWorkspacePassed := false
	foundOutputDirFailed := false
	for _, r := range results {
		for _, child := range r.Children {
			if strings.Contains(child.Message, complytime.WorkspaceVarKey) && child.Status == StatusPass {
				foundWorkspacePassed = true
			}
			if strings.Contains(child.Message, "output_dir") && child.Status == StatusFail {
				foundOutputDirFailed = true
			}
		}
	}
	assert.True(t, foundWorkspacePassed, "expected verbose detail showing workspace as passed (auto-injected)")
	assert.True(t, foundOutputDirFailed, "expected verbose detail showing output_dir as failed")
}

// --- CheckVariables Resolution Failure Tests ---

func TestCheckVariables_ResolveFailure_PolicyNotFound(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
		Policies:  []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
		Targets: []complytime.TargetConfig{{
			ID:       "host1",
			Policies: []string{"nonexistent-policy"},
		}},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredGlobalVariables: []string{},
	}}

	resolver := newMockPolicyGraphResolver()
	results := CheckVariables(cfg, health, resolver, false)

	foundResolveWarn := false
	for _, r := range results {
		if r.Status == StatusWarn && strings.Contains(r.Message, "nonexistent-policy") &&
			strings.Contains(r.Message, "not found in config") {
			foundResolveWarn = true
		}
	}
	assert.True(t, foundResolveWarn, "expected StatusWarn result for missing policy")
}

func TestCheckVariables_ResolveFailure_InvalidRef(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
		Policies:  []complytime.PolicyEntry{{URL: "", ID: "bad"}},
		Targets: []complytime.TargetConfig{{
			ID:       "host1",
			Policies: []string{"bad"},
		}},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredGlobalVariables: []string{},
	}}

	resolver := newMockPolicyGraphResolver()
	results := CheckVariables(cfg, health, resolver, false)

	foundResolveWarn := false
	for _, r := range results {
		if r.Status == StatusWarn && strings.Contains(r.Message, "invalid policy reference") {
			foundResolveWarn = true
		}
	}
	assert.True(t, foundResolveWarn, "expected StatusWarn result for invalid policy reference")
}

func TestCheckVariables_ResolveFailure_VersionNotFound(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
		Policies:  []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
		Targets: []complytime.TargetConfig{{
			ID:       "host1",
			Policies: []string{"nist"},
		}},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredGlobalVariables: []string{},
	}}

	resolver := newMockPolicyGraphResolver()
	results := CheckVariables(cfg, health, resolver, false)

	foundResolveWarn := false
	for _, r := range results {
		if r.Status == StatusWarn && strings.Contains(r.Message, "cannot resolve version") {
			foundResolveWarn = true
		}
	}
	assert.True(t, foundResolveWarn, "expected StatusWarn result for version resolution failure")
}

func TestCheckVariables_ResolveFailure_GraphNotFound(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Variables: map[string]string{},
		Policies:  []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
		Targets: []complytime.TargetConfig{{
			ID:       "host1",
			Policies: []string{"nist"},
		}},
	}
	health := []ProviderHealth{{
		EvaluatorID:             "openscap",
		RequiredGlobalVariables: []string{},
	}}

	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"

	results := CheckVariables(cfg, health, resolver, false)

	foundResolveWarn := false
	for _, r := range results {
		if r.Status == StatusWarn && strings.Contains(r.Message, "cannot resolve policy graph") {
			foundResolveWarn = true
		}
	}
	assert.True(t, foundResolveWarn, "expected StatusWarn result for graph resolution failure")
}

// --- CheckPolicyActivePeriod Tests ---

func TestCheckPolicyActivePeriod_NilConfig(t *testing.T) {
	results := CheckPolicyActivePeriod(nil, newMockPolicyGraphResolver(), false)
	assert.Nil(t, results)
}

func TestCheckPolicyActivePeriod_NilResolver(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	results := CheckPolicyActivePeriod(cfg, nil, false)
	assert.Nil(t, results)
}

func TestCheckPolicyActivePeriod_NoTimeline(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID: "policies/nist",
		Timeline: nil,
	}

	results := CheckPolicyActivePeriod(cfg, resolver, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results[0].Status)
	assert.Contains(t, results[0].Message, "no evaluation timeline")
}

func TestCheckPolicyActivePeriod_Active(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID: "policies/nist",
		Timeline: &policy.PolicyTimeline{
			EvaluationStart: "2025-01-01",
			EvaluationEnd:   "2099-12-31",
		},
	}

	results := CheckPolicyActivePeriod(cfg, resolver, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results[0].Status)
	assert.Contains(t, results[0].Message, "active")
}

func TestCheckPolicyActivePeriod_NotYetActive(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID: "policies/nist",
		Timeline: &policy.PolicyTimeline{
			EvaluationStart: "2099-01-01",
			EvaluationEnd:   "2099-12-31",
		},
	}

	results := CheckPolicyActivePeriod(cfg, resolver, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusWarn, results[0].Status)
	assert.Contains(t, results[0].Message, "begins")
}

func TestCheckPolicyActivePeriod_Expired(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID: "policies/nist",
		Timeline: &policy.PolicyTimeline{
			EvaluationStart: "2020-01-01",
			EvaluationEnd:   "2020-12-31",
		},
	}

	results := CheckPolicyActivePeriod(cfg, resolver, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusWarn, results[0].Status)
	assert.Contains(t, results[0].Message, "ended")
}

func TestCheckPolicyActivePeriod_OpenEnded(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID: "policies/nist",
		Timeline: &policy.PolicyTimeline{
			EvaluationStart: "2025-01-01",
		},
	}

	results := CheckPolicyActivePeriod(cfg, resolver, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusPass, results[0].Status)
	assert.Contains(t, results[0].Message, "open-ended")
}

func TestCheckPolicyActivePeriod_Verbose_ShowsEnforcement(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID: "policies/nist",
		Timeline: &policy.PolicyTimeline{
			EvaluationStart:  "2025-01-01",
			EvaluationEnd:    "2099-12-31",
			EvaluationNotes:  "Annual review",
			EnforcementStart: "2025-06-01",
			EnforcementEnd:   "2099-12-31",
			EnforcementNotes: "Quarterly enforcement",
		},
	}

	results := CheckPolicyActivePeriod(cfg, resolver, true)

	if len(results) != 1 {
		t.Fatalf("expected 1 active-period result, got %d", len(results))
	}
	activePeriod := results[0]
	require.GreaterOrEqual(t, len(activePeriod.Children), 2, "expected at least 2 detail children in verbose mode")

	foundEvalNotes := false
	foundEnforcementDetail := false
	foundEnfNotes := false
	for _, child := range activePeriod.Children {
		if strings.Contains(child.Message, "Annual review") {
			foundEvalNotes = true
		}
		if strings.Contains(child.Message, "enforcement") && strings.Contains(child.Message, "active") {
			foundEnforcementDetail = true
		}
		if strings.Contains(child.Message, "Quarterly enforcement") {
			foundEnfNotes = true
		}
	}
	assert.True(t, foundEvalNotes, "expected verbose detail showing evaluation notes")
	assert.True(t, foundEnforcementDetail, "expected verbose detail showing enforcement timeline status")
	assert.True(t, foundEnfNotes, "expected verbose detail showing enforcement notes")
}

func TestCheckPolicyActivePeriod_UnparseableDate(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID: "policies/nist",
		Timeline: &policy.PolicyTimeline{
			EvaluationStart: "not-a-date",
		},
	}

	results := CheckPolicyActivePeriod(cfg, resolver, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusWarn, results[0].Status)
	assert.Contains(t, results[0].Message, "unparseable")
}

func TestCheckPolicyActivePeriod_InvalidPolicyRef(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: ""}},
	}
	resolver := newMockPolicyGraphResolver()

	results := CheckPolicyActivePeriod(cfg, resolver, false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusFail, results[0].Status)
	assert.True(t, results[0].Blocking, "expected blocking for invalid policy reference")
	assert.Contains(t, results[0].Message, "invalid policy reference")
}

// --- CheckCache Tests ---

func TestCheckCache_EmptyPath(t *testing.T) {
	r := CheckCache("")
	assert.Equal(t, StatusFail, r.Status)
}

func TestCheckCache_MissingDir(t *testing.T) {
	r := CheckCache("/nonexistent/path/policies")
	assert.Equal(t, StatusFail, r.Status)
}

func TestCheckCache_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	r := CheckCache(tmpDir)
	assert.Equal(t, StatusFail, r.Status)
}

func TestCheckCache_WithEntries(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "some-policy"), 0755))
	r := CheckCache(tmpDir)
	assert.Equal(t, StatusPass, r.Status)
}

// --- CheckConfig Tests ---

func TestCheckConfig_MissingFile(t *testing.T) {
	r := CheckConfig("/nonexistent/complytime.yaml")
	assert.Equal(t, StatusFail, r.Status)
}

// --- Helper Tests ---

func TestCountResolved(t *testing.T) {
	vars := map[string]string{"a": "1", "b": "2"}
	resolved, total := countResolved([]string{"a", "c"}, vars)
	assert.Equal(t, 2, total)
	assert.Equal(t, 1, resolved)
}

func TestJoinNames(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b", "c"}, "a, b, c"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, joinNames(tt.input))
	}
}

// --- CheckComplypacks Tests ---

// seedComplypackCache creates a fake complypack cache entry at
// {cacheDir}/complypacks/{evaluatorID}/{version}/ with both content.tar.gz
// and config.json so that LookupByEvaluatorID finds and parses it.
func seedComplypackCache(t *testing.T, cacheDir, evaluatorID, version string) {
	t.Helper()
	dir := filepath.Join(cacheDir, complytime.ComplypacksSubdir, evaluatorID, version)
	require.NoError(t, os.MkdirAll(dir, 0755))
	contentPath := filepath.Join(dir, "content.tar.gz")
	require.NoError(t, os.WriteFile(contentPath, []byte("fake-content"), 0600))
	cfg := map[string]string{
		"evaluator-id": evaluatorID,
		"version":      version,
	}
	cfgData, err := json.Marshal(cfg)
	require.NoError(t, err)
	configPath := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(configPath, cfgData, 0600))
}

func TestCheckComplypacks_AllPresent(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
		Complypacks: []complytime.PolicyEntry{
			{URL: "reg.io/complypacks/openscap:v1.0.0"},
		},
	}

	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID:    "policies/nist",
		EvaluatorID: "openscap",
	}

	seedComplypackCache(t, tmpDir, "openscap", "v1.0.0")

	results := CheckComplypacks(cfg, tmpDir, tmpDir, resolver)
	require.GreaterOrEqual(t, len(results), 1)

	foundComplypack := false
	foundCacheSize := false
	for _, r := range results {
		if r.Name == "complypacks/openscap" && r.Status == StatusPass {
			foundComplypack = true
		}
		if r.Name == "complypacks/cache-size" && r.Status == StatusPass {
			foundCacheSize = true
		}
	}
	assert.True(t, foundComplypack, "expected pass result for complypacks/openscap")
	assert.True(t, foundCacheSize, "expected cache-size result")
	assert.Equal(t, "complypacks/openscap", results[0].Name, "expected named complypack result")
	assert.Equal(t, GroupComplypacks, results[0].Group, "expected GroupComplypacks")
}

func TestCheckComplypacks_Missing(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
		Complypacks: []complytime.PolicyEntry{
			{URL: "reg.io/complypacks/openscap:v1.0.0"},
		},
	}

	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"
	resolver.graphs["policies/nist@v1.0.0"] = &policy.DependencyGraph{
		PolicyID:    "policies/nist",
		EvaluatorID: "openscap",
	}

	// Do NOT seed the cache — complypack is missing.
	results := CheckComplypacks(cfg, tmpDir, tmpDir, resolver)
	require.GreaterOrEqual(t, len(results), 1)

	foundMissing := false
	for _, r := range results {
		if r.Name == "complypacks/openscap" && r.Status == StatusWarn {
			foundMissing = true
			if !strings.Contains(r.Message, "complyctl get") {
				t.Errorf("expected 'complyctl get' suggestion in message, got %q", r.Message)
			}
		}
	}
	assert.True(t, foundMissing, "expected warn about missing openscap complypack")
	assert.Equal(t, "complypacks/openscap", results[0].Name, "expected evaluator-id in name")
}

// --- Doctor cache health helper tests ---

// TestWalkCacheSize_KnownFiles verifies that walkCacheSize returns the
// expected byte count for a cache with known-size files.
func TestWalkCacheSize_KnownFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a complypack cache directory with known-size files.
	evalDir := filepath.Join(tmpDir, "complypacks", "io.complytime.opa", "1.0.0")
	require.NoError(t, os.MkdirAll(evalDir, 0755))

	// Write files with known sizes.
	data100 := make([]byte, 100)
	data200 := make([]byte, 200)
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "content.tar.gz"), data100, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "config.json"), data200, 0600))

	size, err := walkCacheSize(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, int64(300), size, "expected 300 bytes total")
}

// TestWalkCacheSize_EmptyDir verifies walkCacheSize returns 0 for a
// non-existent complypacks directory.
func TestWalkCacheSize_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	size, err := walkCacheSize(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, int64(0), size, "expected 0 bytes for non-existent dir")
}

// TestFindOrphanedVersions_OrphanDetected verifies FR-006: when state.json
// tracks v2.0.0 only and v1.0.0 and v2.0.0 directories exist, v1.0.0 is
// identified as orphaned.
func TestFindOrphanedVersions_OrphanDetected(t *testing.T) {
	tmpDir := t.TempDir()

	evalID := "io.complytime.opa"
	// Create v1.0.0 and v2.0.0 directories.
	for _, v := range []string{"1.0.0", "2.0.0"} {
		dir := filepath.Join(tmpDir, "complypacks", evalID, v)
		require.NoError(t, os.MkdirAll(dir, 0755))
	}

	// State only tracks v2.0.0.
	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{
			"repo/opa": {
				EvaluatorID: evalID,
				Version:     "2.0.0",
			},
		},
	}

	orphans := findOrphanedVersions(tmpDir, state)

	require.Len(t, orphans, 1, "expected 1 orphaned version")
	assert.Equal(t, evalID, orphans[0].EvaluatorID)
	assert.Equal(t, "1.0.0", orphans[0].Version)
	assert.False(t, orphans[0].Untracked, "should be orphaned, not untracked")
}

// TestFindOrphanedVersions_NoOrphans verifies that when all on-disk versions
// are tracked in state, no orphans are returned.
func TestFindOrphanedVersions_NoOrphans(t *testing.T) {
	tmpDir := t.TempDir()

	evalID := "io.complytime.opa"
	dir := filepath.Join(tmpDir, "complypacks", evalID, "2.0.0")
	require.NoError(t, os.MkdirAll(dir, 0755))

	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{
			"repo/opa": {
				EvaluatorID: evalID,
				Version:     "2.0.0",
			},
		},
	}

	orphans := findOrphanedVersions(tmpDir, state)
	assert.Empty(t, orphans, "expected no orphans when all versions are tracked")
}

// TestFindOrphanedVersions_UntrackedWithNilState verifies FR-006: when
// state is nil, version directories are reported as "untracked" (not
// "orphaned") with Untracked=true.
func TestFindOrphanedVersions_UntrackedWithNilState(t *testing.T) {
	tmpDir := t.TempDir()

	evalID := "io.complytime.opa"
	for _, v := range []string{"1.0.0", "2.0.0"} {
		dir := filepath.Join(tmpDir, "complypacks", evalID, v)
		require.NoError(t, os.MkdirAll(dir, 0755))
	}

	// nil state — all versions should be untracked.
	orphans := findOrphanedVersions(tmpDir, nil)

	require.Len(t, orphans, 2, "expected 2 untracked versions")
	for _, o := range orphans {
		assert.True(t, o.Untracked,
			"version %s should be untracked, not orphaned", o.Version)
	}
}

// TestFindOrphanedVersions_UntrackedWithEmptyState verifies that when
// state has an empty Complypacks map, versions are reported as untracked.
func TestFindOrphanedVersions_UntrackedWithEmptyState(t *testing.T) {
	tmpDir := t.TempDir()

	evalID := "io.complytime.opa"
	dir := filepath.Join(tmpDir, "complypacks", evalID, "1.0.0")
	require.NoError(t, os.MkdirAll(dir, 0755))

	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{},
	}

	orphans := findOrphanedVersions(tmpDir, state)

	require.Len(t, orphans, 1, "expected 1 untracked version")
	assert.True(t, orphans[0].Untracked, "should be untracked with empty state")
}

func TestCheckComplypacks_NilConfig(t *testing.T) {
	results := CheckComplypacks(nil, "/tmp", "/tmp", newMockPolicyGraphResolver())
	assert.Nil(t, results)
}

func TestCheckComplypacks_NoComplypacks(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
	}
	results := CheckComplypacks(cfg, "/tmp", "/tmp", newMockPolicyGraphResolver())
	assert.Nil(t, results)
}

func TestCheckComplypacks_InvalidPolicyRef(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: ""},
		},
		Complypacks: []complytime.PolicyEntry{
			{URL: "reg.io/complypacks/openscap:v1.0.0"},
		},
	}

	resolver := newMockPolicyGraphResolver()

	results := CheckComplypacks(cfg, tmpDir, tmpDir, resolver)
	require.GreaterOrEqual(t, len(results), 1)

	foundFail := false
	for _, r := range results {
		if r.Status == StatusFail && strings.Contains(r.Message, "invalid policy reference") {
			foundFail = true
			assert.True(t, r.Blocking, "expected blocking for invalid policy reference")
		}
	}
	assert.True(t, foundFail, "expected a StatusFail result for invalid policy reference")
}

// --- CheckDirectoryLayout Tests ---

func TestCheckDirectoryLayout_BothDirsAccessible(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	result := CheckDirectoryLayout(cacheDir, dataDir)
	assert.Equal(t, StatusPass, result.Status)
	assert.Equal(t, "directory-layout", result.Name)
	assert.Contains(t, result.Message, "XDG directory layout valid")
}

func TestCheckDirectoryLayout_CacheDirMissing(t *testing.T) {
	dataDir := t.TempDir()

	result := CheckDirectoryLayout("/nonexistent/cache/dir", dataDir)
	assert.Equal(t, StatusWarn, result.Status)
	assert.Equal(t, "directory-layout", result.Name)
	assert.Contains(t, result.Message, "cache directory does not exist")
}

func TestCheckDirectoryLayout_DataDirMissing(t *testing.T) {
	cacheDir := t.TempDir()

	result := CheckDirectoryLayout(cacheDir, "/nonexistent/data/dir")
	assert.Equal(t, StatusWarn, result.Status)
	assert.Equal(t, "directory-layout", result.Name)
	assert.Contains(t, result.Message, "data directory does not exist")
	assert.Contains(t, result.Message, "run complyctl get to initialize")
}

func TestCheckDirectoryLayout_MisplacedStateJSON(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	// Place state.json in cache dir but NOT in data dir.
	cacheStatePath := filepath.Join(cacheDir, complytime.StateFileName)
	require.NoError(t, os.WriteFile(cacheStatePath, []byte(`{"policies":{}}`), 0600))

	result := CheckDirectoryLayout(cacheDir, dataDir)
	assert.Equal(t, StatusWarn, result.Status)
	assert.Equal(t, "directory-layout", result.Name)
	assert.Contains(t, result.Message, "state.json found in cache directory")
	assert.Contains(t, result.Message, "move it with: mv")
}

func TestCheckDirectoryLayout_StateInBothDirs(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	// Place state.json in both directories — not a misplacement warning.
	require.NoError(t, os.WriteFile(
		filepath.Join(cacheDir, complytime.StateFileName),
		[]byte(`{"policies":{}}`), 0600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dataDir, complytime.StateFileName),
		[]byte(`{"policies":{}}`), 0600,
	))

	result := CheckDirectoryLayout(cacheDir, dataDir)
	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, "XDG directory layout valid")
}

func TestCheckDirectoryLayout_StateOnlyInDataDir(t *testing.T) {
	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	// Place state.json only in data dir — correct layout.
	require.NoError(t, os.WriteFile(
		filepath.Join(dataDir, complytime.StateFileName),
		[]byte(`{"policies":{}}`), 0600,
	))

	result := CheckDirectoryLayout(cacheDir, dataDir)
	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, "XDG directory layout valid")
}

// --- attachByEvaluatorID Tests ---

func TestAttachByEvaluatorID_MatchesCorrectly(t *testing.T) {
	providers := []CheckResult{
		{Name: "provider/ampel", Group: GroupProviders, Status: StatusPass, Message: "healthy"},
		{Name: "provider/opa", Group: GroupProviders, Status: StatusPass, Message: "healthy"},
	}
	vars := []CheckResult{
		{Name: "variables/ampel", Group: GroupVariables, Status: StatusPass, Message: "1/1 global vars"},
		{Name: "variables/opa", Group: GroupVariables, Status: StatusFail, Message: "0/1 global vars"},
		{Name: "variables/ampel/detail", Group: GroupVariables, Status: StatusPass, Message: "detail"},
	}

	result, unmatched := attachByEvaluatorID(providers, vars)

	assert.Empty(t, unmatched)
	require.Len(t, result[0].Children, 2)
	require.Len(t, result[1].Children, 1)
	assert.Equal(t, "variables/ampel", result[0].Children[0].Name)
}

func TestAttachByEvaluatorID_UnmatchedPromoted(t *testing.T) {
	providers := []CheckResult{
		{Name: "provider/ampel", Group: GroupProviders, Status: StatusPass},
	}
	vars := []CheckResult{
		{Name: "variables/ampel", Group: GroupVariables, Status: StatusPass, Message: "matched"},
		{Name: "variables/orphan", Group: GroupVariables, Status: StatusWarn, Message: "orphaned"},
	}

	result, unmatched := attachByEvaluatorID(providers, vars)

	require.Len(t, unmatched, 1)
	assert.Equal(t, "variables/orphan", unmatched[0].Name)
	require.Len(t, result[0].Children, 1)
}

func TestAttachByEvaluatorID_NoProviders(t *testing.T) {
	vars := []CheckResult{
		{Name: "variables/ampel", Group: GroupVariables, Status: StatusPass},
	}

	result, unmatched := attachByEvaluatorID(nil, vars)

	assert.Empty(t, result)
	require.Len(t, unmatched, 1)
	assert.Equal(t, "variables/ampel", unmatched[0].Name)
}

// --- attachByPolicyID Tests ---

func TestAttachByPolicyID_MatchesCorrectly(t *testing.T) {
	policies := []CheckResult{
		{Name: "policy/cis", Group: GroupPolicies, Status: StatusPass, Message: "v1.0.0 (latest)"},
		{Name: "policy/nist", Group: GroupPolicies, Status: StatusWarn, Message: "stale"},
	}
	active := []CheckResult{
		{Name: "policy/cis/active-period", Group: GroupPolicies, Status: StatusPass, Message: "active"},
		{Name: "policy/nist/active-period", Group: GroupPolicies, Status: StatusWarn, Message: "expired"},
		{Name: "policy/cis/active-period/detail", Group: GroupPolicies, Status: StatusPass, Message: "notes"},
	}

	result, unmatched := attachByPolicyID(policies, active)

	assert.Empty(t, unmatched)
	require.Len(t, result[0].Children, 2)
	require.Len(t, result[1].Children, 1)
}

func TestAttachByPolicyID_UnmatchedPromoted(t *testing.T) {
	policies := []CheckResult{
		{Name: "policy/cis", Group: GroupPolicies, Status: StatusPass},
	}
	active := []CheckResult{
		{Name: "policy/cis/active-period", Group: GroupPolicies, Status: StatusPass, Message: "matched"},
		{Name: "policy/orphan/active-period", Group: GroupPolicies, Status: StatusWarn, Message: "orphaned"},
	}

	result, unmatched := attachByPolicyID(policies, active)

	require.Len(t, unmatched, 1)
	assert.Equal(t, "policy/orphan/active-period", unmatched[0].Name)
	require.Len(t, result[0].Children, 1)
}

func TestAttachByPolicyID_NoPolicies(t *testing.T) {
	active := []CheckResult{
		{Name: "policy/cis/active-period", Group: GroupPolicies, Status: StatusPass},
	}

	result, unmatched := attachByPolicyID(nil, active)

	assert.Empty(t, result)
	require.Len(t, unmatched, 1)
	assert.Equal(t, "policy/cis/active-period", unmatched[0].Name)
}

// --- GroupOrder Tests ---

func TestGroupOrder_ContainsExpectedGroups(t *testing.T) {
	order := GroupOrder()

	assert.Equal(t, []CheckGroup{
		GroupProviders,
		GroupPolicies,
		GroupCache,
		GroupComplypacks,
		GroupWorkspace,
		GroupVerify,
	}, order)
	// GroupVariables is intentionally excluded — variables are nested
	// under providers, not rendered as a standalone section.
	assert.NotContains(t, order, GroupVariables)
}

// --- extractID Tests ---

func TestExtractID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "two segments", input: "provider/ampel", expected: "ampel"},
		{name: "three segments", input: "policy/cis/active-period", expected: "cis"},
		{name: "single segment", input: "cache", expected: "cache"},
		{name: "empty string", input: "", expected: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, extractID(tc.input))
		})
	}
}

// --- formatBytes Tests ---

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{name: "zero bytes", input: 0, expected: "0 B"},
		{name: "sub-KB", input: 1023, expected: "1023 B"},
		{name: "exactly 1 KB", input: 1024, expected: "1.0 KB"},
		{name: "fractional KB", input: 3400, expected: "3.3 KB"},
		{name: "exactly 1 MB", input: 1048576, expected: "1.0 MB"},
		{name: "exactly 1 GB", input: 1073741824, expected: "1.0 GB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, formatBytes(tc.input))
		})
	}
}
