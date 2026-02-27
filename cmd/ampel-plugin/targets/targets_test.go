package targets

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadTargets_ValidConfig(t *testing.T) {
	config, warnings, err := LoadTargets("testdata/valid-targets.yaml")
	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Len(t, config.Repositories, 2)
	require.Equal(t, "https://github.com/myorg/repo1", config.Repositories[0].URL)
	require.Equal(t, []string{"main", "release"}, config.Repositories[0].Branches)
	require.Equal(t, "https://gitlab.com/myorg/repo2", config.Repositories[1].URL)
	require.Equal(t, []string{"main"}, config.Repositories[1].Branches)
}

func TestLoadTargets_Duplicates(t *testing.T) {
	config, warnings, err := LoadTargets("testdata/duplicates-targets.yaml")
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "duplicate")

	// Should have 2 unique repos after dedup
	totalBranches := 0
	for _, repo := range config.Repositories {
		totalBranches += len(repo.Branches)
	}
	require.Equal(t, 2, totalBranches, "should have 2 unique repo+branch combos")
}

func TestLoadTargets_EmptyConfig(t *testing.T) {
	_, _, err := LoadTargets("testdata/empty-targets.yaml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no repositories")
}

func TestLoadTargets_InvalidURL(t *testing.T) {
	_, _, err := LoadTargets("testdata/invalid-url-targets.yaml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not-a-valid-url")
}

func TestLoadTargets_MissingFile(t *testing.T) {
	_, _, err := LoadTargets("testdata/nonexistent.yaml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading targets file")
}

func TestLoadTargets_MalformedYAML(t *testing.T) {
	// Write a temp file with invalid YAML
	tmpDir := t.TempDir()
	path := tmpDir + "/bad.yaml"
	writeTestFile(t, path, []byte("{ invalid yaml: [["))
	_, _, err := LoadTargets(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing targets file")
}

func TestLoadTargets_EmptyBranches(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/empty-branches.yaml"
	content := `repositories:
  - url: https://github.com/myorg/repo1
    branches: []
`
	writeTestFile(t, path, []byte(content))
	_, _, err := LoadTargets(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "branches list must not be empty")
}

func TestLoadTargets_WithSpecs(t *testing.T) {
	config, warnings, err := LoadTargets("testdata/valid-targets-with-specs.yaml")
	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Len(t, config.Repositories, 2)

	// First repo has explicit specs
	require.Equal(t, []string{"builtin:github/branch-rules.yaml", "/opt/specs/custom-check.yaml"}, config.Repositories[0].Specs)
	// Second repo has no specs (nil)
	require.Nil(t, config.Repositories[1].Specs)
}

func TestLoadTargets_DuplicateSpecs(t *testing.T) {
	config, warnings, err := LoadTargets("testdata/duplicate-specs-targets.yaml")
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "duplicate specs")

	// Should have 2 unique specs after dedup
	require.Equal(t, []string{"github/branch-rules.yaml", "/opt/specs/custom-check.yaml"}, config.Repositories[0].Specs)
}

func TestParseRepoURL_GitHub(t *testing.T) {
	platform, org, repo, err := ParseRepoURL("https://github.com/myorg/myrepo")
	require.NoError(t, err)
	require.Equal(t, "github", platform)
	require.Equal(t, "myorg", org)
	require.Equal(t, "myrepo", repo)
}

func TestParseRepoURL_GitLab(t *testing.T) {
	platform, org, repo, err := ParseRepoURL("https://gitlab.com/myorg/myrepo")
	require.NoError(t, err)
	require.Equal(t, "gitlab", platform)
	require.Equal(t, "myorg", org)
	require.Equal(t, "myrepo", repo)
}

func TestParseRepoURL_UnsupportedHost(t *testing.T) {
	_, _, _, err := ParseRepoURL("https://bitbucket.org/myorg/myrepo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "GitHub or GitLab")
}

func TestParseRepoURL_MissingPath(t *testing.T) {
	_, _, _, err := ParseRepoURL("https://github.com/onlyorg")
	require.Error(t, err)
	require.Contains(t, err.Error(), "org/repo path")
}

func TestParseRepoURL_NonHTTPS(t *testing.T) {
	_, _, _, err := ParseRepoURL("http://github.com/myorg/myrepo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTPS")
}

func TestSanitizeRepoURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/myorg/myrepo", "github-com-myorg-myrepo"},
		{"https://gitlab.com/org/repo", "gitlab-com-org-repo"},
		{"http://github.com/a/b", "github-com-a-b"},
	}
	for _, tc := range tests {
		got := SanitizeRepoURL(tc.input)
		require.Equal(t, tc.expected, got, "input: %s", tc.input)
	}
}

func TestRepoDisplayName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/myorg/myrepo", "myorg/myrepo"},
		{"https://gitlab.com/org/repo", "org/repo"},
		{"not-a-valid-url", "not-a-valid-url"},
	}
	for _, tc := range tests {
		got := RepoDisplayName(tc.input)
		require.Equal(t, tc.expected, got, "input: %s", tc.input)
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, data, 0600))
}
