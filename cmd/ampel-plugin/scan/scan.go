package scan

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/cmd/ampel-plugin/intoto"
	"github.com/complytime/complyctl/cmd/ampel-plugin/targets"
)

// RepoTarget holds the repository information extracted from target variables.
type RepoTarget struct {
	URL         string
	AccessToken string `json:"-"` //nolint:gosec // G117: struct field, not a hardcoded credential
	Platform    string // "github" or "gitlab"
}

//go:embed specs
var embeddedSpecs embed.FS

// ScanConfig holds configuration for scanning a repository.
type ScanConfig struct {
	PolicyPath string
	OutputDir  string
	SpecDir    string
}

// RawScanResult holds the raw output from an AMPEL verify operation.
type RawScanResult struct {
	Output []byte
}

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
	RunWithEnv(env []string, name string, args ...string) ([]byte, error)
}

// ExecRunner executes commands using os/exec.
type ExecRunner struct{}

// Run executes the named command with the given arguments.
func (r ExecRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// RunWithEnv executes the named command with a custom environment.
func (r ExecRunner) RunWithEnv(env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	return cmd.CombinedOutput()
}

// buildTokenEnv creates a copy of the current environment with the
// appropriate platform-specific token variable set for the given repository.
// For github repos it sets GITHUB_TOKEN; for gitlab repos it sets GITLAB_TOKEN.
func buildTokenEnv(repo RepoTarget) []string {
	tokenVar := "GITHUB_TOKEN" //nolint:gosec // env var name, not a credential
	if repo.Platform == "gitlab" {
		tokenVar = "GITLAB_TOKEN"
	}

	env := os.Environ()
	filtered := make([]string, 0, len(env)+1)
	prefix := tokenVar + "="
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered, tokenVar+"="+repo.AccessToken)
	return filtered
}

// WriteSpecFiles writes all embedded spec files to the given directory.
// It automatically discovers and writes all spec files from the embedded FS,
// preserving the directory structure (e.g., specs/github/*.yaml, specs/gitlab/*.yaml).
func WriteSpecFiles(specDir string) error {
	return fs.WalkDir(embeddedSpecs, "specs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root "specs" directory itself
		if path == "specs" {
			return nil
		}

		// Get relative path from "specs/" (e.g., "github/branch-rules.yaml")
		relPath := strings.TrimPrefix(path, "specs/")
		targetPath := filepath.Join(specDir, relPath)

		if d.IsDir() {
			// Create directory
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				return fmt.Errorf("creating spec directory %s: %w", targetPath, err)
			}
			return nil
		}

		// Read embedded file content
		content, err := embeddedSpecs.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading embedded spec %s: %w", path, err)
		}

		// Write to target location
		if err := os.WriteFile(targetPath, content, 0600); err != nil {
			return fmt.Errorf("writing spec file %s: %w", targetPath, err)
		}

		return nil
	})
}

// ResolveSpecPath resolves a spec reference to an absolute path.
// Specs with the "builtin:" prefix are passed through unchanged for snappy
// to handle. Absolute paths are returned as-is. Relative paths containing
// a "/" or ending in ".yaml"/".yml" are resolved against specDir. Bare
// names (snappy built-ins) are passed through unchanged.
func ResolveSpecPath(specRef, specDir string) string {
	if strings.HasPrefix(specRef, "builtin:") {
		return specRef
	}
	if filepath.IsAbs(specRef) {
		return specRef
	}
	if strings.Contains(specRef, "/") ||
		strings.HasSuffix(specRef, ".yaml") ||
		strings.HasSuffix(specRef, ".yml") {
		return filepath.Join(specDir, specRef)
	}
	return specRef
}

// sanitizeSpecName extracts a filesystem-safe label from a spec reference.
// For example, "github/branch-rules.yaml" becomes "branch-rules".
// The "builtin:" prefix is stripped before extracting the base name,
// so "builtin:github/branch-rules.yaml" also becomes "branch-rules".
func sanitizeSpecName(specRef string) string {
	name := strings.TrimPrefix(specRef, "builtin:")
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	if ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}

// constructSnappyCommand builds the snappy snap CLI arguments for collecting
// branch protection data from a repository using a spec file.
// For GitLab specs, it uses HOST, GROUP, PROJECT variables.
// For GitHub specs, it uses ORG, REPO variables.
func constructSnappyCommand(platform, host, org, repo, branch, specPath string) []string {
	args := []string{"snappy", "snap"}

	if platform == "gitlab" {
		args = append(args,
			"--var", fmt.Sprintf("HOST=%s", host),
			"--var", fmt.Sprintf("GROUP=%s", org),
			"--var", fmt.Sprintf("PROJECT=%s", repo),
			"--var", fmt.Sprintf("BRANCH=%s", branch),
		)
	} else {
		args = append(args,
			"--var", fmt.Sprintf("ORG=%s", org),
			"--var", fmt.Sprintf("REPO=%s", repo),
			"--var", fmt.Sprintf("BRANCH=%s", branch),
		)
	}

	args = append(args, specPath, "--attest")
	return args
}

// constructAmpelVerifyCommand builds the ampel verify CLI arguments.
// The subject is the sha256 hash extracted from the snappy attestation.
// resultsPath is the file where ampel writes the in-toto attestation with evaluation results.
func constructAmpelVerifyCommand(subject, policyPath, attestationPath, resultsPath string) []string {
	return []string{
		"ampel", "verify",
		"--subject-hash",
		"sha256:" + subject,
		"-p", policyPath,
		"-a", attestationPath,
		"--attest-results",
		"--results-path", resultsPath,
	}
}

// inTotoStatement represents an in-toto attestation statement.
type inTotoStatement struct {
	Subject []attestationSubject `json:"subject"`
}

// attestationSubject represents a subject in an in-toto statement.
type attestationSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// extractSubjectHash extracts the sha256 hash from an in-toto attestation.
// It supports both raw in-toto statements and DSSE-wrapped attestations.
func extractSubjectHash(attestationData []byte) (string, error) {
	unwrapped, err := intoto.UnwrapDSSE(attestationData)
	if err != nil {
		return "", fmt.Errorf("unwrapping DSSE envelope: %w", err)
	}
	return extractHashFromStatement(unwrapped)
}

func extractHashFromStatement(data []byte) (string, error) {
	var stmt inTotoStatement
	if err := json.Unmarshal(data, &stmt); err != nil {
		return "", fmt.Errorf("parsing in-toto statement: %w", err)
	}

	if len(stmt.Subject) == 0 {
		return "", fmt.Errorf("attestation has no subjects")
	}

	hash, ok := stmt.Subject[0].Digest["sha256"]
	if !ok || hash == "" {
		return "", fmt.Errorf("first subject has no sha256 digest")
	}

	return hash, nil
}

// ScanRepository runs snappy and ampel verify for a single repository, branch,
// and spec file. The specPath must already be resolved (see ResolveSpecPath).
func ScanRepository(repo RepoTarget, branch, specPath string, cfg ScanConfig, runner CommandRunner) (*RawScanResult, error) {
	logger := hclog.Default()

	platform, org, repoName, err := targets.ParseRepoURL(repo.URL, repo.Platform)
	if err != nil {
		return nil, fmt.Errorf("parsing repository URL: %w", err)
	}

	// Extract host from URL for GitLab specs
	host := ""
	if parsedURL, err := url.Parse(repo.URL); err == nil {
		host = parsedURL.Hostname()
	}

	specLabel := sanitizeSpecName(specPath)
	filePrefix := targets.SanitizeRepoURL(repo.URL) + "-" + branch + "-" + specLabel

	// Run snappy to collect branch protection data as an in-toto attestation
	snappyArgs := constructSnappyCommand(platform, host, org, repoName, branch, specPath)
	logger.Info("running snappy", "repo", repo.URL, "branch", branch, "spec", specPath, "command", strings.Join(snappyArgs, " "))

	var attestationData []byte
	if repo.AccessToken != "" {
		tokenEnv := buildTokenEnv(repo)
		attestationData, err = runner.RunWithEnv(tokenEnv, snappyArgs[0], snappyArgs[1:]...)
	} else {
		attestationData, err = runner.Run(snappyArgs[0], snappyArgs[1:]...)
	}
	if err != nil {
		return nil, fmt.Errorf("snappy failed for %s branch %s spec %s: %w (output: %s)", repo.URL, branch, specPath, err, string(attestationData))
	}

	// Save snappy attestation as in-toto file
	attestationFile := filepath.Join(cfg.OutputDir, filePrefix+"-snappy.intoto.json")
	if err := os.WriteFile(attestationFile, attestationData, 0600); err != nil { // #nosec G703 -- attestationFile path is constructed from validated inputs
		return nil, fmt.Errorf("writing attestation for %s branch %s: %w", repo.URL, branch, err)
	}

	// Extract subject hash from the attestation
	subjectHash, err := extractSubjectHash(attestationData)
	if err != nil {
		return nil, fmt.Errorf("extracting subject hash for %s branch %s: %w", repo.URL, branch, err)
	}

	// Run ampel verify with the subject hash, policy, and attestation.
	// ampel writes the in-toto attestation with results to resultsPath.
	// A non-zero exit code means policy checks failed, not a tool error.
	ampelResultFile := filepath.Join(cfg.OutputDir, filePrefix+"-ampel.intoto.json")
	ampelArgs := constructAmpelVerifyCommand(subjectHash, cfg.PolicyPath, attestationFile, ampelResultFile)
	logger.Info("running ampel verify", "repo", repo.URL, "branch", branch, "spec", specPath, "subject", subjectHash)
	ampelCmdOutput, err := runner.Run(ampelArgs[0], ampelArgs[1:]...)
	if len(ampelCmdOutput) > 0 {
		logger.Debug("ampel verify output", "repo", repo.URL, "branch", branch, "output", string(ampelCmdOutput))
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return nil, fmt.Errorf("ampel verify failed for %s branch %s: %w", repo.URL, branch, err)
		}
		logger.Info("ampel verify returned non-zero exit", "repo", repo.URL, "branch", branch, "exit_code", exitErr.ExitCode())
	}

	// Read the in-toto attestation written by ampel
	ampelOut, err := os.ReadFile(ampelResultFile)
	if err != nil {
		return nil, fmt.Errorf("reading ampel results for %s branch %s: %w", repo.URL, branch, err)
	}

	return &RawScanResult{Output: ampelOut}, nil
}
