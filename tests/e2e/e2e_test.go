// SPDX-License-Identifier: Apache-2.0

//go:build e2e

// End-to-end tests for the complytime CLI exercising the full Gemara workflow.
// Uses an in-process mock OCI registry and the complyctl-provider-test binary.
//
// Run:
//
//	make test-e2e
//
// Or manually:
//
//	make build build-test-provider
//	go test -tags=e2e -mod=vendor ./tests/e2e/... -v -count=1 -timeout 120s
package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
)

const testPolicyID = "nist-800-53-r5"

// TestE2E_FullWorkflow exercises the entire Gemara workflow:
//  1. complytime get    — fetch policies from mock OCI registry into cache
//  2. complytime list   — verify cached policies appear
//  3. complytime generate — resolve policy graph, invoke test provider
//  4. complytime scan --format oscal  — produce OSCAL assessment-results
//  5. complytime scan --format pretty — produce Markdown report
//  6. complytime scan --format sarif  — produce SARIF report
func TestE2E_FullWorkflow(t *testing.T) {
	binary := locateBinary(t)
	srv := startMockRegistry(t)
	defer srv.Close()

	homeDir := t.TempDir()
	workDir := t.TempDir()
	installTestPlugin(t, homeDir)
	writeWorkspaceConfig(t, workDir, srv.URL, testPolicyID)
	env := buildEnv(homeDir)

	// Step 1: get
	t.Run("get", func(t *testing.T) {
		out := runComplytime(t, binary, workDir, env, "get")
		t.Log(out)
		assert.Contains(t, out, "Synchronization completed.")

		cacheDir := filepath.Join(homeDir, ".complytime", "policies", testPolicyID)
		assert.DirExists(t, cacheDir, "policy cache directory must exist after get")
		assert.FileExists(t, filepath.Join(cacheDir, "oci-layout"), "OCI layout marker required")

		stateFile := filepath.Join(homeDir, ".complytime", "state.json")
		assert.FileExists(t, stateFile, "state.json must track synced policies")

		stateData, err := os.ReadFile(stateFile)
		require.NoError(t, err)
		assert.Contains(t, string(stateData), testPolicyID)
	})

	// Step 2: list
	t.Run("list", func(t *testing.T) {
		out := runComplytime(t, binary, workDir, env, "list")
		t.Log(out)
		assert.Contains(t, out, testPolicyID)
	})

	// Step 3: generate
	t.Run("generate", func(t *testing.T) {
		out := runComplytime(t, binary, workDir, env,
			"generate", "--policy-id", testPolicyID)
		t.Log(out)
		assert.Contains(t, out, "Generation completed.")
	})

	// Step 4: scan --format oscal
	t.Run("scan_oscal", func(t *testing.T) {
		scanDir := filepath.Join(workDir, "scan-oscal")
		require.NoError(t, os.MkdirAll(scanDir, 0755))
		copyWorkspaceConfig(t, workDir, scanDir)

		out := runComplytime(t, binary, scanDir, env,
			"scan", "--policy-id", testPolicyID, "--format", "oscal")
		t.Log(out)
		assert.Contains(t, out, "requirements:")

		evalDir := filepath.Join(scanDir, complytime.WorkspaceDir, complytime.ScanOutputDir)
		assertOutputFile(t, evalDir, "evaluation-log-", ".yaml")
		oscalFile := assertOutputFile(t, evalDir, "assessment-results-", ".json")

		data, err := os.ReadFile(oscalFile)
		require.NoError(t, err)
		var doc map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &doc))
		ar, ok := doc["assessment-results"].(map[string]interface{})
		require.True(t, ok, "must have assessment-results root key")
		assert.Contains(t, ar, "uuid")
		assert.Contains(t, ar, "metadata")
		assert.Contains(t, ar, "results")

		meta := ar["metadata"].(map[string]interface{})
		assert.Equal(t, "1.1.3", meta["oscal-version"])
	})

	// Step 5: scan --format pretty
	t.Run("scan_pretty", func(t *testing.T) {
		scanDir := filepath.Join(workDir, "scan-pretty")
		require.NoError(t, os.MkdirAll(scanDir, 0755))
		copyWorkspaceConfig(t, workDir, scanDir)

		out := runComplytime(t, binary, scanDir, env,
			"scan", "--policy-id", testPolicyID, "--format", "pretty")
		t.Log(out)
		assert.Contains(t, out, "requirements:")

		evalDir := filepath.Join(scanDir, complytime.WorkspaceDir, complytime.ScanOutputDir)
		assertOutputFile(t, evalDir, "evaluation-log-", ".yaml")
		mdFile := assertOutputFile(t, evalDir, "report-", ".md")

		data, err := os.ReadFile(mdFile)
		require.NoError(t, err)
		assert.Contains(t, string(data), "Compliance Scan Report")
		assert.Contains(t, string(data), testPolicyID)
	})

	// Step 6: scan --format sarif
	t.Run("scan_sarif", func(t *testing.T) {
		scanDir := filepath.Join(workDir, "scan-sarif")
		require.NoError(t, os.MkdirAll(scanDir, 0755))
		copyWorkspaceConfig(t, workDir, scanDir)

		out := runComplytime(t, binary, scanDir, env,
			"scan", "--policy-id", testPolicyID, "--format", "sarif")
		t.Log(out)
		assert.Contains(t, out, "requirements:")

		evalDir := filepath.Join(scanDir, complytime.WorkspaceDir, complytime.ScanOutputDir)
		assertOutputFile(t, evalDir, "evaluation-log-", ".yaml")
		sarifFile := assertOutputFile(t, evalDir, "scan-", ".json")

		data, err := os.ReadFile(sarifFile)
		require.NoError(t, err)
		var doc map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &doc))
		assert.Contains(t, doc, "$schema", "SARIF must have $schema")
		assert.Equal(t, "2.1.0", doc["version"], "SARIF version must be 2.1.0")
	})
}

// TestE2E_PolicyCache verifies the OCI cache structure and state tracking after get.
func TestE2E_PolicyCache(t *testing.T) {
	binary := locateBinary(t)
	srv := startMockRegistry(t)
	defer srv.Close()

	homeDir := t.TempDir()
	workDir := t.TempDir()
	writeWorkspaceConfig(t, workDir, srv.URL, testPolicyID)
	env := buildEnv(homeDir)

	runComplytime(t, binary, workDir, env, "get")

	storePath := filepath.Join(homeDir, ".complytime", "policies", testPolicyID)
	assert.DirExists(t, storePath)
	assert.FileExists(t, filepath.Join(storePath, "oci-layout"))

	stateFile := filepath.Join(homeDir, ".complytime", "state.json")
	stateData, err := os.ReadFile(stateFile)
	require.NoError(t, err)

	var state map[string]interface{}
	require.NoError(t, json.Unmarshal(stateData, &state))

	policies, ok := state["policies"].(map[string]interface{})
	require.True(t, ok, "state.json must contain policies map")

	policyState, ok := policies[testPolicyID].(map[string]interface{})
	require.True(t, ok, "state.json must track policy %s", testPolicyID)
	assert.NotEmpty(t, policyState["digest"])
	assert.NotEmpty(t, policyState["version"])
}

// TestE2E_MultiplePolicies verifies fetching and listing multiple policies.
func TestE2E_MultiplePolicies(t *testing.T) {
	binary := locateBinary(t)
	srv := startMockRegistry(t)
	defer srv.Close()

	homeDir := t.TempDir()
	workDir := t.TempDir()
	env := buildEnv(homeDir)

	configYAML := fmt.Sprintf(`policies:
  - url: %s/nist-800-53-r5
    id: nist-800-53-r5
  - url: %s/cis-benchmark
    id: cis-benchmark
targets:
  - id: multi-target
    policies:
      - nist-800-53-r5
      - cis-benchmark
    variables:
      env: test
`, srv.URL, srv.URL)
	configDir := filepath.Join(workDir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "complytime.yaml"), []byte(configYAML), 0644))

	out := runComplytime(t, binary, workDir, env, "get")
	t.Log(out)
	assert.Contains(t, out, "Synchronization completed.")

	listOut := runComplytime(t, binary, workDir, env, "list")
	t.Log(listOut)
	assert.Contains(t, listOut, "nist-800-53-r5")
	assert.Contains(t, listOut, "cis-benchmark")

	// Both policies must have OCI layout in cache
	for _, pid := range []string{"nist-800-53-r5", "cis-benchmark"} {
		storePath := filepath.Join(homeDir, ".complytime", "policies", pid)
		assert.DirExists(t, storePath, "%s cache dir must exist", pid)
		assert.FileExists(t, filepath.Join(storePath, "oci-layout"), "%s must have oci-layout", pid)
	}
}

// TestE2E_ScanDefaultFormat verifies scan without --format produces only EvaluationLog.
func TestE2E_ScanDefaultFormat(t *testing.T) {
	binary := locateBinary(t)
	srv := startMockRegistry(t)
	defer srv.Close()

	homeDir := t.TempDir()
	workDir := t.TempDir()
	installTestPlugin(t, homeDir)
	writeWorkspaceConfig(t, workDir, srv.URL, testPolicyID)
	env := buildEnv(homeDir)

	runComplytime(t, binary, workDir, env, "get")

	out := runComplytime(t, binary, workDir, env,
		"scan", "--policy-id", testPolicyID)
	t.Log(out)
	assert.Contains(t, out, "requirements:")

	outDir := filepath.Join(workDir, complytime.WorkspaceDir, complytime.ScanOutputDir)
	assertOutputFile(t, outDir, "evaluation-log-", ".yaml")

	// Verify no formatted report files exist
	entries, err := os.ReadDir(outDir)
	require.NoError(t, err)
	for _, e := range entries {
		name := e.Name()
		assert.False(t, strings.HasPrefix(name, "assessment-results-"),
			"OSCAL output must not exist without --format oscal")
		assert.False(t, strings.HasPrefix(name, "report-"),
			"Markdown output must not exist without --format pretty")
	}
}

// TestE2E_ScanTargetArg verifies the target positional argument scopes scans
// to a single target and that policy inference works for single-policy targets.
func TestE2E_ScanTargetArg(t *testing.T) {
	binary := locateBinary(t)
	srv := startMockRegistry(t)
	defer srv.Close()

	homeDir := t.TempDir()
	workDir := t.TempDir()
	installTestPlugin(t, homeDir)
	env := buildEnv(homeDir)

	// Multi-target config: two targets referencing the same policy.
	configYAML := fmt.Sprintf(`policies:
  - url: %s/nist-800-53-r5
    id: nist-800-53-r5
targets:
  - id: prod
    policies:
      - nist-800-53-r5
    variables:
      env: production
  - id: staging
    policies:
      - nist-800-53-r5
    variables:
      env: staging
`, srv.URL)
	configDir := filepath.Join(workDir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "complytime.yaml"), []byte(configYAML), 0644))

	runComplytime(t, binary, workDir, env, "get")

	// Subtest: scan with target and --policy-id
	t.Run("target_with_policy_id", func(t *testing.T) {
		scanDir := filepath.Join(workDir, "scan-target-policy")
		require.NoError(t, os.MkdirAll(scanDir, 0755))
		copyWorkspaceConfig(t, workDir, scanDir)

		out := runComplytime(t, binary, scanDir, env,
			"scan", "prod", "--policy-id", testPolicyID)
		t.Log(out)
		assert.Contains(t, out, "requirements:", "scan must complete with requirements summary")
		// Verify evaluation log was produced
		evalDir := filepath.Join(scanDir, complytime.WorkspaceDir, complytime.ScanOutputDir)
		assertOutputFile(t, evalDir, "evaluation-log-", ".yaml")
	})

	// Subtest: scan with target only (policy inferred from single-policy target)
	t.Run("target_inferred_policy", func(t *testing.T) {
		scanDir := filepath.Join(workDir, "scan-target-inferred")
		require.NoError(t, os.MkdirAll(scanDir, 0755))
		copyWorkspaceConfig(t, workDir, scanDir)

		out := runComplytime(t, binary, scanDir, env,
			"scan", "staging")
		t.Log(out)
		assert.Contains(t, out, "requirements:", "scan must complete with requirements summary")
		// Verify evaluation log was produced
		evalDir := filepath.Join(scanDir, complytime.WorkspaceDir, complytime.ScanOutputDir)
		assertOutputFile(t, evalDir, "evaluation-log-", ".yaml")
	})

	// Subtest: scan with nonexistent target produces error
	t.Run("target_not_found", func(t *testing.T) {
		cmd := exec.Command(binary, "scan", "nonexistent")
		cmd.Dir = workDir
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		require.Error(t, err, "nonexistent target must produce error")
		assert.Contains(t, string(output), "not found")
		assert.Contains(t, string(output), "prod")
		assert.Contains(t, string(output), "staging")
	})

	// Subtest: scan with no args and no --policy-id produces error
	t.Run("no_args_no_policy", func(t *testing.T) {
		cmd := exec.Command(binary, "scan")
		cmd.Dir = workDir
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		require.Error(t, err, "scan with no args must produce error")
		assert.Contains(t, string(output), "specify a target or --policy-id")
	})
}

// TestE2E_InvalidFormat verifies invalid scan format is rejected.
func TestE2E_InvalidFormat(t *testing.T) {
	binary := locateBinary(t)
	srv := startMockRegistry(t)
	defer srv.Close()

	homeDir := t.TempDir()
	workDir := t.TempDir()
	installTestPlugin(t, homeDir)
	writeWorkspaceConfig(t, workDir, srv.URL, testPolicyID)
	env := buildEnv(homeDir)

	runComplytime(t, binary, workDir, env, "get")

	cmd := exec.Command(binary,
		"scan", "--policy-id", testPolicyID, "--format", "pdf")
	cmd.Dir = workDir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	assert.Error(t, err, "invalid format must produce error")
	assert.Contains(t, string(output), "invalid format")
}

// TestE2E_MissingPolicy verifies scan fails when policy is not cached.
func TestE2E_MissingPolicy(t *testing.T) {
	binary := locateBinary(t)

	homeDir := t.TempDir()
	workDir := t.TempDir()
	installTestPlugin(t, homeDir)
	env := buildEnv(homeDir)

	configYAML := `policies:
  - url: http://localhost:1/nonexistent-policy
    id: nonexistent-policy
targets:
  - id: test-target
    policies:
      - nonexistent-policy
    variables:
      env: test
`
	configDir := filepath.Join(workDir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "complytime.yaml"), []byte(configYAML), 0644))

	cmd := exec.Command(binary, "scan", "--policy-id", "nonexistent-policy")
	cmd.Dir = workDir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	assert.Error(t, err, "scan for uncached policy must fail")
	assert.Contains(t, string(output), "not in cache")
}

// TestE2E_MockRegistryOCICompliance verifies the mock registry serves valid OCI responses.
func TestE2E_MockRegistryOCICompliance(t *testing.T) {
	srv := startMockRegistry(t)
	defer srv.Close()

	t.Run("v2_endpoint", func(t *testing.T) {
		resp, err := srv.Client().Get(srv.URL + "/v2/")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "registry/2.0", resp.Header.Get("Docker-Distribution-API-Version"))
	})

	t.Run("catalog_endpoint", func(t *testing.T) {
		resp, err := srv.Client().Get(srv.URL + "/v2/_catalog")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		repos, ok := body["repositories"].([]interface{})
		require.True(t, ok, "response must contain repositories array")
		assert.GreaterOrEqual(t, len(repos), 2, "must serve at least 2 repositories")
	})

	t.Run("tags_list", func(t *testing.T) {
		resp, err := srv.Client().Get(srv.URL + "/v2/nist-800-53-r5/tags/list")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		tags, ok := body["tags"].([]interface{})
		require.True(t, ok)
		assert.GreaterOrEqual(t, len(tags), 1, "policy must have at least one tag")
	})

	t.Run("manifest_fetch", func(t *testing.T) {
		resp, err := srv.Client().Get(srv.URL + "/v2/nist-800-53-r5/manifests/latest")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "application/vnd.oci.image.manifest.v1+json", resp.Header.Get("Content-Type"))
		assert.NotEmpty(t, resp.Header.Get("Docker-Content-Digest"))

		var manifest map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&manifest))
		assert.Equal(t, float64(2), manifest["schemaVersion"])
		layers, ok := manifest["layers"].([]interface{})
		require.True(t, ok, "manifest must have layers")
		assert.GreaterOrEqual(t, len(layers), 2, "nist policy must have catalog + policy layers")
	})

	t.Run("nonexistent_repo", func(t *testing.T) {
		resp, err := srv.Client().Get(srv.URL + "/v2/does-not-exist/manifests/latest")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 404, resp.StatusCode)
	})
}

// TestE2E_MockPluginDescribe verifies the test provider binary responds to Describe.
func TestE2E_MockPluginDescribe(t *testing.T) {
	binary := locateBinary(t)
	srv := startMockRegistry(t)
	defer srv.Close()

	homeDir := t.TempDir()
	workDir := t.TempDir()
	installTestPlugin(t, homeDir)
	writeWorkspaceConfig(t, workDir, srv.URL, testPolicyID)
	env := buildEnv(homeDir)

	// Fetch policy first so generate has content
	runComplytime(t, binary, workDir, env, "get")

	// Generate dispatches to the test provider via Describe + Generate RPCs
	out := runComplytime(t, binary, workDir, env,
		"generate", "--policy-id", testPolicyID)
	t.Log(out)
	assert.Contains(t, out, "Generation completed.")
	assert.NotContains(t, out, "Describe failed",
		"test provider must pass describe")
}

// TestE2E_Help verifies basic help output structure.
func TestE2E_Help(t *testing.T) {
	binary := locateBinary(t)
	cmd := exec.Command(binary, "--help")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)

	out := string(output)
	for _, expected := range []string{
		"Usage:",
		"Available Commands:",
		"Flags:",
		"complyctl [command]",
	} {
		assert.Contains(t, out, expected, "help output must contain %q", expected)
	}
}

// TestE2E_Version verifies the version command produces output.
func TestE2E_Version(t *testing.T) {
	binary := locateBinary(t)
	cmd := exec.Command(binary, "version")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)
	assert.NotEmpty(t, string(output), "version output must not be empty")
}

// TestE2E_NestedPolicyID exercises the full workflow with a slashed policy ID
// (e.g., "policies/nist-800-53-r5") matching the standalone mock-oci-registry format.
// Validates that cache listing, output filenames, and provider routing all handle
// the "/" correctly.
func TestE2E_NestedPolicyID(t *testing.T) {
	nestedID := "policies/nist-800-53-r5"

	binary := locateBinary(t)
	srv := startMockRegistry(t)
	defer srv.Close()

	homeDir := t.TempDir()
	workDir := t.TempDir()
	installTestPlugin(t, homeDir)
	writeWorkspaceConfig(t, workDir, srv.URL, nestedID)
	env := buildEnv(homeDir)

	// get
	t.Run("get", func(t *testing.T) {
		out := runComplytime(t, binary, workDir, env, "get")
		t.Log(out)
		assert.Contains(t, out, "Synchronization completed.")

		storePath := filepath.Join(homeDir, ".complytime", "policies", "policies", "nist-800-53-r5")
		assert.DirExists(t, storePath, "nested cache directory must exist")
		assert.FileExists(t, filepath.Join(storePath, "oci-layout"))
	})

	// list — must not say "(not cached)"
	t.Run("list", func(t *testing.T) {
		out := runComplytime(t, binary, workDir, env, "list")
		t.Log(out)
		assert.Contains(t, out, "nist-800-53-r5")
		assert.NotContains(t, out, "(not cached)",
			"cached nested policy must not show as uncached")
	})

	// generate
	t.Run("generate", func(t *testing.T) {
		out := runComplytime(t, binary, workDir, env,
			"generate", "--policy-id", nestedID)
		t.Log(out)
		assert.Contains(t, out, "Generation completed.")
	})

	// scan — output files must use sanitized filename (no "/" in filename)
	t.Run("scan_oscal", func(t *testing.T) {
		scanDir := filepath.Join(workDir, "scan-nested")
		require.NoError(t, os.MkdirAll(scanDir, 0755))
		copyWorkspaceConfig(t, workDir, scanDir)

		out := runComplytime(t, binary, scanDir, env,
			"scan", "--policy-id", nestedID, "--format", "oscal")
		t.Log(out)
		assert.Contains(t, out, "requirements:")

		evalDir := filepath.Join(scanDir, complytime.WorkspaceDir, complytime.ScanOutputDir)
		evalLog := assertOutputFile(t, evalDir, "evaluation-log-", ".yaml")
		oscalFile := assertOutputFile(t, evalDir, "assessment-results-", ".json")

		// Filenames must be flat (no intermediate directories from slashed IDs)
		assert.Equal(t, evalDir, filepath.Dir(evalLog),
			"evaluation log must be directly in output dir, not nested")
		assert.Equal(t, evalDir, filepath.Dir(oscalFile),
			"OSCAL file must be directly in scan output dir, not nested")
	})
}

// TODO(#606): Add container-based scan test once a real provider
// (OpenSCAP, Ampel) is available in the test suite. Current test
// provider is mock-only and cannot validate real container scans.

// TestE2E_ListFilterByPolicyID verifies the --policy-id filter on list.
func TestE2E_ListFilterByPolicyID(t *testing.T) {
	binary := locateBinary(t)
	srv := startMockRegistry(t)
	defer srv.Close()

	homeDir := t.TempDir()
	workDir := t.TempDir()
	env := buildEnv(homeDir)

	configYAML := fmt.Sprintf(`policies:
  - url: %s/nist-800-53-r5
    id: nist-800-53-r5
  - url: %s/cis-benchmark
    id: cis-benchmark
targets:
  - id: filter-target
    policies:
      - nist-800-53-r5
      - cis-benchmark
    variables:
      env: test
`, srv.URL, srv.URL)
	configDir := filepath.Join(workDir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "complytime.yaml"), []byte(configYAML), 0644))

	runComplytime(t, binary, workDir, env, "get")

	out := runComplytime(t, binary, workDir, env,
		"list", "--policy-id", "cis-benchmark")
	t.Log(out)
	assert.Contains(t, out, "cis-benchmark")
	assert.NotContains(t, out, "nist-800-53-r5",
		"filter must exclude non-matching policies")
}
