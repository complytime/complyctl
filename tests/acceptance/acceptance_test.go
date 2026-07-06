// SPDX-License-Identifier: Apache-2.0

//go:build acceptance

// Container-based acceptance tests for complyctl exercising real OCI registry
// interop. These tests run inside a container orchestrated by compose.yaml
// against a real zot registry seeded with Gemara policies via oras CLI.
//
// Run:
//
//	make test-acceptance
//
// Or manually:
//
//	podman-compose -f tests/acceptance/compose.yaml up --profile lifecycle --build \
//	    --abort-on-container-exit --exit-code-from sut
package acceptance

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goccy/go-yaml"
)

// TestMain runs a preflight check to verify the registry is reachable and
// seeded before executing any tests. This gives a clear error message when
// the seed container failed silently.
func TestMain(m *testing.M) {
	registryURL := os.Getenv("REGISTRY_URL")
	policyID := os.Getenv("TEST_POLICY_ID")
	targetID := os.Getenv("TEST_TARGET_ID")
	if registryURL == "" || policyID == "" || targetID == "" {
		fmt.Fprintln(os.Stderr, "FATAL: REGISTRY_URL, TEST_POLICY_ID, and TEST_TARGET_ID must be set")
		os.Exit(1)
	}

	// Verify registry is reachable
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(registryURL + "/v2/")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: registry not reachable at %s: %v\n", registryURL, err)
		os.Exit(1)
	}
	resp.Body.Close()

	// Verify policy was seeded (check tags endpoint)
	resp, err = client.Get(registryURL + "/v2/policies/" + policyID + "/tags/list")
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "FATAL: policy policies/%s not found in registry — seed container likely failed\n", policyID)
		if err == nil {
			resp.Body.Close()
		}
		os.Exit(1)
	}
	resp.Body.Close()

	os.Exit(m.Run())
}

// TestAcceptance_OCILifecycle exercises the full happy-path lifecycle against a
// real zot OCI registry: write config -> get -> doctor -> generate -> scan ->
// verify evaluation log.
func TestAcceptance_OCILifecycle(t *testing.T) {
	env := loadTestEnv(t)
	workDir := t.TempDir()
	writeWorkspaceConfig(t, workDir, env)

	// get: fetch policy from zot into OCI layout cache
	out := runComplyctl(t, workDir, "get", "--workspace", workDir)
	assert.Contains(t, out, "Synchronization completed.")

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	cacheDir := filepath.Join(homeDir, ".complytime", "policies")
	assert.DirExists(t, cacheDir, "policy cache directory must exist after get")

	// doctor: verify all checks pass
	out = runComplyctl(t, workDir, "doctor", "--workspace", workDir)
	assert.Contains(t, out, "passed")
	assert.Contains(t, out, "0 failed")

	// generate: resolve policy graph and invoke test provider
	runComplyctl(t, workDir, "generate",
		"--policy-id", env.PolicyID, "--workspace", workDir)

	// scan: produce evaluation log
	out = runComplyctl(t, workDir, "scan", env.TargetID, "--workspace", workDir)
	assert.Contains(t, out, "requirements:")

	// verify evaluation log
	scanDir := filepath.Join(workDir, ".complytime", "scan")
	evalFile := assertOutputFile(t, scanDir, "evaluation-log-", ".yaml")

	data, err := os.ReadFile(evalFile)
	require.NoError(t, err)

	var evalLog map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &evalLog))

	// Check the evaluation log contains the expected requirement ID
	evaluations, ok := evalLog["evaluations"].([]interface{})
	require.True(t, ok, "evaluation log must have evaluations array")
	require.NotEmpty(t, evaluations, "evaluations must not be empty")

	foundRequirement := false
	for _, eval := range evaluations {
		evalMap, ok := eval.(map[string]interface{})
		if !ok {
			continue
		}
		logs, ok := evalMap["assessment-logs"].([]interface{})
		if !ok {
			continue
		}
		for _, log := range logs {
			logMap, ok := log.(map[string]interface{})
			if !ok {
				continue
			}
			req, ok := logMap["requirement"].(map[string]interface{})
			if !ok {
				continue
			}
			if req["entry-id"] == "block-force-push" {
				foundRequirement = true
				result, _ := logMap["result"].(string)
				assert.Equal(t, "Passed", result,
					"block-force-push requirement must pass")
			}
		}
	}
	assert.True(t, foundRequirement,
		"evaluation log must contain block-force-push requirement")

	// Verify target ID in evaluation log
	target, ok := evalLog["target"].(map[string]interface{})
	require.True(t, ok, "evaluation log must have target")
	assert.Equal(t, env.TargetID, target["id"],
		"evaluation log target must match configured target ID")
}

// TestAcceptance_FormatInterop validates that artifacts pushed by the oras CLI
// are correctly consumed by complyctl's oras-go client. Verifies the OSCAL
// assessment-results output contains correct control and requirement IDs
// from the seeded policy.
func TestAcceptance_FormatInterop(t *testing.T) {
	env := loadTestEnv(t)
	workDir := t.TempDir()
	writeWorkspaceConfig(t, workDir, env)

	// get + generate (prerequisites for scan)
	runComplyctl(t, workDir, "get", "--workspace", workDir)
	runComplyctl(t, workDir, "generate",
		"--policy-id", env.PolicyID, "--workspace", workDir)

	// scan with OSCAL output
	out := runComplyctl(t, workDir, "scan", env.TargetID,
		"--format", "oscal", "--workspace", workDir)
	assert.Contains(t, out, "requirements:")

	// verify OSCAL output
	scanDir := filepath.Join(workDir, ".complytime", "scan")
	assertOutputFile(t, scanDir, "evaluation-log-", ".yaml")
	oscalFile := assertOutputFile(t, scanDir, "assessment-results-", ".json")

	findings := parseOSCALFindings(t, oscalFile)
	require.NotEmpty(t, findings, "OSCAL output must contain findings")

	// Verify the block-force-push requirement appears in findings
	foundBlockForcePush := false
	for _, finding := range findings {
		target, ok := finding["target"].(map[string]interface{})
		if !ok {
			continue
		}
		targetID, _ := target["target-id"].(string)
		if targetID == "block-force-push" {
			foundBlockForcePush = true

			// Verify result state
			status, ok := target["status"].(map[string]interface{})
			require.True(t, ok, "finding target must have status")
			assert.Equal(t, "satisfied", status["state"],
				"block-force-push must be satisfied")

			// Verify finding title format
			title, _ := finding["title"].(string)
			assert.True(t, strings.Contains(title, "block-force-push"),
				"finding title must reference block-force-push")
		}
	}
	assert.True(t, foundBlockForcePush,
		"OSCAL findings must contain block-force-push requirement")
}
