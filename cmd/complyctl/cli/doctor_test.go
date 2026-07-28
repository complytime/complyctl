// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/doctor"
)

// --- statusLabel ---

func TestStatusLabel(t *testing.T) {
	tests := []struct {
		name   string
		status doctor.CheckStatus
		want   string
	}{
		{"pass", doctor.StatusPass, "[PASS]"},
		{"fail", doctor.StatusFail, "[FAIL]"},
		{"warn", doctor.StatusWarn, "[WARN]"},
		{"unknown", doctor.CheckStatus("bogus"), "[UNKNOWN]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusLabel(tt.status))
		})
	}
}

// --- resolveFormat ---

func TestResolveFormat_EmptyFlag_NoNOCOLOR(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	got, err := resolveFormat("")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestResolveFormat_EmptyFlag_NOCOLORSet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got, err := resolveFormat("")
	require.NoError(t, err)
	assert.Equal(t, complytime.OutputFormatText, got)
}

func TestResolveFormat_TextFlag(t *testing.T) {
	got, err := resolveFormat(complytime.OutputFormatText)
	require.NoError(t, err)
	assert.Equal(t, complytime.OutputFormatText, got)
}

func TestResolveFormat_JSONFlag(t *testing.T) {
	got, err := resolveFormat(complytime.OutputFormatJSON)
	require.NoError(t, err)
	assert.Equal(t, complytime.OutputFormatJSON, got)
}

func TestResolveFormat_InvalidFlag(t *testing.T) {
	_, err := resolveFormat("xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xml")
}

func TestResolveFormat_HumanFlag(t *testing.T) {
	got, err := resolveFormat(complytime.OutputFormatHuman)
	require.NoError(t, err)
	assert.Equal(t, "", got, "human maps to empty string internally")
}

func TestResolveFormat_ExplicitFlagOverridesNOCOLOR(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got, err := resolveFormat(complytime.OutputFormatJSON)
	require.NoError(t, err)
	assert.Equal(t, complytime.OutputFormatJSON, got)
}

func TestResolveFormat_HumanFlagOverridesNOCOLOR(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got, err := resolveFormat(complytime.OutputFormatHuman)
	require.NoError(t, err)
	assert.Equal(t, "", got, "--format human must produce human output even with NO_COLOR")
}

// --- resultLabel ---

func TestResultLabel_UsesLabelWhenSet(t *testing.T) {
	r := doctor.CheckResult{Name: "provider/ampel", Label: "ampel"}
	assert.Equal(t, "ampel", resultLabel(r))
}

func TestResultLabel_FallsBackToName(t *testing.T) {
	r := doctor.CheckResult{Name: "cache", Label: ""}
	assert.Equal(t, "cache", resultLabel(r))
}

// --- countStatusSummary ---

func TestCountStatusSummary(t *testing.T) {
	var s resultSummary

	countStatusSummary(doctor.StatusPass, &s)
	countStatusSummary(doctor.StatusFail, &s)
	countStatusSummary(doctor.StatusWarn, &s)
	countStatusSummary(doctor.StatusPass, &s)

	assert.Equal(t, 2, s.passCount)
	assert.Equal(t, 1, s.failCount)
	assert.Equal(t, 1, s.warnCount)
}

// --- printDiagnosticsHuman ---

func TestPrintDiagnosticsHuman_ContainsEmojiAndLabel(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusFail, Message: "bad config", Blocking: true, Group: doctor.GroupWorkspace},
		{Name: "providers", Status: doctor.StatusWarn, Message: "no providers", Blocking: false, Group: doctor.GroupProviders},
		{Name: "cache", Status: doctor.StatusPass, Message: "cache ok", Blocking: false, Group: doctor.GroupCache},
	}

	var buf bytes.Buffer
	err := printDiagnosticsHuman(results, &buf)
	require.Error(t, err, "expected error for blocking failure")

	out := buf.String()
	assert.Contains(t, out, "[FAIL]")
	assert.Contains(t, out, "[WARN]")
	assert.Contains(t, out, "[PASS]")
	assert.Contains(t, out, complytime.StatusFailed)
	assert.Contains(t, out, complytime.StatusError)
	assert.Contains(t, out, complytime.StatusPassed)
	assert.Contains(t, out, "3 checks: 1 passed, 1 failed, 1 warnings")
}

func TestPrintDiagnosticsHuman_NoBlockingFailure_ReturnsNil(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusPass, Message: "ok", Blocking: true, Group: doctor.GroupWorkspace},
	}
	var buf bytes.Buffer
	err := printDiagnosticsHuman(results, &buf)
	assert.NoError(t, err)
}

func TestPrintDiagnosticsHuman_WarnBlockingDoesNotFail(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "providers", Status: doctor.StatusWarn, Message: "no providers", Blocking: true, Group: doctor.GroupProviders},
	}
	var buf bytes.Buffer
	err := printDiagnosticsHuman(results, &buf)
	assert.NoError(t, err, "warn+blocking should not trigger error")
}

func TestPrintDiagnosticsHuman_GroupedSections(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "prov1", Status: doctor.StatusPass, Message: "ok", Group: doctor.GroupProviders},
		{Name: "cache1", Status: doctor.StatusPass, Message: "ok", Group: doctor.GroupCache},
	}
	var buf bytes.Buffer
	err := printDiagnosticsHuman(results, &buf)
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, string(doctor.GroupProviders))
	assert.Contains(t, out, string(doctor.GroupCache))
	provIdx := strings.Index(out, string(doctor.GroupProviders))
	cacheIdx := strings.Index(out, string(doctor.GroupCache))
	assert.Less(t, provIdx, cacheIdx, "providers section should appear before cache")
}

func TestPrintDiagnosticsHuman_ChildrenRendered(t *testing.T) {
	results := []doctor.CheckResult{
		{
			Name: "parent", Status: doctor.StatusPass, Message: "ok", Group: doctor.GroupProviders,
			Children: []doctor.CheckResult{
				{Name: "child1", Status: doctor.StatusWarn, Message: "child warning"},
			},
		},
	}
	var buf bytes.Buffer
	err := printDiagnosticsHuman(results, &buf)
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "child1")
	assert.Contains(t, out, "child warning")
	assert.Contains(t, out, "2 checks:", "children should be counted")
}

// --- printDiagnosticsText ---

func TestPrintDiagnosticsText_NoEmoji(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusFail, Message: "bad config", Blocking: true, Group: doctor.GroupWorkspace},
	}

	var buf bytes.Buffer
	err := printDiagnosticsText(results, &buf)
	require.Error(t, err)

	out := buf.String()
	assert.Contains(t, out, "[FAIL]")
	assert.NotContains(t, out, complytime.StatusFailed, "emoji must not appear in text format")
	assert.Contains(t, out, "config: bad config")
}

func TestPrintDiagnosticsText_AllStatuses(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "a", Status: doctor.StatusPass, Message: "ok", Blocking: false, Group: doctor.GroupProviders},
		{Name: "b", Status: doctor.StatusWarn, Message: "warn", Blocking: false, Group: doctor.GroupProviders},
		{Name: "c", Status: doctor.StatusFail, Message: "fail", Blocking: true, Group: doctor.GroupProviders},
	}

	var buf bytes.Buffer
	err := printDiagnosticsText(results, &buf)
	assert.Error(t, err, "expected error for blocking fail entry")
	out := buf.String()

	assert.Contains(t, out, "[PASS] a: ok")
	assert.Contains(t, out, "[WARN] b: warn")
	assert.Contains(t, out, "[FAIL] c: fail")
	assert.Contains(t, out, "3 checks: 1 passed, 1 failed, 1 warnings")
}

func TestPrintDiagnosticsText_NoBlockingFailure_ReturnsNil(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusPass, Message: "ok", Blocking: true, Group: doctor.GroupWorkspace},
	}
	var buf bytes.Buffer
	err := printDiagnosticsText(results, &buf)
	assert.NoError(t, err)
}

// --- printDiagnosticsJSON ---

func TestPrintDiagnosticsJSON_ValidJSON(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusFail, Message: "bad config", Blocking: true, Group: doctor.GroupWorkspace},
		{Name: "providers", Status: doctor.StatusWarn, Message: "no providers", Blocking: false, Group: doctor.GroupProviders},
	}

	var buf bytes.Buffer
	err := printDiagnosticsJSON(results, &buf)
	require.Error(t, err, "expected error for blocking failure")

	var out diagnosticOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))

	assert.Len(t, out.Checks, 2)
	assert.Equal(t, "config", out.Checks[0].Name)
	assert.Equal(t, doctor.StatusFail, out.Checks[0].Status)
	assert.True(t, out.Checks[0].Blocking)
	assert.Equal(t, "providers", out.Checks[1].Name)
	assert.Equal(t, doctor.StatusWarn, out.Checks[1].Status)
	assert.False(t, out.Checks[1].Blocking)
	assert.Equal(t, 2, out.Summary.Total)
	assert.Equal(t, 0, out.Summary.Passed)
	assert.Equal(t, 1, out.Summary.Failed)
	assert.Equal(t, 1, out.Summary.Warnings)
	assert.True(t, out.BlockingFailure)
}

func TestPrintDiagnosticsJSON_NoBlockingFailure(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusPass, Message: "ok", Blocking: true, Group: doctor.GroupWorkspace},
	}
	var buf bytes.Buffer
	err := printDiagnosticsJSON(results, &buf)
	require.NoError(t, err)

	var out diagnosticOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	assert.False(t, out.BlockingFailure)
	assert.Equal(t, 1, out.Summary.Passed)
}

func TestPrintDiagnosticsJSON_EmptyResults(t *testing.T) {
	var buf bytes.Buffer
	err := printDiagnosticsJSON([]doctor.CheckResult{}, &buf)
	require.NoError(t, err)

	var out diagnosticOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	assert.NotNil(t, out.Checks, "checks must be [] not null")
	assert.Empty(t, out.Checks)
}

func TestPrintDiagnosticsJSON_ChildrenIncluded(t *testing.T) {
	results := []doctor.CheckResult{
		{
			Name: "parent", Status: doctor.StatusPass, Message: "ok",
			Group: doctor.GroupProviders,
			Children: []doctor.CheckResult{
				{Name: "child", Status: doctor.StatusWarn, Message: "child warning"},
			},
		},
	}
	var buf bytes.Buffer
	err := printDiagnosticsJSON(results, &buf)
	require.NoError(t, err)

	var out diagnosticOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	require.Len(t, out.Checks, 1)
	require.Len(t, out.Checks[0].Children, 1)
	assert.Equal(t, "child", out.Checks[0].Children[0].Name)
	assert.Equal(t, 2, out.Summary.Total, "children should be counted in summary")
}

func TestPrintDiagnosticsText_GrepPattern(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusFail, Message: "missing policy", Blocking: true, Group: doctor.GroupWorkspace},
		{Name: "cache", Status: doctor.StatusPass, Message: "ok", Blocking: false, Group: doctor.GroupCache},
	}
	var buf bytes.Buffer
	err := printDiagnosticsText(results, &buf)
	assert.Error(t, err, "expected error for blocking fail entry")
	lines := strings.Split(buf.String(), "\n")
	var failLines []string
	for _, l := range lines {
		if strings.Contains(l, "[FAIL]") {
			failLines = append(failLines, l)
		}
	}
	require.Len(t, failLines, 1)
	assert.Contains(t, failLines[0], "config")
	assert.Contains(t, failLines[0], "missing policy")
}

// --- printDiagnosticsTo ---

func TestPrintDiagnosticsTo_DispatchesCorrectly(t *testing.T) {
	passOnly := []doctor.CheckResult{
		{Name: "check", Status: doctor.StatusPass, Message: "ok", Blocking: false, Group: doctor.GroupWorkspace},
	}
	failBlocking := []doctor.CheckResult{
		{Name: "check", Status: doctor.StatusFail, Message: "broken", Blocking: true, Group: doctor.GroupWorkspace},
	}

	t.Run("text format: labels present, no emoji", func(t *testing.T) {
		var buf bytes.Buffer
		err := printDiagnosticsTo(passOnly, complytime.OutputFormatText, &buf)
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "[PASS]")
		assert.NotContains(t, out, complytime.StatusPassed, "emoji must not appear in text mode")
	})

	t.Run("json format: output is valid JSON", func(t *testing.T) {
		var buf bytes.Buffer
		err := printDiagnosticsTo(passOnly, complytime.OutputFormatJSON, &buf)
		require.NoError(t, err)
		var out diagnosticOutput
		require.NoError(t, json.Unmarshal(buf.Bytes(), &out), "output must be valid JSON")
		assert.Len(t, out.Checks, 1)
	})

	t.Run("default (human) format: emoji present", func(t *testing.T) {
		var buf bytes.Buffer
		err := printDiagnosticsTo(passOnly, "", &buf)
		require.NoError(t, err)
		assert.Contains(t, buf.String(), complytime.StatusPassed)
	})

	t.Run("blocking failure propagates through dispatcher", func(t *testing.T) {
		var buf bytes.Buffer
		err := printDiagnosticsTo(failBlocking, complytime.OutputFormatText, &buf)
		assert.Error(t, err)
	})

	t.Run("unrecognised format returns error", func(t *testing.T) {
		var buf bytes.Buffer
		err := printDiagnosticsTo(passOnly, "xml", &buf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported output format")
	})
}

// --- summarizeResults ---

func TestSummarizeResults_CountsAndBlocking(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "a", Status: doctor.StatusPass, Message: "ok", Blocking: false},
		{Name: "b", Status: doctor.StatusFail, Message: "bad", Blocking: true},
		{Name: "c", Status: doctor.StatusWarn, Message: "warn", Blocking: false},
		{Name: "d", Status: doctor.StatusFail, Message: "bad2", Blocking: false},
	}
	s := summarizeResults(results)
	assert.Equal(t, 1, s.passCount)
	assert.Equal(t, 2, s.failCount)
	assert.Equal(t, 1, s.warnCount)
	assert.Equal(t, 4, s.total)
	assert.True(t, s.blockingFailure)
}

func TestSummarizeResults_NoBlockingFailure(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "a", Status: doctor.StatusPass, Message: "ok", Blocking: true},
		{Name: "b", Status: doctor.StatusWarn, Message: "warn", Blocking: true},
	}
	s := summarizeResults(results)
	assert.False(t, s.blockingFailure)
}

func TestSummarizeResults_Empty(t *testing.T) {
	s := summarizeResults([]doctor.CheckResult{})
	assert.Equal(t, 0, s.total)
	assert.False(t, s.blockingFailure)
}

func TestSummarizeResults_CountsChildren(t *testing.T) {
	results := []doctor.CheckResult{
		{
			Name: "parent", Status: doctor.StatusPass, Message: "ok",
			Children: []doctor.CheckResult{
				{Name: "child", Status: doctor.StatusWarn, Message: "warn"},
			},
		},
	}
	s := summarizeResults(results)
	assert.Equal(t, 1, s.passCount, "parent counted")
	assert.Equal(t, 1, s.warnCount, "child counted")
	assert.Equal(t, 2, s.total)
}

func TestSummarizeResults_GrandchildrenNotCounted(t *testing.T) {
	results := []doctor.CheckResult{
		{
			Name: "parent", Status: doctor.StatusPass, Message: "ok",
			Children: []doctor.CheckResult{
				{
					Name: "child", Status: doctor.StatusPass, Message: "ok",
					Children: []doctor.CheckResult{
						{Name: "grandchild", Status: doctor.StatusFail, Message: "fail"},
					},
				},
			},
		},
	}
	s := summarizeResults(results)
	assert.Equal(t, 2, s.total, "grandchildren must not be counted (D5)")
	assert.Equal(t, 0, s.failCount, "grandchild fail must not be counted")
}

// --- statusEmoji ---

func TestStatusEmoji(t *testing.T) {
	tests := []struct {
		name   string
		status doctor.CheckStatus
		want   string
	}{
		{"pass", doctor.StatusPass, complytime.StatusPassed},
		{"fail", doctor.StatusFail, complytime.StatusFailed},
		{"warn", doctor.StatusWarn, complytime.StatusError},
		{"unknown", doctor.CheckStatus("bogus"), complytime.StatusUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusEmoji(tt.status))
		})
	}
}
