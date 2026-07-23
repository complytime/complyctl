// SPDX-License-Identifier: Apache-2.0

package complypack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// ComplyPack is an unpacked ComplyPack artifact with its config and content.
type ComplyPack struct {
	// Config contains the evaluator-id, version, and optional provenance.
	Config Config

	// Content is the opaque policy content (e.g., OPA bundle tarball).
	// Caller must Close() when done.
	Content io.ReadCloser
}

// Unpack extracts a ComplyPack's config and content from an OCI store.
// The descriptor must point to an OCI manifest with a ComplyPack config layer.
//
// IMPORTANT: The returned ComplyPack.Content is an io.ReadCloser that MUST be
// closed by the caller to avoid resource leaks:
//
//	result, err := complypack.Unpack(ctx, store, desc)
//	if err != nil { return err }
//	defer result.Content.Close()  // Required!
//
// Returns ComplyPack with config and content reader. Caller must close Content.
func Unpack(ctx context.Context, store content.ReadOnlyStorage, desc ocispec.Descriptor) (*ComplyPack, error) {
	// Fetch manifest
	manifestData, err := content.FetchAll(ctx, store, desc)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshaling manifest: %w", err)
	}

	// Verify config media type
	if manifest.Config.MediaType != MediaTypeConfig {
		return nil, fmt.Errorf("%w: config media type %q, want %q",
			ErrInvalidMediaType, manifest.Config.MediaType, MediaTypeConfig)
	}

	// Fetch and parse config
	configData, err := content.FetchAll(ctx, store, manifest.Config)
	if err != nil {
		return nil, fmt.Errorf("fetching config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Find content layer
	var contentDesc *ocispec.Descriptor
	for i := range manifest.Layers {
		if manifest.Layers[i].MediaType == MediaTypeContent {
			contentDesc = &manifest.Layers[i]
			break
		}
	}
	if contentDesc == nil {
		return nil, ErrNoContentLayer
	}

	// Fetch content (return as ReadCloser for streaming)
	contentReader, err := store.Fetch(ctx, *contentDesc)
	if err != nil {
		return nil, fmt.Errorf("fetching content layer: %w", err)
	}

	return &ComplyPack{
		Config:  cfg,
		Content: contentReader,
	}, nil
}
