// SPDX-License-Identifier: Apache-2.0

package complypack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

const (
	// MaxContentSize is the maximum allowed content size (100MB).
	// Prevents memory exhaustion attacks via unbounded content.
	MaxContentSize = 100 * 1024 * 1024
)

// Pack assembles a ComplyPack OCI artifact from config and opaque content.
// The content is stored as a single layer with MediaTypeContent.
// The config is stored with MediaTypeConfig.
//
// Memory Usage: Pack loads the entire content into memory for digest calculation.
// Content size is limited to MaxContentSize (100MB) to prevent memory exhaustion.
// Returns ErrContentTooLarge if content exceeds this limit.
//
// Options:
//   - WithAnnotations(map) adds OCI manifest annotations
//
// Returns the OCI manifest descriptor pointing to the packed artifact.
func Pack(ctx context.Context, store content.Storage, cfg Config, content io.Reader, opts ...PackOption) (ocispec.Descriptor, error) {
	// Validate config
	if err := cfg.Validate(); err != nil {
		return ocispec.Descriptor{}, err
	}

	// Apply options
	options := &packOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Read content with size limit to prevent memory exhaustion
	limitedReader := io.LimitReader(content, MaxContentSize+1)
	contentBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("reading content: %w", err)
	}
	if len(contentBytes) == 0 {
		return ocispec.Descriptor{}, ErrEmptyContent
	}
	if int64(len(contentBytes)) > MaxContentSize {
		return ocispec.Descriptor{}, fmt.Errorf("%w: content size %d exceeds maximum %d bytes",
			ErrContentTooLarge, len(contentBytes), MaxContentSize)
	}

	// Push config blob
	configData, err := json.Marshal(cfg)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshaling config: %w", err)
	}
	configDesc, err := pushBlob(ctx, store, MediaTypeConfig, configData)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing config blob: %w", err)
	}

	// Push content blob
	contentDesc, err := pushBlob(ctx, store, MediaTypeContent, contentBytes)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing content blob: %w", err)
	}

	// Build manifest annotations
	annotations := make(map[string]string)
	if options.annotations != nil {
		for k, v := range options.annotations {
			annotations[k] = v
		}
	}
	// Add evaluator-id annotation for discoverability
	annotations["complypack.evaluator-id"] = cfg.EvaluatorID

	// Pack manifest
	manifestDesc, err := oras.PackManifest(ctx, store,
		oras.PackManifestVersion1_1,
		MediaTypeArtifact,
		oras.PackManifestOptions{
			ConfigDescriptor:    &configDesc,
			Layers:              []ocispec.Descriptor{contentDesc},
			ManifestAnnotations: annotations,
		})
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("packing manifest: %w", err)
	}

	return manifestDesc, nil
}

// pushBlob pushes a blob to the store and returns its descriptor.
// Ignores ErrAlreadyExists since content-addressable storage is idempotent.
func pushBlob(ctx context.Context, store content.Storage, mediaType string, data []byte) (ocispec.Descriptor, error) {
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}

	err := store.Push(ctx, desc, bytes.NewReader(data))
	if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return ocispec.Descriptor{}, err
	}

	return desc, nil
}
