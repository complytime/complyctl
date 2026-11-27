// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func openDiscoveryWorkspace(path string) (*os.File, func() error, error) {
	return openPinnedDiscoveryWorkspace(path)
}

func openDiscoveryDirectory(parent *os.File, path string) (*os.File, error) {
	parentFD, err := discoveryFD(parent)
	if err != nil {
		return nil, err
	}
	// openat2 enforces mount-instance boundaries (including same-device bind
	// mounts). Per Simplicity & Isolation, do not replace it with racy path checks
	// or a device-ID-only fallback when the kernel cannot provide confinement.
	fd, err := unix.Openat2(parentFD, path, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_XDEV,
	})
	runtime.KeepAlive(parent)
	if err != nil {
		return nil, fmt.Errorf("confined directory traversal unavailable or unsafe: %w", err)
	}
	return discoveryFile(fd, path)
}
