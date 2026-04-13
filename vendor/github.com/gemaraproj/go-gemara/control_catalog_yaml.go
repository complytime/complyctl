// SPDX-License-Identifier: Apache-2.0

package gemara

import "github.com/gemaraproj/go-gemara/internal/codec"

// UnmarshalYAML allows decoding control catalogs from older/alternate YAML schemas.
// It supports mapping `families` -> `groups`.
func (c *ControlCatalog) UnmarshalYAML(data []byte) error {
	type alias struct {
		Title    string   `yaml:"title"`
		Metadata Metadata `yaml:"metadata"`

		Controls []Control `yaml:"controls,omitempty"`

		Groups   []Group `yaml:"groups,omitempty"`
		Families []Group `yaml:"families,omitempty"`

		Extends []ArtifactMapping   `yaml:"extends,omitempty"`
		Imports []MultiEntryMapping `yaml:"imports,omitempty"`
	}

	var tmp alias
	if err := codec.UnmarshalYAML(data, &tmp); err != nil {
		return err
	}

	c.Title = tmp.Title
	c.Metadata = tmp.Metadata
	c.Controls = tmp.Controls
	c.Extends = tmp.Extends
	c.Imports = tmp.Imports

	if len(tmp.Groups) > 0 {
		c.Groups = tmp.Groups
	} else {
		c.Groups = tmp.Families
	}

	return nil
}
