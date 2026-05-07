// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
)

// Unpack reads the OCI artifact at ref from target and returns a Bundle.
// The bundle's Etag is set to the hex digest of the resolved manifest.
func Unpack(ctx context.Context, target oras.ReadOnlyTarget, ref string) (*Bundle, error) {
	manifestDesc, err := target.Resolve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("resolving %q: %w", ref, err)
	}

	manifestData, err := fetchAll(ctx, target, manifestDesc)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}

	var ociManifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &ociManifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	var m Manifest
	if ociManifest.Config.MediaType == MediaTypeManifest {
		configData, err := fetchAll(ctx, target, ociManifest.Config)
		if err != nil {
			return nil, fmt.Errorf("fetching bundle manifest: %w", err)
		}
		if err := json.Unmarshal(configData, &m); err != nil {
			return nil, fmt.Errorf("parsing bundle manifest: %w", err)
		}
	}

	b := &Bundle{
		Manifest: m,
		Etag:     manifestDesc.Digest.Hex(),
	}

	for _, layerDesc := range ociManifest.Layers {
		if layerDesc.MediaType != MediaTypeArtifact {
			continue
		}

		data, err := fetchAll(ctx, target, layerDesc)
		if err != nil {
			return nil, fmt.Errorf("fetching layer %s: %w", layerDesc.Digest, err)
		}

		f := File{
			Name: layerDesc.Annotations[ocispec.AnnotationTitle],
			Type: layerDesc.Annotations[annotationType],
			Data: data,
		}

		switch layerDesc.Annotations[annotationRole] {
		case roleImport:
			b.Imports = append(b.Imports, f)
		default:
			b.Files = append(b.Files, f)
		}
	}

	return b, nil
}

func fetchAll(ctx context.Context, target oras.ReadOnlyTarget, desc ocispec.Descriptor) ([]byte, error) {
	rc, err := target.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer rc.Close() //nolint:errcheck
	return io.ReadAll(rc)
}
