// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"

	"golang.org/x/sys/unix"
)

func removalPlatformGuard(workspace *os.File) (func() error, error) {
	return newRemovalDarwinGuard(workspace, removalDarwinMounts)
}

func newRemovalDarwinGuard(workspace *os.File,
	readMounts func(*os.File, os.FileInfo) ([]removalDarwinMount, error)) (func() error, error) {
	identity, err := workspace.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect removal workspace: %w", err)
	}
	baseline, err := readMounts(workspace, identity)
	if err != nil {
		return nil, err
	}
	return func() error {
		return revalidateRemovalDarwin(workspace, identity, baseline, readMounts)
	}, nil
}

func revalidateRemovalDarwin(workspace *os.File, identity os.FileInfo, baseline []removalDarwinMount,
	readMounts func(*os.File, os.FileInfo) ([]removalDarwinMount, error)) error {
	if err := validateDiscoveryWorkspace(workspace.Name(), identity); err != nil {
		return err
	}
	current, err := readMounts(workspace, identity)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(baseline, current) {
		return fmt.Errorf("removal mount table changed")
	}
	return nil
}

// Only stable identity/layout fields participate: free blocks and other live
// usage counters change during our own unlinks. x/sys supplies the Darwin ABI;
// ACL inspection continues to use the existing libSystem wrapper, not Linux APIs.
type removalDarwinMount struct {
	path   string
	source string
	fsid   unix.Fsid
	flags  uint32
}

func removalDarwinMounts(workspace *os.File, identity os.FileInfo) ([]removalDarwinMount, error) {
	fd, err := discoveryFD(workspace)
	if err != nil {
		return nil, err
	}
	var fs unix.Statfs_t
	err = unix.Fstatfs(fd, &fs)
	runtime.KeepAlive(workspace)
	if err != nil {
		return nil, fmt.Errorf("inspect removal filesystem: %w", err)
	}
	mounts, err := readDiscoveryMounts()
	if err != nil {
		return nil, err
	}
	return validateRemovalDarwinMounts(workspace.Name(), identity, fs, mounts)
}

func validateRemovalDarwinMounts(path string, identity os.FileInfo, fs unix.Statfs_t,
	mounts []unix.Statfs_t) ([]removalDarwinMount, error) {
	var result []removalDarwinMount
	seen := make(map[string]bool)
	identities := make(map[unix.Fsid]bool)
	matched := false
	for _, mount := range mounts {
		name := unix.ByteSliceToString(mount.Mntonname[:])
		if err := validateRemovalDarwinMount(path, identity, mount, seen, identities); err != nil {
			return nil, err
		}
		if sameRemovalDarwinMount(mount, fs) {
			matched = true
		}
		result = append(result, removalDarwinMount{name,
			unix.ByteSliceToString(mount.Mntfromname[:]), mount.Fsid, mount.Flags})
	}
	if !matched {
		return nil, fmt.Errorf("opened removal filesystem absent from mount table")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].path < result[j].path })
	return result, nil
}

func sameRemovalDarwinMount(mount, fs unix.Statfs_t) bool {
	return mount.Fsid == fs.Fsid && mount.Mntonname == fs.Mntonname && mount.Mntfromname == fs.Mntfromname
}

func validateRemovalDarwinMount(path string, identity os.FileInfo, mount unix.Statfs_t,
	seen map[string]bool, identities map[unix.Fsid]bool) error {
	name := unix.ByteSliceToString(mount.Mntonname[:])
	if err := validateRemovalDarwinMountIdentity(name, mount.Fsid); err != nil {
		return err
	}
	if seen[name] || identities[mount.Fsid] {
		return fmt.Errorf("ambiguous repeated removal mount at %q", name)
	}
	seen[name], identities[mount.Fsid] = true, true
	if err := validateDiscoveryMount(path, name); err != nil {
		return err
	}
	return checkRemovalMountAncestors(name, identity)
}

func validateRemovalDarwinMountIdentity(name string, fsid unix.Fsid) error {
	if !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return fmt.Errorf("ambiguous removal mount path %q", name)
	}
	if fsid == (unix.Fsid{}) {
		return fmt.Errorf("removal mount identity unavailable at %q", name)
	}
	return nil
}

// APFS firmlinks can expose Data-volume paths under a different spelling.
// Compare ancestor identities as well as lexical paths; equality of fsid alone
// is insufficient. This inspection never supplies a deletion pathname.
func checkRemovalMountAncestors(mount string, workspace os.FileInfo) error {
	for parent := filepath.Dir(mount); parent != mount; parent = filepath.Dir(parent) {
		info, err := os.Stat(parent)
		if err != nil {
			return fmt.Errorf("ambiguous removal mount ancestry %q: %w", parent, err)
		}
		if os.SameFile(workspace, info) {
			return fmt.Errorf("nested removal mount through filesystem alias: %q", mount)
		}
		mount = parent
	}
	return nil
}
