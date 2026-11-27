// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package workspacecoord

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"golang.org/x/sys/unix"
)

// BeginActivity persists a private, exclusively created invocation record before
// returning. Call it BEFORE starting any provider, and do not start providers on
// error. Partial records on failure are intentionally retained and block cleanup.
// Keep this Access open through all provider/RPC work and report publication.
// Neither acquisition nor marker creation applies the cleanup filesystem allowlist.
func (a *Access) BeginActivity(opts ActivityOptions) (*Activity, error) {
	return a.beginActivity(opts, persistActivity)
}

func (a *Access) beginActivity(opts ActivityOptions,
	persist func(*os.File, []byte) error,
) (_ *Activity, err error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	dirs, err := a.openActivityDirectories(true)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, closeActivityDirectories(dirs)) }()
	var token [activityTokenBytes]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, fmt.Errorf("allocate activity identity: %w", err)
	}
	id := hex.EncodeToString(token[:])
	op := &Activity{access: a, name: "operation-" + id + ".json"}
	op.contents, err = a.activityContents(opts, id)
	if err != nil {
		return nil, err
	}
	op.directories, err = activityDirectoryIdentities(dirs)
	if err != nil {
		return nil, err
	}
	op.file, err = openActivityAt(dirs[len(dirs)-1], op.name,
		unix.O_RDWR|unix.O_CREAT|unix.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create activity record %q: %w", op.Path(), err)
	}
	// Access owns the descriptor even on write/sync failure. Per Simplicity &
	// Isolation, failure never rolls back a record that might represent activity.
	a.activities = append(a.activities, op.file)
	if err := persist(op.file, op.contents); err != nil {
		return nil, fmt.Errorf("persist activity record %q: %w", op.Path(), err)
	}
	if err := a.syncActivityDirectories(dirs); err != nil {
		return nil, fmt.Errorf("persist activity record %q: %w", op.Path(), err)
	}
	if err := op.validateDirectories(dirs); err != nil {
		return nil, err
	}
	if err := op.validateRecord(dirs[len(dirs)-1]); err != nil {
		return nil, fmt.Errorf("validate persisted activity %q: %w", op.Path(), err)
	}
	return op, nil
}

func persistActivity(file *os.File, contents []byte) error {
	if _, err := file.Write(contents); err != nil {
		return fmt.Errorf("write activity: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync activity: %w", err)
	}
	return nil
}

// CompleteNormal is the caller's explicit assertion that every started provider
// operation/RPC completed normally (including handled application errors), all
// report writes finished, and normal provider teardown completed. This is the
// accepted host-operation boundary, NOT proof that arbitrary descendants exited.
// Never call after transport failure, cancellation with uncertain completion,
// abnormal exit, or force-killing a provider: those paths must retain the record.
// Only this Activity's unchanged record is removed; workspace access stays held.
// As with workspace access, callers must maintain a stable namespace. Unix has
// no inode-conditional unlink; this detects observed replacements, not arbitrary
// external mutation between the final identity check and unlink.
func (a *Activity) CompleteNormal() (err error) {
	if err := a.access.Validate(); err != nil {
		return err
	}
	if a.completed {
		return fmt.Errorf("activity already completed: %q", a.Path())
	}
	dirs, err := a.access.openActivityDirectories(false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closeActivityDirectories(dirs)) }()
	if err := a.validateDirectories(dirs); err != nil {
		return err
	}
	parent := dirs[len(dirs)-1]
	if err := a.validateRecord(parent); err != nil {
		return fmt.Errorf("retain changed activity record %q: %w", a.Path(), err)
	}
	fd, err := directoryFD(parent)
	if err != nil {
		return fmt.Errorf("get activity directory descriptor %q: %w", a.Path(), err)
	}
	err = unix.Unlinkat(fd, a.name, 0)
	runtime.KeepAlive(parent)
	if err != nil {
		return fmt.Errorf("remove completed activity %q: %w", a.Path(), err)
	}
	a.completed = true
	if err := parent.Sync(); err != nil {
		return fmt.Errorf("persist completed activity removal %q: %w", a.Path(), err)
	}
	return nil
}

func (a *Activity) validateRecord(parent *os.File) (err error) {
	current, err := openActivityAt(parent, a.name, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, current.Close()) }()
	original, err := a.file.Stat()
	if err != nil {
		return err
	}
	info, err := current.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || !os.SameFile(original, info) {
		return fmt.Errorf("activity identity changed")
	}
	contents, err := io.ReadAll(io.LimitReader(current, int64(len(a.contents)+1)))
	if err != nil {
		return err
	}
	if !bytes.Equal(contents, a.contents) {
		return fmt.Errorf("activity contents changed")
	}
	return nil
}

// CheckNoActivity requires exclusive access and discovers unresolved records
// without creating, reading, interpreting, or deleting them. Every directory
// entry blocks cleanup, so malformed, unreadable, unknown, or unsafe records
// cannot be mistaken for inactivity. Missing legacy coordination state is safe
// within the updated-host boundary only; older hosts/direct calls are unsupported.
// Call CheckCleanupSupport separately before any evidence cleanup.
func (a *Access) CheckNoActivity() (err error) {
	if err := a.Validate(); err != nil {
		return err
	}
	if a.mode != Exclusive {
		return fmt.Errorf("activity discovery requires exclusive workspace access: %q", a.path)
	}
	dirs, err := a.openActivityDirectories(false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closeActivityDirectories(dirs)) }()
	if len(dirs) < activityDirectoryDepth {
		return a.validateActivityDirectories(dirs)
	}
	parent := dirs[len(dirs)-1]
	entries, err := parent.ReadDir(-1)
	if err != nil {
		return a.blockedActivity(parent.Name(), err)
	}
	if err := a.validateActivityDirectories(dirs); err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	paths := make([]string, len(entries))
	for i, entry := range entries {
		paths[i] = filepath.Join(parent.Name(), entry.Name())
	}
	sort.Strings(paths)
	return &ActivityError{Workspace: a.path, Paths: paths,
		Cause: fmt.Errorf("activity remains unresolved; record contents are diagnostic only")}
}

func (a *Access) blockedActivity(path string, cause error) error {
	return &ActivityError{Workspace: a.path, Paths: []string{path}, Cause: cause}
}

func (a *Access) openActivityDirectories(create bool) (_ []*os.File, err error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	var dirs []*os.File
	defer func() {
		if err != nil {
			err = errors.Join(err, closeActivityDirectories(dirs))
		}
	}()
	parent := a.file
	for _, name := range []string{activityDirectory, coordinationDirectory, operationsDirectory} {
		path := filepath.Join(parent.Name(), name)
		if create {
			if err := mkdirActivityAt(parent, name); err != nil {
				return nil, a.blockedActivity(path, err)
			}
		}
		dir, err := openActivityAt(parent, name, unix.O_RDONLY|unix.O_DIRECTORY, 0)
		if err != nil {
			// Only a missing coordination component is a legacy workspace. Do not
			// swallow ENOENT from workspace validation or an observed replacement.
			if !create && errors.Is(err, os.ErrNotExist) {
				return dirs, nil
			}
			return nil, a.blockedActivity(path, err)
		}
		dirs = append(dirs, dir)
		parent = dir
	}
	return dirs, nil
}

func mkdirActivityAt(parent *os.File, name string) error {
	fd, err := directoryFD(parent)
	if err != nil {
		return err
	}
	err = unix.Mkdirat(fd, name, 0o700)
	runtime.KeepAlive(parent)
	if err != nil && !errors.Is(err, unix.EEXIST) {
		return fmt.Errorf("create activity directory: %w", err)
	}
	return nil
}

func openActivityAt(parent *os.File, name string, flags int, perm uint32) (*os.File, error) {
	fd, err := directoryFD(parent)
	if err != nil {
		return nil, err
	}
	child, err := unix.Openat(fd, name, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC, perm)
	runtime.KeepAlive(parent)
	if err != nil {
		return nil, fmt.Errorf("open activity path %q: %w", filepath.Join(parent.Name(), name), err)
	}
	if child < 0 {
		return nil, fmt.Errorf("invalid activity path descriptor for %q", name)
	}
	return os.NewFile(uintptr(child), filepath.Join(parent.Name(), name)), nil
}

func closeActivityDirectories(dirs []*os.File) error {
	var err error
	for i := len(dirs) - 1; i >= 0; i-- {
		err = errors.Join(err, dirs[i].Close())
	}
	return err
}

func activityDirectoryIdentities(dirs []*os.File) ([]os.FileInfo, error) {
	identities := make([]os.FileInfo, len(dirs))
	for i, dir := range dirs {
		info, err := dir.Stat()
		if err != nil {
			return nil, fmt.Errorf("inspect activity directory %q: %w", dir.Name(), err)
		}
		identities[i] = info
	}
	return identities, nil
}

func (a *Access) syncActivityDirectories(dirs []*os.File) error {
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := dirs[i].Sync(); err != nil {
			return fmt.Errorf("sync activity directory %q: %w", dirs[i].Name(), err)
		}
	}
	if err := a.file.Sync(); err != nil {
		return fmt.Errorf("sync workspace directory %q: %w", a.path, err)
	}
	return nil
}

func (a *Access) validateActivityDirectories(dirs []*os.File) (err error) {
	current, err := a.openActivityDirectories(false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closeActivityDirectories(current)) }()
	original, err := activityDirectoryIdentities(dirs)
	if err != nil {
		return err
	}
	return a.compareActivityDirectories(original, current)
}

func (a *Activity) validateDirectories(dirs []*os.File) error {
	if err := a.access.validateActivityDirectories(dirs); err != nil {
		return err
	}
	return a.access.compareActivityDirectories(a.directories, dirs)
}

func (a *Access) compareActivityDirectories(original []os.FileInfo, dirs []*os.File) error {
	if len(original) != len(dirs) {
		return a.blockedActivity(filepath.Join(a.path, activityDirectory,
			coordinationDirectory, operationsDirectory), fmt.Errorf("activity directory layout changed"))
	}
	for i, dir := range dirs {
		info, err := dir.Stat()
		if err != nil {
			return a.blockedActivity(dir.Name(), err)
		}
		if !os.SameFile(original[i], info) {
			return a.blockedActivity(dir.Name(), fmt.Errorf("activity directory identity changed"))
		}
	}
	return nil
}
