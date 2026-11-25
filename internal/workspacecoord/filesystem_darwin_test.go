// SPDX-License-Identifier: Apache-2.0

package workspacecoord

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestValidateFilesystem_DarwinMatrix(t *testing.T) {
	for _, test := range []struct {
		name      string
		local     bool
		supported bool
	}{
		{"apfs", true, true},
		{"apfs", false, false},
		{"hfs", true, false},
		{"nfs", false, false},
		{"smbfs", false, false},
		{"osxfuse", true, false},
		{"", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var filesystem unix.Statfs_t
			copy(filesystem.Fstypename[:], test.name)
			if test.local {
				filesystem.Flags = unix.MNT_LOCAL
			}
			err := validateFilesystem(filesystem)
			if test.supported {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ErrUnsupportedFilesystem)
				assert.ErrorContains(t, err, "use local APFS")
			}
		})
	}
}

func TestCheckCleanupSupport_DarwinNativeFilesystem(t *testing.T) {
	access := acquireForTest(t, t.TempDir(), Exclusive)
	fd, err := directoryFD(access.file)
	require.NoError(t, err)
	var filesystem unix.Statfs_t
	require.NoError(t, unix.Fstatfs(fd, &filesystem))
	name := unix.ByteSliceToString(filesystem.Fstypename[:])
	t.Logf("native filesystem type: %q, flags: %#x", name, filesystem.Flags)
	err = access.CheckCleanupSupport()
	if name == "apfs" && filesystem.Flags&unix.MNT_LOCAL != 0 {
		assert.NoError(t, err)
	} else {
		assert.ErrorIs(t, err, ErrUnsupportedFilesystem)
		assert.ErrorContains(t, err, access.Path())
	}
}
