package convert

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
)

const (
	// BranchRulesPredicateType is the attestation predicate type for branch protection rules.
	BranchRulesPredicateType = "http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml"
	// PolicyFileName is the output filename for the generated AMPEL policy.
	PolicyFileName = "branch-protection-policy.json"
)

// ConvertConfig holds configuration for the OSCAL to AMPEL conversion.
type ConvertConfig struct {
	Profile string
}

// PolicyToAmpel translates an OSCAL policy (list of RuleSets) into an AMPEL policy.
// It returns nil with no error if the input is empty (no applicable rules).
// It returns an error if the input contains invalid data.
func PolicyToAmpel(oscalPolicy policy.Policy, cfg ConvertConfig) (*AmpelPolicy, error) {
	if oscalPolicy == nil {
		return nil, fmt.Errorf("policy must not be nil")
	}

	if len(oscalPolicy) == 0 {
		return nil, nil
	}

	var tenets []AmpelTenet
	for _, ruleSet := range oscalPolicy {
		if len(ruleSet.Checks) == 0 {
			continue
		}
		for _, check := range ruleSet.Checks {
			if check.ID == "" {
				return nil, fmt.Errorf("check ID must not be empty for rule %q", ruleSet.Rule.ID)
			}
			tenet := AmpelTenet{
				ID:    check.ID,
				Title: check.Description,
				Predicates: PredicateSpec{
					Types: []string{BranchRulesPredicateType},
				},
				Code: buildCELExpression(ruleSet.Rule.Parameters),
			}
			tenets = append(tenets, tenet)
		}
	}

	if len(tenets) == 0 {
		return nil, nil
	}

	ampelPolicy := &AmpelPolicy{
		ID: "branch-protection-policy",
		Meta: AmpelMeta{
			Runtime:     "cel@v14.0",
			Description: "Branch protection policy generated from complyctl assessment plan",
			AssertMode:  "AND",
			Version:     1,
			Controls:    []Control{},
			Enforce:     "ON",
		},
		Tenets: tenets,
	}

	return ampelPolicy, nil
}

// buildCELExpression constructs a CEL expression from rule parameters.
func buildCELExpression(params []extensions.Parameter) string {
	if len(params) == 0 {
		return ""
	}
	// Use the first parameter to build the expression.
	// Each parameter maps to a predicate field check.
	p := params[0]
	if isNumeric(p.Value) {
		return fmt.Sprintf("predicate.%s >= %s", p.ID, p.Value)
	}
	return fmt.Sprintf("predicate.%s == %s", p.ID, p.Value)
}

// isNumeric reports whether s can be parsed as an integer.
func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// WritePolicy marshals an AmpelPolicy to JSON and writes it to the given directory.
// If p is nil, no file is written and nil is returned.
func WritePolicy(p *AmpelPolicy, dir string) error {
	if p == nil {
		return nil
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating policy directory %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling AMPEL policy: %w", err)
	}

	path := filepath.Join(dir, PolicyFileName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing policy file: %w", err)
	}

	return nil
}
