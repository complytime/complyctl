package targets

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
)

// TargetConfig is the top-level structure of the target repository configuration file.
type TargetConfig struct {
	Repositories []TargetRepository `yaml:"repositories"`
}

// TargetRepository represents a GitHub or GitLab repository to scan.
type TargetRepository struct {
	URL      string   `yaml:"url"`
	Branches []string `yaml:"branches"`
	Specs    []string `yaml:"specs,omitempty"`
}

// LoadTargets reads and validates the target configuration file.
// It returns the parsed config, a list of warning messages for duplicates,
// and any error encountered.
func LoadTargets(path string) (*TargetConfig, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading targets file %q: %w", path, err)
	}

	var config TargetConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, nil, fmt.Errorf("parsing targets file: %w", err)
	}

	if len(config.Repositories) == 0 {
		return nil, nil, fmt.Errorf("targets file contains no repositories")
	}

	for i, repo := range config.Repositories {
		if err := validateRepoURL(repo.URL); err != nil {
			return nil, nil, fmt.Errorf("repository %d: %w", i, err)
		}
		if len(repo.Branches) == 0 {
			return nil, nil, fmt.Errorf("repository %d (%s): branches list must not be empty", i, repo.URL)
		}
	}

	deduped, warnings := deduplicateTargets(config.Repositories)
	config.Repositories = deduped

	return &config, warnings, nil
}

// validateRepoURL checks that a repository URL is a valid HTTPS URL
// pointing to a GitHub or GitLab host.
func validateRepoURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("URL %q must use HTTPS scheme", rawURL)
	}

	host := strings.ToLower(parsed.Hostname())
	if !strings.Contains(host, "github.com") && !strings.Contains(host, "gitlab.com") {
		return fmt.Errorf("URL %q must point to a GitHub or GitLab host", rawURL)
	}

	return nil
}

// deduplicateTargets removes duplicate URL+branch combinations and
// deduplicates specs within each repo entry. It returns warnings for
// each duplicate found.
func deduplicateTargets(repos []TargetRepository) ([]TargetRepository, []string) {
	type key struct {
		url    string
		branch string
	}
	seen := make(map[key]bool)
	var result []TargetRepository
	var warnings []string

	for _, repo := range repos {
		var uniqueBranches []string
		for _, branch := range repo.Branches {
			k := key{url: repo.URL, branch: branch}
			if seen[k] {
				warnings = append(warnings, fmt.Sprintf("duplicate target: %s branch %s", repo.URL, branch))
				continue
			}
			seen[k] = true
			uniqueBranches = append(uniqueBranches, branch)
		}
		if len(uniqueBranches) > 0 {
			dedupedSpecs := deduplicateSpecs(repo.Specs)
			if len(dedupedSpecs) != len(repo.Specs) {
				warnings = append(warnings, fmt.Sprintf("duplicate specs removed for %s", repo.URL))
			}
			result = append(result, TargetRepository{
				URL:      repo.URL,
				Branches: uniqueBranches,
				Specs:    dedupedSpecs,
			})
		}
	}

	return result, warnings
}

// deduplicateSpecs removes duplicate entries from a specs slice while
// preserving order.
func deduplicateSpecs(specs []string) []string {
	if len(specs) == 0 {
		return specs
	}
	seen := make(map[string]bool, len(specs))
	unique := make([]string, 0, len(specs))
	for _, s := range specs {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}
	return unique
}
