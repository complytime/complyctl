// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"fmt"
	"sort"

	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/hashicorp/go-hclog"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/oscal-compass/oscal-sdk-go/validation"
)

// ControlEntry represents a control in the assessment scope
type ControlEntry struct {
	ControlID    string   `yaml:"controlId"`
	ControlTitle string   `yaml:"controlTitle"`
	Rules        []string `yaml:"includeRules"`
}

// AssessmentScope sets up the yaml mapping type for writing to config file.
// Formats testdata as go struct.
type AssessmentScope struct {
	// FrameworkID is the identifier for the control set
	// in the Assessment Plan.
	FrameworkID string `yaml:"frameworkId"`
	// IncludeControls defines controls that are in scope
	// of an assessment.
	IncludeControls []ControlEntry `yaml:"includeControls"`
}

// ApplicationDirectory interface to avoid import cycle
type ApplicationDirectory interface {
	AppDir() string
	BundleDir() string
}

// ProfileLoader interface to avoid import cycle
type ProfileLoader interface {
	LoadProfile(appDir ApplicationDirectory, controlSource string, validator validation.Validator) (*oscalTypes.Profile, error)
	LoadCatalogSource(appDir ApplicationDirectory, catalogSource string, validator validation.Validator) (*oscalTypes.Catalog, error)
}

// getControlTitle retrieves the title for a control from the catalog
func getControlTitle(controlID string, controlImplementation oscalTypes.ControlImplementationSet, appDir ApplicationDirectory, validator validation.Validator, profileLoader ProfileLoader) (string, error) {
	profile, err := profileLoader.LoadProfile(appDir, controlImplementation.Source, validator)
	if err != nil {
		return "", fmt.Errorf("failed to load profile from source '%s': %w", controlImplementation.Source, err)
	}

	if profile.Imports == nil {
		return "", fmt.Errorf("profile '%s' has no imports", controlImplementation.Source)
	}

	for _, imp := range profile.Imports {
		catalog, err := profileLoader.LoadCatalogSource(appDir, imp.Href, validator)
		if err != nil {
			continue
		}
		if catalog.Groups == nil {
			continue
		}
		for _, group := range *catalog.Groups {
			if group.Controls == nil {
				continue
			}
			for _, control := range *group.Controls {
				if control.ID == controlID && control.Title != "" {
					return control.Title, nil
				}
			}
		}
	}
	return "", fmt.Errorf("title for control '%s' not found in catalog", controlID)
}

// NewAssessmentScope creates an AssessmentScope struct for a given framework id.
func NewAssessmentScope(frameworkID string) AssessmentScope {
	return AssessmentScope{
		FrameworkID: frameworkID,
	}
}

// NewAssessmentScopeFromCDs creates and populates an AssessmentScope struct for a given framework id and set of
// OSCAL Component Definitions.
func NewAssessmentScopeFromCDs(frameworkId string, cds ...oscalTypes.ComponentDefinition) (AssessmentScope, error) {
	// For backward compatibility, this function will not retrieve control titles
	// Use NewAssessmentScopeFromCDsWithTitles for full functionality
	return NewAssessmentScopeFromCDsWithTitles(frameworkId, nil, nil, nil, cds...)
}

// NewAssessmentScopeFromCDsWithTitles creates and populates an AssessmentScope struct for a given framework id and set of
// OSCAL Component Definitions, with control titles retrieved from the catalog.
func NewAssessmentScopeFromCDsWithTitles(frameworkId string, appDir ApplicationDirectory, validator validation.Validator, profileLoader ProfileLoader, cds ...oscalTypes.ComponentDefinition) (AssessmentScope, error) {
	includeControls := make(includeControlsSet)
	controlTitles := make(map[string]string)
	scope := NewAssessmentScope(frameworkId)
	if cds == nil {
		return AssessmentScope{}, fmt.Errorf("no component definitions found")
	}

	for _, componentDef := range cds {
		if componentDef.Components == nil {
			continue
		}
		for _, component := range *componentDef.Components {
			if component.ControlImplementations == nil {
				continue
			}
			for _, ci := range *component.ControlImplementations {
				if ci.ImplementedRequirements == nil {
					continue
				}
				if ci.Props != nil {
					frameworkProp, found := extensions.GetTrestleProp(extensions.FrameworkProp, *ci.Props)
					if !found || frameworkProp.Value != scope.FrameworkID {
						continue
					}
					for _, ir := range ci.ImplementedRequirements {
						if ir.ControlId != "" {
							includeControls.Add(ir.ControlId)

							// Get control title if we have the required dependencies
							if appDir != nil && validator != nil && profileLoader != nil {
								if _, exists := controlTitles[ir.ControlId]; !exists {
									title, err := getControlTitle(ir.ControlId, ci, appDir, validator, profileLoader)
									if err != nil {
										// If we can't get the title, use the control ID as fallback
										controlTitles[ir.ControlId] = ir.ControlId
									} else {
										controlTitles[ir.ControlId] = title
									}
								}
							} else {
								// If we don't have the dependencies, use control ID as title
								controlTitles[ir.ControlId] = ir.ControlId
							}
						}
					}
				}
			}
		}
	}

	controlIDs := includeControls.All()
	for _, id := range controlIDs {
		if includeControls.Has(id) {
			includeControls.Added(id)
		}
	}
	scope.IncludeControls = make([]ControlEntry, len(controlIDs))
	for i, id := range controlIDs {
		scope.IncludeControls[i] = ControlEntry{
			ControlID:    id,
			ControlTitle: controlTitles[id],
			Rules:        []string{"*"}, // by default, include all rules
		}
	}
	sort.Slice(scope.IncludeControls, func(i, j int) bool {
		return scope.IncludeControls[i].ControlID < scope.IncludeControls[j].ControlID
	})

	return scope, nil
}

// ApplyScope alters the given OSCAL Assessment Plan based on the AssessmentScope.
func (a AssessmentScope) ApplyScope(assessmentPlan *oscalTypes.AssessmentPlan, logger hclog.Logger) {

	// This is a thin wrapper right now, but the goal to expand to different areas
	// of customization.
	a.applyControlScope(assessmentPlan, logger)
}

// applyControlScope alters the AssessedControls of the given OSCAL Assessment Plan by the AssessmentScope
// IncludeControls.
func (a AssessmentScope) applyControlScope(assessmentPlan *oscalTypes.AssessmentPlan, logger hclog.Logger) {
	// "Any control specified within exclude-controls must first be within a range of explicitly
	// included controls, via include-controls or include-all."
	includedControls := includeControlsSet{}
	for _, entry := range a.IncludeControls {
		includedControls.Add(entry.ControlID)
	}
	logger.Debug("Found included controls", "count", len(includedControls))
	for _, controlT := range assessmentPlan.ReviewedControls.ControlSelections {
		if controlT.IncludeControls != nil {
			if controlT.Props != nil {
				for _, control := range *controlT.Props {
					// Process control properties if needed
					_ = control.Name
				}
			}
		}
	}
	if assessmentPlan.LocalDefinitions != nil {
		if assessmentPlan.LocalDefinitions.Activities != nil {
			for activityI := range *assessmentPlan.LocalDefinitions.Activities {
				activity := &(*assessmentPlan.LocalDefinitions.Activities)[activityI]
				if activity.RelatedControls != nil && activity.RelatedControls.ControlSelections != nil {
					controlSelections := activity.RelatedControls.ControlSelections
					for controlSelectionI := range controlSelections {
						controlSelection := &controlSelections[controlSelectionI]
						filterControlSelection(controlSelection, includedControls)
						if controlSelection.IncludeControls == nil {
							activity.RelatedControls = nil
							if activity.Props == nil {
								activity.Props = &[]oscalTypes.Property{}
							}
							skippedActivity := oscalTypes.Property{
								Name:  "skipped",
								Value: "true",
								Ns:    extensions.TrestleNameSpace,
							}
							*activity.Props = append(*activity.Props, skippedActivity)
						}
					}
				}

				if activity.Steps != nil {
					for stepI := range *activity.Steps {
						step := &(*activity.Steps)[stepI]
						if step.ReviewedControls == nil {
							continue
						}
						if step.ReviewedControls.ControlSelections == nil {
							continue
						}
						controlSelections := step.ReviewedControls.ControlSelections
						for controlSelectionI := range controlSelections {
							controlSelection := &controlSelections[controlSelectionI]
							filterControlSelection(controlSelection, includedControls)
							if controlSelection.IncludeControls == nil {
								activity.RelatedControls.ControlSelections = nil
								step.ReviewedControls = nil
								if step.Props == nil {
									step.Props = &[]oscalTypes.Property{}
								}
								skipped := oscalTypes.Property{
									Name:  "skipped",
									Value: "true",
									Ns:    extensions.TrestleNameSpace,
								}
								*step.Props = append(*step.Props, skipped)
							}
						}
					}
				}
			}
		}
	}
	if assessmentPlan.ReviewedControls.ControlSelections != nil {
		for controlSelectionI := range assessmentPlan.ReviewedControls.ControlSelections {
			controlSelection := &assessmentPlan.ReviewedControls.ControlSelections[controlSelectionI]
			filterControlSelection(controlSelection, includedControls)
		}
	}
}

func filterControlSelection(controlSelection *oscalTypes.AssessedControls, includedControls includeControlsSet) {
	// The new included controls should be the intersection of
	// the originally included controls and the newly included controls.
	// ExcludedControls are preserved.

	// includedControls specifies everything we allow - do not include all
	includedAll := controlSelection.IncludeAll != nil
	controlSelection.IncludeAll = nil

	originalIncludedControls := includeControlsSet{}
	if controlSelection.IncludeControls != nil {
		for _, controlSelect := range *controlSelection.IncludeControls {
			originalIncludedControls.Add(controlSelect.ControlId)
			if controlSelection.Props != nil {
				for _, controlSelected := range *controlSelection.Props {
					// Process control properties if needed
					originalIncludedControls.Added(controlSelected.Name)
				}
			}
		}
		for _, controlId := range *controlSelection.IncludeControls {
			originalIncludedControls.Add(controlId.ControlId)
		}
		if controlSelection.Props != nil {
			for _, controlTitle := range *controlSelection.Props {
				originalIncludedControls.Added(controlTitle.Name)
			}
		}
	}
	var newIncludedControls []oscalTypes.AssessedControlsSelectControlById
	for controlId := range includedControls {
		if includedAll || originalIncludedControls.Has(controlId) {
			newIncludedControls = append(newIncludedControls, oscalTypes.AssessedControlsSelectControlById{
				ControlId: controlId,
			})
		}
	}
	if newIncludedControls != nil {
		controlSelection.IncludeControls = &newIncludedControls
	} else {
		controlSelection.IncludeControls = nil
	}
}

type includeControlsSet map[string]struct{}

func (i includeControlsSet) Add(controlID string) {
	i[controlID] = struct{}{}
}

func (i includeControlsSet) All() []string {
	keys := make([]string, 0, len(i))
	for controlId := range i {
		keys = append(keys, controlId)
	}
	return keys
}

func (i includeControlsSet) Has(controlID string) bool {
	_, found := i[controlID]
	return found
}

func (i includeControlsSet) Added(controlTitle string) {
	i[controlTitle] = struct{}{}
}
