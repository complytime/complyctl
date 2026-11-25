// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestDiscover_SpecialEntryRefusesWithoutOpeningIt(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, managedDirectory, "ampel", resultsDirectory)
	require.NoError(t, os.MkdirAll(root, 0o755))
	fifo := filepath.Join(root, "result-pipe")
	require.NoError(t, unix.Mkfifo(fifo, 0o600))
	result, err := Discover(DiscoveryOptions{Workspace: workspace})
	require.ErrorContains(t, err, "symlink or special file")
	assert.Nil(t, result)
	info, err := os.Lstat(fifo)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeNamedPipe)
}

func TestDiscoveryDirectory_RejectsEscapeAndFollowsNoLinks(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(workspace, "safe"), 0o755))
	require.NoError(t, os.Symlink("safe", filepath.Join(workspace, "alias")))
	parent, validate, err := openDiscoveryWorkspace(workspace)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, parent.Close()) })
	for _, path := range []string{"..", "../outside", "alias"} {
		child, err := openDiscoveryDirectory(parent, path)
		require.Error(t, err, path)
		assert.Nil(t, child, path)
	}
	child, err := openDiscoveryDirectory(parent, "safe")
	require.NoError(t, err)
	require.NoError(t, child.Close())
	require.NoError(t, validate())
}

func TestDiscoveryEntry_DetectsDirectoryReplacement(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "label")
	require.NoError(t, os.Mkdir(path, 0o755))
	parent, _, err := openDiscoveryWorkspace(workspace)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, parent.Close()) })
	entry, err := inspectDiscoveryEntry(parent, "label")
	require.NoError(t, err)
	assert.True(t, entry.directory)
	require.NoError(t, os.Rename(path, filepath.Join(workspace, "original")))
	require.NoError(t, os.Mkdir(path, 0o755))
	replacement, err := openDiscoveryDirectory(parent, "label")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, replacement.Close()) })
	require.ErrorContains(t, entry.validate(replacement), "changed during discovery")
	info, err := replacement.Stat()
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestPinnedDiscoveryWorkspace_RevalidatesIdentity(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	require.NoError(t, os.Mkdir(workspace, 0o755))
	parent, validate, err := openPinnedDiscoveryWorkspace(workspace)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, parent.Close()) })
	original, err := parent.Stat()
	require.NoError(t, err)
	require.NoError(t, validate())
	require.NoError(t, os.Rename(workspace, workspace+"-original"))
	require.ErrorIs(t, validate(), os.ErrNotExist)
	require.NoError(t, os.Mkdir(workspace, 0o755))
	require.ErrorContains(t, validate(), "workspace directory changed")
	current, err := parent.Stat()
	require.NoError(t, err)
	assert.True(t, os.SameFile(original, current), "descriptor must stay pinned to original")
}

func TestPinnedDiscoveryWorkspace_RejectsMissingFileAndSymlink(t *testing.T) {
	base := t.TempDir()
	filePath := filepath.Join(base, "file")
	require.NoError(t, os.WriteFile(filePath, []byte("not a directory"), 0o600))
	alias := filepath.Join(base, "alias")
	require.NoError(t, os.Symlink(base, alias))
	for _, path := range []string{filepath.Join(base, "missing"), filePath, alias} {
		file, validate, err := openPinnedDiscoveryWorkspace(path)
		require.Error(t, err, path)
		assert.Nil(t, file, path)
		assert.Nil(t, validate, path)
	}
}

func TestWalkDiscoveryDirectory_ConfinesPathsAndOwnsDescriptors(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"safe/nested", ""},
		{"safe/missing", "confined directory traversal"},
		{"safe/alias", "confined directory traversal"},
		{"safe/../outside", "invalid discovery component"},
		{"safe/./nested", "invalid discovery component"},
		{"safe//nested", "invalid discovery component"},
		{"safe/", "invalid discovery component"},
		{"/safe", "invalid discovery component"},
		{"", "invalid discovery component"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			workspace := t.TempDir()
			nested := filepath.Join(workspace, "safe", "nested")
			require.NoError(t, os.MkdirAll(nested, 0o755))
			require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(workspace, "safe", "alias")))
			parent, _, err := openPinnedDiscoveryWorkspace(workspace)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, parent.Close()) })
			var opened []*os.File
			openComponent := func(dir *os.File, name string) (*os.File, error) {
				file, err := openDiscoveryDirectory(dir, name)
				if err == nil {
					opened = append(opened, file)
				}
				return file, err
			}
			leaf, err := walkDiscoveryDirectory(parent, test.path, openComponent)
			if test.want != "" {
				require.ErrorContains(t, err, test.want)
				assert.Nil(t, leaf)
			} else {
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, leaf.Close()) })
				actual, err := leaf.Stat()
				require.NoError(t, err)
				expected, err := os.Stat(nested)
				require.NoError(t, err)
				assert.True(t, os.SameFile(expected, actual))
			}
			for _, file := range opened {
				if file != leaf {
					_, err := file.Stat()
					assert.ErrorIs(t, err, os.ErrClosed, "intermediate descriptor must close")
				}
			}
			info, err := parent.Stat()
			require.NoError(t, err, "walker must not close caller's descriptor")
			assert.True(t, info.IsDir())
		})
	}
}

func TestWalkDiscoveryDirectory_CloseFailureClosesNewChild(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "safe", "nested"), 0o755))
	parent, _, err := openPinnedDiscoveryWorkspace(workspace)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, parent.Close()) })
	var opened []*os.File
	openComponent := func(dir *os.File, name string) (*os.File, error) {
		file, err := openDiscoveryDirectory(dir, name)
		require.NoError(t, err)
		opened = append(opened, file)
		if len(opened) == 2 {
			// Inject an intermediate close failure after opening its child.
			require.NoError(t, dir.Close())
		}
		return file, nil
	}
	leaf, err := walkDiscoveryDirectory(parent, "safe/nested", openComponent)
	require.ErrorIs(t, err, os.ErrClosed)
	assert.Nil(t, leaf)
	require.Len(t, opened, 2)
	for _, file := range opened {
		_, err := file.Stat()
		assert.ErrorIs(t, err, os.ErrClosed)
	}
	_, err = parent.Stat()
	assert.NoError(t, err)
}

func TestValidateDiscoveryMount_RejectsNestedAndAmbiguousPaths(t *testing.T) {
	tests := []struct {
		name, workspace, mount, want string
	}{
		{"workspace mount", "/Volumes/data/work", "/Volumes/data/work", ""},
		{"root mount", "/Volumes/data/work", "/", ""},
		{"ancestor mount", "/Volumes/data/work", "/Volumes/data", ""},
		{"sibling mount", "/Volumes/data/work", "/Volumes/data/other", ""},
		{"prefix sibling", "/Volumes/data/work", "/Volumes/data/work-copy", ""},
		{"nested mount", "/Volumes/data/work", "/Volumes/data/work/results", "nested mount"},
		{"root workspace", "/", "/Volumes/data", "nested mount"},
		{"relative mount", "/Volumes/data/work", "results", "ambiguous discovery mount path"},
		{"empty mount", "/Volumes/data/work", "", "ambiguous discovery mount path"},
		{"relative workspace", "relative", "/Volumes/data", "resolve discovery mount path"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateDiscoveryMount(test.workspace, test.mount)
			if test.want != "" {
				require.ErrorContains(t, err, test.want)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestDiscoveryDescriptors_RejectClosedAndInvalidHandles(t *testing.T) {
	file, err := os.Open(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, file.Close())
	fd, err := discoveryFD(file)
	require.ErrorIs(t, err, unix.EBADF)
	assert.Equal(t, -1, fd)
	opened, err := discoveryFile(-1, "invalid")
	require.ErrorIs(t, err, unix.EBADF)
	assert.Nil(t, opened)
	_, err = inspectDiscoveryEntry(file, "missing")
	assert.ErrorIs(t, err, unix.EBADF)
	assert.False(t, errors.Is(err, os.ErrNotExist), "invalid handle is not a missing root")
}
