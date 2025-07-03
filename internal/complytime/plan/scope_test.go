// SPDX-License-Identifier: Apache-2.0
package plan

import (
	"testing"

	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/hashicorp/go-hclog"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/stretchr/testify/require"
)

func TestNewAssessmentScopeFromCDs(t *testing.T) {
	_, err := NewAssessmentScopeFromCDs("example")
	require.EqualError(t, err, "no component definitions found")

	cd := oscalTypes.ComponentDefinition{
		Components: &[]oscalTypes.DefinedComponent{
			{
				Title: "Component",
				ControlImplementations: &[]oscalTypes.ControlImplementationSet{
					{
						Props: &[]oscalTypes.Property{
							{
								Name:  extensions.FrameworkProp,
								Value: "example",
								Ns:    extensions.TrestleNameSpace,
							},
						},
						ImplementedRequirements: []oscalTypes.ImplementedRequirementControlImplementation{
							{
								ControlId: "control-1",
							},
							{
								ControlId: "control-2",
							},
						},
					},
				},
			},
		},
	}

	wantScope := AssessmentScope{
		FrameworkID: "example",
		IncludeControls: []ControlEntry{
			{ControlID: "control-1", IncludeRules: []string{"*"}},
			{ControlID: "control-2", IncludeRules: []string{"*"}},
		},
	}
	scope, err := NewAssessmentScopeFromCDs("example", cd)
	require.NoError(t, err)
	require.Equal(t, wantScope, scope)

	// Reproduce duplicates
	anotherComponent := oscalTypes.DefinedComponent{
		Title: "AnotherComponent",
		ControlImplementations: &[]oscalTypes.ControlImplementationSet{
			{
				Props: &[]oscalTypes.Property{
					{
						Name:  extensions.FrameworkProp,
						Value: "example",
						Ns:    extensions.TrestleNameSpace,
					},
				},
				ImplementedRequirements: []oscalTypes.ImplementedRequirementControlImplementation{
					{
						ControlId: "control-1",
					},
					{
						ControlId: "control-2",
					},
				},
			},
		},
	}
	*cd.Components = append(*cd.Components, anotherComponent)

	scope, err = NewAssessmentScopeFromCDs("example", cd)
	require.NoError(t, err)
	require.Equal(t, wantScope, scope)
}

func TestAssessmentScope_ApplyScope(t *testing.T) {
	testLogger := hclog.NewNullLogger()

	tests := []struct {
		name           string
		basePlan       *oscalTypes.AssessmentPlan
		scope          AssessmentScope
		wantSelections []oscalTypes.AssessedControls
	}{
		{
			name: "Success/Default",
			basePlan: &oscalTypes.AssessmentPlan{
				ReviewedControls: oscalTypes.ReviewedControls{
					ControlSelections: []oscalTypes.AssessedControls{
						{
							IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
								{
									ControlId: "example-1",
								},
								{
									ControlId: "example-2",
								},
							},
						},
					},
				},
			},
			scope: AssessmentScope{
				FrameworkID: "test",
				IncludeControls: []ControlEntry{
					{ControlID: "example-2", IncludeRules: []string{"*"}},
				},
			},
			wantSelections: []oscalTypes.AssessedControls{
				{
					IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
						{
							ControlId: "example-2",
						},
					},
				},
			},
		},
		// Testing for out-of-scope controls
		{
			name: "All Controls Out-of-Scope",
			basePlan: &oscalTypes.AssessmentPlan{
				ReviewedControls: oscalTypes.ReviewedControls{
					ControlSelections: []oscalTypes.AssessedControls{
						{
							IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
								{
									ControlId: "",
								},
							},
						},
					},
				},
			},
			scope: AssessmentScope{
				FrameworkID:     "test",
				IncludeControls: nil,
			},
			wantSelections: []oscalTypes.AssessedControls{
				{
					IncludeControls: nil,
				},
			},
		},
		{
			name: "Some Controls Out-of-Scope",
			basePlan: &oscalTypes.AssessmentPlan{
				ReviewedControls: oscalTypes.ReviewedControls{
					ControlSelections: []oscalTypes.AssessedControls{
						{
							IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
								{
									ControlId: "example-1",
								},
							},
						},
					},
				},
			},
			scope: AssessmentScope{
				FrameworkID: "test",
				IncludeControls: []ControlEntry{
					{ControlID: "example-1", IncludeRules: []string{"*"}},
					{ControlID: "example-2", IncludeRules: []string{"*"}},
				},
			},
			wantSelections: []oscalTypes.AssessedControls{
				{
					IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
						{
							ControlId: "example-1",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := tt.scope
			scope.ApplyScope(tt.basePlan, testLogger)
			require.Equal(t, tt.wantSelections, tt.basePlan.ReviewedControls.ControlSelections)
		})
	}
}

func TestAssessmentScope_ApplyRuleScope(t *testing.T) {
	testLogger := hclog.NewNullLogger()

	tests := []struct {
		name           string
		basePlan       *oscalTypes.AssessmentPlan
		scope          AssessmentScope
		wantActivities *[]oscalTypes.Activity
	}{
		{
			name: "Success/Default",
			basePlan: &oscalTypes.AssessmentPlan{
				LocalDefinitions: &oscalTypes.LocalDefinitions{
					Activities: &[]oscalTypes.Activity{
						{
							Title: "rule-1",
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "example-1",
											},
											{
												ControlId: "example-2",
											},
										},
									},
								},
							},
						},
						{
							Title: "rule-2",
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "example-1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			scope: AssessmentScope{
				FrameworkID: "test",
				IncludeControls: []ControlEntry{
					{ControlID: "example-1", IncludeRules: []string{"*"}},
					{ControlID: "example-2", IncludeRules: []string{"*"}},
				},
			},
			wantActivities: &[]oscalTypes.Activity{
				{
					Title: "rule-1",
					RelatedControls: &oscalTypes.ReviewedControls{
						ControlSelections: []oscalTypes.AssessedControls{
							{
								IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
									{
										ControlId: "example-1",
									},
									{
										ControlId: "example-2",
									},
								},
							},
						},
					},
				},
				{
					Title: "rule-2",
					RelatedControls: &oscalTypes.ReviewedControls{
						ControlSelections: []oscalTypes.AssessedControls{
							{
								IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
									{
										ControlId: "example-1",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Success/ExcludeRuleForControl",
			basePlan: &oscalTypes.AssessmentPlan{
				LocalDefinitions: &oscalTypes.LocalDefinitions{
					Activities: &[]oscalTypes.Activity{
						{
							Title: "rule-1",
							Props: &[]oscalTypes.Property{
								{
									Name:  extensions.RuleIdProp,
									Value: "rule-1",
									Ns:    extensions.TrestleNameSpace,
								},
							},
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "control-1",
											},
											{
												ControlId: "control-2",
											},
										},
									},
								},
							},
						},
						{
							Title: "rule-2",
							Props: &[]oscalTypes.Property{
								{
									Name:  extensions.RuleIdProp,
									Value: "rule-2",
									Ns:    extensions.TrestleNameSpace,
								},
							},
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "control-1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			scope: AssessmentScope{
				FrameworkID: "test",
				IncludeControls: []ControlEntry{
					{ControlID: "control-1", IncludeRules: []string{"*"}, ExcludeRules: []string{"rule-1"}},
					{ControlID: "control-2", IncludeRules: []string{"*"}},
				},
			},
			wantActivities: &[]oscalTypes.Activity{
				{
					Title: "rule-1",
					Props: &[]oscalTypes.Property{
						{
							Name:  extensions.RuleIdProp,
							Value: "rule-1",
							Ns:    extensions.TrestleNameSpace,
						},
					},
					RelatedControls: &oscalTypes.ReviewedControls{
						ControlSelections: []oscalTypes.AssessedControls{
							{
								IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
									{
										ControlId: "control-2",
									},
								},
							},
						},
					},
				},
				{
					Title: "rule-2",
					Props: &[]oscalTypes.Property{
						{
							Name:  extensions.RuleIdProp,
							Value: "rule-2",
							Ns:    extensions.TrestleNameSpace,
						},
					},
					RelatedControls: &oscalTypes.ReviewedControls{
						ControlSelections: []oscalTypes.AssessedControls{
							{
								IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
									{
										ControlId: "control-1",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Success/ActivityMarkedSkipped",
			basePlan: &oscalTypes.AssessmentPlan{
				LocalDefinitions: &oscalTypes.LocalDefinitions{
					Activities: &[]oscalTypes.Activity{
						{
							Title: "rule-1",
							Props: &[]oscalTypes.Property{
								{
									Name:  extensions.RuleIdProp,
									Value: "rule-1",
									Ns:    extensions.TrestleNameSpace,
								},
							},
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "control-1",
											},
										},
									},
								},
							},
						},
						{
							Title: "rule-2",
							Props: &[]oscalTypes.Property{
								{
									Name:  extensions.RuleIdProp,
									Value: "rule-2",
									Ns:    extensions.TrestleNameSpace,
								},
							},
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "control-2",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			scope: AssessmentScope{
				FrameworkID: "test",
				IncludeControls: []ControlEntry{
					{ControlID: "control-1", IncludeRules: []string{"*"}, ExcludeRules: []string{"rule-1"}},
					{ControlID: "control-2", IncludeRules: []string{"*"}},
				},
			},
			wantActivities: &[]oscalTypes.Activity{
				{
					Title: "rule-1",
					Props: &[]oscalTypes.Property{
						{
							Name:  extensions.RuleIdProp,
							Value: "rule-1",
							Ns:    extensions.TrestleNameSpace,
						},
						{
							Name:  "skipped",
							Value: "true",
							Ns:    extensions.TrestleNameSpace,
						},
					},
					RelatedControls: nil,
				},
				{
					Title: "rule-2",
					Props: &[]oscalTypes.Property{
						{
							Name:  extensions.RuleIdProp,
							Value: "rule-2",
							Ns:    extensions.TrestleNameSpace,
						},
					},
					RelatedControls: &oscalTypes.ReviewedControls{
						ControlSelections: []oscalTypes.AssessedControls{
							{
								IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
									{
										ControlId: "control-2",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Success/MissingIncludeRules",
			basePlan: &oscalTypes.AssessmentPlan{
				LocalDefinitions: &oscalTypes.LocalDefinitions{
					Activities: &[]oscalTypes.Activity{
						{
							Title: "rule-1",
							Props: &[]oscalTypes.Property{
								{
									Name:  extensions.RuleIdProp,
									Value: "rule-1",
									Ns:    extensions.TrestleNameSpace,
								},
							},
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "control-1",
											},
										},
									},
								},
							},
						},
						{
							Title: "rule-2",
							Props: &[]oscalTypes.Property{
								{
									Name:  extensions.RuleIdProp,
									Value: "rule-2",
									Ns:    extensions.TrestleNameSpace,
								},
							},
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "control-1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			scope: AssessmentScope{
				FrameworkID: "test",
				IncludeControls: []ControlEntry{
					{ControlID: "control-1", ExcludeRules: []string{"rule-1"}}, // Missing includeRules should default to "*"
				},
			},
			wantActivities: &[]oscalTypes.Activity{
				{
					Title: "rule-1",
					Props: &[]oscalTypes.Property{
						{
							Name:  extensions.RuleIdProp,
							Value: "rule-1",
							Ns:    extensions.TrestleNameSpace,
						},
						{
							Name:  "skipped",
							Value: "true",
							Ns:    extensions.TrestleNameSpace,
						},
					},
					RelatedControls: nil,
				},
				{
					Title: "rule-2",
					Props: &[]oscalTypes.Property{
						{
							Name:  extensions.RuleIdProp,
							Value: "rule-2",
							Ns:    extensions.TrestleNameSpace,
						},
					},
					RelatedControls: &oscalTypes.ReviewedControls{
						ControlSelections: []oscalTypes.AssessedControls{
							{
								IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
									{
										ControlId: "control-1",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Success/ExcludeAllRules",
			basePlan: &oscalTypes.AssessmentPlan{
				LocalDefinitions: &oscalTypes.LocalDefinitions{
					Activities: &[]oscalTypes.Activity{
						{
							Title: "rule-1",
							Props: &[]oscalTypes.Property{
								{
									Name:  extensions.RuleIdProp,
									Value: "rule-1",
									Ns:    extensions.TrestleNameSpace,
								},
							},
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "control-1",
											},
										},
									},
								},
							},
						},
						{
							Title: "rule-2",
							Props: &[]oscalTypes.Property{
								{
									Name:  extensions.RuleIdProp,
									Value: "rule-2",
									Ns:    extensions.TrestleNameSpace,
								},
							},
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "control-1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			scope: AssessmentScope{
				FrameworkID: "test",
				IncludeControls: []ControlEntry{
					{ControlID: "control-1", IncludeRules: []string{"rule-1", "rule-2"}, ExcludeRules: []string{"*"}}, // ExcludeRules="*" should override includeRules
				},
			},
			wantActivities: &[]oscalTypes.Activity{
				{
					Title: "rule-1",
					Props: &[]oscalTypes.Property{
						{
							Name:  extensions.RuleIdProp,
							Value: "rule-1",
							Ns:    extensions.TrestleNameSpace,
						},
						{
							Name:  "skipped",
							Value: "true",
							Ns:    extensions.TrestleNameSpace,
						},
					},
					RelatedControls: nil,
				},
				{
					Title: "rule-2",
					Props: &[]oscalTypes.Property{
						{
							Name:  extensions.RuleIdProp,
							Value: "rule-2",
							Ns:    extensions.TrestleNameSpace,
						},
						{
							Name:  "skipped",
							Value: "true",
							Ns:    extensions.TrestleNameSpace,
						},
					},
					RelatedControls: nil,
				},
			},
		},
		{
			name: "Success/GlobalExcludeOverridesInclude",
			basePlan: &oscalTypes.AssessmentPlan{
				LocalDefinitions: &oscalTypes.LocalDefinitions{
					Activities: &[]oscalTypes.Activity{
						{
							Title: "rule-1",
							Props: &[]oscalTypes.Property{
								{
									Name:  extensions.RuleIdProp,
									Value: "rule-1",
									Ns:    extensions.TrestleNameSpace,
								},
							},
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "control-1",
											},
										},
									},
								},
							},
						},
						{
							Title: "rule-2",
							Props: &[]oscalTypes.Property{
								{
									Name:  extensions.RuleIdProp,
									Value: "rule-2",
									Ns:    extensions.TrestleNameSpace,
								},
							},
							RelatedControls: &oscalTypes.ReviewedControls{
								ControlSelections: []oscalTypes.AssessedControls{
									{
										IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
											{
												ControlId: "control-1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			scope: AssessmentScope{
				FrameworkID:        "test",
				GlobalExcludeRules: []string{"rule-1"}, // Global exclude should override control-specific include
				IncludeControls: []ControlEntry{
					{ControlID: "control-1", IncludeRules: []string{"rule-1", "rule-2"}}, // Explicitly includes rule-1, but global exclude wins
				},
			},
			wantActivities: &[]oscalTypes.Activity{
				{
					Title: "rule-1",
					Props: &[]oscalTypes.Property{
						{
							Name:  extensions.RuleIdProp,
							Value: "rule-1",
							Ns:    extensions.TrestleNameSpace,
						},
						{
							Name:  "skipped",
							Value: "true",
							Ns:    extensions.TrestleNameSpace,
						},
					},
					RelatedControls: nil,
				},
				{
					Title: "rule-2",
					Props: &[]oscalTypes.Property{
						{
							Name:  extensions.RuleIdProp,
							Value: "rule-2",
							Ns:    extensions.TrestleNameSpace,
						},
					},
					RelatedControls: &oscalTypes.ReviewedControls{
						ControlSelections: []oscalTypes.AssessedControls{
							{
								IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
									{
										ControlId: "control-1",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := tt.scope
			scope.ApplyScope(tt.basePlan, testLogger)
			require.Equal(t, tt.wantActivities, tt.basePlan.LocalDefinitions.Activities)
		})
	}
}
