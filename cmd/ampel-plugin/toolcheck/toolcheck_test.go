package toolcheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckTools_KnownGoodTool(t *testing.T) {
	// Temporarily override RequiredTools with a tool that exists on all systems
	origTools := RequiredTools
	RequiredTools = []string{"ls"}
	defer func() { RequiredTools = origTools }()

	missing, err := CheckTools()
	require.NoError(t, err)
	require.Empty(t, missing)
}

func TestCheckTools_OneMissing(t *testing.T) {
	origTools := RequiredTools
	RequiredTools = []string{"ls", "nonexistent-tool-xyz-12345"}
	defer func() { RequiredTools = origTools }()

	missing, err := CheckTools()
	require.NoError(t, err)
	require.Len(t, missing, 1)
	require.Equal(t, "nonexistent-tool-xyz-12345", missing[0])
}

func TestCheckTools_AllMissing(t *testing.T) {
	origTools := RequiredTools
	RequiredTools = []string{"nonexistent-tool-abc", "nonexistent-tool-xyz"}
	defer func() { RequiredTools = origTools }()

	missing, err := CheckTools()
	require.NoError(t, err)
	require.Len(t, missing, 2)
	require.Contains(t, missing, "nonexistent-tool-abc")
	require.Contains(t, missing, "nonexistent-tool-xyz")
}

func TestFormatMissingToolsError_OneTool(t *testing.T) {
	err := FormatMissingToolsError([]string{"ampel"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ampel")
	require.Contains(t, err.Error(), "PATH")
}

func TestFormatMissingToolsError_MultipleTools(t *testing.T) {
	err := FormatMissingToolsError([]string{"snappy", "ampel"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "snappy")
	require.Contains(t, err.Error(), "ampel")
}

func TestFormatMissingToolsError_MentionsPATH(t *testing.T) {
	err := FormatMissingToolsError([]string{"snappy"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PATH")
}

func TestFormatMissingToolsError_Empty(t *testing.T) {
	err := FormatMissingToolsError(nil)
	require.NoError(t, err)
}
