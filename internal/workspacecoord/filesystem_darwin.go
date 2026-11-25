// SPDX-License-Identifier: Apache-2.0

package workspacecoord

import (
	"fmt"

	"golang.org/x/sys/unix"
)

const apfsFilesystem = "apfs"

func validateFilesystem(filesystem unix.Statfs_t) error {
	name := unix.ByteSliceToString(filesystem.Fstypename[:])
	if name != apfsFilesystem || filesystem.Flags&unix.MNT_LOCAL == 0 {
		return fmt.Errorf("%w: Darwin type %q (flags %#x); use local APFS",
			ErrUnsupportedFilesystem, name, filesystem.Flags)
	}
	return nil
}
