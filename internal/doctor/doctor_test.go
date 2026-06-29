// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if results != nil {
		t.Errorf("expected nil results for nil config, got %d", len(results))
	}
}

func TestCheckPolicyVersions_NoPolicies(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{}
	results := CheckPolicyVersions(cfg, "/tmp", newMockVersionResolver())
	if results != nil {
		t.Errorf("expected nil results for empty policies, got %d", len(results))
	}
}

func TestCheckPolicyVersions_NilResolver(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	results := CheckPolicyVersions(cfg, "/tmp", nil)
	if results != nil {
		t.Errorf("expected nil results for nil resolver, got %d", len(results))
	}
}

func TestCheckPolicyVersions_PolicyAtLatest(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v1.0.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "(pinned)") {
		t.Errorf("expected '(pinned)' in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_UnpinnedAtLatest(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v1.0.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "(latest)") {
		t.Errorf("expected '(latest)' in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_PinnedMatchesCached_LatestDiffers(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v1.1.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass for pinned matching cached, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "(pinned") {
		t.Errorf("expected 'pinned' in message, got %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "latest available: v1.1.0") {
		t.Errorf("expected 'latest available' info in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_UnpinnedStale(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v1.1.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "cached v1.0.0") {
		t.Errorf("expected cached version in message, got %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "available v1.1.0") {
		t.Errorf("expected available version in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_PinnedMismatchCached(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v2.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v2.0.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn for pin mismatch, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "does not match configured pin") {
		t.Errorf("expected pin mismatch message, got %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "@v2.0.0") {
		t.Errorf("expected configured version in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_NotCached(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.versions["reg.io|policies/nist"] = "v1.0.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn, got %s", results[0].Status)
	}
	if !strings.Contains(results[0].Message, "not cached") {
		t.Errorf("expected 'not cached' in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_RegistryUnreachable(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "unreachable.io/policies/nist:v1.0.0"},
			{URL: "unreachable.io/policies/cis:v2.0.0", ID: "cis"},
		},
	}

	vr := newMockVersionResolver()
	vr.unreachable["unreachable.io"] = true

	state.UpdatePolicyState("policies/cis", "v2.0.0", "sha256:abc")
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	results := CheckPolicyVersions(cfg, tmpDir, vr)

	if len(results) != 1 {
		t.Fatalf("expected 1 result (registry warn), got %d: %+v", len(results), results)
	}
	if results[0].Name != "registry/unreachable.io" {
		t.Errorf("expected registry warning, got %q", results[0].Name)
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn, got %s", results[0].Status)
	}
	if !strings.Contains(results[0].Message, "connection refused") {
		t.Errorf("expected actual error in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_LatestMissing_PinnedResolves(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.latestMissing["reg.io"] = true
	vr.pinnedVersions["reg.io|policies/nist|v1.0.0"] = "v1.0.0"

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass for reachable registry with pinned version, got %s: %s",
			results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "pinned") {
		t.Errorf("expected 'pinned' in message, got %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "latest tag unavailable") {
		t.Errorf("expected 'latest tag unavailable' in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_LatestMissing_NoPinnedVersion(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist"},
		},
	}

	vr := newMockVersionResolver()
	vr.latestMissing["reg.io"] = true

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "latest tag not found") {
		t.Errorf("expected 'latest tag not found' in message, got %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "pin a specific version") {
		t.Errorf("expected guidance to pin version, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_LatestMissing_DoesNotPoisonSameRegistry(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/alpha": {Version: "v1.0.0"},
		"policies/beta":  {Version: "v2.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/alpha"},
			{URL: "reg.io/policies/beta"},
		},
	}

	vr := newMockVersionResolver()
	vr.latestMissing["reg.io"] = true

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (404 should not skip second policy), got %d: %+v", len(results), results)
	}
	for _, r := range results {
		if r.Status != StatusWarn {
			t.Errorf("expected warn for %s, got %s", r.Name, r.Status)
		}
		if !strings.Contains(r.Message, "latest tag not found") {
			t.Errorf("expected 'latest tag not found' in %s message, got %q", r.Name, r.Message)
		}
	}
}

func TestCheckPolicyVersions_PinnedBoth404(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v2.0.0"},
		},
	}

	vr := newMockVersionResolver()
	vr.latestMissing["reg.io"] = true
	// pinnedVersions NOT populated → ResolveVersion also fails

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "not found in registry") {
		t.Errorf("expected 'not found in registry' in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_MixedRegistries_Unreachable_And_404(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/alpha": {Version: "v1.0.0"},
		"policies/beta":  {Version: "v2.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

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
	if len(results) != 2 {
		t.Fatalf("expected 2 results (one per registry), got %d: %+v", len(results), results)
	}
	if results[0].Name != "registry/down.io" {
		t.Errorf("expected registry/down.io, got %q", results[0].Name)
	}
	if !strings.Contains(results[0].Message, "unreachable") {
		t.Errorf("expected 'unreachable' for down.io, got %q", results[0].Message)
	}
	if !strings.Contains(results[1].Message, "latest tag not found") {
		t.Errorf("expected '404' message for up.io, got %q", results[1].Message)
	}
}

func TestCheckPolicyVersions_PinnedNetworkFailure_BothFail(t *testing.T) {
	tmpDir := t.TempDir()

	state := &cache.State{Policies: map[string]cache.PolicyState{
		"policies/nist": {Version: "v1.0.0"},
		"policies/cis":  {Version: "v1.0.0"},
	}}
	if err := cache.SaveState(state, tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "flaky.io/policies/nist:v1.0.0"},
			{URL: "flaky.io/policies/cis:v1.0.0", ID: "cis"},
		},
	}

	vr := newMockVersionResolver()
	vr.unreachable["flaky.io"] = true

	results := CheckPolicyVersions(cfg, tmpDir, vr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (second policy skipped via unreachable), got %d: %+v", len(results), results)
	}
	if results[0].Name != "registry/flaky.io" {
		t.Errorf("expected registry-level warning, got %q", results[0].Name)
	}
	if !strings.Contains(results[0].Message, "unreachable") {
		t.Errorf("expected 'unreachable' in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyVersions_BadCacheState(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, complytime.StateFileName), []byte("{bad json"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}

	results := CheckPolicyVersions(cfg, tmpDir, newMockVersionResolver())
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn for bad cache state, got %s", results[0].Status)
	}
}

// --- CheckVariables Tests (refactored with summary + verbose) ---

func TestCheckVariables_NoHealthData(t *testing.T) {
	results := CheckVariables(nil, nil, nil, false)
	if results != nil {
		t.Errorf("expected nil, got %d results", len(results))
	}
}

func TestCheckVariables_NilConfig(t *testing.T) {
	health := []ProviderHealth{{EvaluatorID: "openscap"}}
	results := CheckVariables(nil, health, nil, false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusFail {
		t.Errorf("expected fail, got %s", results[0].Status)
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "1/1 global vars") {
		t.Errorf("expected count summary, got %q", results[0].Message)
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusFail {
		t.Errorf("expected fail, got %s", results[0].Status)
	}
	if !strings.Contains(results[0].Message, "0/2 global vars") {
		t.Errorf("expected count in message, got %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "output_dir") {
		t.Errorf("expected missing var name, got %q", results[0].Message)
	}
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

	summaryCount := 0
	detailCount := 0
	for _, r := range results {
		if strings.HasSuffix(r.Name, "/detail") {
			detailCount++
		} else {
			summaryCount++
		}
	}

	if summaryCount != 1 {
		t.Errorf("expected 1 summary result, got %d", summaryCount)
	}
	if detailCount != 2 {
		t.Errorf("expected 2 detail results (one per global var), got %d", detailCount)
	}

	foundPassed := false
	foundFailed := false
	for _, r := range results {
		if strings.Contains(r.Message, "output_dir") && strings.Contains(r.Message, complytime.StatusPassed) {
			foundPassed = true
		}
		if strings.Contains(r.Message, "scan_target") && strings.Contains(r.Message, complytime.StatusFailed) {
			foundFailed = true
		}
	}
	if !foundPassed {
		t.Error("expected verbose detail showing output_dir as passed")
	}
	if !foundFailed {
		t.Error("expected verbose detail showing scan_target as failed")
	}
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
		if strings.HasSuffix(r.Name, "/detail") {
			t.Errorf("did not expect detail results in non-verbose mode, got %q", r.Name)
		}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusFail {
		t.Errorf("expected fail for unmapped target vars, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "target vars not validated") {
		t.Errorf("expected 'target vars not validated' in message, got %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "no policy resolver") {
		t.Errorf("expected reason in message, got %q", results[0].Message)
	}
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
	// Expect 2 results: a resolve warning + the summary.
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d: %+v", len(results), results)
	}

	// First result: resolve failure warning.
	foundResolveWarn := false
	for _, r := range results {
		if r.Status == StatusWarn && strings.Contains(r.Name, "variables/resolve/") {
			foundResolveWarn = true
		}
	}
	if !foundResolveWarn {
		t.Error("expected a resolve failure warning result")
	}

	// Last result: summary with target vars not validated.
	last := results[len(results)-1]
	if last.Status != StatusFail {
		t.Errorf("expected fail, got %s: %s", last.Status, last.Message)
	}
	if !strings.Contains(last.Message, "target vars not validated") {
		t.Errorf("expected 'target vars not validated' in message, got %q", last.Message)
	}
	if !strings.Contains(last.Message, "policy graph unresolved") {
		t.Errorf("expected 'policy graph unresolved' in message, got %q", last.Message)
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass for unused evaluator, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "no target mapping") {
		t.Errorf("expected 'no target mapping' in message, got %q", results[0].Message)
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusFail {
		t.Errorf("expected fail for missing profile, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "profile") {
		t.Errorf("expected 'profile' in message, got %q", results[0].Message)
	}
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

	foundNotValidated := false
	for _, r := range results {
		if strings.Contains(r.Message, "profile") && strings.Contains(r.Message, "not validated") {
			foundNotValidated = true
		}
	}
	if !foundNotValidated {
		t.Error("expected verbose detail showing profile as not validated")
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass (workspace auto-injected), got %s: %s",
			results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "1/1 global vars") {
		t.Errorf("expected workspace counted as resolved, got %q", results[0].Message)
	}
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

	foundWorkspacePassed := false
	foundOutputDirFailed := false
	for _, r := range results {
		if strings.Contains(r.Message, complytime.WorkspaceVarKey) && strings.Contains(r.Message, complytime.StatusPassed) {
			foundWorkspacePassed = true
		}
		if strings.Contains(r.Message, "output_dir") && strings.Contains(r.Message, complytime.StatusFailed) {
			foundOutputDirFailed = true
		}
	}
	if !foundWorkspacePassed {
		t.Error("expected verbose detail showing workspace as passed (auto-injected)")
	}
	if !foundOutputDirFailed {
		t.Error("expected verbose detail showing output_dir as failed")
	}
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
	if !foundResolveWarn {
		t.Errorf("expected StatusWarn result for missing policy, results: %+v", results)
	}
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
	if !foundResolveWarn {
		t.Errorf("expected StatusWarn result for invalid policy reference, results: %+v", results)
	}
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

	// Resolver with no versions configured — ResolveVersion will fail.
	resolver := newMockPolicyGraphResolver()
	results := CheckVariables(cfg, health, resolver, false)

	foundResolveWarn := false
	for _, r := range results {
		if r.Status == StatusWarn && strings.Contains(r.Message, "cannot resolve version") {
			foundResolveWarn = true
		}
	}
	if !foundResolveWarn {
		t.Errorf("expected StatusWarn result for version resolution failure, results: %+v", results)
	}
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

	// Resolver with version but no graph — ResolvePolicyGraph will fail.
	resolver := newMockPolicyGraphResolver()
	resolver.versions["policies/nist@v1.0.0"] = "v1.0.0"

	results := CheckVariables(cfg, health, resolver, false)

	foundResolveWarn := false
	for _, r := range results {
		if r.Status == StatusWarn && strings.Contains(r.Message, "cannot resolve policy graph") {
			foundResolveWarn = true
		}
	}
	if !foundResolveWarn {
		t.Errorf("expected StatusWarn result for graph resolution failure, results: %+v", results)
	}
}

// --- CheckPolicyActivePeriod Tests ---

func TestCheckPolicyActivePeriod_NilConfig(t *testing.T) {
	results := CheckPolicyActivePeriod(nil, newMockPolicyGraphResolver(), false)
	if results != nil {
		t.Errorf("expected nil for nil config, got %d results", len(results))
	}
}

func TestCheckPolicyActivePeriod_NilResolver(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "reg.io/policies/nist:v1.0.0"}},
	}
	results := CheckPolicyActivePeriod(cfg, nil, false)
	if results != nil {
		t.Errorf("expected nil for nil resolver, got %d results", len(results))
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "no evaluation timeline") {
		t.Errorf("expected 'no evaluation timeline' in message, got %q", results[0].Message)
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "active") {
		t.Errorf("expected 'active' in message, got %q", results[0].Message)
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "begins") {
		t.Errorf("expected 'begins' in message, got %q", results[0].Message)
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "ended") {
		t.Errorf("expected 'ended' in message, got %q", results[0].Message)
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "open-ended") {
		t.Errorf("expected 'open-ended' in message, got %q", results[0].Message)
	}
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

	detailCount := 0
	for _, r := range results {
		if strings.HasSuffix(r.Name, "/detail") {
			detailCount++
		}
	}
	if detailCount < 2 {
		t.Errorf("expected at least 2 detail results in verbose mode, got %d", detailCount)
	}

	foundEvalNotes := false
	foundEnforcementDetail := false
	foundEnfNotes := false
	for _, r := range results {
		if strings.Contains(r.Message, "Annual review") {
			foundEvalNotes = true
		}
		if strings.Contains(r.Message, "enforcement") && strings.Contains(r.Message, "active") {
			foundEnforcementDetail = true
		}
		if strings.Contains(r.Message, "Quarterly enforcement") {
			foundEnfNotes = true
		}
	}
	if !foundEvalNotes {
		t.Error("expected verbose detail showing evaluation notes")
	}
	if !foundEnforcementDetail {
		t.Error("expected verbose detail showing enforcement timeline status")
	}
	if !foundEnfNotes {
		t.Error("expected verbose detail showing enforcement notes")
	}
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
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn for unparseable date, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "unparseable") {
		t.Errorf("expected 'unparseable' in message, got %q", results[0].Message)
	}
}

func TestCheckPolicyActivePeriod_InvalidPolicyRef(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: ""}},
	}
	resolver := newMockPolicyGraphResolver()

	results := CheckPolicyActivePeriod(cfg, resolver, false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusFail {
		t.Errorf("expected fail for invalid ref, got %s: %s", results[0].Status, results[0].Message)
	}
	if !results[0].Blocking {
		t.Error("expected blocking for invalid policy reference")
	}
	if !strings.Contains(results[0].Message, "invalid policy reference") {
		t.Errorf("expected 'invalid policy reference' in message, got %q", results[0].Message)
	}
}

// --- CheckCache Tests ---

func TestCheckCache_EmptyPath(t *testing.T) {
	r := CheckCache("")
	if r.Status != StatusFail {
		t.Errorf("expected fail for empty path, got %s", r.Status)
	}
}

func TestCheckCache_MissingDir(t *testing.T) {
	r := CheckCache("/nonexistent/path/policies")
	if r.Status != StatusFail {
		t.Errorf("expected fail for missing dir, got %s", r.Status)
	}
}

func TestCheckCache_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	r := CheckCache(tmpDir)
	if r.Status != StatusFail {
		t.Errorf("expected fail for empty dir, got %s", r.Status)
	}
}

func TestCheckCache_WithEntries(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDir, "some-policy"), 0755); err != nil {
		t.Fatal(err)
	}
	r := CheckCache(tmpDir)
	if r.Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", r.Status, r.Message)
	}
}

// --- CheckConfig Tests ---

func TestCheckConfig_MissingFile(t *testing.T) {
	r := CheckConfig("/nonexistent/complytime.yaml")
	if r.Status != StatusFail {
		t.Errorf("expected fail, got %s", r.Status)
	}
}

// --- Helper Tests ---

func TestCountResolved(t *testing.T) {
	vars := map[string]string{"a": "1", "b": "2"}
	resolved, total := countResolved([]string{"a", "c"}, vars)
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if resolved != 1 {
		t.Errorf("expected resolved 1, got %d", resolved)
	}
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
		got := joinNames(tt.input)
		if got != tt.expected {
			t.Errorf("joinNames(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- CheckComplypacks Tests ---

// seedComplypackCache creates a fake complypack cache entry at
// {cacheDir}/complypacks/{evaluatorID}/{version}/ with both content.tar.gz
// and config.json so that LookupByEvaluatorID finds and parses it.
func seedComplypackCache(t *testing.T, cacheDir, evaluatorID, version string) {
	t.Helper()
	dir := filepath.Join(cacheDir, complytime.ComplypacksSubdir, evaluatorID, version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	contentPath := filepath.Join(dir, "content.tar.gz")
	if err := os.WriteFile(contentPath, []byte("fake-content"), 0600); err != nil {
		t.Fatal(err)
	}
	// Write config.json — LookupByEvaluatorID now parses this alongside
	// content.tar.gz to return the complypack config.
	cfg := map[string]string{
		"evaluator-id": evaluatorID,
		"version":      version,
	}
	cfgData, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, cfgData, 0600); err != nil {
		t.Fatal(err)
	}
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

	// Seed the complypack cache so LookupByEvaluatorID finds it.
	seedComplypackCache(t, tmpDir, "openscap", "v1.0.0")

	results := CheckComplypacks(cfg, tmpDir, resolver)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if results[0].Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", results[0].Status, results[0].Message)
	}
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
	results := CheckComplypacks(cfg, tmpDir, resolver)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if results[0].Status != StatusWarn {
		t.Errorf("expected warn, got %s: %s", results[0].Status, results[0].Message)
	}
	if !strings.Contains(results[0].Message, "openscap") {
		t.Errorf("expected evaluator-id 'openscap' in message, got %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "complyctl get") {
		t.Errorf("expected 'complyctl get' suggestion in message, got %q", results[0].Message)
	}
}

func TestCheckComplypacks_NilConfig(t *testing.T) {
	results := CheckComplypacks(nil, "/tmp", newMockPolicyGraphResolver())
	if results != nil {
		t.Errorf("expected nil results for nil config, got %d", len(results))
	}
}

func TestCheckComplypacks_NoComplypacks(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "reg.io/policies/nist:v1.0.0"},
		},
		// Complypacks is nil/empty — check should be skipped.
	}
	results := CheckComplypacks(cfg, "/tmp", newMockPolicyGraphResolver())
	if results != nil {
		t.Errorf("expected nil results for empty complypacks, got %d", len(results))
	}
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

	results := CheckComplypacks(cfg, tmpDir, resolver)
	// Expect fail result for invalid ref; may also include a pass result
	// for the evaluator cache summary when no evaluators were resolved.
	if len(results) < 1 {
		t.Fatalf("expected at least 1 result, got %d: %+v", len(results), results)
	}

	foundFail := false
	for _, r := range results {
		if r.Status == StatusFail && strings.Contains(r.Message, "invalid policy reference") {
			foundFail = true
			if !r.Blocking {
				t.Error("expected blocking for invalid policy reference")
			}
		}
	}
	if !foundFail {
		t.Errorf("expected a StatusFail result for invalid policy reference, results: %+v", results)
	}
}
