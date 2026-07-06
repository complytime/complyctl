// SPDX-License-Identifier: Apache-2.0

//go:build acceptance

package acceptance

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
)

const (
	complyctlBinary = "/usr/local/bin/complyctl"
)

// testEnv holds configuration sourced from environment variables
// injected by compose.yaml.
type testEnv struct {
	RegistryURL string
	PolicyID    string
	TargetID    string
}

// loadTestEnv reads required environment variables. Fails the test if
// any required variable is missing.
func loadTestEnv(t *testing.T) testEnv {
	t.Helper()
	registryURL := os.Getenv("REGISTRY_URL")
	require.NotEmpty(t, registryURL, "REGISTRY_URL must be set")
	policyID := os.Getenv("TEST_POLICY_ID")
	require.NotEmpty(t, policyID, "TEST_POLICY_ID must be set")
	targetID := os.Getenv("TEST_TARGET_ID")
	require.NotEmpty(t, targetID, "TEST_TARGET_ID must be set")
	return testEnv{
		RegistryURL: registryURL,
		PolicyID:    policyID,
		TargetID:    targetID,
	}
}

// writeWorkspaceConfig creates .complytime/complytime.yaml in dir
// with the given registry URL, policy ID, and target ID.
func writeWorkspaceConfig(t *testing.T, dir string, env testEnv) {
	t.Helper()
	configYAML := fmt.Sprintf(`policies:
  - url: %s/policies/%s
    id: %s
variables:
  workspace: %s
targets:
  - id: %s
    policies:
      - %s
    variables:
      env: acceptance
`, env.RegistryURL, env.PolicyID, env.PolicyID, dir, env.TargetID, env.PolicyID)

	configDir := filepath.Join(dir, ".complytime")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "complytime.yaml"),
		[]byte(configYAML), 0o644))
}

// runComplyctl executes the complyctl binary with given args and returns
// combined stdout+stderr. Fails the test on non-zero exit code.
func runComplyctl(t *testing.T, workDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(complyctlBinary, args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("complyctl %s failed:\n%s\nerror: %v",
			strings.Join(args, " "), string(out), err)
	}
	return string(out)
}

// assertOutputFile finds a file matching prefix+suffix in dir. Returns
// its path. Fails the test if no matching file exists or is empty.
func assertOutputFile(t *testing.T, dir, prefix, suffix string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "output directory %s must be readable", dir)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			path := filepath.Join(dir, name)
			assert.FileExists(t, path)
			info, statErr := os.Stat(path)
			require.NoError(t, statErr)
			assert.Greater(t, info.Size(), int64(0), "%s must not be empty", name)
			return path
		}
	}
	t.Fatalf("no file matching %s*%s found in %s (contents: %v)",
		prefix, suffix, dir, entries)
	return ""
}

// parseOSCALFindings reads an OSCAL assessment-results JSON file and
// returns the findings array from the first result. Fails the test if
// the structure is invalid.
func parseOSCALFindings(t *testing.T, path string) []map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &doc))

	ar, ok := doc["assessment-results"].(map[string]interface{})
	require.True(t, ok, "must have assessment-results root key")

	results, ok := ar["results"].([]interface{})
	require.True(t, ok, "must have results array")
	require.NotEmpty(t, results, "results must not be empty")

	result0, ok := results[0].(map[string]interface{})
	require.True(t, ok, "first result must be an object")

	findingsRaw, ok := result0["findings"].([]interface{})
	require.True(t, ok, "first result must have findings")

	findings := make([]map[string]interface{}, 0, len(findingsRaw))
	for _, f := range findingsRaw {
		fm, ok := f.(map[string]interface{})
		require.True(t, ok, "each finding must be an object")
		findings = append(findings, fm)
	}
	return findings
}
