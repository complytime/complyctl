// SPDX-License-Identifier: Apache-2.0

package output_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/output"
)

func TestToSARIF_ProducesValidJSON(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()

	path, err := output.ToSARIF(log, "file:///scan", outDir)
	require.NoError(t, err)
	assert.FileExists(t, path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var sarifDoc map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &sarifDoc))

	assert.Contains(t, sarifDoc, "$schema")
	assert.Contains(t, sarifDoc, "version")

	runs, ok := sarifDoc["runs"].([]interface{})
	assert.True(t, ok)
	assert.NotEmpty(t, runs)
}

func TestToSARIF_OutputFileNaming(t *testing.T) {
	outDir := t.TempDir()
	log := mockGemaraEvalLog()

	path, err := output.ToSARIF(log, "file:///scan", outDir)
	require.NoError(t, err)

	filename := filepath.Base(path)
	assert.Contains(t, filename, "scan-test-policy-")
	assert.Contains(t, filename, "web-server")
	assert.Contains(t, filename, ".sarif.json")
}
