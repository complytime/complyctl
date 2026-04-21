// SPDX-License-Identifier: Apache-2.0

package output

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gemaraproj/go-gemara"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/terminal"
	"github.com/complytime/complyctl/pkg/provider"
)

type nonPassingEntry struct {
	targetID      string
	requirementID string
	controlID     string
	result        gemara.Result
	emoji         string
	message       string
}

func nonPassingSortPriority(r gemara.Result) int {
	switch r {
	case gemara.Failed:
		return 1
	case gemara.Unknown:
		return 2
	case gemara.NeedsReview:
		return 3
	case gemara.NotApplicable, gemara.NotRun:
		return 4
	default:
		return 5
	}
}

// matchingStepMessage returns the message from the first step whose result
// matches the aggregated outcome. Falls back to the first step's message.
// See R45: scanning provider authors control the failure text.
func matchingStepMessage(steps []provider.Step, target gemara.Result) string {
	for _, s := range steps {
		if providerResultToGemara(s.Result) == target {
			return s.Message
		}
	}
	if len(steps) > 0 {
		return steps[0].Message
	}
	return ""
}

// FormatScanSummary builds a report-style post-scan output per FR-037.
// Intro text, plain aligned text table of non-passing results, compact totals.
// See spec.md Session 2026-02-26e.
func FormatScanSummary(assessments []provider.AssessmentLog, assessmentTargets []string, reqToControl map[string]string, policyID string, targetIDs []string) string {
	var passCount, failCount, skipCount, errCount int
	var entries []nonPassingEntry

	for i := range assessments {
		a := &assessments[i]
		result := aggregateResultFromSteps(a.Steps)

		ctrlID := reqToControl[a.RequirementID]
		if ctrlID == "" {
			ctrlID = "-"
		}

		targetID := "-"
		if i < len(assessmentTargets) {
			targetID = assessmentTargets[i]
		}

		switch result {
		case gemara.Passed:
			passCount++
		case gemara.Failed:
			failCount++
			entries = append(entries, nonPassingEntry{
				targetID:      targetID,
				requirementID: a.RequirementID,
				controlID:     ctrlID,
				result:        result,
				emoji:         complytime.StatusFailed,
				message:       matchingStepMessage(a.Steps, result),
			})
		case gemara.NotApplicable, gemara.NotRun:
			skipCount++
			entries = append(entries, nonPassingEntry{
				targetID:      targetID,
				requirementID: a.RequirementID,
				controlID:     ctrlID,
				result:        result,
				emoji:         complytime.StatusSkipped,
				message:       matchingStepMessage(a.Steps, result),
			})
		default:
			errCount++
			entries = append(entries, nonPassingEntry{
				targetID:      targetID,
				requirementID: a.RequirementID,
				controlID:     ctrlID,
				result:        result,
				emoji:         complytime.StatusError,
				message:       matchingStepMessage(a.Steps, result),
			})
		}
	}

	slices.SortStableFunc(entries, func(a, b nonPassingEntry) int {
		return nonPassingSortPriority(a.result) - nonPassingSortPriority(b.result)
	})

	total := len(assessments)
	intro := fmt.Sprintf("Scan: %s | Target: %s | %d requirements",
		policyID, strings.Join(targetIDs, ", "), total)

	headers := []string{"TARGET ID", "REQUIREMENT ID", "CONTROL ID", "STATUS", "MESSAGE"}
	var rows [][]string
	for _, e := range entries {
		rows = append(rows, []string{e.targetID, e.requirementID, e.controlID, e.emoji, e.message})
	}

	conclusion := fmt.Sprintf("%d requirements: %d passed, %d failed, %d skipped, %d error",
		total, passCount, failCount, skipCount, errCount)

	var b strings.Builder
	fmt.Fprintln(&b, intro)
	if len(rows) > 0 {
		fmt.Fprintln(&b)
		terminal.ShowPlainTable(&b, headers, rows)
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, conclusion)
	return b.String()
}
