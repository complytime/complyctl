// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func openDiscoveryWorkspace(path string) (*os.File, func() error, error) {
	if err := checkDiscoveryMounts(path); err != nil {
		return nil, nil, err
	}
	file, validate, err := openPinnedDiscoveryWorkspace(path)
	if err != nil {
		return nil, nil, err
	}
	return file, func() error {
		if err := validate(); err != nil {
			return err
		}
		return checkDiscoveryMounts(path)
	}, nil
}

func openDiscoveryDirectory(parent *os.File, path string) (*os.File, error) {
	return walkDiscoveryDirectory(parent, path, openDiscoveryComponent)
}

func openDiscoveryComponent(parent *os.File, name string) (*os.File, error) {
	parentFD, err := discoveryFD(parent)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Openat(parentFD, name,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	runtime.KeepAlive(parent)
	if err != nil {
		return nil, fmt.Errorf("confined directory traversal unavailable or unsafe: %w", err)
	}
	child, err := discoveryFile(fd, name)
	if err != nil {
		return nil, err
	}
	err = checkDiscoveryFilesystem(parentFD, fd, name)
	runtime.KeepAlive(parent)
	runtime.KeepAlive(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return child, nil
}

func checkDiscoveryFilesystem(parentFD, childFD int, name string) error {
	var parentFS, childFS unix.Statfs_t
	if err := unix.Fstatfs(parentFD, &parentFS); err != nil {
		return fmt.Errorf("inspect discovery parent filesystem: %w", err)
	}
	if err := unix.Fstatfs(childFD, &childFS); err != nil {
		return fmt.Errorf("inspect discovery child filesystem: %w", err)
	}
	if parentFS.Fsid != childFS.Fsid {
		return fmt.Errorf("discovery crosses a filesystem boundary at %q", name)
	}
	return nil
}

// A mount table check also excludes same-filesystem nested mounts, which
// fstatfs identity equality cannot detect. Refuse an ambiguous/changing table.
// Administrative namespace changes during discovery remain unsupported.
func checkDiscoveryMounts(workspace string) error {
	mounts, err := readDiscoveryMounts()
	if err != nil {
		return err
	}
	for _, mount := range mounts {
		if err := validateDiscoveryMount(workspace, unix.ByteSliceToString(mount.Mntonname[:])); err != nil {
			return err
		}
	}
	return nil
}

func discoveryMountBuffer() ([]unix.Statfs_t, error) {
	count, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, fmt.Errorf("inspect discovery mount table: %w", err)
	}
	if count < 1 {
		return nil, fmt.Errorf("empty discovery mount table")
	}
	return make([]unix.Statfs_t, count+1), nil
}

func readDiscoveryMounts() ([]unix.Statfs_t, error) {
	mounts, err := discoveryMountBuffer()
	if err != nil {
		return nil, err
	}
	count, err := unix.Getfsstat(mounts, unix.MNT_NOWAIT)
	if err != nil {
		return nil, fmt.Errorf("read discovery mount table: %w", err)
	}
	if count < 1 || count >= len(mounts) {
		return nil, fmt.Errorf("discovery mount table empty or changed during enumeration")
	}
	return mounts[:count], nil
}
