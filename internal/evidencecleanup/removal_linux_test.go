// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

const removalMountHelper = "COMPLYCTL_TEST_REMOVAL_MOUNT_HELPER"

func TestRemovalLinux_NestedMounts(t *testing.T) {
	if os.Getenv(removalMountHelper) == "1" {
		runRemovalMountHelper(t)
		return
	}
	for _, kind := range []string{"bind", "tmpfs"} {
		for _, when := range []string{"prepare", "held"} {
			for _, target := range []string{"root", "unit", "nested"} {
				t.Run(kind+"/"+when+"/"+target, func(t *testing.T) {
					exe, err := os.Executable()
					require.NoError(t, err)
					cmd := exec.Command(exe, "-test.run=^TestRemovalLinux_NestedMounts$", "-test.v")
					cmd.Env = append(os.Environ(), removalMountHelper+"=1",
						"REMOVAL_KIND="+kind,
						"REMOVAL_WHEN="+when, "REMOVAL_TARGET="+target)
					// A private mount namespace ensures even test failure cannot
					// leave mounts in the developer's namespace. No capability skip.
					cmd.SysProcAttr = &syscall.SysProcAttr{
						Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
						UidMappings: []syscall.SysProcIDMap{{ContainerID: 0,
							HostID: os.Getuid(), Size: 1}},
						GidMappingsEnableSetgroups: false,
						GidMappings: []syscall.SysProcIDMap{{ContainerID: 0,
							HostID: os.Getgid(), Size: 1}},
					}
					output, err := cmd.CombinedOutput()
					require.NoError(t, err, "native mount test requires user/mount namespaces: %s", output)
					assert.Contains(t, string(output), "PASS")
				})
			}
		}
	}
}

func runRemovalMountHelper(t *testing.T) {
	t.Helper()
	// Create fixtures inside this private namespace rather than accepting deletion
	// paths from the environment. Only the bounded scenario selectors cross it.
	workspace, unit, file, identity := removalFixture(t)
	source := removalOutsideSentinel(t)
	beforeWorkspace := groupSnapshot(t, workspace)
	var err error
	var r *removal
	if os.Getenv("REMOVAL_WHEN") == "held" {
		r, err = prepareRemoval(file, unit, identity)
		require.NoError(t, err)
		defer func() { require.NoError(t, r.close()) }()
	}
	target := filepath.Join(workspace, unit)
	switch os.Getenv("REMOVAL_TARGET") {
	case "root":
		target = filepath.Join(workspace, managedDirectory, "ampel", resultsDirectory)
	case "nested":
		target = filepath.Join(target, "nested")
	}
	if os.Getenv("REMOVAL_KIND") == "bind" {
		require.NoError(t, unix.Mount(source, target, "", unix.MS_BIND, ""))
	} else {
		require.NoError(t, unix.Mount("tmpfs", target, "tmpfs", 0, "size=1m"))
		require.NoError(t, os.WriteFile(filepath.Join(target, "sentinel"), []byte("mounted bytes"), 0o600))
	}
	defer func() {
		require.NoError(t, unix.Unmount(target, 0))
		assert.Equal(t, beforeWorkspace, groupSnapshot(t, workspace))
	}()
	before := groupSnapshot(t, target)
	if r == nil {
		r, err = prepareRemoval(file, unit, identity)
		require.Error(t, err)
		assert.Nil(t, r)
	} else {
		require.Error(t, r.remove())
	}
	assert.Equal(t, before, groupSnapshot(t, target))
	data, err := os.ReadFile(filepath.Join(source, "sentinel"))
	require.NoError(t, err)
	assert.Equal(t, "independent outside bytes", string(data))
}

func TestRemovalLinux_UnavailableFacilities(t *testing.T) {
	workspace, _, file, _ := removalFixture(t)
	removalOutsideSentinel(t)
	before := groupSnapshot(t, workspace)
	for _, unavailable := range []string{"mount-id", "openat2"} {
		t.Run(unavailable, func(t *testing.T) {
			mountID := removalMountID
			probe := probeRemovalLinux
			if unavailable == "mount-id" {
				mountID = func(*os.File) (uint64, error) { return 0, unix.ENOSYS }
			} else {
				probe = func(*os.File) error { return unix.ENOSYS }
			}
			guard, err := newRemovalLinuxGuard(file, mountID, probe)
			require.ErrorIs(t, err, unix.ENOSYS)
			assert.Nil(t, guard)
		})
	}
	assert.Equal(t, before, groupSnapshot(t, workspace))
}
