// SPDX-License-Identifier: Apache-2.0

package xccdf

import (
	"path/filepath"
	"regexp"
	"testing"
	"time"

	xccdf "github.com/complytime/complyctl/cmd/openscap-plugin/xccdftype"
	"github.com/complytime/complyctl/pkg/provider"
)

// This is a supporting function to get the profile element from the testing Datastream.
// It is used by TestGetTailoringSelections and TestGetTailoringValues.
func getProfileElementTest(t *testing.T, profileID string) (*xccdf.ProfileElement, error) {
	doc, _ := LoadDsTest(t, "ssg-rhel-ds.xml")
	dsProfile, err := getDsProfile(doc, profileID)
	if err != nil {
		t.Fatalf("failed to get profile: %v", err)
	}
	parsedProfile, err := initProfile(dsProfile, profileID)
	if err != nil {
		t.Fatalf("failed to init parsed profile: %v", err)
	}
	return parsedProfile, nil
}

// TestGetTailoringID tests the getTailoringID function.
func TestGetTailoringID(t *testing.T) {
	expected := "xccdf_complytime.openscapplugin_tailoring_complytime"
	result := getTailoringID()

	if result != expected {
		t.Errorf("getTailoringID() = %v; want %v", result, expected)
	}
}

// TestGetTailoringProfileID tests the getTailoringProfileID function.
func TestGetTailoringProfileID(t *testing.T) {
	profileId := "test_profile"
	expected := "xccdf_complytime.openscapplugin_profile_test_profile_complytime"
	result := getTailoringProfileID(profileId)

	if result != expected {
		t.Errorf("getTailoringProfileID(%v) = %v; want %v", profileId, result, expected)
	}
}

// TestGetTailoringProfileTitle tests the getTailoringProfileTitle function.
func TestGetTailoringProfileTitle(t *testing.T) {
	tests := []struct {
		profileTitle string
		expected     string
	}{
		{"Test Profile", "ComplyTime Tailoring Profile - Test Profile"},
		{"test_profile_id", "ComplyTime Tailoring Profile - test_profile_id"},
	}

	for _, tt := range tests {
		result := getTailoringProfileTitle(tt.profileTitle)
		if result != tt.expected {
			t.Errorf("getTailoringProfileTitle(%v) = %v; want %v", tt.profileTitle, result, tt.expected)
		}
	}
}

// TestGetTailoringVersion tests the getTailoringVersion function.
func TestGetTailoringVersion(t *testing.T) {
	result := getTailoringVersion()

	if result.Value != "1" {
		t.Errorf("getTailoringVersion().Value = %v; want %v", result.Value, "1")
	}

	_, err := time.Parse(time.RFC3339, result.Time)
	if err != nil {
		t.Errorf("getTailoringVersion().Time = %v; not in RFC3339 format", result.Time)
	}
}

// TestGetTailoringBenchmarkHref tests the getTailoringBenchmarkHref function.
func TestGetTailoringBenchmarkHref(t *testing.T) {
	dsPath := filepath.Join(testDataDir, "ssg-rhel-ds.xml")
	expected := xccdf.BenchmarkElement{
		Href: dsPath,
	}
	result := getTailoringBenchmarkHref(dsPath)

	if result != expected {
		t.Errorf("getTailoringBenchmarkHref(%v) = %v; want %v", dsPath, result, expected)
	}
}

// TestValidateRuleExistence tests the validateRuleExistence function.
func TestValidateRuleExistence(t *testing.T) {
	tests := []struct {
		name          string
		policyRuleID  string
		dsRules       []DsRules
		expectedExist bool
	}{
		{
			name:         "Rule exists",
			policyRuleID: "rule1",
			dsRules: []DsRules{
				{ID: "xccdf_org.ssgproject.content_rule_rule1"},
				{ID: "xccdf_org.ssgproject.content_rule_rule2"},
			},
			expectedExist: true,
		},
		{
			name:         "Rule does not exist",
			policyRuleID: "rule3",
			dsRules: []DsRules{
				{ID: "xccdf_org.ssgproject.content_rule_rule1"},
				{ID: "xccdf_org.ssgproject.content_rule_rule2"},
			},
			expectedExist: false,
		},
		{
			name:          "Empty dsRules",
			policyRuleID:  "rule1",
			dsRules:       []DsRules{},
			expectedExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateRuleExistence(tt.policyRuleID, tt.dsRules)
			if result != tt.expectedExist {
				t.Errorf("validateRuleExistence(%v, %v) = %v; want %v", tt.policyRuleID, tt.dsRules, result, tt.expectedExist)
			}
		})
	}
}

// TestValidateVariableExistence tests the validateVariableExistence function.
func TestValidateVariableExistence(t *testing.T) {
	tests := []struct {
		name              string
		policyVariableID  string
		dsVariables       []DsVariables
		expectedExistence bool
	}{
		{
			name:             "Variable exists",
			policyVariableID: "var1",
			dsVariables: []DsVariables{
				{ID: "xccdf_org.ssgproject.content_value_var1"},
				{ID: "xccdf_org.ssgproject.content_value_var2"},
			},
			expectedExistence: true,
		},
		{
			name:             "Variable does not exist",
			policyVariableID: "var3",
			dsVariables: []DsVariables{
				{ID: "xccdf_org.ssgproject.content_value_var1"},
				{ID: "xccdf_org.ssgproject.content_value_var2"},
			},
			expectedExistence: false,
		},
		{
			name:              "Empty dsVariables",
			policyVariableID:  "var1",
			dsVariables:       []DsVariables{},
			expectedExistence: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateVariableExistence(tt.policyVariableID, tt.dsVariables)
			if result != tt.expectedExistence {
				t.Errorf("validateVariableExistence(%v, %v) = %v; want %v", tt.policyVariableID, tt.dsVariables, result, tt.expectedExistence)
			}
		})
	}
}

// TestUnselectAbsentRules tests the unselectAbsentRules function.
func TestUnselectAbsentRules(t *testing.T) {
	tests := []struct {
		name                string
		tailoringSelections []xccdf.SelectElement
		dsProfileSelections []xccdf.SelectElement
		configuration       []provider.AssessmentConfiguration
		expectedSelections  []xccdf.SelectElement
	}{
		{
			name:                "No absent rules",
			tailoringSelections: []xccdf.SelectElement{},
			dsProfileSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule1", Selected: true},
				{IDRef: "xccdf_org.ssgproject.content_rule_rule2", Selected: true},
			},
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "rule1"},
				{RequirementID: "rule2"},
			},
			expectedSelections: []xccdf.SelectElement{},
		},
		{
			name:                "One absent rule",
			tailoringSelections: []xccdf.SelectElement{},
			dsProfileSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule1", Selected: true},
				{IDRef: "xccdf_org.ssgproject.content_rule_rule2", Selected: true},
			},
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "rule1"},
			},
			expectedSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule2", Selected: false},
			},
		},
		{
			name:                "All absent rules",
			tailoringSelections: []xccdf.SelectElement{},
			dsProfileSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule1", Selected: true},
				{IDRef: "xccdf_org.ssgproject.content_rule_rule2", Selected: true},
			},
			configuration: []provider.AssessmentConfiguration{},
			expectedSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule1", Selected: false},
				{IDRef: "xccdf_org.ssgproject.content_rule_rule2", Selected: false},
			},
		},
		{
			name:                "No dsProfileSelections",
			tailoringSelections: []xccdf.SelectElement{},
			dsProfileSelections: []xccdf.SelectElement{},
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "rule1"},
			},
			expectedSelections: []xccdf.SelectElement{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unselectAbsentRules(tt.tailoringSelections, tt.dsProfileSelections, tt.configuration)
			if len(result) != len(tt.expectedSelections) {
				t.Errorf("unselectAbsentRules() length = %v; want %v", len(result), len(tt.expectedSelections))
			}
			for i, selection := range result {
				if selection.IDRef != tt.expectedSelections[i].IDRef || selection.Selected != tt.expectedSelections[i].Selected {
					t.Errorf("unselectAbsentRules()[%d] = %v; want %v", i, selection, tt.expectedSelections[i])
				}
			}
		})
	}
}

// TestSelectAdditionalRules tests the selectAdditionalRules function.
func TestSelectAdditionalRules(t *testing.T) {
	tests := []struct {
		name                string
		tailoringSelections []xccdf.SelectElement
		dsProfileSelections []xccdf.SelectElement
		configuration       []provider.AssessmentConfiguration
		expectedSelections  []xccdf.SelectElement
	}{
		{
			name:                "No additional rules",
			tailoringSelections: []xccdf.SelectElement{},
			dsProfileSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule1", Selected: true},
				{IDRef: "xccdf_org.ssgproject.content_rule_rule2", Selected: true},
			},
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "rule1"},
				{RequirementID: "rule2"},
			},
			expectedSelections: []xccdf.SelectElement{},
		},
		{
			name:                "One additional rule",
			tailoringSelections: []xccdf.SelectElement{},
			dsProfileSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule1", Selected: true},
			},
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "rule1"},
				{RequirementID: "rule2"},
			},
			expectedSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule2", Selected: true},
			},
		},
		{
			name:                "All additional rules",
			tailoringSelections: []xccdf.SelectElement{},
			dsProfileSelections: []xccdf.SelectElement{},
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "rule1"},
				{RequirementID: "rule2"},
			},
			expectedSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule1", Selected: true},
				{IDRef: "xccdf_org.ssgproject.content_rule_rule2", Selected: true},
			},
		},
		{
			name:                "Rule already in dsProfile but unselected",
			tailoringSelections: []xccdf.SelectElement{},
			dsProfileSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule1", Selected: false},
			},
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "rule1"},
			},
			expectedSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule1", Selected: true},
			},
		},
		{
			name:                "One additional rule informed twice",
			tailoringSelections: []xccdf.SelectElement{},
			dsProfileSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule1", Selected: true},
			},
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "rule1"},
				{RequirementID: "rule2"},
				{RequirementID: "rule2"},
			},
			expectedSelections: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_rule2", Selected: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectAdditionalRules(tt.tailoringSelections, tt.dsProfileSelections, tt.configuration)
			if len(result) != len(tt.expectedSelections) {
				t.Errorf("selectAdditionalRules() length = %v; want %v", len(result), len(tt.expectedSelections))
			}
			for i, selection := range result {
				if selection.IDRef != tt.expectedSelections[i].IDRef || selection.Selected != tt.expectedSelections[i].Selected {
					t.Errorf("selectAdditionalRules()[%d] = %v; want %v", i, selection, tt.expectedSelections[i])
				}
			}
		})
	}
}

// TestFilterValidRules tests the filterValidRules function.
func TestFilterValidRules(t *testing.T) {
	dsPath := filepath.Join(testDataDir, "ssg-rhel-ds.xml")

	tests := []struct {
		name                 string
		configuration        []provider.AssessmentConfiguration
		expectedError        bool
		expectedValidCount   int
		expectedSkippedCount int
		expectedSkipped      []string
	}{
		{
			name: "All rules present",
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "package_telnet-server_removed"},
				{RequirementID: "package_telnet_removed"},
			},
			expectedValidCount:   2,
			expectedSkippedCount: 0,
		},
		{
			name: "One rule missing in datastream",
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "package_telnet-server_removed"},
				{RequirementID: "this_rule_is_not_in_datastream"},
			},
			expectedValidCount:   1,
			expectedSkippedCount: 1,
			expectedSkipped:      []string{"this_rule_is_not_in_datastream"},
		},
		{
			name: "All rules missing",
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "bogus_rule_a"},
				{RequirementID: "bogus_rule_b"},
			},
			expectedValidCount:   0,
			expectedSkippedCount: 2,
		},
		{
			name:                 "Empty configuration",
			configuration:        []provider.AssessmentConfiguration{},
			expectedValidCount:   0,
			expectedSkippedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, skipped, err := filterValidRules(tt.configuration, dsPath)
			if (err != nil) != tt.expectedError {
				t.Errorf("filterValidRules() error = %v; want error=%v", err, tt.expectedError)
			}
			if len(valid) != tt.expectedValidCount {
				t.Errorf("filterValidRules() valid count = %d; want %d", len(valid), tt.expectedValidCount)
			}
			if len(skipped) != tt.expectedSkippedCount {
				t.Errorf("filterValidRules() skipped count = %d; want %d", len(skipped), tt.expectedSkippedCount)
			}
			for i, s := range tt.expectedSkipped {
				if i < len(skipped) && skipped[i] != s {
					t.Errorf("filterValidRules() skipped[%d] = %s; want %s", i, skipped[i], s)
				}
			}
		})
	}
}

// TestGetTailoringSelections tests the getTailoringSelections function.
func TestGetTailoringSelections(t *testing.T) {
	parsedProfile, _ := getProfileElementTest(t, "xccdf_org.ssgproject.content_profile_test_profile")

	tests := []struct {
		name           string
		configuration  []provider.AssessmentConfiguration
		expectedResult []xccdf.SelectElement
	}{
		{
			name: "All rules present",
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "package_telnet-server_removed"},
				{RequirementID: "package_telnet_removed"},
				{RequirementID: "set_password_hashing_algorithm_logindefs"},
				{RequirementID: "set_password_hashing_algorithm_systemauth"},
			},
			expectedResult: []xccdf.SelectElement{},
		},
		{
			name:          "No rules in configuration",
			configuration: []provider.AssessmentConfiguration{},
			expectedResult: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_package_telnet-server_removed", Selected: false},
				{IDRef: "xccdf_org.ssgproject.content_rule_package_telnet_removed", Selected: false},
				{IDRef: "xccdf_org.ssgproject.content_rule_set_password_hashing_algorithm_logindefs", Selected: false},
				{IDRef: "xccdf_org.ssgproject.content_rule_set_password_hashing_algorithm_systemauth", Selected: false},
			},
		},
		{
			name: "Additional rule in configuration",
			configuration: []provider.AssessmentConfiguration{
				{RequirementID: "package_telnet-server_removed"},
				{RequirementID: "package_telnet_removed"},
				{RequirementID: "set_password_hashing_algorithm_logindefs"},
				{RequirementID: "set_password_hashing_algorithm_systemauth"},
				{RequirementID: "account_unique_id"},
			},
			expectedResult: []xccdf.SelectElement{
				{IDRef: "xccdf_org.ssgproject.content_rule_account_unique_id", Selected: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTailoringSelections(tt.configuration, parsedProfile)
			if len(result) != len(tt.expectedResult) {
				t.Errorf("getTailoringSelections() length = %v; want %v", len(result), len(tt.expectedResult))
			}
			for i, selection := range result {
				if selection.IDRef != tt.expectedResult[i].IDRef || selection.Selected != tt.expectedResult[i].Selected {
					t.Errorf("getTailoringSelections()[%d] = %v; want %v", i, selection, tt.expectedResult[i])
				}
			}
		})
	}
}

// TestUpdateTailoringValues tests the updateTailoringValues function.
func TestUpdateTailoringValues(t *testing.T) {
	tests := []struct {
		name            string
		tailoringValues []xccdf.SetValueElement
		dsProfileValues []xccdf.SetValueElement
		configuration   []provider.AssessmentConfiguration
		expectedValues  []xccdf.SetValueElement
	}{
		{
			name:            "No additional values",
			tailoringValues: []xccdf.SetValueElement{},
			dsProfileValues: []xccdf.SetValueElement{
				{IDRef: "xccdf_org.ssgproject.content_value_var1", Value: "value1"},
				{IDRef: "xccdf_org.ssgproject.content_value_var2", Value: "value2"},
			},
			configuration: []provider.AssessmentConfiguration{
				{Parameters: map[string]string{"var1": "value1"}},
				{Parameters: map[string]string{"var2": "value2"}},
			},
			expectedValues: []xccdf.SetValueElement{},
		},
		{
			name:            "One additional value",
			tailoringValues: []xccdf.SetValueElement{},
			dsProfileValues: []xccdf.SetValueElement{
				{IDRef: "xccdf_org.ssgproject.content_value_var1", Value: "value1"},
			},
			configuration: []provider.AssessmentConfiguration{
				{Parameters: map[string]string{"var1": "value1"}},
				{Parameters: map[string]string{"var2": "value2"}},
			},
			expectedValues: []xccdf.SetValueElement{
				{IDRef: "xccdf_org.ssgproject.content_value_var2", Value: "value2"},
			},
		},
		{
			name:            "All additional values",
			tailoringValues: []xccdf.SetValueElement{},
			dsProfileValues: []xccdf.SetValueElement{},
			configuration: []provider.AssessmentConfiguration{
				{Parameters: map[string]string{"var1": "value1"}},
				{Parameters: map[string]string{"var2": "value2"}},
			},
			expectedValues: []xccdf.SetValueElement{
				{IDRef: "xccdf_org.ssgproject.content_value_var1", Value: "value1"},
				{IDRef: "xccdf_org.ssgproject.content_value_var2", Value: "value2"},
			},
		},
		{
			name:            "Variable already in dsProfile but different value",
			tailoringValues: []xccdf.SetValueElement{},
			dsProfileValues: []xccdf.SetValueElement{
				{IDRef: "xccdf_org.ssgproject.content_value_var1", Value: "old_value"},
			},
			configuration: []provider.AssessmentConfiguration{
				{Parameters: map[string]string{"var1": "new_value"}},
			},
			expectedValues: []xccdf.SetValueElement{
				{IDRef: "xccdf_org.ssgproject.content_value_var1", Value: "new_value"},
			},
		},
		{
			name:            "Rule without parameter",
			tailoringValues: []xccdf.SetValueElement{},
			dsProfileValues: []xccdf.SetValueElement{
				{IDRef: "xccdf_org.ssgproject.content_value_var1", Value: "value1"},
			},
			configuration: []provider.AssessmentConfiguration{
				{Parameters: nil},
			},
			expectedValues: []xccdf.SetValueElement{},
		},
		{
			name:            "One additional value informed twice",
			tailoringValues: []xccdf.SetValueElement{},
			dsProfileValues: []xccdf.SetValueElement{
				{IDRef: "xccdf_org.ssgproject.content_value_var1", Value: "value1"},
			},
			configuration: []provider.AssessmentConfiguration{
				{Parameters: map[string]string{"var1": "value1"}},
				{Parameters: map[string]string{"var2": "value2"}},
				{Parameters: map[string]string{"var2": "value2"}},
			},
			expectedValues: []xccdf.SetValueElement{
				{IDRef: "xccdf_org.ssgproject.content_value_var2", Value: "value2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := updateTailoringValues(tt.tailoringValues, tt.dsProfileValues, tt.configuration)
			if len(result) != len(tt.expectedValues) {
				t.Errorf("updateTailoringValues() length = %v; want %v", len(result), len(tt.expectedValues))
			}
			for i, value := range result {
				if value.IDRef != tt.expectedValues[i].IDRef || value.Value != tt.expectedValues[i].Value {
					t.Errorf("updateTailoringValues()[%d] = %v; want %v", i, value, tt.expectedValues[i])
				}
			}
		})
	}
}

// TestGetTailoringValues tests the getTailoringValues function.
func TestGetTailoringValues(t *testing.T) {
	dsPath := filepath.Join(testDataDir, "ssg-rhel-ds.xml")

	tests := []struct {
		name           string
		configuration  []provider.AssessmentConfiguration
		expectedError  bool
		expectedResult []xccdf.SetValueElement
	}{
		{
			name: "All variables present",
			configuration: []provider.AssessmentConfiguration{
				{Parameters: map[string]string{"var_password_hashing_algorithm": "SHA512"}},
				{Parameters: map[string]string{"var_password_hashing_algorithm_pam": "sha512"}},
				{Parameters: map[string]string{"var_accounts_tmout": "900"}},
				{Parameters: map[string]string{"var_password_pam_remember_control_flag": "requisite,required"}},
				{Parameters: map[string]string{"var_password_pam_remember": "5"}},
				{Parameters: map[string]string{"var_system_crypto_policy": "DEFAULT"}},
			},
			expectedError:  false,
			expectedResult: []xccdf.SetValueElement{},
		},
		{
			name: "One variable missing in datastream",
			configuration: []provider.AssessmentConfiguration{
				{Parameters: map[string]string{"var_password_hashing_algorithm": "SHA512"}},
				{Parameters: map[string]string{"var_password_hashing_algorithm_pam": "sha512"}},
				{Parameters: map[string]string{"var_accounts_tmout": "900"}},
				{Parameters: map[string]string{"var_password_pam_remember_control_flag": "requisite,required"}},
				{Parameters: map[string]string{"var_password_pam_remember": "5"}},
				{Parameters: map[string]string{"this_variable_is_not_in_datastream": "value"}},
			},
			expectedError:  true,
			expectedResult: nil,
		},
		{
			name: "Additional variable in configuration",
			configuration: []provider.AssessmentConfiguration{
				{Parameters: map[string]string{"var_password_hashing_algorithm": "SHA512"}},
				{Parameters: map[string]string{"var_password_hashing_algorithm_pam": "sha512"}},
				{Parameters: map[string]string{"var_accounts_tmout": "900"}},
				{Parameters: map[string]string{"var_password_pam_remember_control_flag": "requisite,required"}},
				{Parameters: map[string]string{"var_password_pam_remember": "5"}},
				{Parameters: map[string]string{"var_system_crypto_policy": "DEFAULT"}},
				{Parameters: map[string]string{"var_selinux_policy_name": "mls"}},
			},
			expectedError: false,
			expectedResult: []xccdf.SetValueElement{
				{IDRef: "xccdf_org.ssgproject.content_value_var_selinux_policy_name", Value: "mls"},
			},
		},
		{
			name: "Configuration without variables",
			configuration: []provider.AssessmentConfiguration{
				{Parameters: nil},
			},
			expectedError:  false,
			expectedResult: []xccdf.SetValueElement{},
		},
	}

	for _, tt := range tests {
		parsedProfile, _ := getProfileElementTest(t, "xccdf_org.ssgproject.content_profile_test_profile")

		t.Run(tt.name, func(t *testing.T) {
			result, err := getTailoringValues(tt.configuration, parsedProfile, dsPath)
			if (err != nil) != tt.expectedError {
				t.Errorf("getTailoringValues() error = %v; want %v", err, tt.expectedError)
			}
			if len(result) != len(tt.expectedResult) {
				t.Errorf("getTailoringValues() length = %v; want %v", len(result), len(tt.expectedResult))
			}
			for i, value := range result {
				if value.IDRef != tt.expectedResult[i].IDRef || value.Value != tt.expectedResult[i].Value {
					t.Errorf("getTailoringValues()[%d] = %v; want %v", i, value, tt.expectedResult[i])
				}
			}
		})
	}
}

// TestGetTailoringProfile tests the getTailoringProfile function.
func TestGetTailoringProfile(t *testing.T) {
	dsPath := filepath.Join(testDataDir, "ssg-rhel-ds.xml")
	profileId := "test_profile"

	tailoringConfig := []provider.AssessmentConfiguration{
		{
			RequirementID: "set_password_hashing_algorithm_logindefs",
			Parameters:    map[string]string{"var_password_hashing_algorithm": "YESCRYPT"},
		},
	}

	expected := xccdf.ProfileElement{
		ID: getTailoringProfileID(profileId),
		Title: &xccdf.TitleOrDescriptionElement{
			Value: "ComplyTime Tailoring Profile - Test Profile",
		},
		Selections: []xccdf.SelectElement{
			{IDRef: "xccdf_org.ssgproject.content_rule_package_telnet-server_removed", Selected: false},
			{IDRef: "xccdf_org.ssgproject.content_rule_package_telnet_removed", Selected: false},
			{IDRef: "xccdf_org.ssgproject.content_rule_set_password_hashing_algorithm_systemauth", Selected: false},
		},
		Values: []xccdf.SetValueElement{
			{IDRef: "xccdf_org.ssgproject.content_value_var_password_hashing_algorithm", Value: "YESCRYPT"},
		},
	}

	result, err := getTailoringProfile(profileId, dsPath, tailoringConfig)
	if err != nil {
		t.Fatalf("getTailoringProfile() error = %v", err)
	}

	if result.ID != expected.ID {
		t.Errorf("getTailoringProfile().ID = %v; want %v", result.ID, expected.ID)
	}
	if result.Title.Value != expected.Title.Value {
		t.Errorf("getTailoringProfile().Title.Value = %v; want %v", result.Title.Value, expected.Title.Value)
	}
	if len(result.Selections) != len(expected.Selections) {
		t.Errorf("getTailoringProfile().Selections = %v; want %v", result.Selections, expected.Selections)
	}
	if len(result.Values) != len(expected.Values) {
		t.Errorf("getTailoringProfile().Values = %v; want %v", result.Values, expected.Values)
	}
}

func removeVersionTimeTest(xml string) string {
	re := regexp.MustCompile(`time="[^"]*"`)
	return re.ReplaceAllString(xml, `time=""`)
}

// TestPolicyToXML tests the PolicyToXML function.
func TestPolicyToXML(t *testing.T) {
	dsPath := filepath.Join(testDataDir, "ssg-rhel-ds.xml")
	profileId := "test_profile"

	tailoringConfig := []provider.AssessmentConfiguration{
		{
			RequirementID: "account_unique_id",
			Parameters:    map[string]string{"var_password_hashing_algorithm": "YESCRYPT"},
		},
	}

	expectedXML := `<?xml version="1.0" encoding="UTF-8"?>
<xccdf-1.2:Tailoring xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2" id="xccdf_complytime.openscapplugin_tailoring_complytime">
  <xccdf-1.2:benchmark href="` + dsPath + `"></xccdf-1.2:benchmark>
  <xccdf-1.2:version time="` + getTailoringVersion().Time + `">1</xccdf-1.2:version>
  <xccdf-1.2:Profile id="xccdf_complytime.openscapplugin_profile_test_profile_complytime" extends="xccdf_org.ssgproject.content_profile_test_profile">
    <xccdf-1.2:title override="true">ComplyTime Tailoring Profile - Test Profile</xccdf-1.2:title>
    <xccdf-1.2:select idref="xccdf_org.ssgproject.content_rule_package_telnet-server_removed" selected="false"></xccdf-1.2:select>
    <xccdf-1.2:select idref="xccdf_org.ssgproject.content_rule_package_telnet_removed" selected="false"></xccdf-1.2:select>
    <xccdf-1.2:select idref="xccdf_org.ssgproject.content_rule_set_password_hashing_algorithm_logindefs" selected="false"></xccdf-1.2:select>
    <xccdf-1.2:select idref="xccdf_org.ssgproject.content_rule_set_password_hashing_algorithm_systemauth" selected="false"></xccdf-1.2:select>
    <xccdf-1.2:select idref="xccdf_org.ssgproject.content_rule_account_unique_id" selected="true"></xccdf-1.2:select>
    <xccdf-1.2:set-value idref="xccdf_org.ssgproject.content_value_var_password_hashing_algorithm">YESCRYPT</xccdf-1.2:set-value>
  </xccdf-1.2:Profile>
</xccdf-1.2:Tailoring>`

	result, err := PolicyToXML(tailoringConfig, dsPath, profileId)
	if err != nil {
		t.Fatalf("PolicyToXML() error = %v", err)
	}

	expected := removeVersionTimeTest(expectedXML)
	actual := removeVersionTimeTest(result)

	if actual != expected {
		t.Errorf("PolicyToXML() = %v; want %v", actual, expected)
	}
}
