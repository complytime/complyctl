// SPDX-License-Identifier: Apache-2.0

//go:build acceptance

package acceptance

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"

	"github.com/goccy/go-yaml"
)

// policyEntryConfig captures per-entry configuration for
// writeVerificationConfig. URL, PolicyID, and TargetID are required.
// Verification and SkipVerify allow per-entry overrides.
type policyEntryConfig struct {
	URL          string
	PolicyID     string
	TargetID     string
	Verification *complytime.VerificationConfig
	SkipVerify   bool
}

// verifyOption is a functional option for writeVerificationConfig.
type verifyOption func(*verifyConfig)

// verifyConfig aggregates all options for writeVerificationConfig.
type verifyConfig struct {
	verification *complytime.VerificationConfig
	entries      []policyEntryConfig
}

// withKeyedVerification sets workspace-level keyed (public key)
// verification.
func withKeyedVerification(keyPath string) verifyOption {
	return func(vc *verifyConfig) {
		vc.verification = &complytime.VerificationConfig{
			Key: keyPath,
		}
	}
}

// withKeylessVerification sets workspace-level keyless (OIDC issuer +
// identity) verification.
func withKeylessVerification(issuer, identity, trustedRootPath string) verifyOption {
	return func(vc *verifyConfig) {
		vc.verification = &complytime.VerificationConfig{
			Issuer:      issuer,
			Identity:    identity,
			TrustedRoot: trustedRootPath,
		}
	}
}

// withPolicyEntries provides explicit policy entries for multi-entry
// scenarios. When set, the default single-entry policy is not generated.
func withPolicyEntries(entries []policyEntryConfig) verifyOption {
	return func(vc *verifyConfig) {
		vc.entries = entries
	}
}

// writeVerificationConfig creates .complytime/complytime.yaml in dir
// with verification settings and policy entries. When no explicit
// entries are provided via withPolicyEntries, a single default policy
// entry is created using registryURL.
func writeVerificationConfig(t *testing.T, dir string, registryURL string, opts ...verifyOption) {
	t.Helper()

	vc := &verifyConfig{}
	for _, opt := range opts {
		opt(vc)
	}

	// All verification tests must provide explicit entries via
	// withPolicyEntries. There is no default fallback because
	// verification tests use their own env vars (VERIFY_*_POLICY_ID),
	// not the lifecycle TEST_POLICY_ID.
	entries := vc.entries
	require.NotEmpty(t, entries,
		"writeVerificationConfig requires withPolicyEntries")

	// Assemble the workspace config struct.
	wsCfg := complytime.WorkspaceConfig{
		Verification: vc.verification,
	}

	for _, e := range entries {
		pe := complytime.PolicyEntry{
			URL:          e.URL,
			ID:           e.PolicyID,
			Verification: e.Verification,
			SkipVerify:   e.SkipVerify,
		}
		wsCfg.Policies = append(wsCfg.Policies, pe)
	}

	// Build targets from unique target IDs across entries.
	targetSeen := make(map[string][]string)
	for _, e := range entries {
		require.NotEmpty(t, e.TargetID,
			"policyEntryConfig.TargetID must be set for %s", e.PolicyID)
		targetSeen[e.TargetID] = append(targetSeen[e.TargetID], e.PolicyID)
	}
	for tid, pids := range targetSeen {
		wsCfg.Targets = append(wsCfg.Targets, complytime.TargetConfig{
			ID:       tid,
			Policies: pids,
		})
	}

	data, err := yaml.Marshal(&wsCfg)
	require.NoError(t, err, "marshal workspace config")

	configDir := filepath.Join(dir, complytime.WorkspaceDir)
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, complytime.WorkspaceConfigFile),
		data, 0o644))
}

// readPolicyState reads and unmarshals state.json from the XDG data
// directory. This matches where complyctl get persists sync state via
// cache.SaveState (which writes to the data dir, not the cache dir).
// Fails the test if the file cannot be read or parsed.
func readPolicyState(t *testing.T) *cache.State {
	t.Helper()

	dataDir, err := complytime.ResolveDataDir()
	require.NoError(t, err, "resolve data directory")

	statePath := filepath.Join(dataDir, complytime.StateFileName)

	data, err := os.ReadFile(statePath)
	require.NoError(t, err, "read state.json from %s", statePath)

	var state cache.State
	require.NoError(t, json.Unmarshal(data, &state), "unmarshal state.json")

	return &state
}

// runComplyctlExpectError executes the complyctl binary and returns the
// combined stdout+stderr output along with any error. Unlike runComplyctl,
// it does not call t.Fatalf on non-zero exit, allowing the caller to
// assert on stderr content and exit code for negative tests.
func runComplyctlExpectError(t *testing.T, workDir string, args ...string) (string, error) {
	t.Helper()
	return runComplyctlCapture(t, workDir, args...)
}

// runComplyctlCapture executes the complyctl binary and returns the combined
// stdout+stderr output along with any error, without calling t.Fatalf on a
// non-zero exit. Unlike runComplyctl (which fatals on error) it lets the caller
// decide how to treat the exit code, so it suits both negative tests asserting
// on failure output and success paths that still need to inspect stderr (e.g.
// --skip-verify warnings).
func runComplyctlCapture(t *testing.T, workDir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(complyctlBinary, args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	t.Logf("complyctl %s output:\n%s", strings.Join(args, " "), string(out))
	return string(out), err
}

// clearPolicyCache removes the global OCI policy cache directory and
// state.json so that negative tests (e.g., wrong key, unsigned artifact)
// are not affected by cache entries from prior positive tests.
func clearPolicyCache(t *testing.T) {
	t.Helper()

	cacheDir, err := complytime.ResolveCacheDir()
	require.NoError(t, err, "resolve cache directory")

	policiesDir := filepath.Join(cacheDir, "policies")
	if err := os.RemoveAll(policiesDir); err != nil {
		t.Logf("warning: failed to remove policies cache: %v", err)
	}

	// state.json lives in the XDG data directory (where cache.SaveState
	// writes it), not the cache directory that holds the OCI layouts.
	dataDir, err := complytime.ResolveDataDir()
	require.NoError(t, err, "resolve data directory")

	statePath := filepath.Join(dataDir, complytime.StateFileName)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		t.Logf("warning: failed to remove state.json: %v", err)
	}
}

// keylessTrustedRootPath returns the trusted-root path from the
// TRUSTED_ROOT_PATH env var wired by the verify-sut compose service,
// falling back to the shared-volume default when unset.
func keylessTrustedRootPath() string {
	if p := os.Getenv("TRUSTED_ROOT_PATH"); p != "" {
		return p
	}
	return "/shared/trusted_root.json"
}

// keylessIssuer returns the OIDC issuer from the OIDC_ISSUER env var
// wired by the verify-sut compose service, falling back to the Dex
// issuer default when unset.
func keylessIssuer() string {
	if v := os.Getenv("OIDC_ISSUER"); v != "" {
		return v
	}
	return "http://dex-idp:5556/dex"
}

// keylessIdentity returns the expected signer identity from the
// OIDC_IDENTITY env var wired by the verify-sut compose service,
// falling back to the Dex static-user default when unset.
func keylessIdentity() string {
	if v := os.Getenv("OIDC_IDENTITY"); v != "" {
		return v
	}
	return "admin@example.com"
}

// findTableRow returns the first line in out that contains needle. It
// fails the test when no such line exists. Callers use it to scope a
// column assertion to a specific policy's row rather than the whole
// table, so a value in an unrelated row or column cannot mask a
// regression.
func findTableRow(t *testing.T, out, needle string) string {
	t.Helper()

	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}

	require.Failf(t, "table row not found",
		"no row containing %q in output:\n%s", needle, out)
	return ""
}
