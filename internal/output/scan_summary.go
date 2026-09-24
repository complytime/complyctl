// SPDX-License-Identifier: Apache-2.0

package output

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
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
		rows = append(rows, []string{e.targetID, e.requirementID, e.controlID, e.emoji + " " + e.result.String(), e.message})
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

// MappingCollision records a single merge collision between mapping
// references sharing the same ID. It captures enough context for
// formatting user-facing warnings.
type MappingCollision struct {
	ID             string // colliding reference ID
	RetainedTitle  string // title of the entry that was kept
	DiscardedTitle string // title of the entry that was dropped
	// PolicyWins is true when a policy ref beat a provider ref,
	// false when an earlier provider ref beat a later one.
	PolicyWins bool
}

// MergeMappingReferences combines policy-defined and provider-reported
// mapping references into a single deduplicated slice. Policy refs
// always take priority: when a provider ref shares an ID with a policy
// ref, the policy ref is retained. Among provider refs, first-wins.
// A collision note is prepended to the retained entry's Description
// and a MappingCollision is recorded for warning channels.
func MergeMappingReferences(
	policyRefs []gemara.MappingReference,
	providerRefs []provider.MappingReference,
) ([]gemara.MappingReference, []MappingCollision) {
	seen := make(map[string]int)
	isPolicyOrigin := make(map[string]bool)
	var result []gemara.MappingReference
	var collisions []MappingCollision

	// Policy refs take priority — add them first.
	for _, pr := range policyRefs {
		seen[pr.Id] = len(result)
		isPolicyOrigin[pr.Id] = true
		result = append(result, pr)
	}

	// Process provider refs: convert, deduplicate, record collisions.
	for _, pr := range providerRefs {
		if pr.ID == "" {
			log.Warn(
				"discarding provider mapping reference"+
					" with empty id",
				"title", pr.Title,
			)
			continue
		}

		idx, exists := seen[pr.ID]
		if exists {
			retainedBy := "policy ref"
			policyWins := isPolicyOrigin[pr.ID]
			if !policyWins {
				retainedBy = "earlier provider ref"
			}

			collisions = append(collisions, MappingCollision{
				ID:             pr.ID,
				RetainedTitle:  result[idx].Title,
				DiscardedTitle: pr.Title,
				PolicyWins:     policyWins,
			})

			note := buildCollisionNote(
				pr.ID, pr.Title, retainedBy,
			)
			result[idx].Description = note +
				result[idx].Description
			continue
		}

		seen[pr.ID] = len(result)
		result = append(result, providerRefToGemara(pr))
	}

	return result, collisions
}

// providerRefToGemara converts a provider.MappingReference to the
// equivalent gemara.MappingReference type.
func providerRefToGemara(
	pr provider.MappingReference,
) gemara.MappingReference {
	return gemara.MappingReference{
		Id:          pr.ID,
		Title:       pr.Title,
		Version:     pr.Version,
		Description: pr.Description,
		Url:         pr.URL,
	}
}

// buildCollisionNote constructs a human-readable prefix for the
// Description field of a retained mapping reference when a collision
// occurs. When title is non-empty it is included in the note.
func buildCollisionNote(
	id, title, retainedBy string,
) string {
	if title != "" {
		return fmt.Sprintf(
			"collision: provider ref '%s' (id: '%s')"+
				" discarded in favor of %s; ",
			title, id, retainedBy,
		)
	}
	return fmt.Sprintf(
		"collision: provider ref (id: '%s')"+
			" discarded in favor of %s; ",
		id, retainedBy,
	)
}

// FormatMappingCollisions formats mapping reference collisions as a
// distinct warnings block for stderr. Returns empty string when there
// are no collisions.
func FormatMappingCollisions(
	collisions []MappingCollision,
) string {
	if len(collisions) == 0 {
		return ""
	}
	var b strings.Builder
	noun := "collisions"
	if len(collisions) == 1 {
		noun = "collision"
	}
	fmt.Fprintf(
		&b,
		"\nWARNING: %d mapping reference %s"+
			" during merge:\n",
		len(collisions), noun,
	)
	for _, c := range collisions {
		retained := "policy ref"
		if !c.PolicyWins {
			retained = "earlier provider ref"
		}
		if c.DiscardedTitle != "" {
			fmt.Fprintf(
				&b,
				"  - id '%s': provider ref '%s'"+
					" discarded in favor of %s\n",
				c.ID, c.DiscardedTitle, retained,
			)
		} else {
			fmt.Fprintf(
				&b,
				"  - id '%s': provider ref"+
					" discarded in favor of %s\n",
				c.ID, retained,
			)
		}
	}
	fmt.Fprintln(&b)
	return b.String()
}
