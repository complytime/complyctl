// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func removalTestDarwinMount(path, source string, id int32) unix.Statfs_t {
	var mount unix.Statfs_t
	copy(mount.Mntonname[:], path)
	copy(mount.Mntfromname[:], source)
	mount.Fsid.Val[0] = id
	return mount
}

func TestRemovalDarwin_MountLayoutValidation(t *testing.T) {
	workspace, unit, file, _ := removalFixture(t)
	identity, err := file.Stat()
	require.NoError(t, err)
	// Model the distinct System and Data volumes of APFS without assuming that
	// the workspace's canonical path has the mount point as a lexical prefix.
	system := removalTestDarwinMount("/", "system-snapshot", 1)
	data := removalTestDarwinMount("/System/Volumes/Data", "data-volume", 2)
	for _, scenario := range []string{"normal-apfs", "nested", "same-fsid", "duplicate-path",
		"missing-opened", "missing-identity", "relative", "unclean", "alias"} {
		t.Run(scenario, func(t *testing.T) {
			mounts := []unix.Statfs_t{system, data}
			switch scenario {
			case "nested":
				mounts = append(mounts, removalTestDarwinMount(filepath.Join(file.Name(), unit), "nested", 3))
			case "same-fsid":
				mounts = append(mounts, removalTestDarwinMount("/Volumes/alias", "data-volume", 2))
			case "duplicate-path":
				mounts = append(mounts, removalTestDarwinMount("/", "another-root", 3))
			case "missing-opened":
				mounts = mounts[:1]
			case "missing-identity":
				mounts = append(mounts, removalTestDarwinMount("/Volumes/unknown", "unknown", 0))
			case "relative":
				mounts = append(mounts, removalTestDarwinMount("relative", "unknown", 3))
			case "unclean":
				mounts = append(mounts, removalTestDarwinMount("/Volumes/../ambiguous", "unknown", 3))
			case "alias":
				alias := filepath.Join(t.TempDir(), "workspace-alias")
				require.NoError(t, os.Symlink(workspace, alias))
				mounts = append(mounts, removalTestDarwinMount(filepath.Join(alias, unit), "aliased", 3))
			}
			result, err := validateRemovalDarwinMounts(file.Name(), identity, data, mounts)
			if scenario == "normal-apfs" {
				require.NoError(t, err)
				require.Len(t, result, 2)
				assert.Equal(t, "/", result[0].path)
				assert.Equal(t, data.Fsid, result[1].fsid)
				assert.Equal(t, "data-volume", result[1].source)
				return
			}
			require.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func TestRemovalDarwin_UnavailableOrChangedMountTablePreservesMember(t *testing.T) {
	for _, fault := range []string{"prepare", "revalidate", "identity", "flags"} {
		t.Run(fault, func(t *testing.T) {
			workspace, unit, file, identity := removalFixture(t)
			removalOutsideSentinel(t)
			before := groupSnapshot(t, workspace)
			calls := 0
			guard, err := newRemovalDarwinGuard(file,
				func(opened *os.File, info os.FileInfo) ([]removalDarwinMount, error) {
					assert.Same(t, file, opened)
					assert.True(t, info.IsDir())
					calls++
					if fault == "prepare" || (fault == "revalidate" && calls > 1) {
						return nil, unix.ENOSYS
					}
					mount := removalDarwinMount{path: "/", source: "original", fsid: unix.Fsid{Val: [2]int32{1, 0}}}
					if calls > 1 {
						if fault == "identity" {
							mount.source = "replacement"
						} else {
							mount.flags = unix.MNT_RDONLY
						}
					}
					return []removalDarwinMount{mount}, nil
				})
			if fault == "prepare" {
				require.ErrorIs(t, err, unix.ENOSYS)
				assert.Nil(t, guard)
			} else {
				require.NoError(t, err)
				r, err := prepareRemoval(file, unit, identity)
				require.NoError(t, err)
				defer func() { require.NoError(t, r.close()) }()
				r.guard = guard
				require.Error(t, r.remove())
				assert.Equal(t, 2, calls)
			}
			assert.Equal(t, before, groupSnapshot(t, workspace))
		})
	}
}

func TestRemovalDarwin_APFSDescriptorDeletion(t *testing.T) {
	workspace, unit, file, identity := removalFixture(t)
	fd, err := discoveryFD(file)
	require.NoError(t, err)
	var fs unix.Statfs_t
	require.NoError(t, unix.Fstatfs(fd, &fs))
	require.Equal(t, "apfs", unix.ByteSliceToString(fs.Fstypename[:]),
		"native task 3.1 validation requires an APFS fixture volume")
	removalOutsideSentinel(t)
	r, err := prepareRemoval(file, unit, identity)
	require.NoError(t, err)
	defer func() { require.NoError(t, r.close()) }()
	require.NoError(t, r.remove())
	assert.NoDirExists(t, filepath.Join(workspace, unit))
	assert.DirExists(t, filepath.Dir(filepath.Join(workspace, unit)))
}
