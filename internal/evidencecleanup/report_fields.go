// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// Typed JSON decoding matches field names case-insensitively. Check each object
// before that decoding can overwrite references with a differently cased key.
func validateReportFields(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("inspect report fields: %w", err)
	}
	return validateReportObject(value)
}

func validateReportObject(value any) error {
	switch value := value.(type) {
	case map[string]any:
		seen := make(map[string]string, len(value))
		for name, child := range value {
			folded := strings.Map(foldReportRune, name)
			if previous, exists := seen[folded]; exists {
				return fmt.Errorf("ambiguous report field names %q and %q", previous, name)
			}
			seen[folded] = name
			if err := validateReportObject(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := validateReportObject(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func foldReportRune(value rune) rune {
	minimum := value
	for next := unicode.SimpleFold(value); next != value; next = unicode.SimpleFold(next) {
		if next < minimum {
			minimum = next
		}
	}
	return minimum
}
