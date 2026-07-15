// SPDX-License-Identifier: Apache-2.0

package cache

import (
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
