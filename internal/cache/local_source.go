// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	ocistore "oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/errdef"

	"github.com/complytime/complyctl/internal/registry"
)

// LocalPolicySource provides policy artifacts from a local OCI image layout.
type LocalPolicySource struct {
	layoutPath string
	openStore  localStoreOpener
}

var _ PolicySource = (*LocalPolicySource)(nil)

// LocalComplypackSource provides complypack artifacts from a local OCI image layout.
type LocalComplypackSource struct {
	layoutPath string
	openStore  localStoreOpener
}

var _ ComplypackSource = (*LocalComplypackSource)(nil)

type localStoreOpener func(context.Context, fs.FS) (oras.ReadOnlyTarget, error)

// NewLocalPolicySource creates a policy source for the OCI image layout at layoutPath.
func NewLocalPolicySource(layoutPath string) *LocalPolicySource {
	return newLocalPolicySource(layoutPath, openLocalStore)
}

func newLocalPolicySource(layoutPath string, openStore localStoreOpener) *LocalPolicySource {
	return &LocalPolicySource{layoutPath: layoutPath, openStore: openStore}
}

// NewLocalComplypackSource creates a complypack source for the OCI image layout at layoutPath.
func NewLocalComplypackSource(layoutPath string) *LocalComplypackSource {
	return newLocalComplypackSource(layoutPath, openLocalStore)
}

func newLocalComplypackSource(layoutPath string, openStore localStoreOpener) *LocalComplypackSource {
	return &LocalComplypackSource{layoutPath: layoutPath, openStore: openStore}
}

// DefinitionVersion resolves the configured policy digest from the local OCI image layout.
func (s *LocalPolicySource) DefinitionVersion(ctx context.Context, lookupRef string) (string, string, error) {
	return localDefinitionVersion(ctx, s.layoutPath, s.openStore, lookupRef)
}

func localDefinitionVersion(
	ctx context.Context,
	layoutPath string,
	openStore localStoreOpener,
	lookupRef string,
) (string, string, error) {
	requestedDigest, err := localReferenceDigest(lookupRef)
	if err != nil {
		return "", "", err
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}

	layoutFS := os.DirFS(layoutPath)
	indexDescriptor, err := indexManifestDescriptor(ctx, layoutFS, requestedDigest)
	if err != nil {
		return "", "", err
	}
	store, err := openSelectedLocalStore(ctx, layoutFS, indexDescriptor, openStore)
	if err != nil {
		return "", "", fmt.Errorf("open OCI layout %q: %w", layoutPath, err)
	}
	resolvedDescriptor, err := store.Resolve(ctx, requestedDigest.String())
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			return "", "", fmt.Errorf("%w: manifest %s", registry.ErrVersionNotFound, requestedDigest)
		}
		return "", "", fmt.Errorf("resolve manifest %s from OCI layout: %w", requestedDigest, err)
	}
	if resolvedDescriptor.Digest != indexDescriptor.Digest {
		return "", "", fmt.Errorf("resolved manifest digest %s does not match index descriptor %s",
			resolvedDescriptor.Digest, indexDescriptor.Digest)
	}
	if exists, err := store.Exists(ctx, indexDescriptor); err != nil {
		return "", "", fmt.Errorf("check manifest %s in OCI layout: %w", requestedDigest, err)
	} else if !exists {
		return "", "", fmt.Errorf("%w: manifest %s", registry.ErrVersionNotFound, requestedDigest)
	}

	return indexDescriptor.Digest.String(), requestedDigest.String(), nil
}

func localReferenceDigest(lookupRef string) (digest.Digest, error) {
	_, digestRef, hasDigest := strings.Cut(lookupRef, "@")
	if !hasDigest || digestRef == "" {
		return "", fmt.Errorf("local OCI layouts require a digest-pinned reference: %q", lookupRef)
	}
	requestedDigest, err := digest.Parse(digestRef)
	if err != nil {
		return "", fmt.Errorf("invalid OCI digest in local reference %q: %w", lookupRef, err)
	}
	return requestedDigest, nil
}

func indexManifestDescriptor(
	ctx context.Context,
	layoutFS fs.FS,
	requestedDigest digest.Digest,
) (ocispec.Descriptor, error) {
	indexBytes, err := readFileContext(ctx, layoutFS, ocispec.ImageIndexFile)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("read OCI layout index: %w", err)
	}

	var index ocispec.Index
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("decode OCI layout index: %w", err)
	}
	for _, descriptor := range index.Manifests {
		if descriptor.Digest == requestedDigest {
			if err := descriptor.Digest.Validate(); err != nil {
				return ocispec.Descriptor{}, fmt.Errorf("invalid manifest digest %s: %w", descriptor.Digest, err)
			}
			return descriptor, nil
		}
	}

	return ocispec.Descriptor{}, fmt.Errorf("%w: manifest %s", registry.ErrVersionNotFound, requestedDigest)
}

// CopyPolicy will copy a policy artifact from the local OCI image layout into dst.
func (s *LocalPolicySource) CopyPolicy(
	ctx context.Context,
	_ string,
	version string,
	dst *ocistore.Store,
) (ocispec.Descriptor, error) {
	return copyLocalManifest(ctx, s.layoutPath, s.openStore, version, dst)
}

func copyLocalManifest(
	ctx context.Context,
	layoutPath string,
	openStore localStoreOpener,
	version string,
	dst *ocistore.Store,
) (ocispec.Descriptor, error) {
	manifestDigest, err := digest.Parse(version)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("local OCI layouts require a digest version %q: %w", version, err)
	}
	if err := ctx.Err(); err != nil {
		return ocispec.Descriptor{}, err
	}

	store, err := openLocalManifestStore(ctx, layoutPath, manifestDigest, openStore)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("open OCI layout %q: %w", layoutPath, err)
	}

	if err := validateManifestContent(ctx, store, manifestDigest); err != nil {
		return ocispec.Descriptor{}, err
	}

	stagingPath, err := os.MkdirTemp("", "complyctl-local-policy-*")
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("create temporary OCI store: %w", err)
	}
	defer os.RemoveAll(stagingPath)
	stagingStore, err := ocistore.New(stagingPath)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("open temporary OCI store: %w", err)
	}

	descriptor, err := oras.Copy(ctx, store, manifestDigest.String(), stagingStore, manifestDigest.String(), oras.CopyOptions{})
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("copy manifest %s from OCI layout: %w", manifestDigest, err)
	}
	if err := ctx.Err(); err != nil {
		return ocispec.Descriptor{}, err
	}
	if _, err := oras.Copy(ctx, stagingStore, manifestDigest.String(), dst, manifestDigest.String(), oras.CopyOptions{}); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("copy staged manifest %s to destination: %w", manifestDigest, err)
	}
	return descriptor, nil
}

// DefinitionVersion resolves the configured complypack digest from the local OCI image layout.
func (s *LocalComplypackSource) DefinitionVersion(ctx context.Context, lookupRef string) (string, string, error) {
	return localDefinitionVersion(ctx, s.layoutPath, s.openStore, lookupRef)
}

// CopyComplypack copies a valid complypack artifact from the local OCI image layout into dst.
func (s *LocalComplypackSource) CopyComplypack(
	ctx context.Context,
	_ string,
	version string,
	dst *ocistore.Store,
) (ocispec.Descriptor, error) {
	manifestDigest, err := digest.Parse(version)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("local OCI layouts require a digest version %q: %w", version, err)
	}
	if err := ctx.Err(); err != nil {
		return ocispec.Descriptor{}, err
	}

	store, err := openLocalManifestStore(ctx, s.layoutPath, manifestDigest, s.openStore)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("open OCI layout %q: %w", s.layoutPath, err)
	}
	if err := validateManifestContent(ctx, store, manifestDigest); err != nil {
		return ocispec.Descriptor{}, err
	}

	// Validate before copying so an invalid artifact cannot leave partial content in dst.
	descriptor, err := store.Resolve(ctx, manifestDigest.String())
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("resolve complypack manifest %s from OCI layout: %w", manifestDigest, err)
	}
	if _, err := VerifyComplypackArtifactType(ctx, descriptor, store); err != nil {
		return ocispec.Descriptor{}, err
	}

	descriptor, err = oras.Copy(ctx, store, manifestDigest.String(), dst, manifestDigest.String(), oras.CopyOptions{})
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("copy complypack manifest %s from OCI layout: %w", manifestDigest, err)
	}
	return VerifyComplypackArtifactType(ctx, descriptor, dst)
}

func openLocalManifestStore(
	ctx context.Context,
	layoutPath string,
	manifestDigest digest.Digest,
	openStore localStoreOpener,
) (oras.ReadOnlyTarget, error) {
	layoutFS := os.DirFS(layoutPath)
	descriptor, err := indexManifestDescriptor(ctx, layoutFS, manifestDigest)
	if err != nil {
		return nil, err
	}
	return openSelectedLocalStore(ctx, layoutFS, descriptor, openStore)
}

func openSelectedLocalStore(
	ctx context.Context,
	layoutFS fs.FS,
	manifest ocispec.Descriptor,
	openStore localStoreOpener,
) (oras.ReadOnlyTarget, error) {
	index := ocispec.Index{Manifests: []ocispec.Descriptor{manifest}}
	indexBytes, err := json.Marshal(index)
	if err != nil {
		return nil, fmt.Errorf("encode selected OCI index: %w", err)
	}
	return openStore(ctx, selectedIndexFS{FS: layoutFS, index: indexBytes})
}

type selectedIndexFS struct {
	fs.FS
	index []byte
}

func (s selectedIndexFS) Open(name string) (fs.File, error) {
	if path.Clean(name) == ocispec.ImageIndexFile {
		return &memoryFile{Reader: strings.NewReader(string(s.index))}, nil
	}
	return s.FS.Open(name)
}

type memoryFile struct {
	*strings.Reader
}

func (f *memoryFile) Close() error { return nil }

func (f *memoryFile) Stat() (fs.FileInfo, error) {
	return nil, fs.ErrInvalid
}

func validateManifestContent(ctx context.Context, store oras.ReadOnlyTarget, manifestDigest digest.Digest) error {
	descriptor, err := store.Resolve(ctx, manifestDigest.String())
	if err != nil {
		return fmt.Errorf("resolve manifest %s from OCI layout: %w", manifestDigest, err)
	}
	if descriptor.Digest != manifestDigest {
		return fmt.Errorf("resolved manifest digest %s does not match requested digest %s",
			descriptor.Digest, manifestDigest)
	}

	manifest, err := store.Fetch(ctx, descriptor)
	if err != nil {
		return fmt.Errorf("fetch manifest %s from OCI layout: %w", manifestDigest, err)
	}
	defer manifest.Close()

	if _, err := content.ReadAll(manifest, descriptor); err != nil {
		return fmt.Errorf("verify manifest %s content digest: %w", manifestDigest, err)
	}
	return nil
}

func openLocalStore(ctx context.Context, layoutFS fs.FS) (oras.ReadOnlyTarget, error) {
	return awaitContext(ctx, func() (oras.ReadOnlyTarget, error) {
		store, err := ocistore.NewFromFS(ctx, layoutFS)
		if err != nil {
			return nil, err
		}
		return contextReadOnlyTarget{ReadOnlyTarget: store}, nil
	})
}

func readFileContext(ctx context.Context, layoutFS fs.FS, name string) ([]byte, error) {
	type readResult struct {
		contents []byte
		err      error
	}

	result := make(chan readResult, 1)
	go func() {
		contents, err := fs.ReadFile(layoutFS, name)
		result <- readResult{contents: contents, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case read := <-result:
		return read.contents, read.err
	}
}

func awaitContext[T any](ctx context.Context, operation func() (T, error)) (T, error) {
	type operationResult struct {
		value T
		err   error
	}

	var zero T
	result := make(chan operationResult, 1)
	go func() {
		value, err := operation()
		result <- operationResult{value: value, err: err}
	}()

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case operationResult := <-result:
		return operationResult.value, operationResult.err
	}
}

type contextReadOnlyTarget struct {
	oras.ReadOnlyTarget
}

func (t contextReadOnlyTarget) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	reader, err := t.ReadOnlyTarget.Fetch(ctx, target)
	if err != nil {
		return nil, err
	}
	return &contextReadCloser{ctx: ctx, ReadCloser: reader}, nil
}

type contextReadCloser struct {
	ctx context.Context
	io.ReadCloser
}

func (r *contextReadCloser) Read(buffer []byte) (int, error) {
	type readResult struct {
		count int
		err   error
		data  []byte
	}

	result := make(chan readResult, 1)
	readBuffer := make([]byte, len(buffer))
	go func() {
		count, err := r.ReadCloser.Read(readBuffer)
		result <- readResult{count: count, err: err, data: readBuffer}
	}()

	select {
	case <-r.ctx.Done():
		_ = r.Close()
		return 0, r.ctx.Err()
	case read := <-result:
		copy(buffer, read.data[:read.count])
		return read.count, read.err
	}
}
