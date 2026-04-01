package targets

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRepoURL_GitHub(t *testing.T) {
	platform, org, repo, err := ParseRepoURL("https://github.com/myorg/myrepo", "")
	require.NoError(t, err)
	require.Equal(t, "github", platform)
	require.Equal(t, "myorg", org)
	require.Equal(t, "myrepo", repo)
}

func TestParseRepoURL_GitLab(t *testing.T) {
	platform, org, repo, err := ParseRepoURL("https://gitlab.com/myorg/myrepo", "")
	require.NoError(t, err)
	require.Equal(t, "gitlab", platform)
	require.Equal(t, "myorg", org)
	require.Equal(t, "myrepo", repo)
}

func TestParseRepoURL_GitLabNestedGroups(t *testing.T) {
	platform, group, repo, err := ParseRepoURL("https://gitlab.com/mygroup/subgroup/myproject", "")
	require.NoError(t, err)
	require.Equal(t, "gitlab", platform)
	require.Equal(t, "mygroup/subgroup", group)
	require.Equal(t, "myproject", repo)
}

func TestParseRepoURL_GitLabDeeplyNestedGroups(t *testing.T) {
	platform, group, repo, err := ParseRepoURL("https://gitlab.com/a/b/c/d/project", "")
	require.NoError(t, err)
	require.Equal(t, "gitlab", platform)
	require.Equal(t, "a/b/c/d", group)
	require.Equal(t, "project", repo)
}

func TestParseRepoURL_UnsupportedHost(t *testing.T) {
	_, _, _, err := ParseRepoURL("https://bitbucket.org/myorg/myrepo", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown host")
}

func TestParseRepoURL_MissingPath(t *testing.T) {
	_, _, _, err := ParseRepoURL("https://github.com/onlyorg", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "org/repo path")
}

func TestParseRepoURL_NonHTTPS(t *testing.T) {
	_, _, _, err := ParseRepoURL("http://github.com/myorg/myrepo", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTPS")
}

func TestParseRepoURL_WithPlatformHint(t *testing.T) {
	platform, org, repo, err := ParseRepoURL("https://git.corp.com/myorg/repo", "github")
	require.NoError(t, err)
	require.Equal(t, "github", platform)
	require.Equal(t, "myorg", org)
	require.Equal(t, "repo", repo)
}

func TestParseRepoURL_HintOverridesHostDetection(t *testing.T) {
	platform, _, _, err := ParseRepoURL("https://github.com/org/repo", "gitlab")
	require.NoError(t, err)
	require.Equal(t, "gitlab", platform)
}

func TestParseRepoURL_SelfHostedWithoutHint(t *testing.T) {
	_, _, _, err := ParseRepoURL("https://git.corp.com/org/repo", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown host")
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
