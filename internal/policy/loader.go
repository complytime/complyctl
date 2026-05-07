// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gemaraproj/go-gemara/bundle"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	ocistore "oras.land/oras-go/v2/content/oci"

	"github.com/complytime/complyctl/internal/cache"
)

// Loader reads policy artifacts from OCI Layout cache stores.
type Loader struct {
	cacheMgr *cache.Cache
}

// NewLoader creates a Loader backed by the given cache manager.
func NewLoader(cacheMgr *cache.Cache) *Loader {
	return &Loader{
		cacheMgr: cacheMgr,
	}
}

// ResolveVersion resolves a policy version against the cache. If the requested
// version is empty or "latest", it returns the last cached tag. If the exact
// version exists in cache, it is returned as-is.
func (l *Loader) ResolveVersion(policyID, configVersion string) (string, error) {
	version := configVersion
	if version == "" {
		version = "latest"
	}

	if l.PolicyExists(policyID, version) {
		return version, nil
	}

	if version == "latest" {
		cached, _ := l.GetCachedVersions(policyID)
		if len(cached) > 0 {
			return cached[len(cached)-1], nil
		}
	}

	cached, _ := l.GetCachedVersions(policyID)
	return "", fmt.Errorf(
		"policy %s@%s not in cache (run 'complyctl get' first); cached: %v",
		policyID, version, cached,
	)
}

// LoadLayerByMediaType loads a specific Gemara layer from the policy's OCI manifest
// by matching the layer descriptor's media type.
func (l *Loader) LoadLayerByMediaType(policyID, version, mediaType string) ([]byte, error) {
	if policyID == "" {
		return nil, fmt.Errorf("policy ID cannot be empty")
	}
	if version == "" {
		return nil, fmt.Errorf("version cannot be empty")
	}
	if mediaType == "" {
		return nil, fmt.Errorf("media type cannot be empty")
	}

	store, err := l.cacheMgr.NewPolicyStore(policyID)
	if err != nil {
		return nil, fmt.Errorf("policy not found in cache: %s: %w", policyID, err)
	}

	ctx := context.Background()

	manifest, err := resolveManifest(ctx, store, version)
	if err != nil {
		return nil, fmt.Errorf("policy %s@%s: %w", policyID, version, err)
	}

	for _, layer := range manifest.Layers {
		if layer.MediaType == mediaType {
			layerRC, err := store.Fetch(ctx, layer)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch layer %s: %w", mediaType, err)
			}
			data, err := io.ReadAll(layerRC)
			layerRC.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read layer %s: %w", mediaType, err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf(
		"layer with media type %s not found for policy %s@%s",
		mediaType, policyID, version,
	)
}

// isBundleManifest returns true when the OCI manifest uses the Gemara bundle
// config media type, indicating layers are differentiated by annotations
// rather than by distinct media types.
func isBundleManifest(manifest ocispec.Manifest) bool {
	return manifest.Config.MediaType == bundle.MediaTypeManifest
}

// LoadBundleFiles unpacks a Gemara bundle from the local OCI store and returns
// the file list keyed by artifact type (e.g. "Policy", "ControlCatalog").
// This is the bundle-path counterpart to LoadLayerByMediaType.
func (l *Loader) LoadBundleFiles(policyID, version string) (map[string][]byte, error) {
	if policyID == "" {
		return nil, fmt.Errorf("policy ID cannot be empty")
	}
	if version == "" {
		return nil, fmt.Errorf("version cannot be empty")
	}

	store, err := l.cacheMgr.NewPolicyStore(policyID)
	if err != nil {
		return nil, fmt.Errorf("policy not found in cache: %s: %w", policyID, err)
	}

	ctx := context.Background()
	b, err := bundle.Unpack(ctx, store, version)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack bundle for %s@%s: %w", policyID, version, err)
	}

	artTypes := make(map[string]string, len(b.Manifest.Artifacts))
	for _, a := range b.Manifest.Artifacts {
		artTypes[a.Name] = a.Type
	}

	fileType := func(f bundle.File) string {
		if f.Type != "" {
			return f.Type
		}
		return artTypes[f.Name]
	}

	files := make(map[string][]byte, len(b.Files)+len(b.Imports))
	for _, f := range b.Files {
		if t := fileType(f); t != "" {
			files[t] = f.Data
		}
	}
	for _, f := range b.Imports {
		if t := fileType(f); t != "" {
			if _, exists := files[t]; !exists {
				files[t] = f.Data
			}
		}
	}
	return files, nil
}

// DetectManifestShape opens the manifest for a cached policy and reports
// whether it is a bundle or split-layer layout.
func (l *Loader) DetectManifestShape(policyID, version string) (isBundleShape bool, err error) {
	if policyID == "" || version == "" {
		return false, fmt.Errorf("policy ID and version are required")
	}

	store, err := l.cacheMgr.NewPolicyStore(policyID)
	if err != nil {
		return false, fmt.Errorf("policy not found in cache: %s: %w", policyID, err)
	}

	manifest, err := resolveManifest(context.Background(), store, version)
	if err != nil {
		return false, err
	}

	return isBundleManifest(manifest), nil
}

// resolveManifest fetches and parses the OCI manifest for a given version.
func resolveManifest(ctx context.Context, store *ocistore.Store, version string) (ocispec.Manifest, error) {
	manifestDesc, err := store.Resolve(ctx, version)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("version not in cache: %w", err)
	}

	rc, err := store.Fetch(ctx, manifestDesc)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to parse manifest JSON: %w", err)
	}
	return manifest, nil
}

// PolicyExists reports whether a policy with the given ID and version exists in the cache.
func (l *Loader) PolicyExists(policyID, version string) bool {
	if policyID == "" || version == "" {
		return false
	}

	store, err := l.cacheMgr.NewPolicyStore(policyID)
	if err != nil {
		return false
	}

	ctx := context.Background()
	_, err = store.Resolve(ctx, version)
	return err == nil
}

// GetCachedVersions returns all cached version tags for the given policy.
func (l *Loader) GetCachedVersions(policyID string) ([]string, error) {
	if !l.cacheMgr.PolicyStoreExists(policyID) {
		return []string{}, nil
	}

	store, err := l.cacheMgr.NewPolicyStore(policyID)
	if err != nil {
		return nil, fmt.Errorf("failed to open store for policy %s: %w", policyID, err)
	}

	ctx := context.Background()
	var versions []string
	err = store.Tags(ctx, "", func(tags []string) error {
		versions = append(versions, tags...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list tags for policy %s: %w", policyID, err)
	}

	return versions, nil
}

// ListCachedPolicies returns a map of all cached policy IDs to their cached version tags.
func (l *Loader) ListCachedPolicies() (map[string][]string, error) {
	policyIDs, err := l.cacheMgr.ListPolicies()
	if err != nil {
		return nil, fmt.Errorf("failed to list cached policies: %w", err)
	}

	result := make(map[string][]string)
	for _, policyID := range policyIDs {
		versions, err := l.GetCachedVersions(policyID)
		if err != nil {
			return nil, fmt.Errorf("failed to get versions for policy %s: %w", policyID, err)
		}
		if len(versions) > 0 {
			result[policyID] = versions
		}
	}
	return result, nil
}
