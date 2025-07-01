/*
 Copyright 2025 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package transformers

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/go-oscal/src/pkg/uuid"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"

	"github.com/oscal-compass/oscal-sdk-go/internal/plans"
	"github.com/oscal-compass/oscal-sdk-go/internal/results"
	"github.com/oscal-compass/oscal-sdk-go/models/components"
	"github.com/oscal-compass/oscal-sdk-go/settings"
)

// ComponentDefinitionsToAssessmentPlan transforms the data from one or more OSCAL Component Definitions to a single OSCAL Assessment Plan.
func ComponentDefinitionsToAssessmentPlan(ctx context.Context, definitions []oscalTypes.ComponentDefinition, framework string) (*oscalTypes.AssessmentPlan, error) {
	// Collect and aggregate all component information for each component definition
	var allComponents []components.Component
	var allImplementations []oscalTypes.ControlImplementationSet
	for _, compDef := range definitions {
		if compDef.Components == nil {
			continue
		}
		for _, comp := range *compDef.Components {
			if comp.ControlImplementations != nil || comp.Type == string(components.Validation) {
				componentAdapter := components.NewDefinedComponentAdapter(comp)
				allComponents = append(allComponents, componentAdapter)
				if comp.ControlImplementations != nil {
					allImplementations = append(allImplementations, *comp.ControlImplementations...)
				}
			}
		}
	}
	implementationSettings, frameworkSrc, err := settings.ByFramework(framework, allImplementations)
	if err != nil || implementationSettings == nil {
		return nil, fmt.Errorf("cannot transform definitions for framework %s: %w", framework, err)
	}
	assessmentPlan, err := plans.GenerateAssessmentPlan(ctx, allComponents, *implementationSettings)
	if err != nil {
		return nil, err
	}

	// Add control source resource to maintain traceability to original control set.
	controlSource := oscalTypes.Resource{
		UUID:        uuid.NewUUID(),
		Description: frameworkSrc.Description,
		Title:       frameworkSrc.Title,
		Rlinks: &[]oscalTypes.ResourceLink{
			{
				MediaType: "application/oscal+json",
				Href:      frameworkSrc.Href,
			},
		},
	}
	backmatter := oscalTypes.BackMatter{
		Resources: &[]oscalTypes.Resource{controlSource},
	}
	assessmentPlan.BackMatter = &backmatter

	// Add a link to the ReviewedControls to source
	sourceRef := oscalTypes.Link{
		Href: fmt.Sprintf("#%s", controlSource.UUID),
		Rel:  "includes-controls-from-source",
		Text: "The reviewed controls are derived from the linked OSCAL profile.",
	}

	if assessmentPlan.ReviewedControls.Links == nil {
		assessmentPlan.ReviewedControls.Links = &[]oscalTypes.Link{}
	}

	*assessmentPlan.ReviewedControls.Links = append(*assessmentPlan.ReviewedControls.Links, sourceRef)

	return assessmentPlan, nil
}

// SSPToAssessmentPlan transforms the data from a System Security Plan at a given import location to a single OSCAL Assessment Plan.
func SSPToAssessmentPlan(ctx context.Context, ssp oscalTypes.SystemSecurityPlan, sspImportPath string) (*oscalTypes.AssessmentPlan, error) {
	var allComponents []components.Component
	for _, sysComp := range ssp.SystemImplementation.Components {
		componentAdapter := components.NewSystemComponentAdapter(sysComp)
		// Skip any components that don't have attached rules
		// For an SSP, this is likely the "This System" component
		if len(componentAdapter.Props()) == 0 || componentAdapter.Title() == "This System" {
			continue
		}
		allComponents = append(allComponents, componentAdapter)
	}
	implementationAdapter := components.NewControlImplementationAdapter(ssp.ControlImplementation)
	implementationSettings := settings.NewImplementationSettings(implementationAdapter)

	if implementationSettings == nil {
		return nil, fmt.Errorf("cannot transform ssp at path %s", sspImportPath)
	}

	return plans.GenerateAssessmentPlan(ctx, allComponents, *implementationSettings, plans.WithImport(sspImportPath))
}

// AssessmentPlanToAssessmentResults transforms the data from an Assessment Plan at a given import location to OSCAL Assessment Results.
func AssessmentPlanToAssessmentResults(plan oscalTypes.AssessmentPlan, apImportPath string, observations ...oscalTypes.Observation) (*oscalTypes.AssessmentResults, error) {
	options := []results.GenerateOption{
		results.WithImport(apImportPath),
	}
	if observations != nil {
		options = append(options, results.WithObservations(observations))
	}
	return results.GenerateAssessmentResults(plan, options...)
}
