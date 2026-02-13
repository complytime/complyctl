package scan

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/cmd/ampel-plugin/targets"
)

// ScanConfig holds configuration for scanning a repository.
type ScanConfig struct {
	PolicyPath string
	OutputDir  string
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

// constructSnappyCommand builds the snappy CLI arguments for collecting
// branch protection data from a repository.
func constructSnappyCommand(repo targets.TargetRepository, branch, outputDir string) []string {
	outputFile := filepath.Join(outputDir, sanitizeRepoName(repo.URL)+"-"+branch+".json")
	return []string{
		"snappy",
		"--repo", repo.URL,
		"--branch", branch,
		"--output", outputFile,
	}
}

// constructAmpelVerifyCommand builds the ampel verify CLI arguments.
func constructAmpelVerifyCommand(policyPath, attestationPath, subjectRepo string) []string {
	return []string{
		"ampel",
		"verify",
		"--policy", policyPath,
		"--attestation", attestationPath,
		"--subject", subjectRepo,
		"--output", "json",
	}
}

// ScanRepository runs snappy and ampel verify for a single repository and branch.
func ScanRepository(repo targets.TargetRepository, branch string, cfg ScanConfig, runner CommandRunner) (*RawScanResult, error) {
	logger := hclog.Default()

	// Run snappy to collect branch protection data
	snappyArgs := constructSnappyCommand(repo, branch, cfg.OutputDir)
	logger.Info("running snappy", "repo", repo.URL, "branch", branch)
	snappyOut, err := runner.Run(snappyArgs[0], snappyArgs[1:]...)
	if err != nil {
		return nil, fmt.Errorf("snappy failed for %s branch %s: %w (output: %s)", repo.URL, branch, err, string(snappyOut))
	}

	// Run ampel verify against the collected data
	attestationPath := filepath.Join(cfg.OutputDir, sanitizeRepoName(repo.URL)+"-"+branch+".json")
	ampelArgs := constructAmpelVerifyCommand(cfg.PolicyPath, attestationPath, repo.URL)
	logger.Info("running ampel verify", "repo", repo.URL, "branch", branch)
	ampelOut, err := runner.Run(ampelArgs[0], ampelArgs[1:]...)
	if err != nil {
		return nil, fmt.Errorf("ampel verify failed for %s branch %s: %w (output: %s)", repo.URL, branch, err, string(ampelOut))
	}

	return &RawScanResult{Output: ampelOut}, nil
}

// sanitizeRepoName converts a repository URL into a safe filename component.
func sanitizeRepoName(repoURL string) string {
	name := repoURL
	// Remove scheme
	for _, prefix := range []string{"https://", "http://"} {
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			name = name[len(prefix):]
			break
		}
	}
	// Replace path separators and dots
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
