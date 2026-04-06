// SPDX-License-Identifier: Apache-2.0

package gemara

import "github.com/goccy/go-yaml"

// UnmarshalYAML allows decoding control catalogs from older/alternate YAML schemas.
// It supports mapping `families` -> `groups`.
func (c *ControlCatalog) UnmarshalYAML(data []byte) error {
	type controlCatalogYAML struct {
		Groups   []Group `yaml:"groups,omitempty"`
		Families []Group `yaml:"families,omitempty"`

		Title    string   `yaml:"title"`
		Metadata Metadata `yaml:"metadata"`

		Extends []ArtifactMapping   `yaml:"extends,omitempty"`
		Imports []MultiEntryMapping `yaml:"imports,omitempty"`

		Controls []Control `yaml:"controls,omitempty"`
	}

	var tmp controlCatalogYAML
	if err := yaml.Unmarshal(data, &tmp); err != nil {
		return err
	}

	c.Groups = tmp.Groups
	if len(c.Groups) == 0 {
		c.Groups = tmp.Families
	}
	c.Controls = tmp.Controls

	c.Title = tmp.Title
	c.Metadata = tmp.Metadata
	c.Extends = tmp.Extends
	c.Imports = tmp.Imports

	return nil
}
