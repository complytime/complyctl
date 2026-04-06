// SPDX-License-Identifier: Apache-2.0

package gemara

import "github.com/goccy/go-yaml"

// UnmarshalYAML allows decoding controls from older/alternate YAML schemas.
// In particular, it supports using `family` instead of the struct's `group` key.
func (c *Control) UnmarshalYAML(data []byte) error {
	type controlYAML struct {
		Id        string `yaml:"id"`
		Title     string `yaml:"title"`
		Objective string `yaml:"objective"`
		Group     string `yaml:"group,omitempty"`
		Family    string `yaml:"family,omitempty"`

		AssessmentRequirements []AssessmentRequirement `yaml:"assessment-requirements,omitempty"`

		Guidelines []MultiEntryMapping `yaml:"guidelines,omitempty"`
		Threats    []MultiEntryMapping `yaml:"threats,omitempty"`

		State      Lifecycle     `yaml:"state"`
		ReplacedBy *EntryMapping `yaml:"replaced-by,omitempty"`
	}

	var tmp controlYAML
	if err := yaml.Unmarshal(data, &tmp); err != nil {
		return err
	}

	c.Id = tmp.Id
	c.Title = tmp.Title
	c.Objective = tmp.Objective
	if tmp.Group != "" {
		c.Group = tmp.Group
	} else {
		c.Group = tmp.Family
	}

	c.AssessmentRequirements = tmp.AssessmentRequirements
	c.Guidelines = tmp.Guidelines
	c.Threats = tmp.Threats
	c.State = tmp.State
	c.ReplacedBy = tmp.ReplacedBy

	return nil
}
