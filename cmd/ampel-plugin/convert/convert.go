package convert

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/complytime/complyctl/pkg/provider"
)

const (
	// PolicyFileName is the output filename for the merged AMPEL policy bundle.
	PolicyFileName = "complytime-ampel-policy.json"
)

// LoadGranularPolicies reads all .json files from dir (skipping PolicyFileName)
// and returns a map keyed by each policy's ID field.
func LoadGranularPolicies(dir string) (map[string]*AmpelPolicy, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading policy directory %q: %w", dir, err)
	}

	policies := make(map[string]*AmpelPolicy)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if entry.Name() == PolicyFileName {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading policy file %q: %w", path, err)
		}

		var p AmpelPolicy
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parsing policy file %q: %w", path, err)
		}
		if p.ID == "" {
			return nil, fmt.Errorf("policy file %q has empty id field", path)
		}

		policies[p.ID] = &p
	}

	return policies, nil
}

// MatchPolicies looks up each requirement ID from the assessment configurations
// in the granular policy map. It returns the matched policies and warning
// strings for unmatched requirements.
func MatchPolicies(configs []provider.AssessmentConfiguration, granular map[string]*AmpelPolicy) ([]*AmpelPolicy, []string) {
	var matched []*AmpelPolicy
	var warnings []string
	seen := make(map[string]bool)

	for _, config := range configs {
		reqID := config.RequirementID
		if seen[reqID] {
			continue
		}
		seen[reqID] = true

		p, ok := granular[reqID]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("no granular policy found for requirement %q", reqID))
			continue
		}
		matched = append(matched, p)
	}

	// Sort by policy ID for deterministic output.
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ID < matched[j].ID
	})

	return matched, warnings
}

// MergeToBundle wraps matched policies into a top-level AmpelPolicyBundle.
func MergeToBundle(policies []*AmpelPolicy) *AmpelPolicyBundle {
	return &AmpelPolicyBundle{
		ID: "complytime-ampel-policy",
		Meta: BundleMeta{
			Frameworks: []Framework{
				{
					ID:   "ComplyTime-AMPEL-Policy",
					Name: "ComplyTime AMPEL Policy",
				},
			},
		},
		Policies: policies,
	}
}

// WritePolicy marshals an AmpelPolicyBundle to JSON and writes it to the given directory.
// If bundle is nil or has no policies, no file is written and nil is returned.
func WritePolicy(bundle *AmpelPolicyBundle, dir string) error {
	if bundle == nil || len(bundle.Policies) == 0 {
		return nil
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating policy directory %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling AMPEL policy bundle: %w", err)
	}

	path := filepath.Join(dir, PolicyFileName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing policy file: %w", err)
	}

	return nil
}
