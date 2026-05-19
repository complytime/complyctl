// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemaraproj/go-gemara"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/pkg/provider"
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
  - url: registry.example.com/policies/test-policy@v1.0
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
  - url: registry.example.com/policies/test@v1.0
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

func TestScanOptions_Run_ExportEnabledWithoutCollector(t *testing.T) {
	chdirTemp(t)
	t.Setenv(complytime.ExportEnabledEnvVar, "true")
	writeWorkspaceConfig(t, minimalConfig)
	o := &scanOptions{
		Common:   &Common{},
		policyID: "test-policy",
		timeout:  5 * time.Second,
		cacheDir: t.TempDir(),
	}
	// The scan will fail before reaching export (no cache/providers),
	// but we verify the export trigger does not cause an early collector error.
	// The collector validation now happens in runExport(), not in run().
	err := o.run(context.Background())
	require.Error(t, err)
	// Should fail on cache/provider resolution, not on collector config.
	assert.NotContains(t, err.Error(), "collector")
}

// --- maybeExport env var parsing ---

func TestMaybeExport_EnvVarParsing(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		setEnv      bool
		wantExport  bool
		wantWarning bool
	}{
		{name: "unset", setEnv: false, wantExport: false},
		{name: "empty", envValue: "", setEnv: true, wantExport: false},
		{name: "true", envValue: "true", setEnv: true, wantExport: true},
		{name: "TRUE", envValue: "TRUE", setEnv: true, wantExport: true},
		{name: "True", envValue: "True", setEnv: true, wantExport: true},
		{name: "1", envValue: "1", setEnv: true, wantExport: true},
		{name: "t", envValue: "t", setEnv: true, wantExport: true},
		{name: "T", envValue: "T", setEnv: true, wantExport: true},
		{name: "false", envValue: "false", setEnv: true, wantExport: false},
		{name: "FALSE", envValue: "FALSE", setEnv: true, wantExport: false},
		{name: "0", envValue: "0", setEnv: true, wantExport: false},
		{name: "f", envValue: "f", setEnv: true, wantExport: false},
		{name: "yes", envValue: "yes", setEnv: true, wantExport: false, wantWarning: true},
		{name: "on", envValue: "on", setEnv: true, wantExport: false, wantWarning: true},
		{name: "enabled", envValue: "enabled", setEnv: true, wantExport: false, wantWarning: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(complytime.ExportEnabledEnvVar, tt.envValue)
			}

			// Capture stderr to check for warnings.
			// NOTE: os.Stderr swap is safe here because subtests run sequentially,
			// but this pattern should not be used in parallel tests.
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			o := &scanOptions{Common: &Common{}}
			err := o.maybeExport(context.Background(), &complytime.WorkspaceConfig{}, nil, nil)

			w.Close()
			var stderrBuf bytes.Buffer
			_, _ = stderrBuf.ReadFrom(r)
			os.Stderr = oldStderr

			if tt.wantExport {
				// When export is triggered with no collector config, we expect an error
				require.Error(t, err)
				assert.Contains(t, err.Error(), "collector")
			} else {
				assert.NoError(t, err)
			}

			if tt.wantWarning {
				assert.Contains(t, stderrBuf.String(), "not a recognized boolean value")
			}
		})
	}
}

// --- target resolution tests ---

// multiPolicyConfig has a target referencing two policies for disambiguation tests.
const multiPolicyConfig = `policies:
  - url: registry.example.com/policies/nist@v1.0
    id: nist
  - url: registry.example.com/policies/cis@v1.0
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

// --- export helpers ---

func TestAuthRequired_NilAuth(t *testing.T) {
	assert.False(t, authRequired(nil))
}

func TestAuthRequired_EmptyTokenEndpoint(t *testing.T) {
	assert.False(t, authRequired(&complytime.AuthConfig{}))
}

func TestAuthRequired_WithTokenEndpoint(t *testing.T) {
	assert.True(t, authRequired(&complytime.AuthConfig{TokenEndpoint: "https://idp.example.com/token"})) //nolint:gosec // test fixture
}

func TestValidateAuthCredentials_MissingClientID(t *testing.T) {
	err := validateAuthCredentials(&complytime.AuthConfig{ClientSecret: "s"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client-id")
}

func TestValidateAuthCredentials_MissingClientSecret(t *testing.T) {
	err := validateAuthCredentials(&complytime.AuthConfig{ClientID: "id"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client-secret")
}

func TestValidateAuthCredentials_Valid(t *testing.T) {
	err := validateAuthCredentials(&complytime.AuthConfig{ClientID: "id", ClientSecret: "s"})
	require.NoError(t, err)
}

func TestCountExportFailures_NoneSkipped(t *testing.T) {
	results := []exportResult{}
	assert.Equal(t, 0, countExportFailures(results))
}

func TestCountExportFailures_SkippedNotCounted(t *testing.T) {
	results := []exportResult{
		{skipped: true},
		{skipped: true},
	}
	assert.Equal(t, 0, countExportFailures(results))
}

func TestCountExportFailures_TransportError(t *testing.T) {
	results := []exportResult{
		{err: fmt.Errorf("connection refused")},
		{skipped: true},
	}
	assert.Equal(t, 1, countExportFailures(results))
}

func TestCountExportFailures_ResponseFailure(t *testing.T) {
	results := []exportResult{
		{response: &provider.ExportResponse{Success: false}},
		{response: &provider.ExportResponse{Success: true, FailedCount: 3}},
		{response: &provider.ExportResponse{Success: true, FailedCount: 0}},
	}
	assert.Equal(t, 2, countExportFailures(results))
}

func TestExportResponseStatus_Success(t *testing.T) {
	r := exportResult{
		providerID: "openscap",
		response:   &provider.ExportResponse{Success: true, ExportedCount: 5},
	}
	status, errMsg := exportResponseStatus(r)
	assert.Equal(t, complytime.StatusPassed, status)
	assert.Empty(t, errMsg)
}

func TestExportResponseStatus_FailedCount(t *testing.T) {
	r := exportResult{
		providerID: "openscap",
		response:   &provider.ExportResponse{Success: true, FailedCount: 2, ErrorMessage: "timeout"},
	}
	status, errMsg := exportResponseStatus(r)
	assert.Equal(t, complytime.StatusFailed, status)
	assert.Contains(t, errMsg, "openscap")
	assert.Contains(t, errMsg, "timeout")
}

func TestExportResponseStatus_SuccessFalse(t *testing.T) {
	r := exportResult{
		providerID: "openscap",
		response:   &provider.ExportResponse{Success: false, ErrorMessage: "not yet supported"},
	}
	status, errMsg := exportResponseStatus(r)
	assert.Equal(t, complytime.StatusFailed, status)
	assert.Contains(t, errMsg, "not yet supported")
}

func TestFormatExportSummary_MixedResults(t *testing.T) {
	results := []exportResult{
		{providerID: "plugin-a", skipped: true},
		{providerID: "plugin-b", err: fmt.Errorf("dial error")},
		{providerID: "plugin-c", response: &provider.ExportResponse{Success: true, ExportedCount: 10}},
		{providerID: "plugin-d", response: &provider.ExportResponse{Success: false, FailedCount: 2, ErrorMessage: "partial failure"}},
	}
	out := formatExportSummary(results)
	assert.Contains(t, out, "plugin-a")
	assert.Contains(t, out, "no export support")
	assert.Contains(t, out, "plugin-b")
	assert.Contains(t, out, "dial error")
	assert.Contains(t, out, "plugin-c")
	assert.Contains(t, out, "plugin-d")
	assert.Contains(t, out, "partial failure")
}

// --- scan help text ---

func TestScanCmd_HelpContainsExportEnvVar(t *testing.T) {
	cmd := scanCmd(&Common{})
	assert.Contains(t, cmd.Long, complytime.ExportEnabledEnvVar)
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
	}
	err := o.run(context.Background())
	require.NoError(t, err)
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
	reqToControl := map[string]string{"req-1": "ctrl-1"}

	err = processScanOutput("", scanOut, "test-repo", reqToControl, policyTargets, "test-policy", []string{"target-1"})
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
	reqToControl := map[string]string{"req-1": "ctrl-1"}

	err = processScanOutput("", scanOut, "test-repo", reqToControl, policyTargets, "test-policy", []string{"target-1"})
	w.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 operational error")
}

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
	assert.True(t, evaluatorArtifactsExist(nil), "nil evaluator list should return true")
	assert.True(t, evaluatorArtifactsExist([]string{}), "empty evaluator list should return true")
}

func TestEvaluatorArtifactsExist_DirExistsWithFiles(t *testing.T) {
	chdirTemp(t)
	evalDir := filepath.Join(".", complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.MkdirAll(evalDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "policy.rego"), []byte("package test"), 0600))

	assert.True(t, evaluatorArtifactsExist([]string{"ampel"}))
}

func TestEvaluatorArtifactsExist_DirMissing(t *testing.T) {
	chdirTemp(t)
	assert.False(t, evaluatorArtifactsExist([]string{"ampel"}))
}

func TestEvaluatorArtifactsExist_DirExistsButEmpty(t *testing.T) {
	chdirTemp(t)
	evalDir := filepath.Join(".", complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.MkdirAll(evalDir, 0750))

	assert.False(t, evaluatorArtifactsExist([]string{"ampel"}))
}

func TestEvaluatorArtifactsExist_MultipleEvaluators_AllPresent(t *testing.T) {
	chdirTemp(t)
	for _, id := range []string{"ampel", "openscap"} {
		evalDir := filepath.Join(".", complytime.WorkspaceDir, id)
		require.NoError(t, os.MkdirAll(evalDir, 0750))
		require.NoError(t, os.WriteFile(filepath.Join(evalDir, "artifact.json"), []byte("{}"), 0600))
	}

	assert.True(t, evaluatorArtifactsExist([]string{"ampel", "openscap"}))
}

func TestEvaluatorArtifactsExist_MultipleEvaluators_OneMissing(t *testing.T) {
	chdirTemp(t)
	evalDir := filepath.Join(".", complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.MkdirAll(evalDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "artifact.json"), []byte("{}"), 0600))

	assert.False(t, evaluatorArtifactsExist([]string{"ampel", "openscap"}),
		"should return false when any evaluator directory is missing")
}

func TestEvaluatorArtifactsExist_PathIsFile(t *testing.T) {
	chdirTemp(t)
	require.NoError(t, os.MkdirAll(complytime.WorkspaceDir, 0750))
	filePath := filepath.Join(".", complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.WriteFile(filePath, []byte("not a dir"), 0600))

	assert.False(t, evaluatorArtifactsExist([]string{"ampel"}),
		"should return false when path is a file, not a directory")
}

// --- needsRegeneration tests ---

func TestNeedsRegeneration_NilState(t *testing.T) {
	assert.True(t, needsRegeneration(nil, "sha256:abc", "test-policy"),
		"nil generation state should require regeneration")
}

func TestNeedsRegeneration_StaleDigest(t *testing.T) {
	state := &policy.GenerationState{
		PolicyDigest: "sha256:old",
		EvaluatorIDs: []string{"ampel"},
	}
	assert.True(t, needsRegeneration(state, "sha256:new", "test-policy"),
		"mismatched digest should require regeneration")
}

func TestNeedsRegeneration_FreshWithArtifacts(t *testing.T) {
	chdirTemp(t)
	evalDir := filepath.Join(".", complytime.WorkspaceDir, "ampel")
	require.NoError(t, os.MkdirAll(evalDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(evalDir, "policy.rego"), []byte("package test"), 0600))

	state := &policy.GenerationState{
		PolicyDigest: "sha256:current",
		EvaluatorIDs: []string{"ampel"},
	}
	assert.False(t, needsRegeneration(state, "sha256:current", "test-policy"),
		"fresh digest with existing artifacts should not require regeneration")
}

func TestNeedsRegeneration_FreshButArtifactsMissing(t *testing.T) {
	chdirTemp(t)
	// Do not create the evaluator directory — simulates deleted artifacts.
	state := &policy.GenerationState{
		PolicyDigest: "sha256:current",
		EvaluatorIDs: []string{"ampel"},
	}
	assert.True(t, needsRegeneration(state, "sha256:current", "test-policy"),
		"fresh digest but missing artifacts should require regeneration")
}
