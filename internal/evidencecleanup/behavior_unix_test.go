// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Use distinct binary sentinels so empty, truncated, or rewritten retained files
// cannot satisfy a mere existence check. No configuration needs to be valid.
func addBehavioralExclusions(t *testing.T, fixture reportFixture) {
	t.Helper()
	for _, path := range []string{
		".complytime/cli.log", ".complytime/logs/scan.log",
		".complytime/complytime.yaml", ".complytime/generation/state.json",
		".complytime/opa/working/clone/.git/config", ".complytime/opa/input.json",
		".complytime/cache/policy.json", ".complytime/scan/notes.txt", "unrelated.bin",
		".complytime/ampel/results/legacy-flat.intoto.json",
		".complytime/opa/results/label/evaluation-unrecognized/evidence.json",
		".complytime/openscap/results/invalid.label/" + fixtureUnit + "/results.xml",
	} {
		absolute := filepath.Join(fixture.workspace, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(absolute), 0o700))
		require.NoError(t, os.WriteFile(absolute, []byte("\x00retain\xff:"+path+"\n"), 0o600))
	}
	// External and renamed report copies do not belong to the managed inventory,
	// even though their bytes contain references to evidence that will expire.
	log, err := fixture.evaluator.Write(t.TempDir(), "json")
	require.NoError(t, err)
	data, err := os.ReadFile(log)
	require.NoError(t, err)
	// Keep copied report bytes confined to the test workspace (Simplicity & Isolation).
	root, err := os.OpenRoot(fixture.workspace)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	for _, path := range []string{"archived-report.json", ".complytime/scan/renamed-copy.txt"} {
		require.NoError(t, root.WriteFile(path, data, 0o600))
	}
}

func assertBehavioralRemoval(t *testing.T, workspace string,
	before map[string]retainedEntry, removed []string,
) {
	t.Helper()
	for _, member := range removed {
		require.Contains(t, before, member, "deletion assertion must start with an existing member")
		assert.NoFileExists(t, filepath.Join(workspace, member))
		assert.NoDirExists(t, filepath.Join(workspace, member))
	}
	expected := make(map[string]retainedEntry)
	for path, entry := range before {
		deleted := false
		for _, member := range removed {
			if path == member || strings.HasPrefix(path, member+string(filepath.Separator)) {
				deleted = true
				break
			}
		}
		if !deleted {
			expected[path] = entry
		}
	}
	// Equality also rejects unexpected new logs, state files, and pruned labels.
	assert.Equal(t, expected, groupSnapshot(t, workspace))
}

func TestRun_BehavioralProviderLayoutsExpireRealVariantsOnly(t *testing.T) {
	for _, providerName := range []string{"ampel", "opa", "openscap"} {
		t.Run(providerName, func(t *testing.T) {
			fixture := newReportFixture(t, providerName)
			requirePreviewSupport(t, fixture.workspace)
			variants := writeFixtureVariants(t, fixture)
			addBehavioralExclusions(t, fixture)
			orphan := addGroupEvaluation(t, fixture.workspace, providerName,
				"unreferenced/incomplete", groupOldStart)
			require.NoError(t, os.Remove(filepath.Join(fixture.workspace, orphan, "first.json")))
			empty := filepath.Join(managedDirectory, providerName, resultsDirectory,
				"unreferenced/empty", fixtureUnit)
			require.NoError(t, os.MkdirAll(filepath.Join(fixture.workspace, empty), 0o700))
			// These legacy units have no metadata, completion, or cleanup markers.
			removed := []string{fixture.unit, orphan, empty}
			for _, path := range variants {
				relative, err := filepath.Rel(fixture.workspace, path)
				require.NoError(t, err)
				removed = append(removed, relative)
				// Reports and evidence may be touched after collection. Their mtimes
				// must not turn an old collection into a retained group.
				require.NoError(t, os.Chtimes(path,
					groupAge(t).CapturedAt(), groupAge(t).CapturedAt()))
			}
			require.NoError(t, os.Chtimes(filepath.Join(fixture.workspace, fixture.coordinate),
				groupAge(t).CapturedAt(), groupAge(t).CapturedAt()))
			other := newReportFixture(t, providerName)
			writeFixtureVariants(t, other)
			otherBefore := previewSnapshot(t, other.workspace)
			before := groupSnapshot(t, fixture.workspace)

			opts := executionOptions(t, fixture.workspace)
			result, err := Run(opts)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, groupAge(t), result.Age)
			assert.Equal(t, 1, opts.Clock.(*previewClock).calls)
			assert.Len(t, result.Groups, 3)
			assert.Equal(t, 3, result.Totals.DeletedGroups)
			assert.Equal(t, len(removed), result.Totals.DeletedMembers)
			assert.Zero(t, result.Totals.RetainedMembers)
			assert.Zero(t, result.Totals.FailedGroups)
			for _, path := range removed {
				assert.Equal(t, ExecutionDeleted, findExecutionMember(t, result, path).Status)
			}
			assertBehavioralRemoval(t, fixture.workspace, before, removed)
			assert.Equal(t, otherBefore, previewSnapshot(t, other.workspace))
		})
	}
}

func TestRun_BehavioralTransitiveRealVariantsRetainWholeGroup(t *testing.T) {
	for _, condition := range []string{"newer", "unsafe"} {
		t.Run(condition, func(t *testing.T) {
			fixture := newReportFixture(t, "ampel")
			requirePreviewSupport(t, fixture.workspace)
			variants := writeFixtureVariants(t, fixture)
			addBehavioralExclusions(t, fixture)
			secondFile := filepath.Join(fixture.unit, "other.json")
			require.NoError(t, os.WriteFile(filepath.Join(fixture.workspace, secondFile),
				[]byte("\x00different contained file\xff"), 0o600))
			shared := addGroupEvaluation(t, fixture.workspace, "opa", "shared", groupOldStart)
			start := groupOldStart
			if condition == "newer" {
				start = "20261118T120000Z"
			}
			last := addGroupEvaluation(t, fixture.workspace, "openscap", "last", start)
			bridge := addGroupReport(t, fixture.workspace, "bridge", secondFile,
				filepath.Join(shared, "first.json"))
			tail := addGroupReport(t, fixture.workspace, "tail",
				filepath.Join(shared, "nested", "second.json"), filepath.Join(last, "first.json"))
			if condition == "unsafe" {
				require.NoError(t, os.Symlink("first.json",
					filepath.Join(fixture.workspace, last, "unsafe-link")))
			}
			members := []string{fixture.unit, shared, last, bridge, tail}
			for _, path := range variants {
				relative, err := filepath.Rel(fixture.workspace, path)
				require.NoError(t, err)
				members = append(members, relative)
			}
			// Even backdated reports cannot authorize removal of newer evidence.
			for _, path := range members {
				require.NoError(t, os.Chtimes(filepath.Join(fixture.workspace, path),
					groupAge(t).Cutoff().Add(-dayDuration), groupAge(t).Cutoff().Add(-dayDuration)))
			}
			before := previewSnapshot(t, fixture.workspace)

			result, err := Run(executionOptions(t, fixture.workspace))
			if condition == "newer" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrPreflightBlocked)
			}
			require.NotNil(t, result)
			require.Len(t, result.Groups, 1, "different contained files still share evaluation units")
			group := result.Groups[0]
			require.NotEmpty(t, group.Blockers)
			assert.Equal(t, last, group.Blockers[0].Path)
			assert.Equal(t, condition == "newer", group.Blockers[0].AgeExclusion)
			assert.False(t, group.Deleted)
			assert.False(t, group.PartialLoss)
			assert.Len(t, group.Members, len(members))
			assert.Equal(t, len(members), result.Totals.RetainedMembers)
			assert.Zero(t, result.Totals.DeletedMembers)
			for _, path := range members {
				member := findExecutionMember(t, result, path)
				assert.Equal(t, ExecutionRetained, member.Status)
				assert.Empty(t, member.RemovedPaths)
			}
			assert.Equal(t, before, previewSnapshot(t, fixture.workspace))
		})
	}
}

func TestRun_BehavioralSARIFRefusesBeforeAnyGroupMutation(t *testing.T) {
	for _, variant := range []string{"sarif", "empty-sarif"} {
		t.Run(variant, func(t *testing.T) {
			fixture := newReportFixture(t, "ampel")
			requirePreviewSupport(t, fixture.workspace)
			writeFixtureVariants(t, fixture)
			addBehavioralExclusions(t, fixture)
			addGroupEvaluation(t, fixture.workspace, "opa", "unreferenced", groupOldStart)
			newer := addGroupEvaluation(t, fixture.workspace, "openscap", "newer", "20261118T120000Z")
			addGroupReport(t, fixture.workspace, "newer", filepath.Join(newer, "first.json"))
			// Both alert-bearing and all-passing real SARIF omit provenance. A
			// matching policy/target filename is not permission to guess an edge.
			addUnresolvedFixture(t, fixture, variant)
			before := previewSnapshot(t, fixture.workspace)

			result, err := Run(executionOptions(t, fixture.workspace))
			require.ErrorContains(t, err, "SARIF has no originating EvaluationLog/evidence provenance")
			assert.ErrorContains(t, err, "possible relationships are unbounded")
			assert.Nil(t, result, "unknown inventory cannot authorize even standalone old evidence")
			assert.Equal(t, before, previewSnapshot(t, fixture.workspace))
		})
	}
}
