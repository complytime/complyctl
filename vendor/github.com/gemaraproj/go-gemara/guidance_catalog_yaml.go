// SPDX-License-Identifier: Apache-2.0

package gemara

import "github.com/gemaraproj/go-gemara/internal/codec"

// UnmarshalYAML allows decoding guidance from older/alternate YAML schemas.
// It supports:
// - `families` -> `groups`
// - `document-type` -> `type`
func (g *GuidanceCatalog) UnmarshalYAML(data []byte) error {
	type alias struct {
		Title    string              `yaml:"title"`
		Metadata Metadata            `yaml:"metadata"`
		Extends  []ArtifactMapping   `yaml:"extends,omitempty"`
		Imports  []MultiEntryMapping `yaml:"imports,omitempty"`

		Type         GuidanceType `yaml:"type,omitempty"`
		DocumentType GuidanceType `yaml:"document-type,omitempty"`

		FrontMatter string `yaml:"front-matter,omitempty"`

		Groups   []Group `yaml:"groups,omitempty"`
		Families []Group `yaml:"families,omitempty"`

		Guidelines []Guideline `yaml:"guidelines,omitempty"`
		Exemptions []Exemption `yaml:"exemptions,omitempty"`
	}

	var tmp alias
	if err := codec.UnmarshalYAML(data, &tmp); err != nil {
		return err
	}

	g.Title = tmp.Title
	g.Metadata = tmp.Metadata
	g.Extends = tmp.Extends
	g.Imports = tmp.Imports
	g.FrontMatter = tmp.FrontMatter
	g.Guidelines = tmp.Guidelines
	g.Exemptions = tmp.Exemptions

	if tmp.Type != 0 {
		g.GuidanceType = tmp.Type
	} else {
		g.GuidanceType = tmp.DocumentType
	}

	// Prefer `groups` when present, otherwise fall back to `families`.
	if len(tmp.Groups) > 0 {
		g.Groups = tmp.Groups
	} else {
		g.Groups = tmp.Families
	}

	return nil
}
