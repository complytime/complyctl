// SPDX-License-Identifier: Apache-2.0

package behavioral

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gemaraproj/go-gemara"

	"github.com/complytime/complyctl/internal/complytime"
)

// UnsetEnvVarFails verifies config loading fails with a descriptive error
// when a referenced environment variable is not set (CTRL03.AR01).
func UnsetEnvVarFails(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, result, msg, conf := verifyContext(payload)
	if ctx == nil {
		return result, msg, conf
	}

	configYAML := fmt.Sprintf(`policies:
  - url: %s/%s
    id: %s
targets:
  - id: e2e-target
    policies:
      - %s
    variables:
      secret_token: ${COMPLYTIME_E2E_NONEXISTENT_VAR}
`, ctx.RegistryURL, ctx.PolicyID, ctx.PolicyID, ctx.PolicyID)

	configDir := filepath.Join(ctx.WorkDir, complytime.WorkspaceDir)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return gemara.Unknown, "failed to create workspace dir: " + err.Error(), gemara.Undetermined
	}
	configPath := filepath.Join(configDir, complytime.WorkspaceConfigFile)
	if err := os.WriteFile(configPath, []byte(configYAML), 0600); err != nil {
		return gemara.Unknown, "failed to write config: " + err.Error(), gemara.Undetermined
	}

	cmd := exec.Command(ctx.Binary, "get") //nolint:gosec // Binary path is set by the test harness, not user input.
	cmd.Dir = ctx.WorkDir
	cmd.Env = ctx.Env
	output, err := cmd.CombinedOutput()

	if err == nil {
		return gemara.Failed, "config with unset env var did not fail", gemara.High
	}
	if strings.Contains(string(output), "COMPLYTIME_E2E_NONEXISTENT_VAR") {
		return gemara.Passed, "config loading failed with descriptive error naming the missing variable", gemara.High
	}
	return gemara.Failed, "config failed but error does not name the variable: " + string(output), gemara.High
}

// EnvVarResolution verifies ${VAR} references resolve from the process
// environment at config load time (CTRL03.AR02).
func EnvVarResolution(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, result, msg, conf := verifyContext(payload)
	if ctx == nil {
		return result, msg, conf
	}

	configYAML := fmt.Sprintf(`policies:
  - url: %s/%s
    id: %s
targets:
  - id: e2e-target
    policies:
      - %s
    variables:
      env: ${COMPLYTIME_E2E_TEST_ENV}
`, ctx.RegistryURL, ctx.PolicyID, ctx.PolicyID, ctx.PolicyID)

	configDir := filepath.Join(ctx.WorkDir, complytime.WorkspaceDir)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return gemara.Unknown, "failed to create workspace dir: " + err.Error(), gemara.Undetermined
	}
	configPath := filepath.Join(configDir, complytime.WorkspaceConfigFile)
	if err := os.WriteFile(configPath, []byte(configYAML), 0600); err != nil {
		return gemara.Unknown, "failed to write config: " + err.Error(), gemara.Undetermined
	}

	env := append(ctx.Env, "COMPLYTIME_E2E_TEST_ENV=staging")
	cmd := exec.Command(ctx.Binary, "get") //nolint:gosec // Binary path is set by the test harness, not user input.
	cmd.Dir = ctx.WorkDir
	cmd.Env = env
	output, err := cmd.CombinedOutput()

	if err != nil {
		return gemara.Failed, "get failed with valid env var reference: " + string(output), gemara.High
	}
	if strings.Contains(string(output), "Synchronization completed.") {
		return gemara.Passed, "env var reference resolved successfully", gemara.High
	}
	return gemara.Failed, "get succeeded but output unexpected: " + string(output), gemara.High
}
