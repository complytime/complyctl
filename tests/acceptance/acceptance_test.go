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
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goccy/go-yaml"
)

// requireRegistry verifies the registry is reachable and seeded before a live
// acceptance test runs. Per-test setup keeps report inspection tests isolated
// (constitution II) without a flag that could bypass live-test preflights.
//
// The preflight seed check only runs when TEST_POLICY_ID is set (lifecycle
// profile). Verification tests use their own env vars (COSIGN_KEY_PATH,
// VERIFY_*_POLICY_ID) and skip via skipIfNoVerificationEnv when the
// verification profile is not active.
func requireRegistry(t *testing.T) {
	t.Helper()
	registryURL := os.Getenv("REGISTRY_URL")
	require.NotEmpty(t, registryURL, "REGISTRY_URL must be set")

	// Verify registry is reachable
	client := &http.Client{Timeout: 10 * time.Second}
	// #nosec G704 -- The trusted runner sets REGISTRY_URL; private test registry access is intentional.
	resp, err := client.Get(registryURL + "/v2/")
	require.NoError(t, err, "registry not reachable at %s", registryURL)
	require.NoError(t, resp.Body.Close())

	// Lifecycle profile preflight: verify the seeded policy exists.
	// Skip this check in the verification profile where TEST_POLICY_ID
	// is not set — verification tests have their own assertions.
	policyID := os.Getenv("TEST_POLICY_ID")
	if policyID != "" {
		// #nosec G704 -- REGISTRY_URL and TEST_POLICY_ID are trusted runner configuration
		// for private test registry access, not public service request inputs.
		resp, err = client.Get(registryURL + "/v2/policies/" + policyID + "/tags/list")
		require.NoError(t, err, "fetch seeded policy policies/%s", policyID)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusOK, resp.StatusCode,
			"policy policies/%s not found in registry — seed container likely failed", policyID)
	}
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
	cacheDir := filepath.Join(homeDir, ".cache", "complytime", "policies")
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
// assessment-results output contains the control-level finding and its
// evidence-bearing assessment observations from the seeded policy, and that
// the complete document conforms to OSCAL 1.1.3.
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
	assert.NoError(t, checkOSCALSchema(oscalFile),
		"complete scan output must conform to OSCAL 1.1.3")

	result := parseOSCALResult(t, oscalFile)
	// testdata/policy.yaml selects plan block-force-push. The test provider
	// uses that plan's MatchID for the description and source coordinate;
	// its source reference and digest are fixed in cmd/test-provider/main.go.
	assert.NoError(t, checkOSCALEvidence(result, oscalEvidenceExpectation{
		ControlID:     "force-push-protection",
		RequirementID: "block-force-push",
		Description:   "test evidence for block-force-push",
		ReferenceID:   "test-policy",
		Coordinate:    "/checks/block-force-push",
		Digest:        "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}), "control finding must link to the expected assessment and its provider evidence")

	findings, ok := result["findings"].([]interface{})
	require.True(t, ok, "OSCAL result must contain findings")
	require.NotEmpty(t, findings, "OSCAL output must contain findings")
	observations, ok := result["observations"].([]interface{})
	require.True(t, ok, "OSCAL result must contain observations")

	observationsByUUID := make(map[string]map[string]interface{}, len(observations))
	for _, rawObservation := range observations {
		observation, ok := rawObservation.(map[string]interface{})
		require.True(t, ok, "each observation must be an object")
		observationUUID, ok := observation["uuid"].(string)
		require.True(t, ok, "each observation must have a UUID")
		observationsByUUID[observationUUID] = observation
	}

	// Verify the force-push-protection control appears in findings.
	foundForcePushProtection := false
	for _, rawFinding := range findings {
		finding, ok := rawFinding.(map[string]interface{})
		require.True(t, ok, "each finding must be an object")
		target, ok := finding["target"].(map[string]interface{})
		if !ok {
			continue
		}
		targetID, _ := target["target-id"].(string)
		if targetID == "force-push-protection" {
			foundForcePushProtection = true

			// Verify result state
			status, ok := target["status"].(map[string]interface{})
			require.True(t, ok, "finding target must have status")
			assert.Equal(t, "satisfied", status["state"],
				"force-push-protection must be satisfied")

			relatedObservations, ok := finding["related-observations"].([]interface{})
			require.True(t, ok, "control finding must link to assessment observations")
			assert.NotEmpty(t, relatedObservations)
			for _, rawRelatedObservation := range relatedObservations {
				relatedObservation, ok := rawRelatedObservation.(map[string]interface{})
				require.True(t, ok, "each related observation must be an object")
				observationUUID, ok := relatedObservation["observation-uuid"].(string)
				require.True(t, ok, "each related observation must reference a UUID")

				observation, found := observationsByUUID[observationUUID]
				require.True(t, found, "related observation must exist in the result")
				properties, ok := observation["props"].([]interface{})
				require.True(t, ok, "related observation must include properties")

				foundRequirementID := false
				for _, rawProperty := range properties {
					property, ok := rawProperty.(map[string]interface{})
					require.True(t, ok, "each observation property must be an object")
					if property["name"] == "requirement-id" && property["value"] == "block-force-push" {
						foundRequirementID = true
					}
				}
				assert.True(t, foundRequirementID,
					"related observation must retain the block-force-push requirement ID")
			}
		}
	}
	assert.True(t, foundForcePushProtection,
		"OSCAL findings must contain force-push-protection control")
}
