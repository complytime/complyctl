// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
	defer func() {
		if backupPath != "" {
			_ = os.RemoveAll(backupPath) // A stale backup does not affect the installed cache.
		}
	}()

	if backupPath != "" {
		if err := os.Rename(s.activePath, policyStoreBackupContentPath(backupPath)); err != nil {
			return fmt.Errorf("back up existing policy store: %w", err)
		}
	}

	if err := ctx.Err(); err != nil {
		return s.restoreBackup(backupPath, err)
	}
	if err := os.Rename(s.stagingPath, s.activePath); err != nil {
		return s.restoreBackup(backupPath, fmt.Errorf("install staged policy store: %w", err))
	}

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

func (s *stagedPolicyStore) restoreBackup(backupPath string, installErr error) error {
	if backupPath == "" {
		return installErr
	}
	if err := os.Rename(policyStoreBackupContentPath(backupPath), s.activePath); err != nil {
		return fmt.Errorf("%w; restore previous policy store: %w", installErr, err)
	}
	return installErr
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
