// SPDX-License-Identifier: Apache-2.0

package behavioral

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gemaraproj/go-gemara"
)

// MatchedEvaluatorRouting verifies generate routes to the matched provider
// when the policy graph specifies an evaluator ID (CTRL04.AR01).
func MatchedEvaluatorRouting(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, result, msg, conf := verifyContext(payload)
	if ctx == nil {
		return result, msg, conf
	}

	output, err := ctx.RunBinary("generate", "--policy-id", ctx.PolicyID)
	if err != nil {
		return gemara.Failed, "generate failed: " + output, gemara.High
	}
	if strings.Contains(output, "Generation completed.") {
		return gemara.Passed, "generate routed to matched provider", gemara.High
	}
	return gemara.Failed, "generate did not complete successfully: " + output, gemara.High
}

// EvaluatorMismatchRejected renames the installed test provider so the
// evaluator ID no longer matches, then verifies generate fails (CTRL04.AR02).
func EvaluatorMismatchRejected(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, result, msg, conf := verifyContext(payload)
	if ctx == nil {
		return result, msg, conf
	}

	providerDir := filepath.Join(ctx.HomeDir, ".complytime", "providers")
	oldPath := filepath.Join(providerDir, "complyctl-provider-test")
	newPath := filepath.Join(providerDir, "complyctl-provider-other")
	if err := os.Rename(oldPath, newPath); err != nil {
		return gemara.Unknown, "failed to rename provider: " + err.Error(), gemara.Undetermined
	}

	cmd := exec.Command(ctx.Binary, "generate", "--policy-id", ctx.PolicyID) //nolint:gosec // G204 - binary/args from controlled test context
	cmd.Dir = ctx.WorkDir
	cmd.Env = ctx.Env
	output, err := cmd.CombinedOutput()

	if err == nil {
		return gemara.Failed, "generate succeeded with mismatched evaluator; expected failure", gemara.High
	}
	if strings.Contains(string(output), "no provider registered for evaluator") {
		return gemara.Passed, "evaluator mismatch correctly rejected", gemara.High
	}
	return gemara.Failed, "generate failed but error unclear: " + string(output), gemara.High
}

// PluginBinaryIntegrityCheck tests whether the provider discovery process
// verifies binary integrity before launching a subprocess (CTRL07.AR01).
// NOT IMPLEMENTED: discovery matches only on filename prefix and executable bit.
// This test documents the gap and is expected to fail.
func PluginBinaryIntegrityCheck(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, result, msg, conf := verifyContext(payload)
	if ctx == nil {
		return result, msg, conf
	}

	providerDir := filepath.Join(ctx.HomeDir, ".complytime", "providers")
	providerPath := filepath.Join(providerDir, "complyctl-provider-test")
	data, err := os.ReadFile(providerPath)
	if err != nil {
		return gemara.Unknown, "could not read provider binary: " + err.Error(), gemara.Undetermined
	}

	hash := sha256.Sum256(data)
	_ = hex.EncodeToString(hash[:])

	tampered := make([]byte, len(data))
	copy(tampered, data)
	if len(tampered) > 0 {
		tampered[len(tampered)-1] ^= 0xFF
	}
	if err := os.WriteFile(providerPath, tampered, 0700); err != nil { //nolint:gosec // G306 - test needs execute permission
		return gemara.Unknown, "could not write tampered binary: " + err.Error(), gemara.Undetermined
	}

	cmd := exec.Command(ctx.Binary, "generate", "--policy-id", ctx.PolicyID) //nolint:gosec // G204 - controlled test context
	cmd.Dir = ctx.WorkDir
	cmd.Env = ctx.Env
	output, err := cmd.CombinedOutput()

	if err != nil && strings.Contains(string(output), "integrity") {
		return gemara.Passed, "tampered binary rejected with integrity error", gemara.High
	}

	return gemara.Failed, "no binary integrity verification — tampered provider was not detected", gemara.High
}

// PluginSubprocessIsolation tests whether provider subprocesses run with
// restricted privileges (CTRL08.AR01).
// NOT IMPLEMENTED: providers run with same OS privileges as parent process.
// This test documents the gap and is expected to fail.
func PluginSubprocessIsolation(payload any) (gemara.Result, string, gemara.ConfidenceLevel) {
	_, result, msg, conf := verifyContext(payload)
	if result != 0 {
		return result, msg, conf
	}

	return gemara.Failed, "no subprocess isolation — providers run with parent process privileges", gemara.High
}
