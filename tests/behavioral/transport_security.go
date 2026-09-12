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

// HTTPSchemeRejected verifies the CLI rejects registry URLs with http://
// scheme and requires https:// or localhost (CTRL05.AR01).
func HTTPSchemeRejected(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, result, msg, conf := verifyContext(payload)
	if ctx == nil {
		return result, msg, conf
	}

	configYAML := fmt.Sprintf(`policies:
  - url: http://evil.example.com/policies/%s
    id: %s
`, ctx.PolicyID, ctx.PolicyID)

	configDir := filepath.Join(ctx.WorkDir, complytime.WorkspaceDir)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return gemara.Unknown, "failed to create workspace dir: " + err.Error(), gemara.Undetermined
	}
	configPath := filepath.Join(configDir, complytime.WorkspaceConfigFile)
	if err := os.WriteFile(configPath, []byte(configYAML), 0600); err != nil {
		return gemara.Unknown, "failed to write config: " + err.Error(), gemara.Undetermined
	}

	cmd := exec.Command(ctx.Binary, "get") //nolint:gosec // G204 - binary path is from controlled test context
	cmd.Dir = ctx.WorkDir
	cmd.Env = ctx.Env
	output, err := cmd.CombinedOutput()

	if err == nil {
		return gemara.Failed, "get succeeded with http:// registry URL; expected rejection", gemara.High
	}
	if strings.Contains(string(output), "http://") || strings.Contains(string(output), "insecure") {
		return gemara.Passed, "http:// registry URL correctly rejected", gemara.High
	}
	return gemara.Failed, "get failed but error does not mention insecure scheme: " + string(output), gemara.High
}

// HTTPSSchemeNoPlainHTTP verifies that an https:// registry URL does not
// enable plainHTTP on the OCI client (CTRL05.AR02).
func HTTPSSchemeNoPlainHTTP(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, result, msg, conf := verifyContext(payload)
	if ctx == nil {
		return result, msg, conf
	}

	configYAML := fmt.Sprintf(`policies:
  - url: https://registry.example.com/%s
    id: %s
targets:
  - id: test-target
    policies:
      - %s
`, ctx.PolicyID, ctx.PolicyID, ctx.PolicyID)

	configDir := filepath.Join(ctx.WorkDir, complytime.WorkspaceDir)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return gemara.Unknown, "failed to create workspace dir: " + err.Error(), gemara.Undetermined
	}
	configPath := filepath.Join(configDir, complytime.WorkspaceConfigFile)
	if err := os.WriteFile(configPath, []byte(configYAML), 0600); err != nil {
		return gemara.Unknown, "failed to write config: " + err.Error(), gemara.Undetermined
	}

	cmd := exec.Command(ctx.Binary, "get") //nolint:gosec // G204 - binary path is from controlled test context
	cmd.Dir = ctx.WorkDir
	cmd.Env = ctx.Env
	output, _ := cmd.CombinedOutput()

	if strings.Contains(string(output), "plainHTTP") || strings.Contains(string(output), "plain HTTP") {
		return gemara.Failed, "https:// registry URL incorrectly enabled plainHTTP", gemara.High
	}

	return gemara.Passed, "https:// registry URL does not enable plainHTTP", gemara.High
}
