package gemara

import (
	"sync"

	"github.com/gemaraproj/go-gemara/internal/codec"
)

// SControl wraps a Control with cached cross-reference lookups.
type SControl struct {
	Control

	referencesOnce  sync.Once
	referencesCache []string
}

// Sugar wraps this Control in a SControl for convenient cached helper
// access. Cached results are computed once on first access and never
// invalidated, so the wrapper should not be reused after the underlying
// data has changed. Call Sugar again or use FromBase to reset caches.
func (c *Control) Sugar() *SControl {
	return &SControl{Control: *c}
}

func (c *SControl) ToBase() Control {
	return c.Control
}

func (c *SControl) FromBase(s *Control) {
	c.Control = *s
	c.referencesOnce = sync.Once{}
	c.referencesCache = nil
}

func (c *SControl) MarshalYAML() ([]byte, error) {
	return codec.MarshalBaseYAML[Control](c)
}

func (c *SControl) UnmarshalYAML(data []byte) error {
	return codec.UnmarshalBaseYAML[Control](data, c)
}

func (c *SControl) GetMappingReferences() []string {
	c.referencesOnce.Do(func() {
		for _, ref := range c.Guidelines {
			c.referencesCache = append(c.referencesCache, ref.ReferenceId)
		}
		for _, ref := range c.Threats {
			c.referencesCache = append(c.referencesCache, ref.ReferenceId)
		}
	})
	return c.referencesCache
}
