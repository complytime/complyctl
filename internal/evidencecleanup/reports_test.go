// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemaraproj/go-gemara"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/output"
	"github.com/complytime/complyctl/pkg/provider"
)

const fixtureUnit = "evaluation-20261001T120000Z-0123456789abcdef0123456789abcdef"

type reportFixture struct {
	workspace  string
	dir        string
	unit       string
	coordinate string
	evaluator  *output.Evaluator
}

func newReportFixture(t *testing.T, providerName string) reportFixture {
	t.Helper()
	workspace := t.TempDir()
	unit := filepath.Join(managedDirectory, providerName, resultsDirectory, "policy", "target", fixtureUnit)
	coordinate := filepath.Join(unit, "evidence.json")
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, unit), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, coordinate), []byte("evidence"), 0o600))
	dir := filepath.Join(workspace, filepath.FromSlash(reportDirectory))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	return reportFixture{workspace: workspace, dir: dir, unit: unit, coordinate: coordinate,
		evaluator: fixtureEvaluator("policy", "target", coordinate)}
}

func fixtureEvaluator(policyID, targetID, coordinate string) *output.Evaluator {
	evaluator := output.NewEvaluator(policyID, targetID,
		map[string]string{"requirement": "control"}, nil, nil,
		[]gemara.MappingReference{{Id: "retained", Title: "Retained evidence", Version: "1",
			Url: "https://example.invalid/never-fetch"}})
	evaluator.AddTarget([]provider.AssessmentLog{{
		RequirementID: "requirement", Message: "assessment failed", Confidence: provider.ConfidenceLevelHigh,
		Steps: []provider.Step{{Name: "inspect", Result: provider.ResultFailed, Message: "assessment failed"}},
		Evidence: []provider.Evidence{{ID: "proof", Type: "command-output", Description: "retained proof",
			CollectedAt: "2026-10-01T12:00:00Z", Payload: []byte("inline is not a path"),
			Source: &provider.EvidenceSource{ReferenceID: "retained", Coordinate: coordinate}}},
	}})
	return evaluator
}

func writeFixtureVariants(t *testing.T, fixture reportFixture) []string {
	t.Helper()
	yamlPath, err := fixture.evaluator.Write(fixture.dir, "yaml")
	require.NoError(t, err)
	jsonPath, err := fixture.evaluator.Write(fixture.dir, "json")
	require.NoError(t, err)
	markdown := output.NewMarkdown("policy", fixture.evaluator.GemaraLog())
	markdown.SetEmbedEvaluationLog(jsonPath) // Formatter labels even JSON as yaml.
	markdownPath, err := markdown.Write(fixture.dir)
	require.NoError(t, err)
	oscalPath, err := output.ToOSCAL(fixture.evaluator.GemaraLog(), fixture.dir)
	require.NoError(t, err)
	return []string{yamlPath, jsonPath, markdownPath, oscalPath}
}

func TestInventoryReports_RealVariantsUseContentAcrossSeconds(t *testing.T) {
	for _, providerName := range []string{"ampel", "opa", "openscap"} {
		t.Run(providerName, func(t *testing.T) {
			fixture := newReportFixture(t, providerName)
			paths := writeFixtureVariants(t, fixture)
			// Different seconds and deliberately misleading name identities cannot
			// change the exact content-derived edges.
			prefixes := []string{"evaluation-log-unrelated-20000101-000000.yaml",
				"evaluation-log-collision-20000101-000001.json",
				"report-other-20000101-000002.md", "assessment-results-other-20000101-000003.json"}
			for i, path := range paths {
				require.NoError(t, os.Rename(path, filepath.Join(fixture.dir, prefixes[i])))
			}
			result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
			require.NoError(t, err)
			require.Len(t, result.Reports, 4)
			for _, report := range result.Reports {
				assert.Equal(t, "policy", report.PolicyID)
				assert.Equal(t, "target", report.TargetID)
				assert.Equal(t, []string{fixture.unit}, report.Evaluations)
				assert.Empty(t, report.Reasons)
				assert.True(t, report.Identity.Mode().IsRegular())
			}
			assert.Len(t, result.Evaluations, 1)
		})
	}
}

func TestInventoryReports_EmbeddedYAMLWithoutStandaloneLog(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	path, err := fixture.evaluator.Write(t.TempDir(), "yaml")
	require.NoError(t, err)
	markdown := output.NewMarkdown("policy", fixture.evaluator.GemaraLog())
	markdown.SetEmbedEvaluationLog(path)
	_, err = markdown.Write(fixture.dir)
	require.NoError(t, err)
	result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	assert.Equal(t, formatMarkdown, result.Reports[0].Format)
	assert.Equal(t, []string{fixture.unit}, result.Reports[0].Evaluations)
}

func TestInventoryReports_MissingEvidenceProtectsAllReferringVariants(t *testing.T) {
	fixture := newReportFixture(t, "ampel")
	paths := writeFixtureVariants(t, fixture)
	before := make(map[string][]byte)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		before[path] = data
	}
	require.NoError(t, os.Remove(filepath.Join(fixture.workspace, fixture.coordinate)))
	result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	require.NoError(t, err)
	for _, report := range result.Reports {
		assert.Equal(t, []string{fixture.unit}, report.Evaluations)
		require.Len(t, report.Reasons, 1)
		assert.Contains(t, report.Reasons[0], "open contained evidence")
	}
	for path, expected := range before {
		actual, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
}

func TestInventoryReports_AbsoluteAndFileCoordinates(t *testing.T) {
	for _, fileURI := range []bool{false, true} {
		t.Run(map[bool]string{false: "absolute", true: "file-uri"}[fileURI], func(t *testing.T) {
			fixture := newReportFixture(t, "openscap")
			coordinate := filepath.Join(fixture.workspace, fixture.coordinate)
			if fileURI {
				coordinate = "file://" + coordinate
			}
			fixture.evaluator = fixtureEvaluator("policy", "target", coordinate)
			writeFixtureVariants(t, fixture)
			result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
			require.NoError(t, err)
			for _, report := range result.Reports {
				assert.Equal(t, []string{fixture.unit}, report.Evaluations)
				assert.Empty(t, report.Reasons)
			}
		})
	}
}

func TestInventoryReports_WorkspaceAliasCoordinates(t *testing.T) {
	for _, selectedAlias := range []bool{false, true} {
		for _, coordinateAlias := range []bool{false, true} {
			for _, fileURI := range []bool{false, true} {
				name := fmt.Sprintf("workspace-alias=%t/coordinate-alias=%t/file-uri=%t",
					selectedAlias, coordinateAlias, fileURI)
				t.Run(name, func(t *testing.T) {
					fixture := newReportFixture(t, "openscap")
					canonical, err := filepath.EvalSymlinks(fixture.workspace)
					require.NoError(t, err)
					// Model /var -> /private/var without depending on the host's TMPDIR.
					alias := filepath.Join(t.TempDir(), "alias")
					require.NoError(t, os.Symlink(filepath.Dir(canonical), alias))
					aliased := filepath.Join(alias, filepath.Base(canonical))
					workspace := canonical
					if selectedAlias {
						workspace = aliased
					}
					coordinate := filepath.Join(canonical, fixture.coordinate)
					if coordinateAlias {
						coordinate = filepath.Join(aliased, fixture.coordinate)
					}
					if fileURI {
						coordinate = "file://" + coordinate
					}
					fixture.evaluator = fixtureEvaluator("policy", "target", coordinate)
					writeFixtureVariants(t, fixture)
					before := groupSnapshot(t, canonical)
					result, err := InventoryReports(DiscoveryOptions{Workspace: workspace})
					require.NoError(t, err)
					require.Len(t, result.Reports, 4)
					for _, report := range result.Reports {
						assert.Equal(t, []string{fixture.unit}, report.Evaluations)
						assert.Empty(t, report.Reasons)
					}
					assert.Equal(t, before, groupSnapshot(t, canonical))
				})
			}
		}
	}
}

func TestInventoryReports_WorkspaceAliasDoesNotResolveManagedSymlinks(t *testing.T) {
	for _, kind := range []string{"missing", "file-link", "directory-link", "outside-unit-alias"} {
		t.Run(kind, func(t *testing.T) {
			fixture := newReportFixture(t, "openscap")
			outside := removalOutsideSentinel(t)
			alias := filepath.Join(t.TempDir(), "workspace")
			require.NoError(t, os.Symlink(fixture.workspace, alias))
			coordinate := filepath.Join(alias, fixture.coordinate)
			switch kind {
			case "missing":
				require.NoError(t, os.Remove(filepath.Join(fixture.workspace, fixture.coordinate)))
			case "file-link":
				require.NoError(t, os.Remove(filepath.Join(fixture.workspace, fixture.coordinate)))
				require.NoError(t, os.Symlink(filepath.Join(outside, "sentinel"),
					filepath.Join(fixture.workspace, fixture.coordinate)))
			case "directory-link":
				require.NoError(t, os.Symlink(outside, filepath.Join(fixture.workspace, fixture.unit, "link")))
				coordinate = filepath.Join(alias, fixture.unit, "link", "sentinel")
			case "outside-unit-alias":
				unitAlias := filepath.Join(outside, "unit")
				require.NoError(t, os.Symlink(filepath.Join(fixture.workspace, fixture.unit), unitAlias))
				coordinate = filepath.Join(unitAlias, "evidence.json")
			}
			fixture.evaluator = fixtureEvaluator("policy", "target", "file://"+coordinate)
			writeFixtureVariants(t, fixture)
			before := groupSnapshot(t, fixture.workspace)
			result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
			if kind != "missing" {
				require.ErrorContains(t, err, "unbounded")
				require.Nil(t, result)
				assert.Equal(t, before, groupSnapshot(t, fixture.workspace))
				return
			}
			require.NoError(t, err)
			require.Len(t, result.Reports, 4)
			for _, report := range result.Reports {
				assert.Equal(t, []string{fixture.unit}, report.Evaluations)
				require.Len(t, report.Reasons, 1)
				assert.Contains(t, report.Reasons[0], "open contained evidence")
			}
			assert.Equal(t, before, groupSnapshot(t, fixture.workspace))
		})
	}
}

func TestIndexMappingReferences_ValidatesIDsAndPreservesMembership(t *testing.T) {
	for _, ids := range [][]string{nil, {"first", "last"}, {""}, {"duplicate", "duplicate"}} {
		t.Run(fmt.Sprint(ids), func(t *testing.T) {
			references := make([]gemara.MappingReference, len(ids))
			for index, id := range ids {
				references[index].Id = id
			}
			indexed, err := indexMappingReferences(references)
			if len(ids) > 0 && (ids[0] == "" || ids[0] == "duplicate") {
				require.ErrorContains(t, err, "missing/duplicate mapping-reference ID")
				assert.Nil(t, indexed)
				return
			}
			require.NoError(t, err)
			assert.Len(t, indexed, len(ids))
			for _, id := range ids {
				assert.True(t, indexed[id])
			}
			assert.False(t, indexed["absent"])
		})
	}
}

func TestInventoryReports_NameCollisionDoesNotMergeDifferentEvidence(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	second := filepath.Join(managedDirectory, "ampel", resultsDirectory, fixtureUnit)
	require.NoError(t, os.MkdirAll(filepath.Join(fixture.workspace, second), 0o700))
	coordinate := filepath.Join(second, "other.json")
	require.NoError(t, os.WriteFile(filepath.Join(fixture.workspace, coordinate), []byte("other"), 0o600))
	first := fixtureEvaluator("a/b", "same", fixture.coordinate)
	other := fixtureEvaluator("a-b", "same", coordinate)
	// Serialize separately so fixture membership never depends on the writers'
	// wall-clock timestamps. FilenameSafe gives these distinct policies the same
	// basename; only their content establishes the two evidence relationships.
	firstPath, err := first.Write(t.TempDir(), "json")
	require.NoError(t, err)
	otherPath, err := other.Write(t.TempDir(), "yaml")
	require.NoError(t, err)
	root, err := os.OpenRoot(fixture.dir)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, root.Close()) })
	for _, variant := range []struct{ source, extension string }{
		{firstPath, ".json"}, {otherPath, ".yaml"},
	} {
		data, err := os.ReadFile(variant.source)
		require.NoError(t, err)
		name := "evaluation-log-a-b-same-20261001-120000" + variant.extension
		require.NoError(t, root.WriteFile(name, data, 0o600))
	}
	result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	require.NoError(t, err)
	require.Len(t, result.Reports, 2)
	byPolicy := make(map[string][]string)
	for _, report := range result.Reports {
		byPolicy[report.PolicyID] = report.Evaluations
	}
	assert.Equal(t, []string{fixture.unit}, byPolicy["a/b"])
	assert.Equal(t, []string{second}, byPolicy["a-b"])
}

func TestInventoryReports_UnboundedUncertaintyReturnsNoPartialInventory(t *testing.T) {
	for _, variant := range []string{"sarif", "empty-sarif", "markdown-no-log", "malformed", "unknown", "future-field"} {
		t.Run(variant, func(t *testing.T) {
			fixture := newReportFixture(t, "opa")
			good, err := fixture.evaluator.Write(fixture.dir, "json")
			require.NoError(t, err)
			addUnresolvedFixture(t, fixture, variant)
			before, err := os.ReadFile(good)
			require.NoError(t, err)
			result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "report")
			assert.Nil(t, result)
			after, err := os.ReadFile(good)
			require.NoError(t, err)
			assert.Equal(t, before, after)
			assert.DirExists(t, filepath.Join(fixture.workspace, fixture.unit))
		})
	}
}

func addUnresolvedFixture(t *testing.T, fixture reportFixture, variant string) {
	t.Helper()
	switch variant {
	case "sarif", "empty-sarif":
		log := fixture.evaluator.GemaraLog()
		if variant == "empty-sarif" {
			log.Evaluations[0].AssessmentLogs[0].Result = gemara.Passed
		}
		_, err := output.ToSARIF(log, "file:///scan", fixture.dir)
		require.NoError(t, err)
	case "markdown-no-log":
		_, err := output.NewMarkdown("policy", fixture.evaluator.GemaraLog()).Write(fixture.dir)
		require.NoError(t, err)
	case "malformed":
		require.NoError(t, os.WriteFile(filepath.Join(fixture.dir, "evaluation-log-bad.yaml"), []byte("[invalid"), 0o600))
	case "unknown":
		require.NoError(t, os.WriteFile(filepath.Join(fixture.dir, "unknown-report.json"), []byte("{}"), 0o600))
	case "future-field":
		data := []byte(`{"metadata":{},"future-reference":"unknown"}`)
		require.NoError(t, os.WriteFile(filepath.Join(fixture.dir, "evaluation-log-future.json"), data, 0o600))
	}
}

func TestInventoryReports_OutOfScopeNeverExpandsAuthority(t *testing.T) {
	for _, coordinate := range []string{"/outside/other-workspace/evidence.json", ".complytime/opa/clone/file.json"} {
		t.Run(coordinate, func(t *testing.T) {
			fixture := newReportFixture(t, "opa")
			fixture.evaluator = fixtureEvaluator("policy", "target", coordinate)
			writeFixtureVariants(t, fixture)
			before := groupSnapshot(t, fixture.workspace)
			result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
			require.ErrorContains(t, err, "possible relationships are unbounded")
			assert.Nil(t, result)
			assert.Equal(t, before, groupSnapshot(t, fixture.workspace))
		})
	}
}

func TestDecodeReport_RejectsAmbiguousSyntax(t *testing.T) {
	for _, input := range []string{
		`{"key":1,"key":2}`, "key: 1\nkey: 2\n", "key: 1\n---\nkey: 2\n",
		"key: &ref 1\nother: *ref\n", strings.Repeat("[", maxReportNesting+1) + "0" + strings.Repeat("]", maxReportNesting+1),
	} {
		t.Run(input, func(t *testing.T) {
			var target map[string]any
			assert.Error(t, decodeReport([]byte(input), &target))
		})
	}
}

func TestInventoryReports_UnsafeAndOversizedReportRefuseWorkspace(t *testing.T) {
	for _, variant := range []string{"symlink", "directory", "oversized"} {
		t.Run(variant, func(t *testing.T) {
			fixture := newReportFixture(t, "opa")
			path := filepath.Join(fixture.dir, "evaluation-log-unsafe.json")
			switch variant {
			case "symlink":
				require.NoError(t, os.Symlink(filepath.Join(fixture.workspace, fixture.coordinate), path))
			case "directory":
				require.NoError(t, os.Mkdir(path, 0o700))
			case "oversized":
				require.NoError(t, os.WriteFile(path, []byte(strings.Repeat(" ", maxReportBytes+1)), 0o600))
			}
			result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func TestInventoryReports_InlinePayloadAndUnrelatedFilesAreNotTargets(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	log := fixture.evaluator.GemaraLog()
	log.Evaluations[0].AssessmentLogs[0].Evidence[0].Source = gemara.EvidenceMapping{}
	// Use the actual OSCAL formatter: payload is intentionally not projected.
	_, err := output.ToOSCAL(log, fixture.dir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(fixture.dir, "notes.txt"), []byte("notes"), 0o600))
	result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	assert.Empty(t, result.Reports[0].Evaluations)
	assert.Equal(t, []string{"report has no establishable evaluation age"}, result.Reports[0].Reasons)
	assert.Contains(t, result.Exclusions, DiscoveryExclusion{
		Path: filepath.Join(reportDirectory, "notes.txt"), Reason: "unrelated report-directory entry"})
}

func TestInventoryReports_UnknownCoordinateCannotAuthorizePartialInventory(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	fixture.evaluator = fixtureEvaluator("policy", "target", "$.unknown[0]")
	_, err := fixture.evaluator.Write(fixture.dir, "json")
	require.NoError(t, err)
	result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unbounded")
	assert.Nil(t, result)
}

func TestParseInventoryOSCAL_RejectsMalformedReport(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	path, err := output.ToOSCAL(fixture.evaluator.GemaraLog(), fixture.dir)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(data, &document))
	document["assessment-results"].(map[string]any)["results"] = nil
	data, err = json.Marshal(document)
	require.NoError(t, err)
	parsed, err := parseInventoryOSCAL(data)
	assert.Error(t, err)
	assert.Nil(t, parsed)
}

func TestInventoryReports_OSCALWithoutStandaloneLogUsesTypedCoordinates(t *testing.T) {
	fixture := newReportFixture(t, "ampel")
	_, err := output.ToOSCAL(fixture.evaluator.GemaraLog(), fixture.dir)
	require.NoError(t, err)
	result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	assert.Equal(t, []string{fixture.unit}, result.Reports[0].Evaluations)
	assert.Empty(t, result.Reports[0].Reasons)
}

func TestInventoryReports_SharedAndMultipleEvaluationEdges(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	second := filepath.Join(managedDirectory, "openscap", resultsDirectory, fixtureUnit)
	require.NoError(t, os.MkdirAll(filepath.Join(fixture.workspace, second), 0o700))
	other := filepath.Join(second, "other.txt")
	require.NoError(t, os.WriteFile(filepath.Join(fixture.workspace, other), []byte("other"), 0o600))
	fixture.evaluator.AddTarget([]provider.AssessmentLog{{
		RequirementID: "second", Steps: []provider.Step{{Name: "inspect", Result: provider.ResultPassed}},
		Evidence: []provider.Evidence{
			{ID: "other", Source: &provider.EvidenceSource{ReferenceID: "retained", Coordinate: other}},
			{ID: "repeat", Source: &provider.EvidenceSource{ReferenceID: "retained", Coordinate: fixture.coordinate}},
		},
	}})
	_, err := fixture.evaluator.Write(fixture.dir, "yaml")
	require.NoError(t, err)
	result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	assert.ElementsMatch(t, []string{fixture.unit, second}, result.Reports[0].Evaluations)
	assert.Empty(t, result.Reports[0].Reasons)
}

func TestInventoryReports_UnsafeEvidenceRefusesUnboundedScope(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	outside := filepath.Join(t.TempDir(), "outside.json")
	require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o600))
	require.NoError(t, os.Remove(filepath.Join(fixture.workspace, fixture.coordinate)))
	require.NoError(t, os.Symlink(outside, filepath.Join(fixture.workspace, fixture.coordinate)))
	writeFixtureVariants(t, fixture)
	result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	require.ErrorContains(t, err, "unbounded")
	require.Nil(t, result)
	data, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "outside", string(data))
}

func TestInventoryReports_CompleteDirectoryBeyondOneBatch(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	path, err := fixture.evaluator.Write(fixture.dir, "json")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	root, err := os.OpenRoot(fixture.dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	for index := range readDirectoryBatch + 1 {
		name := fmt.Sprintf("evaluation-log-copy-%04d.json", index)
		require.NoError(t, root.WriteFile(name, data, 0o600))
	}
	result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	require.NoError(t, err)
	assert.Len(t, result.Reports, readDirectoryBatch+2)
	require.NoError(t, os.WriteFile(filepath.Join(fixture.dir, "scan-unknown-format.xml"), []byte("<unknown/>"), 0o600))
	result, err = InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestLocalEvidencePath_RejectsUnboundedInterpretations(t *testing.T) {
	for _, coordinate := range []string{
		"../../outside.json", "https://example.invalid/evidence", "file://remote/evidence",
		"file:///tmp/evidence?query=1", "file:///tmp/evidence#fragment", "%broken", "$.items[0]",
	} {
		t.Run(coordinate, func(t *testing.T) {
			_, err := localEvidencePath(t.TempDir(), coordinate)
			assert.Error(t, err)
		})
	}
}

func TestParseInventoryMarkdown_RejectsIncompleteOrAmbiguousEmbeds(t *testing.T) {
	for _, content := range []string{
		markdownLogStart + "incomplete", markdownLogStart + "\n```\ninjected" + markdownLogEnd,
		markdownLogStart + markdownLogStart + markdownLogEnd,
	} {
		parsed, err := parseInventoryMarkdown([]byte(content))
		assert.Error(t, err)
		assert.Nil(t, parsed)
	}
}

func TestInventoryReports_MissingMappingReferenceRetainsBoundedReport(t *testing.T) {
	fixture := newReportFixture(t, "opa")
	log := fixture.evaluator.GemaraLog()
	log.Metadata.MappingReferences = nil
	_, err := output.ToOSCAL(log, fixture.dir)
	require.NoError(t, err)
	result, err := InventoryReports(DiscoveryOptions{Workspace: fixture.workspace})
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	assert.Equal(t, []string{fixture.unit}, result.Reports[0].Evaluations)
	require.Len(t, result.Reports[0].Reasons, 1)
	assert.Contains(t, result.Reports[0].Reasons[0], "unresolved mapping-reference")
}
