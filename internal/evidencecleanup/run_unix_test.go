// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/workspacecoord"
)

type executionFixture struct {
	workspace   string
	evaluations []string
	reports     []string
}

func newExecutionFixture(t *testing.T) executionFixture {
	t.Helper()
	f := executionFixture{workspace: t.TempDir()}
	requirePreviewSupport(t, f.workspace)
	var coordinates []string
	for _, label := range []string{"a", "b", "c"} {
		path := addGroupEvaluation(t, f.workspace, "ampel", label, groupOldStart)
		f.evaluations = append(f.evaluations, path)
		coordinates = append(coordinates, filepath.Join(path, "first.json"))
	}
	for _, name := range []string{"a", "b", "c"} {
		f.reports = append(f.reports, addGroupReport(t, f.workspace, name, coordinates...))
	}
	slices.Sort(f.reports)
	return f
}

func executionOptions(t *testing.T, workspace string) Options {
	t.Helper()
	preview := previewOptions(t, workspace)
	return Options{Workspace: workspace, OlderThan: preview.OlderThan, Clock: preview.Clock}
}

func runPrepared(t *testing.T, workspace string, prepare removalPreparer) (*Result, error) {
	t.Helper()
	return run(executionOptions(t, workspace),
		func(access *workspacecoord.Access, groups *GroupResult, result *Result) error {
			// The public runner acquired exclusive access before inventory and keeps
			// it through this boundary and every real descriptor-relative unlink.
			other, err := workspacecoord.Acquire(workspacecoord.Options{
				Workspace: workspace, Mode: workspacecoord.Shared,
			})
			require.ErrorIs(t, err, workspacecoord.ErrBusy)
			require.Nil(t, other)
			return executeGroupsWith(access, groups, result, prepare)
		})
}

func findExecutionMember(t *testing.T, result *Result, path string) MemberResult {
	t.Helper()
	for _, group := range result.Groups {
		for _, member := range group.Members {
			if member.Path == path {
				return member
			}
		}
	}
	require.FailNow(t, "missing execution member", "%s", path)
	return MemberResult{}
}

func TestRun_EvidenceFirstAndRepeat(t *testing.T) {
	f := newExecutionFixture(t)
	var prepared, removed []string
	result, err := runPrepared(t, f.workspace,
		func(workspace *os.File, path string, identity os.FileInfo) (*removal, error) {
			r, err := prepareRemoval(workspace, path, identity)
			if err != nil {
				return nil, err
			}
			prepared = append(prepared, path)
			r.unlink = func(parent *os.File, name string, directory bool) error {
				assert.Len(t, prepared, 6, "every report and evaluation prepared before any unlink")
				if filepath.Dir(path) == reportDirectory {
					for _, evaluation := range f.evaluations {
						assert.NoDirExists(t, filepath.Join(f.workspace, evaluation))
					}
				}
				if err := unlinkRemovalEntry(parent, name, directory); err != nil {
					return err
				}
				if name == filepath.Base(path) {
					removed = append(removed, path)
				}
				return nil
			}
			return r, nil
		})
	require.NoError(t, err)
	wantOrder := append(slices.Clone(f.evaluations), f.reports...)
	assert.Equal(t, wantOrder, prepared)
	assert.Equal(t, wantOrder, removed)
	assert.Equal(t, ExecutionTotals{DeletedGroups: 1, DeletedMembers: 6}, result.Totals)
	require.Len(t, result.Groups, 1)
	assert.True(t, result.Groups[0].Deleted)
	assert.False(t, result.Groups[0].PartialLoss)
	for _, path := range wantOrder {
		member := findExecutionMember(t, result, path)
		assert.Equal(t, ExecutionDeleted, member.Status)
		assert.Equal(t, path, member.RemovedPaths[len(member.RemovedPaths)-1])
		assert.Empty(t, member.Failure)
		assert.NoFileExists(t, filepath.Join(f.workspace, path))
	}
	repeated, err := Run(executionOptions(t, f.workspace))
	require.NoError(t, err)
	assert.Empty(t, repeated.Groups)
	assert.Zero(t, repeated.Totals.DeletedMembers)
	assert.Zero(t, repeated.Totals.FailedGroups)
}

func TestRun_EvaluationDirectoryCoordinateRetainsConnectedGroup(t *testing.T) {
	for _, kind := range []string{"relative", "relative-trailing-slash", "absolute", "file-uri"} {
		t.Run(kind, func(t *testing.T) {
			workspace := t.TempDir()
			requirePreviewSupport(t, workspace)
			unit := addGroupEvaluation(t, workspace, "opa", "old", groupOldStart)
			coordinate := unit
			switch kind {
			case "relative-trailing-slash":
				coordinate += "/"
			case "absolute":
				coordinate = filepath.Join(workspace, unit)
			case "file-uri":
				coordinate = (&url.URL{Scheme: "file", Path: filepath.Join(workspace, unit)}).String()
			}
			addGroupReport(t, workspace, "directory", coordinate)
			addGroupReport(t, workspace, "file", filepath.Join(unit, "first.json"))
			before := groupSnapshot(t, workspace)

			preview, err := Preview(previewOptions(t, workspace))
			require.Error(t, err)
			assert.Len(t, preview.Groups, 1, "directory and file references share the evaluation")
			assert.NotEmpty(t, preview.Groups[0].Blockers)
			assert.Equal(t, before, groupSnapshot(t, workspace))

			result, err := Run(executionOptions(t, workspace))
			require.Error(t, err)
			assert.Zero(t, result.Totals.DeletedMembers)
			assert.Equal(t, before, groupSnapshot(t, workspace), "retain all connected evidence and reports")
		})
	}
}

func TestRun_LiteralBackslashNamesDoNotCauseLateRemovalFailure(t *testing.T) {
	for _, kind := range []string{"evidence-file", "evidence-directory", "report"} {
		t.Run(kind, func(t *testing.T) {
			workspace := t.TempDir()
			requirePreviewSupport(t, workspace)
			unit := addGroupEvaluation(t, workspace, "opa", "old", groupOldStart)
			report := addGroupReport(t, workspace, "old", filepath.Join(unit, "first.json"))
			switch kind {
			case "evidence-file":
				require.NoError(t, os.WriteFile(filepath.Join(workspace, unit, `z\output.json`), []byte("proof"), 0o600))
			case "evidence-directory":
				dir := filepath.Join(workspace, unit, `z\output`)
				require.NoError(t, os.Mkdir(dir, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "proof.json"), []byte("proof"), 0o600))
			case "report":
				renamed := filepath.Join(reportDirectory, `evaluation-log-z\output.json`)
				require.NoError(t, os.Rename(filepath.Join(workspace, report), filepath.Join(workspace, renamed)))
				report = renamed
			}
			preview, err := Preview(previewOptions(t, workspace))
			require.NoError(t, err)
			require.Len(t, preview.Groups, 1)
			assert.Empty(t, preview.Groups[0].Blockers)
			result, err := Run(executionOptions(t, workspace))
			assert.NoError(t, err)
			assert.Equal(t, ExecutionTotals{DeletedGroups: 1, DeletedMembers: 2}, result.Totals)
			assert.NoDirExists(t, filepath.Join(workspace, unit))
			assert.NoFileExists(t, filepath.Join(workspace, report))
		})
	}
}

func TestRun_EvidenceFailureReportsExactPartialTreeAndContinuesIndependentGroup(t *testing.T) {
	f := newExecutionFixture(t)
	independent := addGroupEvaluation(t, f.workspace, "opa", "independent", groupOldStart)
	before := groupSnapshot(t, f.workspace)
	failure := errors.New("injected evidence I/O failure")
	result, err := runPrepared(t, f.workspace,
		func(workspace *os.File, path string, identity os.FileInfo) (*removal, error) {
			r, err := prepareRemoval(workspace, path, identity)
			if err == nil && path == f.evaluations[1] {
				r.unlink = func(parent *os.File, name string, directory bool) error {
					if name == "second.json" {
						return failure
					}
					return unlinkRemovalEntry(parent, name, directory)
				}
			}
			return r, err
		})
	require.ErrorIs(t, err, failure)
	assert.ErrorContains(t, err, "no rollback")
	assert.Equal(t, ExecutionTotals{DeletedGroups: 1, FailedGroups: 1,
		DeletedMembers: 2, PartialMembers: 1, RetainedMembers: 4}, result.Totals)
	assert.True(t, result.Groups[0].PartialLoss)
	assert.False(t, result.Groups[0].Deleted)
	assert.Equal(t, ExecutionDeleted, findExecutionMember(t, result, f.evaluations[0]).Status)
	partial := findExecutionMember(t, result, f.evaluations[1])
	assert.Equal(t, ExecutionPartial, partial.Status)
	assert.Equal(t, []string{filepath.Join(f.evaluations[1], "first.json")}, partial.RemovedPaths)
	assert.Contains(t, partial.Failure, "second.json")
	assert.Equal(t, ExecutionRetained, findExecutionMember(t, result, f.evaluations[2]).Status)
	assert.Equal(t, ExecutionDeleted, findExecutionMember(t, result, independent).Status)
	after := groupSnapshot(t, f.workspace)
	for _, path := range f.reports {
		assert.Equal(t, before[path], after[path], "reports preserved after evidence failure")
		member := findExecutionMember(t, result, path)
		assert.Equal(t, ExecutionRetained, member.Status)
		assert.Empty(t, member.RemovedPaths)
		assert.Empty(t, member.Failure, "unattempted report is not a failed unlink")
	}
	assert.Equal(t, before[filepath.Join(f.evaluations[2], "first.json")],
		after[filepath.Join(f.evaluations[2], "first.json")])
	// A later invocation sees the missing required evidence and must NOT erase
	// surviving reports of the partially deleted group.
	retry, err := Run(executionOptions(t, f.workspace))
	require.ErrorContains(t, err, "possible relationships are unbounded")
	if retry != nil {
		assert.Zero(t, retry.Totals.DeletedMembers)
	}
	assert.Equal(t, after, groupSnapshot(t, f.workspace))
}

func TestRun_ReportFailureStopsRemainingReports(t *testing.T) {
	f := newExecutionFixture(t)
	failure := errors.New("injected report I/O failure")
	result, err := runPrepared(t, f.workspace,
		func(workspace *os.File, path string, identity os.FileInfo) (*removal, error) {
			r, err := prepareRemoval(workspace, path, identity)
			if err == nil && path == f.reports[1] {
				r.unlink = func(*os.File, string, bool) error { return failure }
			}
			return r, err
		})
	require.ErrorIs(t, err, failure)
	assert.Equal(t, ExecutionTotals{FailedGroups: 1, DeletedMembers: 4, RetainedMembers: 2},
		result.Totals)
	assert.True(t, result.Groups[0].PartialLoss)
	for _, path := range append(slices.Clone(f.evaluations), f.reports[0]) {
		assert.Equal(t, ExecutionDeleted, findExecutionMember(t, result, path).Status)
	}
	for _, path := range f.reports[1:] {
		assert.FileExists(t, filepath.Join(f.workspace, path))
		member := findExecutionMember(t, result, path)
		assert.Equal(t, ExecutionRetained, member.Status)
		assert.Empty(t, member.RemovedPaths)
	}
	assert.Contains(t, findExecutionMember(t, result, f.reports[1]).Failure, failure.Error())
	assert.Empty(t, findExecutionMember(t, result, f.reports[2]).Failure)
}

func TestRun_EvidenceFailureDistinguishesRetainedFromPartialMember(t *testing.T) {
	for _, stopAt := range []string{"first.json", "root"} {
		t.Run(stopAt, func(t *testing.T) {
			f := newExecutionFixture(t)
			before := groupSnapshot(t, f.workspace)
			failure := errors.New("injected evidence unlink failure")
			result, err := runPrepared(t, f.workspace,
				func(workspace *os.File, path string, identity os.FileInfo) (*removal, error) {
					r, err := prepareRemoval(workspace, path, identity)
					if err == nil && path == f.evaluations[0] {
						r.unlink = func(parent *os.File, name string, directory bool) error {
							if name == stopAt || (stopAt == "root" && name == filepath.Base(path)) {
								return failure
							}
							return unlinkRemovalEntry(parent, name, directory)
						}
					}
					return r, err
				})
			require.ErrorIs(t, err, failure)
			require.Len(t, result.Groups, 1)
			group := result.Groups[0]
			assert.False(t, group.Deleted)
			assert.ErrorContains(t, err, "no rollback")
			assert.DirExists(t, filepath.Join(f.workspace, f.evaluations[0]))
			member := findExecutionMember(t, result, f.evaluations[0])
			assert.Contains(t, member.Failure, failure.Error())
			if stopAt == "first.json" {
				assert.Equal(t, ExecutionRetained, member.Status)
				assert.Empty(t, member.RemovedPaths)
				assert.False(t, group.PartialLoss)
				assert.Equal(t, ExecutionTotals{FailedGroups: 1, RetainedMembers: 6}, result.Totals)
				assert.Equal(t, before, groupSnapshot(t, f.workspace))
			} else {
				assert.Equal(t, ExecutionPartial, member.Status)
				assert.Equal(t, []string{filepath.Join(member.Path, "first.json"),
					filepath.Join(member.Path, "nested", "second.json"),
					filepath.Join(member.Path, "nested")}, member.RemovedPaths)
				assert.True(t, group.PartialLoss)
				assert.Equal(t, ExecutionTotals{FailedGroups: 1, PartialMembers: 1,
					RetainedMembers: 5}, result.Totals)
			}
			after := groupSnapshot(t, f.workspace)
			for _, path := range f.reports {
				assert.Equal(t, before[path], after[path], "report untouched after evidence error")
			}
			for _, path := range f.evaluations[1:] {
				assert.Equal(t, ExecutionRetained, findExecutionMember(t, result, path).Status)
				assert.Equal(t, before[filepath.Join(path, "first.json")],
					after[filepath.Join(path, "first.json")])
			}
		})
	}
}

func TestRun_PrepareLastReportFailureRetainsWholeGroup(t *testing.T) {
	f := newExecutionFixture(t)
	independent := addGroupEvaluation(t, f.workspace, "opa", "independent", groupOldStart)
	before := groupSnapshot(t, f.workspace)
	failure := errors.New("injected descriptor exhaustion")
	result, err := runPrepared(t, f.workspace,
		func(workspace *os.File, path string, identity os.FileInfo) (*removal, error) {
			if path == f.reports[2] {
				return nil, failure
			}
			return prepareRemoval(workspace, path, identity)
		})
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, err, ErrPreflightBlocked)
	for _, path := range findExecutionMember(t, result, independent).RemovedPaths {
		delete(before, path)
	}
	assert.Equal(t, before, groupSnapshot(t, f.workspace))
	assert.Equal(t, ExecutionDeleted, findExecutionMember(t, result, independent).Status)
	assert.NoDirExists(t, filepath.Join(f.workspace, independent))
	assert.Equal(t, ExecutionTotals{DeletedGroups: 1, DeletedMembers: 1,
		FailedGroups: 1, RetainedMembers: 6}, result.Totals)
	assert.False(t, result.Groups[0].PartialLoss)
	assert.Contains(t, findExecutionMember(t, result, f.reports[2]).Failure, failure.Error())
	for _, member := range result.Groups[0].Members {
		assert.Equal(t, ExecutionRetained, member.Status)
		assert.Empty(t, member.RemovedPaths)
	}
}

func TestRun_WholeGroupPreflightRetention(t *testing.T) {
	for _, condition := range []string{"newer", "unsafe", "unbounded-report"} {
		t.Run(condition, func(t *testing.T) {
			f := newExecutionFixture(t)
			switch condition {
			case "newer":
				newer := addGroupEvaluation(t, f.workspace, "ampel", "new", "20261118T120000Z")
				addGroupReport(t, f.workspace, "new", filepath.Join(newer, "first.json"),
					filepath.Join(f.evaluations[0], "first.json"))
			case "unsafe":
				require.NoError(t, os.Symlink("first.json",
					filepath.Join(f.workspace, f.evaluations[2], "link")))
			case "unbounded-report":
				require.NoError(t, os.WriteFile(filepath.Join(f.workspace, reportDirectory,
					"evaluation-log-malformed.json"), []byte("{"), 0o600))
			}
			before := previewSnapshot(t, f.workspace)
			result, err := Run(executionOptions(t, f.workspace))
			if condition == "newer" {
				require.NoError(t, err)
				assert.Equal(t, 8, result.Totals.RetainedMembers)
			} else {
				require.Error(t, err)
			}
			assert.Equal(t, before, previewSnapshot(t, f.workspace))
		})
	}
}

func TestRun_BlockedGroupDoesNotBlockIndependentSafeGroup(t *testing.T) {
	f := newExecutionFixture(t)
	require.NoError(t, os.Symlink("first.json", filepath.Join(f.workspace, f.evaluations[0], "link")))
	independent := addGroupEvaluation(t, f.workspace, "opa", "independent", groupOldStart)
	result, err := Run(executionOptions(t, f.workspace))
	require.ErrorIs(t, err, ErrPreflightBlocked)
	assert.Equal(t, ExecutionTotals{DeletedGroups: 1, RetainedGroups: 1,
		DeletedMembers: 1, RetainedMembers: 6}, result.Totals)
	assert.Equal(t, ExecutionDeleted, findExecutionMember(t, result, independent).Status)
	for _, path := range f.reports {
		assert.FileExists(t, filepath.Join(f.workspace, path))
	}
}

func TestRun_ValidationBusyAndActivityAreNonmutating(t *testing.T) {
	for _, condition := range []string{"invalid-age", "busy", "activity", "empty"} {
		t.Run(condition, func(t *testing.T) {
			workspace := t.TempDir()
			requirePreviewSupport(t, workspace)
			if condition != "empty" {
				evaluation := addGroupEvaluation(t, workspace, "ampel", "old", groupOldStart)
				addGroupReport(t, workspace, "old", filepath.Join(evaluation, "first.json"))
			}
			opts := executionOptions(t, workspace)
			switch condition {
			case "invalid-age":
				opts.OlderThan = "0d"
			case "busy":
				access, err := workspacecoord.Acquire(workspacecoord.Options{
					Workspace: workspace, Mode: workspacecoord.Shared,
				})
				require.NoError(t, err)
				defer func() { require.NoError(t, access.Close()) }()
			case "activity":
				dir := filepath.Join(workspace, ".complytime", "coordination", "operations")
				require.NoError(t, os.MkdirAll(dir, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "unresolved.json"), []byte("{}"), 0o600))
			}
			before := previewSnapshot(t, workspace)
			result, err := Run(opts)
			if condition == "empty" {
				require.NoError(t, err)
				assert.Equal(t, ExecutionTotals{}, result.Totals)
			} else {
				require.Error(t, err)
				assert.Nil(t, result)
			}
			assert.Equal(t, before, previewSnapshot(t, workspace))
		})
	}
}
