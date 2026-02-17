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
				ID:          "SC-CODE-01.01",
				Description: "Validate branch protection settings require pull requests",
			},
			Checks: []extensions.Check{
				{ID: "check-SC-CODE-01.01", Description: "Verify pull request reviews are required"},
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

func makeAmpelResultAttestation() []byte {
	stmt := map[string]interface{}{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []map[string]interface{}{
			{
				"name":   "test-subject",
				"digest": map[string]string{"sha256": "abc123def456"},
			},
		},
		"predicateType": "https://carabiner.dev/ampel/resultset/v0",
		"predicate": map[string]interface{}{
			"status": "PASS",
			"results": []map[string]interface{}{
				{
					"status": "PASS",
					"policy": map[string]string{"id": "SC-CODE-01.01"},
					"eval_results": []map[string]interface{}{
						{
							"id":         "01",
							"status":     "PASS",
							"assessment": map[string]string{"message": "OK"},
						},
					},
					"meta": map[string]string{"description": "Check PR"},
				},
			},
		},
	}
	data, _ := json.Marshal(stmt)
	return data
}

// writeGranularPolicies creates granular policy files in the given directory
// so that Generate can load and match them.
func writeGranularPolicies(t *testing.T, dir string, policyIDs ...string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0750))
	for _, id := range policyIDs {
		p := convert.AmpelPolicy{
			ID: id,
			Meta: convert.PolicyMeta{
				Description: "Test policy " + id,
				Controls:    []convert.PolicyControl{{Framework: "SC", Class: "SC-CODE", ID: "01"}},
			},
			Tenets: []convert.AmpelTenet{
				{
					ID:         "01",
					Code:       "true",
					Predicates: convert.PredicateSpec{Types: []string{"http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml"}},
					Assessment: convert.TenetMessage{Message: "OK"},
					Error:      convert.TenetError{Message: "FAIL", Guidance: "Fix it"},
				},
			},
		}
		data, err := json.MarshalIndent(p, "", "  ")
		require.NoError(t, err)
		filename := filepath.Join(dir, id+".json")
		require.NoError(t, os.WriteFile(filename, data, 0600))
	}
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

	// Write granular policy files to the default policy dir
	policyDir := s.Config.PolicyDirPath()
	writeGranularPolicies(t, policyDir, "SC-CODE-01.01")

	return s, dir
}

func TestGenerate_ValidPolicy(t *testing.T) {
	s, dir := setupServer(t)
	err := s.Generate(context.Background(), makeTestPolicy())
	require.NoError(t, err)

	outputPath := filepath.Join(dir, "ampel", "policy", convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "SC-CODE-01.01")
	require.Contains(t, string(data), "complytime-ampel-policy")
}

func TestGenerate_EmptyPolicy(t *testing.T) {
	s, dir := setupServer(t)
	err := s.Generate(context.Background(), policy.Policy{})
	require.NoError(t, err)

	outputPath := filepath.Join(dir, "ampel", "policy", convert.PolicyFileName)
	_, err = os.Stat(outputPath)
	require.True(t, os.IsNotExist(err), "no policy file should be created for empty input")
}

func TestGenerate_NoMatchingPolicies(t *testing.T) {
	s, dir := setupServer(t)

	// Use a policy with a rule ID that doesn't match any granular file
	noMatchPolicy := policy.Policy{
		{
			Rule:   extensions.Rule{ID: "nonexistent-rule"},
			Checks: []extensions.Check{{ID: "check-1", Description: "Test"}},
		},
	}
	err := s.Generate(context.Background(), noMatchPolicy)
	require.NoError(t, err)

	outputPath := filepath.Join(dir, "ampel", "policy", convert.PolicyFileName)
	_, err = os.Stat(outputPath)
	require.True(t, os.IsNotExist(err), "no policy file should be created when no rules match")
}

func TestGenerate_OverwritesExistingPolicy(t *testing.T) {
	s, dir := setupServer(t)

	// Add a second granular policy
	policyDir := s.Config.PolicyDirPath()
	writeGranularPolicies(t, policyDir, "SC-CODE-03.01")

	p1 := makeTestPolicy()
	p2 := policy.Policy{
		{
			Rule:   extensions.Rule{ID: "SC-CODE-03.01"},
			Checks: []extensions.Check{{ID: "check-SC-CODE-03.01", Description: "Force push"}},
		},
	}

	require.NoError(t, s.Generate(context.Background(), p1))
	require.NoError(t, s.Generate(context.Background(), p2))

	outputPath := filepath.Join(dir, "ampel", "policy", convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "SC-CODE-03.01")
	// The second generate should have overwritten with only SC-CODE-03.01
	// (it only matched one policy from p2)
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
	// Write ampel output to the results path specified by --results-path
	for i, arg := range args {
		if arg == "--results-path" && i+1 < len(args) {
			_ = os.WriteFile(args[i+1], m.ampelOutput, 0600)
			break
		}
	}
	return nil, nil
}

func setupServerWithTargets(t *testing.T) (PluginServer, string) {
	t.Helper()
	s, dir := setupServer(t)

	// Write a targets file with explicit specs
	targetsContent := `repositories:
  - url: https://github.com/myorg/repo1
    branches:
      - main
    specs:
      - builtin:github/branch-rules.yaml
`
	targetsDir := filepath.Join(dir, "ampel")
	require.NoError(t, os.MkdirAll(targetsDir, 0750))
	require.NoError(t, os.WriteFile(
		filepath.Join(targetsDir, "ampel-targets.yaml"),
		[]byte(targetsContent), 0600,
	))

	// Generate a policy bundle so paths exist
	require.NoError(t, s.Generate(context.Background(), makeTestPolicy()))

	return s, dir
}

func TestGetResults_ValidScan(t *testing.T) {
	s, dir := setupServerWithTargets(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	pvp, err := s.GetResults(context.Background(), makeTestPolicy())
	require.NoError(t, err)
	require.Len(t, pvp.ObservationsByCheck, 1)
	require.Equal(t, policy.ResultPass, pvp.ObservationsByCheck[0].Subjects[0].Result)

	// Verify snappy attestation and ampel intoto result files were created
	resultsDir := filepath.Join(dir, "ampel", "results")
	files, err := os.ReadDir(resultsDir)
	require.NoError(t, err)
	require.Len(t, files, 2) // snappy attestation + ampel intoto result
}

func TestGetResults_NoSpecs_SkipsRepo(t *testing.T) {
	s, dir := setupServer(t)

	// Write targets where one repo has specs and one does not
	targetsContent := `repositories:
  - url: https://github.com/myorg/repo-no-specs
    branches:
      - main
  - url: https://github.com/myorg/repo-with-specs
    branches:
      - main
    specs:
      - builtin:github/branch-rules.yaml
`
	targetsDir := filepath.Join(dir, "ampel")
	require.NoError(t, os.MkdirAll(targetsDir, 0750))
	require.NoError(t, os.WriteFile(
		filepath.Join(targetsDir, "ampel-targets.yaml"),
		[]byte(targetsContent), 0600,
	))

	// Generate a policy bundle so paths exist
	require.NoError(t, s.Generate(context.Background(), makeTestPolicy()))

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	pvp, err := s.GetResults(context.Background(), makeTestPolicy())
	require.NoError(t, err)
	// Only repo-with-specs should be scanned; repo-no-specs is skipped
	require.Len(t, pvp.ObservationsByCheck, 1)
	require.Equal(t, policy.ResultPass, pvp.ObservationsByCheck[0].Subjects[0].Result)
}

func TestGetResults_MultipleSpecs(t *testing.T) {
	s, dir := setupServer(t)

	// Write targets with two specs
	targetsContent := `repositories:
  - url: https://github.com/myorg/repo1
    branches:
      - main
    specs:
      - github/branch-rules.yaml
      - github/custom-check.yaml
`
	targetsDir := filepath.Join(dir, "ampel")
	require.NoError(t, os.MkdirAll(targetsDir, 0750))
	require.NoError(t, os.WriteFile(
		filepath.Join(targetsDir, "ampel-targets.yaml"),
		[]byte(targetsContent), 0600,
	))

	// Generate a policy bundle so paths exist
	require.NoError(t, s.Generate(context.Background(), makeTestPolicy()))

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	pvp, err := s.GetResults(context.Background(), makeTestPolicy())
	require.NoError(t, err)
	// 2 specs × 1 branch with same CheckID = 1 observation with 2 subjects
	require.Len(t, pvp.ObservationsByCheck, 1)
	require.Len(t, pvp.ObservationsByCheck[0].Subjects, 2)

	// Verify 4 output files (2 snappy + 2 ampel)
	resultsDir := filepath.Join(dir, "ampel", "results")
	files, err := os.ReadDir(resultsDir)
	require.NoError(t, err)
	require.Len(t, files, 4)
}

func TestGetResults_ScanError_ContinuesScanning(t *testing.T) {
	s, dir := setupServerWithTargets(t)

	// Write targets with two repos, both with specs
	targetsContent := `repositories:
  - url: https://github.com/myorg/repo1
    branches:
      - main
    specs:
      - builtin:github/branch-rules.yaml
  - url: https://github.com/myorg/repo2
    branches:
      - main
    specs:
      - builtin:github/branch-rules.yaml
`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "ampel", "ampel-targets.yaml"),
		[]byte(targetsContent), 0600,
	))

	ampelOutput := makeAmpelResultAttestation()

	// Mock runner that fails for first repo's snappy call, succeeds for second
	callCount := 0
	origRunner := ScanRunner
	ScanRunner = &mockCallCountRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
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
	// Write ampel output to the results path specified by --results-path
	for i, arg := range args {
		if arg == "--results-path" && i+1 < len(args) {
			_ = os.WriteFile(args[i+1], m.ampelOutput, 0600)
			break
		}
	}
	return nil, nil
}

func TestGetResults_MissingTargetsFile(t *testing.T) {
	dir := t.TempDir()
	s := New()
	err := s.Configure(context.Background(), map[string]string{
		"workspace": dir,
		"profile":   "test-profile",
	})
	require.NoError(t, err)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  []byte("{}"),
	}
	defer func() { ScanRunner = origRunner }()

	_, err = s.GetResults(context.Background(), makeTestPolicy())
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
	dir := t.TempDir()
	s := New()
	err := s.Configure(context.Background(), map[string]string{
		"workspace": dir,
		"profile":   "test-profile",
	})
	require.NoError(t, err)

	origSkip := SkipToolCheck
	SkipToolCheck = false
	origTools := toolcheck.RequiredTools
	toolcheck.RequiredTools = []string{"nonexistent-ampel-tool-xyz"}
	defer func() {
		SkipToolCheck = origSkip
		toolcheck.RequiredTools = origTools
	}()

	_, err = s.GetResults(context.Background(), makeTestPolicy())
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

	// Write granular policies to custom source dir
	sourceDir := s.Config.PolicyDirPath()
	writeGranularPolicies(t, sourceDir, "SC-CODE-01.01")

	err = s.Generate(context.Background(), makeTestPolicy())
	require.NoError(t, err)

	// Generated output goes to workspace generated-policy dir, not the custom source dir
	outputPath := filepath.Join(dir, "ampel", "policy", convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "SC-CODE-01.01")

	// Verify the bundle was NOT written to the custom source dir
	sourcePath := filepath.Join(dir, "ampel", "custom-pol", convert.PolicyFileName)
	_, err = os.Stat(sourcePath)
	require.True(t, os.IsNotExist(err), "generated bundle should not be in the source policy dir")
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

	// Write granular policies
	policyDir := s.Config.PolicyDirPath()
	writeGranularPolicies(t, policyDir, "SC-CODE-01.01")

	// Write targets file with explicit specs
	targetsContent := `repositories:
  - url: https://github.com/myorg/repo1
    branches:
      - main
    specs:
      - builtin:github/branch-rules.yaml
`
	require.NoError(t, os.WriteFile(
		s.Config.TargetsFilePath(),
		[]byte(targetsContent), 0600,
	))

	// Generate policy first
	require.NoError(t, s.Generate(context.Background(), makeTestPolicy()))

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	pvp, err := s.GetResults(context.Background(), makeTestPolicy())
	require.NoError(t, err)
	require.Len(t, pvp.ObservationsByCheck, 1)

	// Verify results are in custom dir (snappy attestation + ampel intoto result)
	customResultsDir := filepath.Join(dir, "ampel", "custom-res")
	files, err := os.ReadDir(customResultsDir)
	require.NoError(t, err)
	require.Len(t, files, 2)
}

// Ensure unused imports are used
var _ = scan.ExecRunner{}
var _ = convert.PolicyFileName
