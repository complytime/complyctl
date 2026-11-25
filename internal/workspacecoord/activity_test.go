// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package workspacecoord

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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

const activityHelperEnvironment = "COMPLYCTL_ACTIVITY_TEST_WORKSPACE"
const providerHelperEnvironment = "COMPLYCTL_ACTIVITY_TEST_PROVIDER_RECORD"

func activityOptions() ActivityOptions {
	return ActivityOptions{Operation: OperationScan,
		StartedAt: time.Date(2026, time.November, 19, 12, 0, 0, 0, time.UTC)}
}

func beginForTest(t *testing.T, access *Access) *Activity {
	t.Helper()
	activity, err := access.BeginActivity(activityOptions())
	require.NoError(t, err)
	return activity
}

func assertActivityPaths(t *testing.T, access *Access, paths ...string) {
	t.Helper()
	err := access.CheckNoActivity()
	if len(paths) == 0 {
		assert.NoError(t, err)
		return
	}
	require.ErrorIs(t, err, ErrUnresolvedActivity)
	var blocked *ActivityError
	require.ErrorAs(t, err, &blocked)
	assert.Equal(t, access.Path(), blocked.Workspace)
	assert.ElementsMatch(t, paths, blocked.Paths)
	assert.Contains(t, err.Error(), "no evidence was deleted")
	for _, path := range paths {
		assert.Contains(t, err.Error(), fmt.Sprintf("%q", path))
	}
}

func TestBeginActivity_PersistsPrivateUniqueRecords(t *testing.T) {
	workspace := t.TempDir()
	access := acquireForTest(t, workspace, Shared)
	opts := activityOptions()
	opts.Operation = OperationGenerate
	opts.ProviderPIDs = []int{123, 456}
	first, err := access.BeginActivity(opts)
	require.NoError(t, err)
	second := beginForTest(t, access)
	assert.NotEqual(t, first.Path(), second.Path())
	assert.Equal(t, filepath.Join(access.Path(), activityDirectory,
		coordinationDirectory, operationsDirectory), filepath.Dir(first.Path()))
	contents, err := os.ReadFile(first.Path())
	require.NoError(t, err)
	var record activityRecord
	require.NoError(t, json.Unmarshal(contents, &record))
	assert.Equal(t, activityVersion, record.Version)
	assert.Equal(t, "operation-"+record.ID+".json", filepath.Base(first.Path()))
	assert.Len(t, record.ID, 2*activityTokenBytes)
	assert.Equal(t, opts.StartedAt, record.StartedAt)
	assert.Equal(t, "generate", record.Operation)
	assert.Equal(t, access.Path(), record.Workspace)
	assert.Equal(t, os.Getpid(), record.HostPID)
	assert.Equal(t, opts.ProviderPIDs, record.ProviderPIDs)
	info, err := os.Stat(first.Path())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	for _, path := range []string{filepath.Dir(first.Path()),
		filepath.Dir(filepath.Dir(first.Path())), filepath.Join(workspace, activityDirectory)} {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	}
	flags, err := unix.FcntlInt(first.file.Fd(), unix.F_GETFD, 0)
	require.NoError(t, err)
	assert.NotZero(t, flags&unix.FD_CLOEXEC)

	require.NoError(t, first.CompleteNormal())
	assert.NoFileExists(t, first.Path())
	assert.FileExists(t, second.Path(), "completion owns only its allocated record")
	assert.ErrorContains(t, first.CompleteNormal(), "already completed")
	// Explicit completion must not release access before late report publication.
	response, stop := startLockProcess(t, helperRequest{
		Options: Options{Workspace: workspace, Mode: Exclusive},
	})
	assert.True(t, response.Busy)
	stop()
	require.NoError(t, access.Close())
	assert.ErrorIs(t, second.CompleteNormal(), os.ErrClosed)
	cleanup := acquireForTest(t, workspace, Exclusive)
	assertActivityPaths(t, cleanup, second.Path())
	_, err = first.file.Stat()
	assert.ErrorIs(t, err, os.ErrClosed)
}

func TestCheckNoActivity_ReadOnlyLegacyDiscovery(t *testing.T) {
	for _, depth := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			workspace := t.TempDir()
			path := workspace
			for _, name := range []string{activityDirectory,
				coordinationDirectory, operationsDirectory}[:depth] {
				path = filepath.Join(path, name)
				require.NoError(t, os.Mkdir(path, 0o700))
			}
			require.NoError(t, os.Chmod(path, 0o500))
			t.Cleanup(func() { assert.NoError(t, os.Chmod(path, 0o700)) })
			before, err := os.Stat(path)
			require.NoError(t, err)
			access := acquireForTest(t, workspace, Exclusive)
			assertActivityPaths(t, access)
			after, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, before.Mode(), after.Mode())
			assert.Equal(t, before.ModTime(), after.ModTime())
			entries, err := os.ReadDir(path)
			require.NoError(t, err)
			assert.Empty(t, entries, "preview cannot create missing coordination state")
		})
	}
}

func TestBeginActivity_InvalidOptionsDoNotWrite(t *testing.T) {
	for _, opts := range []ActivityOptions{
		{}, {Operation: "scan"},
		{Operation: "exec untrusted", StartedAt: activityOptions().StartedAt},
		{Operation: "scan", StartedAt: activityOptions().StartedAt, ProviderPIDs: []int{0}},
		{Operation: "scan", StartedAt: activityOptions().StartedAt, ProviderPIDs: []int{-1}},
		{Operation: "scan", StartedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(fmt.Sprint(opts), func(t *testing.T) {
			workspace := t.TempDir()
			access := acquireForTest(t, workspace, Shared)
			activity, err := access.BeginActivity(opts)
			assert.Nil(t, activity)
			assert.Error(t, err)
			// Encoding errors may create empty private directories, never a record.
			require.NoError(t, access.Close())
			assertActivityPaths(t, acquireForTest(t, workspace, Exclusive))
		})
	}
}

func TestBeginActivity_WriteAndSyncFailuresRetainRecords(t *testing.T) {
	failure := errors.New("injected activity persistence failure")
	for _, stage := range []string{"write", "sync", "replaced-directory", "replaced-record"} {
		t.Run(stage, func(t *testing.T) {
			workspace := t.TempDir()
			access := acquireForTest(t, workspace, Shared)
			var recordPath string
			activity, err := access.beginActivity(activityOptions(),
				func(file *os.File, contents []byte) error {
					recordPath = file.Name()
					switch stage {
					case "write":
						_, err := file.Write(contents[:1])
						return errors.Join(failure, err)
					case "sync":
						return errors.Join(failure, persistActivity(file, contents))
					case "replaced-directory":
						require.NoError(t, persistActivity(file, contents))
						parent := filepath.Dir(file.Name())
						require.NoError(t, os.Rename(parent, parent+"-moved"))
						return os.Mkdir(parent, 0o700)
					case "replaced-record":
						require.NoError(t, os.Remove(file.Name()))
						return os.WriteFile(file.Name(), contents, 0o600)
					}
					return nil
				})
			assert.Nil(t, activity, "no successful marker means no permission to start a provider")
			require.Error(t, err)
			if stage == "write" || stage == "sync" {
				assert.ErrorIs(t, err, failure)
				assert.Contains(t, err.Error(), recordPath)
			}
			require.NoError(t, access.Close())
			cleanup := acquireForTest(t, workspace, Exclusive)
			if stage == "replaced-directory" {
				// Namespace mutation is unsupported; the replacement is not touched.
				assert.FileExists(t, filepath.Join(filepath.Dir(recordPath)+"-moved",
					filepath.Base(recordPath)))
			} else {
				assertActivityPaths(t, cleanup, recordPath)
			}
		})
	}
}

func TestActivity_RejectsReplacements(t *testing.T) {
	for _, replacement := range []string{"record", "contents", "symlink", "operations",
		"coordination", "workspace"} {
		t.Run(replacement, func(t *testing.T) {
			workspace := filepath.Join(t.TempDir(), "workspace")
			require.NoError(t, os.Mkdir(workspace, 0o700))
			access := acquireForTest(t, workspace, Shared)
			activity := beginForTest(t, access)
			contents, err := os.ReadFile(activity.Path())
			require.NoError(t, err)
			retainedPath := activity.Path()
			switch replacement {
			case "record":
				require.NoError(t, os.Rename(activity.Path(), activity.Path()+"-original"))
				root, err := os.OpenRoot(filepath.Dir(activity.Path()))
				require.NoError(t, err)
				t.Cleanup(func() { assert.NoError(t, root.Close()) })
				require.NoError(t, root.WriteFile(filepath.Base(activity.Path()), contents, 0o600))
			case "contents":
				require.NoError(t, os.WriteFile(activity.Path(), []byte("changed"), 0o600))
			case "symlink":
				require.NoError(t, os.Rename(activity.Path(), activity.Path()+"-original"))
				require.NoError(t, os.Symlink(activity.Path()+"-original", activity.Path()))
			default:
				path := filepath.Dir(activity.Path())
				switch replacement {
				case "coordination":
					path = filepath.Dir(path)
				case "workspace":
					path = access.Path()
				}
				require.NoError(t, os.Rename(path, path+"-original"))
				require.NoError(t, os.Mkdir(path, 0o700))
				relative, err := filepath.Rel(path, activity.Path())
				require.NoError(t, err)
				retainedPath = filepath.Join(path+"-original", relative)
			}
			assert.Error(t, activity.CompleteNormal())
			assert.False(t, activity.completed)
			assert.FileExists(t, retainedPath, "records are never unlinked after replacement")
		})
	}
}

func TestActivity_RejectsUnsafeDirectoryComponents(t *testing.T) {
	for _, name := range []string{activityDirectory, coordinationDirectory, operationsDirectory} {
		for _, kind := range []string{"symlink", "file"} {
			t.Run(name+"-"+kind, func(t *testing.T) {
				workspace := t.TempDir()
				canonicalWorkspace, err := filepath.EvalSymlinks(workspace)
				require.NoError(t, err)
				target := t.TempDir()
				path := canonicalWorkspace
				for _, component := range []string{activityDirectory,
					coordinationDirectory, operationsDirectory} {
					path = filepath.Join(path, component)
					if component == name {
						break
					}
					require.NoError(t, os.Mkdir(path, 0o700))
				}
				if kind == "symlink" {
					require.NoError(t, os.Symlink(target, path))
				} else {
					require.NoError(t, os.WriteFile(path, []byte("untouched"), 0o600))
				}
				access := acquireForTest(t, workspace, Exclusive)
				activity, err := access.BeginActivity(activityOptions())
				assert.Nil(t, activity)
				assert.ErrorIs(t, err, ErrUnresolvedActivity)
				assertActivityPaths(t, access, path)
				entries, err := os.ReadDir(target)
				require.NoError(t, err)
				assert.Empty(t, entries, "never follow symlinked coordination paths")
			})
		}
	}
}

func TestCheckNoActivity_UntrustedEntriesAlwaysBlock(t *testing.T) {
	workspace := t.TempDir()
	access := acquireForTest(t, workspace, Exclusive)
	activity := beginForTest(t, access)
	require.NoError(t, activity.CompleteNormal())
	parent := filepath.Dir(activity.Path())
	outside := filepath.Join(t.TempDir(), "not-executed")
	paths := []string{filepath.Join(parent, "malformed.json"),
		filepath.Join(parent, "unreadable.json"), filepath.Join(parent, "directory"),
		filepath.Join(parent, "fifo"), filepath.Join(parent, "symlink"),
		filepath.Join(parent, "control\nrecord")}
	require.NoError(t, os.WriteFile(paths[0], []byte(`{"command":"touch `+outside+`"`), 0o600))
	require.NoError(t, os.WriteFile(paths[1], nil, 0))
	require.NoError(t, os.Mkdir(paths[2], 0o700))
	require.NoError(t, unix.Mkfifo(paths[3], 0o600))
	require.NoError(t, os.Symlink(outside, paths[4]))
	require.NoError(t, os.WriteFile(paths[5], []byte("unknown"), 0o600))
	before, err := os.Stat(parent)
	require.NoError(t, err)
	assertActivityPaths(t, access, paths...)
	assert.NotContains(t, access.CheckNoActivity().Error(), "control\nrecord")
	after, err := os.Stat(parent)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())
	assert.NoFileExists(t, outside)
	for _, path := range paths {
		_, err := os.Lstat(path)
		assert.NoError(t, err, "unsafe entries must be retained")
	}
}

func TestActivity_UnreadableDirectoriesBlock(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permission restrictions")
	}
	access := acquireForTest(t, t.TempDir(), Exclusive)
	activity := beginForTest(t, access)
	parent := filepath.Dir(activity.Path())
	require.NoError(t, os.Chmod(parent, 0))
	t.Cleanup(func() { assert.NoError(t, os.Chmod(parent, 0o700)) })
	assertActivityPaths(t, access, parent)
	assert.ErrorIs(t, access.CheckNoActivity(), os.ErrPermission)
	assert.ErrorIs(t, activity.CompleteNormal(), os.ErrPermission)
	op, err := access.BeginActivity(activityOptions())
	assert.Nil(t, op)
	assert.ErrorIs(t, err, os.ErrPermission)
	require.NoError(t, os.Chmod(parent, 0o700))
	assert.FileExists(t, activity.Path())
}

// TestActivity_HelperProcess announces readiness only after marker persistence.
// stdin supplies a normal-completion or uncertainty barrier, never a sleep.
func TestActivity_HelperProcess(t *testing.T) {
	workspace := os.Getenv(activityHelperEnvironment)
	if workspace == "" {
		return
	}
	access, err := Acquire(Options{Workspace: workspace, Mode: Shared})
	require.NoError(t, err)
	activity := beginForTest(t, access)
	provider, finishProvider := startActivityProvider(t, activity.Path())
	require.NoError(t, json.NewEncoder(os.Stdout).Encode(activity.Path()))
	var outcome string
	require.NoError(t, json.NewDecoder(os.Stdin).Decode(&outcome))
	switch outcome {
	case "normal", "handled-error":
		finishProvider(false)
		require.NoError(t, activity.CompleteNormal())
	case "uncertain":
		// The operation completion signal was lost, even if teardown later exits.
		finishProvider(false)
	case "force-killed-provider":
		finishProvider(true)
		assert.False(t, provider.ProcessState.Success())
	default:
		t.Fatalf("unknown test outcome %q", outcome)
	}
	require.NoError(t, access.Close())
}

// TestActivity_ProviderHelperProcess models unchanged provider work behind a
// pipe barrier. It cannot announce startup until a complete record exists. A
// killed host closes the pipe, so this controlled child cannot remain a writer.
func TestActivity_ProviderHelperProcess(t *testing.T) {
	path := os.Getenv(providerHelperEnvironment)
	if path == "" {
		return
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, root.Close()) })
	contents, err := root.ReadFile(filepath.Base(path))
	require.NoError(t, err)
	var record activityRecord
	require.NoError(t, json.Unmarshal(contents, &record))
	require.Equal(t, activityVersion, record.Version)
	require.Equal(t, os.Getppid(), record.HostPID)
	require.NoError(t, json.NewEncoder(os.Stdout).Encode("provider-ready"))
	_, err = io.Copy(io.Discard, os.Stdin)
	require.NoError(t, err)
}

func startActivityProvider(t *testing.T, recordPath string) (*exec.Cmd, func(bool)) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.Command(executable, "-test.run=^TestActivity_ProviderHelperProcess$")
	cmd.Env = append(os.Environ(), providerHelperEnvironment+"="+recordPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	input, err := cmd.StdinPipe()
	require.NoError(t, err)
	output, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	var once sync.Once
	finish := func(kill bool) {
		once.Do(func() {
			if kill {
				require.NoError(t, cmd.Process.Kill())
			}
			require.NoError(t, input.Close())
			err := cmd.Wait()
			if kill {
				var exit *exec.ExitError
				require.ErrorAs(t, err, &exit)
			} else {
				require.NoError(t, err, "provider stderr: %s", stderr.String())
			}
		})
	}
	t.Cleanup(func() { finish(true) })
	var ready string
	require.NoError(t, json.NewDecoder(output).Decode(&ready))
	require.Equal(t, "provider-ready", ready)
	return cmd, finish
}

func startActivityProcess(t *testing.T, workspace string) (string, func(string)) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), helperTimeout)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestActivity_HelperProcess$")
	cmd.Env = append(os.Environ(), activityHelperEnvironment+"="+workspace)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	input, err := cmd.StdinPipe()
	require.NoError(t, err)
	output, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	var once sync.Once
	finish := func(outcome string) {
		once.Do(func() {
			if outcome == "killed-host" {
				assert.NoError(t, cmd.Process.Kill())
			} else {
				assert.NoError(t, json.NewEncoder(input).Encode(outcome))
			}
			assert.NoError(t, input.Close())
			err := cmd.Wait()
			if outcome == "killed-host" {
				var exit *exec.ExitError
				require.ErrorAs(t, err, &exit)
				assert.False(t, exit.Success())
			} else {
				assert.NoError(t, err, "child stderr: %s", stderr.String())
			}
			assert.NoError(t, ctx.Err())
		})
	}
	t.Cleanup(func() { finish("uncertain") })
	var path string
	require.NoError(t, json.NewDecoder(output).Decode(&path))
	return path, finish
}

func TestActivity_SubprocessCompletionBoundary(t *testing.T) {
	for _, outcome := range []string{"normal", "handled-error", "uncertain",
		"force-killed-provider", "killed-host"} {
		t.Run(outcome, func(t *testing.T) {
			workspace := t.TempDir()
			path, finish := startActivityProcess(t, workspace)
			assert.FileExists(t, path, "provider readiness barrier follows marker persistence")
			access, err := Acquire(Options{Workspace: workspace, Mode: Exclusive})
			assert.Nil(t, access)
			assert.ErrorIs(t, err, ErrBusy)
			finish(outcome)
			cleanup := acquireForTest(t, workspace, Exclusive)
			if outcome == "normal" || outcome == "handled-error" {
				assert.NoFileExists(t, path)
				assertActivityPaths(t, cleanup)
			} else {
				assertActivityPaths(t, cleanup, path)
			}
		})
	}
}

func TestActivity_ManualRemovalCannotBypassLiveLock(t *testing.T) {
	workspace := t.TempDir()
	path, finish := startActivityProcess(t, workspace)
	require.NoError(t, os.Remove(path))
	access, err := Acquire(Options{Workspace: workspace, Mode: Exclusive})
	assert.Nil(t, access)
	assert.ErrorIs(t, err, ErrBusy)
	finish("uncertain")
	assertActivityPaths(t, acquireForTest(t, workspace, Exclusive))
}

func TestActivity_ConcurrentHostsOwnOnlyTheirRecords(t *testing.T) {
	workspace := t.TempDir()
	first, finishFirst := startActivityProcess(t, workspace)
	second, finishSecond := startActivityProcess(t, workspace)
	assert.NotEqual(t, first, second)
	finishFirst("normal")
	assert.NoFileExists(t, first)
	assert.FileExists(t, second)
	finishSecond("uncertain")
	assertActivityPaths(t, acquireForTest(t, workspace, Exclusive), second)
}

func TestActivity_DiscoveryRequiresLiveExclusiveAccess(t *testing.T) {
	access := acquireForTest(t, t.TempDir(), Shared)
	assert.ErrorContains(t, access.CheckNoActivity(), "requires exclusive")
	require.NoError(t, access.Close())
	assert.ErrorIs(t, access.CheckNoActivity(), os.ErrClosed)
	op, err := access.BeginActivity(activityOptions())
	assert.Nil(t, op)
	assert.ErrorIs(t, err, os.ErrClosed)
}

func TestCheckNoActivity_MissingWorkspaceIsNotLegacyState(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	require.NoError(t, os.Mkdir(workspace, 0o700))
	access := acquireForTest(t, workspace, Exclusive)
	require.NoError(t, os.Remove(workspace))
	assert.ErrorIs(t, access.CheckNoActivity(), ErrWorkspaceChanged)
}

func TestPersistActivity_ClosedFile(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "record"))
	require.NoError(t, err)
	require.NoError(t, file.Close())
	assert.ErrorIs(t, persistActivity(file, []byte("record")), os.ErrClosed)
}
