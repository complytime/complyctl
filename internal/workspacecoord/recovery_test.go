// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package workspacecoord

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckNoActivity_RecoveryMessage(t *testing.T) {
	for _, kind := range []string{"record", "malformed", "unreadable", "unsafe-directory"} {
		t.Run(kind, func(t *testing.T) {
			access := acquireForTest(t, t.TempDir(), Exclusive)
			activity := beginForTest(t, access)
			path := activity.Path()
			switch kind {
			case "malformed":
				path = filepath.Join(filepath.Dir(path), "untrusted\n$(touch sentinel).json")
				require.NoError(t, os.Rename(activity.Path(), path))
				require.NoError(t, os.WriteFile(path,
					[]byte(`{"workspace":"untrusted-workspace","command":"untrusted-command"`), 0o600))
			case "unreadable":
				require.NoError(t, os.Chmod(path, 0))
			case "unsafe-directory":
				require.NoError(t, activity.CompleteNormal())
				path = filepath.Dir(path)
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.WriteFile(path, []byte("not a directory"), 0o600))
			}
			err := access.CheckNoActivity()
			require.ErrorIs(t, err, ErrUnresolvedActivity)
			var blocked *ActivityError
			require.ErrorAs(t, err, &blocked)
			assert.Equal(t, []string{path}, blocked.Paths)
			message := err.Error()
			for _, text := range []string{
				fmt.Sprintf("workspace %q", access.Path()), fmt.Sprintf("%q", path),
				"no evidence was deleted and no reports were deleted",
				"pause scheduled jobs and prevent new scans", "Stop all writers",
				"hosts, providers, and provider-launched child tools",
				"If shutdown is uncertain, restart the machine or VM",
				"complete isolated container environment", "restarting a terminal",
				"complyctl alone, or an unrelated container is insufficient",
				"Prevent automatic job restarts", "other machines/environments",
				"or do not proceed", "all writers have stopped and new scans remain paused",
				"ONLY the exact reported activity records", "Do not remove coordination directories",
				"inspection and repair, not recursive removal", "not proof of inactivity",
				"not proof of inactivity or authority to kill processes",
				"no automatic stale-record or force override", "cannot bypass a live workspace lock",
				"Rerun cleanup preview", "docs/EVIDENCE.md#interrupted-scan-recovery",
			} {
				assert.Contains(t, message, text)
			}
			assert.NotContains(t, message, "untrusted-workspace")
			assert.NotContains(t, message, "untrusted-command")
			assert.NotContains(t, message, "untrusted\n")
			_, err = os.Lstat(path)
			assert.NoError(t, err, "diagnosis must retain the exact blocked entry")
		})
	}
}

// checkPreviewCoordination exercises only the read-only coordination boundary;
// the cleanup CLI and the remaining preview safety checks are not implemented yet.
func checkPreviewCoordination(t *testing.T, workspace string) error {
	t.Helper()
	access, err := Acquire(Options{Workspace: workspace, Mode: Exclusive})
	if err != nil {
		return err
	}
	defer func() { require.NoError(t, access.Close()) }()
	return access.CheckNoActivity()
}

func TestActivity_ManualRecoveryRemovesOnlyStoppedRecords(t *testing.T) {
	workspace := t.TempDir()
	first, finishFirst := startActivityProcess(t, workspace)
	second, finishSecond := startActivityProcess(t, workspace)
	// Each barrier stops and waits for its controlled provider and host. Lost
	// completion signals retain both records despite those known test exits.
	finishFirst("uncertain")
	finishSecond("uncertain")
	secondContents, err := os.ReadFile(second)
	require.NoError(t, err)
	retained := map[string][]byte{
		filepath.Join(workspace, "report.json"):   []byte("retained report\n"),
		filepath.Join(workspace, "evidence.json"): []byte("retained evidence\n"),
	}
	for path, contents := range retained {
		require.NoError(t, os.WriteFile(path, contents, 0o600))
	}
	var blocked *ActivityError
	err = checkPreviewCoordination(t, workspace)
	require.ErrorAs(t, err, &blocked)
	assert.ElementsMatch(t, []string{first, second}, blocked.Paths)

	// Simulate the operator, not a recovery API: no automatic removal is offered.
	require.NoError(t, os.Remove(first))
	err = checkPreviewCoordination(t, workspace)
	require.ErrorIs(t, err, ErrUnresolvedActivity)
	require.ErrorAs(t, err, &blocked)
	assert.Equal(t, []string{second}, blocked.Paths)
	actual, err := os.ReadFile(second)
	require.NoError(t, err)
	assert.Equal(t, secondContents, actual)
	require.NoError(t, os.Remove(second))
	require.NoError(t, checkPreviewCoordination(t, workspace))
	entries, err := os.ReadDir(filepath.Dir(first))
	require.NoError(t, err)
	assert.Empty(t, entries, "preview-like check does not create replacement records")
	for path, contents := range retained {
		actual, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, contents, actual)
	}
}

func TestActivity_PreviewCoordinationCannotBypassLiveLock(t *testing.T) {
	workspace := t.TempDir()
	path, finish := startActivityProcess(t, workspace)
	require.ErrorIs(t, checkPreviewCoordination(t, workspace), ErrBusy)
	// Deliberately violate recovery conditions to prove the lock is independent
	// of its persistent record. This is not a supported operator recovery step.
	require.NoError(t, os.Remove(path))
	require.ErrorIs(t, checkPreviewCoordination(t, workspace), ErrBusy)
	finish("uncertain")
	require.NoError(t, checkPreviewCoordination(t, workspace))
}
