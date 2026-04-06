package gemara

import "sync"

// SControl wraps the generated Control with cached
// cross-reference lookups.
type SControl struct {
	Control

	referencesOnce  sync.Once
	referencesCache []string
}

// Sugar wraps this Control in a SControl for convenient
// cached helper access.
func (c Control) Sugar() *SControl {
	return &SControl{Control: c}
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
