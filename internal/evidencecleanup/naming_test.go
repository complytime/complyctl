// SPDX-License-Identifier: Apache-2.0

package evidencecleanup_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/evidencecleanup"
)

func TestIsLabel_ProviderGrammar(t *testing.T) {
	for _, tt := range []struct {
		name  string
		valid bool
	}{
		{"ampel", true}, {"OPA_target-01", true}, {"openscap", true},
		{"_", true}, {"-", true}, {"0", true}, {strings.Repeat("a", 80), true},
		{"", false}, {strings.Repeat("a", 81), false}, {".", false}, {"..", false},
		{"nested/label", false}, {`nested\label`, false}, {"a.b", false},
		{" label", false}, {"label ", false}, {"a\nb", false}, {"a\x00b", false},
		{"café", false}, {"é", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, evidencecleanup.IsLabel(tt.name))
		})
	}
}

func TestParseEvaluationName_ProviderContract(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef"
	valid := "evaluation-20261118T120000Z-" + token
	for _, tt := range []struct {
		name     string
		input    string
		expected time.Time
		errText  string
	}{
		{"valid", valid, time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC), ""},
		{"leap day", "evaluation-20240229T235959Z-" + token,
			time.Date(2024, 2, 29, 23, 59, 59, 0, time.UTC), ""},
		{"future is syntactically valid", "evaluation-20990101T000000Z-" + token,
			time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), ""},
		{"empty", "", time.Time{}, "unrecognized evaluation name"},
		{"prefix", strings.Replace(valid, "evaluation-", "scan-", 1),
			time.Time{}, "unrecognized evaluation name"},
		{"path", "label/" + valid, time.Time{}, "unrecognized evaluation name"},
		{"trailing path", valid + "/evidence.json", time.Time{}, "unrecognized evaluation name"},
		{"uppercase token", strings.ToUpper(valid), time.Time{}, "unrecognized evaluation name"},
		{"uppercase hex", "evaluation-20261118T120000Z-" + strings.ToUpper(token),
			time.Time{}, "unrecognized evaluation name"},
		{"short token", valid[:len(valid)-1], time.Time{}, "unrecognized evaluation name"},
		{"empty token", "evaluation-20261118T120000Z-",
			time.Time{}, "unrecognized evaluation name"},
		{"long token", valid + "a", time.Time{}, "unrecognized evaluation name"},
		{"non hex", valid[:len(valid)-1] + "g", time.Time{}, "unrecognized evaluation name"},
		{"extra suffix", valid + ".json", time.Time{}, "unrecognized evaluation name"},
		{"missing separator", "evaluation-20261118T120000Z" + token,
			time.Time{}, "unrecognized evaluation name"},
		{"impossible date", "evaluation-20260229T120000Z-" + token,
			time.Time{}, "parse evaluation collection-start time"},
		{"invalid hour", "evaluation-20261118T240000Z-" + token,
			time.Time{}, "parse evaluation collection-start time"},
		{"invalid second", "evaluation-20261118T120060Z-" + token,
			time.Time{}, "parse evaluation collection-start time"},
		{"fraction", "evaluation-20261118T120000.5Z-" + token,
			time.Time{}, "noncanonical evaluation timestamp"},
		{"comma fraction", "evaluation-20261118T120000,5Z-" + token,
			time.Time{}, "noncanonical evaluation timestamp"},
		{"short hour", "evaluation-20261118T10000Z-" + token,
			time.Time{}, "parse evaluation collection-start time"},
		{"numeric utc offset", "evaluation-20261118T120000+0000-" + token,
			time.Time{}, "parse evaluation collection-start time"},
		{"lowercase utc", "evaluation-20261118T120000z-" + token,
			time.Time{}, "parse evaluation collection-start time"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evidencecleanup.ParseEvaluationName(tt.input)
			if tt.errText != "" {
				require.ErrorContains(t, err, tt.errText)
				assert.True(t, got.IsZero(), "invalid names must not yield a collection-start time")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
			assert.Same(t, time.UTC, got.Location())
		})
	}
}
