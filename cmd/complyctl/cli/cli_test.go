// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemaraproj/go-gemara"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/internal/terminal"
	"github.com/complytime/complyctl/internal/version"
	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complypack/pkg/complypack"
)

func chdirTemp(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	tmp := t.TempDir()
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func writeWorkspaceConfig(t *testing.T, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile("complytime.yaml", []byte(content), 0600))
}

const minimalConfig = `policies:
  - url: registry.example.com/policies/test-policy:v1.0
    id: test-policy
targets:
  - id: local
    policies:
      - test-policy
    variables:
      profile: test
`

// --- scanOptions tests ---

func TestScanOptions_Run_NoWorkspace(t *testing.T) {
	chdirTemp(t)
	o := &scanOptions{
		Common:  &Common{},
		timeout: 5 * time.Second,
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load workspace config")
}

func TestScanOptions_Run_NoTargets(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, `policies:
  - url: registry.example.com/policies/test:v1.0
targets: []
`)
	o := &scanOptions{
		Common:  &Common{},
		timeout: 5 * time.Second,
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no targets")
}

func TestScanOptions_Run_PolicyNotFound(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, minimalConfig)
	o := &scanOptions{
		Common:   &Common{},
		policyID: "nonexistent-policy",
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in config")
}

func TestScanOptions_Validate_InvalidFormat(t *testing.T) {
	o := &scanOptions{format: "xml"}
	err := o.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid format")
}

func TestScanOptions_Validate_ValidFormats(t *testing.T) {
	for _, f := range []string{"", "oscal", "pretty", "sarif"} {
		o := &scanOptions{format: f}
		assert.NoError(t, o.validate(), "format %q should be valid", f)
	}
}

// --- target resolution tests ---

// multiPolicyConfig has a target referencing two policies for disambiguation tests.
const multiPolicyConfig = `policies:
  - url: registry.example.com/policies/nist:v1.0
    id: nist
  - url: registry.example.com/policies/cis:v1.0
    id: cis
targets:
  - id: prod
    policies:
      - nist
      - cis
    variables:
      env: production
  - id: staging
    policies:
      - nist
    variables:
      env: staging
`

func TestResolveTarget_Found(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Targets: []complytime.TargetConfig{
			{ID: "prod", Policies: []string{"nist"}},
			{ID: "staging", Policies: []string{"nist"}},
		},
	}
	target, err := resolveTarget(cfg, "prod")
	require.NoError(t, err)
	assert.Equal(t, "prod", target.ID)
}

func TestResolveTarget_NotFound(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Targets: []complytime.TargetConfig{
			{ID: "prod", Policies: []string{"nist"}},
			{ID: "staging", Policies: []string{"nist"}},
		},
	}
	_, err := resolveTarget(cfg, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "prod")
	assert.Contains(t, err.Error(), "staging")
}

func TestResolvePolicy_BothTargetAndPolicyID_Valid(t *testing.T) {
	target := &complytime.TargetConfig{
		ID:       "prod",
		Policies: []string{"nist", "cis"},
	}
	policyID, err := resolvePolicy(target, "nist")
	require.NoError(t, err)
	assert.Equal(t, "nist", policyID)
}

func TestResolvePolicy_BothTargetAndPolicyID_Mismatch(t *testing.T) {
	target := &complytime.TargetConfig{
		ID:       "prod",
		Policies: []string{"nist"},
	}
	_, err := resolvePolicy(target, "cis")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not reference policy")
	assert.Contains(t, err.Error(), "nist")
}

func TestResolvePolicy_TargetOnly_SinglePolicy(t *testing.T) {
	target := &complytime.TargetConfig{
		ID:       "staging",
		Policies: []string{"nist"},
	}
	policyID, err := resolvePolicy(target, "")
	require.NoError(t, err)
	assert.Equal(t, "nist", policyID)
}

func TestResolvePolicy_TargetOnly_MultiplePolicies(t *testing.T) {
	target := &complytime.TargetConfig{
		ID:       "prod",
		Policies: []string{"nist", "cis"},
	}
	_, err := resolvePolicy(target, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple policies")
	assert.Contains(t, err.Error(), "nist")
	assert.Contains(t, err.Error(), "cis")
}

func TestResolvePolicy_TargetOnly_ZeroPolicies(t *testing.T) {
	target := &complytime.TargetConfig{
		ID:       "empty",
		Policies: []string{},
	}
	_, err := resolvePolicy(target, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no policies configured")
}

func TestResolvePolicy_NeitherTargetNorPolicyID(t *testing.T) {
	_, err := resolvePolicy(nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specify a target or --policy-id")
}

func TestResolvePolicy_PolicyIDOnly(t *testing.T) {
	policyID, err := resolvePolicy(nil, "nist")
	require.NoError(t, err)
	assert.Equal(t, "nist", policyID)
}

// TestScanOptions_Run_TargetNotFound verifies that specifying a nonexistent
// target produces an error listing available target IDs.
func TestScanOptions_Run_TargetNotFound(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, minimalConfig)
	o := &scanOptions{
		Common:  &Common{},
		target:  "nonexistent",
		timeout: 5 * time.Second,
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "local")
}

// TestScanOptions_Run_TargetPolicyMismatch verifies that specifying a target
// and a policy the target doesn't reference produces a mismatch error.
func TestScanOptions_Run_TargetPolicyMismatch(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, multiPolicyConfig)
	o := &scanOptions{
		Common:   &Common{},
		target:   "staging",
		policyID: "cis",
		timeout:  5 * time.Second,
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not reference policy")
	assert.Contains(t, err.Error(), "nist")
}

// TestScanOptions_Run_TargetMultiplePoliciesNoPolicyID verifies that a target
// with multiple policies and no --policy-id produces an error listing policies.
func TestScanOptions_Run_TargetMultiplePoliciesNoPolicyID(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, multiPolicyConfig)
	o := &scanOptions{
		Common:  &Common{},
		target:  "prod",
		timeout: 5 * time.Second,
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple policies")
	assert.Contains(t, err.Error(), "nist")
	assert.Contains(t, err.Error(), "cis")
}

// TestScanOptions_Run_NoArgsNoPolicyID verifies that running scan with neither
// a target nor --policy-id produces a clear error.
func TestScanOptions_Run_NoArgsNoPolicyID(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, minimalConfig)
	o := &scanOptions{
		Common:  &Common{},
		timeout: 5 * time.Second,
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specify a target or --policy-id")
}

// TestScanOptions_Run_TargetSinglePolicyInferred verifies that a target with
// exactly one policy infers the policy ID (proceeds past resolution to
// policy lookup, which fails because the cache is empty — that's expected).
func TestScanOptions_Run_TargetSinglePolicyInferred(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, multiPolicyConfig)
	o := &scanOptions{
		Common:   &Common{},
		target:   "staging",
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	// The resolution succeeds (infers "nist") but the scan fails later
	// because the policy is not cached. We verify it gets past resolution.
	err := o.run(context.Background())
	require.Error(t, err)
	// Should NOT contain resolution errors — it should fail at a later stage.
	assert.NotContains(t, err.Error(), "specify a target or --policy-id")
	assert.NotContains(t, err.Error(), "multiple policies")
	assert.NotContains(t, err.Error(), "not found in complytime.yaml")
}

// TestScanOptions_Run_PolicyIDOnlyBackwardCompat verifies that the existing
// --policy-id-only flow still works (backward compatibility).
func TestScanOptions_Run_PolicyIDOnlyBackwardCompat(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, minimalConfig)
	o := &scanOptions{
		Common:   &Common{},
		policyID: "test-policy",
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	// Resolution succeeds, scan fails later at cache lookup — expected.
	err := o.run(context.Background())
	require.Error(t, err)
	// Should NOT contain resolution errors.
	assert.NotContains(t, err.Error(), "specify a target or --policy-id")
	assert.NotContains(t, err.Error(), "not found in complytime.yaml")
}

// TestScanOptions_Run_TargetWithPolicyID verifies that specifying both a valid
// target and a matching policy-id proceeds past resolution.
func TestScanOptions_Run_TargetWithPolicyID(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, multiPolicyConfig)
	o := &scanOptions{
		Common:   &Common{},
		target:   "prod",
		policyID: "nist",
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.Error(t, err)
	// Should NOT contain resolution errors.
	assert.NotContains(t, err.Error(), "specify a target or --policy-id")
	assert.NotContains(t, err.Error(), "does not reference policy")
	assert.NotContains(t, err.Error(), "not found in complytime.yaml")
}

func TestFilterTargetByID_Found(t *testing.T) {
	targets := []complytime.TargetConfig{
		{ID: "prod", Policies: []string{"nist"}},
		{ID: "staging", Policies: []string{"nist"}},
	}
	result := filterTargetByID(targets, "prod")
	require.Len(t, result, 1)
	assert.Equal(t, "prod", result[0].ID)
}

func TestFilterTargetByID_NotFound(t *testing.T) {
	targets := []complytime.TargetConfig{
		{ID: "prod", Policies: []string{"nist"}},
		{ID: "staging", Policies: []string{"cis"}},
	}
	result := filterTargetByID(targets, "nonexistent")
	require.Len(t, result, 2, "returns original slice when ID not found")
	assert.Equal(t, "prod", result[0].ID)
	assert.Equal(t, "staging", result[1].ID)
}

func TestCompleteTargetIDs_NoConfig(t *testing.T) {
	chdirTemp(t)
	ids, directive := completeTargetIDs(nil, nil, "")
	assert.Nil(t, ids)
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
}

func TestCompleteTargetIDs_WithConfig(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, multiPolicyConfig)
	ids, directive := completeTargetIDs(nil, nil, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Contains(t, ids, "prod")
	assert.Contains(t, ids, "staging")
}

func TestCompleteTargetIDs_AlreadyHasArg(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, multiPolicyConfig)
	ids, directive := completeTargetIDs(nil, []string{"prod"}, "")
	assert.Nil(t, ids, "should not complete when arg already provided")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
}

// --- generateOptions tests ---

func TestGenerateOptions_Run_NoWorkspace(t *testing.T) {
	chdirTemp(t)
	o := &generateOptions{
		Common:  &Common{},
		timeout: 5 * time.Second,
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load workspace config")
}

func TestGenerateOptions_Run_PolicyNotFound(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, minimalConfig)
	o := &generateOptions{
		Common:   &Common{},
		policyID: "nonexistent",
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in config")
}

// --- providersOptions tests (complete) ---

// TestProvidersOptions_Complete verifies that complete() populates cacheDir
// and providerDir from the user's home directory.
func TestProvidersOptions_Complete(t *testing.T) {
	o := &providersOptions{Common: &Common{}}
	err := o.complete()
	require.NoError(t, err)
	assert.NotEmpty(t, o.providerDir, "providerDir should be populated")
	assert.NotEmpty(t, o.cacheDir, "cacheDir should be populated")
}

// TestProvidersOptions_Complete_CacheDirPreset verifies that complete() does
// not overwrite cacheDir when it is already set.
func TestProvidersOptions_Complete_CacheDirPreset(t *testing.T) {
	preset := t.TempDir()
	o := &providersOptions{Common: &Common{}, cacheDir: preset}
	err := o.complete()
	require.NoError(t, err)
	assert.Equal(t, preset, o.cacheDir, "cacheDir should not be overwritten when preset")
	assert.NotEmpty(t, o.providerDir, "providerDir should be populated")
}

// --- generateForAllTargets tests ---

// TestGenerateForAllTargets_EmptyGroups verifies that generateForAllTargets
// returns nil when no evaluator groups are provided.
func TestGenerateForAllTargets_EmptyGroups(t *testing.T) {
	cacheDir := t.TempDir()
	groups := map[string]policy.EvaluatorGroup{}
	targets := []complytime.TargetConfig{{ID: "local"}}
	err := generateForAllTargets(context.Background(), cacheDir, nil, groups, targets, nil)
	assert.NoError(t, err)
}

// --- getOptions tests ---

func TestGetOptions_Run_NoWorkspace(t *testing.T) {
	chdirTemp(t)
	o := &getOptions{
		Common:   &Common{},
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load workspace config")
}

// TestGetOptions_Run_WithComplypacks verifies that run() exercises the
// syncComplypacks code path when complypacks are configured. The sync
// itself fails (no registry) but the code path through run() is covered.
func TestGetOptions_Run_WithComplypacks(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, `policies:
  - url: registry.example.com/policies/test-policy:v1.0
    id: test-policy
complypacks:
  - url: registry.example.com/packs/test-pack:v1.0
    id: test-pack
targets:
  - id: local
    policies:
      - test-policy
`)
	o := &getOptions{
		Common:   &Common{},
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	// The run will fail during policy sync (no registry), but the code
	// path through run() including the syncComplypacks call is exercised.
	err := o.run(context.Background())
	require.Error(t, err)
	// Verify it fails on sync, not on config loading or complypacks parsing.
	assert.Contains(t, err.Error(), "registry")
}

// --- listOptions tests ---

func TestListOptions_Run_NoWorkspace(t *testing.T) {
	chdirTemp(t)
	o := &listOptions{
		Common:   &Common{Output: Output{Out: &bytes.Buffer{}}},
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load workspace config")
}

func TestListOptions_Run_ValidWorkspace(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, minimalConfig)
	var buf bytes.Buffer
	o := &listOptions{
		Common:   &Common{Output: Output{Out: &buf}},
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "POLICY ID")
}

// --- initOptions tests ---

func TestInitOptions_Run_AlreadyExists(t *testing.T) {
	chdirTemp(t)
	writeWorkspaceConfig(t, minimalConfig)
	o := &initOptions{Common: &Common{}}
	err := o.run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// --- providersOptions tests ---

func TestProvidersOptions_Run_EmptyProviderDir(t *testing.T) {
	o := &providersOptions{
		Common:      &Common{},
		providerDir: t.TempDir(),
		cacheDir:    t.TempDir(),
	}
	err := o.run(context.Background())
	require.NoError(t, err)
}

// TestProviders_WithComplypack verifies that the providers table shows the
// cached complypack version when a complypack is stored for a provider's
// evaluator-id.
func TestProviders_WithComplypack(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	// Store a complypack for the evaluator-id.
	cfg := complypack.Config{
		EvaluatorID: "io.complytime.opa",
		Version:     "1.2.3",
	}
	_, err := cc.Store(cfg, strings.NewReader("test content"))
	require.NoError(t, err)

	// Verify lookupComplypackVersion returns the stored version.
	version := lookupComplypackVersion(cc, "io.complytime.opa")
	assert.Equal(t, "1.2.3", version)

	// Verify the version appears in a rendered table row.
	// Build a row manually (simulating what buildProviderRows produces)
	// and render it through ShowPlainTable to confirm the COMPLYPACK column.
	headers := []string{"PROVIDER ID", "PATH", "STATUS", "VERSION", "COMPLYPACK"}
	rows := [][]string{
		{"io.complytime.opa", "complyctl-provider-opa", "healthy", "0.1.0", version},
	}
	var buf bytes.Buffer
	terminal.ShowPlainTable(&buf, headers, rows)
	output := buf.String()
	assert.Contains(t, output, "COMPLYPACK")
	assert.Contains(t, output, "1.2.3")
}

// TestProviders_WithoutComplypack verifies that the providers table shows
// "none" in the COMPLYPACK column when no complypack is cached for a
// provider's evaluator-id.
func TestProviders_WithoutComplypack(t *testing.T) {
	cacheDir := t.TempDir()
	cc := cache.NewComplypackCache(cacheDir)

	// No complypack stored — lookupComplypackVersion should return "none".
	version := lookupComplypackVersion(cc, "io.complytime.opa")
	assert.Equal(t, "none", version)

	// Verify "none" appears in a rendered table row.
	headers := []string{"PROVIDER ID", "PATH", "STATUS", "VERSION", "COMPLYPACK"}
	rows := [][]string{
		{"io.complytime.opa", "complyctl-provider-opa", "healthy", "0.1.0", version},
	}
	var buf bytes.Buffer
	terminal.ShowPlainTable(&buf, headers, rows)
	output := buf.String()
	assert.Contains(t, output, "COMPLYPACK")
	assert.Contains(t, output, "none")
}

// --- helper tests ---

func TestFilterTargetsForPolicy(t *testing.T) {
	targets := []complytime.TargetConfig{
		{ID: "t1", Policies: []string{"p1", "p2"}},
		{ID: "t2", Policies: []string{"p2"}},
		{ID: "t3", Policies: []string{"p3"}},
	}
	result := filterTargetsForPolicy(targets, "p2")
	require.Len(t, result, 2)
	assert.Equal(t, "t1", result[0].ID)
	assert.Equal(t, "t2", result[1].ID)
}

// --- checkOperationalErrors tests ---

func TestCheckOperationalErrors_NoErrors(t *testing.T) {
	assert.NoError(t, checkOperationalErrors(nil))
	assert.NoError(t, checkOperationalErrors([]string{}))
}

func TestCheckOperationalErrors_SingleError(t *testing.T) {
	err := checkOperationalErrors([]string{"target 'staging': clone failed"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 operational error —")
	assert.NotContains(t, err.Error(), "1 operational errors")
	assert.Contains(t, err.Error(), "some targets could not be evaluated")
}

func TestCheckOperationalErrors_MultipleErrors(t *testing.T) {
	err := checkOperationalErrors([]string{
		"target 'staging': clone failed",
		"target 'dev': missing tool",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 operational errors")
}

// --- reportOperationalWarnings tests ---

func TestReportOperationalWarnings_NoErrors(t *testing.T) {
	oldStderr := os.Stderr
	t.Cleanup(func() { os.Stderr = oldStderr })

	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	reportOperationalWarnings(nil)

	w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	assert.Empty(t, buf.String())
}

func TestReportOperationalWarnings_WithErrors(t *testing.T) {
	oldStderr := os.Stderr
	t.Cleanup(func() { os.Stderr = oldStderr })

	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	reportOperationalWarnings([]string{"target 'staging': clone failed"})

	w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	assert.Contains(t, buf.String(), "WARNING: 1 operational error during scan")
	assert.Contains(t, buf.String(), "clone failed")
}

// --- processScanOutput tests ---

func TestProcessScanOutput_NoErrors_ReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	scanOut := &scanOutput{
		assessments: []provider.AssessmentLog{{
			RequirementID: "req-1",
			Steps:         []provider.Step{{Name: "check", Result: provider.ResultPassed}},
		}},
		assessmentTargets: []string{"target-1"},
		errors:            nil,
	}
	policyTargets := []complytime.TargetConfig{{ID: "target-1"}}
	mappings := &resolvedMappings{
		reqToControl: map[string]string{"req-1": "ctrl-1"},
	}

	err = processScanOutput("", scanOut, "test-repo", mappings, policyTargets, "test-policy", []string{"target-1"}, tmpDir, true)
	assert.NoError(t, err)
}

func TestProcessScanOutput_WithErrors_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Suppress warnings to stderr during test.
	oldStderr := os.Stderr
	t.Cleanup(func() { os.Stderr = oldStderr })
	_, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stderr = w

	scanOut := &scanOutput{
		assessments: []provider.AssessmentLog{{
			RequirementID: "req-1",
			Steps:         []provider.Step{{Name: "check", Result: provider.ResultPassed}},
		}},
		assessmentTargets: []string{"target-1"},
		errors:            []string{"target 'staging': clone failed"},
	}
	policyTargets := []complytime.TargetConfig{{ID: "target-1"}}
	mappings := &resolvedMappings{
		reqToControl: map[string]string{"req-1": "ctrl-1"},
	}

	err = processScanOutput("", scanOut, "test-repo", mappings, policyTargets, "test-policy", []string{"target-1"}, tmpDir, true)
	w.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 operational error")
}

// --- extractPlanToReqMap tests ---

func TestExtractPlanToReqMap_NilGraph(t *testing.T) {
	m := extractPlanToReqMap(nil)
	assert.Empty(t, m)
}

func TestExtractPlanToReqMap_WithAssessments(t *testing.T) {
	graph := &policy.DependencyGraph{
		Assessments: []policy.Assessment{
			{ID: "ap-1", RequirementID: "req-1"},
			{ID: "ap-2", RequirementID: "req-2"},
			{ID: "ap-3", RequirementID: ""}, // empty requirement — should be skipped
		},
	}
	m := extractPlanToReqMap(graph)
	assert.Equal(t, "req-1", m["ap-1"])
	assert.Equal(t, "req-2", m["ap-2"])
	_, exists := m["ap-3"]
	assert.False(t, exists, "empty RequirementID should not be mapped")
}

func TestExtractPlanToReqMap_EmptyAssessments(t *testing.T) {
	graph := &policy.DependencyGraph{
		Assessments: []policy.Assessment{},
	}
	m := extractPlanToReqMap(graph)
	assert.Empty(t, m)
}

// --- resolveAssessmentIDs tests ---

func TestResolveAssessmentIDs_ReplacesMatchingIDs(t *testing.T) {
	assessments := []provider.AssessmentLog{
		{RequirementID: "ap-1"},
		{RequirementID: "req-already-correct"},
	}
	planToReq := map[string]string{"ap-1": "req-1"}
	resolveAssessmentIDs(assessments, planToReq)
	assert.Equal(t, "req-1", assessments[0].RequirementID)
	assert.Equal(t, "req-already-correct", assessments[1].RequirementID)
}

func TestResolveAssessmentIDs_EmptyMap(t *testing.T) {
	assessments := []provider.AssessmentLog{{RequirementID: "ap-1"}}
	resolveAssessmentIDs(assessments, map[string]string{})
	assert.Equal(t, "ap-1", assessments[0].RequirementID)
}

func TestResolveAssessmentIDs_NilAssessments(t *testing.T) {
	resolveAssessmentIDs(nil, map[string]string{"ap-1": "req-1"})
	// Should not panic
}

func TestResolveAssessmentIDs_AllReplaced(t *testing.T) {
	assessments := []provider.AssessmentLog{
		{RequirementID: "ap-1"},
		{RequirementID: "ap-2"},
	}
	planToReq := map[string]string{
		"ap-1": "req-1",
		"ap-2": "req-2",
	}
	resolveAssessmentIDs(assessments, planToReq)
	assert.Equal(t, "req-1", assessments[0].RequirementID)
	assert.Equal(t, "req-2", assessments[1].RequirementID)
}

// --- extractReqToControlMap tests ---

func TestExtractReqToControlMap_NilGraph(t *testing.T) {
	m := extractReqToControlMap(nil)
	assert.Empty(t, m)
}

func TestExtractReqToControlMap_WithControls(t *testing.T) {
	graph := &policy.DependencyGraph{
		Controls: []policy.Control{
			{
				Parsed: &gemara.ControlCatalog{
					Controls: []gemara.Control{
						{
							Id: "ctrl-1",
							AssessmentRequirements: []gemara.AssessmentRequirement{
								{Id: "req-1"},
								{Id: "req-2"},
							},
						},
					},
				},
			},
		},
	}
	m := extractReqToControlMap(graph)
	assert.Equal(t, "ctrl-1", m["req-1"])
	assert.Equal(t, "ctrl-1", m["req-2"])
}

// --- evaluatorArtifactsExist tests ---

func TestEvaluatorArtifactsExist_EmptySlice(t *testing.T) {
	chdirTemp(t)
	assert.True(t, evaluatorArtifactsExist(".", nil), "nil evaluator list should return true")
	assert.True(t, evaluatorArtifactsExist(".", []string{}), "empty evaluator list should return true")
}

func TestEvaluatorArtifactsExist_DirExistsWithFiles(t *testing.T) {
	baseDir := t.TempDir()
	evalDir := filepath.Join(baseDir, complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.MkdirAll(evalDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "policy.rego"), []byte("package test"), 0600))

	assert.True(t, evaluatorArtifactsExist(baseDir, []string{"ampel"}))
}

func TestEvaluatorArtifactsExist_DirMissing(t *testing.T) {
	baseDir := t.TempDir()
	assert.False(t, evaluatorArtifactsExist(baseDir, []string{"ampel"}))
}

func TestEvaluatorArtifactsExist_DirExistsButEmpty(t *testing.T) {
	baseDir := t.TempDir()
	evalDir := filepath.Join(baseDir, complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.MkdirAll(evalDir, 0750))

	assert.False(t, evaluatorArtifactsExist(baseDir, []string{"ampel"}))
}

func TestEvaluatorArtifactsExist_MultipleEvaluators_AllPresent(t *testing.T) {
	baseDir := t.TempDir()
	for _, id := range []string{"ampel", "openscap"} {
		evalDir := filepath.Join(baseDir, complytime.WorkspaceDir, id)
		require.NoError(t, os.MkdirAll(evalDir, 0750))
		require.NoError(t, os.WriteFile(filepath.Join(evalDir, "artifact.json"), []byte("{}"), 0600))
	}

	assert.True(t, evaluatorArtifactsExist(baseDir, []string{"ampel", "openscap"}))
}

func TestEvaluatorArtifactsExist_MultipleEvaluators_OneMissing(t *testing.T) {
	baseDir := t.TempDir()
	evalDir := filepath.Join(baseDir, complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.MkdirAll(evalDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "artifact.json"), []byte("{}"), 0600))

	assert.False(t, evaluatorArtifactsExist(baseDir, []string{"ampel", "openscap"}),
		"should return false when any evaluator directory is missing")
}

func TestEvaluatorArtifactsExist_PathIsFile(t *testing.T) {
	baseDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, complytime.WorkspaceDir), 0750))
	filePath := filepath.Join(baseDir, complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.WriteFile(filePath, []byte("not a dir"), 0600))

	assert.False(t, evaluatorArtifactsExist(baseDir, []string{"ampel"}),
		"should return false when path is a file, not a directory")
}

// --- needsRegeneration tests ---

func TestNeedsRegeneration_NilState(t *testing.T) {
	assert.True(t, needsRegeneration(".", nil, "sha256:abc", nil, "test-policy"),
		"nil generation state should require regeneration")
}

func TestNeedsRegeneration_StaleDigest(t *testing.T) {
	state := &policy.GenerationState{
		PolicyDigest: "sha256:old",
		EvaluatorIDs: []string{"ampel"},
	}
	assert.True(t, needsRegeneration(".", state, "sha256:new", nil, "test-policy"),
		"mismatched digest should require regeneration")
}

func TestNeedsRegeneration_FreshWithArtifacts(t *testing.T) {
	baseDir := t.TempDir()
	evalDir := filepath.Join(baseDir, complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.MkdirAll(evalDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "policy.rego"), []byte("package test"), 0600))

	state := &policy.GenerationState{
		PolicyDigest: "sha256:current",
		EvaluatorIDs: []string{"ampel"},
	}
	assert.False(t, needsRegeneration(baseDir, state, "sha256:current", nil, "test-policy"),
		"fresh digest with existing artifacts should not require regeneration")
}

func TestNeedsRegeneration_FreshButArtifactsMissing(t *testing.T) {
	baseDir := t.TempDir()
	// Do not create the evaluator directory — simulates deleted artifacts.
	state := &policy.GenerationState{
		PolicyDigest: "sha256:current",
		EvaluatorIDs: []string{"ampel"},
	}
	assert.True(t, needsRegeneration(baseDir, state, "sha256:current", nil, "test-policy"),
		"fresh digest but missing artifacts should require regeneration")
}

func TestNeedsRegeneration_ComplypackDigestChanged(t *testing.T) {
	baseDir := t.TempDir()
	evalDir := filepath.Join(baseDir, complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.MkdirAll(evalDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "policy.rego"), []byte("package test"), 0600))

	state := &policy.GenerationState{
		PolicyDigest:      "sha256:current",
		ComplypackDigests: map[string]string{"opa": "sha256:old-cp"},
		EvaluatorIDs:      []string{"ampel"},
	}
	assert.True(t, needsRegeneration(baseDir, state, "sha256:current",
		map[string]string{"opa": "sha256:new-cp"}, "test-policy"),
		"changed complypack digest should require regeneration")
}

func TestComplypackDigestsByEvaluator_Empty(t *testing.T) {
	state := &cache.State{
		Complypacks: make(map[string]cache.PolicyState),
	}
	result := complypackDigestsByEvaluator(state)
	assert.Empty(t, result)
}

func TestComplypackDigestsByEvaluator_SkipsIncomplete(t *testing.T) {
	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{
			"repo/missing-digest":    {EvaluatorID: "opa", Digest: ""},
			"repo/missing-evaluator": {EvaluatorID: "", Digest: "sha256:abc"},
		},
	}
	result := complypackDigestsByEvaluator(state)
	assert.Empty(t, result)
}

func TestComplypackDigestsByEvaluator_ExtractsDigests(t *testing.T) {
	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{
			"example.com/cp/opa":   {EvaluatorID: "io.complytime.opa", Digest: "sha256:aaa"},
			"example.com/cp/ampel": {EvaluatorID: "io.complytime.ampel", Digest: "sha256:bbb"},
		},
	}
	result := complypackDigestsByEvaluator(state)
	assert.Equal(t, map[string]string{
		"io.complytime.opa":   "sha256:aaa",
		"io.complytime.ampel": "sha256:bbb",
	}, result)
}

// --- lazyLogWriter tests ---

func TestLazyLogWriter_Write_CreatesLogFile(t *testing.T) {
	baseDir := t.TempDir()
	w := &lazyLogWriter{}
	w.SetWorkspace(baseDir)

	n, err := w.Write([]byte("test log line\n"))

	require.NoError(t, err)
	assert.Equal(t, 14, n)

	logPath := filepath.Join(baseDir, complytime.WorkspaceDir, complytime.LogFileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Equal(t, "test log line\n", string(content))

	require.NoError(t, w.Close())
}

func TestLazyLogWriter_Write_DefaultBaseDir(t *testing.T) {
	chdirTemp(t)
	w := &lazyLogWriter{}
	// No SetWorkspace call — should default to "."

	n, err := w.Write([]byte("default dir\n"))

	require.NoError(t, err)
	assert.Equal(t, 12, n)

	logPath := filepath.Join(".", complytime.WorkspaceDir, complytime.LogFileName)
	_, statErr := os.Stat(logPath)
	assert.NoError(t, statErr, "log file should exist in cwd .complytime/")

	require.NoError(t, w.Close())
}

func TestLazyLogWriter_Close_NilFile(t *testing.T) {
	w := &lazyLogWriter{}
	// Never written to — file is nil.
	assert.NoError(t, w.Close())
}

func TestLazyLogWriter_Write_MultipleWrites(t *testing.T) {
	baseDir := t.TempDir()
	w := &lazyLogWriter{}
	w.SetWorkspace(baseDir)

	_, err := w.Write([]byte("first\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte("second\n"))
	require.NoError(t, err)

	require.NoError(t, w.Close())

	logPath := filepath.Join(baseDir, complytime.WorkspaceDir, complytime.LogFileName)
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond\n", string(content))
}

// --- New() tests ---

func TestNew_ReturnsValidCommand(t *testing.T) {
	cmd := New()
	require.NotNil(t, cmd)
	assert.Equal(t, "complyctl [command]", cmd.Use)

	// Verify workspace flag is registered as a persistent flag.
	wsFlag := cmd.PersistentFlags().Lookup("workspace")
	require.NotNil(t, wsFlag, "workspace flag should be registered")
	assert.Equal(t, "w", wsFlag.Shorthand)

	// Verify subcommands are registered.
	subNames := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		subNames = append(subNames, sub.Name())
	}
	assert.Contains(t, subNames, "scan")
	assert.Contains(t, subNames, "init")
	assert.Contains(t, subNames, "doctor")
	assert.Contains(t, subNames, "get")
	assert.Contains(t, subNames, "generate")
	assert.Contains(t, subNames, "list")
	assert.Contains(t, subNames, "providers")
	assert.Contains(t, subNames, "version")
}

// TestNew_VersionFlagOutput verifies that `complyctl --version` produces
// the expected version output with all required fields.
func TestNew_VersionFlagOutput(t *testing.T) {
	cmd := New()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--version"})
	require.NoError(t, cmd.Execute())

	output := buf.String()
	require.NotEmpty(t, output, "--version flag should produce output")
	assert.Contains(t, output, "Version:")
	assert.Contains(t, output, "Go Version:")
	assert.Contains(t, output, "Git Commit:")
	assert.Contains(t, output, "Build Date:")
	assert.Contains(t, output, "Platform:")
}

// TestNew_VersionFlagMatchesSubcommand verifies that `complyctl --version`
// produces the same output as `complyctl version`.
func TestNew_VersionFlagMatchesSubcommand(t *testing.T) {
	// Generate the expected version string directly.
	var expected strings.Builder
	require.NoError(t, version.WriteVersion(&expected))

	// Run --version flag and capture its output.
	cmd := New()
	var flagOut bytes.Buffer
	cmd.SetOut(&flagOut)
	cmd.SetArgs([]string{"--version"})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, expected.String(), flagOut.String(),
		"--version flag output must match version.WriteVersion output")
}

// TestNew_VersionFlagRegistered verifies that the --version flag is registered
// on the root command.
func TestNew_VersionFlagRegistered(t *testing.T) {
	cmd := New()
	vFlag := cmd.Flags().Lookup("version")
	require.NotNil(t, vFlag, "--version flag should be registered on root command")
	assert.Equal(t, "false", vFlag.DefValue)
}

func TestNew_PersistentPreRunSetsWorkspace(t *testing.T) {
	baseDir := t.TempDir()
	cmd := New()
	require.NotNil(t, cmd.PersistentPreRun)
	// Set the workspace flag and invoke PersistentPreRun.
	require.NoError(t, cmd.PersistentFlags().Set("workspace", baseDir))
	cmd.PersistentPreRun(cmd, nil)
	// Verify the log writer workspace was set by writing to it.
	_, err := lw.Write([]byte("test\n"))
	require.NoError(t, err)

	// PostRun closes the file.
	require.NotNil(t, cmd.PersistentPostRun)
	cmd.PersistentPostRun(cmd, nil)
}

// --- workspace resolution in run methods ---

func TestScanOptions_Run_WithWorkspaceFlag(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "complytime.yaml"),
		[]byte(minimalConfig), 0o600))

	o := &scanOptions{
		Common:   &Common{Workspace: dir},
		policyID: "test-policy",
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.Error(t, err)
	// Should get past workspace resolution — fail at a later stage (cache).
	assert.NotContains(t, err.Error(), "failed to load workspace config")
}

func TestGenerateOptions_Run_WithWorkspaceFlag(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "complytime.yaml"),
		[]byte(minimalConfig), 0o600))

	o := &generateOptions{
		Common:   &Common{Workspace: dir},
		policyID: "test-policy",
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "failed to load workspace config")
}

func TestGetOptions_Run_WithWorkspaceFlag(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "complytime.yaml"),
		[]byte(minimalConfig), 0o600))

	o := &getOptions{
		Common:   &Common{Workspace: dir},
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.Error(t, err)
	// Should get past workspace resolution — fail at cache/registry stage.
	assert.NotContains(t, err.Error(), "failed to load workspace config")
}

func TestListOptions_Run_WithWorkspaceFlag(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "complytime.yaml"),
		[]byte(minimalConfig), 0o600))

	var buf bytes.Buffer
	o := &listOptions{
		Common:   &Common{Workspace: dir, Output: Output{Out: &buf}},
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "POLICY ID")
}

func TestInitOptions_Run_WithWorkspaceFlag_NoExistingConfig(t *testing.T) {
	dir := t.TempDir()
	o := &initOptions{Common: &Common{Workspace: dir}}

	// init calls promptPolicies which reads stdin — will get EOF immediately
	// and produce an empty config template.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })

	err = o.run()
	require.NoError(t, err)

	// Verify config was created in the workspace.
	configPath := filepath.Join(dir, ".complytime", "complytime.yaml")
	_, statErr := os.Stat(configPath)
	assert.NoError(t, statErr, "config file should be created in workspace")
}

func TestInitOptions_Run_WithWorkspaceFlag_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "complytime.yaml"),
		[]byte(minimalConfig), 0o600))

	o := &initOptions{Common: &Common{Workspace: dir}}
	err := o.run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// --- doctorCmd tests ---

func TestDoctorCmd_ReturnsCommand(t *testing.T) {
	common := &Common{}
	cmd := doctorCmd(common)
	require.NotNil(t, cmd)
	assert.Equal(t, "doctor", cmd.Use)

	// Verify verbose flag is registered.
	vFlag := cmd.Flags().Lookup("verbose")
	require.NotNil(t, vFlag)
}

func TestDoctorCmd_RunE_InvalidWorkspace(t *testing.T) {
	common := &Common{Workspace: "/nonexistent/path/xyz123"}
	cmd := doctorCmd(common)
	err := cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestDoctorCmd_RunE_ValidWorkspace(t *testing.T) {
	dir := t.TempDir()
	// Create a minimal config so doctor doesn't get nil config.
	configDir := filepath.Join(dir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "complytime.yaml"),
		[]byte("policies:\n  - url: registry.example.com/p:v1\ntargets: []"), 0o600))

	common := &Common{Workspace: dir}
	cmd := doctorCmd(common)
	// Doctor runs diagnostic checks and prints results. With a valid config
	// but no providers/cache it reports failures but does not error (non-blocking).
	err := cmd.RunE(cmd, nil)
	// May fail due to blocking checks (no providers found), which is expected.
	// The key assertion: it gets past workspace resolution successfully.
	if err != nil {
		assert.Contains(t, err.Error(), "blocking checks failed")
	}
}

// --- completeTargetIDs workspace resolution ---

func TestScanOptions_Run_InvalidWorkspacePath(t *testing.T) {
	o := &scanOptions{
		Common:  &Common{Workspace: "/nonexistent/path/xyz123"},
		timeout: 5 * time.Second,
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestGenerateOptions_Run_InvalidWorkspacePath(t *testing.T) {
	o := &generateOptions{
		Common:   &Common{Workspace: "/nonexistent/path/xyz123"},
		policyID: "test",
		timeout:  5 * time.Second,
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestGetOptions_Run_InvalidWorkspacePath(t *testing.T) {
	o := &getOptions{
		Common:  &Common{Workspace: "/nonexistent/path/xyz123"},
		timeout: 5 * time.Second,
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestListOptions_Run_InvalidWorkspacePath(t *testing.T) {
	var buf bytes.Buffer
	o := &listOptions{
		Common:   &Common{Workspace: "/nonexistent/path/xyz123", Output: Output{Out: &buf}},
		cacheDir: t.TempDir(),
	}
	err := o.run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

// --- injectWorkspaceIntoTargets tests ---

func TestInjectWorkspaceIntoTargets_InjectsWorkspace(t *testing.T) {
	targets := []complytime.TargetConfig{
		{ID: "prod", Variables: map[string]string{"profile": "cis"}},
		{ID: "staging", Variables: map[string]string{"profile": "stig"}},
	}
	result := injectWorkspaceIntoTargets(targets, "/resolved/workspace")

	for _, target := range result {
		assert.Equal(t, "/resolved/workspace", target.Variables[complytime.WorkspaceVarKey])
	}
	assert.Equal(t, "cis", result[0].Variables["profile"])
	assert.Equal(t, "stig", result[1].Variables["profile"])
}

func TestInjectWorkspaceIntoTargets_DoesNotMutateOriginal(t *testing.T) {
	targets := []complytime.TargetConfig{
		{ID: "prod", Variables: map[string]string{"profile": "cis"}},
	}
	_ = injectWorkspaceIntoTargets(targets, "/workspace")

	_, hasWorkspace := targets[0].Variables[complytime.WorkspaceVarKey]
	assert.False(t, hasWorkspace, "original target variables must not be mutated")
}

func TestInjectWorkspaceIntoTargets_NilVariables(t *testing.T) {
	targets := []complytime.TargetConfig{
		{ID: "prod"},
	}
	result := injectWorkspaceIntoTargets(targets, "/workspace")

	assert.Equal(t, "/workspace", result[0].Variables[complytime.WorkspaceVarKey])
}

func TestInjectWorkspaceIntoTargets_EmptySlice(t *testing.T) {
	result := injectWorkspaceIntoTargets(nil, "/workspace")
	assert.Empty(t, result)
}

func TestCompleteTargetIDs_WithWorkspaceConfig(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "complytime.yaml"),
		[]byte(multiPolicyConfig), 0o600))

	// completeTargetIDs uses ResolveWorkspaceDir(""), which falls back to cwd.
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ids, directive := completeTargetIDs(nil, nil, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Contains(t, ids, "prod")
	assert.Contains(t, ids, "staging")
}

// --- reverseMap tests ---

func TestReverseMap_Basic(t *testing.T) {
	m := map[string]string{"ap-1": "req-1", "ap-2": "req-2"}
	r := reverseMap(m)
	assert.Equal(t, "ap-1", r["req-1"])
	assert.Equal(t, "ap-2", r["req-2"])
	assert.Len(t, r, 2)
}

func TestReverseMap_Empty(t *testing.T) {
	r := reverseMap(map[string]string{})
	assert.Empty(t, r)
}

func TestReverseMap_Nil(t *testing.T) {
	r := reverseMap(nil)
	assert.Empty(t, r)
}

// --- buildReqToComplypackRef tests ---

func TestBuildReqToComplypackRef_EmptyGroups(t *testing.T) {
	m := buildReqToComplypackRef(t.TempDir(), map[string]policy.EvaluatorGroup{})
	assert.Empty(t, m)
}

func TestBuildReqToComplypackRef_NoCacheState(t *testing.T) {
	groups := map[string]policy.EvaluatorGroup{
		"opa": {
			EvaluatorID: "opa",
			Configs:     []provider.AssessmentConfiguration{{PlanID: "plan-1", RequirementID: "req-1"}},
		},
	}
	m := buildReqToComplypackRef(t.TempDir(), groups)
	assert.Empty(t, m)
}

func TestBuildReqToComplypackRef_ResolvesRequirements(t *testing.T) {
	cacheDir := t.TempDir()
	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{
			"registry.example.com/complypacks/opa": {
				Digest:      "sha256:abc123",
				EvaluatorID: "opa",
			},
			"registry.example.com/complypacks/kyverno": {
				Digest:      "sha256:def456",
				EvaluatorID: "kyverno",
			},
		},
	}
	require.NoError(t, cache.SaveState(state, cacheDir))

	groups := map[string]policy.EvaluatorGroup{
		"opa": {
			EvaluatorID: "opa",
			Configs: []provider.AssessmentConfiguration{
				{PlanID: "plan-1", RequirementID: "req-1"},
				{PlanID: "plan-2", RequirementID: "req-2"},
			},
		},
		"kyverno": {
			EvaluatorID: "kyverno",
			Configs: []provider.AssessmentConfiguration{
				{RequirementID: "req-3"},
			},
		},
	}
	m := buildReqToComplypackRef(cacheDir, groups)
	assert.Equal(t, "registry.example.com/complypacks/opa@sha256:abc123", m["plan-1"])
	assert.Equal(t, "registry.example.com/complypacks/opa@sha256:abc123", m["plan-2"])
	assert.Equal(t, "registry.example.com/complypacks/kyverno@sha256:def456", m["req-3"])
}

func TestBuildReqToComplypackRef_SkipsMissingDigest(t *testing.T) {
	cacheDir := t.TempDir()
	state := &cache.State{
		Complypacks: map[string]cache.PolicyState{
			"registry.example.com/complypacks/opa": {
				Digest:      "",
				EvaluatorID: "opa",
			},
		},
	}
	require.NoError(t, cache.SaveState(state, cacheDir))

	groups := map[string]policy.EvaluatorGroup{
		"opa": {
			EvaluatorID: "opa",
			Configs:     []provider.AssessmentConfiguration{{PlanID: "plan-1", RequirementID: "req-1"}},
		},
	}
	m := buildReqToComplypackRef(cacheDir, groups)
	assert.Empty(t, m)
}
