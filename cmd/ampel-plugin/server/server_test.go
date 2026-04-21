package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/cmd/ampel-plugin/config"
	"github.com/complytime/complyctl/cmd/ampel-plugin/convert"
	"github.com/complytime/complyctl/cmd/ampel-plugin/scan"
	"github.com/complytime/complyctl/cmd/ampel-plugin/toolcheck"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/pkg/provider"
)

func TestMain(m *testing.M) {
	// Skip tool check for most tests since snappy/ampel may not be installed
	SkipToolCheck = true
	os.Exit(m.Run())
}

func makeTestConfigurations() []provider.AssessmentConfiguration {
	return []provider.AssessmentConfiguration{
		{RequirementID: "BP-1.01"},
	}
}

func makeTestAttestation() []byte {
	stmt := map[string]any{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []map[string]any{
			{
				"name": "test-subject",
				"digest": map[string]string{
					"sha256": "abc123def456",
				},
			},
		},
		"predicateType": "http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml",
		"predicate":     map[string]any{},
	}
	data, _ := json.Marshal(stmt)
	return data
}

func makeAmpelResultAttestation() []byte {
	stmt := map[string]any{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []map[string]any{
			{
				"name":   "test-subject",
				"digest": map[string]string{"sha256": "abc123def456"},
			},
		},
		"predicateType": "https://carabiner.dev/ampel/resultset/v0",
		"predicate": map[string]any{
			"status": "PASS",
			"results": []map[string]any{
				{
					"status": "PASS",
					"policy": map[string]string{"id": "BP-1.01"},
					"eval_results": []map[string]any{
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
				Controls:    []convert.PolicyControl{{Framework: "repo-branch-protection", Class: "source-code", ID: "BP-1"}},
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

// setupServer creates a temp directory, changes to it, and sets up
// granular policy files for testing.
func setupServer(t *testing.T) (*PluginServer, string) {
	t.Helper()
	dir := t.TempDir()

	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	require.NoError(t, config.EnsureDirectories())

	s := New()

	// Write granular policy files to the default granular policy dir
	policyDir := config.GranularPolicyDirPath()
	writeGranularPolicies(t, policyDir, "BP-1.01")

	return s, dir
}

// setupServerWithGenerate creates a server and runs Generate to prepare
// policy artifacts for scanning.
func setupServerWithGenerate(t *testing.T) (*PluginServer, string) {
	t.Helper()
	s, dir := setupServer(t)

	// Generate a policy bundle so paths exist
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	return s, dir
}

// --- Describe tests (US4) ---

func TestDescribe_Healthy(t *testing.T) {
	s := New()
	resp, err := s.Describe(context.Background(), &provider.DescribeRequest{})
	require.NoError(t, err)
	require.True(t, resp.Healthy)
	require.Equal(t, "0.1.0", resp.Version)
	require.Equal(t, []string{"url", "specs"}, resp.RequiredTargetVariables)
}

// --- Generate tests (US1) ---

func TestGenerate_ValidConfiguration(t *testing.T) {
	s, dir := setupServer(t)
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Empty(t, resp.ErrorMessage)

	outputPath := filepath.Join(dir, complytime.WorkspaceDir, config.PluginDir, config.GeneratedPolicyDir, convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-1.01")
	require.Contains(t, string(data), "complytime-ampel-policy")
}

func TestGenerate_EmptyConfiguration(t *testing.T) {
	s, _ := setupServer(t)
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{},
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.ErrorMessage, "no assessment configurations")
}

func TestGenerate_NoMatchingPolicies(t *testing.T) {
	s, dir := setupServer(t)

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "nonexistent-rule"},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Success, "should succeed with no matches (no error)")

	outputPath := filepath.Join(dir, complytime.WorkspaceDir, config.PluginDir, config.GeneratedPolicyDir, convert.PolicyFileName)
	_, err = os.Stat(outputPath)
	require.True(t, os.IsNotExist(err), "no policy file should be created when no rules match")
}

func TestGenerate_OverwritesExistingPolicy(t *testing.T) {
	s, dir := setupServer(t)

	// Add a second granular policy
	policyDir := config.GranularPolicyDirPath()
	writeGranularPolicies(t, policyDir, "BP-3.01")

	configs1 := makeTestConfigurations()
	configs2 := []provider.AssessmentConfiguration{
		{RequirementID: "BP-3.01"},
	}

	resp1, err := s.Generate(context.Background(), &provider.GenerateRequest{Configuration: configs1})
	require.NoError(t, err)
	require.True(t, resp1.Success)

	resp2, err := s.Generate(context.Background(), &provider.GenerateRequest{Configuration: configs2})
	require.NoError(t, err)
	require.True(t, resp2.Success)

	outputPath := filepath.Join(dir, complytime.WorkspaceDir, config.PluginDir, config.GeneratedPolicyDir, convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-3.01")
}

func TestGenerate_CustomPolicyDir(t *testing.T) {
	s, dir := setupServer(t)

	// Write granular policies to a custom directory
	customDir := filepath.Join(dir, "custom-policies")
	writeGranularPolicies(t, customDir, "BP-1.01")

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration:   makeTestConfigurations(),
		GlobalVariables: map[string]string{"ampel_policy_dir": customDir},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Empty(t, resp.ErrorMessage)

	outputPath := filepath.Join(dir, complytime.WorkspaceDir, config.PluginDir, config.GeneratedPolicyDir, convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-1.01")
}

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

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.ErrorMessage, "nonexistent-ampel-tool-xyz")
}

// --- Scan tests (US2) ---

// mockScanRunner returns different outputs for snappy vs ampel calls.
type mockScanRunner struct {
	snappyOutput []byte
	ampelOutput  []byte
	err          error
}

func (m *mockScanRunner) Run(name string, args ...string) ([]byte, error) {
	return m.run(name, args...)
}

func (m *mockScanRunner) RunWithEnv(_ []string, name string, args ...string) ([]byte, error) {
	return m.run(name, args...)
}

func (m *mockScanRunner) run(name string, args ...string) ([]byte, error) {
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

func TestScan_ValidTargets(t *testing.T) {
	s, dir := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	resp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Assessments, 1)
	require.Equal(t, provider.ResultPassed, resp.Assessments[0].Steps[0].Result)

	// Verify snappy attestation and ampel intoto result files were created
	resultsDir := filepath.Join(dir, complytime.WorkspaceDir, config.PluginDir, config.DefaultResultsDir)
	files, err := os.ReadDir(resultsDir)
	require.NoError(t, err)
	require.Len(t, files, 2) // snappy attestation + ampel intoto result
}

func TestScan_EmptySpecs_ReturnsError(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  []byte("{}"),
	}
	defer func() { ScanRunner = origRunner }()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url": "https://github.com/myorg/repo1",
			}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required variable 'specs'")
}

func TestScan_MultipleSpecs(t *testing.T) {
	s, dir := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	scanResp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "github/branch-rules.yaml,github/custom-check.yaml",
			}},
		},
	})
	require.NoError(t, err)
	// 2 specs × 1 branch with same requirement ID = 1 assessment with 2 steps
	require.Len(t, scanResp.Assessments, 1)
	require.Len(t, scanResp.Assessments[0].Steps, 2)

	// Verify 4 output files (2 snappy + 2 ampel)
	resultsDir := filepath.Join(dir, complytime.WorkspaceDir, config.PluginDir, config.DefaultResultsDir)
	files, err := os.ReadDir(resultsDir)
	require.NoError(t, err)
	require.Len(t, files, 4)
}

func TestScan_ScanError_ContinuesScanning(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	// Mock runner that fails for first target's snappy call, succeeds for second
	callCount := 0
	origRunner := ScanRunner
	ScanRunner = &mockCallCountRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
		failOnCall:   1,
		callCount:    &callCount,
	}
	defer func() { ScanRunner = origRunner }()

	scanResp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
			}},
			{TargetID: "myorg-repo2", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo2",
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.NoError(t, err)
	// Should have 2 assessments: one error, one pass
	require.Len(t, scanResp.Assessments, 2)
}

type mockCallCountRunner struct {
	snappyOutput []byte
	ampelOutput  []byte
	failOnCall   int
	callCount    *int
}

func (m *mockCallCountRunner) Run(name string, args ...string) ([]byte, error) {
	return m.run(name, args...)
}

func (m *mockCallCountRunner) RunWithEnv(_ []string, name string, args ...string) ([]byte, error) {
	return m.run(name, args...)
}

func (m *mockCallCountRunner) run(name string, args ...string) ([]byte, error) {
	*m.callCount++
	// Fail on the snappy call for the first target
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

func TestScan_MissingURLVariable(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  []byte("{}"),
	}
	defer func() { ScanRunner = origRunner }()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "test", Variables: map[string]string{
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required variable 'url'")
}

func TestScan_MissingSpecsVariable(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  []byte("{}"),
	}
	defer func() { ScanRunner = origRunner }()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "test", Variables: map[string]string{
				"url": "https://github.com/myorg/repo1",
			}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required variable 'specs'")
}

func TestScan_BranchesDefault(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	resp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
				// branches omitted — should default to "main"
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Assessments, 1)
}

func TestScan_CommaSeparatedBranches(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	resp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":      "https://github.com/myorg/repo1",
				"specs":    "builtin:github/branch-rules.yaml",
				"branches": "main,develop",
			}},
		},
	})
	require.NoError(t, err)
	// 2 branches × 1 spec = 2 scan results, merged into assessments
	require.NotEmpty(t, resp.Assessments)
}

func TestScan_PlatformHintVariable(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	resp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "corp-repo", Variables: map[string]string{
				"url":      "https://git.corp.com/myorg/repo1",
				"specs":    "builtin:github/branch-rules.yaml",
				"platform": "github",
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Assessments, 1)
}

func TestScan_BranchValidation(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  []byte("{}"),
	}
	defer func() { ScanRunner = origRunner }()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "test", Variables: map[string]string{
				"url":      "https://github.com/myorg/repo1",
				"specs":    "builtin:github/branch-rules.yaml",
				"branches": "main;rm -rf /",
			}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid characters")
}

func TestScan_EmptyTargets(t *testing.T) {
	s := New()
	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no targets")
}

// Tool check integration tests

func TestGenerate_ToolCheckError_IncludesToolName(t *testing.T) {
	s, _ := setupServer(t)

	origSkip := SkipToolCheck
	SkipToolCheck = false
	origTools := toolcheck.RequiredTools
	toolcheck.RequiredTools = []string{"missing-snappy-test", "missing-ampel-test"}
	defer func() {
		SkipToolCheck = origSkip
		toolcheck.RequiredTools = origTools
	}()

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.ErrorMessage, "missing-snappy-test")
	require.Contains(t, resp.ErrorMessage, "missing-ampel-test")
	require.Contains(t, resp.ErrorMessage, "PATH")
}

func TestScan_MissingToolReturnsError(t *testing.T) {
	s, _ := setupServer(t)

	origSkip := SkipToolCheck
	SkipToolCheck = false
	origTools := toolcheck.RequiredTools
	toolcheck.RequiredTools = []string{"nonexistent-ampel-tool-xyz"}
	defer func() {
		SkipToolCheck = origSkip
		toolcheck.RequiredTools = origTools
	}()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "test", Variables: map[string]string{"github_token": "ghp_test"}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent-ampel-tool-xyz")
}

// Ensure unused imports are used
var _ = scan.ExecRunner{}
var _ = convert.PolicyFileName
