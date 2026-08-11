// SPDX-License-Identifier: Apache-2.0

//go:build acceptance

package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
)

// skipIfNoVerificationEnv skips the calling test when verification
// environment variables are not set. Verification tests require
// COSIGN_KEY_PATH to be configured, which is only available in the
// verify-sut container profile — not in the lifecycle sut container.
func skipIfNoVerificationEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("COSIGN_KEY_PATH") == "" {
		t.Skip("verification env vars not set")
	}
}

// TestVerification_Keyed_HappyPath verifies that complyctl get with a
// correct cosign public key succeeds and marks the policy as verified in
// state.json. It also checks that complyctl list shows VERIFIED=Yes and
// complyctl doctor reports the verification status.
func TestVerification_Keyed_HappyPath(t *testing.T) {
	skipIfNoVerificationEnv(t)
	clearPolicyCache(t)

	workDir := t.TempDir()

	registryURL := os.Getenv("REGISTRY_URL")
	require.NotEmpty(t, registryURL, "REGISTRY_URL must be set")

	keyPath := os.Getenv("COSIGN_KEY_PATH")
	require.NotEmpty(t, keyPath, "COSIGN_KEY_PATH must be set")

	policyID := os.Getenv("VERIFY_KEYED_POLICY_ID")
	require.NotEmpty(t, policyID, "VERIFY_KEYED_POLICY_ID must be set")

	targetID := os.Getenv("TEST_TARGET_ID")
	require.NotEmpty(t, targetID, "TEST_TARGET_ID must be set")

	policyURL := registryURL + "/policies/" + policyID

	writeVerificationConfig(t, workDir, registryURL,
		withKeyedVerification(keyPath),
		withPolicyEntries([]policyEntryConfig{{
			URL:      policyURL,
			PolicyID: policyID,
			TargetID: targetID,
		}}),
	)

	// get: fetch and verify the signed policy
	runComplyctl(t, workDir, "get", "--workspace", workDir)

	// Assert state.json records the policy as verified
	state := readPolicyState(t)
	policyKey := "policies/" + policyID
	require.Contains(t, state.Policies, policyKey,
		"state must contain an entry for %s", policyKey)
	assert.True(t, state.Policies[policyKey].Verified,
		"policy %s must be marked as verified", policyKey)

	// list: the policy's row must show VERIFIED=Yes. Assert on the row
	// that names this policy so a stray "Yes" elsewhere in the table
	// (a different column or unrelated policy) cannot mask a regression.
	listOut := runComplyctl(t, workDir, "list", "--workspace", workDir)
	policyRow := findTableRow(t, listOut, policyID)
	assert.Contains(t, policyRow, "Yes",
		"complyctl list row for %s must show VERIFIED=Yes", policyID)

	// doctor: must report verification status. doctor exits non-zero when
	// any blocking check fails (e.g. no provider is installed in the
	// verify-sut container), which is unrelated to verification, so capture
	// the output without gating on the exit code and assert on the
	// verification status it reports.
	doctorOut, _ := runComplyctlExpectError(t, workDir, "doctor", "--workspace", workDir)
	// doctor's verification check emits an aggregate summary line (see
	// doctor.CheckVerification), not a per-policy row, so scope the assertion
	// to that summary line and require it reports the artifact as verified
	// (not unverified). A bare "verified" substring would also match the
	// "N/M cached artifacts unverified" warning and mask a regression.
	doctorRow := findTableRow(t, doctorOut, "cached artifacts")
	doctorRowLower := strings.ToLower(doctorRow)
	assert.Contains(t, doctorRowLower, "verified",
		"complyctl doctor must report the artifact as verified")
	assert.NotContains(t, doctorRowLower, "unverified",
		"complyctl doctor must not report the artifact as unverified")
}

// TestVerification_Keyed_WrongKey verifies that complyctl get fails with
// a non-zero exit code when configured with an incorrect cosign public
// key and that the error output contains the expected verification
// failure message.
func TestVerification_Keyed_WrongKey(t *testing.T) {
	skipIfNoVerificationEnv(t)
	clearPolicyCache(t)

	workDir := t.TempDir()

	registryURL := os.Getenv("REGISTRY_URL")
	require.NotEmpty(t, registryURL, "REGISTRY_URL must be set")

	wrongKeyPath := os.Getenv("COSIGN_WRONG_KEY_PATH")
	require.NotEmpty(t, wrongKeyPath, "COSIGN_WRONG_KEY_PATH must be set")

	policyID := os.Getenv("VERIFY_KEYED_POLICY_ID")
	require.NotEmpty(t, policyID, "VERIFY_KEYED_POLICY_ID must be set")

	targetID := os.Getenv("TEST_TARGET_ID")
	require.NotEmpty(t, targetID, "TEST_TARGET_ID must be set")

	policyURL := registryURL + "/policies/" + policyID

	writeVerificationConfig(t, workDir, registryURL,
		withKeyedVerification(wrongKeyPath),
		withPolicyEntries([]policyEntryConfig{{
			URL:      policyURL,
			PolicyID: policyID,
			TargetID: targetID,
		}}),
	)

	// get: must fail because the key does not match the signature
	out, err := runComplyctlExpectError(t, workDir, "get", "--workspace", workDir)
	require.Error(t, err, "complyctl get must fail with wrong verification key")
	assert.Contains(t, out, "signature verification failed",
		"error output must indicate signature verification failure")
	// Discriminate a key/signature mismatch from an identity mismatch: the
	// wrong-key path must NOT reference certificate identity, which is the
	// discriminator the wrong-identity test asserts on. Without this a
	// regression that turned the key check into the identity path (or a
	// generic bail-out that happened to fail closed) could pass silently.
	assert.NotContains(t, strings.ToLower(out), "identity",
		"wrong-key failure must not be an identity mismatch")

	// Assert no policy content was cached for this policy.
	// The cache stores OCI layouts under the XDG cache dir at
	// policies/<repository> where repository is the full OCI path
	// e.g. "policies/verify-keyed".
	cacheDir, cacheErr := complytime.ResolveCacheDir()
	require.NoError(t, cacheErr)
	policyLayoutDir := filepath.Join(cacheDir, "policies", "policies", policyID)
	assert.NoDirExists(t, policyLayoutDir,
		"OCI layout directory must not exist for policy with failed verification")
}

// TestVerification_Keyless_HappyPath verifies that complyctl get with
// keyless (OIDC) verification succeeds and records signer identity and
// issuer in state.json. Keyless verification is active: the trusted root
// is supplied via TRUSTED_ROOT_PATH (keylessTrustedRootPath). The test
// runs only in the verification compose profile and is otherwise skipped
// by skipIfNoVerificationEnv.
func TestVerification_Keyless_HappyPath(t *testing.T) {
	skipIfNoVerificationEnv(t)
	clearPolicyCache(t)

	workDir := t.TempDir()

	registryURL := os.Getenv("REGISTRY_URL")
	require.NotEmpty(t, registryURL, "REGISTRY_URL must be set")

	policyID := os.Getenv("VERIFY_KEYLESS_POLICY_ID")
	require.NotEmpty(t, policyID, "VERIFY_KEYLESS_POLICY_ID must be set")

	targetID := os.Getenv("TEST_TARGET_ID")
	require.NotEmpty(t, targetID, "TEST_TARGET_ID must be set")

	trustedRootPath := keylessTrustedRootPath()

	issuer := keylessIssuer()
	identity := keylessIdentity()

	policyURL := registryURL + "/policies/" + policyID

	writeVerificationConfig(t, workDir, registryURL,
		withKeylessVerification(issuer, identity, trustedRootPath),
		withPolicyEntries([]policyEntryConfig{{
			URL:      policyURL,
			PolicyID: policyID,
			TargetID: targetID,
		}}),
	)

	// get: fetch and verify the keyless-signed policy
	runComplyctl(t, workDir, "get", "--workspace", workDir)

	// Assert state.json records the policy as verified with identity
	state := readPolicyState(t)
	policyKey := "policies/verify-keyless"
	require.Contains(t, state.Policies, policyKey,
		"state must contain an entry for %s", policyKey)
	assert.True(t, state.Policies[policyKey].Verified,
		"policy %s must be marked as verified", policyKey)
	assert.Equal(t, identity, state.Policies[policyKey].SignerIdentity,
		"signer identity must match configured OIDC identity")
	assert.Equal(t, issuer, state.Policies[policyKey].Issuer,
		"issuer must match configured OIDC issuer")
}

// TestVerification_Keyless_WrongIdentity verifies that complyctl get
// fails when the configured OIDC identity does not match the signer
// identity in the signature. Keyless verification is active: the trusted
// root is supplied via TRUSTED_ROOT_PATH (keylessTrustedRootPath). The
// test runs only in the verification compose profile and is otherwise
// skipped by skipIfNoVerificationEnv.
func TestVerification_Keyless_WrongIdentity(t *testing.T) {
	skipIfNoVerificationEnv(t)
	clearPolicyCache(t)

	workDir := t.TempDir()

	registryURL := os.Getenv("REGISTRY_URL")
	require.NotEmpty(t, registryURL, "REGISTRY_URL must be set")

	policyID := os.Getenv("VERIFY_KEYLESS_POLICY_ID")
	require.NotEmpty(t, policyID, "VERIFY_KEYLESS_POLICY_ID must be set")

	targetID := os.Getenv("TEST_TARGET_ID")
	require.NotEmpty(t, targetID, "TEST_TARGET_ID must be set")

	trustedRootPath := keylessTrustedRootPath()

	policyURL := registryURL + "/policies/" + policyID

	// Deliberately configure an identity that does not match the signer
	// so verification must fail; the issuer still comes from the wired env.
	writeVerificationConfig(t, workDir, registryURL,
		withKeylessVerification(keylessIssuer(), "wrong@example.com", trustedRootPath),
		withPolicyEntries([]policyEntryConfig{{
			URL:      policyURL,
			PolicyID: policyID,
			TargetID: targetID,
		}}),
	)

	// get: must fail because the identity does not match the signature
	out, err := runComplyctlExpectError(t, workDir, "get", "--workspace", workDir)
	require.Error(t, err, "complyctl get must fail with wrong OIDC identity")
	assert.Contains(t, out, "signature verification failed",
		"error output must indicate signature verification failure")
	// Distinguish an identity mismatch from any other verification failure
	// (e.g. a wrong key): the wrong-identity path must reference the
	// certificate identity / SAN mismatch rather than only the generic
	// verification-failed prefix shared with the wrong-key test.
	assert.Contains(t, strings.ToLower(out), "identity",
		"error output must indicate a certificate identity mismatch")
}

// TestVerification_SkipVerifyFlag verifies that complyctl get
// --skip-verify bypasses signature verification, succeeds, and marks
// the policy as unverified in state.json. The CLI must emit a WARNING
// about skipping and a NOTE about the unverified fetch.
func TestVerification_SkipVerifyFlag(t *testing.T) {
	skipIfNoVerificationEnv(t)
	clearPolicyCache(t)

	workDir := t.TempDir()

	registryURL := os.Getenv("REGISTRY_URL")
	require.NotEmpty(t, registryURL, "REGISTRY_URL must be set")

	keyPath := os.Getenv("COSIGN_KEY_PATH")
	require.NotEmpty(t, keyPath, "COSIGN_KEY_PATH must be set")

	policyID := os.Getenv("VERIFY_KEYED_POLICY_ID")
	require.NotEmpty(t, policyID, "VERIFY_KEYED_POLICY_ID must be set")

	targetID := os.Getenv("TEST_TARGET_ID")
	require.NotEmpty(t, targetID, "TEST_TARGET_ID must be set")

	policyURL := registryURL + "/policies/" + policyID

	writeVerificationConfig(t, workDir, registryURL,
		withKeyedVerification(keyPath),
		withPolicyEntries([]policyEntryConfig{{
			URL:      policyURL,
			PolicyID: policyID,
			TargetID: targetID,
		}}),
	)

	// get --skip-verify: must succeed but skip verification.
	// Use runComplyctlCapture to inspect combined output (stdout+stderr) on
	// the success path without fataling, so the skip warning and unverified
	// note can be asserted.
	out, err := runComplyctlCapture(t, workDir,
		"get", "--skip-verify", "--workspace", workDir)
	require.NoError(t, err,
		"complyctl get --skip-verify must succeed")

	// Assert the CLI warned about skipping verification
	assert.Contains(t, out, "WARNING: signature verification skipped",
		"output must contain skip-verify warning")

	// Assert the CLI noted the policy was fetched without verification
	assert.Contains(t, out, "NOTE: policy",
		"output must contain unverified fetch note prefix")
	assert.Contains(t, out, "was fetched without",
		"output must contain unverified fetch note body")

	// Assert state.json records the policy as unverified
	state := readPolicyState(t)
	policyKey := "policies/" + policyID
	require.Contains(t, state.Policies, policyKey,
		"state must contain an entry for %s", policyKey)
	assert.False(t, state.Policies[policyKey].Verified,
		"policy %s must NOT be marked as verified when --skip-verify is used", policyKey)
}

// TestVerification_PerEntrySkipVerify verifies that per-entry
// skip_verify allows one policy to bypass verification while another
// policy in the same workspace inherits workspace-level keyed
// verification and is verified normally.
func TestVerification_PerEntrySkipVerify(t *testing.T) {
	skipIfNoVerificationEnv(t)
	clearPolicyCache(t)

	workDir := t.TempDir()

	registryURL := os.Getenv("REGISTRY_URL")
	require.NotEmpty(t, registryURL, "REGISTRY_URL must be set")

	keyPath := os.Getenv("COSIGN_KEY_PATH")
	require.NotEmpty(t, keyPath, "COSIGN_KEY_PATH must be set")

	keyedPolicyID := os.Getenv("VERIFY_KEYED_POLICY_ID")
	require.NotEmpty(t, keyedPolicyID, "VERIFY_KEYED_POLICY_ID must be set")

	unsignedPolicyID := os.Getenv("VERIFY_UNSIGNED_POLICY_ID")
	require.NotEmpty(t, unsignedPolicyID, "VERIFY_UNSIGNED_POLICY_ID must be set")

	targetID := os.Getenv("TEST_TARGET_ID")
	require.NotEmpty(t, targetID, "TEST_TARGET_ID must be set")

	writeVerificationConfig(t, workDir, registryURL,
		withKeyedVerification(keyPath),
		withPolicyEntries([]policyEntryConfig{
			{
				// Entry 1: inherits workspace-level keyed verification
				URL:      registryURL + "/policies/" + keyedPolicyID,
				PolicyID: keyedPolicyID,
				TargetID: targetID,
			},
			{
				// Entry 2: per-entry skip_verify bypasses verification
				URL:        registryURL + "/policies/" + unsignedPolicyID,
				PolicyID:   unsignedPolicyID,
				TargetID:   targetID,
				SkipVerify: true,
			},
		}),
	)

	// get: must succeed — keyed entry verifies, unsigned entry skips
	runComplyctl(t, workDir, "get", "--workspace", workDir)

	state := readPolicyState(t)

	// Entry 1: verified via workspace-level keyed config
	keyedKey := "policies/" + keyedPolicyID
	require.Contains(t, state.Policies, keyedKey,
		"state must contain an entry for %s", keyedKey)
	assert.True(t, state.Policies[keyedKey].Verified,
		"policy %s must be marked as verified", keyedKey)

	// Entry 2: unverified due to per-entry skip_verify
	unsignedKey := "policies/" + unsignedPolicyID
	require.Contains(t, state.Policies, unsignedKey,
		"state must contain an entry for %s", unsignedKey)
	assert.False(t, state.Policies[unsignedKey].Verified,
		"policy %s must NOT be verified when skip_verify is set", unsignedKey)
}

// TestVerification_PerEntryOverride verifies that a per-entry
// verification config overrides the workspace-level config. The
// workspace is configured with keyless verification (which would fail
// without trusted_root), but the first entry overrides with keyed
// verification that succeeds. The second entry uses skip_verify to
// avoid the keyless failure.
func TestVerification_PerEntryOverride(t *testing.T) {
	skipIfNoVerificationEnv(t)
	clearPolicyCache(t)

	workDir := t.TempDir()

	registryURL := os.Getenv("REGISTRY_URL")
	require.NotEmpty(t, registryURL, "REGISTRY_URL must be set")

	keyPath := os.Getenv("COSIGN_KEY_PATH")
	require.NotEmpty(t, keyPath, "COSIGN_KEY_PATH must be set")

	keyedPolicyID := os.Getenv("VERIFY_KEYED_POLICY_ID")
	require.NotEmpty(t, keyedPolicyID, "VERIFY_KEYED_POLICY_ID must be set")

	unsignedPolicyID := os.Getenv("VERIFY_UNSIGNED_POLICY_ID")
	require.NotEmpty(t, unsignedPolicyID, "VERIFY_UNSIGNED_POLICY_ID must be set")

	targetID := os.Getenv("TEST_TARGET_ID")
	require.NotEmpty(t, targetID, "TEST_TARGET_ID must be set")

	trustedRootPath := keylessTrustedRootPath()

	writeVerificationConfig(t, workDir, registryURL,
		// Workspace-level: keyless (would fail without trusted_root)
		withKeylessVerification(keylessIssuer(), keylessIdentity(), trustedRootPath),
		withPolicyEntries([]policyEntryConfig{
			{
				// Entry 1: per-entry keyed override supersedes workspace keyless
				URL:      registryURL + "/policies/" + keyedPolicyID,
				PolicyID: keyedPolicyID,
				TargetID: targetID,
				Verification: &complytime.VerificationConfig{
					Key: keyPath,
				},
			},
			{
				// Entry 2: skip_verify avoids keyless failure
				URL:        registryURL + "/policies/" + unsignedPolicyID,
				PolicyID:   unsignedPolicyID,
				TargetID:   targetID,
				SkipVerify: true,
			},
		}),
	)

	// get: must succeed — entry 1 uses keyed override, entry 2 skips
	runComplyctl(t, workDir, "get", "--workspace", workDir)

	state := readPolicyState(t)

	// Entry 1: verified via per-entry keyed override
	keyedKey := "policies/" + keyedPolicyID
	require.Contains(t, state.Policies, keyedKey,
		"state must contain an entry for %s", keyedKey)
	assert.True(t, state.Policies[keyedKey].Verified,
		"policy %s must be verified via per-entry keyed override", keyedKey)

	// Entry 2: unverified due to skip_verify
	unsignedKey := "policies/" + unsignedPolicyID
	require.Contains(t, state.Policies, unsignedKey,
		"state must contain an entry for %s", unsignedKey)
	assert.False(t, state.Policies[unsignedKey].Verified,
		"policy %s must NOT be verified when skip_verify is set", unsignedKey)
}

// TestVerification_UnsignedWithVerification verifies that complyctl get
// fails when keyed verification is configured but the target OCI
// artifact has no cosign signature. The error must indicate no signature
// was found, and no policy content should be cached.
func TestVerification_UnsignedWithVerification(t *testing.T) {
	skipIfNoVerificationEnv(t)
	clearPolicyCache(t)

	workDir := t.TempDir()

	registryURL := os.Getenv("REGISTRY_URL")
	require.NotEmpty(t, registryURL, "REGISTRY_URL must be set")

	keyPath := os.Getenv("COSIGN_KEY_PATH")
	require.NotEmpty(t, keyPath, "COSIGN_KEY_PATH must be set")

	unsignedPolicyID := os.Getenv("VERIFY_UNSIGNED_POLICY_ID")
	require.NotEmpty(t, unsignedPolicyID, "VERIFY_UNSIGNED_POLICY_ID must be set")

	targetID := os.Getenv("TEST_TARGET_ID")
	require.NotEmpty(t, targetID, "TEST_TARGET_ID must be set")

	policyURL := registryURL + "/policies/" + unsignedPolicyID

	writeVerificationConfig(t, workDir, registryURL,
		withKeyedVerification(keyPath),
		withPolicyEntries([]policyEntryConfig{{
			URL:      policyURL,
			PolicyID: unsignedPolicyID,
			TargetID: targetID,
		}}),
	)

	// get: must fail because the unsigned artifact has no cosign signature
	out, err := runComplyctlExpectError(t, workDir, "get", "--workspace", workDir)
	require.Error(t, err,
		"complyctl get must fail when verifying an unsigned artifact")
	assert.Contains(t, strings.ToLower(out), "no cosign signature found",
		"error output must indicate no cosign signature was found")

	// Assert state.json does NOT contain an entry for the unsigned policy
	state := readPolicyState(t)
	unsignedKey := "policies/" + unsignedPolicyID
	assert.NotContains(t, state.Policies, unsignedKey,
		"state must NOT contain an entry for unsigned policy %s", unsignedKey)

	// Assert no OCI layout directory was cached for the unsigned policy
	cacheDir, cacheErr := complytime.ResolveCacheDir()
	require.NoError(t, cacheErr)
	policyLayoutDir := filepath.Join(cacheDir, "policies",
		"policies", unsignedPolicyID)
	assert.NoDirExists(t, policyLayoutDir,
		"OCI layout directory must not exist for unsigned policy")
}
