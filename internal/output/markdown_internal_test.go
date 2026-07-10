// SPDX-License-Identifier: Apache-2.0

package output

import (
	"testing"

	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"

	"github.com/complytime/complyctl/internal/complytime"
)

func TestResultEmoji(t *testing.T) {
	tests := []struct {
		name   string
		result gemara.Result
		want   string
	}{
		{"Passed", gemara.Passed, complytime.StatusPassed},
		{"Failed", gemara.Failed, complytime.StatusFailed},
		{"NotApplicable", gemara.NotApplicable, complytime.StatusSkipped},
		{"NotRun", gemara.NotRun, complytime.StatusSkipped},
		{"Unknown", gemara.Unknown, complytime.StatusError},
		{"NeedsReview", gemara.NeedsReview, complytime.StatusError},
		{"default", gemara.Result(99), complytime.StatusError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resultEmoji(tt.result)
			assert.Equal(t, tt.want, got)
		})
	}
}
