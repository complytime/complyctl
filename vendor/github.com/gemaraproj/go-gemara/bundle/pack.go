// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

// Pack writes the Bundle as OCI layers into target and returns the
// manifest descriptor. The caller is responsible for tagging or copying
// the result to a registry.
func Pack(ctx context.Context, target oras.Target, b *Bundle, opts ...PackOption) (ocispec.Descriptor, error) {
	if b == nil {
		return ocispec.Descriptor{}, fmt.Errorf("bundle must not be nil")
	}
	if len(b.Files) == 0 {
		return ocispec.Descriptor{}, fmt.Errorf("bundle must contain at least one artifact file")
	}

	o := &packOptions{}
	for _, opt := range opts {
		opt(o)
	}

	manifestData, err := json.Marshal(b.Manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshaling manifest: %w", err)
	}
	manifestDesc := descFromBytes(MediaTypeManifest, manifestData)
	if err := pushBlob(ctx, target, manifestDesc, manifestData); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing manifest: %w", err)
	}

	layers := make([]ocispec.Descriptor, 0, len(b.Files)+len(b.Imports))
	for _, f := range b.Files {
		desc, err := pushLayer(ctx, target, f, roleArtifact)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		layers = append(layers, desc)
	}
	for _, f := range b.Imports {
		desc, err := pushLayer(ctx, target, f, roleImport)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		layers = append(layers, desc)
	}

	return oras.PackManifest(ctx, target, oras.PackManifestVersion1_1, MediaTypeBundle, oras.PackManifestOptions{
		ConfigDescriptor:    &manifestDesc,
		Layers:              layers,
		ManifestAnnotations: o.annotations,
	})
}

func pushLayer(ctx context.Context, target content.Pusher, f File, role string) (ocispec.Descriptor, error) {
	desc := descFromBytes(MediaTypeArtifact, f.Data)
	desc.Annotations = map[string]string{
		ocispec.AnnotationTitle: f.Name,
		annotationRole:          role,
	}
	if f.Type != "" {
		desc.Annotations[annotationType] = f.Type
	}
	if err := pushBlob(ctx, target, desc, f.Data); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing %s %s: %w", role, f.Name, err)
	}
	return desc, nil
}

func pushBlob(ctx context.Context, target content.Pusher, desc ocispec.Descriptor, data []byte) error {
	err := target.Push(ctx, desc, bytes.NewReader(data))
	if err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return err
	}
	return nil
}

func descFromBytes(mediaType string, data []byte) ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}
}
