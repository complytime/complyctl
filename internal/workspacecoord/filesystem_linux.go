// SPDX-License-Identifier: Apache-2.0

package workspacecoord

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func validateFilesystem(filesystem unix.Statfs_t) error {
	switch filesystem.Type {
	case unix.EXT4_SUPER_MAGIC, unix.XFS_SUPER_MAGIC, unix.BTRFS_SUPER_MAGIC, unix.TMPFS_MAGIC:
		return nil
	default:
		return fmt.Errorf("%w: Linux type %#x; use local ext4, XFS, Btrfs, or tmpfs",
			ErrUnsupportedFilesystem, filesystem.Type)
	}
}
