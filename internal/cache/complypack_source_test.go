// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complypack/pkg/complypack"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/memory"
)

func digestOf(data []byte) digest.Digest {
	h := sha256.Sum256(data)
	return digest.Digest(fmt.Sprintf("sha256:%x", h))
}

func TestVerifyComplypackArtifactType_AlreadyPopulated(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	// Create a real complypack artifact — Pack sets ArtifactType on the descriptor
	cfg := complypack.Config{ID: "test-pack", EvaluatorID: "test-eval", Version: "1.0.0"}
	desc, err := complypack.Pack(ctx, store, cfg, strings.NewReader("content"))
	require.NoError(t, err)
	require.Equal(t, complypack.MediaTypeArtifact, desc.ArtifactType)

	// Verify passes without fallback
	result, err := cache.VerifyComplypackArtifactType(ctx, desc, store)
	require.NoError(t, err)
	assert.Equal(t, complypack.MediaTypeArtifact, result.ArtifactType)
}

func TestVerifyComplypackArtifactType_FallbackToManifestBody(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	// Create a real complypack artifact
	cfg := complypack.Config{ID: "test-pack", EvaluatorID: "test-eval", Version: "1.0.0"}
	desc, err := complypack.Pack(ctx, store, cfg, strings.NewReader("content"))
	require.NoError(t, err)

	// Clear ArtifactType on the descriptor to simulate a registry that doesn't populate it
	desc.ArtifactType = ""

	// Verify falls back to parsing manifest body
	result, err := cache.VerifyComplypackArtifactType(ctx, desc, store)
	require.NoError(t, err)
	assert.Equal(t, complypack.MediaTypeArtifact, result.ArtifactType)
}

func TestVerifyComplypackArtifactType_WrongType(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	// Push a manifest with a wrong artifactType
	manifest := map[string]interface{}{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"artifactType":  "application/vnd.oci.image.config.v1+json",
		"config":        map[string]interface{}{"mediaType": "application/vnd.oci.empty.v1+json", "digest": "sha256:abc", "size": 2},
		"layers":        []interface{}{},
	}
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)

	desc := ocispec.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      int64(len(manifestBytes)),
	}
	desc.Digest = digestOf(manifestBytes)

	err = store.Push(ctx, desc, strings.NewReader(string(manifestBytes)))
	require.NoError(t, err)

	// Clear ArtifactType to force fallback
	desc.ArtifactType = ""

	_, err = cache.VerifyComplypackArtifactType(ctx, desc, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "artifact type mismatch")
}

func TestVerifyComplypackArtifactType_MismatchOnDescriptor(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	// Descriptor has wrong type already set — no fallback needed
	desc := ocispec.Descriptor{
		ArtifactType: "application/vnd.wrong.type",
	}

	_, err := cache.VerifyComplypackArtifactType(ctx, desc, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "artifact type mismatch")
	assert.Contains(t, err.Error(), "application/vnd.wrong.type")
}
