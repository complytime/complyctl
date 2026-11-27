// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseDays accepts only canonical [1-9][0-9]*d thresholds and returns their
// duration in exact 24-hour days. Invalid input and counts exceeding the duration
// representation return zero and an error, without arithmetic overflow.
func ParseDays(value string) (time.Duration, error) {
	count, found := strings.CutSuffix(value, daySuffix)
	if !found || count == "" || count[0] == '0' || strings.Trim(count, decimalDigits) != "" {
		return 0, fmt.Errorf("invalid age threshold %q: expected a positive whole number followed by d", value)
	}
	days, err := strconv.ParseInt(count, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse age threshold %q: %w", value, err)
	}
	if days > maxAgeDays {
		return 0, fmt.Errorf("age threshold %q exceeds maximum of %dd", value, maxAgeDays)
	}
	return time.Duration(days) * dayDuration, nil
}

// Clock supplies the invocation time independently of the system clock.
type Clock interface {
	Now() time.Time
}

// AgeOptions configures an invocation's threshold and required clock.
type AgeOptions struct {
	OlderThan string
	Clock     Clock
}

// AgeWindow holds one immutable UTC invocation time and cutoff for all members.
// Its zero value never selects an evaluation.
type AgeWindow struct {
	capturedAt time.Time
	cutoff     time.Time
}

// NewAgeWindow validates opts before reading its clock exactly once. It returns
// an error for an invalid threshold or missing clock. Subtraction uses durations,
// not calendar days, so time zones and daylight-saving transitions cannot alter age.
func NewAgeWindow(opts AgeOptions) (*AgeWindow, error) {
	age, err := ParseDays(opts.OlderThan)
	if err != nil {
		return nil, fmt.Errorf("create age window: %w", err)
	}
	if opts.Clock == nil {
		return nil, fmt.Errorf("create age window: clock is required")
	}
	capturedAt := opts.Clock.Now().UTC()
	return &AgeWindow{capturedAt: capturedAt, cutoff: capturedAt.Add(-age)}, nil
}

// CapturedAt returns the fixed UTC invocation time.
func (w AgeWindow) CapturedAt() time.Time {
	return w.capturedAt
}

// Cutoff returns the fixed UTC age cutoff, which is exclusive.
func (w AgeWindow) Cutoff() time.Time {
	return w.cutoff
}

// IsOlder reports whether a parsed evaluation start is strictly before the
// cutoff. Exact-cutoff, newer, and future starts are excluded. This age predicate
// does not establish group completeness, inactivity, or filesystem safety.
func (w AgeWindow) IsOlder(startedAt time.Time) bool {
	return w.cutoff.Before(w.capturedAt) && startedAt.Before(w.cutoff)
}
