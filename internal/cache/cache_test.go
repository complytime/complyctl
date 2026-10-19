// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyStoreBackupPath_RetainsReservationDirectory(t *testing.T) {
	activePath := filepath.Join(t.TempDir(), "policy")
	require.NoError(t, os.MkdirAll(activePath, 0o700))

	backupPath, err := policyStoreBackupPath(activePath)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, os.RemoveAll(backupPath)) })

	assert.DirExists(t, backupPath)
	assert.Equal(t, filepath.Join(backupPath, "previous"), policyStoreBackupContentPath(backupPath))
}

func TestStagedPolicyStoreInstall_PreservesCachedDescendantStore(t *testing.T) {
	cacheRoot := t.TempDir()
	activePath := filepath.Join(cacheRoot, "parent")
	stagingPath := filepath.Join(cacheRoot, "staging")
	descendantPath := filepath.Join(activePath, "child")
	require.NoError(t, os.MkdirAll(descendantPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(activePath, "oci-layout"), []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(descendantPath, "oci-layout"), []byte("child"), 0o600))
	require.NoError(t, os.MkdirAll(stagingPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(stagingPath, "oci-layout"), []byte("new"), 0o600))

	store := stagedPolicyStore{activePath: activePath, stagingPath: stagingPath}

	require.NoError(t, store.install(context.Background()))
	assert.FileExists(t, filepath.Join(activePath, "child", "oci-layout"))
	contents, err := os.ReadFile(filepath.Join(activePath, "oci-layout"))
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), contents)
}

func TestStagedPolicyStoreInstall_RetainsBackupWhenRestoreFails(t *testing.T) {
	cacheRoot := t.TempDir()
	activePath := filepath.Join(cacheRoot, "policy")
	stagingPath := filepath.Join(cacheRoot, "staging")
	require.NoError(t, os.MkdirAll(activePath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(activePath, "oci-layout"), []byte("old"), 0o600))
	require.NoError(t, os.MkdirAll(stagingPath, 0o700))

	store := stagedPolicyStore{
		activePath:  activePath,
		stagingPath: stagingPath,
		rename: func(oldPath, newPath string) error {
			if oldPath == stagingPath || filepath.Base(oldPath) == "previous" {
				return errors.New("rename blocked")
			}
			return os.Rename(oldPath, newPath)
		},
	}
	err := store.install(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "install staged policy store")

	backupMatches, globErr := filepath.Glob(filepath.Join(cacheRoot, ".policy-backup-*", "previous", "oci-layout"))
	require.NoError(t, globErr)
	require.Len(t, backupMatches, 1)
	contents, readErr := os.ReadFile(backupMatches[0])
	require.NoError(t, readErr)
	assert.Equal(t, []byte("old"), contents)
}

func TestStagedPolicyStoreInstall_RestoresActiveStoreWhenDescendantRestoreFails(t *testing.T) {
	cacheRoot := t.TempDir()
	activePath := filepath.Join(cacheRoot, "policy")
	stagingPath := filepath.Join(cacheRoot, "staging")
	descendantPath := filepath.Join(activePath, "child")
	require.NoError(t, os.MkdirAll(descendantPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(activePath, "oci-layout"), []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(descendantPath, "oci-layout"), []byte("old child"), 0o600))
	require.NoError(t, os.MkdirAll(stagingPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(stagingPath, "oci-layout"), []byte("new"), 0o600))

	store := stagedPolicyStore{
		activePath:  activePath,
		stagingPath: stagingPath,
		rename: func(oldPath, newPath string) error {
			if filepath.Base(oldPath) != "previous" && newPath == filepath.Join(activePath, "child") {
				return errors.New("descendant install blocked")
			}
			return os.Rename(oldPath, newPath)
		},
	}

	err := store.install(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "preserve cached descendant policy stores")
	contents, readErr := os.ReadFile(filepath.Join(activePath, "oci-layout"))
	require.NoError(t, readErr)
	assert.Equal(t, []byte("old"), contents)
	descendantContents, descendantReadErr := os.ReadFile(filepath.Join(activePath, "child", "oci-layout"))
	require.NoError(t, descendantReadErr)
	assert.Equal(t, []byte("old child"), descendantContents)
}

func TestOutermostPaths_RetainsSiblingStore(t *testing.T) {
	rootPath := t.TempDir()
	paths := []string{filepath.Join(rootPath, "policy"), filepath.Join(rootPath, "policy-child")}

	assert.Equal(t, paths, outermostPaths(paths))
}
