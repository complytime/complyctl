// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/complytime/complyctl/internal/workspacecoord"
)

// ErrPreflightBlocked indicates bounded safety/access failures. Preview still
// returns every group, including independent eligible groups, alongside this error.
var ErrPreflightBlocked = errors.New("cleanup group preflight blocked")

// MemberKind distinguishes indivisible evaluation directories from managed reports.
type MemberKind string

const (
	// MemberEvaluation includes the evaluation directory and all its contents.
	MemberEvaluation MemberKind = "evaluation"
	// MemberReport identifies one managed report file.
	MemberReport MemberKind = "report"
)

// MemberStatus describes a preview decision, never a completed removal.
type MemberStatus string

const (
	// StatusWouldDelete means every member passed whole-group preflight.
	StatusWouldDelete MemberStatus = "would-delete"
	// StatusRetained means at least one member blocks the entire group.
	StatusRetained MemberStatus = "retained"
)

// PlannedMember identifies a workspace-relative member and its group-wide status.
// Reasons name this member's blockers, or explain its retention/eligibility when
// the blockers belong to another member. Evaluation contents are never split.
type PlannedMember struct {
	Path    string
	Kind    MemberKind
	Status  MemberStatus
	Reasons []string
}

// PlannedGroup contains every member and blocker in path order. Blockers with
// the same path are ordered by reason, then age-exclusion flag. One blocker
// retains all members; independent groups keep their own decisions.
type PlannedGroup struct {
	Members  []PlannedMember
	Blockers []GroupBlocker
}

// PreviewTotals counts whole groups and indivisible members, not contained files.
// ExcludedEntries counts entries outside the managed groups.
type PreviewTotals struct {
	WouldDeleteGroups  int
	RetainedGroups     int
	WouldDeleteMembers int
	RetainedMembers    int
	ExcludedEntries    int
}

// PreviewOptions selects an existing workspace and an explicit day threshold.
// Clock is required and read exactly once after threshold validation.
type PreviewOptions struct {
	Workspace string
	OlderThan string
	Clock     Clock
}

// PreviewResult is a read-only snapshot with canonical workspace identity and
// one UTC age window. Groups are ordered by their first member path; exclusions
// by path then reason. No descriptors or removal authority escape in the result.
// A later execution must reacquire access and repeat inventory and preflight;
// preflight does not guarantee future I/O success or transactional deletion.
type PreviewResult struct {
	Workspace  string
	Age        AgeWindow
	Groups     []PlannedGroup
	Exclusions []DiscoveryExclusion
	Totals     PreviewTotals
}

// Preview obtains exclusive access before inventory and holds it through planning.
// It reuses complete group preflight without initializing config, providers,
// registries, logging, lock files, or activity records. Busy, unsupported, unsafe
// coordination, and incomplete inventory errors return nil results. Bounded
// group safety failures return a full result and ErrPreflightBlocked; ordinary
// age exclusions and no matches succeed. Workspace files remain unchanged.
func Preview(opts PreviewOptions) (*PreviewResult, error) {
	return preview(opts, PreflightGroups)
}

// Inject only the existing preflight boundary, keeping policy and coordination
// shared rather than inventing a separate dry-run inventory (FR-007).
func preview(opts PreviewOptions,
	preflight func(GroupOptions) (*GroupResult, error),
) (result *PreviewResult, err error) {
	age, err := NewAgeWindow(AgeOptions{OlderThan: opts.OlderThan, Clock: opts.Clock})
	if err != nil {
		return nil, fmt.Errorf("preview cleanup: %w", err)
	}
	access, err := workspacecoord.Acquire(workspacecoord.Options{
		Workspace: opts.Workspace, Mode: workspacecoord.Exclusive,
	})
	if err != nil {
		return nil, fmt.Errorf("preview cleanup: %w", err)
	}
	defer func() {
		if closeErr := access.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("release preview workspace access: %w", closeErr))
			result = nil
		}
	}()
	if err := access.CheckCleanupSupport(); err != nil {
		return nil, fmt.Errorf("preview cleanup: %w", err)
	}
	if err := access.CheckNoActivity(); err != nil {
		return nil, fmt.Errorf("preview cleanup: %w", err)
	}
	groups, err := preflight(GroupOptions{Workspace: access.Path(), Age: *age})
	if err != nil {
		return nil, fmt.Errorf("preview cleanup in workspace %q: %w", access.Path(), err)
	}
	if err := preparePreviewGroups(access, groups); err != nil {
		return nil, fmt.Errorf("prepare cleanup preview: %w", err)
	}
	result = planPreview(access.Path(), *age, groups)
	if err := access.Validate(); err != nil {
		return nil, fmt.Errorf("revalidate preview workspace: %w", err)
	}
	return result, previewBlockerError(result)
}

func planPreview(workspace string, age AgeWindow, groups *GroupResult) *PreviewResult {
	result := &PreviewResult{Workspace: workspace, Age: age,
		Exclusions: slices.Clone(groups.Exclusions)}
	slices.SortFunc(result.Exclusions, func(a, b DiscoveryExclusion) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Reason, b.Reason))
	})
	result.Totals.ExcludedEntries = len(result.Exclusions)
	for _, group := range groups.Groups {
		planned := planGroup(group)
		result.Groups = append(result.Groups, planned)
		if group.Eligible() {
			result.Totals.WouldDeleteGroups++
			result.Totals.WouldDeleteMembers += len(planned.Members)
		} else {
			result.Totals.RetainedGroups++
			result.Totals.RetainedMembers += len(planned.Members)
		}
	}
	slices.SortFunc(result.Groups, func(a, b PlannedGroup) int {
		return cmp.Compare(a.Members[0].Path, b.Members[0].Path)
	})
	return result
}

func planGroup(group Group) PlannedGroup {
	planned := PlannedGroup{Blockers: slices.Clone(group.Blockers)}
	slices.SortFunc(planned.Blockers, func(a, b GroupBlocker) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Reason, b.Reason),
			cmp.Compare(boolOrder(a.AgeExclusion), boolOrder(b.AgeExclusion)))
	})
	for _, evaluation := range group.Evaluations {
		planned.Members = append(planned.Members,
			planMember(evaluation.Path, MemberEvaluation, group.Eligible(), planned.Blockers))
	}
	for _, report := range group.Reports {
		planned.Members = append(planned.Members,
			planMember(report.Path, MemberReport, group.Eligible(), planned.Blockers))
	}
	slices.SortFunc(planned.Members, func(a, b PlannedMember) int {
		return cmp.Compare(a.Path, b.Path)
	})
	return planned
}

func boolOrder(value bool) int {
	if value {
		return 1
	}
	return 0
}

func planMember(path string, kind MemberKind, eligible bool, blockers []GroupBlocker) PlannedMember {
	member := PlannedMember{Path: path, Kind: kind, Status: StatusWouldDelete,
		Reasons: []string{"whole-group preflight passed; reports and evidence would expire together"}}
	if eligible {
		return member
	}
	member.Status = StatusRetained
	member.Reasons = nil
	for _, blocker := range blockers {
		if blocker.Path == path {
			member.Reasons = append(member.Reasons, blocker.Reason)
		}
	}
	if len(member.Reasons) == 0 {
		member.Reasons = []string{"entire group retained; see blocking members"}
	}
	return member
}

func previewBlockerError(result *PreviewResult) error {
	var err error
	for _, group := range result.Groups {
		for _, blocker := range group.Blockers {
			if !blocker.AgeExclusion {
				err = errors.Join(err, fmt.Errorf("%w in workspace %q: member %q: %s; entire group retained",
					ErrPreflightBlocked, result.Workspace, blocker.Path, blocker.Reason))
			}
		}
	}
	return err
}
