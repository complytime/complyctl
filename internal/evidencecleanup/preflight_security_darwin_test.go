// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/ebitengine/purego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestLoadDarwinSecurity_InitializationFailures(t *testing.T) {
	openFailure := errors.New("open failure")
	lookupFailure := errors.New("lookup failure")
	closeFailure := errors.New("close failure")
	for _, failure := range []string{"open", "fgetattrlist", "null symbol"} {
		t.Run(failure, func(t *testing.T) {
			var closed bool
			var symbols []string
			api, err := loadDarwinSecurity(
				func(path string, flags int) (uintptr, error) {
					assert.Equal(t, darwinSystemLibrary, path)
					assert.Equal(t, purego.RTLD_NOW|purego.RTLD_LOCAL, flags)
					if failure == "open" {
						return 0, openFailure
					}
					return 7, nil
				},
				func(handle uintptr, name string) (uintptr, error) {
					assert.EqualValues(t, 7, handle)
					symbols = append(symbols, name)
					if failure == name {
						return 0, lookupFailure
					}
					// No fabricated address may reach RegisterFunc.
					return 0, nil
				},
				func(handle uintptr) error {
					assert.EqualValues(t, 7, handle)
					closed = true
					return closeFailure
				})
			require.Error(t, err)
			assert.Nil(t, api.read)
			if failure == "open" {
				assert.ErrorIs(t, err, openFailure)
				assert.False(t, closed)
				assert.Empty(t, symbols)
			} else {
				assert.True(t, closed)
				assert.ErrorIs(t, err, closeFailure)
				if failure == "null symbol" {
					assert.ErrorContains(t, err, "null symbol")
				} else {
					assert.ErrorIs(t, err, lookupFailure)
				}
			}
		})
	}
}

func TestDarwinSecurityAPI_OpenedDescriptorAndErrno(t *testing.T) {
	path := filepath.Join(t.TempDir(), "member")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	removeDarwinTestACL(t, path)
	file, err := os.Open(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()
	fd, err := discoveryFD(file)
	require.NoError(t, err)
	require.NoError(t, os.Rename(path, path+"-original"))
	require.NoError(t, os.WriteFile(path, []byte("replacement"), 0o600))
	installDarwinTestACL(t, path)
	api, err := darwinPreflightSecurity()
	require.NoError(t, err)
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			runtime.GC()
			buffer, err := api.readSecurity(fd)
			assert.NoError(t, err)
			assert.NoError(t, checkDarwinSecurityBuffer(buffer), "must inspect opened original")
			_, err = api.readSecurity(math.MaxInt32)
			assert.ErrorIs(t, err, unix.EBADF, "errno must belong to this call's thread")
		})
	}
	workers.Wait()
	replacement, err := os.Open(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, replacement.Close()) }()
	replacementFD, err := discoveryFD(replacement)
	require.NoError(t, err)
	buffer, err := api.readSecurity(replacementFD)
	require.NoError(t, err)
	require.ErrorContains(t, checkDarwinSecurityBuffer(buffer), "ACL requires retention")
}

func TestDarwinSecurityAPI_CapturedErrnoSurvivesLaterForeignCall(t *testing.T) {
	api, err := darwinPreflightSecurity()
	require.NoError(t, err)
	read := api.read
	api.read = func(fd int32, buffer *[darwinSecurityBufferSize]byte) (int32, int32) {
		result, errno := read(fd, buffer)
		// Another foreign call on the same thread must not affect the errno
		// already returned by the binding. This deterministically exercises
		// the boundary that the concurrent GC stress test also covers.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		_, laterErr := unix.Open("", unix.O_RDONLY, 0)
		require.ErrorIs(t, laterErr, unix.ENOENT)
		return result, errno
	}
	_, err = api.readSecurity(math.MaxInt32)
	require.ErrorIs(t, err, unix.EBADF)
}

func TestLoadDarwinSecurity_RetainsLibraryForBoundCalls(t *testing.T) {
	var handle uintptr
	api, err := loadDarwinSecurity(
		func(path string, flags int) (uintptr, error) {
			var err error
			handle, err = purego.Dlopen(path, flags)
			return handle, err
		}, purego.Dlsym, func(uintptr) error {
			t.Error("successful binding must retain its library reference")
			return nil
		})
	require.NoError(t, err)
	// Unlike the process-lifetime cache, this test owns no escaping functions.
	defer func() { require.NoError(t, purego.Dlclose(handle)) }()
	_, err = api.readSecurity(math.MaxInt32)
	require.ErrorIs(t, err, unix.EBADF)
}

func TestPreflightGroups_DarwinACLRetainsWholeGroup(t *testing.T) {
	for _, member := range []string{"evidence", "parent", "report"} {
		t.Run(member, func(t *testing.T) {
			workspace := t.TempDir()
			old := addGroupEvaluation(t, workspace, "ampel", "old", groupOldStart)
			shared := addGroupEvaluation(t, workspace, "opa", "shared", groupOldStart)
			report := addGroupReport(t, workspace, "a", filepath.Join(old, "first.json"),
				filepath.Join(shared, "first.json"))
			path := filepath.Join(workspace, old, "first.json")
			switch member {
			case "parent":
				path = filepath.Dir(filepath.Join(workspace, old))
			case "report":
				path = filepath.Join(workspace, report)
			}
			installDarwinTestACL(t, path)
			before := groupSnapshot(t, workspace)
			result, err := PreflightGroups(GroupOptions{Workspace: workspace, Age: groupAge(t)})
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			group := result.Groups[0]
			assert.False(t, group.Eligible())
			require.NotEmpty(t, group.Blockers)
			assert.Contains(t, group.Blockers[0].Reason, "ACL requires retention")
			assert.Len(t, group.Evaluations, 2)
			assert.Len(t, group.Reports, 1)
			assert.Equal(t, before, groupSnapshot(t, workspace))
		})
	}
}

func removeDarwinTestACL(t *testing.T, path string) {
	t.Helper()
	output, err := exec.Command("/bin/chmod", "-N", path).CombinedOutput()
	require.NoError(t, err, "%s", output)
}

func installDarwinTestACL(t *testing.T, path string) {
	t.Helper()
	removeDarwinTestACL(t, path)
	output, err := exec.Command("/bin/chmod", "+a", "everyone deny delete", path).CombinedOutput()
	require.NoError(t, err, "%s", output)
	t.Cleanup(func() { removeDarwinTestACL(t, path) })
}
