// SPDX-License-Identifier: Apache-2.0

//go:build acceptance

package acceptance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Keep the schema fixture independent of the production converter so its
// validation cannot hide a malformed acceptance fixture.
const oscalSchemaDocumentJSON = `{
  "assessment-results": {
    "uuid": "00000000-0000-4000-8000-000000000001",
    "metadata": {
      "title": "Evidence schema fixture",
      "last-modified": "2026-01-01T00:00:00Z",
      "version": "2.0.0",
      "oscal-version": "1.1.3"
    },
    "import-ap": {"href": "https://example.test/assessment-plan.json"},
    "results": [{
      "uuid": "00000000-0000-4000-8000-000000000002",
      "title": "Assessment result",
      "description": "Evidence-bearing assessment result",
      "start": "2026-01-01T00:00:00Z",
      "reviewed-controls": {"control-selections": [{"include-all": {}}]},
      "observations": [{
        "uuid": "00000000-0000-4000-8000-000000000003",
        "description": "Assessment observation",
        "methods": ["TEST"],
        "collected": "2026-01-01T00:00:00Z",
        "relevant-evidence": [{
          "href": "https://example.test/evidence.json",
          "description": "test evidence for block-force-push"
        }]
      }]
    }]
  }
}`

func newOSCALSchemaFixture(t *testing.T) map[string]interface{} {
	t.Helper()
	var document map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(oscalSchemaDocumentJSON), &document))
	return document
}

func writeOSCALSchemaFixture(t *testing.T, document map[string]interface{}) string {
	t.Helper()
	data, err := json.Marshal(document)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "assessment-results.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

// TestOSCALSchema_Valid accepts a complete evidence-bearing report from disk.
func TestOSCALSchema_Valid(t *testing.T) {
	document := newOSCALSchemaFixture(t)
	assert.NoError(t, checkOSCALSchema(writeOSCALSchemaFixture(t, document)))
}

// TestOSCALSchema_RejectsInvalidEvidence checks evidence beyond the first result
// and observation, rather than validating a selected result or evidence item.
func TestOSCALSchema_RejectsInvalidEvidence(t *testing.T) {
	for _, location := range []string{"first_observation", "later_observation", "later_result"} {
		t.Run(location, func(t *testing.T) {
			document := newOSCALSchemaFixture(t)
			assessment := document["assessment-results"].(map[string]interface{})
			results, err := oscalObjects(assessment, "results")
			require.NoError(t, err)
			result := results[0]
			if location == "later_result" {
				other := newOSCALSchemaFixture(t)["assessment-results"].(map[string]interface{})
				otherResults, err := oscalObjects(other, "results")
				require.NoError(t, err)
				result = otherResults[0]
				result["uuid"] = "00000000-0000-4000-8000-000000000004"
				assessment["results"] = []interface{}{results[0], result}
			}
			observations, err := oscalObjects(result, "observations")
			require.NoError(t, err)
			observation := observations[0]
			if location == "later_observation" {
				observation = map[string]interface{}{}
				for key, value := range observations[0] {
					observation[key] = value
				}
				observation["uuid"] = "00000000-0000-4000-8000-000000000005"
				result["observations"] = []interface{}{observations[0], observation}
			}
			path := writeOSCALSchemaFixture(t, document)
			require.NoError(t, checkOSCALSchema(path), "unmodified document must be valid")
			// Replace the slice so the earlier observation's evidence stays valid.
			observation["relevant-evidence"] = []interface{}{map[string]interface{}{
				"description": "test evidence for block-force-push", "href": "https://[invalid",
			}}
			err = checkOSCALSchema(writeOSCALSchemaFixture(t, document))
			require.Error(t, err)
			assert.ErrorContains(t, err, "validate OSCAL 1.1.3 document")
			assert.ErrorContains(t, err, "relevant-evidence")
			assert.ErrorContains(t, err, "href")
		})
	}
}

// TestOSCALSchema_RejectsInvalidDocument proves the envelope and original JSON
// fields are validated, including fields that a typed model would discard.
func TestOSCALSchema_RejectsInvalidDocument(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]interface{})
		want   string
	}{
		{"invalid_metadata", func(document map[string]interface{}) {
			assessment := document["assessment-results"].(map[string]interface{})
			delete(assessment["metadata"].(map[string]interface{}), "title")
		}, "title"},
		{"unknown_field", func(document map[string]interface{}) {
			assessment := document["assessment-results"].(map[string]interface{})
			assessment["unexpected-field"] = true
		}, "unexpected-field"},
		{"null_metadata", func(document map[string]interface{}) {
			assessment := document["assessment-results"].(map[string]interface{})
			assessment["metadata"] = nil
		}, "metadata"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := newOSCALSchemaFixture(t)
			tt.mutate(document)
			err := checkOSCALSchema(writeOSCALSchemaFixture(t, document))
			assert.ErrorContains(t, err, "validate OSCAL 1.1.3 document")
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

// TestOSCALSchema_RejectsUnreadableOrMalformedReport checks helper error context.
func TestOSCALSchema_RejectsUnreadableOrMalformedReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "assessment-results.json")
	err := checkOSCALSchema(path)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.ErrorContains(t, err, "read OSCAL document")
	for _, tt := range []struct{ name, data, want string }{
		{"malformed_JSON", `{"assessment-results":`, "decode OSCAL document"},
		{"missing_assessment_results", `{}`, "assessment-results must be present"},
		{"null_document", `null`, "assessment-results must be present"},
		{"null_assessment_results", `{"assessment-results":null}`, "assessment-results must be present"},
		{"validator_setup", `{"assessment-results":{},"catalog":{}}`, "create OSCAL 1.1.3 schema validator"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "assessment-results.json")
			require.NoError(t, os.WriteFile(path, []byte(tt.data), 0o600))
			err := checkOSCALSchema(path)
			assert.ErrorContains(t, err, tt.want)
			assert.ErrorContains(t, err, path)
		})
	}
}
