// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"math"
	"time"
)

const (
	dayDuration          = 24 * time.Hour
	maxAgeDays           = math.MaxInt64 / int64(dayDuration)
	daySuffix            = "d"
	decimalDigits        = "0123456789"
	evaluationPrefix     = "evaluation-"
	evaluationTimeLayout = "20060102T150405Z"
	evaluationSeparator  = "-"
	evaluationTokenSize  = 32
	lowercaseHexDigits   = "0123456789abcdef"
	labelCharacters      = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"
	maxLabelSize         = 80
)
