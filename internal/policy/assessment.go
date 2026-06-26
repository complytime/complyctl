// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"github.com/complytime/complyctl/pkg/provider"
)

// EvaluatorGroup bundles per-requirement configs for a single evaluator.
type EvaluatorGroup struct {
	EvaluatorID string
	Configs     []provider.AssessmentConfiguration
}

// ExtractAssessmentConfigs converts a DependencyGraph into provider-ready
// AssessmentConfiguration entries. EvaluatorID is set as a routing field
// on the struct — it is not injected into Parameters. Parameters should
// only carry per-requirement variable overrides for the provider.
func ExtractAssessmentConfigs(graph *DependencyGraph) []provider.AssessmentConfiguration {
	configs := make([]provider.AssessmentConfiguration, 0, len(graph.Assessments))

	for _, a := range graph.Assessments {
		configs = append(configs, provider.AssessmentConfiguration{
			PlanID:        a.ID,
			RequirementID: a.RequirementID,
			Parameters:    a.Parameters,
			EvaluatorID:   a.EvaluatorID,
		})
	}

	return configs
}

// GroupByEvaluator groups assessment configs by EvaluatorID.
// See R32: specs/001-gemara-native-workflow/research.md
func GroupByEvaluator(configs []provider.AssessmentConfiguration, graph *DependencyGraph) map[string]EvaluatorGroup {
	groups := make(map[string]EvaluatorGroup)

	if graph.EvaluatorID != "" {
		groups[graph.EvaluatorID] = EvaluatorGroup{
			EvaluatorID: graph.EvaluatorID,
			Configs:     configs,
		}
		return groups
	}

	for _, cfg := range configs {
		evalID := cfg.EvaluatorID
		g := groups[evalID]
		g.EvaluatorID = evalID
		g.Configs = append(g.Configs, cfg)
		groups[evalID] = g
	}

	return groups
}
