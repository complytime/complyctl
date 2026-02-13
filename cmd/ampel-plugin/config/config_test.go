package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadSettings_AllRequiredFields(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace": dir,
		"profile":   "test-profile",
	})
	require.NoError(t, err)
	require.Equal(t, dir, c.Workspace)
	require.Equal(t, "test-profile", c.Profile)
}

func TestLoadSettings_MissingWorkspace(t *testing.T) {
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"profile": "test-profile",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "workspace")
}

func TestLoadSettings_MissingProfile(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace": dir,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "profile")
}

func TestLoadSettings_AppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace": dir,
		"profile":   "test",
	})
	require.NoError(t, err)
	ampelDir := filepath.Join(dir, PluginDir)
	require.Equal(t, filepath.Join(ampelDir, DefaultPolicyDir), c.PolicyDir)
	require.Equal(t, filepath.Join(ampelDir, DefaultResultsDir), c.ResultsDir)
	require.Equal(t, filepath.Join(ampelDir, DefaultTargetsFile), c.TargetsFile)
}

func TestLoadSettings_CustomOptionalFields(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace":    dir,
		"profile":      "test",
		"policy_dir":   "custom-policy",
		"results_dir":  "custom-results",
		"targets_file": "custom-targets.yaml",
	})
	require.NoError(t, err)
	ampelDir := filepath.Join(dir, PluginDir)
	require.Equal(t, filepath.Join(ampelDir, "custom-policy"), c.PolicyDir)
	require.Equal(t, filepath.Join(ampelDir, "custom-results"), c.ResultsDir)
	require.Equal(t, filepath.Join(ampelDir, "custom-targets.yaml"), c.TargetsFile)
}

func TestLoadSettings_EmptyWorkspaceValue(t *testing.T) {
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace": "",
		"profile":   "test",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "workspace")
}

func TestLoadSettings_EmptyProfileValue(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace": dir,
		"profile":   "",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "profile")
}

func TestLoadSettings_CreatesWorkspaceDir(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace": dir,
		"profile":   "test",
	})
	require.NoError(t, err)

	ampelDir := filepath.Join(dir, PluginDir)
	info, err := os.Stat(ampelDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

// T033: Path resolution tests

func TestResolvePaths_AbsolutePolicyDirUnchanged(t *testing.T) {
	dir := t.TempDir()
	absPolicy := filepath.Join(dir, "abs-policy")
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace":  dir,
		"profile":    "test",
		"policy_dir": absPolicy,
	})
	require.NoError(t, err)
	require.Equal(t, absPolicy, c.PolicyDirPath())
}

func TestResolvePaths_RelativePolicyDirResolved(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace":  dir,
		"profile":    "test",
		"policy_dir": "my-policies",
	})
	require.NoError(t, err)
	expected := filepath.Join(dir, PluginDir, "my-policies")
	require.Equal(t, expected, c.PolicyDirPath())
}

func TestResolvePaths_CreatesNonExistentDirs(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace":   dir,
		"profile":     "test",
		"policy_dir":  "new-policy-dir",
		"results_dir": "new-results-dir",
	})
	require.NoError(t, err)

	policyInfo, err := os.Stat(c.PolicyDirPath())
	require.NoError(t, err)
	require.True(t, policyInfo.IsDir())

	resultsInfo, err := os.Stat(c.ResultsDirPath())
	require.NoError(t, err)
	require.True(t, resultsInfo.IsDir())
}

func TestResolvePaths_CustomTargetsFileResolved(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace":    dir,
		"profile":      "test",
		"targets_file": "my-targets.yaml",
	})
	require.NoError(t, err)
	expected := filepath.Join(dir, PluginDir, "my-targets.yaml")
	require.Equal(t, expected, c.TargetsFilePath())
}

func TestResolvePaths_AbsoluteTargetsFileUnchanged(t *testing.T) {
	dir := t.TempDir()
	absTargets := filepath.Join(dir, "my-targets.yaml")
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace":    dir,
		"profile":      "test",
		"targets_file": absTargets,
	})
	require.NoError(t, err)
	require.Equal(t, absTargets, c.TargetsFilePath())
}

func TestResolvePaths_CustomResultsDirResolved(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace":   dir,
		"profile":     "test",
		"results_dir": "custom-results",
	})
	require.NoError(t, err)
	expected := filepath.Join(dir, PluginDir, "custom-results")
	require.Equal(t, expected, c.ResultsDirPath())
}

func TestResolvePaths_EmptyCustomPathFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	c := NewConfig()
	err := c.LoadSettings(map[string]string{
		"workspace":  dir,
		"profile":    "test",
		"policy_dir": "",
	})
	require.NoError(t, err)
	expected := filepath.Join(dir, PluginDir, DefaultPolicyDir)
	require.Equal(t, expected, c.PolicyDirPath())
}

func TestValidate_EmptyWorkspace(t *testing.T) {
	c := &Config{Profile: "test"}
	err := c.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "workspace")
}

func TestValidate_EmptyProfile(t *testing.T) {
	c := &Config{Workspace: "/tmp/test"}
	err := c.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "profile")
}
