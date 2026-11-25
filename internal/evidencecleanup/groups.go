// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// GroupOptions selects an existing workspace and a previously captured age window.
// Callers must hold exclusive workspace access and check support/activity first,
// as required by InventoryReports. No configuration or provider is loaded.
type GroupOptions struct {
	Workspace string
	Age       AgeWindow
}

// GroupBlocker names a member preventing the entire group from qualifying.
// AgeExclusion distinguishes normal age retention from a failed safety check.
type GroupBlocker struct {
	Path         string
	Reason       string
	AgeExclusion bool
}

// Group is one connected component of complete evaluation units and reports.
// Blockers retain every member, including members which individually qualify.
// A preflight result is not deletion authority or a guarantee of future I/O.
type Group struct {
	Evaluations []Evaluation
	Reports     []ManagedReport
	Blockers    []GroupBlocker
}

// Eligible reports whether every member passed this invocation's preflight.
func (g Group) Eligible() bool {
	return len(g.Evaluations) > 0 && len(g.Blockers) == 0
}

// GroupResult contains preflighted groups, with no workspace mutations. Groups
// and members are path-ordered; exclusions remain outside the managed groups.
type GroupResult struct {
	Groups     []Group
	Exclusions []DiscoveryExclusion
}

// PreflightGroups obtains a fresh complete inventory before grouping or checking
// any reportless evaluation. Unbounded inventory uncertainty returns nil/error,
// never an actionable partial result. Bounded member failures become Blockers.
// All checks are read-only and do not reserve or authorize later removal.
func PreflightGroups(opts GroupOptions) (result *GroupResult, err error) {
	inventory, err := InventoryReports(DiscoveryOptions{Workspace: opts.Workspace})
	if err != nil {
		return nil, fmt.Errorf("preflight groups: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(opts.Workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve preflight workspace: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute preflight workspace: %w", err)
	}
	workspace, validate, err := openDiscoveryWorkspace(canonical)
	if err != nil {
		return nil, fmt.Errorf("open preflight workspace: %w", err)
	}
	defer func() {
		err = errors.Join(err, workspace.Close())
		if err != nil {
			result = nil
		}
	}()
	result = &GroupResult{Groups: buildGroups(inventory), Exclusions: inventory.Exclusions}
	for i := range result.Groups {
		preflightGroup(&result.Groups[i], opts.Age, func(path string, identity os.FileInfo) error {
			return preflightMember(workspace, path, identity)
		})
	}
	if err := validate(); err != nil {
		return nil, fmt.Errorf("revalidate preflight workspace: %w", err)
	}
	return result, nil
}

func buildGroups(inventory *ReportInventory) []Group {
	// Edges use evaluation paths, never contained file paths or report names.
	// Connected components (rather than per-report selection) preserve transitive
	// sharing per FR-015 while keeping grouping independent of filesystem I/O.
	adjacent := make(map[string][]string)
	evaluations := make(map[string]Evaluation)
	reports := make(map[string]ManagedReport)
	for _, evaluation := range inventory.Evaluations {
		evaluations[evaluation.Path] = evaluation
		adjacent[evaluation.Path] = nil
	}
	for _, report := range inventory.Reports {
		reports[report.Path] = report
		adjacent[report.Path] = append([]string(nil), report.Evaluations...)
		for _, path := range report.Evaluations {
			adjacent[path] = append(adjacent[path], report.Path)
		}
	}
	paths := make([]string, 0, len(adjacent))
	for path := range adjacent {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	var groups []Group
	visited := make(map[string]bool)
	for _, path := range paths {
		if visited[path] {
			continue
		}
		group := Group{}
		queue := []string{path}
		visited[path] = true
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if evaluation, ok := evaluations[current]; ok {
				group.Evaluations = append(group.Evaluations, evaluation)
			} else {
				group.Reports = append(group.Reports, reports[current])
			}
			for _, neighbor := range adjacent[current] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
		slices.SortFunc(group.Evaluations, func(a, b Evaluation) int { return strings.Compare(a.Path, b.Path) })
		slices.SortFunc(group.Reports, func(a, b ManagedReport) int { return strings.Compare(a.Path, b.Path) })
		groups = append(groups, group)
	}
	return groups
}

func preflightGroup(group *Group, age AgeWindow, check func(string, os.FileInfo) error) {
	block := func(path, reason string, ageExclusion bool) {
		group.Blockers = append(group.Blockers, GroupBlocker{path, reason, ageExclusion})
	}
	for _, evaluation := range group.Evaluations {
		switch {
		case evaluation.Reason != "":
			block(evaluation.Path, evaluation.Reason, true)
		case evaluation.StartedAt.IsZero():
			block(evaluation.Path, "evaluation has no collection-start timestamp", true)
		case !age.IsOlder(evaluation.StartedAt):
			block(evaluation.Path, "collection start is not strictly before the cutoff", true)
		}
		if err := check(evaluation.Path, evaluation.Identity); err != nil {
			block(evaluation.Path, err.Error(), false)
		}
	}
	for _, report := range group.Reports {
		if len(report.Evaluations) == 0 {
			block(report.Path, "report has no establishable evaluation age", false)
		}
		for _, reason := range report.Reasons {
			block(report.Path, reason, false)
		}
		if err := check(report.Path, report.Identity); err != nil {
			block(report.Path, err.Error(), false)
		}
	}
}
