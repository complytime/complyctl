// SPDX-License-Identifier: Apache-2.0

package complytime

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// legacyDirName is the legacy home-relative directory that predates XDG
// Base Directory adoption. It previously held both cache and state data.
const legacyDirName = ".complytime"

// CheckLegacyDir prints a deprecation warning to w when the legacy
// ~/.complytime/ directory exists but the XDG cache or data directory
// has not yet been created. This guides users through migration from
// the legacy single-directory layout to the XDG-compliant split layout.
//
// The function is informational only and never modifies the filesystem.
// Callers should pass os.Stderr as w for production use; tests pass a
// bytes.Buffer for assertion.
func CheckLegacyDir(w io.Writer) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	checkLegacyDirWithHome(w, homeDir)
}

// checkLegacyDirWithHome is the testable core of CheckLegacyDir.
// Accepting homeDir as a parameter lets tests control the filesystem
// layout without overriding os.UserHomeDir() or environment variables.
func checkLegacyDirWithHome(w io.Writer, homeDir string) {
	legacyDir := filepath.Join(homeDir, legacyDirName)
	if !dirExists(legacyDir) {
		return
	}

	cacheDir, err := ResolveCacheDir()
	if err != nil {
		return
	}
	dataDir, err := ResolveDataDir()
	if err != nil {
		return
	}

	cacheExists := dirExists(cacheDir)
	dataExists := dirExists(dataDir)

	// If both XDG directories already exist, the user has migrated
	// (or complyctl has created them on a previous run). No warning needed.
	if cacheExists && dataExists {
		return
	}

	fmt.Fprintf(w, `WARNING: Legacy directory found at %s
This location is deprecated. complyctl now uses XDG Base Directories:

  Cache (re-fetchable policies/complypacks): %s
  Data  (state.json — not recoverable):     %s

To migrate:
  1. Copy state.json to the new data directory (important — not recoverable):
       mkdir -p %q
       cp %s %s

  2. Move cached policies to the new cache directory (optional — will be re-fetched):
       mkdir -p %q
       cp -r %s %s

  3. Remove the legacy directory after verifying the migration:
       rm -rf %q

`, legacyDir, cacheDir, dataDir,
		dataDir,
		filepath.Join(legacyDir, StateFileName),
		filepath.Join(dataDir, StateFileName),
		cacheDir,
		filepath.Join(legacyDir, PoliciesSubdir, "*"),
		filepath.Join(cacheDir, PoliciesSubdir)+"/",
		legacyDir,
	)
}

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
