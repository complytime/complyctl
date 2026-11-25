// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"fmt"
	"os"
)

func walkPreflightParent(workspace *os.File, path string) (*os.File, error) {
	return walkDiscoveryDirectory(workspace, path, func(parent *os.File, name string) (*os.File, error) {
		return openPreflightEntry(parent, name, true)
	})
}

func checkPreflightAccess(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat preflight access: %w", err)
	}
	if info.Mode()&os.ModeSticky != 0 {
		return fmt.Errorf("sticky directory requires retention")
	}
	required := os.FileMode(0o400)
	if info.IsDir() {
		required = 0o700
	}
	// Conservative even for privileged callers: do not use root's permission
	// bypass to present known inaccessible fixtures as safe cleanup candidates.
	if info.Mode().Perm()&required != required {
		return fmt.Errorf("owner access permissions prohibit complete preflight/removal")
	}
	fd, err := discoveryFD(file)
	if err != nil {
		return err
	}
	return checkPlatformPreflight(file, fd, info.IsDir())
}
