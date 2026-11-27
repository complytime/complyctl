// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func removalFixture(t *testing.T) (string, string, *os.File, os.FileInfo) {
	t.Helper()
	workspace := t.TempDir()
	unit := addGroupEvaluation(t, workspace, "ampel", "label", groupOldStart)
	canonical, err := filepath.EvalSymlinks(workspace)
	require.NoError(t, err)
	file, _, err := openDiscoveryWorkspace(canonical)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })
	identity, err := os.Stat(filepath.Join(workspace, unit))
	require.NoError(t, err)
	return workspace, unit, file, identity
}

func removalOutsideSentinel(t *testing.T) string {
	t.Helper()
	outside := t.TempDir()
	path := filepath.Join(outside, "sentinel")
	require.NoError(t, os.WriteFile(path, []byte("independent outside bytes"), 0o600))
	identity, err := os.Lstat(path)
	require.NoError(t, err)
	t.Cleanup(func() {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "independent outside bytes", string(data))
		current, err := os.Lstat(path)
		require.NoError(t, err)
		assert.True(t, os.SameFile(identity, current), "outside sentinel inode must not change")
		assert.Equal(t, identity.Mode(), current.Mode())
		assert.Equal(t, identity.ModTime(), current.ModTime())
	})
	return outside
}

func TestRemoval_DeletesOnlyPinnedMember(t *testing.T) {
	for _, provider := range []string{"ampel", "opa", "openscap"} {
		t.Run(provider, func(t *testing.T) {
			workspace, _, file, _ := removalFixture(t)
			unit := addGroupEvaluation(t, workspace, provider, "nested/label", groupOldStart)
			identity, err := os.Stat(filepath.Join(workspace, unit))
			require.NoError(t, err)
			outside := removalOutsideSentinel(t)
			require.NotEqual(t, workspace, outside)
			excluded := filepath.Join(workspace, "unrelated")
			require.NoError(t, os.WriteFile(excluded, []byte("keep"), 0o600))
			r, err := prepareRemoval(file, unit, identity)
			require.NoError(t, err)
			defer func() { require.NoError(t, r.close()) }()
			require.NoError(t, r.remove())
			assert.NoDirExists(t, filepath.Join(workspace, unit))
			assert.DirExists(t, filepath.Dir(filepath.Join(workspace, unit)))
			data, err := os.ReadFile(excluded)
			require.NoError(t, err)
			assert.Equal(t, "keep", string(data))
		})
	}
}

func TestRemoval_ReportIdentityAndScope(t *testing.T) {
	workspace, _, file, _ := removalFixture(t)
	path := filepath.Join(reportDirectory, "evaluation-log-policy.json")
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, reportDirectory), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, path), []byte("report"), 0o600))
	identity, err := os.Stat(filepath.Join(workspace, path))
	require.NoError(t, err)
	r, err := prepareRemoval(file, path, identity)
	require.NoError(t, err)
	defer func() { require.NoError(t, r.close()) }()
	require.NoError(t, r.remove())
	assert.NoFileExists(t, filepath.Join(workspace, path))
	assert.DirExists(t, filepath.Join(workspace, reportDirectory))
	for _, path := range []string{"../escape", ".complytime/scan/../log", ".complytime/scan/debug.log",
		".complytime/scan", ".complytime/unknown/results/unit", ".complytime/ampel/results/label"} {
		_, err := prepareRemoval(file, path, identity)
		require.Error(t, err, path)
	}
	_, err = prepareRemoval(file, "anything", nil)
	require.ErrorContains(t, err, "missing identity")
}

func TestRemoval_UnsafeEntriesPreserveBytes(t *testing.T) {
	for _, kind := range []string{"root", "label", "unit", "nested-link", "fifo", "socket", "permission"} {
		t.Run(kind, func(t *testing.T) {
			workspace, unit, file, identity := removalFixture(t)
			outside := removalOutsideSentinel(t)
			switch kind {
			case "root", "label", "unit":
				path := filepath.Join(workspace, unit)
				switch kind {
				case "root":
					path = filepath.Join(workspace, managedDirectory, "ampel", resultsDirectory)
				case "label":
					path = filepath.Dir(path)
				}
				require.NoError(t, os.Rename(path, path+"-saved"))
				require.NoError(t, os.Symlink(outside, path))
			case "nested-link":
				require.NoError(t, os.Symlink(outside, filepath.Join(workspace, unit, "nested", "link")))
			case "fifo":
				require.NoError(t, unix.Mkfifo(filepath.Join(workspace, unit, "fifo"), 0o600))
			case "socket":
				// Bind relative to a test-scoped cwd: even a TempDir-based "short"
				// absolute path can exceed Darwin's sockaddr_un limit. No link or
				// filesystem-specific socket relocation is needed.
				path := filepath.Join(workspace, unit, "s")
				t.Chdir(filepath.Join(workspace, unit))
				listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: "s", Net: "unix"})
				require.NoError(t, err)
				listener.SetUnlinkOnClose(false)
				require.NoError(t, listener.Close())
				info, err := os.Lstat(path)
				require.NoError(t, err)
				require.Equal(t, os.ModeSocket, info.Mode().Type())
			case "permission":
				path := filepath.Join(workspace, unit, "nested")
				require.NoError(t, os.Chmod(path, 0o500))
				t.Cleanup(func() { require.NoError(t, os.Chmod(path, 0o700)) })
			}
			before := groupSnapshot(t, workspace)
			r, err := prepareRemoval(file, unit, identity)
			require.Error(t, err)
			assert.Nil(t, r)
			assert.Equal(t, before, groupSnapshot(t, workspace))
		})
	}
}

func TestRemoval_ReplacementWhileHandlesHeld(t *testing.T) {
	for _, target := range []string{"workspace", "root", "label", "unit", "nested", "file"} {
		t.Run(target, func(t *testing.T) {
			workspace, unit, file, identity := removalFixture(t)
			outside := removalOutsideSentinel(t)
			r, err := prepareRemoval(file, unit, identity)
			require.NoError(t, err)
			defer func() { require.NoError(t, r.close()) }()
			path := filepath.Join(workspace, unit)
			switch target {
			case "workspace":
				path = workspace
			case "root":
				path = filepath.Join(workspace, managedDirectory, "ampel", resultsDirectory)
			case "label":
				path = filepath.Dir(path)
			case "nested":
				path = filepath.Join(path, "nested")
			case "file":
				path = filepath.Join(path, "first.json")
			}
			// Preparation is the deterministic barrier: every original handle is
			// retained before the attacker installs a replacement. No sleeps.
			saved := path + "-saved"
			require.NoError(t, os.Rename(path, saved))
			t.Cleanup(func() {
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.Rename(saved, path))
			})
			require.NoError(t, os.Symlink(outside, path))
			if target == "file" {
				before, err := os.ReadFile(saved)
				require.NoError(t, err)
				require.Error(t, r.remove())
				after, err := os.ReadFile(saved)
				require.NoError(t, err)
				assert.Equal(t, before, after)
				return
			}
			before := groupSnapshot(t, saved)
			require.Error(t, r.remove())
			assert.Equal(t, before, groupSnapshot(t, saved))
		})
	}
}

func TestRemoval_SameTypeReplacementWhileHandlesHeld(t *testing.T) {
	for _, target := range []string{"unit", "file"} {
		t.Run(target, func(t *testing.T) {
			workspace, unit, file, identity := removalFixture(t)
			removalOutsideSentinel(t)
			r, err := prepareRemoval(file, unit, identity)
			require.NoError(t, err)
			defer func() { require.NoError(t, r.close()) }()
			path := filepath.Join(workspace, unit)
			if target == "file" {
				path = filepath.Join(path, "first.json")
			}
			original, err := os.Lstat(path)
			require.NoError(t, err)
			saved := path + "-saved"
			require.NoError(t, os.Rename(path, saved))
			if target == "unit" {
				require.NoError(t, os.Mkdir(path, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(path, "sentinel"), []byte("replacement"), 0o600))
			} else {
				require.NoError(t, os.WriteFile(path, []byte("replacement"), 0o600))
			}
			replacement, err := os.Lstat(path)
			require.NoError(t, err)
			require.False(t, os.SameFile(original, replacement))
			before := groupSnapshot(t, workspace)
			require.ErrorContains(t, r.remove(), "identity")
			assert.Equal(t, before, groupSnapshot(t, workspace))
			current, err := os.Lstat(path)
			require.NoError(t, err)
			assert.True(t, os.SameFile(replacement, current))
			retained, err := os.Lstat(saved)
			require.NoError(t, err)
			assert.True(t, os.SameFile(original, retained))
		})
	}
}

func TestRemoval_ChangedCandidateAndInjectedFailures(t *testing.T) {
	for _, fault := range []string{"candidate", "mode", "contents", "guard", "unlink"} {
		t.Run(fault, func(t *testing.T) {
			workspace, unit, file, identity := removalFixture(t)
			removalOutsideSentinel(t)
			if fault == "candidate" {
				path := filepath.Join(workspace, unit)
				require.NoError(t, os.Rename(path, path+"-saved"))
				require.NoError(t, os.Mkdir(path, 0o700))
				r, err := prepareRemoval(file, unit, identity)
				require.ErrorContains(t, err, "identity")
				assert.Nil(t, r)
				assert.DirExists(t, path)
				assert.FileExists(t, filepath.Join(path+"-saved", "first.json"))
				return
			}
			r, err := prepareRemoval(file, unit, identity)
			require.NoError(t, err)
			defer func() { require.NoError(t, r.close()) }()
			switch fault {
			case "mode":
				require.NoError(t, os.Chmod(filepath.Join(workspace, unit, "first.json"), 0o400))
			case "contents":
				require.NoError(t, os.WriteFile(filepath.Join(workspace, unit, "first.json"), []byte("changed"), 0o600))
			case "guard":
				r.guard = func() error { return fmt.Errorf("injected unavailable confinement") }
			case "unlink":
				r.unlink = func(*os.File, string, bool) error { return unix.EIO }
			}
			before := groupSnapshot(t, workspace)
			require.Error(t, r.remove())
			assert.Equal(t, before, groupSnapshot(t, workspace))
		})
	}
}

func TestRemoval_InvalidComponentsAndBounds(t *testing.T) {
	_, unit, file, identity := removalFixture(t)
	for _, name := range []string{".", "..", "", "nested/name", "/absolute"} {
		require.Error(t, unlinkRemovalEntry(file, name, false))
	}
	r := &removal{workspace: file}
	defer func() { require.NoError(t, r.close()) }()
	parent, err := r.pinParents(filepath.Dir(unit))
	require.NoError(t, err)
	for _, budget := range []int{0, -1} {
		_, err := r.pinEntry(parent, filepath.Base(unit), unit, identity, 0, &budget)
		require.ErrorContains(t, err, "limit")
	}
	budget := maxPreflightEntries
	_, err = r.pinEntry(parent, filepath.Base(unit), unit, identity, maxPreflightDepth, &budget)
	require.ErrorContains(t, err, "limit")
	assert.Error(t, validateRemovalPath(strings.Replace(unit, "ampel", "unknown", 1), identity))
	assert.Error(t, validateRemovalPath(strings.Replace(unit, "label", "bad.label", 1), identity))
}
