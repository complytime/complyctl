// SPDX-License-Identifier: Apache-2.0

package output

import (
	"fmt"
	"time"

	"github.com/complytime/complyctl/internal/complytime"
)

// BuildReportFilename constructs a deterministic report filename from the given
// components. It sanitizes policyID and targetID via complytime.FilenameSafe
// (replacing path separators with dashes) and appends a timestamp. When targetID
// is empty the segment is omitted without producing double dashes.
//
// Examples:
//
//	BuildReportFilename("evaluation-log", "pol/v1", "web", "yaml")
//	  → "evaluation-log-pol-v1-web-20260729-143012.yaml"
//
//	BuildReportFilename("report", "pol", "", "md")
//	  → "report-pol-20260729-143012.md"
func BuildReportFilename(prefix, policyID, targetID, ext string) string {
	safePolicyID := complytime.FilenameSafe(policyID)
	ts := time.Now().Format("20060102-150405")

	if targetID != "" {
		safeTargetID := complytime.FilenameSafe(targetID)
		return fmt.Sprintf("%s-%s-%s-%s.%s",
			prefix, safePolicyID, safeTargetID, ts, ext)
	}
	return fmt.Sprintf("%s-%s-%s.%s", prefix, safePolicyID, ts, ext)
}
