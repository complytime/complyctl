package scan

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"

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

// parseRepoURL extracts the hosting platform, organization, and repository
// name from a repository URL.
func parseRepoURL(repoURL string) (platform, org, repo string, err error) {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL %q: %w", repoURL, err)
	}

	host := strings.ToLower(parsed.Hostname())
	path := strings.Trim(parsed.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("URL %q must contain org/repo path", repoURL)
	}

	if strings.Contains(host, "github.com") {
		platform = "github"
	} else if strings.Contains(host, "gitlab.com") {
		platform = "gitlab"
	} else {
		return "", "", "", fmt.Errorf("unsupported host in %q: must be github.com or gitlab.com", repoURL)
	}

	return platform, parts[0], parts[1], nil
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

// dsseEnvelope represents a DSSE signed envelope.
type dsseEnvelope struct {
	PayloadType string `json:"payloadType"`
	Payload     string `json:"payload"`
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
	var envelope dsseEnvelope
	if err := json.Unmarshal(attestationData, &envelope); err == nil && envelope.PayloadType != "" && envelope.Payload != "" {
		// DSSE uses base64url encoding without padding
		decoded, err := base64.RawURLEncoding.DecodeString(envelope.Payload)
		if err != nil {
			// Fall back to standard base64
			decoded, err = base64.StdEncoding.DecodeString(envelope.Payload)
			if err != nil {
				return "", fmt.Errorf("decoding DSSE payload: %w", err)
			}
		}
		return extractHashFromStatement(decoded)
	}

	return extractHashFromStatement(attestationData)
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

	platform, org, repoName, err := parseRepoURL(repo.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing repository URL: %w", err)
	}

	if platform != "github" {
		return nil, fmt.Errorf("snappy specs are currently only available for GitHub repositories; %s is not supported", platform)
	}

	specLabel := sanitizeSpecName(specPath)
	filePrefix := sanitizeRepoName(repo.URL) + "-" + branch + "-" + specLabel

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

// sanitizeRepoName converts a repository URL into a safe filename component.
func sanitizeRepoName(repoURL string) string {
	name := repoURL
	for _, prefix := range []string{"https://", "http://"} {
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			name = name[len(prefix):]
			break
		}
	}
	replacer := func(r rune) rune {
		if r == '/' || r == '.' || r == ':' {
			return '-'
		}
		return r
	}
	result := make([]rune, 0, len(name))
	for _, r := range name {
		result = append(result, replacer(r))
	}
	return string(result)
}
