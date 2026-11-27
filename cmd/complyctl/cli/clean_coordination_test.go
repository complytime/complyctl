// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package cli

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/evidencecleanup"
	"github.com/complytime/complyctl/internal/workspacecoord"
)

// Run Cobra, its hooks, and the real cleanup service in the existing host
// subprocess. No lock or activity implementation is substituted by this helper.
func coordCleanMain(preview bool) error {
	cmd := New()
	if os.Getenv(coordModeEnv) == "cleanup-first" {
		// Only the clock is controlled. Run captures it under its own exclusive
		// access, then performs real discovery, preflight, and confined deletion.
		common := &Common{Workspace: filepath.Join(os.Getenv(coordRootEnv), "workspace")}
		cmd = cleanCmdWithOptions(&cleanOptions{
			Common: common, runCleanup: evidencecleanup.Run,
			clock: cleanClock(func() time.Time {
				coordBarrier("cleanup-exclusive")
				return time.Date(2026, time.November, 18, 12, 0, 0, 0, time.UTC)
			}),
		})
		cmd.SetArgs([]string{"--older-than", "30d"})
		return cmd.Execute()
	}
	args := []string{"clean", "--workspace", filepath.Join(os.Getenv(coordRootEnv), "workspace"),
		"--older-than", "30d"}
	if preview {
		args = append(args, "--dry-run")
	}
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestCleanCommands_CleanupFirstRefusesBeforeProviderStartup(t *testing.T) {
	root, workspace, unit, report := setupCleanCoordHost(t)
	before := snapshotCleanWorkspace(t, workspace)
	listener := coordListener(t)
	clean, cleanLogs := startCoordHost(t, root, "clean", "cleanup-first", listener.Addr().String())
	conn := awaitCoordStage(t, listener, "cleanup-exclusive")
	for _, command := range []string{workspacecoord.OperationScan, workspacecoord.OperationGenerate} {
		t.Run(command, func(t *testing.T) {
			host, logs := startCoordHost(t, root, command, "", "")
			require.Error(t, host.Wait())
			assert.Contains(t, logs.String(), "workspace busy")
			assert.NoFileExists(t, filepath.Join(root, "controlled-provider.pid"))
			assert.NoDirExists(t, coordRecords(workspace))
			assert.NoDirExists(t, filepath.Join(workspace, complytime.WorkspaceDir, "fixture"))
			assertCleanWorkspaceUnchanged(t, workspace, before)
		})
	}
	otherRoot, otherWorkspace := setupCoordHost(t)
	other, otherLogs := startCoordHost(t, otherRoot, workspacecoord.OperationGenerate, "", "")
	require.NoError(t, other.Wait(), otherLogs.String())
	assert.FileExists(t, filepath.Join(otherRoot, "controlled-provider.pid"))
	assert.FileExists(t, filepath.Join(otherWorkspace, complytime.WorkspaceDir, "fixture", "generated.json"))
	assertCoordResolved(t, otherWorkspace, false)
	assertCleanWorkspaceUnchanged(t, workspace, before)
	releaseCoordStage(t, conn)
	require.NoError(t, clean.Wait(), cleanLogs.String())
	assert.Contains(t, cleanLogs.String(), "members deleted=2")
	assert.NoDirExists(t, filepath.Join(workspace, unit))
	assert.NoFileExists(t, filepath.Join(workspace, report))
	assertCoordResolved(t, workspace, false)
	// Busy invocations fail rather than queue. A fresh invocation must now reach
	// provider startup, strictly after cleanup completed and removed its group.
	for _, command := range []string{workspacecoord.OperationScan, workspacecoord.OperationGenerate} {
		host, logs := startCoordHost(t, root, command, "", listener.Addr().String())
		startup := awaitCoordStage(t, listener, "startup")
		assertCoordActive(t, workspace, command)
		assert.NoDirExists(t, filepath.Join(workspace, unit))
		assert.NoFileExists(t, filepath.Join(workspace, report))
		releaseCoordStage(t, startup)
		releaseCoordStage(t, awaitCoordStage(t, listener, "generate"))
		if command == workspacecoord.OperationScan {
			releaseCoordStage(t, awaitCoordStage(t, listener, "scan"))
		}
		releaseCoordStage(t, awaitCoordStage(t, listener, "shutdown"))
		require.NoError(t, host.Wait(), logs.String())
		assertCoordResolved(t, workspace, false)
	}
}

func setupCleanCoordHost(t *testing.T) (root, workspace, unit, report string) {
	t.Helper()
	root, workspace = setupCoordHost(t)
	requireCleanSupport(t, workspace)
	unit = addCleanEvaluation(t, workspace, "long-running", cleanTestStart)
	// Neither a very old collection start nor apparently completed metadata
	// authorizes deletion while a supported invocation holds workspace access.
	require.NoError(t, os.WriteFile(filepath.Join(workspace, unit, "metadata.json"),
		[]byte(`{"complete":true}`), 0o600))
	report = addCleanReport(t, workspace, unit)
	return root, workspace, unit, report
}

func assertCleanCoordRefused(t *testing.T, root, workspace, reason string) {
	t.Helper()
	before := snapshotCleanWorkspace(t, workspace)
	for _, command := range []string{"clean-preview", "clean"} {
		cmd, logs := startCoordHost(t, root, command, "", "")
		require.Error(t, cmd.Wait(), "cleanup unexpectedly succeeded: %s", logs.String())
		assert.Contains(t, logs.String(), reason)
		assertCleanWorkspaceUnchanged(t, workspace, before)
	}
}

func assertCleanCoordExpired(t *testing.T, root, workspace, unit, report string, published bool) {
	t.Helper()
	cmd, logs := startCoordHost(t, root, "clean-preview", "", "")
	assertCleanCoordResult(t, cmd.Wait(), logs.String(), published)
	assert.Contains(t, logs.String(), "members would-delete=2")
	assert.FileExists(t, filepath.Join(workspace, unit, "proof.json"))
	assert.FileExists(t, filepath.Join(workspace, report))
	cmd, logs = startCoordHost(t, root, "clean", "", "")
	assertCleanCoordResult(t, cmd.Wait(), logs.String(), published)
	assert.Contains(t, logs.String(), "members deleted=2")
	assert.NoDirExists(t, filepath.Join(workspace, unit))
	assert.NoFileExists(t, filepath.Join(workspace, report))
}

func assertCleanCoordResult(t *testing.T, err error, text string, published bool) {
	t.Helper()
	if published {
		// The unchanged Phase 1 provider returns inline assessments, not file
		// evidence. Its new reports have no evaluation age and remain blocked;
		// that bounded group must not prevent expiry of the old independent group.
		require.Error(t, err, text)
		assert.Contains(t, text, "report has no establishable evaluation age")
		assert.Contains(t, text, "retained=2")
	} else {
		require.NoError(t, err, text)
	}
	assert.NotContains(t, text, "workspace busy")
	assert.NotContains(t, text, "unresolved activity")
}

func assertCleanCoordReports(t *testing.T, workspace string, count int) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(workspace, complytime.WorkspaceDir, complytime.ScanOutputDir))
	require.NoError(t, err)
	assert.Len(t, entries, count)
}

func TestCleanCommands_ProviderFirstProtectsOldEvaluation(t *testing.T) {
	for _, command := range []string{workspacecoord.OperationScan, workspacecoord.OperationGenerate} {
		t.Run(command, func(t *testing.T) {
			root, workspace, unit, report := setupCleanCoordHost(t)
			listener := coordListener(t)
			cmd, logs := startCoordHost(t, root, command, "", listener.Addr().String())
			stages := []string{"startup", "generate"}
			if command == workspacecoord.OperationScan {
				stages = append(stages, "scan")
			}
			stages = append(stages, "shutdown")
			for _, stage := range stages {
				conn := awaitCoordStage(t, listener, stage)
				assertCoordActive(t, workspace, command)
				assertCleanCoordRefused(t, root, workspace, "workspace busy")
				releaseCoordStage(t, conn)
			}
			require.NoError(t, cmd.Wait(), logs.String())
			assertCoordResolved(t, workspace, false)
			assertCleanCoordExpired(t, root, workspace, unit, report, command == workspacecoord.OperationScan)
		})
	}
}

func TestCleanCommands_LatePublication(t *testing.T) {
	root, workspace, unit, report := setupCleanCoordHost(t)
	listener := coordListener(t)
	cmd, logs := startCoordHost(t, root, workspacecoord.OperationScan,
		"late-publication", listener.Addr().String())
	for _, stage := range []string{"startup", "generate", "scan"} {
		releaseCoordStage(t, awaitCoordStage(t, listener, stage))
	}
	conn := awaitCoordStage(t, listener, "publication")
	// The real scan has returned from its RPC and written its EvaluationLog,
	// but the existing formatter seam delays Markdown publication in this child.
	assertCleanCoordReports(t, workspace, 2) // Seeded report plus the new EvaluationLog.
	assertCoordActive(t, workspace, workspacecoord.OperationScan)
	assertCleanCoordRefused(t, root, workspace, "workspace busy")
	releaseCoordStage(t, conn)
	conn = awaitCoordStage(t, listener, "shutdown")
	assertCleanCoordReports(t, workspace, 3)
	assertCleanCoordRefused(t, root, workspace, "workspace busy")
	releaseCoordStage(t, conn)
	require.NoError(t, cmd.Wait(), logs.String())
	assertCoordResolved(t, workspace, false)
	assertCleanCoordExpired(t, root, workspace, unit, report, true)
	assertCleanCoordReports(t, workspace, 2) // New reports have no expirable evaluation age.
}

func TestCleanCommands_HandledFailureClearsAfterTeardown(t *testing.T) {
	for _, tc := range []struct{ command, mode, message string }{
		{workspacecoord.OperationScan, "scan-handled", "handled scan failure"},
		{workspacecoord.OperationGenerate, "generate-handled", "handled generation failure"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			root, workspace, unit, report := setupCleanCoordHost(t)
			listener := coordListener(t)
			cmd, logs := startCoordHost(t, root, tc.command, tc.mode, listener.Addr().String())
			for _, stage := range []string{"startup", "generate"} {
				releaseCoordStage(t, awaitCoordStage(t, listener, stage))
			}
			if tc.command == workspacecoord.OperationScan {
				releaseCoordStage(t, awaitCoordStage(t, listener, "scan"))
			}
			conn := awaitCoordStage(t, listener, "shutdown")
			if tc.command == workspacecoord.OperationScan {
				assertCleanCoordReports(t, workspace, 3)
			}
			assertCoordActive(t, workspace, tc.command)
			assertCleanCoordRefused(t, root, workspace, "workspace busy")
			releaseCoordStage(t, conn)
			require.Error(t, cmd.Wait())
			assert.Contains(t, logs.String(), tc.message)
			assert.NotContains(t, logs.String(), "retaining uncertain activity record")
			assertCoordResolved(t, workspace, false)
			assertCleanCoordExpired(t, root, workspace, unit, report, tc.command == workspacecoord.OperationScan)
		})
	}
}

func assertCleanCoordUnresolved(t *testing.T, root, workspace string) {
	t.Helper()
	// Exclusive access is now obtainable: the persistent host marker, not a
	// still-held kernel lock or a process-liveness guess, must block cleanup.
	assertCoordResolved(t, workspace, true)
	entries, err := os.ReadDir(coordRecords(workspace))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assertCleanCoordRefused(t, root, workspace, filepath.Join(coordRecords(workspace), entries[0].Name()))
}

func TestCleanCommands_UncertainFailureRetainsAndBlocks(t *testing.T) {
	for _, tc := range []struct{ command, mode string }{
		{workspacecoord.OperationScan, "scan-rpc"},
		{workspacecoord.OperationGenerate, "generate-rpc"},
		{workspacecoord.OperationScan, "crash"},
		{workspacecoord.OperationScan, "abnormal-shutdown"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			root, workspace, _, _ := setupCleanCoordHost(t)
			cmd, logs := startCoordHost(t, root, tc.command, tc.mode, "")
			require.Error(t, cmd.Wait())
			assert.Contains(t, logs.String(), "retaining uncertain activity record")
			assertCleanCoordUnresolved(t, root, workspace)
		})
	}
}

func TestCleanCommands_ForcedShutdownRetainsAndBlocks(t *testing.T) {
	root, workspace, _, _ := setupCleanCoordHost(t)
	listener := coordListener(t)
	cmd, logs := startCoordHost(t, root, workspacecoord.OperationScan, "", listener.Addr().String())
	for _, stage := range []string{"startup", "generate", "scan"} {
		releaseCoordStage(t, awaitCoordStage(t, listener, stage))
	}
	conn := awaitCoordStage(t, listener, "shutdown")
	assertCleanCoordRefused(t, root, workspace, "workspace busy")
	// Leave Serve's exit barrier closed. The existing go-plugin shutdown bound
	// forces this controlled provider to exit; that does not resolve uncertainty.
	require.Error(t, cmd.Wait())
	assert.Contains(t, logs.String(), "retaining uncertain activity record")
	_, err := io.ReadAll(conn)
	require.NoError(t, err)
	assertCleanCoordUnresolved(t, root, workspace)
}

func TestCleanCommands_KilledHostRetainsAndBlocks(t *testing.T) {
	root, workspace, _, _ := setupCleanCoordHost(t)
	listener := coordListener(t)
	cmd, _ := startCoordHost(t, root, workspacecoord.OperationScan, "", listener.Addr().String())
	for _, stage := range []string{"startup", "generate"} {
		releaseCoordStage(t, awaitCoordStage(t, listener, stage))
	}
	conn := awaitCoordStage(t, listener, "scan")
	assertCleanCoordRefused(t, root, workspace, "workspace busy")
	pidBytes, err := os.ReadFile(filepath.Join(root, "controlled-provider.pid"))
	require.NoError(t, err)
	pid, err := strconv.Atoi(string(pidBytes))
	require.NoError(t, err)
	process, err := os.FindProcess(pid)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = process.Kill() // Only this fixture's provider, never a PID from an activity record.
		_ = process.Release()
	})
	require.NoError(t, cmd.Process.Kill())
	// Reap the known fixture before Wait; its inherited pipes must not hang Wait.
	// This does not claim that killing a provider stops arbitrary descendants.
	require.NoError(t, process.Kill())
	_, err = io.ReadAll(conn)
	require.NoError(t, err)
	require.Error(t, cmd.Wait())
	assertCleanCoordUnresolved(t, root, workspace)
}

func TestCleanCommands_DifferentWorkspacesIndependent(t *testing.T) {
	root, workspace, unit, report := setupCleanCoordHost(t)
	listener := coordListener(t)
	cmd, logs := startCoordHost(t, root, workspacecoord.OperationScan, "", listener.Addr().String())
	conn := awaitCoordStage(t, listener, "startup")
	otherRoot, otherWorkspace, otherUnit, otherReport := setupCleanCoordHost(t)
	assertCleanCoordExpired(t, otherRoot, otherWorkspace, otherUnit, otherReport, false)
	other, otherLogs := startCoordHost(t, otherRoot, workspacecoord.OperationGenerate, "", "")
	require.NoError(t, other.Wait(), otherLogs.String())
	assertCoordResolved(t, otherWorkspace, false)
	assertCleanCoordRefused(t, root, workspace, "workspace busy")
	releaseCoordStage(t, conn)
	for _, stage := range []string{"generate", "scan", "shutdown"} {
		releaseCoordStage(t, awaitCoordStage(t, listener, stage))
	}
	require.NoError(t, cmd.Wait(), logs.String())
	assertCleanCoordExpired(t, root, workspace, unit, report, true)
}
