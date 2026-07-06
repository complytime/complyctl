// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/complytime/complyctl/internal/cache"
)

func TestCacheRetentionCount_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		setEnv   bool
		expected int
	}{
		{
			name:     "default when env not set",
			setEnv:   false,
			expected: 1,
		},
		{
			name:     "env var set to 3",
			envVal:   "3",
			setEnv:   true,
			expected: 3,
		},
		{
			name:     "empty string falls back to default",
			envVal:   "",
			setEnv:   true,
			expected: 1,
		},
		{
			name:     "non-integer value falls back to default",
			envVal:   "abc",
			setEnv:   true,
			expected: 1,
		},
		{
			name:     "zero clamped to 1",
			envVal:   "0",
			setEnv:   true,
			expected: 1,
		},
		{
			name:     "negative value clamped to 1",
			envVal:   "-1",
			setEnv:   true,
			expected: 1,
		},
		{
			name:     "very large value overflows falls back to default",
			envVal:   fmt.Sprintf("%d0", math.MaxInt64),
			setEnv:   true,
			expected: 1,
		},
		{
			name:     "valid value 10",
			envVal:   "10",
			setEnv:   true,
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv("COMPLYTIME_CACHE_VERSIONS", tt.envVal)
			}
			got := cache.CacheRetentionCount()
			assert.Equal(t, tt.expected, got)
		})
	}
}
