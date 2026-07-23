// SPDX-License-Identifier: Apache-2.0

package complypack

import (
	"fmt"

	"github.com/complytime/complypack/schemas/jsonschema"
)

// Config is the ComplyPack OCI artifact configuration.
// Embedded in the OCI config layer so consumers can identify the pack
// and route it to the correct provider without inspecting the content.
type Config struct {
	// ID is the globally unique pack identifier using reverse-domain convention
	// (e.g., "io.complytime.my-controls", "com.acme.appsec").
	// Required. Survives registry moves and distinguishes packs from different
	// authors that target the same evaluator.
	ID string `json:"id"`

	// EvaluatorID identifies the provider plugin that evaluates this pack's
	// content (e.g., "opa"). Must match the provider's registered ID.
	// Required. Used by complyctl to dispatch content to the correct provider.
	EvaluatorID string `json:"evaluator-id"`

	// Version is the ComplyPack artifact version.
	// Required. Semantic versioning recommended.
	Version string `json:"version"`

	// Source links this ComplyPack to the Gemara content it implements.
	// Optional. Nil for standalone policies.
	Source *Provenance `json:"source,omitempty"`
}

// Provenance links a ComplyPack to the Gemara content and policy it implements.
type Provenance struct {
	// GemaraContent is the URI or hash of the Gemara catalog.
	// Examples: "oci://registry/gemara/controls:latest", "sha256:abc123..."
	GemaraContent string `json:"gemara-content"`

	// PolicyID identifies the policy within the Gemara catalog.
	PolicyID string `json:"policy-id"`
}

// Validate checks that required Config fields are present and well-formed.
// Format patterns are derived from the embedded JSON Schema — same source
// of truth used by the YAML config layer.
// Returns ErrInvalidConfig if validation fails.
func (c Config) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidConfig)
	}
	if !jsonschema.IDPattern().MatchString(c.ID) {
		return fmt.Errorf("%w: id %q must use reverse-domain notation (e.g. io.complytime.my-pack)", ErrInvalidConfig, c.ID)
	}
	if c.EvaluatorID == "" {
		return fmt.Errorf("%w: evaluator-id is required", ErrInvalidConfig)
	}
	if !jsonschema.EvaluatorIDPattern().MatchString(c.EvaluatorID) {
		return fmt.Errorf("%w: evaluator-id %q must be a lowercase identifier (e.g. opa)", ErrInvalidConfig, c.EvaluatorID)
	}
	if c.Version == "" {
		return fmt.Errorf("%w: version is required", ErrInvalidConfig)
	}
	if !jsonschema.VersionPattern().MatchString(c.Version) {
		return fmt.Errorf("%w: version %q must be semver (e.g. 1.0.0)", ErrInvalidConfig, c.Version)
	}
	if c.Source != nil {
		if c.Source.GemaraContent == "" {
			return fmt.Errorf("%w: source.gemara-content is required when source is set", ErrInvalidConfig)
		}
		if c.Source.PolicyID == "" {
			return fmt.Errorf("%w: source.policy-id is required when source is set", ErrInvalidConfig)
		}
	}
	return nil
}
