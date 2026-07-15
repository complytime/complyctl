// SPDX-License-Identifier: Apache-2.0

package behavioral

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/gemaraproj/go-gemara"
)

// BehavioralContext carries the runtime environment needed by behavioral
// assessment steps. Each step receives this as its payload via the
// gemara.AssessmentStep interface.
type BehavioralContext struct {
	Binary             string
	TestProviderBinary string
	HomeDir            string
	WorkDir            string
	Env                []string
	PolicyID           string
	RegistryURL        string
}

// RunBinary executes the complyctl binary with the given args in the
// context's working directory and returns combined output.
func (c *BehavioralContext) RunBinary(args ...string) (string, error) {
	cmd := exec.Command(c.Binary, args...) //nolint:gosec // Binary path is set by the test harness, not user input.
	cmd.Dir = c.WorkDir
	cmd.Env = c.Env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func verifyContext(payload any) (*BehavioralContext, gemara.Result, string, gemara.ConfidenceLevel) {
	ctx, ok := payload.(*BehavioralContext)
	if !ok {
		return nil, gemara.Unknown,
			fmt.Sprintf("expected *BehavioralContext, got %T", payload),
			gemara.Undetermined
	}
	return ctx, 0, "", 0
}

// cacheDir returns the XDG cache directory for a given home:
// $HOME/.cache/complytime (Linux default when XDG_CACHE_HOME is unset).
func cacheDir(homeDir string) string {
	return filepath.Join(homeDir, ".cache", "complytime")
}

// dataDir returns the XDG data directory for a given home:
// $HOME/.local/share/complytime (Linux default when XDG_DATA_HOME is unset).
func dataDir(homeDir string) string {
	return filepath.Join(homeDir, ".local", "share", "complytime")
}
