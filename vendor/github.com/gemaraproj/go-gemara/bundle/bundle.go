// SPDX-License-Identifier: Apache-2.0

// Package bundle defines the Gemara OCI artifact format and provides
// Assemble, Pack, and Unpack operations for building, storing, and
// retrieving Gemara artifacts in OCI-compliant registries.
package bundle

const (
	// DefaultSizeLimitBytes caps bundle reads at 256 MB.
	DefaultSizeLimitBytes int64 = 256 * 1024 * 1024
)

// OCI media types for the Gemara bundle format.
// All artifact layers are YAML; the assembler and resolver expect YAML input.
const (
	// MediaTypeBundle is the OCI artifactType written into the image manifest.
	MediaTypeBundle = "application/vnd.gemara.bundle.v1"

	// MediaTypeManifest is the media type for the bundle manifest blob.
	MediaTypeManifest = "application/vnd.gemara.manifest.v1+json"

	// MediaTypeArtifact is the media type for individual Gemara YAML layers.
	MediaTypeArtifact = "application/vnd.gemara.artifact.v1+yaml"
)

// Internal annotations carried on layer descriptors.
const (
	annotationRole = "org.gemara.artifact.role"
	annotationType = "org.gemara.artifact.type"
	roleArtifact   = "artifact"
	roleImport     = "import"
)

// Manifest defines the OCI config blob stored in the bundle.
type Manifest struct {
	BundleVersion string         `json:"bundle-version"`
	GemaraVersion string         `json:"gemara-version"`
	Revision      string         `json:"revision,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Artifacts     []Artifact     `json:"artifacts,omitempty"`
}

// Empty reports whether the manifest carries no meaningful data.
func (m Manifest) Empty() bool {
	return m.BundleVersion == "" && m.GemaraVersion == "" &&
		m.Revision == "" && len(m.Metadata) == 0 &&
		len(m.Artifacts) == 0
}

// Artifact describes a single file in the bundle and its position
// in the dependency tree, similar to rootfs in an OCI image config.
type Artifact struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	ID           string   `json:"id"`
	Role         string   `json:"role"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// File is a single Gemara YAML artifact within the bundle.
type File struct {
	// Name is the bundle-relative path, e.g. "controls.yaml".
	Name string
	// Type is the Gemara artifact type, e.g. "ControlCatalog".
	Type string
	// Data is the raw YAML content of the artifact.
	Data []byte
}

// Bundle is an in-memory representation of a Gemara OCI artifact.
type Bundle struct {
	// Manifest is the OCI config blob describing the bundle contents.
	Manifest Manifest
	// Files are the primary artifact files provided as assembly sources.
	Files []File
	// Imports are the resolved transitive dependency files.
	Imports []File
	// Etag is the OCI manifest digest used for cache comparison.
	Etag string

	sizeLimitBytes int64
}

// SizeLimitBytes returns the configured size limit.
// If unset, DefaultSizeLimitBytes is returned.
func (b *Bundle) SizeLimitBytes() int64 {
	if b.sizeLimitBytes > 0 {
		return b.sizeLimitBytes
	}
	return DefaultSizeLimitBytes
}

// SetSizeLimitBytes configures the maximum allowed bundle size.
func (b *Bundle) SetSizeLimitBytes(n int64) {
	b.sizeLimitBytes = n
}

// PackOption configures Pack behaviour.
type PackOption func(*packOptions)

type packOptions struct {
	annotations map[string]string
}

// WithAnnotations adds custom annotations to the OCI manifest.
func WithAnnotations(annotations map[string]string) PackOption {
	return func(o *packOptions) {
		o.annotations = annotations
	}
}
