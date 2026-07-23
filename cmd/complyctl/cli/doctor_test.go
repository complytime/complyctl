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

func TestStatusLabel_Pass(t *testing.T) {
	assert.Equal(t, "[PASS]", statusLabel(doctor.StatusPass))
}

func TestStatusLabel_Fail(t *testing.T) {
	assert.Equal(t, "[FAIL]", statusLabel(doctor.StatusFail))
}

func TestStatusLabel_Warn(t *testing.T) {
	assert.Equal(t, "[WARN]", statusLabel(doctor.StatusWarn))
}

func TestStatusLabel_Unknown(t *testing.T) {
	assert.Equal(t, "[UNKNOWN]", statusLabel(doctor.CheckStatus("bogus")))
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

func TestResolveFormat_ExplicitFlagOverridesNOCOLOR(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got, err := resolveFormat(complytime.OutputFormatJSON)
	require.NoError(t, err)
	assert.Equal(t, complytime.OutputFormatJSON, got)
}

// --- printDiagnosticsHuman ---

func TestPrintDiagnosticsHuman_ContainsEmojiAndLabel(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusFail, Message: "bad config", Blocking: true},
		{Name: "providers", Status: doctor.StatusWarn, Message: "no providers", Blocking: false},
		{Name: "cache", Status: doctor.StatusPass, Message: "cache ok", Blocking: false},
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
		{Name: "config", Status: doctor.StatusPass, Message: "ok", Blocking: true},
	}
	var buf bytes.Buffer
	err := printDiagnosticsHuman(results, &buf)
	assert.NoError(t, err)
}

func TestPrintDiagnosticsHuman_WarnBlockingDoesNotFail(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "providers", Status: doctor.StatusWarn, Message: "no providers", Blocking: true},
	}
	var buf bytes.Buffer
	err := printDiagnosticsHuman(results, &buf)
	assert.NoError(t, err, "warn+blocking should not trigger error")
}

// --- printDiagnosticsText ---

func TestPrintDiagnosticsText_NoEmoji(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusFail, Message: "bad config", Blocking: true},
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
		{Name: "a", Status: doctor.StatusPass, Message: "ok", Blocking: false},
		{Name: "b", Status: doctor.StatusWarn, Message: "warn", Blocking: false},
		{Name: "c", Status: doctor.StatusFail, Message: "fail", Blocking: true},
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
		{Name: "config", Status: doctor.StatusPass, Message: "ok", Blocking: true},
	}
	var buf bytes.Buffer
	err := printDiagnosticsText(results, &buf)
	assert.NoError(t, err)
}

// --- printDiagnosticsJSON ---

func TestPrintDiagnosticsJSON_ValidJSON(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusFail, Message: "bad config", Blocking: true},
		{Name: "providers", Status: doctor.StatusWarn, Message: "no providers", Blocking: false},
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
		{Name: "config", Status: doctor.StatusPass, Message: "ok", Blocking: true},
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

// --- NO_COLOR integration via resolveFormat ---

func TestResolveFormat_NOCOLOREmpty_ReturnsHuman(t *testing.T) {
	// Ensure NO_COLOR is not set.
	t.Setenv("NO_COLOR", "")
	got, err := resolveFormat("")
	require.NoError(t, err)
	assert.Equal(t, "", got, "empty string signals human mode")
}

func TestResolveFormat_NOCOLORNonEmpty_ReturnsText(t *testing.T) {
	t.Setenv("NO_COLOR", "true")
	got, err := resolveFormat("")
	require.NoError(t, err)
	assert.Equal(t, complytime.OutputFormatText, got)
}

func TestPrintDiagnosticsText_GrepPattern(t *testing.T) {
	results := []doctor.CheckResult{
		{Name: "config", Status: doctor.StatusFail, Message: "missing policy", Blocking: true},
		{Name: "cache", Status: doctor.StatusPass, Message: "ok", Blocking: false},
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
		{Name: "check", Status: doctor.StatusPass, Message: "ok", Blocking: false},
	}
	failBlocking := []doctor.CheckResult{
		{Name: "check", Status: doctor.StatusFail, Message: "broken", Blocking: true},
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
}
