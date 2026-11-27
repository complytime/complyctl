// SPDX-License-Identifier: Apache-2.0

package evidencecleanup_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/evidencecleanup"
)

func TestParseDays_CanonicalThresholds(t *testing.T) {
	for _, tt := range []struct {
		input    string
		expected time.Duration
		errText  string
	}{
		{"1d", 24 * time.Hour, ""},
		{"30d", 720 * time.Hour, ""},
		{"106751d", 2562024 * time.Hour, ""},
		{"106752d", 0, "exceeds maximum of 106751d"},
		{"9223372036854775807d", 0, "exceeds maximum"},
		{"9223372036854775808d", 0, "value out of range"},
		{strings.Repeat("9", 100) + "d", 0, "value out of range"},
		{"", 0, "expected a positive whole number"},
		{"d", 0, "expected a positive whole number"},
		{"0d", 0, "expected a positive whole number"},
		{"-1d", 0, "expected a positive whole number"},
		{"+1d", 0, "expected a positive whole number"},
		{"1.5d", 0, "expected a positive whole number"},
		{"1.0d", 0, "expected a positive whole number"},
		{"01d", 0, "expected a positive whole number"},
		{"00d", 0, "expected a positive whole number"},
		{"30h", 0, "expected a positive whole number"},
		{"1D", 0, "expected a positive whole number"},
		{"1w", 0, "expected a positive whole number"},
		{"1", 0, "expected a positive whole number"},
		{"1dd", 0, "expected a positive whole number"},
		{"1d24h", 0, "expected a positive whole number"},
		{" 1d", 0, "expected a positive whole number"},
		{"1d ", 0, "expected a positive whole number"},
		{"1d\n", 0, "expected a positive whole number"},
		{"1\td", 0, "expected a positive whole number"},
		{"１d", 0, "expected a positive whole number"},
		{"1_000d", 0, "expected a positive whole number"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got, err := evidencecleanup.ParseDays(tt.input)
			if tt.errText != "" {
				require.ErrorContains(t, err, tt.errText)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseDays_IntegerOverflowPreservesCause(t *testing.T) {
	got, err := evidencecleanup.ParseDays("9223372036854775808d")
	require.Error(t, err)
	assert.Zero(t, got)
	assert.True(t, errors.Is(err, strconv.ErrRange))
}

type countingClock struct {
	now   time.Time
	calls int
}

func (c *countingClock) Now() time.Time {
	c.calls++
	return c.now
}

func TestNewAgeWindow_FixedUTCInvocation(t *testing.T) {
	for _, tt := range []struct {
		name      string
		now       time.Time
		threshold string
		cutoff    time.Time
	}{
		{"spec example", time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC), "30d",
			time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)},
		{"offset and nanoseconds", time.Date(2026, 11, 18, 7, 0, 0, 123,
			time.FixedZone("offset", -5*60*60)), "1d",
			time.Date(2026, 11, 17, 12, 0, 0, 123, time.UTC)},
		{"maximum duration", time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC), "106751d",
			time.Date(1734, 8, 10, 12, 0, 0, 0, time.UTC)},
		{"leap day", time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC), "1d",
			time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC)},
		{"year boundary", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "1d",
			time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clock := &countingClock{now: tt.now}
			window, err := evidencecleanup.NewAgeWindow(evidencecleanup.AgeOptions{
				OlderThan: tt.threshold, Clock: clock,
			})
			require.NoError(t, err)
			clock.now = clock.now.Add(48 * time.Hour)
			assert.Equal(t, tt.now.UTC(), window.CapturedAt())
			assert.Equal(t, tt.cutoff, window.Cutoff())
			assert.Same(t, time.UTC, window.Cutoff().Location())
			assert.True(t, window.IsOlder(tt.cutoff.Add(-time.Nanosecond)))
			assert.False(t, window.IsOlder(tt.cutoff))
			assert.Equal(t, 1, clock.calls, "candidate checks must never reread the clock")
		})
	}
}

func TestNewAgeWindow_InvalidOptions(t *testing.T) {
	for _, threshold := range []string{"", "0d", "01d", "1.5d", "30h", "106752d"} {
		t.Run(threshold, func(t *testing.T) {
			clock := &countingClock{}
			window, err := evidencecleanup.NewAgeWindow(evidencecleanup.AgeOptions{
				OlderThan: threshold, Clock: clock,
			})
			require.ErrorContains(t, err, "create age window")
			assert.Nil(t, window)
			assert.Zero(t, clock.calls, "validate before reading the clock")
		})
	}
	window, err := evidencecleanup.NewAgeWindow(evidencecleanup.AgeOptions{OlderThan: "1d"})
	require.ErrorContains(t, err, "clock is required")
	assert.Nil(t, window)
}

func TestAgeWindow_IsOlderStrictBoundaries(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	window, err := evidencecleanup.NewAgeWindow(evidencecleanup.AgeOptions{
		OlderThan: "30d", Clock: &countingClock{now: now},
	})
	require.NoError(t, err)
	cutoff := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name  string
		start time.Time
		older bool
	}{
		{"before", cutoff.Add(-time.Nanosecond), true},
		{"exact cutoff", cutoff, false},
		{"after", cutoff.Add(time.Nanosecond), false},
		{"invocation time", now, false},
		{"future", now.Add(time.Second), false},
		{"equivalent offset", cutoff.In(time.FixedZone("offset", 60*60)), false},
		{"older with offset", cutoff.Add(-time.Nanosecond).In(
			time.FixedZone("offset", -5*60*60)), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.older, window.IsOlder(tt.start))
		})
	}
	assert.False(t, (evidencecleanup.AgeWindow{}).IsOlder(cutoff))
}
