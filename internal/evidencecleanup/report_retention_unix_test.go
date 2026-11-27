// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_CaseVariantFieldsRefuseBeforeDeletion(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		for _, placement := range []string{"before", "after"} {
			t.Run(format+"/"+placement, func(t *testing.T) {
				workspace := t.TempDir()
				requirePreviewSupport(t, workspace)
				unit := addGroupEvaluation(t, workspace, "opa", "old", groupOldStart)
				report := addGroupReport(t, workspace, "old", filepath.Join(unit, "first.json"))
				root, err := os.OpenRoot(workspace)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, root.Close()) })
				data, err := root.ReadFile(report)
				require.NoError(t, err)
				data = bytes.TrimSpace(data)
				if placement == "before" {
					data = append([]byte(`{"Evaluations":[],`), data[1:]...)
				} else {
					data = append(data[:len(data)-1], []byte(`,"Evaluations":[]}`)...)
				}
				if format == "yaml" {
					data, err = yaml.JSONToYAML(data)
					require.NoError(t, err)
					require.NoError(t, root.Remove(report))
					report += ".yaml"
				}
				require.NoError(t, root.WriteFile(report, data, 0o600))
				assertCleanupRefusesWithoutMutation(t, workspace)
			})
		}
	}
}

func TestRun_SymlinkCoordinateProtectsOtherEvaluation(t *testing.T) {
	for _, kind := range []string{"directory", "file"} {
		t.Run(kind, func(t *testing.T) {
			workspace := t.TempDir()
			requirePreviewSupport(t, workspace)
			a := addGroupEvaluation(t, workspace, "opa", "a", groupOldStart)
			b := addGroupEvaluation(t, workspace, "opa", "b", groupOldStart)
			target := filepath.Join(workspace, b)
			coordinate := filepath.Join(a, "link", "first.json")
			if kind == "file" {
				target = filepath.Join(target, "first.json")
				coordinate = filepath.Join(a, "link")
			}
			require.NoError(t, os.Symlink(target, filepath.Join(workspace, a, "link")))
			addGroupReport(t, workspace, "link", coordinate)
			assertCleanupRefusesWithoutMutation(t, workspace)
		})
	}
}

func TestRun_SymlinkOutsideResultsProtectsReferencedEvaluation(t *testing.T) {
	for _, kind := range []string{"directory", "file"} {
		t.Run(kind, func(t *testing.T) {
			workspace := t.TempDir()
			requirePreviewSupport(t, workspace)
			unit := addGroupEvaluation(t, workspace, "opa", "old", groupOldStart)
			target := filepath.Join(workspace, unit)
			coordinate := filepath.Join("evidence-link", "first.json")
			if kind == "file" {
				target = filepath.Join(target, "first.json")
				coordinate = "evidence-link"
			}
			require.NoError(t, os.Symlink(target, filepath.Join(workspace, "evidence-link")))
			addGroupReport(t, workspace, "link", coordinate)
			assertCleanupRefusesWithoutMutation(t, workspace)
		})
	}
}

func assertCleanupRefusesWithoutMutation(t *testing.T, workspace string) {
	t.Helper()
	before := groupSnapshot(t, workspace)
	_, err := Preview(previewOptions(t, workspace))
	require.Error(t, err)
	assert.Equal(t, before, groupSnapshot(t, workspace))
	result, err := Run(executionOptions(t, workspace))
	require.Error(t, err)
	if result != nil {
		assert.Zero(t, result.Totals.DeletedMembers)
	}
	assert.Equal(t, before, groupSnapshot(t, workspace))
}

func TestRun_CaseEquivalentCoordinateProtectsEvaluation(t *testing.T) {
	for _, filename := range []string{"first.json", "missing.json"} {
		t.Run(filename, func(t *testing.T) {
			workspace := t.TempDir()
			requirePreviewSupport(t, workspace)
			unit := addGroupEvaluation(t, workspace, "opa", "old", groupOldStart)
			alternate := strings.Replace(unit, "opa", "OPA", 1)
			original, err := os.Stat(filepath.Join(workspace, unit))
			require.NoError(t, err)
			variant, err := os.Stat(filepath.Join(workspace, alternate))
			if os.IsNotExist(err) {
				t.Skip("native filesystem uses case-sensitive pathname lookup")
			}
			require.NoError(t, err)
			require.True(t, os.SameFile(original, variant), "fixture must resolve to the same evaluation")
			addGroupReport(t, workspace, "case-equivalent", filepath.Join(alternate, filename))
			assertCleanupRefusesWithoutMutation(t, workspace)
		})
	}
}

func TestRun_UnmatchedCoordinateRefusesUnprovenRelationships(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(fmt.Sprintf("exists=%t", exists), func(t *testing.T) {
			workspace := t.TempDir()
			requirePreviewSupport(t, workspace)
			addGroupEvaluation(t, workspace, "opa", "old", groupOldStart)
			if exists {
				root, err := os.OpenRoot(workspace)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, root.Close()) })
				require.NoError(t, root.WriteFile("unmatched.json", []byte("unclassified evidence"), 0o600))
			}
			addGroupReport(t, workspace, "unmatched", "unmatched.json")
			assertCleanupRefusesWithoutMutation(t, workspace)
		})
	}
}

func TestRun_ExternalAliasProtectsReferencedEvaluation(t *testing.T) {
	for _, kind := range []string{"directory", "file"} {
		for _, fileURI := range []bool{false, true} {
			for _, filename := range []string{"first.json", "missing.json"} {
				t.Run(fmt.Sprintf("%s/uri=%t/%s", kind, fileURI, filename), func(t *testing.T) {
					workspace := t.TempDir()
					requirePreviewSupport(t, workspace)
					unit := addGroupEvaluation(t, workspace, "opa", "old", groupOldStart)
					target := filepath.Join(workspace, unit)
					alias := filepath.Join(t.TempDir(), "evidence-alias")
					coordinate := filepath.Join(alias, filename)
					if kind == "file" {
						target = filepath.Join(target, filename)
						coordinate = alias
					}
					require.NoError(t, os.Symlink(target, alias))
					if fileURI {
						coordinate = "file://" + coordinate
					}
					addGroupReport(t, workspace, "external-alias", coordinate)
					assertCleanupRefusesWithoutMutation(t, workspace)
				})
			}
		}
	}
}
