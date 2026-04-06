package gemara

import "github.com/goccy/go-yaml"

// UnmarshalYAML allows decoding guidance from older/alternate YAML schemas.
// It supports:
// - `families` -> `groups`
// - `document-type` -> `type`
func (g *GuidanceCatalog) UnmarshalYAML(data []byte) error {
	type guidanceCatalogYAML struct {
		Title    string              `yaml:"title"`
		Metadata Metadata            `yaml:"metadata"`
		Extends  []ArtifactMapping   `yaml:"extends,omitempty"`
		Imports  []MultiEntryMapping `yaml:"imports,omitempty"`

		// Current schema uses `type`, older test data uses `document-type`.
		Type         GuidanceType `yaml:"type,omitempty"`
		DocumentType GuidanceType `yaml:"document-type,omitempty"`

		FrontMatter string `yaml:"front-matter,omitempty"`

		Groups   []Group `yaml:"groups,omitempty"`
		Families []Group `yaml:"families,omitempty"`

		Guidelines []Guideline `yaml:"guidelines,omitempty"`
		Exemptions []Exemption `yaml:"exemptions,omitempty"`
	}

	var tmp guidanceCatalogYAML
	if err := yaml.Unmarshal(data, &tmp); err != nil {
		return err
	}

	g.Title = tmp.Title
	g.Metadata = tmp.Metadata
	g.Extends = tmp.Extends
	g.Imports = tmp.Imports
	g.GuidanceType = tmp.Type
	if g.GuidanceType == 0 && tmp.DocumentType != 0 {
		g.GuidanceType = tmp.DocumentType
	}
	g.FrontMatter = tmp.FrontMatter

	// Prefer `groups` when present, otherwise fall back to `families`.
	if len(tmp.Groups) > 0 {
		g.Groups = tmp.Groups
	} else {
		g.Groups = tmp.Families
	}
	g.Guidelines = tmp.Guidelines
	g.Exemptions = tmp.Exemptions

	return nil
}
