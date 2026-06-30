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

type summaryEntry struct {
	targetID      string
	requirementID string
	controlID     string
	result        gemara.Result
	emoji         string
	message       string
}

func sortPriority(r gemara.Result) int {
	switch r {
	case gemara.Failed:
		return 1
	case gemara.Unknown:
		return 2
	case gemara.NeedsReview:
		return 3
	case gemara.NotApplicable, gemara.NotRun:
		return 4
	case gemara.Passed:
		return 6
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

// FormatScanSummary builds a report-style post-scan output.
// Intro text, plain aligned text table of results, compact totals.
// When showPassing is true, passing controls are included in the table;
// when false, only non-passing results are shown. Pass counts are always
// included in the totals line regardless of showPassing.
func FormatScanSummary(assessments []provider.AssessmentLog, assessmentTargets []string, reqToControl map[string]string, policyID string, targetIDs []string, showPassing bool) string {
	var passCount, failCount, notApplicableCount, skipCount, errCount int
	var entries []summaryEntry

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
			entries = append(entries, summaryEntry{
				targetID:      targetID,
				requirementID: a.RequirementID,
				controlID:     ctrlID,
				result:        result,
				emoji:         complytime.StatusPassed,
				message:       matchingStepMessage(a.Steps, result),
			})
		case gemara.Failed:
			failCount++
			entries = append(entries, summaryEntry{
				targetID:      targetID,
				requirementID: a.RequirementID,
				controlID:     ctrlID,
				result:        result,
				emoji:         complytime.StatusFailed,
				message:       matchingStepMessage(a.Steps, result),
			})
		case gemara.NotApplicable:
			notApplicableCount++
			entries = append(entries, summaryEntry{
				targetID:      targetID,
				requirementID: a.RequirementID,
				controlID:     ctrlID,
				result:        result,
				emoji:         complytime.StatusSkipped,
				message:       matchingStepMessage(a.Steps, result),
			})
		case gemara.NotRun:
			skipCount++
			entries = append(entries, summaryEntry{
				targetID:      targetID,
				requirementID: a.RequirementID,
				controlID:     ctrlID,
				result:        result,
				emoji:         complytime.StatusSkipped,
				message:       matchingStepMessage(a.Steps, result),
			})
		default:
			errCount++
			entries = append(entries, summaryEntry{
				targetID:      targetID,
				requirementID: a.RequirementID,
				controlID:     ctrlID,
				result:        result,
				emoji:         complytime.StatusError,
				message:       matchingStepMessage(a.Steps, result),
			})
		}
	}

	// Filter out passing entries when showPassing is false.
	// Pass count is already accumulated above for the totals line.
	if !showPassing {
		entries = slices.DeleteFunc(entries, func(e summaryEntry) bool {
			return e.result == gemara.Passed
		})
	}

	slices.SortStableFunc(entries, func(a, b summaryEntry) int {
		return sortPriority(a.result) - sortPriority(b.result)
	})

	total := len(assessments)
	intro := fmt.Sprintf("Scan: %s | Target: %s | %d requirements",
		policyID, strings.Join(targetIDs, ", "), total)

	headers := []string{"TARGET ID", "REQUIREMENT ID", "CONTROL ID", "STATUS", "MESSAGE"}
	var rows [][]string
	for _, e := range entries {
		rows = append(rows, []string{e.targetID, e.requirementID, e.controlID, e.emoji, e.message})
	}

	conclusion := fmt.Sprintf("%d requirements: %d passed, %d failed, %d not applicable, %d skipped, %d errors",
		total, passCount, failCount, notApplicableCount, skipCount, errCount)

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

// NothingAssessed returns true when no requirements received a definitive
// pass or fail result, indicating the scan produced no actionable compliance signal.
func NothingAssessed(assessments []provider.AssessmentLog) bool {
	for i := range assessments {
		result := aggregateResultFromSteps(assessments[i].Steps)
		if result == gemara.Passed || result == gemara.Failed {
			return false
		}
	}
	return true
}

// FormatOperationalWarnings formats provider-reported operational errors
// as a distinct warnings block for stderr. Returns empty string when there
// are no errors.
func FormatOperationalWarnings(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	var b strings.Builder
	noun := "errors"
	if len(errors) == 1 {
		noun = "error"
	}
	fmt.Fprintf(&b, "\nWARNING: %d operational %s during scan:\n", len(errors), noun)
	for _, e := range errors {
		fmt.Fprintf(&b, "  - %s\n", e)
	}
	fmt.Fprintln(&b)
	return b.String()
}
