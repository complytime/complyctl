// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func openInventoryRegular(parent *os.File, name string) (*os.File, error) {
	fd, parentFD, err := openInventoryDarwinFD(parent, name)
	if err != nil {
		return nil, err
	}
	file, err := validateInventoryRegular(fd, name)
	if err != nil {
		return nil, err
	}
	err = checkDiscoveryFilesystem(parentFD, fd, name)
	runtime.KeepAlive(parent)
	runtime.KeepAlive(file)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func openInventoryDarwinFD(parent *os.File, name string) (int, int, error) {
	if err := validateDiscoveryComponent(name); err != nil {
		return -1, -1, err
	}
	parentFD, err := discoveryFD(parent)
	if err != nil {
		return -1, -1, err
	}
	fd, err := unix.Openat(parentFD, name,
		unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	runtime.KeepAlive(parent)
	if err != nil {
		return -1, -1, fmt.Errorf("confined regular-file traversal: %w", err)
	}
	return fd, parentFD, nil
}
