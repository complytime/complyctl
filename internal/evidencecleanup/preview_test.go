// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/workspacecoord"
)

const previewHelperEnvironment = "COMPLYCTL_PREVIEW_TEST_LOCK"

type previewClock struct {
	now   time.Time
	calls int
}

func (c *previewClock) Now() time.Time {
	c.calls++
	return c.now
}

func previewOptions(t *testing.T, workspace string) PreviewOptions {
	t.Helper()
	return PreviewOptions{Workspace: workspace, OlderThan: "30d",
		Clock: &previewClock{now: groupAge(t).CapturedAt()}}
}

func requirePreviewSupport(t *testing.T, workspace string) {
	t.Helper()
	access, err := workspacecoord.Acquire(workspacecoord.Options{
		Workspace: workspace, Mode: workspacecoord.Exclusive,
	})
	require.NoError(t, err)
	err = access.CheckCleanupSupport()
	require.NoError(t, access.Close())
	if errors.Is(err, workspacecoord.ErrUnsupportedFilesystem) {
		t.Skipf("preview runtime needs a supported local filesystem: %v", err)
	}
	require.NoError(t, err)
}

func plannedPaths(group PlannedGroup) []string {
	paths := make([]string, len(group.Members))
	for i, member := range group.Members {
		paths[i] = member.Path
	}
	return paths
}

type previewEntry struct {
	retainedEntry
	Modified time.Time
	Size     int64
	Device   string
	Inode    uint64
	Links    uint64
	UID      uint32
	GID      uint32
}

// Include directory and inode identity so replacing identical bytes, touching
// metadata, or creating then removing coordination files cannot pass unnoticed.
// Access times are excluded because reading the inventory may update them.
func previewSnapshot(t *testing.T, workspace string) map[string]previewEntry {
	t.Helper()
	entries := make(map[string]previewEntry)
	for path, content := range groupSnapshot(t, workspace) {
		info, err := os.Lstat(filepath.Join(workspace, path))
		require.NoError(t, err)
		stat, ok := info.Sys().(*syscall.Stat_t)
		require.True(t, ok, "native Unix file identity is required")
		entries[path] = previewEntry{retainedEntry: content,
			Modified: info.ModTime(), Size: info.Size(), Device: fmt.Sprint(stat.Dev),
			Inode: stat.Ino, Links: uint64(stat.Nlink), UID: stat.Uid, GID: stat.Gid}
	}
	return entries
}

func TestPreview_CompleteStableGroupsWithoutConfigurationOrProviders(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	requirePreviewSupport(t, fixture.workspace)
	variants := writeFixtureVariants(t, fixture)
	orphan := addGroupEvaluation(t, fixture.workspace, "ampel", "orphan", groupOldStart)
	newer := addGroupEvaluation(t, fixture.workspace, "openscap", "newer", "20261019T120000Z")
	newReport := addGroupReport(t, fixture.workspace, "new", filepath.Join(newer, "first.json"))
	// Include excluded content from both discovery passes, deliberately created
	// in reverse lexical order. Nothing reads or repairs workspace configuration.
	for _, path := range []string{"z-note", "a-note", ".complytime/cli.log",
		".complytime/opa/working/input", ".complytime/scan/z-note", ".complytime/scan/a-note"} {
		absolute := filepath.Join(fixture.workspace, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(absolute), 0o700))
		require.NoError(t, os.WriteFile(absolute, []byte(path), 0o600))
	}
	isolated := t.TempDir()
	t.Setenv("PATH", "")
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", isolated)
	t.Setenv("XDG_DATA_HOME", isolated)
	t.Setenv("XDG_CACHE_HOME", isolated)
	before := previewSnapshot(t, fixture.workspace)
	environmentBefore := previewSnapshot(t, isolated)
	opts := previewOptions(t, fixture.workspace)
	result, err := Preview(opts)
	require.NoError(t, err)
	canonical, err := filepath.EvalSymlinks(fixture.workspace)
	require.NoError(t, err)
	assert.Equal(t, canonical, result.Workspace)
	assert.Equal(t, groupAge(t), result.Age)
	assert.Equal(t, 1, opts.Clock.(*previewClock).calls)
	require.Len(t, result.Groups, 3)
	assert.Equal(t, []string{orphan}, plannedPaths(result.Groups[0]))
	expected := []string{fixture.unit}
	for _, path := range variants {
		relative, err := filepath.Rel(fixture.workspace, path)
		require.NoError(t, err)
		expected = append(expected, relative)
	}
	slices.Sort(expected)
	assert.Equal(t, expected, plannedPaths(result.Groups[1]))
	assert.Equal(t, []string{newer, newReport}, plannedPaths(result.Groups[2]))
	for _, group := range result.Groups[:2] {
		assert.Empty(t, group.Blockers)
		for _, member := range group.Members {
			assert.Equal(t, StatusWouldDelete, member.Status)
			assert.Equal(t, []string{
				"whole-group preflight passed; reports and evidence would expire together",
			}, member.Reasons)
		}
	}
	assert.Equal(t, MemberEvaluation, result.Groups[1].Members[0].Kind)
	assert.Equal(t, MemberReport, result.Groups[1].Members[1].Kind)
	assert.Equal(t, []GroupBlocker{{Path: newer, AgeExclusion: true,
		Reason: "collection start is not strictly before the cutoff"}}, result.Groups[2].Blockers)
	assert.Equal(t, PreviewTotals{WouldDeleteGroups: 2, RetainedGroups: 1,
		WouldDeleteMembers: 6, RetainedMembers: 2, ExcludedEntries: 2}, result.Totals)
	assert.Equal(t, []DiscoveryExclusion{
		{Path: ".complytime/scan/a-note", Reason: "unrelated report-directory entry"},
		{Path: ".complytime/scan/z-note", Reason: "unrelated report-directory entry"},
	}, result.Exclusions)
	again, err := Preview(previewOptions(t, fixture.workspace))
	require.NoError(t, err)
	assert.Equal(t, result, again, "repeated previews must have identical typed results")
	assert.Equal(t, before, previewSnapshot(t, fixture.workspace))
	assert.Equal(t, environmentBefore, previewSnapshot(t, isolated))
}

func TestPreview_AllMembersAndBlockersRetainedWithIndependentSafeGroup(t *testing.T) {
	workspace := t.TempDir()
	requirePreviewSupport(t, workspace)
	old := addGroupEvaluation(t, workspace, "ampel", "old", groupOldStart)
	shared := addGroupEvaluation(t, workspace, "opa", "shared", "20261019T120000Z")
	last := addGroupEvaluation(t, workspace, "openscap", "last", groupOldStart)
	firstReport := addGroupReport(t, workspace, "a", filepath.Join(old, "first.json"),
		filepath.Join(shared, "first.json"))
	secondReport := addGroupReport(t, workspace, "b", filepath.Join(shared, "nested/second.json"),
		filepath.Join(last, "first.json"))
	safe := addGroupEvaluation(t, workspace, "opa", "independent", groupOldStart)
	// Multiple independently detectable blockers, not a short-circuited group.
	require.NoError(t, os.Remove(filepath.Join(workspace, last, "first.json")))
	require.NoError(t, os.Chmod(filepath.Join(workspace, shared, "nested"), 0o500))
	t.Cleanup(func() {
		require.NoError(t, os.Chmod(filepath.Join(workspace, shared, "nested"), 0o700))
	})
	before := previewSnapshot(t, workspace)
	result, err := Preview(previewOptions(t, workspace))
	require.ErrorIs(t, err, ErrPreflightBlocked)
	require.NotNil(t, result)
	require.Len(t, result.Groups, 2)
	blocked := result.Groups[0]
	expected := []string{old, shared, last, firstReport, secondReport}
	slices.Sort(expected)
	assert.Equal(t, expected, plannedPaths(blocked))
	require.Len(t, blocked.Blockers, 3)
	assert.Equal(t, shared, blocked.Blockers[0].Path)
	assert.True(t, blocked.Blockers[0].AgeExclusion)
	assert.Equal(t, shared, blocked.Blockers[1].Path)
	assert.False(t, blocked.Blockers[1].AgeExclusion)
	assert.Contains(t, blocked.Blockers[1].Reason, "nested")
	assert.Equal(t, secondReport, blocked.Blockers[2].Path)
	assert.Contains(t, blocked.Blockers[2].Reason, "first.json")
	for _, member := range blocked.Members {
		assert.Equal(t, StatusRetained, member.Status)
		assert.NotEmpty(t, member.Reasons)
	}
	assert.Equal(t, []string{blocked.Blockers[0].Reason, blocked.Blockers[1].Reason},
		blocked.Members[1].Reasons)
	assert.Equal(t, []string{"entire group retained; see blocking members"},
		blocked.Members[0].Reasons)
	assert.Equal(t, []string{safe}, plannedPaths(result.Groups[1]))
	assert.Equal(t, StatusWouldDelete, result.Groups[1].Members[0].Status)
	assert.Equal(t, PreviewTotals{WouldDeleteGroups: 1, RetainedGroups: 1,
		WouldDeleteMembers: 1, RetainedMembers: 5}, result.Totals)
	assert.Contains(t, err.Error(), secondReport)
	assert.Contains(t, err.Error(), "entire group retained")
	again, againErr := Preview(previewOptions(t, workspace))
	require.ErrorIs(t, againErr, ErrPreflightBlocked)
	assert.Equal(t, result, again)
	assert.Equal(t, err.Error(), againErr.Error())
	assert.Equal(t, before, previewSnapshot(t, workspace))
}

func TestPreview_UnboundedUncertaintyHasNoPartialPlan(t *testing.T) {
	for _, variant := range []string{"sarif", "malformed", "unknown"} {
		t.Run(variant, func(t *testing.T) {
			fixture := newReportFixture(t, "opa")
			requirePreviewSupport(t, fixture.workspace)
			addUnresolvedFixture(t, fixture, variant)
			before := groupSnapshot(t, fixture.workspace)
			result, err := Preview(previewOptions(t, fixture.workspace))
			require.ErrorContains(t, err, "report")
			assert.Nil(t, result, "unbounded report uncertainty must not yield standalone candidates")
			assert.Equal(t, before, groupSnapshot(t, fixture.workspace))
		})
	}
}

func TestPreview_NoMatchesAndMalformedConfigDoNotInitializeWorkspace(t *testing.T) {
	for _, config := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "malformed-config"}[config], func(t *testing.T) {
			workspace := t.TempDir()
			requirePreviewSupport(t, workspace)
			if config {
				path := filepath.Join(workspace, managedDirectory, "complytime.yaml")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
				require.NoError(t, os.WriteFile(path, []byte("invalid: ["), 0o600))
			}
			before := groupSnapshot(t, workspace)
			result, err := Preview(previewOptions(t, workspace))
			require.NoError(t, err)
			assert.Empty(t, result.Groups)
			assert.Empty(t, result.Exclusions)
			assert.Equal(t, PreviewTotals{}, result.Totals)
			assert.Equal(t, before, groupSnapshot(t, workspace))
		})
	}
}

func TestPreview_InvalidOptionsDoNotDiscoverOrWrite(t *testing.T) {
	workspace := t.TempDir()
	for _, failure := range []string{"threshold", "clock", "missing-workspace", "empty-workspace"} {
		t.Run(failure, func(t *testing.T) {
			opts := previewOptions(t, workspace)
			switch failure {
			case "threshold":
				opts.OlderThan = "0d"
			case "clock":
				opts.Clock = nil
			case "missing-workspace":
				opts.Workspace = filepath.Join(workspace, "missing")
			case "empty-workspace":
				opts.Workspace = ""
			}
			before := groupSnapshot(t, workspace)
			result, err := Preview(opts)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Equal(t, before, groupSnapshot(t, workspace))
			if failure == "threshold" {
				assert.Zero(t, opts.Clock.(*previewClock).calls)
			}
		})
	}
}

type previewLockResponse struct {
	Busy  bool
	Error string
}

// The JSON response is an acquisition barrier; stdin EOF releases a held lock.
func TestPreview_LockProcess(t *testing.T) {
	request := os.Getenv(previewHelperEnvironment)
	if request == "" {
		return
	}
	var opts workspacecoord.Options
	require.NoError(t, json.Unmarshal([]byte(request), &opts))
	access, err := workspacecoord.Acquire(opts)
	response := previewLockResponse{Busy: errors.Is(err, workspacecoord.ErrBusy)}
	if err != nil {
		response.Error = err.Error()
	}
	require.NoError(t, json.NewEncoder(os.Stdout).Encode(response))
	if access != nil {
		_, err = io.Copy(io.Discard, os.Stdin)
		require.NoError(t, err)
		require.NoError(t, access.Close())
	}
}

func previewLockProcess(t *testing.T, workspace string,
	mode workspacecoord.Mode,
) (previewLockResponse, func()) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	request, err := json.Marshal(workspacecoord.Options{Workspace: workspace, Mode: mode})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestPreview_LockProcess$")
	cmd.Env = append(os.Environ(), previewHelperEnvironment+"="+string(request))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	input, err := cmd.StdinPipe()
	require.NoError(t, err)
	output, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	var once sync.Once
	stop := func() {
		once.Do(func() {
			assert.NoError(t, input.Close())
			assert.NoError(t, cmd.Wait(), "child stderr: %s", stderr.String())
			assert.NoError(t, ctx.Err(), "lock acquisition must not wait for contention to end")
		})
	}
	t.Cleanup(stop)
	var response previewLockResponse
	require.NoError(t, json.NewDecoder(output).Decode(&response))
	return response, stop
}

func TestPreview_BusyAliasesAndIndependentWorkspaces(t *testing.T) {
	for _, mode := range []workspacecoord.Mode{workspacecoord.Shared, workspacecoord.Exclusive} {
		t.Run(map[workspacecoord.Mode]string{
			workspacecoord.Shared: "shared", workspacecoord.Exclusive: "exclusive",
		}[mode], func(t *testing.T) {
			workspace := t.TempDir()
			requirePreviewSupport(t, workspace)
			addGroupEvaluation(t, workspace, "opa", "old", groupOldStart)
			alias := filepath.Join(t.TempDir(), "alias")
			require.NoError(t, os.Symlink(workspace, alias))
			other := t.TempDir()
			otherUnit := addGroupEvaluation(t, other, "ampel", "other", groupOldStart)
			before, otherBefore := groupSnapshot(t, workspace), groupSnapshot(t, other)
			response, stop := previewLockProcess(t, alias, mode)
			require.Empty(t, response.Error)
			for _, path := range []string{workspace, alias} {
				result, err := Preview(previewOptions(t, path))
				require.ErrorIs(t, err, workspacecoord.ErrBusy)
				assert.Nil(t, result)
			}
			result, err := Preview(previewOptions(t, other))
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			assert.Equal(t, []string{otherUnit}, plannedPaths(result.Groups[0]))
			stop()
			result, err = Preview(previewOptions(t, alias))
			require.NoError(t, err)
			assert.Equal(t, 1, result.Totals.WouldDeleteMembers)
			assert.Equal(t, before, groupSnapshot(t, workspace))
			assert.Equal(t, otherBefore, groupSnapshot(t, other))
		})
	}
}

func TestPreview_AccessHeldThroughInventoryAndPreflight(t *testing.T) {
	workspace := t.TempDir()
	requirePreviewSupport(t, workspace)
	unit := addGroupEvaluation(t, workspace, "opa", "old", groupOldStart)
	alias := filepath.Join(t.TempDir(), "alias")
	require.NoError(t, os.Symlink(workspace, alias))
	before := groupSnapshot(t, workspace)
	result, err := preview(previewOptions(t, alias), func(opts GroupOptions) (*GroupResult, error) {
		for _, mode := range []workspacecoord.Mode{workspacecoord.Shared, workspacecoord.Exclusive} {
			response, stop := previewLockProcess(t, workspace, mode)
			assert.True(t, response.Busy, response.Error)
			stop()
		}
		groups, err := PreflightGroups(opts)
		require.NoError(t, err)
		response, stop := previewLockProcess(t, alias, workspacecoord.Shared)
		assert.True(t, response.Busy, "access must survive complete inventory and preflight")
		stop()
		return groups, nil
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, []string{unit}, plannedPaths(result.Groups[0]))
	response, stop := previewLockProcess(t, workspace, workspacecoord.Exclusive)
	assert.Empty(t, response.Error, "returned preview cannot reserve later execution")
	stop()
	assert.Equal(t, before, groupSnapshot(t, workspace))
}

func TestPreview_UnresolvedActivityPreservesEveryRecord(t *testing.T) {
	workspace := t.TempDir()
	requirePreviewSupport(t, workspace)
	addGroupEvaluation(t, workspace, "opa", "old", groupOldStart)
	access, err := workspacecoord.Acquire(workspacecoord.Options{
		Workspace: workspace, Mode: workspacecoord.Shared,
	})
	require.NoError(t, err)
	activity, err := access.BeginActivity(workspacecoord.ActivityOptions{
		Operation: workspacecoord.OperationScan, StartedAt: groupAge(t).CapturedAt(),
	})
	require.NoError(t, err)
	require.NoError(t, access.Close()) // Uncertain shutdown: deliberately leave the owned record.
	malformed := filepath.Join(filepath.Dir(activity.Path()), "malformed.json")
	require.NoError(t, os.WriteFile(malformed, []byte("not JSON"), 0o600))
	for _, record := range []string{activity.Path(), malformed} {
		before := groupSnapshot(t, workspace)
		result, err := Preview(previewOptions(t, workspace))
		require.ErrorIs(t, err, workspacecoord.ErrUnresolvedActivity)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), record)
		assert.Contains(t, err.Error(), "Stop all writers")
		assert.Contains(t, err.Error(), "no evidence was deleted")
		assert.Equal(t, before, groupSnapshot(t, workspace))
		// Controlled host has exited and no provider was started. Simulate only
		// the operator's exact-record recovery, never an automatic expiry rule.
		require.NoError(t, os.Remove(record))
	}
	result, err := Preview(previewOptions(t, workspace))
	require.NoError(t, err)
	assert.Equal(t, 1, result.Totals.WouldDeleteMembers)
}

func TestPlanPreview_OrderDoesNotDependOnInventoryOrBlockerInsertion(t *testing.T) {
	age := groupAge(t)
	input := &GroupResult{
		Groups: []Group{
			{Evaluations: []Evaluation{{Path: "z"}, {Path: "y"}}},
			{Evaluations: []Evaluation{{Path: "b"}, {Path: "a"}},
				Reports: []ManagedReport{{Path: "d"}, {Path: "c"}},
				Blockers: []GroupBlocker{
					{Path: "c", Reason: "z"}, {Path: "b", Reason: "b"},
					{Path: "b", Reason: "a", AgeExclusion: true}, {Path: "b", Reason: "a"},
				}},
		},
		Exclusions: []DiscoveryExclusion{
			{Path: "z", Reason: "excluded"}, {Path: "a", Reason: "z"}, {Path: "a", Reason: "a"},
		},
	}
	result := planPreview("workspace", age, input)
	assert.Equal(t, []string{"a", "b", "c", "d"}, plannedPaths(result.Groups[0]))
	assert.Equal(t, []string{"y", "z"}, plannedPaths(result.Groups[1]))
	assert.Equal(t, []GroupBlocker{
		{Path: "b", Reason: "a"}, {Path: "b", Reason: "a", AgeExclusion: true},
		{Path: "b", Reason: "b"}, {Path: "c", Reason: "z"},
	}, result.Groups[0].Blockers)
	assert.Equal(t, []DiscoveryExclusion{
		{Path: "a", Reason: "a"}, {Path: "a", Reason: "z"}, {Path: "z", Reason: "excluded"},
	}, result.Exclusions)
	assert.Equal(t, []string{"a", "a", "b"}, result.Groups[0].Members[1].Reasons)
	slices.Reverse(input.Groups)
	slices.Reverse(input.Exclusions)
	for i := range input.Groups {
		slices.Reverse(input.Groups[i].Evaluations)
		slices.Reverse(input.Groups[i].Reports)
		slices.Reverse(input.Groups[i].Blockers)
	}
	assert.Equal(t, result, planPreview("workspace", age, input))
	// Sorting and caller edits must not rewrite the source inventory or blockers.
	assert.Equal(t, "a", input.Groups[0].Blockers[0].Reason)
	result.Groups[0].Blockers[0].Reason = "caller edit"
	result.Exclusions[0].Reason = "caller edit"
	assert.Equal(t, "a", input.Groups[0].Blockers[0].Reason)
	assert.Equal(t, "a", input.Exclusions[0].Reason)
}

func TestPreview_ChangedAliasDiscardsPlanAndReleasesOriginalAccess(t *testing.T) {
	workspace := t.TempDir()
	requirePreviewSupport(t, workspace)
	alias := filepath.Join(t.TempDir(), "alias")
	require.NoError(t, os.Symlink(workspace, alias))
	other := t.TempDir()
	result, err := preview(previewOptions(t, alias), func(opts GroupOptions) (*GroupResult, error) {
		groups, err := PreflightGroups(opts)
		require.NoError(t, err)
		// Simulate an unsupported external namespace change at the I/O boundary.
		require.NoError(t, os.Remove(alias))
		require.NoError(t, os.Symlink(other, alias))
		return groups, nil
	})
	require.ErrorIs(t, err, workspacecoord.ErrWorkspaceChanged)
	assert.Nil(t, result)
	response, stop := previewLockProcess(t, workspace, workspacecoord.Exclusive)
	assert.Empty(t, response.Error)
	stop()
}

func TestPreview_UnsafeCoordinationRefusesBeforeInventory(t *testing.T) {
	workspace := t.TempDir()
	requirePreviewSupport(t, workspace)
	outside := t.TempDir()
	managed := filepath.Join(workspace, managedDirectory)
	require.NoError(t, os.Mkdir(managed, 0o700))
	require.NoError(t, os.Symlink(outside, filepath.Join(managed, "coordination")))
	before, outsideBefore := groupSnapshot(t, workspace), groupSnapshot(t, outside)
	result, err := preview(previewOptions(t, workspace), func(GroupOptions) (*GroupResult, error) {
		t.Error("unsafe coordination must be refused before report discovery")
		return nil, errors.New("unexpected inventory")
	})
	require.ErrorIs(t, err, workspacecoord.ErrUnresolvedActivity)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "coordination")
	assert.Equal(t, before, groupSnapshot(t, workspace))
	assert.Equal(t, outsideBefore, groupSnapshot(t, outside))
}
