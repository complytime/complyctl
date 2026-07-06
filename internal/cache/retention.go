// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"os"
	"strconv"

	"github.com/charmbracelet/log"

	"github.com/complytime/complyctl/internal/complytime"
)

// defaultRetentionCount is the number of complypack versions retained per
// evaluator-id when COMPLYTIME_CACHE_VERSIONS is not set. A value of 1
// preserves the current single-version-per-evaluator behavior (FR-002).
const defaultRetentionCount = 1

// CacheRetentionCount reads the COMPLYTIME_CACHE_VERSIONS environment
// variable and returns the configured retention count. Returns 1 when
// the variable is unset, empty, or contains an invalid value. Values
// less than 1 are clamped to 1. Invalid values produce a warning via
// charmbracelet/log.
func CacheRetentionCount() int {
	raw := os.Getenv(complytime.CacheVersionsEnvVar)
	if raw == "" {
		return defaultRetentionCount
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		log.Warn("invalid COMPLYTIME_CACHE_VERSIONS value, using default",
			"value", raw, "default", defaultRetentionCount)
		return defaultRetentionCount
	}

	if n < 1 {
		log.Warn("COMPLYTIME_CACHE_VERSIONS below minimum, clamping to 1",
			"value", n)
		return 1
	}

	return n
}
