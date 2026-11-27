// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func removalPlatformGuard(workspace *os.File) (func() error, error) {
	return newRemovalLinuxGuard(workspace, removalMountID, probeRemovalLinux)
}

func newRemovalLinuxGuard(workspace *os.File, mountID func(*os.File) (uint64, error),
	probeResolver func(*os.File) error) (func() error, error) {
	identity, err := workspace.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect removal workspace: %w", err)
	}
	mount, err := mountID(workspace)
	if err != nil {
		return nil, err
	}
	// Probe the required resolver even for an empty candidate. Never downgrade
	// to openat when openat2 or mount-instance identity is unavailable.
	if err := probeResolver(workspace); err != nil {
		return nil, err
	}
	return func() error {
		if err := validateDiscoveryWorkspace(workspace.Name(), identity); err != nil {
			return err
		}
		current, err := mountID(workspace)
		if err != nil {
			return err
		}
		if current != mount {
			return fmt.Errorf("removal workspace mount instance changed")
		}
		return nil
	}, nil
}

func probeRemovalLinux(workspace *os.File) error {
	probe, err := openDiscoveryDirectory(workspace, ".")
	if err != nil {
		return err
	}
	return probe.Close()
}

func removalMountID(file *os.File) (uint64, error) {
	fd, err := discoveryFD(file)
	if err != nil {
		return 0, err
	}
	var stat unix.Statx_t
	err = unix.Statx(fd, "", unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW, unix.STATX_MNT_ID, &stat)
	runtime.KeepAlive(file)
	if err != nil {
		return 0, fmt.Errorf("inspect removal mount instance: %w", err)
	}
	if stat.Mask&unix.STATX_MNT_ID == 0 || stat.Mnt_id == 0 {
		return 0, fmt.Errorf("removal mount-instance identity unavailable")
	}
	return stat.Mnt_id, nil
}
