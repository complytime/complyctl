package gemara

import (
	"sync"

	"github.com/gemaraproj/go-gemara/internal/codec"
)

// SControlCatalog wraps a ControlCatalog with pre-built indexes for
// efficient group, control, and requirement lookups.
type SControlCatalog struct {
	ControlCatalog

	groupsOnce  sync.Once
	groupsCache []string

	sugarControlsOnce  sync.Once
	sugarControlsCache []*SControl

	controlsByGroupOnce  sync.Once
	controlsByGroupCache map[string][]*SControl

	requirementsOnce  sync.Once
	requirementsCache map[string][]AssessmentRequirement
}

// Sugar wraps this ControlCatalog in a SControlCatalog for convenient
// cached helper access. Cached results are computed once on first access
// and never invalidated, so the wrapper should not be reused after the
// underlying data has changed. Call Sugar again or use FromBase to reset
// caches.
func (c *ControlCatalog) Sugar() *SControlCatalog {
	return &SControlCatalog{ControlCatalog: *c}
}

func (c *SControlCatalog) ToBase() ControlCatalog {
	return c.ControlCatalog
}

func (c *SControlCatalog) FromBase(s *ControlCatalog) {
	c.ControlCatalog = *s
	c.groupsOnce = sync.Once{}
	c.groupsCache = nil
	c.sugarControlsOnce = sync.Once{}
	c.sugarControlsCache = nil
	c.controlsByGroupOnce = sync.Once{}
	c.controlsByGroupCache = nil
	c.requirementsOnce = sync.Once{}
	c.requirementsCache = nil
}

func (c *SControlCatalog) MarshalYAML() ([]byte, error) {
	return codec.MarshalBaseYAML[ControlCatalog](c)
}

func (c *SControlCatalog) UnmarshalYAML(data []byte) error {
	return codec.UnmarshalBaseYAML[ControlCatalog](data, c)
}

// SControls returns all controls as cached SControl instances.
func (c *SControlCatalog) SControls() []*SControl {
	c.sugarControlsOnce.Do(func() {
		c.sugarControlsCache = make([]*SControl, len(c.Controls))
		for i := range c.Controls {
			c.sugarControlsCache[i] = c.Controls[i].Sugar()
		}
	})
	return c.sugarControlsCache
}

func (c *SControlCatalog) GetGroupNames() []string {
	c.groupsOnce.Do(func() {
		for _, group := range c.Groups {
			c.groupsCache = append(c.groupsCache, group.Title)
		}
	})
	return c.groupsCache
}

func (c *SControlCatalog) GetControlsForGroup(group string) []*SControl {
	c.controlsByGroupOnce.Do(func() {
		c.controlsByGroupCache = make(map[string][]*SControl)
		for _, sc := range c.SControls() {
			c.controlsByGroupCache[sc.Group] = append(
				c.controlsByGroupCache[sc.Group], sc,
			)
		}
	})
	return c.controlsByGroupCache[group]
}

func (c *SControlCatalog) GetRequirementForApplicability(applicability string) []AssessmentRequirement {
	c.requirementsOnce.Do(func() {
		c.requirementsCache = make(map[string][]AssessmentRequirement)
		for _, control := range c.Controls {
			for _, req := range control.AssessmentRequirements {
				for _, app := range req.Applicability {
					c.requirementsCache[app] = append(
						c.requirementsCache[app], req,
					)
				}
			}
		}
	})
	return c.requirementsCache[applicability]
}
