// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	maxPreflightDepth   = 128
	maxPreflightEntries = 100000
)

func preflightMember(workspace *os.File, path string, identity os.FileInfo) (err error) {
	if identity == nil || !filepath.IsLocal(path) {
		return fmt.Errorf("missing identity or nonlocal member path")
	}
	// Check all parents, not just the member's readability: removal requires
	// directory permissions and can be prohibited by ACLs or filesystem flags.
	if err := checkPreflightAccess(workspace); err != nil {
		return fmt.Errorf("workspace access: %w", err)
	}
	parent, err := openPreflightParent(workspace, filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("member parent %q: %w", path, err)
	}
	defer func() { err = errors.Join(err, parent.Close()) }()
	member, err := openPreflightEntry(parent, filepath.Base(path), identity.IsDir())
	if err != nil {
		return fmt.Errorf("open member %q: %w", path, err)
	}
	defer func() { err = errors.Join(err, member.Close()) }()
	current, err := member.Stat()
	if err != nil {
		return fmt.Errorf("stat member: %w", err)
	}
	if !os.SameFile(identity, current) || identity.Mode() != current.Mode() ||
		(!identity.IsDir() && (identity.Size() != current.Size() ||
			!identity.ModTime().Equal(current.ModTime()))) {
		return fmt.Errorf("member identity or report contents changed since inventory")
	}
	if identity.IsDir() {
		remaining := maxPreflightEntries
		return preflightContents(member, path, 0, &remaining)
	}
	return nil
}

func openPreflightParent(workspace *os.File, path string) (*os.File, error) {
	return walkPreflightParent(workspace, path)
}

func openPreflightEntry(parent *os.File, name string, directory bool) (file *os.File, err error) {
	if directory {
		file, err = openDiscoveryDirectory(parent, name)
	} else {
		file, err = openInventoryRegular(parent, name)
	}
	if err != nil {
		return nil, err
	}
	if err := checkPreflightAccess(file); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func preflightContents(dir *os.File, path string, depth int, remaining *int) error {
	if depth >= maxPreflightDepth {
		return fmt.Errorf("evaluation depth limit reached at %q", path)
	}
	for {
		names, err := dir.Readdirnames(readDirectoryBatch)
		for _, name := range names {
			*remaining--
			if *remaining < 0 {
				return fmt.Errorf("evaluation entry limit reached at %q", path)
			}
			if err := preflightChild(dir, name, filepath.Join(path, name), depth, remaining); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read evaluation contents %q: %w", path, err)
		}
	}
}

func preflightChild(parent *os.File, name, path string, depth int, remaining *int) (err error) {
	entry, err := inspectDiscoveryEntry(parent, name)
	if err != nil {
		return fmt.Errorf("inspect evaluation content %q: %w", path, err)
	}
	child, err := openPreflightEntry(parent, name, entry.directory)
	if err != nil {
		return fmt.Errorf("preflight evaluation content %q: %w", path, err)
	}
	defer func() { err = errors.Join(err, child.Close()) }()
	if err := entry.validate(child); err != nil {
		return fmt.Errorf("revalidate evaluation content %q: %w", path, err)
	}
	if entry.directory {
		return preflightContents(child, path, depth+1, remaining)
	}
	return nil
}
