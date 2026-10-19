// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/complytime/complyctl/internal/complytime"
	"oras.land/oras-go/v2/content/oci"
)

// Cache manages per-policy OCI Layout stores under the XDG cache directory
// (default ~/.cache/complytime/policies/{policy-id}/).
type Cache struct {
	cacheDir string
}

// stagedPolicyStore holds an OCI store that can replace a policy cache after a
// complete transfer. Keeping it next to the active store makes the rename
// operation stay on the same filesystem.
type stagedPolicyStore struct {
	activePath  string
	stagingPath string
	rename      func(string, string) error
}

func NewCache(cacheDir string) *Cache {
	return &Cache{
		cacheDir: cacheDir,
	}
}

func (c *Cache) PolicyStorePath(policyID string) string {
	return filepath.Join(c.cacheDir, complytime.PoliciesSubdir, policyID)
}

// NewPolicyStore creates or opens an OCI Layout store for the given policy ID.
func (c *Cache) NewPolicyStore(policyID string) (*oci.Store, error) {
	storePath := c.PolicyStorePath(policyID)
	if err := os.MkdirAll(storePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create policy store directory %s: %w", storePath, err)
	}

	store, err := oci.New(storePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open OCI Layout store for policy %s: %w", policyID, err)
	}

	return store, nil
}

func (c *Cache) newStagedPolicyStore(policyID string) (*oci.Store, *stagedPolicyStore, error) {
	activePath := c.PolicyStorePath(policyID)
	parentPath := filepath.Dir(activePath)
	if err := os.MkdirAll(parentPath, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create policy store parent directory %s: %w", parentPath, err)
	}

	stagingPath, err := os.MkdirTemp(parentPath, ".policy-staging-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create policy store staging directory: %w", err)
	}

	store, err := oci.New(stagingPath)
	if err != nil {
		_ = os.RemoveAll(stagingPath) // Best effort cleanup after a failed staging-store creation.
		return nil, nil, fmt.Errorf("open staging OCI Layout store for policy %s: %w", policyID, err)
	}

	return store, &stagedPolicyStore{activePath: activePath, stagingPath: stagingPath}, nil
}

func (s *stagedPolicyStore) discard() {
	_ = os.RemoveAll(s.stagingPath) // The transfer error is more useful than a temporary-directory cleanup error.
}

func (s *stagedPolicyStore) install(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	backupPath, err := policyStoreBackupPath(s.activePath)
	if err != nil {
		return err
	}
	backupRetained := false
	defer func() {
		if backupPath != "" && !backupRetained {
			_ = os.RemoveAll(backupPath) // A stale backup does not affect the installed cache.
		}
	}()

	if backupPath != "" {
		if err := s.renamePath(s.activePath, policyStoreBackupContentPath(backupPath)); err != nil {
			return fmt.Errorf("back up existing policy store: %w", err)
		}
		backupRetained = true
	}

	if err := ctx.Err(); err != nil {
		restored, restoreErr := s.restoreBackup(backupPath, err)
		backupRetained = !restored
		return restoreErr
	}
	if err := s.renamePath(s.stagingPath, s.activePath); err != nil {
		restored, restoreErr := s.restoreBackup(backupPath, fmt.Errorf("install staged policy store: %w", err))
		backupRetained = !restored
		return restoreErr
	}
	if err := s.restoreDescendantStores(backupPath); err != nil {
		restored, restoreErr := s.restoreBackup(backupPath, fmt.Errorf(
			"preserve cached descendant policy stores: %w", err,
		))
		backupRetained = !restored
		return restoreErr
	}
	backupRetained = false

	return nil
}

func policyStoreBackupPath(activePath string) (string, error) {
	if _, err := os.Stat(activePath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("check existing policy store: %w", err)
	}

	backupPath, err := os.MkdirTemp(filepath.Dir(activePath), ".policy-backup-*")
	if err != nil {
		return "", fmt.Errorf("create policy store backup directory: %w", err)
	}
	return backupPath, nil
}

func policyStoreBackupContentPath(backupPath string) string {
	return filepath.Join(backupPath, "previous")
}

func (s *stagedPolicyStore) restoreBackup(backupPath string, installErr error) (bool, error) {
	if backupPath == "" {
		return true, installErr
	}
	if err := os.RemoveAll(s.activePath); err != nil {
		return false, fmt.Errorf("%w; remove incomplete policy store: %w", installErr, err)
	}
	if err := s.renamePath(policyStoreBackupContentPath(backupPath), s.activePath); err != nil {
		return false, fmt.Errorf("%w; restore previous policy store: %w", installErr, err)
	}
	return true, installErr
}

func (s *stagedPolicyStore) restoreDescendantStores(backupPath string) error {
	if backupPath == "" {
		return nil
	}

	descendants, err := policyStoreDescendants(policyStoreBackupContentPath(backupPath))
	if err != nil {
		return err
	}
	backupContentPath := policyStoreBackupContentPath(backupPath)
	for _, descendant := range descendants {
		if err := s.restoreDescendantStore(backupContentPath, descendant); err != nil {
			return err
		}
	}
	return nil
}

func policyStoreDescendants(backupContentPath string) ([]string, error) {
	var descendants []string
	err := filepath.WalkDir(backupContentPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Name() != "oci-layout" || entry.IsDir() {
			return nil
		}
		storePath := filepath.Dir(path)
		if storePath == backupContentPath {
			return nil
		}
		descendants = append(descendants, storePath)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find cached descendant policy stores: %w", err)
	}

	sort.Strings(descendants)
	return outermostPaths(descendants), nil
}

func outermostPaths(paths []string) []string {
	var outermost []string
	for _, candidate := range paths {
		isDescendant := false
		for _, parent := range outermost {
			relativePath, err := filepath.Rel(parent, candidate)
			if err == nil && relativePath != "." && relativePath != ".." &&
				!strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) &&
				!filepath.IsAbs(relativePath) {
				isDescendant = true
				break
			}
		}
		if !isDescendant {
			outermost = append(outermost, candidate)
		}
	}
	return outermost
}

func (s *stagedPolicyStore) restoreDescendantStore(backupContentPath, sourcePath string) error {
	relativePath, err := filepath.Rel(backupContentPath, sourcePath)
	if err != nil {
		return fmt.Errorf("resolve cached descendant policy store path: %w", err)
	}
	targetPath := filepath.Join(s.activePath, relativePath)
	temporaryParent := filepath.Dir(targetPath)
	if err := os.MkdirAll(temporaryParent, 0o700); err != nil {
		return fmt.Errorf("create cached descendant policy store parent: %w", err)
	}
	temporaryPath, err := os.MkdirTemp(temporaryParent, ".policy-descendant-staging-*")
	if err != nil {
		return fmt.Errorf("create cached descendant policy store staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(temporaryPath) }()

	if err := os.CopyFS(temporaryPath, os.DirFS(sourcePath)); err != nil {
		return fmt.Errorf("copy cached descendant policy store %s: %w", relativePath, err)
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return fmt.Errorf("replace cached descendant policy store %s: %w", relativePath, err)
	}
	if err := s.renamePath(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("install cached descendant policy store %s: %w", relativePath, err)
	}
	return nil
}

func (s *stagedPolicyStore) renamePath(oldPath, newPath string) error {
	if s.rename != nil {
		return s.rename(oldPath, newPath)
	}
	return os.Rename(oldPath, newPath)
}

func (c *Cache) PolicyStoreExists(policyID string) bool {
	markerPath := filepath.Join(c.PolicyStorePath(policyID), "oci-layout")
	_, err := os.Stat(markerPath)
	return err == nil
}

func (c *Cache) ListPolicies() ([]string, error) {
	policiesDir := filepath.Join(c.cacheDir, complytime.PoliciesSubdir)
	if _, err := os.Stat(policiesDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	var policies []string
	err := filepath.WalkDir(policiesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == "oci-layout" && !d.IsDir() {
			storeDir := filepath.Dir(path)
			rel, relErr := filepath.Rel(policiesDir, storeDir)
			if relErr != nil {
				return nil
			}
			policies = append(policies, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk policies directory: %w", err)
	}
	return policies, nil
}

func (c *Cache) Dir() string {
	return c.cacheDir
}
