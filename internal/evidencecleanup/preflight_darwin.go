// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func checkPlatformPreflight(file *os.File, fd int, directory bool) error {
	defer runtime.KeepAlive(file)
	// Separate descriptor checks by responsibility (SRP), rather than combining
	// platform policy with traversal. Keep the file alive through every check.
	if err := checkDarwinPreflightOwnership(fd); err != nil {
		return err
	}
	if err := checkDarwinPreflightFilesystem(fd); err != nil {
		return err
	}
	if err := checkDarwinPreflightACL(fd); err != nil {
		return err
	}
	return checkDarwinPreflightDirectoryAccess(fd, directory)
}

func checkDarwinPreflightOwnership(fd int) error {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("inspect preflight ownership: %w", err)
	}
	if int64(stat.Uid) != int64(os.Geteuid()) {
		return fmt.Errorf("member is not owned by the effective user")
	}
	if stat.Flags&(unix.UF_IMMUTABLE|unix.SF_IMMUTABLE|unix.UF_APPEND|unix.SF_APPEND) != 0 {
		return fmt.Errorf("immutable or append-only member requires retention")
	}
	return nil
}

func checkDarwinPreflightFilesystem(fd int) error {
	var fs unix.Statfs_t
	if err := unix.Fstatfs(fd, &fs); err != nil {
		return fmt.Errorf("inspect preflight filesystem: %w", err)
	}
	if fs.Flags&unix.MNT_RDONLY != 0 {
		return fmt.Errorf("read-only filesystem")
	}
	return nil
}

func checkDarwinPreflightDirectoryAccess(fd int, directory bool) error {
	if directory {
		if err := unix.Faccessat(fd, ".", unix.W_OK|unix.X_OK, unix.AT_EACCESS); err != nil {
			return fmt.Errorf("directory removal access: %w", err)
		}
	}
	return nil
}

func checkDarwinPreflightACL(fd int) error {
	buffer, err := readDarwinPreflightSecurity(fd)
	if err != nil {
		return err
	}
	return checkDarwinSecurityBuffer(buffer)
}

func readDarwinPreflightSecurity(fd int) ([darwinSecurityBufferSize]byte, error) {
	api, err := darwinPreflightSecurity()
	if err != nil {
		return [darwinSecurityBufferSize]byte{}, fmt.Errorf("initialize ACL inspection: %w", err)
	}
	return api.readSecurity(fd)
}
