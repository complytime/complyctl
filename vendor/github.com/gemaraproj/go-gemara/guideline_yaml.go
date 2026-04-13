// SPDX-License-Identifier: Apache-2.0

package gemara

import "github.com/gemaraproj/go-gemara/internal/codec"

// UnmarshalYAML allows decoding guidance from older/alternate YAML schemas.
// In particular, it supports using `family` instead of the struct's `group` key.
func (g *Guideline) UnmarshalYAML(data []byte) error {
	type alias struct {
		Id        string `yaml:"id"`
		Title     string `yaml:"title"`
		Objective string `yaml:"objective"`
		Group     string `yaml:"group,omitempty"`
		Family    string `yaml:"family,omitempty"`

		Recommendations []string      `yaml:"recommendations,omitempty"`
		Extends         *EntryMapping `yaml:"extends,omitempty"`
		Applicability   []string      `yaml:"applicability,omitempty"`
		Rationale       *Rationale    `yaml:"rationale,omitempty"`

		Statements []Statement         `yaml:"statements,omitempty"`
		Principles []MultiEntryMapping `yaml:"principles,omitempty"`
		Vectors    []MultiEntryMapping `yaml:"vectors,omitempty"`
		SeeAlso    []string            `yaml:"see-also,omitempty"`

		State      Lifecycle     `yaml:"state"`
		ReplacedBy *EntryMapping `yaml:"replaced-by,omitempty"`
	}

	var tmp alias
	if err := codec.UnmarshalYAML(data, &tmp); err != nil {
		return err
	}

	g.Id = tmp.Id
	g.Title = tmp.Title
	g.Objective = tmp.Objective
	if tmp.Group != "" {
		g.Group = tmp.Group
	} else {
		g.Group = tmp.Family
	}

	g.Recommendations = tmp.Recommendations
	g.Extends = tmp.Extends
	g.Applicability = tmp.Applicability
	g.Rationale = tmp.Rationale
	g.Statements = tmp.Statements
	g.Principles = tmp.Principles
	g.Vectors = tmp.Vectors
	g.SeeAlso = tmp.SeeAlso
	g.State = tmp.State
	g.ReplacedBy = tmp.ReplacedBy

	return nil
}
