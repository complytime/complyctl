// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"fmt"

	"github.com/complytime/complyctl/internal/workspacecoord"
)

// Options selects an existing workspace and explicit age threshold for execution.
// Clock is required and captured once. Execution never accepts a saved preview.
type Options struct {
	Workspace string
	OlderThan string
	Clock     Clock
}

// ExecutionStatus describes actual progress, not an attempted deletion.
type ExecutionStatus string

const (
	// ExecutionRetained means this invocation did not unlink anything in the member.
	ExecutionRetained ExecutionStatus = "retained"
	// ExecutionDeleted means the complete member, including its root, was removed.
	ExecutionDeleted ExecutionStatus = "deleted"
	// ExecutionPartial means some contained entries, but not the complete member, were removed.
	ExecutionPartial ExecutionStatus = "partially-deleted"
)

// MemberResult records a member's actual outcome. RemovedPaths lists successful
// descriptor-relative unlinks in execution order, including directories. Failure
// names the unexpected error for this member; an unattempted member has none.
type MemberResult struct {
	Path         string
	Kind         MemberKind
	Status       ExecutionStatus
	Reasons      []string
	RemovedPaths []string
	Failure      string
}

// GroupExecution records stable path-ordered membership and any group failure.
// PartialLoss means at least one unlink succeeded in a failed group. No rollback
// is attempted or promised. Deleted is true only for a completely removed group.
type GroupExecution struct {
	Members     []MemberResult
	Blockers    []GroupBlocker
	Deleted     bool
	PartialLoss bool
	Failure     string
}

// ExecutionTotals counts complete members separately from partially removed ones.
// RetainedMembers includes failed members with zero successful unlinks.
type ExecutionTotals struct {
	DeletedGroups   int
	RetainedGroups  int
	FailedGroups    int
	DeletedMembers  int
	RetainedMembers int
	PartialMembers  int
	ExcludedEntries int
}

// Result exposes actual execution progress even when Run returns an error.
// Groups and members use preview's stable ordering. RemovedPaths is the precise
// progress of this invocation, not a claim about subsequent external mutations.
type Result struct {
	Workspace  string
	Age        AgeWindow
	Groups     []GroupExecution
	Exclusions []DiscoveryExclusion
	Totals     ExecutionTotals
}

// Run holds exclusive workspace access across fresh complete inventory, whole-
// group preflight, and evidence-first removal. Busy/activity/unknown inventory
// failures occur before mutation. Bounded blocked groups are retained while
// independent safe groups proceed. Any safety, removal, or close failure returns
// an error; age exclusions, no matches, and repeated successful cleanup do not.
func Run(opts Options) (*Result, error) {
	return run(opts, executeGroups)
}

func run(opts Options,
	execute func(*workspacecoord.Access, *GroupResult, *Result) error,
) (result *Result, err error) {
	access, err := workspacecoord.Acquire(workspacecoord.Options{
		Workspace: opts.Workspace, Mode: workspacecoord.Exclusive,
	})
	if err != nil {
		return nil, fmt.Errorf("execute cleanup: %w", err)
	}
	defer func() { err = errors.Join(err, access.Close()) }()
	// Capture the cutoff within the protected inventory interval, rather than
	// letting an injected clock run outside coordination (Simplicity & Isolation).
	age, err := NewAgeWindow(AgeOptions{OlderThan: opts.OlderThan, Clock: opts.Clock})
	if err != nil {
		return nil, fmt.Errorf("execute cleanup: %w", err)
	}
	if err := access.CheckCleanupSupport(); err != nil {
		return nil, fmt.Errorf("execute cleanup: %w", err)
	}
	if err := access.CheckNoActivity(); err != nil {
		return nil, fmt.Errorf("execute cleanup: %w", err)
	}
	groups, err := PreflightGroups(GroupOptions{Workspace: access.Path(), Age: *age})
	if err != nil {
		return nil, fmt.Errorf("execute cleanup in workspace %q: %w", access.Path(), err)
	}
	if err := access.Validate(); err != nil {
		return nil, fmt.Errorf("revalidate cleanup workspace: %w", err)
	}
	plan := planPreview(access.Path(), *age, groups)
	result = executionResult(plan)
	err = errors.Join(previewBlockerError(plan), execute(access, groups, result))
	result.Totals = executionTotals(result)
	return result, err
}

func executionResult(plan *PreviewResult) *Result {
	result := &Result{Workspace: plan.Workspace, Age: plan.Age, Exclusions: plan.Exclusions}
	for _, group := range plan.Groups {
		outcome := GroupExecution{Blockers: group.Blockers}
		for _, member := range group.Members {
			reasons := member.Reasons
			if member.Status == StatusWouldDelete {
				reasons = []string{"not removed; group execution has not reached this member"}
			}
			outcome.Members = append(outcome.Members, MemberResult{
				Path: member.Path, Kind: member.Kind, Status: ExecutionRetained, Reasons: reasons,
			})
		}
		result.Groups = append(result.Groups, outcome)
	}
	return result
}

func executionTotals(result *Result) ExecutionTotals {
	totals := ExecutionTotals{ExcludedEntries: len(result.Exclusions)}
	for _, group := range result.Groups {
		switch {
		case group.Failure != "":
			totals.FailedGroups++
		case group.Deleted:
			totals.DeletedGroups++
		default:
			totals.RetainedGroups++
		}
		for _, member := range group.Members {
			switch member.Status {
			case ExecutionDeleted:
				totals.DeletedMembers++
			case ExecutionPartial:
				totals.PartialMembers++
			default:
				totals.RetainedMembers++
			}
		}
	}
	return totals
}
