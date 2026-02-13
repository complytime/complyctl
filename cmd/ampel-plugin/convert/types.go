package convert

// AmpelPolicy represents a complete AMPEL verification policy.
type AmpelPolicy struct {
	ID     string       `json:"id"`
	Meta   AmpelMeta    `json:"meta"`
	Tenets []AmpelTenet `json:"tenets"`
}

// AmpelMeta holds policy metadata.
type AmpelMeta struct {
	Runtime     string    `json:"runtime"`
	Description string    `json:"description"`
	AssertMode  string    `json:"assert_mode"`
	Version     int64     `json:"version"`
	Controls    []Control `json:"controls"`
	Enforce     string    `json:"enforce"`
}

// AmpelTenet represents a single verification check within a policy.
type AmpelTenet struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Predicates PredicateSpec     `json:"predicates"`
	Code       string            `json:"code"`
	Outputs    map[string]Output `json:"outputs,omitempty"`
}

// Control references an OSCAL control associated with the policy.
type Control struct {
	Source string `json:"source"`
	ID     string `json:"id"`
}

// PredicateSpec defines the attestation predicate types a tenet evaluates.
type PredicateSpec struct {
	Types []string `json:"types"`
}

// Output defines a named output extractor from a tenet evaluation.
type Output struct {
	Code  string `json:"code"`
	Value string `json:"value"`
}
