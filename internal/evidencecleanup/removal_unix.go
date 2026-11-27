// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// removal is a single-member filesystem primitive, not group authorization.
// The caller must hold exclusive workspace access, check cleanup support and
// activity, and preflight the complete group before calling remove. Keeping all
// opened identities until close trades descriptor use for confinement (SRP);
// descriptor exhaustion fails preparation without deleting anything.
type removal struct {
	workspace *os.File
	root      *removalEntry
	opened    []*os.File
	guard     func() error
	unlink    func(*os.File, string, bool) error
	removed   []string
}

type removalEntry struct {
	file     *os.File
	identity os.FileInfo
	parent   *removalEntry
	name     string
	path     string
	children []*removalEntry
}

func prepareRemoval(workspace *os.File, path string, identity os.FileInfo) (_ *removal, err error) {
	if err := validateRemovalPath(path, identity); err != nil {
		return nil, err
	}
	guard, err := removalPlatformGuard(workspace)
	if err != nil {
		return nil, fmt.Errorf("prepare removal confinement: %w", err)
	}
	r := &removal{workspace: workspace, guard: guard, unlink: unlinkRemovalEntry}
	defer func() {
		if err != nil {
			err = errors.Join(err, r.close())
		}
	}()
	parent, err := r.pinParents(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	remaining := maxPreflightEntries
	r.root, err = r.pinEntry(parent, filepath.Base(path), path, identity, 0, &remaining)
	if err != nil {
		return nil, fmt.Errorf("prepare member %q: %w", path, err)
	}
	if err := r.guard(); err != nil {
		return nil, err
	}
	return r, nil
}

func validateRemovalPath(path string, identity os.FileInfo) error {
	if identity == nil || !filepath.IsLocal(path) || filepath.Clean(path) != path {
		return fmt.Errorf("missing identity or noncanonical removal path")
	}
	if identity.Mode().IsRegular() && filepath.Dir(path) == reportDirectory {
		format, _ := classifyReportName(filepath.Base(path))
		if format != "" {
			return nil
		}
	}
	return validateRemovalEvaluationPath(path, identity)
}

func validateRemovalEvaluationPath(path string, identity os.FileInfo) error {
	parts := strings.Split(path, string(filepath.Separator))
	if !identity.IsDir() || len(parts) < 4 || parts[0] != managedDirectory ||
		parts[2] != resultsDirectory {
		return fmt.Errorf("removal is outside approved members: %q", path)
	}
	switch parts[1] {
	case "ampel", "opa", "openscap":
	default:
		return fmt.Errorf("unknown removal provider: %q", parts[1])
	}
	for _, label := range parts[3 : len(parts)-1] {
		if !IsLabel(label) || strings.HasPrefix(label, evaluationPrefix) {
			return fmt.Errorf("invalid removal label: %q", label)
		}
	}
	_, err := ParseEvaluationName(parts[len(parts)-1])
	return err
}

func (r *removal) pinParents(path string) (*removalEntry, error) {
	identity, err := r.workspace.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect removal workspace: %w", err)
	}
	parent := &removalEntry{file: r.workspace, identity: identity}
	if err := checkPreflightAccess(parent.file); err != nil {
		return nil, err
	}
	for _, name := range strings.Split(path, string(filepath.Separator)) {
		child, err := r.openEntry(parent, name, filepath.Join(parent.path, name), true)
		if err != nil {
			return nil, err
		}
		parent = child
	}
	return parent, nil
}

func (r *removal) openEntry(parent *removalEntry, name, path string, directory bool) (*removalEntry, error) {
	file, err := openPreflightEntry(parent.file, name, directory)
	if err != nil {
		return nil, fmt.Errorf("open removal entry %q: %w", path, err)
	}
	r.opened = append(r.opened, file)
	identity, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect removal entry %q: %w", path, err)
	}
	return &removalEntry{file: file, identity: identity, parent: parent, name: name, path: path}, nil
}

func (r *removal) pinEntry(parent *removalEntry, name, path string, identity os.FileInfo,
	depth int, remaining *int) (*removalEntry, error) {
	*remaining--
	if depth >= maxPreflightDepth || *remaining < 0 {
		return nil, fmt.Errorf("removal traversal limit at %q", path)
	}
	node, err := r.openEntry(parent, name, path, identity.IsDir())
	if err != nil {
		return nil, err
	}
	if err := sameRemovalIdentity(identity, node.identity); err != nil {
		return nil, err
	}
	if identity.IsDir() {
		if err := r.pinContents(node, depth, remaining); err != nil {
			return nil, err
		}
	}
	return node, nil
}

func (r *removal) pinContents(parent *removalEntry, depth int, remaining *int) error {
	for {
		names, readErr := parent.file.Readdirnames(readDirectoryBatch)
		sort.Strings(names)
		for _, name := range names {
			if err := r.pinChild(parent, name, depth+1, remaining); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read removal contents %q: %w", parent.path, readErr)
		}
	}
}

func (r *removal) pinChild(parent *removalEntry, name string, depth int, remaining *int) error {
	*remaining--
	if depth >= maxPreflightDepth || *remaining < 0 {
		return fmt.Errorf("removal traversal limit at %q", parent.path)
	}
	inspected, err := inspectDiscoveryEntry(parent.file, name)
	if err != nil {
		return fmt.Errorf("inspect removal content %q: %w", filepath.Join(parent.path, name), err)
	}
	node, err := r.openEntry(parent, name, filepath.Join(parent.path, name), inspected.directory)
	if err != nil {
		return err
	}
	if err := inspected.validate(node.file); err != nil {
		return err
	}
	parent.children = append(parent.children, node)
	if node.identity.IsDir() {
		return r.pinContents(node, depth, remaining)
	}
	return nil
}

func sameRemovalIdentity(expected, current os.FileInfo) error {
	if !os.SameFile(expected, current) || expected.Mode() != current.Mode() {
		return fmt.Errorf("removal identity or mode changed")
	}
	if !expected.IsDir() && (expected.Size() != current.Size() ||
		!expected.ModTime().Equal(current.ModTime())) {
		return fmt.Errorf("removal file contents changed")
	}
	return nil
}

// validate follows the retained ancestry, not a newly resolved absolute path.
// No OS here supplies inode-conditional unlink: the supported mode requires a
// stable namespace. Observed replacement stops removal; arbitrary external
// rename/mount races after the last check are not an atomicity guarantee.
func (r *removal) validate(node *removalEntry) (err error) {
	if node.parent == nil {
		return checkPreflightAccess(node.file)
	}
	if err := r.validate(node.parent); err != nil {
		return err
	}
	current, err := openPreflightEntry(node.parent.file, node.name, node.identity.IsDir())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, current.Close()) }()
	info, err := current.Stat()
	if err != nil {
		return fmt.Errorf("reinspect removal identity: %w", err)
	}
	return sameRemovalIdentity(node.identity, info)
}

func (r *removal) remove() error {
	if err := r.guard(); err != nil {
		return err
	}
	if err := preflightMember(r.workspace, r.root.path, r.root.identity); err != nil {
		return err
	}
	if err := r.validateTree(r.root); err != nil {
		return err
	}
	return r.removeEntry(r.root)
}

func (r *removal) validateTree(node *removalEntry) error {
	if err := r.revalidate(node); err != nil {
		return err
	}
	for _, child := range node.children {
		if err := r.validateTree(child); err != nil {
			return err
		}
	}
	return nil
}

func (r *removal) removeEntry(node *removalEntry) error {
	if err := r.revalidate(node); err != nil {
		return err
	}
	for _, child := range node.children {
		if err := r.removeEntry(child); err != nil {
			return err
		}
	}
	if err := r.revalidate(node); err != nil {
		return err
	}
	if err := r.unlink(node.parent.file, node.name, node.identity.IsDir()); err != nil {
		return fmt.Errorf("unlink member entry %q: %w", node.path, err)
	}
	r.removed = append(r.removed, node.path)
	return nil
}

func (r *removal) revalidate(node *removalEntry) error {
	if err := r.guard(); err != nil {
		return fmt.Errorf("revalidate removal confinement %q: %w", node.path, err)
	}
	if err := r.validate(node); err != nil {
		return fmt.Errorf("revalidate removal entry %q: %w", node.path, err)
	}
	return nil
}

func (r *removal) close() error {
	var err error
	for i := len(r.opened) - 1; i >= 0; i-- {
		err = errors.Join(err, r.opened[i].Close())
	}
	r.opened = nil
	return err
}

func unlinkRemovalEntry(parent *os.File, name string, directory bool) error {
	// Backslashes are literal filename bytes on both supported Unix platforms.
	// Keep the same component boundary used when opening and preflighting entries.
	if filepath.Base(name) != name {
		return fmt.Errorf("invalid unlink component %q", name)
	}
	if err := validateDiscoveryComponent(name); err != nil {
		return err
	}
	fd, err := discoveryFD(parent)
	if err != nil {
		return err
	}
	flags := 0
	if directory {
		flags = unix.AT_REMOVEDIR
	}
	err = unix.Unlinkat(fd, name, flags)
	runtime.KeepAlive(parent)
	return err
}
