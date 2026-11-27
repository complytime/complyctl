// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

const preparationLimitHelper = "COMPLYCTL_TEST_PREPARATION_LIMIT"

func TestPreview_PreparationDescriptorLimit(t *testing.T) {
	if os.Getenv(preparationLimitHelper) == "1" {
		checkPreviewDescriptorLimit(t)
		return
	}
	executable, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestPreview_PreparationDescriptorLimit$")
	cmd.Env = append(os.Environ(), preparationLimitHelper+"=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
}

func checkPreviewDescriptorLimit(t *testing.T) {
	t.Helper()
	workspace := t.TempDir()
	requirePreviewSupport(t, workspace)
	unit := addGroupEvaluation(t, workspace, "opa", "descriptor-limit", groupOldStart)
	root, err := os.OpenRoot(filepath.Join(workspace, unit))
	require.NoError(t, err)
	for i := range 512 {
		require.NoError(t, root.WriteFile(fmt.Sprintf("extra-%04d.json", i), []byte("{}"), 0o600))
	}
	require.NoError(t, root.Close())
	before := previewSnapshot(t, workspace)
	var original unix.Rlimit
	require.NoError(t, unix.Getrlimit(unix.RLIMIT_NOFILE, &original))
	require.GreaterOrEqual(t, original.Max, uint64(256))
	limited := original
	limited.Cur = 256
	require.NoError(t, unix.Setrlimit(unix.RLIMIT_NOFILE, &limited))
	t.Cleanup(func() { require.NoError(t, unix.Setrlimit(unix.RLIMIT_NOFILE, &original)) })
	preview, previewErr := Preview(previewOptions(t, workspace))
	result, runErr := Run(Options{Workspace: workspace, OlderThan: "30d",
		Clock: &previewClock{now: groupAge(t).CapturedAt()}})
	require.NoError(t, unix.Setrlimit(unix.RLIMIT_NOFILE, &original))
	require.ErrorIs(t, runErr, ErrPreflightBlocked)
	require.NotNil(t, result)
	assert.Zero(t, result.Totals.DeletedMembers)
	assert.Equal(t, before, previewSnapshot(t, workspace))
	require.ErrorIs(t, previewErr, ErrPreflightBlocked)
	require.NotNil(t, preview)
	assert.Zero(t, preview.Totals.WouldDeleteGroups)
	assert.Equal(t, 1, preview.Totals.RetainedGroups)
}
