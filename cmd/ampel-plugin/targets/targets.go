package targets

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseRepoURL extracts the hosting platform, organization, and repository
// name from a repository URL. The URL must use HTTPS.
//
// If platformHint is non-empty, it is used as the platform name (for
// self-hosted instances). If platformHint is empty, the platform is detected
// from the hostname (github.com → "github", gitlab.com → "gitlab").
func ParseRepoURL(repoURL, platformHint string) (platform, org, repo string, err error) {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL %q: %w", repoURL, err)
	}

	if parsed.Scheme != "https" {
		return "", "", "", fmt.Errorf("URL %q must use HTTPS scheme", repoURL)
	}

	path := strings.Trim(parsed.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("URL %q must contain org/repo path", repoURL)
	}

	if platformHint != "" {
		platform = platformHint
	} else {
		host := strings.ToLower(parsed.Hostname())
		if strings.Contains(host, "github.com") {
			platform = "github"
		} else if strings.Contains(host, "gitlab.com") {
			platform = "gitlab"
		} else {
			return "", "", "", fmt.Errorf("URL %q: unknown host (set 'platform' variable for self-hosted instances)", repoURL)
		}
	}

	// For GitLab, support nested groups: all segments except the last form the group path.
	// For GitHub, use the first two segments (org/repo).
	if platform == "gitlab" && len(parts) > 2 {
		org = strings.Join(parts[:len(parts)-1], "/")
		repo = parts[len(parts)-1]
	} else {
		org = parts[0]
		repo = parts[1]
	}

	return platform, org, repo, nil
}

// SanitizeRepoURL converts a repository URL into a filesystem-safe name
// by stripping the scheme and replacing special characters with hyphens.
func SanitizeRepoURL(repoURL string) string {
	name := repoURL
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(name, prefix) {
			name = name[len(prefix):]
			break
		}
	}
	var result []rune
	for _, r := range name {
		if r == '/' || r == '.' || r == ':' {
			result = append(result, '-')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// RepoDisplayName extracts the "org/repo" portion from a repository URL
// for use in human-readable output.
func RepoDisplayName(repoURL string) string {
	_, org, repo, err := ParseRepoURL(repoURL, "")
	if err != nil {
		return repoURL
	}
	return org + "/" + repo
}
