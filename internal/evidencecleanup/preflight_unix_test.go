// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestPreflightGroups_BlockedMemberRetainsEntireConnectedGroup(t *testing.T) {
	for _, failure := range []string{
		"missing-evidence", "unresolved-mapping", "symlink", "fifo", "nested-permission",
		"parent-permission", "report-permission", "sticky-parent",
	} {
		t.Run(failure, func(t *testing.T) {
			workspace := t.TempDir()
			old := addGroupEvaluation(t, workspace, "ampel", "old", groupOldStart)
			shared := addGroupEvaluation(t, workspace, "opa", "shared", groupOldStart)
			last := addGroupEvaluation(t, workspace, "openscap", "last", groupOldStart)
			firstReport := addGroupReport(t, workspace, "a", filepath.Join(old, "first.json"),
				filepath.Join(shared, "first.json"))
			addGroupReport(t, workspace, "b", filepath.Join(shared, "nested/second.json"),
				filepath.Join(last, "first.json"))
			independent := addGroupEvaluation(t, workspace, "opa", "independent", groupOldStart)
			blockPath := installGroupBlocker(t, workspace, last, firstReport, failure)
			before := groupSnapshot(t, workspace)
			result, err := PreflightGroups(GroupOptions{Workspace: workspace, Age: groupAge(t)})
			require.NoError(t, err)
			require.Len(t, result.Groups, 2)
			var blocked, safe *Group
			for i := range result.Groups {
				group := &result.Groups[i]
				if group.Evaluations[0].Path == independent {
					safe = group
				} else {
					blocked = group
				}
			}
			require.NotNil(t, safe)
			require.NotNil(t, blocked)
			assert.True(t, safe.Eligible(), "%+v", safe.Blockers)
			assert.False(t, blocked.Eligible())
			assert.Len(t, blocked.Evaluations, 3)
			assert.Len(t, blocked.Reports, 2)
			require.NotEmpty(t, blocked.Blockers)
			var reasons []string
			for _, blocker := range blocked.Blockers {
				assert.False(t, blocker.AgeExclusion)
				reasons = append(reasons, blocker.Path+": "+blocker.Reason)
			}
			assert.Contains(t, strings.Join(reasons, "\n"), blockPath)
			assert.Equal(t, before, groupSnapshot(t, workspace))
		})
	}
}

func installGroupBlocker(t *testing.T, workspace, unit, report, failure string) string {
	t.Helper()
	path := filepath.Join(workspace, unit, "nested")
	switch failure {
	case "missing-evidence":
		require.NoError(t, os.Remove(filepath.Join(workspace, unit, "first.json")))
		return filepath.Join(unit, "first.json")
	case "unresolved-mapping":
		root, err := os.OpenRoot(workspace)
		require.NoError(t, err)
		defer func() { require.NoError(t, root.Close()) }()
		data, err := root.ReadFile(report)
		require.NoError(t, err)
		data = []byte(strings.ReplaceAll(string(data), `"reference-id": "retained"`,
			`"reference-id": "missing-reference"`))
		require.Contains(t, string(data), "missing-reference")
		require.NoError(t, root.WriteFile(report, data, 0o600))
		return "unresolved mapping-reference"
	case "symlink":
		outside := filepath.Join(t.TempDir(), "untouched")
		require.NoError(t, os.WriteFile(outside, []byte("external evidence"), 0o600))
		t.Cleanup(func() {
			data, err := os.ReadFile(outside)
			require.NoError(t, err)
			assert.Equal(t, "external evidence", string(data))
		})
		require.NoError(t, os.Symlink(outside, filepath.Join(path, "unsafe")))
		return filepath.Join(unit, "nested/unsafe")
	case "fifo":
		require.NoError(t, unix.Mkfifo(filepath.Join(path, "pipe"), 0o600))
		return filepath.Join(unit, "nested/pipe")
	case "nested-permission":
		require.NoError(t, os.Chmod(path, 0o500))
	case "parent-permission":
		path = filepath.Dir(filepath.Join(workspace, unit))
		require.NoError(t, os.Chmod(path, 0o500))
	case "report-permission":
		path = filepath.Dir(filepath.Join(workspace, report))
		require.NoError(t, os.Chmod(path, 0o500))
		t.Cleanup(func() { require.NoError(t, os.Chmod(path, 0o700)) })
		return report
	case "sticky-parent":
		path = filepath.Dir(filepath.Join(workspace, unit))
		require.NoError(t, os.Chmod(path, os.ModeSticky|0o700))
	}
	t.Cleanup(func() { require.NoError(t, os.Chmod(path, 0o700)) })
	return unit
}

func TestPreflightMember_RevalidatesIdentityAndContents(t *testing.T) {
	for _, change := range []string{"unchanged", "replaced", "resized", "mode", "symlink"} {
		t.Run(change, func(t *testing.T) {
			workspace := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(workspace, "scan"), 0o700))
			path := filepath.Join(workspace, "scan/report")
			require.NoError(t, os.WriteFile(path, []byte("report bytes"), 0o600))
			identity, err := os.Stat(path)
			require.NoError(t, err)
			parent, _, err := openDiscoveryWorkspace(workspace)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, parent.Close()) })
			switch change {
			case "replaced":
				require.NoError(t, os.Rename(path, path+"-original"))
				require.NoError(t, os.WriteFile(path, []byte("report bytes"), 0o600))
			case "resized":
				require.NoError(t, os.WriteFile(path, []byte("changed"), 0o600))
			case "mode":
				require.NoError(t, os.Chmod(path, 0o400))
			case "symlink":
				require.NoError(t, os.Rename(path, path+"-original"))
				require.NoError(t, os.Symlink(path+"-original", path))
			}
			before := groupSnapshot(t, workspace)
			err = preflightMember(parent, "scan/report", identity)
			if change == "unchanged" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				if change != "symlink" {
					assert.Contains(t, err.Error(), "identity or report contents changed")
				}
			}
			assert.Equal(t, before, groupSnapshot(t, workspace))
		})
	}
}

func TestPreflightContents_BoundsTraversalAndRejectsInvalidMembers(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(workspace, "nested"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "nested/file"), []byte("bytes"), 0o600))
	for _, test := range []struct {
		name      string
		depth     int
		remaining int
		reason    string
	}{
		{"complete", 0, 2, ""},
		{"depth", maxPreflightDepth, 2, "depth limit"},
		{"entries", 0, 1, "entry limit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent, _, err := openDiscoveryWorkspace(workspace)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, parent.Close()) })
			remaining := test.remaining
			err = preflightContents(parent, ".", test.depth, &remaining)
			if test.reason == "" {
				require.NoError(t, err)
				assert.Zero(t, remaining)
			} else {
				require.ErrorContains(t, err, test.reason)
			}
		})
	}
	parent, _, err := openDiscoveryWorkspace(workspace)
	require.NoError(t, err)
	defer func() { require.NoError(t, parent.Close()) }()
	info, err := parent.Stat()
	require.NoError(t, err)
	require.ErrorContains(t, preflightMember(parent, "nested", nil), "missing identity")
	require.ErrorContains(t, preflightMember(parent, "../outside", info), "nonlocal")
	require.ErrorContains(t, preflightMember(parent, "absent/child", info), "member parent")
}
