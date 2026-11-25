// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func openInventoryRegular(parent *os.File, name string) (*os.File, error) {
	parentFD, err := discoveryFD(parent)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Openat2(parentFD, name, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_NONBLOCK | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_XDEV,
	})
	runtime.KeepAlive(parent)
	if err != nil {
		return nil, fmt.Errorf("confined regular-file traversal: %w", err)
	}
	return validateInventoryRegular(fd, name)
}
