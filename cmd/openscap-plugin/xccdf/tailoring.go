// SPDX-License-Identifier: Apache-2.0

package xccdf

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"

	xccdf "github.com/complytime/complyctl/cmd/openscap-plugin/xccdftype"
	"github.com/complytime/complyctl/pkg/provider"
)

const (
	XCCDFCaCNamespace    string = "xccdf_org.ssgproject.content"
	XCCDFNamespace       string = "complytime.openscapplugin"
	XCCDFTailoringSuffix string = "complytime"
)

func removePrefix(str, prefix string) string {
	return strings.TrimPrefix(str, prefix)
}

func getTailoringID() string {
	return fmt.Sprintf("xccdf_%s_tailoring_%s", XCCDFNamespace, XCCDFTailoringSuffix)
}

func getTailoringExtendedProfileID(profileId string) string {
	return fmt.Sprintf(
		"%s_profile_%s", XCCDFCaCNamespace, profileId)
}

func getTailoringProfileID(profileId string) string {
	return fmt.Sprintf(
		"xccdf_%s_profile_%s_%s", XCCDFNamespace, profileId, XCCDFTailoringSuffix)
}

func getTailoringProfileTitle(dsProfileTitle string) string {
	return fmt.Sprintf("ComplyTime Tailoring Profile - %s", dsProfileTitle)
}

func getTailoringVersion() xccdf.VersionElement {
	return xccdf.VersionElement{
		Time:  time.Now().Format(time.RFC3339),
		Value: "1",
	}
}

func getTailoringBenchmarkHref(datastreamPath string) xccdf.BenchmarkElement {
	return xccdf.BenchmarkElement{
		Href: datastreamPath,
	}
}

func validateRuleExistence(policyRuleID string, dsRules []DsRules) bool {
	for _, dsRule := range dsRules {
		ruleID := removePrefix(dsRule.ID, ruleIDPrefix)
		if policyRuleID == ruleID {
			return true
		}
	}
	return false
}

func validateVariableExistence(policyVariableID string, dsVariables []DsVariables) bool {
	for _, dsVariable := range dsVariables {
		varID := removePrefix(dsVariable.ID, varIDPrefix)
		if policyVariableID == varID {
			return true
		}
	}
	return false
}

func unselectAbsentRules(tailoringSelections, dsProfileSelections []xccdf.SelectElement, configuration []provider.AssessmentConfiguration) []xccdf.SelectElement {
	for _, dsRule := range dsProfileSelections {
		dsRuleAlsoInPolicy := false
		ruleID := removePrefix(dsRule.IDRef, ruleIDPrefix)
		for _, cfg := range configuration {
			if ruleID == cfg.RequirementID {
				dsRuleAlsoInPolicy = true
				break
			}
		}
		if !dsRuleAlsoInPolicy && dsRule.Selected {
			tailoringSelections = append(tailoringSelections, xccdf.SelectElement{
				IDRef:    dsRule.IDRef,
				Selected: false,
			})
		}
	}
	return tailoringSelections
}

func selectAdditionalRules(tailoringSelections, dsProfileSelections []xccdf.SelectElement, configuration []provider.AssessmentConfiguration) []xccdf.SelectElement {
	rulesMap := make(map[string]bool)

	for _, cfg := range configuration {
		ruleAlreadyInDsProfile := false
		for _, dsRule := range dsProfileSelections {
			dsRuleID := removePrefix(dsRule.IDRef, ruleIDPrefix)
			if cfg.RequirementID == dsRuleID {
				if dsRule.Selected {
					ruleAlreadyInDsProfile = true
				}
				break
			}
		}
		ruleID := getDsRuleID(cfg.RequirementID)
		if !ruleAlreadyInDsProfile && !rulesMap[ruleID] {
			rulesMap[ruleID] = true
			tailoringSelections = append(tailoringSelections, xccdf.SelectElement{
				IDRef:    ruleID,
				Selected: true,
			})
		}
	}
	return tailoringSelections
}

func filterValidRules(configuration []provider.AssessmentConfiguration, dsPath string) ([]provider.AssessmentConfiguration, []string, error) {
	dsRules, err := GetDsRules(dsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get rules from datastream: %w", err)
	}

	var validConfigs []provider.AssessmentConfiguration
	var skippedRules []string
	for _, cfg := range configuration {
		if validateRuleExistence(cfg.RequirementID, dsRules) {
			validConfigs = append(validConfigs, cfg)
		} else {
			skippedRules = append(skippedRules, cfg.RequirementID)
		}
	}

	if len(skippedRules) > 0 {
		hclog.Default().Warn(
			fmt.Sprintf("%d rule(s) not found in datastream %s and will be skipped: %s",
				len(skippedRules), dsPath, skippedRules))
	}

	return validConfigs, skippedRules, nil
}

func getTailoringSelections(configuration []provider.AssessmentConfiguration, dsProfile *xccdf.ProfileElement) []xccdf.SelectElement {
	var tailoringSelections []xccdf.SelectElement
	tailoringSelections = unselectAbsentRules(tailoringSelections, dsProfile.Selections, configuration)
	tailoringSelections = selectAdditionalRules(tailoringSelections, dsProfile.Selections, configuration)

	return tailoringSelections
}

func updateTailoringValues(tailoringValues, dsProfileValues []xccdf.SetValueElement, configuration []provider.AssessmentConfiguration) []xccdf.SetValueElement {
	varsMap := make(map[string]bool)

	for _, cfg := range configuration {
		for prmID, prmValue := range cfg.Parameters {
			varAlreadyInDsProfile := false
			for _, dsVar := range dsProfileValues {
				dsVarID := removePrefix(dsVar.IDRef, varIDPrefix)
				if prmID == dsVarID {
					if prmValue == dsVar.Value {
						varAlreadyInDsProfile = true
					}
					break
				}
			}
			varID := getDsVarID(prmID)
			if !varAlreadyInDsProfile && !varsMap[varID] {
				varsMap[varID] = true
				tailoringValues = append(tailoringValues, xccdf.SetValueElement{
					IDRef: varID,
					Value: prmValue,
				})
			}
		}
	}
	return tailoringValues
}

func getTailoringValues(configuration []provider.AssessmentConfiguration, dsProfile *xccdf.ProfileElement, dsPath string) ([]xccdf.SetValueElement, error) {
	dsVariables, err := GetDsVariablesValues(dsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get variables from datastream: %w", err)
	}

	for _, cfg := range configuration {
		for prmID := range cfg.Parameters {
			if !validateVariableExistence(prmID, dsVariables) {
				return nil, fmt.Errorf("variable %s not found in datastream: %s", prmID, dsPath)
			}
		}
	}

	dsProfile, err = ResolveDsVariableOptions(dsProfile, dsVariables)
	if err != nil {
		return nil, fmt.Errorf("failed to get values from variables options: %w", err)
	}

	var tailoringValues []xccdf.SetValueElement
	tailoringValues = updateTailoringValues(tailoringValues, dsProfile.Values, configuration)

	return tailoringValues, nil
}

func getTailoringProfile(profileId string, dsPath string, configuration []provider.AssessmentConfiguration) (*xccdf.ProfileElement, error) {
	tailoringProfile := new(xccdf.ProfileElement)
	tailoringProfile.ID = getTailoringProfileID(profileId)

	dsProfile, err := GetDsProfile(profileId, dsPath)
	if err != nil {
		return tailoringProfile, fmt.Errorf("failed to get base profile from datastream: %w", err)
	}

	validConfiguration, _, err := filterValidRules(configuration, dsPath)
	if err != nil {
		return tailoringProfile, fmt.Errorf("failed to filter rules against datastream: %w", err)
	}

	if len(validConfiguration) == 0 {
		return tailoringProfile, fmt.Errorf("no valid rules found in datastream %s for the provided configuration", dsPath)
	}

	tailoringProfile.Extends = getTailoringExtendedProfileID(profileId)

	tailoringProfile.Title = &xccdf.TitleOrDescriptionElement{
		Override: true,
		Value:    getTailoringProfileTitle(dsProfile.Title.Value),
	}

	tailoringProfile.Selections = getTailoringSelections(validConfiguration, dsProfile)

	tailoringProfile.Values, err = getTailoringValues(validConfiguration, dsProfile, dsPath)
	if err != nil {
		return tailoringProfile, fmt.Errorf("failed to get values for tailoring profile: %w", err)
	}
	return tailoringProfile, nil
}

func PolicyToXML(configuration []provider.AssessmentConfiguration, datastreamPath, profileId string) (string, error) {

	if len(configuration) == 0 {
		return "", fmt.Errorf("assessment configuration is empty")
	}

	tailoringProfile, err := getTailoringProfile(profileId, datastreamPath, configuration)
	if err != nil {
		return "", err
	}

	tailoring := xccdf.TailoringElement{
		XMLNamespaceURI: xccdf.XCCDFURI,
		ID:              getTailoringID(),
		Version:         getTailoringVersion(),
		Benchmark:       getTailoringBenchmarkHref(datastreamPath),
		Profile:         *tailoringProfile,
	}

	output, err := xml.MarshalIndent(tailoring, "", "  ")
	if err != nil {
		return "", err
	}
	return xccdf.XMLHeader + "\n" + string(output), nil
}
