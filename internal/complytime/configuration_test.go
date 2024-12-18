package complytime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplicationDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	appDir, err := newApplicationDirectory(tmpDir, false)
	require.NoError(t, err)

	expectedAppDir := filepath.Join(tmpDir, "complytime")
	expectedPluginDir := filepath.Join(tmpDir, "complytime", "plugins")
	expectedBundleDir := filepath.Join(tmpDir, "complytime", "bundles")

	require.Equal(t, expectedAppDir, appDir.AppDir())
	require.Equal(t, expectedPluginDir, appDir.PluginDir())
	require.Equal(t, expectedBundleDir, appDir.BundleDir())
	require.Equal(t, []string{expectedAppDir, expectedPluginDir, expectedBundleDir}, appDir.Dirs())

	appDir, err = newApplicationDirectory(tmpDir, true)
	require.NoError(t, err)
	_, err = os.Stat(appDir.AppDir())
	require.NoError(t, err)
	_, err = os.Stat(appDir.PluginDir())
	require.NoError(t, err)
	_, err = os.Stat(appDir.BundleDir())
	require.NoError(t, err)
}

func TestFindComponentDefinitions(t *testing.T) {
	compDefs, err := FindComponentDefinitions("testdata/bundles")
	require.NoError(t, err)
	require.Len(t, compDefs, 1)

	_, err = FindComponentDefinitions("testdata/")
	require.ErrorIs(t, err, ErrNoComponentDefinitionsFound)

}
