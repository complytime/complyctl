// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antchfx/xmlquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

func TestMapResultStatus(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult provider.Result
		expectedError  error
	}{
		{"pass", "pass", provider.ResultPassed, nil},
		{"fixed", "fixed", provider.ResultPassed, nil},
		{"fail", "fail", provider.ResultFailed, nil},
		{"error", "error", provider.ResultError, nil},
		{"unknown", "unknown", provider.ResultError, nil},
		{"invalid", "invalid", provider.ResultError, errors.New("couldn't match invalid")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapResultStatus(tt.input)
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseCheck(t *testing.T) {
	tests := []struct {
		name           string
		xmlContent     string
		expectedResult string
		expectedError  error
	}{
		{
			name:           "Valid/ExpectedFormat",
			xmlContent:     `<check-content-ref name="oval:ssg-audit_perm_change_success:def:1"/>`,
			expectedResult: "audit_perm_change_success",
		},
		{
			name:           "Invalid/UnexpectedFormat",
			xmlContent:     `<check-content-ref name="ovalssg-audit_perm_change_success:def:1"/>`,
			expectedResult: "",
			expectedError:  errors.New("check id \"ovalssg-audit_perm_change_success:def:1\" is in unexpected format"),
		},
		{
			name:           "Invalid/NoNameAttribute",
			xmlContent:     `<check-content-ref/>`,
			expectedResult: "",
			expectedError:  errors.New("check-content-ref node has no 'name' attribute"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := xmlquery.Parse(strings.NewReader(tt.xmlContent))
			assert.NoError(t, err)
			check, err := parseCheck(node.SelectElement("check-content-ref"))
			assert.Equal(t, tt.expectedResult, check)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPluginServer_Describe(t *testing.T) {
	s := New()
	resp, err := s.Describe(context.Background(), &provider.DescribeRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Healthy)
	assert.Equal(t, "0.1.0", resp.Version)
	assert.Contains(t, resp.RequiredTargetVariables, "profile")
}

func TestPluginServer_Generate_NoConfig(t *testing.T) {
	s := New()
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{})
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "no assessment configurations")
}

func TestPluginServer_Scan_NoTargets(t *testing.T) {
	s := New()
	_, err := s.Scan(context.Background(), &provider.ScanRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no targets")
}

func TestParseARFFile_Missing(t *testing.T) {
	_, err := parseARFFile("/nonexistent/arf.xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open ARF")
}

func TestParseARFFile_InvalidXML(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "arf.xml")
	require.NoError(t, os.WriteFile(tmp, []byte("not xml <<<<"), 0600))
	_, err := parseARFFile(tmp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse ARF")
}

func TestParseARFFile_Valid(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "arf.xml")
	require.NoError(t, os.WriteFile(tmp, []byte("<root><target>host</target></root>"), 0600))
	node, err := parseARFFile(tmp)
	require.NoError(t, err)
	assert.NotNil(t, node)
}

func TestBuildAssessmentsFromARF_NoTarget(t *testing.T) {
	xml := `<root><ds:component xmlns:ds="http://scap.nist.gov/schema/scap/source/1.2">
		<xccdf-1.2:Benchmark xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2"></xccdf-1.2:Benchmark>
		</ds:component></root>`
	node, err := xmlquery.Parse(strings.NewReader(xml))
	require.NoError(t, err)
	_, err = buildAssessmentsFromARF(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no 'target' attribute")
}

func TestBuildAssessmentsFromARF_NoResults(t *testing.T) {
	xml := `<root>
		<target>host1</target>
		<ds:component xmlns:ds="http://scap.nist.gov/schema/scap/source/1.2">
		<xccdf-1.2:Benchmark xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2"></xccdf-1.2:Benchmark>
		</ds:component></root>`
	node, err := xmlquery.Parse(strings.NewReader(xml))
	require.NoError(t, err)
	assessments, err := buildAssessmentsFromARF(node)
	require.NoError(t, err)
	assert.Empty(t, assessments)
}

func TestFindOVALCheckContentRef_NoChecks(t *testing.T) {
	node, err := xmlquery.Parse(strings.NewReader("<rule></rule>"))
	require.NoError(t, err)
	ref := findOVALCheckContentRef(node.SelectElement("rule"))
	assert.Nil(t, ref)
}

func TestMergeVariables(t *testing.T) {
	global := map[string]string{"a": "1", "b": "2"}
	target := map[string]string{"b": "override", "c": "3"}
	merged := mergeVariables(global, target)
	assert.Equal(t, "1", merged["a"])
	assert.Equal(t, "override", merged["b"])
	assert.Equal(t, "3", merged["c"])
}

func TestRuleResultMessage(t *testing.T) {
	tests := []struct {
		name       string
		ruleXML    string
		resultXML  string
		resultText string
		contains   string
	}{
		{
			name:       "TitleAndMessage",
			ruleXML:    `<rule xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2"><xccdf-1.2:title>My Rule</xccdf-1.2:title></rule>`,
			resultXML:  `<rule-result><message>check failed</message></rule-result>`,
			resultText: "fail",
			contains:   "My Rule",
		},
		{
			name:       "NoTitleNoMessage",
			ruleXML:    `<rule></rule>`,
			resultXML:  `<rule-result></rule-result>`,
			resultText: "pass",
			contains:   "openscap rule-result is pass",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleNode, err := xmlquery.Parse(strings.NewReader(tt.ruleXML))
			require.NoError(t, err)
			resultNode, err := xmlquery.Parse(strings.NewReader(tt.resultXML))
			require.NoError(t, err)
			msg := ruleResultMessage(ruleNode.SelectElement("rule"), resultNode.SelectElement("rule-result"), tt.resultText)
			assert.Contains(t, msg, tt.contains)
		})
	}
}
