// SPDX-License-Identifier: Apache-2.0
package plan

import (
	"testing"

	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/hashicorp/go-hclog"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/oscal-compass/oscal-sdk-go/validation"
	"github.com/stretchr/testify/require"
)

// Test implementations for testing
type testAppDir struct{}

func (t *testAppDir) AppDir() string    { return "/test/app" }
func (t *testAppDir) BundleDir() string { return "/test/bundle" }

type testValidator struct{}

func (t *testValidator) Validate(oscalTypes.OscalModels) error { return nil }

type testProfileLoader struct{}

func (t *testProfileLoader) LoadProfile(appDir ApplicationDirectory, controlSource string, validator validation.Validator) (*oscalTypes.Profile, error) {
	return &oscalTypes.Profile{
		Imports: []oscalTypes.Import{
			{Href: "catalog.json"},
		},
	}, nil
}
func (t *testProfileLoader) LoadCatalogSource(appDir ApplicationDirectory, catalogSource string, validator validation.Validator) (*oscalTypes.Catalog, error) {
	return &oscalTypes.Catalog{
		Groups: &[]oscalTypes.Group{
			{
				Controls: &[]oscalTypes.Control{
					{ID: "control-1", Title: "Example Control 1"},
					{ID: "control-2", Title: "Example Control 2"},
				},
			},
		},
	}, nil
}

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
						Source: "profile.json",
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
			{ControlID: "control-1", ControlTitle: "Example Control 1", Rules: []string{"*"}},
			{ControlID: "control-2", ControlTitle: "Example Control 2", Rules: []string{"*"}},
		},
	}

	// Test with test implementations to retrieve actual control titles
	testAppDir := &testAppDir{}
	testValidator := &testValidator{}
	testProfileLoader := &testProfileLoader{}

	scope, err := NewAssessmentScopeFromCDsWithTitles("example", testAppDir, testValidator, testProfileLoader, cd)
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
				Source: "profile.json",
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

	scope, err = NewAssessmentScopeFromCDsWithTitles("example", testAppDir, testValidator, testProfileLoader, cd)
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
					{ControlID: "example-2", ControlTitle: "Example Control 2", Rules: []string{"*"}},
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
					{ControlID: "example-1", ControlTitle: "Example Control 1", Rules: []string{"*"}},
					{ControlID: "example-2", ControlTitle: "Example Control 2", Rules: []string{"*"}},
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
	{
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := tt.scope
			scope.ApplyScope(tt.basePlan, testLogger)
			require.Equal(t, tt.wantSelections, tt.basePlan.ReviewedControls.ControlSelections)
		})
	}
}
