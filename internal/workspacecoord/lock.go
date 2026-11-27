// SPDX-License-Identifier: Apache-2.0

// Package workspacecoord coordinates participating host operations by locking
// the existing workspace directory. It does not exclude independent writers or
// establish that providers have stopped after a host exits.
package workspacecoord

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Mode selects shared provider access or exclusive workspace mutation access.
type Mode uint8

const (
	// Shared allows other shared holders, but excludes exclusive access.
	Shared Mode = iota + 1
	// Exclusive excludes all other participating operations in this workspace.
	Exclusive
)

var (
	// ErrBusy means another participating operation holds conflicting access.
	ErrBusy = errors.New("workspace busy")
	// ErrWorkspaceChanged means the selected path no longer names the pinned directory.
	ErrWorkspaceChanged = errors.New("workspace directory changed")
	// ErrUnsupportedFilesystem means cleanup has not been validated on this filesystem.
	ErrUnsupportedFilesystem = errors.New("unsupported cleanup filesystem")
)

// Options identifies an existing workspace and the required access mode.
// Workspace may be relative or a symlink; an empty path or invalid Mode is rejected.
type Options struct {
	Workspace string
	Mode      Mode
}

// Access owns a pinned directory descriptor and its kernel lock until Close.
// Obtain it through Acquire, do not copy it, and keep it open through protected
// work. There is deliberately no lock upgrade operation or exposed descriptor.
// Do not call Close concurrently with other methods or protected work.
type Access struct {
	file       *os.File
	identity   os.FileInfo
	path       string
	requested  string
	mode       Mode
	activities []*os.File
}

// Acquire takes nonblocking access on the existing workspace directory, without
// creating or writing any files. Aliases contend on the directory's identity,
// not on their spelling. It returns ErrBusy for a conflicting operation, or a
// contextual error for unavailable locking or an invalid workspace.
//
// Acquisition alone does not authorize cleanup: callers must also use
// CheckCleanupSupport and, in the cleanup layer, confined traversal checks.
// Other operations may coordinate on filesystems outside the cleanup allowlist.
func Acquire(opts Options) (_ *Access, err error) {
	if opts.Workspace == "" {
		return nil, fmt.Errorf("workspace path must not be empty")
	}
	if opts.Mode != Shared && opts.Mode != Exclusive {
		return nil, fmt.Errorf("invalid workspace access mode: %d", opts.Mode)
	}
	requested, err := filepath.Abs(opts.Workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace %q: %w", opts.Workspace, err)
	}
	canonical, err := filepath.EvalSymlinks(requested)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace %q: %w", requested, err)
	}
	file, err := openDirectory(canonical)
	if err != nil {
		return nil, fmt.Errorf("open workspace directory %q: %w", canonical, err)
	}
	access := &Access{file: file, path: canonical, requested: requested, mode: opts.Mode}
	defer func() {
		if err != nil {
			err = errors.Join(err, access.Close())
		}
	}()
	access.identity, err = file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect workspace directory %q: %w", canonical, err)
	}
	if err := lockDirectory(file, opts.Mode); err != nil {
		return nil, fmt.Errorf("lock workspace %q: %w", canonical, err)
	}
	if err := access.Validate(); err != nil {
		return nil, err
	}
	return access, nil
}

// Path returns the absolute, symlink-resolved path captured at acquisition.
// The directory descriptor, rather than this path string, owns the lock.
func (a *Access) Path() string {
	return a.path
}

// Validate checks that the original selection and canonical path still name the
// pinned directory. It returns ErrWorkspaceChanged on observed namespace changes.
// Callers must keep a stable workspace namespace; this check is not an atomic
// defense against arbitrary external renames or administrative mount changes.
func (a *Access) Validate() error {
	if _, err := a.file.Stat(); err != nil {
		return fmt.Errorf("inspect held workspace %q: %w", a.path, err)
	}
	resolved, err := filepath.EvalSymlinks(a.requested)
	if err != nil {
		return fmt.Errorf("revalidate workspace %q: %w", a.requested,
			errors.Join(ErrWorkspaceChanged, err))
	}
	current, err := os.Lstat(a.path)
	if err != nil {
		return fmt.Errorf("revalidate workspace %q: %w", a.path,
			errors.Join(ErrWorkspaceChanged, err))
	}
	if resolved != a.path || !os.SameFile(a.identity, current) {
		return fmt.Errorf("revalidate workspace %q: %w", a.path, ErrWorkspaceChanged)
	}
	return nil
}

// CheckCleanupSupport requires exclusive access and a baseline local filesystem:
// ext4, XFS, Btrfs, or tmpfs on Linux; APFS on Darwin. Unknown, network, FUSE,
// overlay, and HFS+ filesystems are refused with ErrUnsupportedFilesystem.
// This read-only check does not validate descendant mounts or deletion safety.
func (a *Access) CheckCleanupSupport() error {
	if err := a.Validate(); err != nil {
		return err
	}
	if a.mode != Exclusive {
		return fmt.Errorf("cleanup requires exclusive workspace access: %q", a.path)
	}
	if err := checkFilesystem(a.file); err != nil {
		return fmt.Errorf("check cleanup support for workspace %q: %w", a.path, err)
	}
	return nil
}

// Close releases this access by closing its directory descriptor. No workspace
// files are removed or changed. Repeated calls return an error wrapping os.ErrClosed.
// Activity descriptors are also closed, but their persistent records remain;
// only an explicit Activity.CompleteNormal call removes an owned record.
func (a *Access) Close() error {
	var activityErr error
	for _, file := range a.activities {
		activityErr = errors.Join(activityErr, file.Close())
	}
	a.activities = nil
	// Closing the sole descriptor releases flock without a separate unlock step.
	// Per Simplicity & Isolation, no lock-file lifecycle or lock upgrade is needed.
	if err := a.file.Close(); err != nil {
		return fmt.Errorf("close workspace access %q: %w", a.path, errors.Join(err, activityErr))
	}
	if activityErr != nil {
		return fmt.Errorf("close activity handles for workspace %q: %w", a.path, activityErr)
	}
	return nil
}
