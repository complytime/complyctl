// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/complytime/complyctl/internal/workspacecoord"
)

type removalPreparer func(*os.File, string, os.FileInfo) (*removal, error)

// Preview must exercise the same simultaneous descriptor budget and platform
// facilities as execution, then release every handle without unlinking anything.
func preparePreviewGroups(access *workspacecoord.Access, groups *GroupResult) (err error) {
	workspace, validate, err := openDiscoveryWorkspace(access.Path())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, workspace.Close()) }()
	for i := range groups.Groups {
		group := &groups.Groups[i]
		if !group.Eligible() {
			continue
		}
		if err := errors.Join(access.Validate(), validate()); err != nil {
			return err
		}
		if err := preparePreviewGroup(workspace, group); err != nil {
			return err
		}
	}
	return nil
}

func preparePreviewGroup(workspace *os.File, group *Group) error {
	outcome := &GroupExecution{}
	prepared, prepareErr := prepareGroup(workspace, *group, outcome, prepareRemoval)
	var closeErr error
	for _, member := range prepared {
		closeErr = errors.Join(closeErr, member.close())
	}
	if prepareErr != nil {
		group.Blockers = append(group.Blockers, outcome.Blockers...)
	}
	return closeErr
}

func executeGroups(access *workspacecoord.Access, groups *GroupResult, result *Result) error {
	return executeGroupsWith(access, groups, result, prepareRemoval)
}

func executeGroupsWith(access *workspacecoord.Access, groups *GroupResult, result *Result,
	prepare removalPreparer,
) (err error) {
	workspace, validate, err := openDiscoveryWorkspace(access.Path())
	if err != nil {
		return fmt.Errorf("open cleanup workspace: %w", err)
	}
	defer func() { err = errors.Join(err, workspace.Close()) }()
	byMember := indexGroupsByMember(groups.Groups)
	for i := range result.Groups {
		outcome := &result.Groups[i]
		if len(outcome.Blockers) != 0 {
			continue
		}
		// Match by identity path rather than depending on a separate sort's indices.
		group := byMember[outcome.Members[0].Path]
		if checkErr := errors.Join(access.Validate(), validate()); checkErr != nil {
			return errors.Join(err, fmt.Errorf("revalidate cleanup workspace: %w", checkErr))
		}
		groupErr := executeGroup(workspace, *group, outcome, prepare, access.Validate)
		err = errors.Join(err, groupErr)
	}
	return err
}

func indexGroupsByMember(groups []Group) map[string]*Group {
	byMember := make(map[string]*Group)
	for i := range groups {
		group := &groups[i]
		for _, member := range group.Evaluations {
			byMember[member.Path] = group
		}
		for _, member := range group.Reports {
			byMember[member.Path] = group
		}
	}
	return byMember
}

func executeGroup(workspace *os.File, group Group, outcome *GroupExecution,
	prepare removalPreparer, validate func() error,
) (err error) {
	// Pin EVERY member before the first unlink. Preparing lazily would allow a
	// detectable report/descriptor failure to destroy evidence (FR-016, SRP).
	prepared, err := prepareGroup(workspace, group, outcome, prepare)
	defer func() {
		for _, member := range prepared {
			err = errors.Join(err, member.close())
		}
		if err != nil {
			outcome.Deleted = false
			outcome.Failure = err.Error()
			for _, member := range outcome.Members {
				outcome.PartialLoss = outcome.PartialLoss || len(member.RemovedPaths) > 0
			}
		}
	}()
	if err != nil {
		return err
	}
	for _, member := range prepared {
		if err := removeGroupMember(member, outcome, validate); err != nil {
			return fmt.Errorf("cleanup group stopped; partial deletion possible; no rollback: %w", err)
		}
	}
	outcome.Deleted = true
	return nil
}

func prepareGroup(workspace *os.File, group Group, outcome *GroupExecution,
	prepare removalPreparer,
) ([]*removal, error) {
	var prepared []*removal
	prepareMember := func(path string, identity os.FileInfo) error {
		member, err := prepare(workspace, path, identity)
		if err != nil {
			reason := fmt.Sprintf("prepare removal: %v", err)
			outcome.Blockers = append(outcome.Blockers, GroupBlocker{Path: path, Reason: reason})
			for i := range outcome.Members {
				outcome.Members[i].Reasons = []string{"entire group retained; see blocking members"}
				if outcome.Members[i].Path == path {
					outcome.Members[i].Failure = reason
					outcome.Members[i].Reasons = []string{reason}
				}
			}
			return fmt.Errorf("%w: member %q: %w; entire group retained", ErrPreflightBlocked, path, err)
		}
		prepared = append(prepared, member)
		return nil
	}
	// Evidence-first order is independent of the presentation's path order.
	for _, member := range group.Evaluations {
		if err := prepareMember(member.Path, member.Identity); err != nil {
			return prepared, err
		}
	}
	for _, member := range group.Reports {
		if err := prepareMember(member.Path, member.Identity); err != nil {
			return prepared, err
		}
	}
	return prepared, nil
}

func removeGroupMember(member *removal, group *GroupExecution, validate func() error) error {
	index := slices.IndexFunc(group.Members, func(result MemberResult) bool {
		return result.Path == member.root.path
	})
	outcome := &group.Members[index]
	err := validate()
	if err == nil {
		err = member.remove()
	}
	outcome.RemovedPaths = slices.Clone(member.removed)
	if err == nil {
		outcome.Status = ExecutionDeleted
		outcome.Reasons = nil
		return nil
	}
	outcome.Failure = err.Error()
	outcome.Reasons = []string{"group stopped after removal failure; remaining entries retained; no rollback"}
	if len(member.removed) > 0 {
		outcome.Status = ExecutionPartial
	}
	for _, result := range group.Members {
		group.PartialLoss = group.PartialLoss || len(result.RemovedPaths) > 0
	}
	return fmt.Errorf("remove member %q: %w", outcome.Path, err)
}
