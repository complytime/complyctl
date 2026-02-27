package scan

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/cmd/ampel-plugin/intoto"
	"github.com/complytime/complyctl/cmd/ampel-plugin/targets"
)

//go:embed specs/github/branch-rules.yaml
var githubBranchRulesSpec []byte

// GitHubSpecFile is the filename for the GitHub branch rules spec.
const GitHubSpecFile = "branch-rules.yaml"

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
}

// ExecRunner executes commands using os/exec.
type ExecRunner struct{}

// Run executes the named command with the given arguments.
func (r ExecRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// WriteSpecFiles writes the embedded spec files to the given directory.
func WriteSpecFiles(specDir string) error {
	githubDir := filepath.Join(specDir, "github")
	if err := os.MkdirAll(githubDir, 0750); err != nil {
		return fmt.Errorf("creating spec directory %s: %w", githubDir, err)
	}

	specPath := filepath.Join(githubDir, GitHubSpecFile)
	if err := os.WriteFile(specPath, githubBranchRulesSpec, 0600); err != nil {
		return fmt.Errorf("writing spec file %s: %w", specPath, err)
	}

	return nil
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
func constructSnappyCommand(org, repo, branch, specPath string) []string {
	return []string{
		"snappy", "snap",
		"--var", fmt.Sprintf("ORG=%s", org),
		"--var", fmt.Sprintf("REPO=%s", repo),
		"--var", fmt.Sprintf("BRANCH=%s", branch),
		specPath,
		"--attest",
	}
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
func ScanRepository(repo targets.TargetRepository, branch, specPath string, cfg ScanConfig, runner CommandRunner) (*RawScanResult, error) {
	logger := hclog.Default()

	platform, org, repoName, err := targets.ParseRepoURL(repo.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing repository URL: %w", err)
	}

	if platform != "github" {
		return nil, fmt.Errorf("snappy specs are currently only available for GitHub repositories; %s is not supported", platform)
	}

	specLabel := sanitizeSpecName(specPath)
	filePrefix := targets.SanitizeRepoURL(repo.URL) + "-" + branch + "-" + specLabel

	// Run snappy to collect branch protection data as an in-toto attestation
	snappyArgs := constructSnappyCommand(org, repoName, branch, specPath)
	logger.Info("running snappy", "repo", repo.URL, "branch", branch, "spec", specPath, "command", strings.Join(snappyArgs, " "))
	attestationData, err := runner.Run(snappyArgs[0], snappyArgs[1:]...)
	if err != nil {
		return nil, fmt.Errorf("snappy failed for %s branch %s spec %s: %w (output: %s)", repo.URL, branch, specPath, err, string(attestationData))
	}

	// Save snappy attestation as in-toto file
	attestationFile := filepath.Join(cfg.OutputDir, filePrefix+"-snappy.intoto.json")
	if err := os.WriteFile(attestationFile, attestationData, 0600); err != nil {
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

