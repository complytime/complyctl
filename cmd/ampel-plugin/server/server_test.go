package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/cmd/ampel-plugin/convert"
	"github.com/complytime/complyctl/cmd/ampel-plugin/results"
	"github.com/complytime/complyctl/cmd/ampel-plugin/scan"
	"github.com/complytime/complyctl/cmd/ampel-plugin/toolcheck"
)

func TestMain(m *testing.M) {
	// Skip tool check for most tests since snappy/ampel may not be installed
	SkipToolCheck = true
	os.Exit(m.Run())
}

func makeTestPolicy() policy.Policy {
	return policy.Policy{
		{
			Rule: extensions.Rule{
				ID:          "require-pull-request",
				Description: "Require pull requests",
				Parameters: []extensions.Parameter{
					{ID: "require_pr", Value: "true"},
				},
			},
			Checks: []extensions.Check{
				{ID: "check-pr-required", Description: "Check PR required"},
			},
		},
	}
}

func makeTestAttestation() []byte {
	stmt := map[string]interface{}{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []map[string]interface{}{
			{
				"name": "test-subject",
				"digest": map[string]string{
					"sha256": "abc123def456",
				},
			},
		},
		"predicateType": "http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml",
		"predicate":     map[string]interface{}{},
	}
	data, _ := json.Marshal(stmt)
	return data
}

func setupServer(t *testing.T) (PluginServer, string) {
	t.Helper()
	dir := t.TempDir()
	s := New()
	err := s.Configure(context.Background(), map[string]string{
		"workspace": dir,
		"profile":   "test-profile",
	})
	require.NoError(t, err)
	return s, dir
}

func TestGenerate_ValidPolicy(t *testing.T) {
	s, dir := setupServer(t)
	err := s.Generate(context.Background(), makeTestPolicy())
	require.NoError(t, err)

	policyPath := filepath.Join(dir, "ampel", "policy", convert.PolicyFileName)
	data, err := os.ReadFile(policyPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "check-pr-required")
}

func TestGenerate_EmptyPolicy(t *testing.T) {
	s, dir := setupServer(t)
	err := s.Generate(context.Background(), policy.Policy{})
	require.NoError(t, err)

	policyPath := filepath.Join(dir, "ampel", "policy", convert.PolicyFileName)
	_, err = os.Stat(policyPath)
	require.True(t, os.IsNotExist(err), "no policy file should be created for empty input")
}

func TestGenerate_OverwritesExistingPolicy(t *testing.T) {
	s, dir := setupServer(t)

	p1 := policy.Policy{
		{
			Rule:   extensions.Rule{ID: "r1", Parameters: []extensions.Parameter{{ID: "p1", Value: "true"}}},
			Checks: []extensions.Check{{ID: "first-check", Description: "First"}},
		},
	}
	p2 := policy.Policy{
		{
			Rule:   extensions.Rule{ID: "r2", Parameters: []extensions.Parameter{{ID: "p2", Value: "true"}}},
			Checks: []extensions.Check{{ID: "second-check", Description: "Second"}},
		},
	}

	require.NoError(t, s.Generate(context.Background(), p1))
	require.NoError(t, s.Generate(context.Background(), p2))

	policyPath := filepath.Join(dir, "ampel", "policy", convert.PolicyFileName)
	data, err := os.ReadFile(policyPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "second-check")
	require.NotContains(t, string(data), "first-check")
}

func TestConfigure_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	s := New()
	err := s.Configure(context.Background(), map[string]string{
		"workspace": dir,
		"profile":   "test-profile",
	})
	require.NoError(t, err)
	require.Equal(t, dir, s.Config.Workspace)
	require.Equal(t, "test-profile", s.Config.Profile)
}

func TestConfigure_MissingWorkspace(t *testing.T) {
	s := New()
	err := s.Configure(context.Background(), map[string]string{
		"profile": "test-profile",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "workspace")
}

// mockScanRunner returns different outputs for snappy vs ampel calls.
type mockScanRunner struct {
	snappyOutput []byte
	ampelOutput  []byte
	err          error
}

func (m *mockScanRunner) Run(name string, args ...string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	if name == "snappy" {
		return m.snappyOutput, nil
	}
	return m.ampelOutput, nil
}

func setupServerWithTargets(t *testing.T) (PluginServer, string) {
	t.Helper()
	s, dir := setupServer(t)

	// Write a targets file
	targetsContent := `repositories:
  - url: https://github.com/myorg/repo1
    branches:
      - main
`
	targetsDir := filepath.Join(dir, "ampel")
	require.NoError(t, os.MkdirAll(targetsDir, 0750))
	require.NoError(t, os.WriteFile(
		filepath.Join(targetsDir, "ampel-targets.yaml"),
		[]byte(targetsContent), 0600,
	))

	// Write a policy file so paths exist
	require.NoError(t, s.Generate(context.Background(), makeTestPolicy()))

	return s, dir
}

func TestGetResults_ValidScan(t *testing.T) {
	s, dir := setupServerWithTargets(t)

	ampelOutput := results.AmpelVerifyOutput{
		PolicyID: "test",
		Passed:   true,
		Results: []results.AmpelVerifyResult{
			{TenetID: "check-pr-required", Title: "Check PR", Passed: true, Reason: "OK"},
		},
	}
	ampelData, err := json.Marshal(ampelOutput)
	require.NoError(t, err)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelData,
	}
	defer func() { ScanRunner = origRunner }()

	pvp, err := s.GetResults(context.Background(), makeTestPolicy())
	require.NoError(t, err)
	require.Len(t, pvp.ObservationsByCheck, 1)
	require.Equal(t, policy.ResultPass, pvp.ObservationsByCheck[0].Subjects[0].Result)

	// Verify per-repo result and attestation files were created
	resultsDir := filepath.Join(dir, "ampel", "results")
	files, err := os.ReadDir(resultsDir)
	require.NoError(t, err)
	require.Len(t, files, 2) // attestation + per-repo result
}

func TestGetResults_ScanError_ContinuesScanning(t *testing.T) {
	s, dir := setupServerWithTargets(t)

	// Write targets with two repos
	targetsContent := `repositories:
  - url: https://github.com/myorg/repo1
    branches:
      - main
  - url: https://github.com/myorg/repo2
    branches:
      - main
`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "ampel", "ampel-targets.yaml"),
		[]byte(targetsContent), 0600,
	))

	// Mock runner that fails for first repo's snappy call, succeeds for second
	callCount := 0
	ampelOutput := results.AmpelVerifyOutput{
		PolicyID: "test", Passed: true,
		Results: []results.AmpelVerifyResult{
			{TenetID: "check-1", Title: "Check", Passed: true, Reason: "OK"},
		},
	}
	ampelData, _ := json.Marshal(ampelOutput)

	origRunner := ScanRunner
	ScanRunner = &mockCallCountRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelData,
		failOnCall:   1,
		callCount:    &callCount,
	}
	defer func() { ScanRunner = origRunner }()

	pvp, err := s.GetResults(context.Background(), makeTestPolicy())
	require.NoError(t, err)
	// Should have 2 observations: one error, one pass
	require.Len(t, pvp.ObservationsByCheck, 2)
}

type mockCallCountRunner struct {
	snappyOutput []byte
	ampelOutput  []byte
	failOnCall   int
	callCount    *int
}

func (m *mockCallCountRunner) Run(name string, args ...string) ([]byte, error) {
	*m.callCount++
	// Fail on the snappy call for the first repo
	if *m.callCount <= 1 && m.failOnCall == 1 {
		return nil, fmt.Errorf("connection refused")
	}
	if name == "snappy" {
		return m.snappyOutput, nil
	}
	return m.ampelOutput, nil
}

func TestGetResults_MissingTargetsFile(t *testing.T) {
	s, _ := setupServer(t)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  []byte("{}"),
	}
	defer func() { ScanRunner = origRunner }()

	_, err := s.GetResults(context.Background(), makeTestPolicy())
	require.Error(t, err)
	require.Contains(t, err.Error(), "loading targets")
}

// Tool check integration tests
func TestGenerate_MissingToolReturnsError(t *testing.T) {
	s, _ := setupServer(t)

	origSkip := SkipToolCheck
	SkipToolCheck = false
	origTools := toolcheck.RequiredTools
	toolcheck.RequiredTools = []string{"nonexistent-ampel-tool-xyz"}
	defer func() {
		SkipToolCheck = origSkip
		toolcheck.RequiredTools = origTools
	}()

	err := s.Generate(context.Background(), makeTestPolicy())
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent-ampel-tool-xyz")
}

func TestGetResults_MissingToolReturnsError(t *testing.T) {
	s, _ := setupServer(t)

	origSkip := SkipToolCheck
	SkipToolCheck = false
	origTools := toolcheck.RequiredTools
	toolcheck.RequiredTools = []string{"nonexistent-ampel-tool-xyz"}
	defer func() {
		SkipToolCheck = origSkip
		toolcheck.RequiredTools = origTools
	}()

	_, err := s.GetResults(context.Background(), makeTestPolicy())
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent-ampel-tool-xyz")
}

func TestToolCheckError_IncludesToolName(t *testing.T) {
	s, _ := setupServer(t)

	origSkip := SkipToolCheck
	SkipToolCheck = false
	origTools := toolcheck.RequiredTools
	toolcheck.RequiredTools = []string{"missing-snappy-test", "missing-ampel-test"}
	defer func() {
		SkipToolCheck = origSkip
		toolcheck.RequiredTools = origTools
	}()

	err := s.Generate(context.Background(), makeTestPolicy())
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing-snappy-test")
	require.Contains(t, err.Error(), "missing-ampel-test")
	require.Contains(t, err.Error(), "PATH")
}

// Custom path configuration tests

func TestGenerate_CustomPolicyDir(t *testing.T) {
	dir := t.TempDir()
	s := New()
	err := s.Configure(context.Background(), map[string]string{
		"workspace":  dir,
		"profile":    "test-profile",
		"policy_dir": "custom-pol",
	})
	require.NoError(t, err)

	err = s.Generate(context.Background(), makeTestPolicy())
	require.NoError(t, err)

	policyPath := filepath.Join(dir, "ampel", "custom-pol", convert.PolicyFileName)
	data, err := os.ReadFile(policyPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "check-pr-required")
}

func TestGetResults_CustomResultsDir(t *testing.T) {
	dir := t.TempDir()
	s := New()
	err := s.Configure(context.Background(), map[string]string{
		"workspace":   dir,
		"profile":     "test-profile",
		"results_dir": "custom-res",
	})
	require.NoError(t, err)

	// Write targets file
	targetsContent := `repositories:
  - url: https://github.com/myorg/repo1
    branches:
      - main
`
	require.NoError(t, os.WriteFile(
		s.Config.TargetsFilePath(),
		[]byte(targetsContent), 0600,
	))

	// Generate policy first
	require.NoError(t, s.Generate(context.Background(), makeTestPolicy()))

	ampelOutput := results.AmpelVerifyOutput{
		PolicyID: "test",
		Passed:   true,
		Results: []results.AmpelVerifyResult{
			{TenetID: "check-pr-required", Title: "Check PR", Passed: true, Reason: "OK"},
		},
	}
	ampelData, err := json.Marshal(ampelOutput)
	require.NoError(t, err)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelData,
	}
	defer func() { ScanRunner = origRunner }()

	pvp, err := s.GetResults(context.Background(), makeTestPolicy())
	require.NoError(t, err)
	require.Len(t, pvp.ObservationsByCheck, 1)

	// Verify results are in custom dir (attestation + per-repo result)
	customResultsDir := filepath.Join(dir, "ampel", "custom-res")
	files, err := os.ReadDir(customResultsDir)
	require.NoError(t, err)
	require.Len(t, files, 2)
}

// Ensure unused imports are used
var _ = scan.ExecRunner{}
var _ = convert.PolicyFileName
