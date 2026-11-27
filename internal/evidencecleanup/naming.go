// SPDX-License-Identifier: Apache-2.0

// Package evidencecleanup defines selection rules for workspace evidence cleanup.
// Naming and age matches alone do not authorize discovery or deletion.
package evidencecleanup

import (
	"fmt"
	"strings"
	"time"
)

// IsLabel reports whether name is one provider label component: 1–80 ASCII
// letters, digits, underscores, or hyphens. It does not accept a path.
func IsLabel(name string) bool {
	return len(name) > 0 && len(name) <= maxLabelSize && strings.Trim(name, labelCharacters) == ""
}

// ParseEvaluationName returns the UTC collection-start time encoded in a single
// evaluation directory name. Unknown formats, invalid dates, and tokens other
// than exactly 32 lowercase hexadecimal characters return an error. Future
// timestamps are parsed here; AgeWindow applies invocation-specific age rules.
func ParseEvaluationName(name string) (time.Time, error) {
	remainder, found := strings.CutPrefix(name, evaluationPrefix)
	timestamp, token, separated := strings.Cut(remainder, evaluationSeparator)
	if !found || !separated || len(token) != evaluationTokenSize ||
		strings.Trim(token, lowercaseHexDigits) != "" {
		return time.Time{}, fmt.Errorf("unrecognized evaluation name %q", name)
	}

	startedAt, err := time.Parse(evaluationTimeLayout, timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse evaluation collection-start time in %q: %w", name, err)
	}
	// Readability First: an exact round trip rejects permissive time parsing
	// rather than maintaining a second implementation of calendar validation.
	if startedAt.Format(evaluationTimeLayout) != timestamp {
		return time.Time{}, fmt.Errorf("noncanonical evaluation timestamp in %q", name)
	}
	return startedAt.UTC(), nil
}
