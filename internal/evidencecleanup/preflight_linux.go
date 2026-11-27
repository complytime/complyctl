// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

const (
	linuxImmutableFlag = 0x10
	linuxAppendFlag    = 0x20
	linuxAccessACL     = "system.posix_acl_access"
)

func checkPlatformPreflight(file *os.File, fd int, directory bool) error {
	defer runtime.KeepAlive(file)
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("inspect preflight ownership: %w", err)
	}
	if int64(stat.Uid) != int64(os.Geteuid()) {
		return fmt.Errorf("member is not owned by the effective user")
	}
	var fs unix.Statfs_t
	if err := unix.Fstatfs(fd, &fs); err != nil {
		return fmt.Errorf("inspect preflight filesystem: %w", err)
	}
	if fs.Flags&unix.ST_RDONLY != 0 {
		return fmt.Errorf("read-only filesystem")
	}
	if err := checkLinuxPreflightFlags(fd); err != nil {
		return err
	}
	if directory {
		if err := unix.Faccessat(fd, ".", unix.W_OK|unix.X_OK, unix.AT_EACCESS); err != nil {
			return fmt.Errorf("directory removal access: %w", err)
		}
	}
	return nil
}

func checkLinuxPreflightFlags(fd int) error {
	_, err := unix.Fgetxattr(fd, linuxAccessACL, nil)
	if err == nil {
		// Do not approximate ACL unlink semantics with mode bits. Retaining ACL
		// members is conservative until an ACL-aware removal policy is available.
		return fmt.Errorf("extended access ACL requires retention")
	}
	if !errors.Is(err, unix.ENODATA) && !errors.Is(err, unix.EOPNOTSUPP) {
		return fmt.Errorf("inspect access ACL: %w", err)
	}
	flags, err := unix.IoctlGetInt(fd, unix.FS_IOC_GETFLAGS)
	if errors.Is(err, unix.ENOTTY) || errors.Is(err, unix.EOPNOTSUPP) {
		// Filesystems without inode flags cannot carry immutable/append flags.
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect inode flags: %w", err)
	}
	if flags&(linuxImmutableFlag|linuxAppendFlag) != 0 {
		return fmt.Errorf("immutable or append-only member requires retention")
	}
	return nil
}
