// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyStoreBackupPath_RetainsReservationDirectory(t *testing.T) {
	activePath := filepath.Join(t.TempDir(), "policy")
	require.NoError(t, os.MkdirAll(activePath, 0o700))

	backupPath, err := policyStoreBackupPath(activePath)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, os.RemoveAll(backupPath)) })

	assert.DirExists(t, backupPath)
	assert.Equal(t, filepath.Join(backupPath, "previous"), policyStoreBackupContentPath(backupPath))
}
