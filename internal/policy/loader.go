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

const (
	artifactTypePolicy   = "Policy"
	artifactTypeControl  = "ControlCatalog"
	artifactTypeGuidance = "GuidanceCatalog"
)

// BundleArtifacts preserves every source and import in a Gemara bundle.
type BundleArtifacts struct {
	Source  bundle.File
	Imports []bundle.File
}

// FilesByType returns the source followed by imports matching typ, in bundle order.
func (a BundleArtifacts) FilesByType(typ string) []bundle.File {
	var files []bundle.File
	if a.Source.Type == typ {
		files = append(files, a.Source)
	}
	return append(files, a.ImportsByType(typ)...)
}

// ImportsByType returns only imports matching typ, in bundle order.
func (a BundleArtifacts) ImportsByType(typ string) []bundle.File {
	var files []bundle.File
	for _, file := range a.Imports {
		if file.Type == typ {
			files = append(files, file)
		}
	}
	return files
}

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

// LoadBundleArtifacts loads every source and import for a cached policy version,
// recovering missing file types from the bundle manifest without changing raw data.
// It returns an error for empty inputs, inaccessible stores, or invalid bundles.
func (l *Loader) LoadBundleArtifacts(policyID, version string) (BundleArtifacts, error) {
	b, err := l.unpackBundle(policyID, version)
	if err != nil {
		return BundleArtifacts{}, err
	}
	artTypes := make(map[string]string, len(b.Manifest.Artifacts))
	for _, artifact := range b.Manifest.Artifacts {
		artTypes[artifact.Name] = artifact.Type
	}
	normalize := func(file bundle.File) bundle.File {
		if file.Type == "" {
			file.Type = artTypes[file.Name]
		}
		return file
	}
	artifacts := BundleArtifacts{Source: normalize(b.Source)}
	for _, file := range b.Imports {
		artifacts.Imports = append(artifacts.Imports, normalize(file))
	}
	return artifacts, nil
}

func (l *Loader) unpackBundle(policyID, version string) (*bundle.Bundle, error) {
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
	return b, nil
}

// LoadBundleFiles returns the first file of each artifact type, source first.
// It is a compatibility wrapper around LoadBundleArtifacts and returns the same errors.
func (l *Loader) LoadBundleFiles(policyID, version string) (map[string][]byte, error) {
	artifacts, err := l.LoadBundleArtifacts(policyID, version)
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte, 1+len(artifacts.Imports))
	if artifacts.Source.Type != "" {
		files[artifacts.Source.Type] = artifacts.Source.Data
	}
	for _, file := range artifacts.Imports {
		if file.Type == "" {
			continue
		}
		if _, exists := files[file.Type]; !exists {
			files[file.Type] = file.Data
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
