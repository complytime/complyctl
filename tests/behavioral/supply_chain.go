// SPDX-License-Identifier: Apache-2.0

package behavioral

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gemaraproj/go-gemara"
)

// SignatureVerified checks that the get operation verified a cryptographic
// signature on the fetched policy manifest before caching (CTRL01.AR01).
// This test is expected to fail until signature validation is implemented.
// See R20 in specs/001-gemara-native-workflow/research.md for the deferral
// rationale and threat model.
func SignatureVerified(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, result, msg, conf := verifyContext(payload)
	if ctx == nil {
		return result, msg, conf
	}

	output, err := ctx.RunBinary("get")
	if err != nil {
		return gemara.Failed, "get failed: " + output, gemara.High
	}

	if strings.Contains(output, "signature verified") || strings.Contains(output, "signature valid") {
		return gemara.Passed, "policy content signature verified for " + ctx.PolicyID, gemara.High
	}

	stateFile := filepath.Join(dataDir(ctx.HomeDir), "state.json")
	data, stateErr := os.ReadFile(stateFile)
	if stateErr == nil {
		if strings.Contains(string(data), "signature") {
			return gemara.Passed, "state.json records signature verification for " + ctx.PolicyID, gemara.High
		}
	}

	return gemara.Failed,
		"no cryptographic signature verification detected during get; " +
			"policy content accepted without publisher authentication (see CT.COMPLYCTL.THR02)",
		gemara.High
}

// DigestRecordedInState checks state.json for a sha256 manifest digest
// recorded for the policy after a successful get (CTRL02.AR01).
func DigestRecordedInState(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, result, msg, conf := verifyContext(payload)
	if ctx == nil {
		return result, msg, conf
	}

	stateFile := filepath.Join(dataDir(ctx.HomeDir), "state.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return gemara.Failed, "state.json not found after get: " + err.Error(), gemara.High
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return gemara.Failed, "state.json is not valid JSON: " + err.Error(), gemara.High
	}

	policies, ok := state["policies"].(map[string]interface{})
	if !ok {
		return gemara.Failed, "state.json missing policies map", gemara.High
	}

	policyState, ok := policies[ctx.PolicyID].(map[string]interface{})
	if !ok {
		return gemara.Failed, "state.json does not track policy " + ctx.PolicyID, gemara.High
	}

	digest, ok := policyState["digest"].(string)
	if !ok || !strings.HasPrefix(digest, "sha256:") {
		return gemara.Failed, "digest missing or not a valid sha256 hash", gemara.High
	}

	return gemara.Passed, "state.json records sha256 digest for " + ctx.PolicyID, gemara.High
}

// OCILayoutExists checks the OCI layout directory for the oci-layout
// marker file (CTRL02.AR02).
func OCILayoutExists(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, result, msg, conf := verifyContext(payload)
	if ctx == nil {
		return result, msg, conf
	}

	storePath := filepath.Join(cacheDir(ctx.HomeDir), "policies", ctx.PolicyID)
	if _, err := os.Stat(storePath); err != nil {
		return gemara.Failed, "policy cache directory does not exist: " + storePath, gemara.High
	}

	ociLayout := filepath.Join(storePath, "oci-layout")
	if _, err := os.Stat(ociLayout); err != nil {
		return gemara.Failed, "oci-layout marker missing in " + storePath, gemara.High
	}

	return gemara.Passed, "OCI layout structure present for " + ctx.PolicyID, gemara.High
}
