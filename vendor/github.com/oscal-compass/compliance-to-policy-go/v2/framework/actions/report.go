/*
 Copyright 2024 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package actions

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/defenseunicorns/go-oscal/src/pkg/uuid"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/oscal-compass/oscal-sdk-go/rules"
	"github.com/oscal-compass/oscal-sdk-go/transformers"

	"github.com/oscal-compass/compliance-to-policy-go/v2/internal/utils"
	"github.com/oscal-compass/compliance-to-policy-go/v2/logging"
	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
)

const (
	InventoryItem = "inventory-item"
	Resource      = "resource"
)

var validSubjectTypes = []string{InventoryItem, Resource}

// Report action generates an Assessment Results from an Assessment Plan and Context.
func Report(ctx context.Context, inputContext *InputContext, planHref string, plan oscalTypes.AssessmentPlan, results []policy.PVPResult) (*oscalTypes.AssessmentResults, error) {
	log := logging.GetLogger("reporter")
	log.Info(fmt.Sprintf("generating assessments results for plan %s", planHref))

	// for each PVPResult.Observation create an OSCAL Observation
	oscalObservations := make([]oscalTypes.Observation, 0)
	oscalFindings := make([]oscalTypes.Finding, 0)
	store := inputContext.Store()

	// Maps resourceIds from observation subjects to subject UUIDs
	// to avoid duplicating subjects for a single resource.
	// This is passed to toOscalObservation to maintain a global
	// state across results.
	subjectUuidMap := make(map[string]string)

	// maps inventory items to subject UUIDs
	invItemMap := make(map[string]oscalTypes.InventoryItem)

	// maps resource items to subject UUIDs
	resourceItemMap := make(map[string]oscalTypes.Resource)

	// Get all the control mappings based on the assessment plan activities
	rulesByControls := make(map[string][]string)
	for _, act := range *plan.LocalDefinitions.Activities {
		var controlSet []string
		if act.RelatedControls != nil {
			controls := act.RelatedControls.ControlSelections
			for _, ctr := range controls {
				for _, assess := range *ctr.IncludeControls {
					targetId := fmt.Sprintf("%s_smt", assess.ControlId)
					controlSet = append(controlSet, targetId)
				}
			}
		}
		rulesByControls[act.Title] = controlSet
	}

	// Process into observations
	for _, result := range results {
		for _, observationByCheck := range result.ObservationsByCheck {
			rule, err := store.GetByCheckID(ctx, observationByCheck.CheckID)
			if err != nil {
				if !errors.Is(err, rules.ErrRuleNotFound) {
					return nil, fmt.Errorf("failed to convert observation for check %v: %w", observationByCheck.CheckID, err)
				} else {
					log.Warn(fmt.Sprintf("skipping observation for check %v: %v", observationByCheck.CheckID, err))
					continue
				}
			}
			obs, err := toOscalObservation(observationByCheck, rule, &subjectUuidMap)
			if err != nil {
				return nil, fmt.Errorf("failed to convert observation for check %v: %w", observationByCheck.CheckID, err)
			}
			oscalObservations = append(oscalObservations, obs)

			if obs.Subjects != nil {
				for _, subject := range *obs.Subjects {

					// Create a new InventoryItem or Resource for this subject if one doesn't already exist
					switch subject.Type {
					case InventoryItem:
						_, ok := invItemMap[subject.SubjectUuid]
						if ok {
							log.Debug(fmt.Sprintf("inventory item already exists for subject %s", subject.SubjectUuid))
						} else {
							invItem := generateInventoryItem(&subject)
							log.Debug(fmt.Sprintf("creating new inventory item for subject %s", subject.SubjectUuid))
							invItemMap[subject.SubjectUuid] = invItem
						}
					case Resource:
						_, ok := resourceItemMap[subject.SubjectUuid]
						if ok {
							log.Debug(fmt.Sprintf("resource %s already exists for subject", subject.SubjectUuid))
						} else {
							resource := generateResource(&subject)
							log.Debug(fmt.Sprintf("creating new resource for subject %s", subject.SubjectUuid))
							resourceItemMap[subject.SubjectUuid] = resource
						}
					}
				}
			}
		}
	}

	assessmentResults, err := transformers.AssessmentPlanToAssessmentResults(plan, planHref, oscalObservations...)
	if err != nil {
		return nil, err
	}

	// New assessment results should only have one Assessment Results
	if len(assessmentResults.Results) != 1 {
		return nil, errors.New("bug: assessment results should only have one result")
	}

	// Create findings after initial observations are added to ensure only observations
	// in-scope of the plan are checked for failure.
	for _, obs := range *assessmentResults.Results[0].Observations {
		// TODO: Empty props indicates that an activity was in scope that results were not received for.
		// We should generate a finding here.
		if obs.Props == nil {
			continue
		}
		rule, found := extensions.GetTrestleProp(extensions.AssessmentRuleIdProp, *obs.Props)
		if !found {
			continue
		}
		targets, found := rulesByControls[rule.Value]
		if !found {
			continue
		}

		// if the observation subject result prop is not "pass" then create relevant findings
		if obs.Subjects != nil {
			for _, subject := range *obs.Subjects {
				result, found := extensions.GetTrestleProp("result", *subject.Props)
				if !found {
					continue
				}
				if result.Value != policy.ResultPass.String() {
					oscalFindings, err = generateFindings(oscalFindings, obs, targets)
					if err != nil {
						return nil, fmt.Errorf("failed to create finding for check: %w", err)
					}
					log.Info(fmt.Sprintf("generated finding for rule %s for subject %s", rule.Value, subject.Title))
					break
				}
			}
		}
	}

	assessmentResults.Results[0].Findings = utils.NilIfEmpty(&oscalFindings)

	// If inventory items were created then add to result
	if len(invItemMap) > 0 {
		invItems := make([]oscalTypes.InventoryItem, 0, len(invItemMap))

		for _, invItem := range invItemMap {
			invItems = append(invItems, invItem)
		}

		localDefs := oscalTypes.LocalDefinitions{
			InventoryItems: &invItems,
		}
		assessmentResults.Results[0].LocalDefinitions = &localDefs
	}

	// If resources were created then add to result
	if len(resourceItemMap) > 0 {
		backmatter := oscalTypes.BackMatter{}
		resources := make([]oscalTypes.Resource, 0, len(resourceItemMap))
		for _, r := range resourceItemMap {
			resources = append(resources, r)
		}
		backmatter.Resources = &resources
		assessmentResults.BackMatter = &backmatter
	}
	return assessmentResults, nil
}

// Generate an OSCAL Inventory Item from a given Subject reference
func generateInventoryItem(subject *oscalTypes.SubjectReference) oscalTypes.InventoryItem {

	// List of props to copy from Subject onto Inventory Item
	invItemPropNames := []string{
		"fqdn",
		"hostname",
		"ipv4-address",
		"ipv6-address",
		"software-name",
		"software-version",
		"uri",
	}

	invItem := oscalTypes.InventoryItem{
		UUID:        subject.SubjectUuid,
		Description: subject.Title,
		Props:       &[]oscalTypes.Property{},
	}

	invItemProps := []oscalTypes.Property{}
	for _, prop := range *subject.Props {
		if slices.Contains(invItemPropNames, prop.Name) {
			invItemProps = append(invItemProps, prop)
		}
	}

	if len(invItemProps) > 0 {
		invItem.Props = &invItemProps
	}
	return invItem
}

// Generate an OSCAL Resource from a given Subject reference
func generateResource(subject *oscalTypes.SubjectReference) oscalTypes.Resource {

	resource := oscalTypes.Resource{
		UUID:  subject.SubjectUuid,
		Title: subject.Title,
	}
	return resource
}

// getFindingForTarget returns an existing finding that matches the targetId if one exists in findings
func getFindingForTarget(findings []oscalTypes.Finding, targetId string) *oscalTypes.Finding {

	for i := range findings {
		if findings[i].Target.TargetId == targetId {
			return &findings[i] // if finding is found, return a pointer to that slice element
		}
	}
	return nil
}

// Generate OSCAL Findings for all non-passing controls in the OSCAL Observation
func generateFindings(findings []oscalTypes.Finding, observation oscalTypes.Observation, targets []string) ([]oscalTypes.Finding, error) {
	for _, targetId := range targets {
		finding := getFindingForTarget(findings, targetId)
		if finding == nil { // if an empty finding was returned, create a new one and append to findings
			newFinding := oscalTypes.Finding{
				UUID: uuid.NewUUID(),
				RelatedObservations: &[]oscalTypes.RelatedObservation{
					{
						ObservationUuid: observation.UUID,
					},
				},
				Target: oscalTypes.FindingTarget{
					TargetId: targetId,
					Type:     "statement-id",
					Status: oscalTypes.ObjectiveStatus{
						State: "not-satisfied",
					},
				},
			}
			findings = append(findings, newFinding)
		} else {
			relObs := oscalTypes.RelatedObservation{
				ObservationUuid: observation.UUID,
			}
			*finding.RelatedObservations = append(*finding.RelatedObservations, relObs) // add new related obs to existing finding for targetId
		}
	}
	return findings, nil
}

// Convert a PVP ObservationByCheck to an OSCAL Observation
func toOscalObservation(observationByCheck policy.ObservationByCheck, ruleSet extensions.RuleSet, subjectUuidMap *map[string]string) (oscalTypes.Observation, error) {
	subjects := make([]oscalTypes.SubjectReference, 0)
	for _, subject := range observationByCheck.Subjects {

		// Verify subject type is allowed
		if !slices.Contains(validSubjectTypes, subject.Type) {
			return oscalTypes.Observation{}, fmt.Errorf("failed to create observation, subject type '%s' is not allowed", subject.Type)
		}

		props := []oscalTypes.Property{
			{
				Name:  "resource-id",
				Value: subject.ResourceID,
				Ns:    extensions.TrestleNameSpace,
			},
			{
				Name:  "result",
				Value: subject.Result.String(),
				Ns:    extensions.TrestleNameSpace,
			},
			{
				Name:  "evaluated-on",
				Value: subject.EvaluatedOn.String(),
				Ns:    extensions.TrestleNameSpace,
			},
			{
				Name:  "reason",
				Value: subject.Reason,
				Ns:    extensions.TrestleNameSpace,
			},
		}

		for _, p := range subject.Props {
			prop := oscalTypes.Property{
				Name:  p.Name,
				Value: p.Value,
				Ns:    extensions.TrestleNameSpace,
			}
			props = append(props, prop)
		}

		// If a subject UUID has already been generated for the
		// given resource ID then do not create a new UUID.
		subjectUuid, ok := (*subjectUuidMap)[subject.ResourceID]
		if !ok {
			subjectUuid = uuid.NewUUID()
			(*subjectUuidMap)[subject.ResourceID] = subjectUuid
		}

		s := oscalTypes.SubjectReference{
			SubjectUuid: subjectUuid,
			Title:       subject.Title,
			Type:        subject.Type,
			Props:       &props,
		}
		subjects = append(subjects, s)
	}

	relevantEvidences := make([]oscalTypes.RelevantEvidence, 0)
	if observationByCheck.RelevantEvidences != nil {
		for _, relEv := range observationByCheck.RelevantEvidences {
			oscalRelEv := oscalTypes.RelevantEvidence{
				Href:        relEv.Href,
				Description: relEv.Description,
			}
			relevantEvidences = append(relevantEvidences, oscalRelEv)
		}
	}

	oscalObservation := oscalTypes.Observation{
		UUID:             uuid.NewUUID(),
		Title:            observationByCheck.Title,
		Description:      observationByCheck.Description,
		Methods:          observationByCheck.Methods,
		Collected:        observationByCheck.Collected,
		Subjects:         utils.NilIfEmpty(&subjects),
		RelevantEvidence: utils.NilIfEmpty(&relevantEvidences),
	}

	props := []oscalTypes.Property{
		{
			Name:  extensions.AssessmentRuleIdProp,
			Value: ruleSet.Rule.ID,
			Ns:    extensions.TrestleNameSpace,
		},
		{
			Name:  extensions.AssessmentCheckIdProp,
			Value: observationByCheck.CheckID,
			Ns:    extensions.TrestleNameSpace,
		},
	}
	oscalObservation.Props = &props

	return oscalObservation, nil
}
