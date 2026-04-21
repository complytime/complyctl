package server

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/cmd/ampel-plugin/config"
	"github.com/complytime/complyctl/cmd/ampel-plugin/convert"
	"github.com/complytime/complyctl/cmd/ampel-plugin/results"
	"github.com/complytime/complyctl/cmd/ampel-plugin/scan"
	"github.com/complytime/complyctl/cmd/ampel-plugin/targets"
	"github.com/complytime/complyctl/cmd/ampel-plugin/toolcheck"
	"github.com/complytime/complyctl/pkg/provider"
)

// ScanRunner is used by Scan to execute scan commands.
// It defaults to scan.ExecRunner{} and can be overridden for testing.
var ScanRunner scan.CommandRunner = scan.ExecRunner{}

// SkipToolCheck disables tool presence validation. Used in tests.
var SkipToolCheck bool

// safeBranchPattern matches valid git branch names.
var safeBranchPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

var _ provider.Provider = (*PluginServer)(nil)

// PluginServer implements the provider.Plugin interface for the AMPEL plugin.
type PluginServer struct{}

// New returns a new PluginServer.
func New() *PluginServer {
	return &PluginServer{}
}

// Describe returns the plugin metadata and health status.
func (s *PluginServer) Describe(_ context.Context, _ *provider.DescribeRequest) (*provider.DescribeResponse, error) {
	return &provider.DescribeResponse{
		Healthy:                 true,
		Version:                 "0.1.0",
		RequiredTargetVariables: []string{"url", "specs"},
	}, nil
}

// Generate matches requirement IDs from the assessment configurations against
// granular AMPEL policy files and merges the matched policies into a single
// bundle for scan.
func (s *PluginServer) Generate(_ context.Context, req *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	logger := hclog.Default()

	if len(req.Configuration) == 0 {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: "no assessment configurations provided",
		}, nil
	}

	if err := checkRequiredTools(logger); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	if err := config.EnsureDirectories(); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("directory setup failed: %v", err),
		}, nil
	}

	logger.Info("generating AMPEL policy")

	sourceDir := config.GranularPolicyDirPath()
	if customDir, ok := req.GlobalVariables["ampel_policy_dir"]; ok && customDir != "" {
		sourceDir = customDir
	}
	outputDir := config.GeneratedPolicyDirPath()

	granular, err := convert.LoadGranularPolicies(sourceDir)
	if err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("loading granular policies: %v", err),
		}, nil
	}

	matched, warnings := convert.MatchPolicies(req.Configuration, granular)
	for _, w := range warnings {
		logger.Warn(w)
	}

	if len(matched) == 0 {
		logger.Info("no matching policies found, skipping policy generation")
		return &provider.GenerateResponse{Success: true}, nil
	}

	bundle := convert.MergeToBundle(matched)
	if err := convert.WritePolicy(bundle, outputDir); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("writing AMPEL policy bundle: %v", err),
		}, nil
	}

	logger.Info("AMPEL policy bundle written", "path", outputDir, "policies", len(matched))
	return &provider.GenerateResponse{Success: true}, nil
}

// Scan invokes the AMPEL toolchain to scan target repositories and returns
// standardized assessment results.
func (s *PluginServer) Scan(_ context.Context, req *provider.ScanRequest) (*provider.ScanResponse, error) {
	logger := hclog.Default()

	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("no targets provided")
	}

	if err := checkRequiredTools(logger); err != nil {
		return nil, err
	}

	if err := config.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("directory setup failed: %w", err)
	}

	logger.Info("scanning target repositories")

	generatedDir := config.GeneratedPolicyDirPath()
	resultsDir := config.ResultsDirPath()
	specDir := config.SpecDirPath()

	if err := scan.WriteSpecFiles(specDir); err != nil {
		return nil, fmt.Errorf("writing spec files: %w", err)
	}

	scanCfg := scan.ScanConfig{
		PolicyPath: filepath.Join(generatedDir, convert.PolicyFileName),
		OutputDir:  resultsDir,
		SpecDir:    specDir,
	}

	var repoResults []*results.PerRepoResult

	for _, target := range req.Targets {
		repoURL := target.Variables["url"]
		if repoURL == "" {
			return nil, fmt.Errorf("target %q: missing required variable 'url'", target.TargetID)
		}

		specsStr := target.Variables["specs"]
		if specsStr == "" {
			return nil, fmt.Errorf("target %q: missing required variable 'specs'", target.TargetID)
		}

		branchesStr := target.Variables["branches"]
		if branchesStr == "" {
			branchesStr = "main"
		}

		accessToken := target.Variables["access_token"]
		platformHint := target.Variables["platform"]

		branches := splitCSV(branchesStr)
		specs := splitCSV(specsStr)

		// Defense-in-depth: validate target variables on the plugin side
		if err := validateTargetVariables(repoURL, branches, specs, accessToken, target.TargetID); err != nil {
			return nil, err
		}

		// Validate and detect platform
		platform, _, _, err := targets.ParseRepoURL(repoURL, platformHint)
		if err != nil {
			return nil, fmt.Errorf("target %q: %w", target.TargetID, err)
		}

		repo := scan.RepoTarget{
			URL:         repoURL,
			AccessToken: accessToken,
			Platform:    platform,
		}

		for _, branch := range branches {
			for _, specRef := range specs {
				specPath := scan.ResolveSpecPath(specRef, scanCfg.SpecDir)
				logger.Info("scanning repository", "url", repoURL, "branch", branch, "spec", specRef)

				rawResult, err := scan.ScanRepository(repo, branch, specPath, scanCfg, ScanRunner)
				if err != nil {
					logger.Error("scan failed", "repo", repoURL, "branch", branch, "spec", specRef, "error", err)
					errResult := &results.PerRepoResult{
						Repository: repoURL,
						Branch:     branch,
						Status:     "error",
						Error:      err.Error(),
					}
					repoResults = append(repoResults, errResult)
					if writeErr := results.WritePerRepoResult(errResult, resultsDir); writeErr != nil {
						logger.Error("failed to write error result", "error", writeErr)
					}
					continue
				}

				parsed, err := results.ParseAmpelOutput(rawResult.Output, repoURL, branch)
				if err != nil {
					logger.Error("failed to parse scan output", "repo", repoURL, "error", err)
					errResult := &results.PerRepoResult{
						Repository: repoURL,
						Branch:     branch,
						Status:     "error",
						Error:      err.Error(),
					}
					repoResults = append(repoResults, errResult)
					if writeErr := results.WritePerRepoResult(errResult, resultsDir); writeErr != nil {
						logger.Error("failed to write error result", "error", writeErr)
					}
					continue
				}

				repoResults = append(repoResults, parsed)
			}
		}
	}

	scanResponse := results.ToScanResponse(repoResults)
	logger.Info("scan complete", "repositories_scanned", len(repoResults))
	return scanResponse, nil
}

// splitCSV splits a comma-separated string into trimmed, non-empty parts.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// validateTargetVariables performs defense-in-depth validation of target
// variable values received from the CLI. This catches issues even if the
// CLI validation was bypassed.
func validateTargetVariables(repoURL string, branches, specs []string, accessToken, targetID string) error {
	prefix := fmt.Sprintf("target %q", targetID)

	// URL: must be HTTPS, valid structure
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("%s: invalid url %q: %w", prefix, repoURL, err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%s: url %q must use HTTPS scheme", prefix, repoURL)
	}

	// Branches: safe characters, no path traversal
	for _, branch := range branches {
		if !safeBranchPattern.MatchString(branch) {
			return fmt.Errorf("%s: branch name contains invalid characters: %q", prefix, branch)
		}
		if strings.Contains(branch, "..") {
			return fmt.Errorf("%s: branch name contains path traversal: %q", prefix, branch)
		}
	}

	// Specs: non-empty, no path traversal
	for _, spec := range specs {
		if spec == "" {
			return fmt.Errorf("%s: spec cannot be empty", prefix)
		}
		if strings.Contains(spec, "..") {
			return fmt.Errorf("%s: spec contains path traversal: %q", prefix, spec)
		}
	}

	// AccessToken: reject newlines and null bytes
	if accessToken != "" {
		if strings.ContainsAny(accessToken, "\n\r\x00") {
			return fmt.Errorf("%s: access_token contains invalid characters (newline or null byte)", prefix)
		}
	}

	return nil
}

// checkRequiredTools validates that all required AMPEL tools are on PATH.
func checkRequiredTools(logger hclog.Logger) error {
	if SkipToolCheck {
		return nil
	}
	missing, err := toolcheck.CheckTools()
	if err != nil {
		return fmt.Errorf("checking required tools: %w", err)
	}
	if len(missing) > 0 {
		logger.Warn("required tools missing", "tools", missing)
		return toolcheck.FormatMissingToolsError(missing)
	}
	return nil
}
