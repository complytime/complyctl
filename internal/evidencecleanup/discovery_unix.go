// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

// Share identity pinning across backends per Simplicity & Isolation; mount
// confinement remains platform-specific rather than using path checks as a fallback.
func openPinnedDiscoveryWorkspace(path string) (*os.File, func() error, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	identity, err := file.Stat()
	if err != nil {
		return nil, nil, errors.Join(err, file.Close())
	}
	return file, func() error { return validateDiscoveryWorkspace(path, identity) }, nil
}

func validateDiscoveryWorkspace(path string, identity os.FileInfo) error {
	current, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("reinspect workspace directory: %w", err)
	}
	if !os.SameFile(identity, current) {
		return fmt.Errorf("workspace directory changed: %q", path)
	}
	return nil
}

// The opener owns syscall confinement; the walker owns intermediate descriptors.
// Keeping these responsibilities separate permits real traversal and failure tests
// without substituting a weaker filesystem implementation on either platform.
func walkDiscoveryDirectory(parent *os.File, path string,
	openComponent func(*os.File, string) (*os.File, error)) (_ *os.File, err error) {
	var opened *os.File
	defer func() {
		if err != nil && opened != nil {
			err = errors.Join(err, opened.Close())
		}
	}()
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		if err := validateDiscoveryComponent(component); err != nil {
			return nil, err
		}
		child, openErr := openComponent(parent, component)
		if openErr != nil {
			return nil, openErr
		}
		if opened != nil {
			if closeErr := opened.Close(); closeErr != nil {
				opened = child
				return nil, closeErr
			}
		}
		opened, parent = child, child
	}
	return opened, nil
}

func validateDiscoveryComponent(component string) error {
	switch component {
	case "", ".", "..":
		return fmt.Errorf("invalid discovery component %q", component)
	}
	return nil
}

func validateDiscoveryMount(workspace, mountPath string) error {
	if !filepath.IsAbs(mountPath) {
		return fmt.Errorf("ambiguous discovery mount path %q", mountPath)
	}
	relative, err := filepath.Rel(workspace, mountPath)
	if err != nil {
		return fmt.Errorf("resolve discovery mount path: %w", err)
	}
	if relative != "." && filepath.IsLocal(relative) {
		return fmt.Errorf("nested mount in discovery workspace: %q", mountPath)
	}
	return nil
}

func inspectDiscoveryEntry(parent *os.File, name string) (discoveryEntry, error) {
	fd, err := discoveryFD(parent)
	if err != nil {
		return discoveryEntry{}, err
	}
	var identity unix.Stat_t
	err = unix.Fstatat(fd, name, &identity, unix.AT_SYMLINK_NOFOLLOW)
	runtime.KeepAlive(parent)
	if err != nil {
		return discoveryEntry{}, fmt.Errorf("inspect confined entry: %w", err)
	}
	fileType := identity.Mode & unix.S_IFMT
	if fileType != unix.S_IFDIR && fileType != unix.S_IFREG {
		return discoveryEntry{}, fmt.Errorf("unsafe provider results entry: symlink or special file")
	}
	return discoveryEntry{
		directory: fileType == unix.S_IFDIR,
		validate: func(file *os.File) error {
			fd, err := discoveryFD(file)
			if err != nil {
				return err
			}
			var current unix.Stat_t
			err = unix.Fstat(fd, &current)
			runtime.KeepAlive(file)
			if err != nil {
				return fmt.Errorf("inspect opened entry identity: %w", err)
			}
			if identity.Dev != current.Dev || identity.Ino != current.Ino {
				return fmt.Errorf("provider results entry changed during discovery")
			}
			return nil
		},
	}, nil
}

func discoveryFD(file *os.File) (int, error) {
	fd := file.Fd()
	if fd > math.MaxInt {
		return -1, fmt.Errorf("invalid discovery directory descriptor: %w", unix.EBADF)
	}
	return int(fd), nil
}

func discoveryFile(fd int, path string) (*os.File, error) {
	if fd < 0 {
		return nil, fmt.Errorf("invalid opened discovery descriptor: %w", unix.EBADF)
	}
	return os.NewFile(uintptr(fd), path), nil
}
