// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package workspacecoord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

const (
	helperEnvironment = "COMPLYCTL_WORKSPACECOORD_TEST_HELPER"
	helperTimeout     = 15 * time.Second
)

type helperRequest struct {
	Options
	CheckCleanup bool
}

type helperResponse struct {
	Path                  string
	Error                 string
	Busy                  bool
	CleanupError          string
	UnsupportedFilesystem bool
}

// TestAcquire_HelperProcess runs only in a child test binary. The JSON response
// is the acquisition barrier; stdin EOF is the release barrier. No timing-based
// inference is used to decide whether the directory is locked.
func TestAcquire_HelperProcess(t *testing.T) {
	requestJSON := os.Getenv(helperEnvironment)
	if requestJSON == "" {
		return
	}
	var request helperRequest
	require.NoError(t, json.Unmarshal([]byte(requestJSON), &request))
	access, err := Acquire(request.Options)
	response := helperResponse{}
	if err != nil {
		response.Error = err.Error()
		response.Busy = errors.Is(err, ErrBusy)
	} else {
		response.Path = access.Path()
		if request.CheckCleanup {
			if err := access.CheckCleanupSupport(); err != nil {
				response.CleanupError = err.Error()
				response.UnsupportedFilesystem = errors.Is(err, ErrUnsupportedFilesystem)
			}
		}
	}
	require.NoError(t, json.NewEncoder(os.Stdout).Encode(response))
	if access != nil {
		_, err := io.Copy(io.Discard, os.Stdin)
		require.NoError(t, err)
		require.NoError(t, access.Close())
	}
}

func startLockProcess(t *testing.T, request helperRequest) (helperResponse, func()) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	requestJSON, err := json.Marshal(request)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), helperTimeout)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestAcquire_HelperProcess$")
	cmd.Env = append(os.Environ(), helperEnvironment+"="+string(requestJSON))
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
			assert.NoError(t, ctx.Err(), "child must respond without waiting for a conflicting lock")
		})
	}
	t.Cleanup(stop)
	var response helperResponse
	require.NoError(t, json.NewDecoder(output).Decode(&response))
	return response, stop
}

func acquireForTest(t *testing.T, workspace string, mode Mode) *Access {
	t.Helper()
	access, err := Acquire(Options{Workspace: workspace, Mode: mode})
	require.NoError(t, err)
	t.Cleanup(func() {
		// Some tests explicitly close early to verify release.
		err := access.Close()
		if !errors.Is(err, os.ErrClosed) {
			assert.NoError(t, err)
		}
	})
	return access
}

func TestAcquire_SubprocessContention(t *testing.T) {
	for _, test := range []struct {
		name      string
		holder    Mode
		contender Mode
		busy      bool
	}{
		{"shared-shared", Shared, Shared, false},
		{"shared-exclusive", Shared, Exclusive, true},
		{"exclusive-shared", Exclusive, Shared, true},
		{"exclusive-exclusive", Exclusive, Exclusive, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			holder := acquireForTest(t, workspace, test.holder)
			response, stop := startLockProcess(t, helperRequest{
				Options: Options{Workspace: workspace, Mode: test.contender},
			})
			assert.Equal(t, test.busy, response.Busy)
			if test.busy {
				assert.Contains(t, response.Error, holder.Path())
				assert.Contains(t, response.Error, "workspace operation is active")
				assert.Contains(t, response.Error, "retry")
			} else {
				assert.Empty(t, response.Error)
				assert.Equal(t, holder.Path(), response.Path)
			}
			stop()
			require.NoError(t, holder.Close())
			response, stop = startLockProcess(t, helperRequest{
				Options: Options{Workspace: workspace, Mode: Exclusive},
			})
			assert.Empty(t, response.Error, "access must become available after release")
			assert.Equal(t, holder.Path(), response.Path)
			stop()
		})
	}
}

func TestAcquire_AliasesContendAcrossProcesses(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	require.NoError(t, os.Mkdir(workspace, 0o700))
	alias := filepath.Join(parent, "alias")
	require.NoError(t, os.Symlink(workspace, alias))
	workingDirectory, err := os.Getwd()
	require.NoError(t, err)
	relative, err := filepath.Rel(workingDirectory, workspace)
	require.NoError(t, err)
	canonical, err := filepath.EvalSymlinks(workspace)
	require.NoError(t, err)
	for _, path := range []string{workspace, relative, alias, alias + string(os.PathSeparator)} {
		t.Run(path, func(t *testing.T) {
			response, stop := startLockProcess(t, helperRequest{
				Options: Options{Workspace: path, Mode: Shared},
			})
			require.Empty(t, response.Error)
			assert.Equal(t, canonical, response.Path)
			access, err := Acquire(Options{Workspace: canonical, Mode: Exclusive})
			assert.Nil(t, access)
			assert.ErrorIs(t, err, ErrBusy)
			stop()
		})
	}
}

func TestAcquire_IndependentWorkspaces(t *testing.T) {
	first := acquireForTest(t, t.TempDir(), Exclusive)
	second := t.TempDir()
	response, stop := startLockProcess(t, helperRequest{
		Options: Options{Workspace: second, Mode: Exclusive},
	})
	require.Empty(t, response.Error)
	assert.NotEqual(t, first.Path(), response.Path)
	access, err := Acquire(Options{Workspace: second, Mode: Shared})
	assert.Nil(t, access)
	assert.ErrorIs(t, err, ErrBusy, "the second process really holds its workspace")
	stop()
	acquireForTest(t, second, Exclusive)
	assert.NoError(t, first.Validate(), "the first workspace must remain independently held")
}

func TestAcquire_ReadOnlyDirectoryHasNoLockFile(t *testing.T) {
	for _, mode := range []Mode{Shared, Exclusive} {
		t.Run(map[Mode]string{Shared: "shared", Exclusive: "exclusive"}[mode], func(t *testing.T) {
			workspace := t.TempDir()
			fixture := filepath.Join(workspace, "existing-evidence")
			contents := []byte("retained evidence\n")
			require.NoError(t, os.WriteFile(fixture, contents, 0o400))
			require.NoError(t, os.Chmod(workspace, 0o500))
			t.Cleanup(func() { assert.NoError(t, os.Chmod(workspace, 0o700)) })
			before, err := os.Stat(workspace)
			require.NoError(t, err)
			response, stop := startLockProcess(t, helperRequest{
				Options:      Options{Workspace: workspace, Mode: mode},
				CheckCleanup: mode == Exclusive,
			})
			require.Empty(t, response.Error)
			if response.CleanupError != "" {
				assert.True(t, response.UnsupportedFilesystem, response.CleanupError)
				t.Log(response.CleanupError)
			}
			stop()
			entries, err := os.ReadDir(workspace)
			require.NoError(t, err)
			require.Len(t, entries, 1, "acquisition must not create coordination state")
			assert.Equal(t, "existing-evidence", entries[0].Name())
			after, err := os.Stat(workspace)
			require.NoError(t, err)
			assert.Equal(t, before.Mode(), after.Mode())
			assert.Equal(t, before.ModTime(), after.ModTime())
			actual, err := os.ReadFile(fixture)
			require.NoError(t, err)
			assert.Equal(t, contents, actual)
		})
	}
}

func TestAcquire_InvalidInputs(t *testing.T) {
	workspace := t.TempDir()
	regular := filepath.Join(workspace, "regular")
	require.NoError(t, os.WriteFile(regular, nil, 0o600))
	for _, test := range []struct {
		name string
		opts Options
		want string
	}{
		{"empty", Options{Mode: Shared}, "must not be empty"},
		{"missing", Options{Workspace: filepath.Join(workspace, "missing"), Mode: Shared},
			"resolve workspace"},
		{"regular", Options{Workspace: regular, Mode: Exclusive}, "open workspace directory"},
		{"zero-mode", Options{Workspace: workspace}, "invalid workspace access mode"},
		{"unknown-mode", Options{Workspace: workspace, Mode: Mode(99)}, "invalid workspace access mode"},
	} {
		t.Run(test.name, func(t *testing.T) {
			access, err := Acquire(test.opts)
			assert.Nil(t, access)
			assert.ErrorContains(t, err, test.want)
		})
	}
	entries, err := os.ReadDir(workspace)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "regular", entries[0].Name())
}

func TestAccess_RejectsReplacedWorkspaceButPinsOriginal(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	renamed := filepath.Join(parent, "renamed")
	require.NoError(t, os.Mkdir(workspace, 0o700))
	access := acquireForTest(t, workspace, Exclusive)
	require.NoError(t, os.Rename(workspace, renamed))
	require.NoError(t, os.Mkdir(workspace, 0o700))
	assert.ErrorIs(t, access.Validate(), ErrWorkspaceChanged)
	assert.ErrorIs(t, access.CheckCleanupSupport(), ErrWorkspaceChanged)
	response, stop := startLockProcess(t, helperRequest{
		Options: Options{Workspace: renamed, Mode: Shared},
	})
	assert.True(t, response.Busy, "renaming must not release the original directory lock")
	stop()
	acquireForTest(t, workspace, Exclusive)
}

func TestAccess_RejectsChangedAlias(t *testing.T) {
	parent := t.TempDir()
	original := t.TempDir()
	alias := filepath.Join(parent, "alias")
	require.NoError(t, os.Symlink(original, alias))
	access := acquireForTest(t, alias, Exclusive)
	require.NoError(t, os.Remove(alias))
	assert.ErrorIs(t, access.Validate(), ErrWorkspaceChanged)
	require.NoError(t, os.Symlink(t.TempDir(), alias))
	assert.ErrorIs(t, access.Validate(), ErrWorkspaceChanged)
	response, stop := startLockProcess(t, helperRequest{
		Options: Options{Workspace: original, Mode: Shared},
	})
	assert.True(t, response.Busy)
	stop()
}

func TestAccess_RequiresExclusiveCleanupAndOpenDescriptor(t *testing.T) {
	access := acquireForTest(t, t.TempDir(), Shared)
	assert.ErrorContains(t, access.CheckCleanupSupport(), "requires exclusive")
	require.NoError(t, access.Close())
	assert.ErrorIs(t, access.Validate(), os.ErrClosed)
	assert.ErrorIs(t, access.CheckCleanupSupport(), os.ErrClosed)
	assert.ErrorIs(t, access.Close(), os.ErrClosed)
	assert.ErrorIs(t, checkFilesystem(access.file), unix.EBADF)
	assert.ErrorIs(t, lockDirectory(access.file, Exclusive), unix.EBADF)
}

func TestOpenDirectory_ReadOnlyCloseOnExecAndNoFollow(t *testing.T) {
	workspace := t.TempDir()
	access := acquireForTest(t, workspace, Exclusive)
	flags, err := unix.FcntlInt(access.file.Fd(), unix.F_GETFL, 0)
	require.NoError(t, err)
	assert.Equal(t, unix.O_RDONLY, flags&unix.O_ACCMODE)
	flags, err = unix.FcntlInt(access.file.Fd(), unix.F_GETFD, 0)
	require.NoError(t, err)
	assert.NotZero(t, flags&unix.FD_CLOEXEC)
	alias := filepath.Join(t.TempDir(), "alias")
	require.NoError(t, os.Symlink(workspace, alias))
	file, err := openDirectory(alias)
	assert.Nil(t, file)
	assert.Error(t, err, "the canonical final component must not turn into a followed symlink")
}
