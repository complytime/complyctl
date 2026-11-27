// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package cli

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/evidencecleanup"
	"github.com/complytime/complyctl/internal/output"
	"github.com/complytime/complyctl/internal/workspacecoord"
	"github.com/complytime/complyctl/pkg/provider"
)

const cleanTestStart = "20200101T000000Z"

func TestCleanError_PreservesCauseAndEscapesControls(t *testing.T) {
	assert.NoError(t, cleanError(nil))
	cause := &os.PathError{Op: "open", Path: "bad\x1b[2J\npath", Err: os.ErrPermission}
	err := cleanError(cause)
	assert.ErrorIs(t, err, os.ErrPermission)
	var pathError *os.PathError
	require.ErrorAs(t, err, &pathError)
	assert.Same(t, cause, pathError)
	assert.NotContains(t, err.Error(), "\x1b")
	assert.NotContains(t, err.Error(), "\n")
	assert.Contains(t, err.Error(), `\x1b[2J\npath`)
}

func TestCleanCmd_MalformedReportControlsAreEscaped(t *testing.T) {
	for _, preview := range []bool{true, false} {
		t.Run(map[bool]string{true: "preview", false: "execute"}[preview], func(t *testing.T) {
			workspace := t.TempDir()
			requireCleanSupport(t, workspace)
			addCleanEvaluation(t, workspace, "target", cleanTestStart)
			root, err := os.OpenRoot(workspace)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, root.Close()) })
			require.NoError(t, root.MkdirAll(".complytime/scan", 0o700))
			require.NoError(t, root.WriteFile(
				".complytime/scan/evaluation-log-policy-target-20200101-000000.yaml",
				[]byte("metadata:\n  id: policy\n  # \x1b[2J\x1b[HINJECTED\n  id: duplicate\n"), 0o600))
			before := snapshotCleanWorkspace(t, workspace)
			args := []string{"--workspace", workspace, "--older-than", "30d"}
			if preview {
				args = append(args, "--dry-run")
			}
			text, err := executeClean(t, args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `\x1b[2J`)
			assert.NotContains(t, err.Error()+text, "\x1b")
			assertCleanWorkspaceUnchanged(t, workspace, before)
		})
	}
}

func executeClean(t *testing.T, args ...string) (string, error) {
	t.Helper()
	saveLogger(t)
	// Isolate user paths rather than suppressing the production legacy warning.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "xdg-cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"clean"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestExecuteClean_RestoresLogger(t *testing.T) {
	saveLogger(t)
	originalLogger, originalWriter := logger, lw
	t.Run("command", func(t *testing.T) {
		_, err := executeClean(t, "--older-than", "invalid")
		require.Error(t, err)
	})
	assert.Same(t, originalLogger, logger)
	assert.Equal(t, originalWriter, lw)
}

func TestExecuteClean_IsolatesLegacyHome(t *testing.T) {
	home := t.TempDir()
	environment := map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, "xdg-config"),
		"XDG_CACHE_HOME":  filepath.Join(home, "xdg-cache"),
		"XDG_DATA_HOME":   filepath.Join(home, "xdg-data"),
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
	legacy := filepath.Join(home, complytime.WorkspaceDir)
	require.NoError(t, os.Mkdir(legacy, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(legacy, complytime.StateFileName),
		[]byte("retained legacy state"), 0o600))
	var warning bytes.Buffer
	complytime.CheckLegacyDir(&warning)
	require.Contains(t, warning.String(), "WARNING: Legacy directory found at "+legacy)
	before := snapshotCleanWorkspace(t, home)
	missing := filepath.Join(t.TempDir(), "absent")
	t.Run("isolated-command", func(t *testing.T) {
		text, err := executeClean(t, "--workspace", missing, "--older-than", "30d")
		require.ErrorContains(t, err, "workspace directory does not exist")
		assert.Empty(t, text)
		assert.NoDirExists(t, missing)
		for key, inherited := range environment {
			assert.NotEqual(t, inherited, os.Getenv(key), "%s must be fixture-owned", key)
		}
	})
	for key, inherited := range environment {
		assert.Equal(t, inherited, os.Getenv(key), "%s must be restored", key)
	}
	assertCleanWorkspaceUnchanged(t, home, before)
}

func requireCleanSupport(t *testing.T, workspace string) {
	t.Helper()
	access, err := workspacecoord.Acquire(workspacecoord.Options{
		Workspace: workspace, Mode: workspacecoord.Exclusive,
	})
	require.NoError(t, err)
	supportErr := access.CheckCleanupSupport()
	require.NoError(t, access.Close())
	if errors.Is(supportErr, workspacecoord.ErrUnsupportedFilesystem) {
		t.Skipf("cleanup runtime requires a supported local filesystem: %v", supportErr)
	}
	require.NoError(t, supportErr)
}

func addCleanEvaluation(t *testing.T, workspace, label, start string) string {
	t.Helper()
	unit := filepath.Join(complytime.WorkspaceDir, "ampel", "results", label,
		"evaluation-"+start+"-0123456789abcdef0123456789abcdef")
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, unit), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, unit, "proof.json"),
		[]byte("retained proof"), 0o600))
	return unit
}

func addCleanReport(t *testing.T, workspace string, units ...string) string {
	t.Helper()
	evaluator := output.NewEvaluator("policy", "target", map[string]string{"req": "control"},
		nil, nil, []gemara.MappingReference{{Id: "retained", Title: "Retained evidence"}})
	var evidence []provider.Evidence
	for _, unit := range units {
		evidence = append(evidence, provider.Evidence{
			ID: unit, Type: "command-output", Description: "proof",
			Source: &provider.EvidenceSource{ReferenceID: "retained",
				Coordinate: filepath.Join(unit, "proof.json")},
		})
	}
	evaluator.AddTarget([]provider.AssessmentLog{{RequirementID: "req", Evidence: evidence,
		Steps: []provider.Step{{Name: "inspect", Result: provider.ResultFailed}}}})
	dir := filepath.Join(workspace, complytime.WorkspaceDir, "scan")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path, err := evaluator.Write(dir, "json")
	require.NoError(t, err)
	relative, err := filepath.Rel(workspace, path)
	require.NoError(t, err)
	return relative
}

func TestCleanCmd_RegistrationAndHelp(t *testing.T) {
	cmd, args, err := New().Find([]string{"clean"})
	require.NoError(t, err)
	assert.Equal(t, "clean", cmd.Name())
	assert.Empty(t, args)
	assert.Equal(t, "", cmd.Flags().Lookup(cleanAgeFlag).DefValue)
	assert.Equal(t, "false", cmd.Flags().Lookup("dry-run").DefValue)
	text, err := executeClean(t, "--help")
	require.NoError(t, err)
	for _, expected := range []string{"--older-than", "--dry-run", "irreversibly",
		"partial", "no rollback", "older hosts", "direct provider", "APFS", "24 hours/day"} {
		assert.Contains(t, text, expected)
	}
}

func TestCleanCmd_InvalidInvocationPreservesEvidence(t *testing.T) {
	for _, args := range [][]string{
		nil, {"--older-than", ""}, {"--older-than", "0d"}, {"--older-than=-1d"},
		{"--older-than", "1.5d"}, {"--older-than", "30h"}, {"--older-than", " 30d"},
		{"--older-than", "30d "}, {"--older-than", "01d"},
		{"--older-than", "999999999999999999999d"}, {"--older-than", "106752d"},
		{"--older-than", "30d", "extra"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			workspace := t.TempDir()
			unit := addCleanEvaluation(t, workspace, "old", cleanTestStart)
			text, err := executeClean(t, append([]string{"--workspace", workspace}, args...)...)
			require.Error(t, err)
			assert.NotContains(t, text, "deleted evaluation")
			data, readErr := os.ReadFile(filepath.Join(workspace, unit, "proof.json"))
			require.NoError(t, readErr)
			assert.Equal(t, "retained proof", string(data))
			assert.NoFileExists(t, filepath.Join(workspace, complytime.WorkspaceDir,
				complytime.WorkspaceConfigFile))
		})
	}
}

func TestCleanCmd_WorkspacePrecedence(t *testing.T) {
	for _, selection := range []string{"flag", "environment", "default"} {
		t.Run(selection, func(t *testing.T) {
			current, environment, flag := t.TempDir(), t.TempDir(), t.TempDir()
			requireCleanSupport(t, current)
			t.Chdir(current)
			t.Setenv(complytime.WorkspaceEnvVar, "")
			paths := make(map[string]string)
			for _, workspace := range []string{current, environment, flag} {
				paths[workspace] = addCleanEvaluation(t, workspace, "old", cleanTestStart)
			}
			selected := current
			args := []string{"--older-than", "30d"}
			if selection != "default" {
				t.Setenv(complytime.WorkspaceEnvVar, environment)
				selected = environment
			}
			if selection == "flag" {
				args = append(args, "--workspace", flag)
				selected = flag
			}
			text, err := executeClean(t, args...)
			require.NoError(t, err)
			assert.Contains(t, text, "Workspace: ")
			assert.Contains(t, text, selected)
			assert.Contains(t, text, "groups deleted=1 retained=0 failed=0")
			for workspace, unit := range paths {
				if workspace == selected {
					assert.NoDirExists(t, filepath.Join(workspace, unit))
				} else {
					assert.FileExists(t, filepath.Join(workspace, unit, "proof.json"))
				}
			}
		})
	}
}

func TestCleanCmd_MissingWorkspace(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	text, err := executeClean(t, "--workspace", missing, "--older-than", "30d")
	require.ErrorContains(t, err, "workspace directory does not exist")
	assert.Empty(t, text)
	assert.NoDirExists(t, missing)
}

func TestCleanCmd_NoConfigProvidersOrRegistryAndRepeat(t *testing.T) {
	workspace := t.TempDir()
	requireCleanSupport(t, workspace)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("DOCKER_CONFIG", filepath.Join(t.TempDir(), "missing-credentials"))
	unit := addCleanEvaluation(t, workspace, "old", cleanTestStart)
	report := addCleanReport(t, workspace, unit)
	text, err := executeClean(t, "--workspace", workspace, "--older-than", "30d", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, text, "would-delete evaluation")
	assert.Contains(t, text, "would-delete report")
	assert.Contains(t, text, "members would-delete=2 retained=0")
	assert.FileExists(t, filepath.Join(workspace, report))
	assert.FileExists(t, filepath.Join(workspace, unit, "proof.json"))
	config := filepath.Join(workspace, complytime.WorkspaceDir, complytime.WorkspaceConfigFile)
	assert.NoFileExists(t, config)
	// Even invalid policy configuration must not enter cleanup's dependency path.
	require.NoError(t, os.WriteFile(config, []byte("invalid: ["), 0o600))
	text, err = executeClean(t, "--workspace", workspace, "--older-than", "30d")
	require.NoError(t, err)
	assert.Contains(t, text, "members deleted=2 retained=0 partially-deleted=0")
	assert.NoFileExists(t, filepath.Join(workspace, report))
	assert.NoDirExists(t, filepath.Join(workspace, unit))
	data, err := os.ReadFile(config)
	require.NoError(t, err)
	assert.Equal(t, "invalid: [", string(data))
	text, err = executeClean(t, "--workspace", workspace, "--older-than", "30d")
	require.NoError(t, err)
	assert.Contains(t, text, "members deleted=0 retained=0 partially-deleted=0")
}

func TestCleanCmd_BlockedGroupAndIndependentDeletion(t *testing.T) {
	for _, preview := range []bool{true, false} {
		t.Run(map[bool]string{true: "preview", false: "execute"}[preview], func(t *testing.T) {
			workspace := t.TempDir()
			requireCleanSupport(t, workspace)
			old := addCleanEvaluation(t, workspace, "old", cleanTestStart)
			blocked := addCleanEvaluation(t, workspace, "blocked", cleanTestStart)
			independent := addCleanEvaluation(t, workspace, "independent", cleanTestStart)
			report := addCleanReport(t, workspace, old, blocked)
			require.NoError(t, os.Remove(filepath.Join(workspace, blocked, "proof.json")))
			before, err := os.ReadFile(filepath.Join(workspace, report))
			require.NoError(t, err)
			args := []string{"--workspace", workspace, "--older-than", "30d"}
			if preview {
				args = append(args, "--dry-run")
			}
			text, err := executeClean(t, args...)
			require.ErrorIs(t, err, evidencecleanup.ErrPreflightBlocked)
			for _, member := range []string{old, blocked, report} {
				assert.Contains(t, text, member)
				assert.Contains(t, text, "retained ")
			}
			assert.Contains(t, text, "Blocker ")
			assert.Contains(t, text, "proof.json")
			assert.Contains(t, text, "entire group retained")
			after, err := os.ReadFile(filepath.Join(workspace, report))
			require.NoError(t, err)
			assert.Equal(t, before, after)
			assert.FileExists(t, filepath.Join(workspace, old, "proof.json"))
			if preview {
				assert.FileExists(t, filepath.Join(workspace, independent, "proof.json"))
				assert.Contains(t, text, "groups would-delete=1 retained=1")
			} else {
				assert.NoDirExists(t, filepath.Join(workspace, independent))
				assert.Contains(t, text, "groups deleted=1 retained=1 failed=0")
			}
		})
	}
}

func TestCleanCmd_AgeExclusionAndUnrecognizedLayout(t *testing.T) {
	workspace := t.TempDir()
	requireCleanSupport(t, workspace)
	unit := addCleanEvaluation(t, workspace, "newer", "20990101T000000Z")
	legacy := filepath.Join(workspace, complytime.WorkspaceDir, "ampel", "results", "legacy.json")
	require.NoError(t, os.WriteFile(legacy, []byte("excluded"), 0o600))
	text, err := executeClean(t, "--workspace", workspace, "--older-than", "30d")
	require.NoError(t, err)
	assert.Contains(t, text, "retained evaluation")
	assert.Contains(t, text, "collection start is not strictly before the cutoff")
	assert.Contains(t, text, "Excluded ")
	assert.Contains(t, text, "unrecognized provider results layout")
	assert.Contains(t, text, "members deleted=0 retained=1 partially-deleted=0; excluded=1")
	assert.FileExists(t, legacy)
	assert.FileExists(t, filepath.Join(workspace, unit, "proof.json"))
}

func TestCleanOptions_PartialFailureStillPrintsProgress(t *testing.T) {
	workspace := t.TempDir()
	requireCleanSupport(t, workspace)
	unit := addCleanEvaluation(t, workspace, "old", cleanTestStart)
	report := addCleanReport(t, workspace, unit)
	failure := errors.New("unexpected unlink failure")
	var out bytes.Buffer
	o := &cleanOptions{Common: &Common{Workspace: workspace}, olderThan: "30d",
		clock: cleanClock(func() time.Time { return time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC) }),
		runCleanup: func(opts evidencecleanup.Options) (*evidencecleanup.Result, error) {
			// Exercise real inventory/preflight, then simulate the service's partial
			// failure contract. Confined-unlink fault injection belongs to its tests.
			plan, err := evidencecleanup.Preview(evidencecleanup.PreviewOptions(opts))
			require.NoError(t, err)
			removed := filepath.Join(unit, "proof.json")
			require.NoError(t, os.Remove(filepath.Join(workspace, removed)))
			return &evidencecleanup.Result{Workspace: plan.Workspace, Age: plan.Age,
				Groups: []evidencecleanup.GroupExecution{{PartialLoss: true, Failure: failure.Error(),
					Members: []evidencecleanup.MemberResult{
						{Path: unit, Kind: evidencecleanup.MemberEvaluation,
							Status: evidencecleanup.ExecutionPartial, RemovedPaths: []string{removed},
							Failure: failure.Error()},
						{Path: report, Kind: evidencecleanup.MemberReport,
							Status: evidencecleanup.ExecutionRetained, Reasons: []string{"evidence failed"}},
					}}},
				Totals: evidencecleanup.ExecutionTotals{
					FailedGroups: 1, PartialMembers: 1, RetainedMembers: 1},
			}, failure
		}}
	err := o.run(&out)
	require.ErrorIs(t, err, failure)
	text := out.String()
	for _, expected := range []string{"2026-10-25T12:00:00Z", "Partial deletion", "no rollback",
		"partially-deleted evaluation", "retained report", "removed ", unit, report,
		failure.Error(), "groups deleted=0 retained=0 failed=1",
		"members deleted=0 retained=1 partially-deleted=1"} {
		assert.Contains(t, text, expected)
	}
	assert.FileExists(t, filepath.Join(workspace, report))
	assert.NoFileExists(t, filepath.Join(workspace, unit, "proof.json"))
	assert.DirExists(t, filepath.Join(workspace, unit))
}

type cleanFailWriter struct{ err error }

func TestCleanText_PreservesServiceOrderAndExactProgress(t *testing.T) {
	var text strings.Builder
	cleanBlockerText(&text, []evidencecleanup.GroupBlocker{
		{Path: "a/proof.json", Reason: "unreadable"},
		{Path: "b/proof.json", Reason: "missing"},
	})
	cleanMemberText(&text, evidencecleanup.MemberResult{
		Path: "evaluation", Kind: evidencecleanup.MemberEvaluation,
		Status: evidencecleanup.ExecutionPartial, Reasons: []string{"old", "preflight passed"},
		RemovedPaths: []string{"evaluation/a.json", "evaluation/b.json"}, Failure: "unlink failed",
	})
	cleanExclusionText(&text, []evidencecleanup.DiscoveryExclusion{
		{Path: "legacy.json", Reason: "unrecognized layout"},
	})
	assert.Equal(t, "  Blocker \"a/proof.json\": unreadable; entire group retained\n"+
		"  Blocker \"b/proof.json\": missing; entire group retained\n"+
		"  partially-deleted evaluation \"evaluation\": old; preflight passed\n"+
		"    removed \"evaluation/a.json\"\n"+
		"    removed \"evaluation/b.json\"\n"+
		"    Failure: unlink failed\n"+
		"Excluded \"legacy.json\": unrecognized layout\n", text.String())
}

func TestCleanCmd_UnboundedSARIFRefusesWithoutDeletion(t *testing.T) {
	for _, preview := range []bool{true, false} {
		t.Run(map[bool]string{true: "preview", false: "execute"}[preview], func(t *testing.T) {
			workspace := t.TempDir()
			requireCleanSupport(t, workspace)
			unit := addCleanEvaluation(t, workspace, "old", cleanTestStart)
			scan := filepath.Join(workspace, complytime.WorkspaceDir, "scan")
			require.NoError(t, os.MkdirAll(scan, 0o700))
			report := filepath.Join(scan, "scan-policy-target-20200101-000000.sarif.json")
			data := []byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"complyctl"}}}]}`)
			require.NoError(t, os.WriteFile(report, data, 0o600))
			args := []string{"--workspace", workspace, "--older-than", "30d"}
			if preview {
				args = append(args, "--dry-run")
			}
			text, err := executeClean(t, args...)
			require.ErrorContains(t, err, "SARIF has no originating EvaluationLog/evidence provenance")
			assert.Empty(t, text)
			after, err := os.ReadFile(report)
			require.NoError(t, err)
			assert.Equal(t, data, after)
			proof, err := os.ReadFile(filepath.Join(workspace, unit, "proof.json"))
			require.NoError(t, err)
			assert.Equal(t, "retained proof", string(proof))
		})
	}
}

func TestCleanCmd_EvidenceURIControlsAreEscaped(t *testing.T) {
	for _, preview := range []bool{true, false} {
		t.Run(map[bool]string{true: "preview", false: "execute"}[preview], func(t *testing.T) {
			workspace := t.TempDir()
			requireCleanSupport(t, workspace)
			unit := addCleanEvaluation(t, workspace, "old", cleanTestStart)
			report := addCleanReport(t, workspace, unit)
			root, err := os.OpenRoot(workspace)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, root.Close()) })
			data, err := root.ReadFile(report)
			require.NoError(t, err)
			coordinate := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, unit,
				"missing\x1b[2J\x1b[H\nINJECTED")}).String()
			data = bytes.ReplaceAll(data, []byte(filepath.Join(unit, "proof.json")), []byte(coordinate))
			require.NoError(t, root.WriteFile(report, data, 0o600))
			before := snapshotCleanWorkspace(t, workspace)
			args := []string{"--workspace", workspace, "--older-than", "30d"}
			if preview {
				args = append(args, "--dry-run")
			}
			text, err := executeClean(t, args...)
			require.ErrorIs(t, err, evidencecleanup.ErrPreflightBlocked)
			assert.NotContains(t, text, "\x1b")
			assert.NotContains(t, text, "\nINJECTED")
			assert.Contains(t, text, `\x1b`)
			assertCleanWorkspaceUnchanged(t, workspace, before)
		})
	}
}

func TestCleanText_DiagnosticControlsAreEscaped(t *testing.T) {
	unsafe := "failure\x1b[2J\nforged\r\t\u009b\u202e"
	var text strings.Builder
	cleanBlockerText(&text, []evidencecleanup.GroupBlocker{{Path: "proof", Reason: unsafe}})
	cleanMemberText(&text, evidencecleanup.MemberResult{
		Path: "proof", Reasons: []string{unsafe}, Failure: unsafe,
	})
	cleanExclusionText(&text, []evidencecleanup.DiscoveryExclusion{{Path: "other", Reason: unsafe}})
	result := &evidencecleanup.Result{Groups: []evidencecleanup.GroupExecution{{Failure: unsafe}}}
	preview := &evidencecleanup.PreviewResult{Groups: []evidencecleanup.PlannedGroup{{
		Members: []evidencecleanup.PlannedMember{{Reasons: []string{unsafe}}},
	}}}
	for _, rendered := range []string{text.String(), cleanExecutionText(result), cleanPreviewText(preview)} {
		assert.NotContains(t, rendered, "\x1b")
		assert.NotContains(t, rendered, "\nforged")
		assert.NotContains(t, rendered, "\r")
		assert.NotContains(t, rendered, "\t")
		assert.NotContains(t, rendered, "\u009b")
		assert.NotContains(t, rendered, "\u202e")
		assert.Contains(t, rendered, `\x1b`)
	}
	assert.Equal(t, `ordinary "quoted" café`, cleanDiagnosticText(`ordinary "quoted" café`))
}

func (w cleanFailWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteCleanText_OutputFailureAndEmptyResult(t *testing.T) {
	failure := errors.New("output unavailable")
	require.ErrorIs(t, writeCleanText(cleanFailWriter{failure}, "results"), failure)
	assert.NoError(t, writeCleanText(cleanFailWriter{failure}, ""))
	assert.Empty(t, cleanExecutionText(nil))
	assert.Empty(t, cleanPreviewText(nil))
	assert.NoError(t, writeCleanText(io.Discard, "results"))
}
