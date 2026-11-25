// SPDX-License-Identifier: Apache-2.0

package workspacecoord

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestValidateFilesystem_LinuxMatrix(t *testing.T) {
	for _, test := range []struct {
		name      string
		typeID    int64
		supported bool
	}{
		{"ext4", unix.EXT4_SUPER_MAGIC, true},
		{"xfs", unix.XFS_SUPER_MAGIC, true},
		{"btrfs", unix.BTRFS_SUPER_MAGIC, true},
		{"tmpfs", unix.TMPFS_MAGIC, true},
		{"overlay", unix.OVERLAYFS_SUPER_MAGIC, false},
		{"fuse", unix.FUSE_SUPER_MAGIC, false},
		{"nfs", unix.NFS_SUPER_MAGIC, false},
		{"cifs", unix.CIFS_SUPER_MAGIC, false},
		{"unknown", 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateFilesystem(unix.Statfs_t{Type: test.typeID})
			if test.supported {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ErrUnsupportedFilesystem)
				assert.ErrorContains(t, err, "use local ext4, XFS, Btrfs, or tmpfs")
			}
		})
	}
}

func TestCheckCleanupSupport_LinuxNativeFilesystem(t *testing.T) {
	access := acquireForTest(t, t.TempDir(), Exclusive)
	fd, err := directoryFD(access.file)
	require.NoError(t, err)
	var filesystem unix.Statfs_t
	require.NoError(t, unix.Fstatfs(fd, &filesystem))
	t.Logf("native filesystem type: %#x", filesystem.Type)
	err = access.CheckCleanupSupport()
	switch filesystem.Type {
	case unix.EXT4_SUPER_MAGIC, unix.XFS_SUPER_MAGIC, unix.BTRFS_SUPER_MAGIC, unix.TMPFS_MAGIC:
		assert.NoError(t, err)
	default:
		assert.ErrorIs(t, err, ErrUnsupportedFilesystem)
		assert.ErrorContains(t, err, access.Path())
	}
}
