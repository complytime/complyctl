// SPDX-License-Identifier: Apache-2.0

package output

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildReportFilename_StandardMultiSegment(t *testing.T) {
	result := BuildReportFilename("report", "test-policy", "web-server", "md")

	assert.True(t, strings.HasPrefix(result, "report-test-policy-web-server-"),
		"expected prefix report-test-policy-web-server-, got %s", result)
	assert.True(t, strings.HasSuffix(result, ".md"),
		"expected suffix .md, got %s", result)

	tsPattern := regexp.MustCompile(`\d{8}-\d{6}`)
	assert.True(t, tsPattern.MatchString(result),
		"expected timestamp pattern YYYYMMDD-HHMMSS in %s", result)
}

func TestBuildReportFilename_SlashSanitizationPolicyID(t *testing.T) {
	result := BuildReportFilename("report", "org/policy/v1", "target", "md")

	assert.True(t, strings.Contains(result, "org-policy-v1"),
		"expected sanitized policyID org-policy-v1 in %s", result)
	assert.False(t, strings.Contains(result, "/"),
		"filename must not contain path separators: %s", result)
}

func TestBuildReportFilename_SlashSanitizationTargetID(t *testing.T) {
	result := BuildReportFilename("report", "policy", "ns/target", "md")

	assert.True(t, strings.Contains(result, "ns-target"),
		"expected sanitized targetID ns-target in %s", result)
	assert.False(t, strings.Contains(result, "/"),
		"filename must not contain path separators: %s", result)
}

func TestBuildReportFilename_CompoundExtension(t *testing.T) {
	result := BuildReportFilename("scan", "policy", "target", "sarif.json")

	assert.True(t, strings.HasSuffix(result, ".sarif.json"),
		"expected compound extension .sarif.json, got %s", result)
}

func TestBuildReportFilename_EmptyTargetID(t *testing.T) {
	result := BuildReportFilename("report", "test-policy", "", "md")

	assert.False(t, strings.Contains(result, "--"),
		"empty targetID must not produce double dashes: %s", result)
	assert.True(t, strings.HasPrefix(result, "report-test-policy-"),
		"expected prefix report-test-policy-, got %s", result)
	assert.True(t, strings.HasSuffix(result, ".md"),
		"expected suffix .md, got %s", result)

	tsPattern := regexp.MustCompile(`\d{8}-\d{6}`)
	assert.True(t, tsPattern.MatchString(result),
		"expected timestamp pattern in %s", result)
}

func TestBuildReportFilename_EvaluationLogPrefix(t *testing.T) {
	result := BuildReportFilename("evaluation-log", "test-policy", "web-server", "yaml")

	assert.True(t, strings.HasPrefix(result, "evaluation-log-test-policy-web-server-"),
		"expected prefix evaluation-log-test-policy-web-server-, got %s", result)
	assert.True(t, strings.HasSuffix(result, ".yaml"),
		"expected suffix .yaml, got %s", result)
}
