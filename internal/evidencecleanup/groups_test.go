// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const groupOldStart = "20261001T120000Z"

func groupAge(t *testing.T) AgeWindow {
	t.Helper()
	now, err := time.Parse(time.RFC3339, "2026-11-18T12:00:00Z")
	require.NoError(t, err)
	return AgeWindow{capturedAt: now, cutoff: now.Add(-30 * dayDuration)}
}

func addGroupEvaluation(t *testing.T, workspace, providerName, label, start string) string {
	t.Helper()
	unit := filepath.Join(managedDirectory, providerName, resultsDirectory, label,
		evaluationPrefix+start+"-0123456789abcdef0123456789abcdef")
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, unit), 0o700))
	for _, name := range []string{"first.json", "nested/second.json"} {
		path := filepath.Join(workspace, unit, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(label+name), 0o600))
	}
	return unit
}

func addGroupReport(t *testing.T, workspace, name string, coordinates ...string) string {
	t.Helper()
	evaluator := fixtureEvaluator(name, "target", coordinates[0])
	log := evaluator.GemaraLog()
	evidence := log.Evaluations[0].AssessmentLogs[0].Evidence[0]
	for _, coordinate := range coordinates[1:] {
		next := evidence
		next.Id = coordinate
		next.Source.Coordinate = coordinate
		log.Evaluations[0].AssessmentLogs[0].Evidence = append(
			log.Evaluations[0].AssessmentLogs[0].Evidence, next)
	}
	dir := filepath.Join(workspace, reportDirectory)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path, err := evaluator.Write(dir, "json")
	require.NoError(t, err)
	relative, err := filepath.Rel(workspace, path)
	require.NoError(t, err)
	return relative
}

type retainedEntry struct {
	Mode fs.FileMode
	Data string
}

// Snapshot links themselves and special-file modes, never their targets/content.
func groupSnapshot(t *testing.T, workspace string) map[string]retainedEntry {
	t.Helper()
	root, err := os.OpenRoot(workspace)
	require.NoError(t, err)
	defer func() { require.NoError(t, root.Close()) }()
	entries := make(map[string]retainedEntry)
	err = filepath.WalkDir(workspace, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		var data string
		switch {
		case info.Mode().IsRegular():
			bytes, err := root.ReadFile(relative)
			if err != nil {
				return err
			}
			data = string(bytes)
		case info.Mode()&os.ModeSymlink != 0:
			data, err = root.Readlink(relative)
			if err != nil {
				return err
			}
		}
		entries[relative] = retainedEntry{Mode: info.Mode(), Data: data}
		return nil
	})
	require.NoError(t, err)
	return entries
}

func groupMemberPaths(group Group) ([]string, []string) {
	var evaluations, reports []string
	for _, evaluation := range group.Evaluations {
		evaluations = append(evaluations, evaluation.Path)
	}
	for _, report := range group.Reports {
		reports = append(reports, report.Path)
	}
	return evaluations, reports
}

func TestPreflightGroups_TransitiveSharingUsesWholeEvaluations(t *testing.T) {
	workspace := t.TempDir()
	first := addGroupEvaluation(t, workspace, "ampel", "first", groupOldStart)
	shared := addGroupEvaluation(t, workspace, "opa", "shared", groupOldStart)
	last := addGroupEvaluation(t, workspace, "openscap", "last", groupOldStart)
	reports := []string{
		addGroupReport(t, workspace, "a", filepath.Join(first, "first.json"),
			filepath.Join(shared, "first.json")),
		addGroupReport(t, workspace, "b", filepath.Join(shared, "nested/second.json"),
			filepath.Join(last, "first.json")),
		addGroupReport(t, workspace, "c", filepath.Join(last, "nested/second.json")),
	}
	// Publication/mtime is deliberately newer than the cutoff; only collection
	// starts establish age, even with reports pointing at different files.
	for _, report := range reports {
		require.NoError(t, os.Chtimes(filepath.Join(workspace, report),
			groupAge(t).CapturedAt(), groupAge(t).CapturedAt()))
	}
	before := groupSnapshot(t, workspace)
	result, err := PreflightGroups(GroupOptions{Workspace: workspace, Age: groupAge(t)})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	group := result.Groups[0]
	assert.True(t, group.Eligible(), "%+v", group.Blockers)
	assert.Empty(t, group.Blockers)
	units := []string{first, shared, last}
	slices.Sort(units)
	slices.Sort(reports)
	gotUnits, gotReports := groupMemberPaths(group)
	assert.Equal(t, units, gotUnits)
	assert.Equal(t, reports, gotReports)
	assert.Equal(t, before, groupSnapshot(t, workspace))
}

func TestPreflightGroups_EveryConnectedStartControlsAge(t *testing.T) {
	for _, start := range []string{
		groupOldStart, "20261019T115959Z", "20261019T120000Z",
		"20261019T120001Z", "20261119T120000Z", "20260230T120000Z",
	} {
		t.Run(start, func(t *testing.T) {
			workspace := t.TempDir()
			old := addGroupEvaluation(t, workspace, "ampel", "old", groupOldStart)
			other := addGroupEvaluation(t, workspace, "opa", "other", start)
			addGroupReport(t, workspace, "a", filepath.Join(old, "first.json"),
				filepath.Join(other, "first.json"))
			addGroupReport(t, workspace, "b", filepath.Join(old, "nested/second.json"))
			before := groupSnapshot(t, workspace)
			result, err := PreflightGroups(GroupOptions{Workspace: workspace, Age: groupAge(t)})
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			group := result.Groups[0]
			eligible := start == groupOldStart || start == "20261019T115959Z"
			assert.Equal(t, eligible, group.Eligible(), "%+v", group.Blockers)
			if !eligible {
				require.Len(t, group.Blockers, 1)
				assert.Equal(t, other, group.Blockers[0].Path)
				assert.True(t, group.Blockers[0].AgeExclusion)
			}
			assert.Len(t, group.Evaluations, 2)
			assert.Len(t, group.Reports, 2)
			assert.Equal(t, before, groupSnapshot(t, workspace))
		})
	}
}

func TestPreflightGroups_ReportlessRequiresCompleteInventory(t *testing.T) {
	for _, variant := range []string{"complete", "sarif", "malformed", "unknown"} {
		t.Run(variant, func(t *testing.T) {
			fixture := newReportFixture(t, "opa")
			// No metadata or completion marker is needed for a reportless unit.
			if variant != "complete" {
				addUnresolvedFixture(t, fixture, variant)
			}
			before := groupSnapshot(t, fixture.workspace)
			result, err := PreflightGroups(GroupOptions{Workspace: fixture.workspace, Age: groupAge(t)})
			if variant == "complete" {
				require.NoError(t, err)
				require.Len(t, result.Groups, 1)
				assert.True(t, result.Groups[0].Eligible(), "%+v", result.Groups[0].Blockers)
				units, reports := groupMemberPaths(result.Groups[0])
				assert.Equal(t, []string{fixture.unit}, units)
				assert.Empty(t, reports)
			} else {
				require.ErrorContains(t, err, "report")
				assert.Nil(t, result, "incomplete inventory cannot authorize a reportless group")
			}
			assert.Equal(t, before, groupSnapshot(t, fixture.workspace))
		})
	}
}

func TestPreflightGroups_ActualVariantsAndIndependentReportlessUnit(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	paths := writeFixtureVariants(t, fixture)
	orphan := addGroupEvaluation(t, fixture.workspace, "ampel", "orphan", groupOldStart)
	before := groupSnapshot(t, fixture.workspace)
	result, err := PreflightGroups(GroupOptions{Workspace: fixture.workspace, Age: groupAge(t)})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)
	assert.True(t, result.Groups[0].Eligible(), "%+v", result.Groups[0].Blockers)
	units, reports := groupMemberPaths(result.Groups[0])
	assert.Equal(t, []string{orphan}, units)
	assert.Empty(t, reports)
	units, reports = groupMemberPaths(result.Groups[1])
	assert.Equal(t, []string{fixture.unit}, units)
	var expected []string
	for _, path := range paths {
		relative, err := filepath.Rel(fixture.workspace, path)
		require.NoError(t, err)
		expected = append(expected, relative)
	}
	slices.Sort(expected)
	assert.Equal(t, expected, reports)
	assert.True(t, result.Groups[1].Eligible(), "%+v", result.Groups[1].Blockers)
	assert.Equal(t, before, groupSnapshot(t, fixture.workspace))
}

func TestPreflightGroups_ReportWithoutEvaluationAgeIsRetained(t *testing.T) {
	fixture := newReportFixture(t, "ampel")
	fixture.evaluator.GemaraLog().Evaluations[0].AssessmentLogs[0].Evidence = []gemara.Evidence{}
	_, err := fixture.evaluator.Write(fixture.dir, "json")
	require.NoError(t, err)
	result, err := PreflightGroups(GroupOptions{Workspace: fixture.workspace, Age: groupAge(t)})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)
	assert.True(t, result.Groups[0].Eligible())
	assert.False(t, result.Groups[1].Eligible())
	assert.Empty(t, result.Groups[1].Evaluations)
	require.NotEmpty(t, result.Groups[1].Blockers)
	assert.Contains(t, result.Groups[1].Blockers[0].Reason, "no establishable evaluation age")
}
