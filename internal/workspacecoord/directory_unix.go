// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package workspacecoord

import (
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func openDirectory(path string) (*os.File, error) {
	// Read-only directory access permits preview without write permission. CLOEXEC
	// keeps provider subprocesses from accidentally extending the host lock lifetime.
	return os.OpenFile(path, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
}

func lockDirectory(file *os.File, mode Mode) error {
	fd, err := directoryFD(file)
	if err != nil {
		return fmt.Errorf("get directory lock descriptor: %w", err)
	}
	operation := unix.LOCK_SH
	conflict := "an exclusive workspace operation is active"
	if mode == Exclusive {
		operation = unix.LOCK_EX
		conflict = "another shared or exclusive workspace operation is active"
	}
	err = unix.Flock(fd, operation|unix.LOCK_NB)
	// Unix calls receive only an integer, so keep the owning file alive until return.
	runtime.KeepAlive(file)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return fmt.Errorf("%w: %s; retry after it finishes", ErrBusy, conflict)
	}
	if err != nil {
		return fmt.Errorf("nonblocking directory locking unavailable: %w", err)
	}
	return nil
}

func directoryFD(file *os.File) (int, error) {
	fd := file.Fd()
	// Fd returns an invalid uintptr for a closed file. Check the range before
	// converting to the signed descriptor type required by the Unix APIs.
	if fd > math.MaxInt {
		return -1, unix.EBADF
	}
	return int(fd), nil
}

func checkFilesystem(file *os.File) error {
	fd, err := directoryFD(file)
	if err != nil {
		return fmt.Errorf("get directory filesystem descriptor: %w", err)
	}
	var filesystem unix.Statfs_t
	err = unix.Fstatfs(fd, &filesystem)
	runtime.KeepAlive(file)
	if err != nil {
		return fmt.Errorf("inspect directory filesystem: %w", err)
	}
	return validateFilesystem(filesystem)
}
