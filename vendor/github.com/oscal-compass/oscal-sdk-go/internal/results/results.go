/*
 Copyright 2025 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package results

import (
	"fmt"
	"time"

	"github.com/defenseunicorns/go-oscal/src/pkg/uuid"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"

	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/oscal-compass/oscal-sdk-go/models"
)

const defaultActor = "tool"

type generateOpts struct {
	title        string
	importAP     string
	observations []oscalTypes.Observation
}

func (g *generateOpts) defaults() {
	g.title = models.SampleRequiredString
	g.importAP = models.SampleRequiredString
}

// GenerateOption defines an option to tune the behavior of the
// GenerateAssessmentPlan function.
type GenerateOption func(opts *generateOpts)

// WithTitle is a GenerateOption that sets the AssessmentPlan title
// in the metadata.
func WithTitle(title string) GenerateOption {
	return func(opts *generateOpts) {
		opts.title = title
	}
}

// WithImport is a GenerateOption that sets the AssessmentPlan
// ImportAP Href value.
func WithImport(importAP string) GenerateOption {
	return func(opts *generateOpts) {
		opts.importAP = importAP
	}
}

// WithObservations is a GenerateOption that adds pre-processed OSCAL Observations
// to Assessment Results for associated to Assessment Plan Activities.
func WithObservations(observations []oscalTypes.Observation) GenerateOption {
	return func(opts *generateOpts) {
		opts.observations = observations
	}
}

// GenerateAssessmentResults generates an AssessmentPlan for a set of Components and ImplementationSettings. The chosen inputs allow an Assessment Plan to be generated from
// a set of OSCAL ComponentDefinitions or a SystemSecurityPlan.
//
// If `WithImport` is not set, all input components are set as Components in the Local Definitions.
// If `WithObservations is not set, default behavior is to create a new, empty Observation for each activity step with the step.Title as the
// Observation title.
func GenerateAssessmentResults(plan oscalTypes.AssessmentPlan, opts ...GenerateOption) (*oscalTypes.AssessmentResults, error) {
	options := generateOpts{}
	options.defaults()
	for _, opt := range opts {
		opt(&options)
	}

	metadata := models.NewSampleMetadata()
	metadata.Title = options.title

	assessmentResults := &oscalTypes.AssessmentResults{
		UUID: uuid.NewUUID(),
		ImportAp: oscalTypes.ImportAp{
			Href: options.importAP,
		},
		Metadata: metadata,
		Results:  make([]oscalTypes.Result, 0), // Required field
	}

	if plan.Tasks == nil {
		return assessmentResults, fmt.Errorf("assessment plan tasks cannot be empty")
	}
	tasks := *plan.Tasks

	observationManager := newObservationManager(plan)
	if options.observations != nil {
		observationManager.load(options.observations)
	}

	activitiesByUUID := make(map[string]oscalTypes.Activity)
	if plan.LocalDefinitions != nil || plan.LocalDefinitions.Activities != nil {
		for _, activity := range *plan.LocalDefinitions.Activities {
			activitiesByUUID[activity.UUID] = activity
		}
	}

	// Process each task in the assessment plan
	for _, task := range tasks {
		result := oscalTypes.Result{
			Title:       fmt.Sprintf("Result For Task %q", task.Title),
			Description: fmt.Sprintf("OSCAL Assessment Result For Task %q", task.Title),
			Start:       time.Now(),
			UUID:        uuid.NewUUID(),
		}

		// Some initial checks before proceeding with the rest
		if task.AssociatedActivities == nil {
			assessmentResults.Results = append(assessmentResults.Results, result)
			continue
		}

		// Observations associated to the tasks found through
		// checks.
		var reviewedControls oscalTypes.ReviewedControls
		var associatedObservations []oscalTypes.Observation
		for _, assocActivity := range *task.AssociatedActivities {
			activity := activitiesByUUID[assocActivity.ActivityUuid]

			if activity.RelatedControls != nil {
				reviewedControls.ControlSelections = append(reviewedControls.ControlSelections, activity.RelatedControls.ControlSelections...)
			}

			if activity.Steps != nil {
				relatedTask := oscalTypes.RelatedTask{
					TaskUuid: task.UUID,
					Subjects: &assocActivity.Subjects,
				}
				var methods []oscalTypes.Property
				if activity.Props != nil {
					methods = extensions.FindAllProps(*activity.Props, extensions.WithName("method"), extensions.WithNamespace(""))
				}
				setWaivedProp := false
				waived, found := extensions.GetTrestleProp(extensions.WaivedRulesProperty, *activity.Props)
				if found && waived.Value == "true" {
					setWaivedProp = true
				}
				// Activity Title == Rule
				// One Observation per Activity Step
				// Observation Title == Check
				for _, step := range *activity.Steps {
					observation := observationManager.createOrGet(step.Title)
					for _, method := range methods {
						observation.Methods = append(observation.Methods, method.Value)
					}
					// Add a waived property to each observation subject if the activity is waived
					if setWaivedProp {
						for _, subject := range *observation.Subjects {
							property := oscalTypes.Property{
								Name:  extensions.WaivedRulesProperty,
								Value: "true",
								Ns:    extensions.TrestleNameSpace,
							}
							*subject.Props = append(*subject.Props, property)
						}
					}

					if observation.Origins != nil && len(*observation.Origins) == 1 {
						origin := *observation.Origins
						if origin[0].RelatedTasks == nil {
							origin[0].RelatedTasks = &[]oscalTypes.RelatedTask{}
						}
						*origin[0].RelatedTasks = append(*origin[0].RelatedTasks, relatedTask)
					}
					associatedObservations = append(associatedObservations, observation)
				}
			}
		}

		result.ReviewedControls = reviewedControls
		if len(associatedObservations) > 0 {
			result.Observations = &associatedObservations
		}
		assessmentResults.Results = append(assessmentResults.Results, result)
	}

	return assessmentResults, nil
}
