// SPDX-License-Identifier: Apache-2.0

// Package cachetest provides test doubles for the cache package.
// This package is intended for use in tests only.
package cachetest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	ocistore "oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/errdef"

	"github.com/complytime/complyctl/internal/registry"
)

// MockPolicySource provides an in-memory mock for testing sync operations.
// Implements cache.PolicySource by pushing OCI content directly into the
// destination store.
type MockPolicySource struct {
	mu            sync.RWMutex
	policies      map[string]*mockPolicyData
	errors        map[string]error
	LastLookupRef string
}

type mockPolicyData struct {
	digest  string
	version string
}

// NewMockPolicySource creates a mock policy source for testing
func NewMockPolicySource() *MockPolicySource {
	return &MockPolicySource{
		policies: make(map[string]*mockPolicyData),
		errors:   make(map[string]error),
	}
}

// SetError injects a custom error for a specific policy lookup. When set,
// DefinitionVersion returns this error instead of performing the normal
// lookup. This allows tests to simulate arbitrary registry failures
// (e.g., network errors) distinct from the "not found" path.
func (m *MockPolicySource) SetError(policyID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[policyID] = err
}

// SeedPolicy adds a mock policy for testing. The policy is registered under
// both the bare policyID and the versioned key (policyID:version) so that
// lookups with an explicit version tag also resolve.
func (m *MockPolicySource) SeedPolicy(policyID, version, digestStr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data := &mockPolicyData{
		digest:  digestStr,
		version: version,
	}
	m.policies[policyID] = data
	m.policies[policyID+":"+version] = data
}

// DefinitionVersion returns digest and version for a mock policy.
// The lookupRef is recorded in LastLookupRef for test assertions.
// If SetError was called for the lookupRef, the injected error is
// returned. If the policy is not seeded, registry.ErrVersionNotFound
// is returned, matching real registry behavior.
func (m *MockPolicySource) DefinitionVersion(_ context.Context, lookupRef string) (string, string, error) {
	m.mu.Lock()
	m.LastLookupRef = lookupRef
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if injected, ok := m.errors[lookupRef]; ok {
		return "", "", injected
	}

	p, ok := m.policies[lookupRef]
	if !ok {
		return "", "", fmt.Errorf("%w", registry.ErrVersionNotFound)
	}
	return p.digest, p.version, nil
}

// CopyPolicy pushes a minimal OCI manifest into the destination store for testing.
// Simulates what oras.Copy() does in production without needing a remote registry.
func (m *MockPolicySource) CopyPolicy(ctx context.Context, policyID, tag string, dst *ocistore.Store) (ocispec.Descriptor, error) {
	m.mu.RLock()
	_, ok := m.policies[policyID]
	m.mu.RUnlock()
	if !ok {
		return ocispec.Descriptor{}, fmt.Errorf("policy %s: %w", policyID, registry.ErrVersionNotFound)
	}

	configData := []byte("{}")
	configDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeEmptyJSON,
		Digest:    digest.FromBytes(configData),
		Size:      int64(len(configData)),
	}
	if err := dst.Push(ctx, configDesc, bytes.NewReader(configData)); err != nil {
		if !isDuplicateErr(err) {
			return ocispec.Descriptor{}, fmt.Errorf("failed to push complytime: %w", err)
		}
	}

	layerData := []byte(fmt.Sprintf(`{"policy_id": %q, "type": "test-layer"}`, policyID))
	layerDesc := ocispec.Descriptor{
		MediaType: "application/vnd.gemara.policy.v1+yaml",
		Digest:    digest.FromBytes(layerData),
		Size:      int64(len(layerData)),
		Annotations: map[string]string{
			ocispec.AnnotationTitle: "policies",
		},
	}
	if err := dst.Push(ctx, layerDesc, bytes.NewReader(layerData)); err != nil {
		if !isDuplicateErr(err) {
			return ocispec.Descriptor{}, fmt.Errorf("failed to push layer: %w", err)
		}
	}

	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{layerDesc},
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(manifestData),
		Size:      int64(len(manifestData)),
	}
	if err := dst.Push(ctx, manifestDesc, bytes.NewReader(manifestData)); err != nil {
		if !isDuplicateErr(err) {
			return ocispec.Descriptor{}, fmt.Errorf("failed to push manifest: %w", err)
		}
	}

	if err := dst.Tag(ctx, manifestDesc, tag); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to tag manifest: %w", err)
	}

	return manifestDesc, nil
}

// MockBundlePolicySource produces bundle-shaped OCI artifacts using
// go-gemara bundle.Pack, matching what gemara-publish-oci publishes.
type MockBundlePolicySource struct {
	mu       sync.RWMutex
	policies map[string]*mockPolicyData
	files    map[string][]bundle.File // key: policyID
}

// NewMockBundlePolicySource creates a mock source that pushes bundle artifacts.
func NewMockBundlePolicySource() *MockBundlePolicySource {
	return &MockBundlePolicySource{
		policies: make(map[string]*mockPolicyData),
		files:    make(map[string][]bundle.File),
	}
}

// SeedBundlePolicy registers a bundle-shape policy with the given files.
func (m *MockBundlePolicySource) SeedBundlePolicy(policyID, version, digestStr string, files []bundle.File) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policies[policyID] = &mockPolicyData{digest: digestStr, version: version}
	m.files[policyID] = files
}

func (m *MockBundlePolicySource) DefinitionVersion(_ context.Context, lookupRef string) (string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.policies[lookupRef]
	if !ok {
		return "", "", fmt.Errorf("policy %s not found", lookupRef)
	}
	return p.digest, p.version, nil
}

// CopyPolicy uses bundle.Pack to push a real bundle-shaped manifest into the
// destination OCI store, matching the production publish pipeline.
func (m *MockBundlePolicySource) CopyPolicy(ctx context.Context, policyID, tag string, dst *ocistore.Store) (ocispec.Descriptor, error) {
	m.mu.RLock()
	files, ok := m.files[policyID]
	m.mu.RUnlock()
	if !ok {
		return ocispec.Descriptor{}, fmt.Errorf("bundle policy %s not found in mock", policyID)
	}

	b := &bundle.Bundle{
		Manifest: bundle.Manifest{BundleVersion: "1", GemaraVersion: "v1.0.0"},
		Files:    files,
	}

	desc, err := bundle.Pack(ctx, dst, b)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("bundle pack: %w", err)
	}

	if err := dst.Tag(ctx, desc, tag); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to tag bundle manifest: %w", err)
	}

	return desc, nil
}

func isDuplicateErr(err error) bool {
	return errors.Is(err, errdef.ErrAlreadyExists)
}
