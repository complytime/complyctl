// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/evidencecleanup"
)

const discoveryToken = "0123456789abcdef0123456789abcdef"

type discoveryClock time.Time

func (c discoveryClock) Now() time.Time { return time.Time(c) }

func evaluationName(timestamp string) string {
	return "evaluation-" + timestamp + "-" + discoveryToken
}

func resultsPath(provider string, parts ...string) string {
	return filepath.Join(append([]string{".complytime", provider, "results"}, parts...)...)
}

func fixtureDirectory(t *testing.T, workspace, relative string) string {
	t.Helper()
	path := filepath.Join(workspace, relative)
	require.NoError(t, os.MkdirAll(path, 0o755))
	return path
}

func fixtureFile(t *testing.T, workspace, relative, content string) {
	t.Helper()
	fixtureDirectory(t, workspace, filepath.Dir(relative))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, relative), []byte(content), 0o600))
}

func workspaceSnapshot(t *testing.T, workspace string) map[string]string {
	t.Helper()
	root, err := os.OpenRoot(workspace)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	snapshot := map[string]string{}
	err = filepath.WalkDir(workspace, func(path string, entry fs.DirEntry, err error) error {
		require.NoError(t, err)
		relative, err := filepath.Rel(workspace, path)
		require.NoError(t, err)
		switch {
		case entry.IsDir():
			snapshot[relative] = "directory"
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			require.NoError(t, err)
			snapshot[relative] = "symlink: " + target
		default:
			content, err := root.ReadFile(relative)
			require.NoError(t, err)
			snapshot[relative] = string(content)
		}
		return nil
	})
	require.NoError(t, err)
	return snapshot
}

func TestDiscover_ApprovedRootsAndIndivisibleOldUnits(t *testing.T) {
	workspace := t.TempDir()
	oldName := evaluationName("20200102T030405Z")
	var expected []string
	for _, provider := range []string{"ampel", "opa", "openscap"} {
		path := resultsPath(provider, "Policy_1", "target-2", oldName)
		fixtureDirectory(t, workspace, path)
		expected = append(expected, path)
		// Empty, partial, and metadata-bearing units all use the same naming
		// contract; no marker or complete artifact set is needed to inventory.
		if provider == "opa" {
			fixtureFile(t, workspace, filepath.Join(path, "partial.tmp"), "partial output")
		}
		if provider == "openscap" {
			fixtureFile(t, workspace, filepath.Join(path, "metadata.json"), "not trusted JSON")
			fixtureFile(t, workspace, filepath.Join(path, "result.xml"), "old retained output")
			fixtureDirectory(t, workspace, filepath.Join(path, oldName))
		}
	}
	before := workspaceSnapshot(t, workspace)
	result, err := evidencecleanup.Discover(evidencecleanup.DiscoveryOptions{Workspace: workspace})
	require.NoError(t, err)
	require.Len(t, result.Evaluations, 3)
	for index, evaluation := range result.Evaluations {
		assert.Equal(t, expected[index], evaluation.Path)
		assert.Equal(t, []string{"ampel", "opa", "openscap"}[index], evaluation.Provider)
		assert.Equal(t, time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC), evaluation.StartedAt)
		assert.Empty(t, evaluation.Reason)
		info, err := os.Stat(filepath.Join(workspace, evaluation.Path))
		require.NoError(t, err)
		assert.True(t, os.SameFile(info, evaluation.Identity))
	}
	assert.Empty(t, result.Exclusions)
	assert.Equal(t, before, workspaceSnapshot(t, workspace))
}

func TestDiscover_TimestampsUseExistingAgeRulesNotMetadataOrMtime(t *testing.T) {
	workspace := t.TempDir()
	window, err := evidencecleanup.NewAgeWindow(evidencecleanup.AgeOptions{
		OlderThan: "30d", Clock: discoveryClock(time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)),
	})
	require.NoError(t, err)
	tests := []struct {
		timestamp string
		older     bool
		malformed bool
	}{
		{"20261019T115959Z", true, false},
		{"20261019T120000Z", false, false},
		{"20261019T120001Z", false, false},
		{"20271118T120000Z", false, false},
		{"20260230T120000Z", false, true},
	}
	for _, test := range tests {
		relative := resultsPath("ampel", evaluationName(test.timestamp))
		path := fixtureDirectory(t, workspace, relative)
		fixtureFile(t, workspace, filepath.Join(relative, "metadata.json"), `{"complete":true}`)
		mtime := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(path, mtime, mtime))
	}
	result, err := evidencecleanup.Discover(evidencecleanup.DiscoveryOptions{Workspace: workspace})
	require.NoError(t, err)
	require.Len(t, result.Evaluations, len(tests))
	byPath := make(map[string]evidencecleanup.Evaluation)
	for _, evaluation := range result.Evaluations {
		byPath[evaluation.Path] = evaluation
	}
	for _, test := range tests {
		evaluation, found := byPath[resultsPath("ampel", evaluationName(test.timestamp))]
		require.True(t, found, test.timestamp)
		if test.malformed {
			assert.Zero(t, evaluation.StartedAt)
			assert.Contains(t, evaluation.Reason, "collection-start time")
		} else {
			assert.Empty(t, evaluation.Reason)
			assert.Equal(t, test.older, window.IsOlder(evaluation.StartedAt), test.timestamp)
		}
	}
}

func TestDiscover_ExcludedLayoutsAndWorkspaceContent(t *testing.T) {
	workspace := t.TempDir()
	name := evaluationName("20200102T030405Z")
	valid := resultsPath("opa", strings.Repeat("a", 80), name)
	fixtureDirectory(t, workspace, valid)
	excludedDirectories := []string{
		resultsPath("opa", strings.Repeat("b", 81)),
		resultsPath("opa", "label.with.dot"),
		resultsPath("opa", "label with space"),
		resultsPath("opa", "é"),
		resultsPath("opa", "evaluation-unknown-format"),
		resultsPath("opa", strings.ReplaceAll(name, "abcdef", "ABCDEF")),
	}
	var expectedExclusions []evidencecleanup.DiscoveryExclusion
	for _, relative := range excludedDirectories {
		fixtureDirectory(t, workspace, filepath.Join(relative, name))
		expectedExclusions = append(expectedExclusions,
			evidencecleanup.DiscoveryExclusion{
				Path: relative, Reason: "unrecognized provider results layout",
			})
	}
	flatFile := resultsPath("opa", "old-result.json")
	fixtureFile(t, workspace, flatFile, "legacy flat output")
	expectedExclusions = append(expectedExclusions, evidencecleanup.DiscoveryExclusion{
		Path: flatFile, Reason: "unrecognized provider results layout",
	})
	for _, path := range []string{
		".complytime/scan/report.yaml", ".complytime/logs/cli.log", ".complytime/complytime.yaml",
		".complytime/opa/clones/input", ".complytime/generate/state.json", "input.json",
		filepath.Join(".complytime", "other", "results", name, "unmanaged"),
	} {
		fixtureFile(t, workspace, path, "must remain unchanged")
	}
	before := workspaceSnapshot(t, workspace)
	result, err := evidencecleanup.Discover(evidencecleanup.DiscoveryOptions{Workspace: workspace})
	require.NoError(t, err)
	require.Len(t, result.Evaluations, 1)
	assert.Equal(t, valid, result.Evaluations[0].Path)
	assert.ElementsMatch(t, expectedExclusions, result.Exclusions)
	assert.IsIncreasing(t, exclusionPaths(result.Exclusions))
	assert.Equal(t, before, workspaceSnapshot(t, workspace))
}

func exclusionPaths(exclusions []evidencecleanup.DiscoveryExclusion) []string {
	var paths []string
	for _, exclusion := range exclusions {
		paths = append(paths, exclusion.Path)
	}
	return paths
}

func TestDiscover_SymlinkConfinementRefusesPartialInventory(t *testing.T) {
	for _, component := range []string{
		".complytime", ".complytime/ampel", resultsPath("ampel"),
		resultsPath("ampel", "label"),
		resultsPath("ampel", "label", evaluationName("20200102T030405Z")),
	} {
		t.Run(component, func(t *testing.T) {
			workspace, outside := t.TempDir(), t.TempDir()
			fixtureFile(t, outside, "untouched", "external evidence")
			fixtureDirectory(t, workspace, filepath.Dir(component))
			require.NoError(t, os.Symlink(outside, filepath.Join(workspace, component)))
			before := workspaceSnapshot(t, workspace)
			result, err := evidencecleanup.Discover(evidencecleanup.DiscoveryOptions{Workspace: workspace})
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "provider results")
			assert.Equal(t, before, workspaceSnapshot(t, workspace))
			assert.Equal(t, map[string]string{".": "directory", "untouched": "external evidence"},
				workspaceSnapshot(t, outside))
		})
	}
}

func TestDiscover_MissingRootsInvalidWorkspaceAndRootTypes(t *testing.T) {
	workspace := t.TempDir()
	result, err := evidencecleanup.Discover(evidencecleanup.DiscoveryOptions{Workspace: workspace})
	require.NoError(t, err)
	assert.Equal(t, &evidencecleanup.DiscoveryResult{}, result)
	assert.Equal(t, map[string]string{".": "directory"}, workspaceSnapshot(t, workspace))
	for _, path := range []string{"", filepath.Join(workspace, "missing")} {
		result, err := evidencecleanup.Discover(evidencecleanup.DiscoveryOptions{Workspace: path})
		require.Error(t, err)
		assert.Nil(t, result)
	}
	fixtureFile(t, workspace, resultsPath("opa"), "not a directory")
	result, err = evidencecleanup.Discover(evidencecleanup.DiscoveryOptions{Workspace: workspace})
	require.ErrorContains(t, err, "provider results")
	assert.Nil(t, result)
}

func TestDiscover_WorkspaceAliasAndRepeatedDiscovery(t *testing.T) {
	workspace := t.TempDir()
	fixtureDirectory(t, workspace, resultsPath("openscap", evaluationName("20200102T030405Z")))
	alias := filepath.Join(t.TempDir(), "workspace-alias")
	require.NoError(t, os.Symlink(workspace, alias))
	first, err := evidencecleanup.Discover(evidencecleanup.DiscoveryOptions{Workspace: alias})
	require.NoError(t, err)
	second, err := evidencecleanup.Discover(evidencecleanup.DiscoveryOptions{Workspace: workspace})
	require.NoError(t, err)
	require.Len(t, first.Evaluations, 1)
	require.Len(t, second.Evaluations, 1)
	assert.Equal(t, first.Evaluations[0].Path, second.Evaluations[0].Path)
	assert.True(t, os.SameFile(first.Evaluations[0].Identity, second.Evaluations[0].Identity))
}
