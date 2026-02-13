package server

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"

	"github.com/complytime/complyctl/cmd/ampel-plugin/config"
	"github.com/complytime/complyctl/cmd/ampel-plugin/convert"
	"github.com/complytime/complyctl/cmd/ampel-plugin/results"
	"github.com/complytime/complyctl/cmd/ampel-plugin/scan"
	"github.com/complytime/complyctl/cmd/ampel-plugin/targets"
	"github.com/complytime/complyctl/cmd/ampel-plugin/toolcheck"
)

// ScanRunner is used by GetResults to execute scan commands.
// It defaults to scan.ExecRunner{} and can be overridden for testing.
var ScanRunner scan.CommandRunner = scan.ExecRunner{}

// SkipToolCheck disables tool presence validation. Used in tests.
var SkipToolCheck bool

var _ policy.Provider = (*PluginServer)(nil)

// PluginServer implements the policy.Provider interface for the AMPEL plugin.
type PluginServer struct {
	Config *config.Config
}

// New returns a new PluginServer with default configuration.
func New() PluginServer {
	return PluginServer{
		Config: config.NewConfig(),
	}
}

// Configure receives the plugin settings from complyctl and initializes
// the plugin configuration.
func (s PluginServer) Configure(_ context.Context, configMap map[string]string) error {
	return s.Config.LoadSettings(configMap)
}

// Generate translates OSCAL assessment plan rules into AMPEL policy files.
func (s PluginServer) Generate(_ context.Context, p policy.Policy) error {
	logger := hclog.Default()

	if err := checkRequiredTools(logger); err != nil {
		return err
	}

	logger.Info("generating AMPEL policy")

	cfg := convert.ConvertConfig{Profile: s.Config.Profile}
	ampelPolicy, err := convert.PolicyToAmpel(p, cfg)
	if err != nil {
		return fmt.Errorf("converting policy to AMPEL format: %w", err)
	}

	if ampelPolicy == nil {
		logger.Info("no applicable rules found, skipping policy generation")
		return nil
	}

	policyDir := s.Config.PolicyDirPath()
	if err := convert.WritePolicy(ampelPolicy, policyDir); err != nil {
		return fmt.Errorf("writing AMPEL policy: %w", err)
	}

	logger.Info("AMPEL policy written", "path", policyDir)
	return nil
}

// GetResults invokes the AMPEL toolchain to scan target repositories and
// returns standardized assessment results.
func (s PluginServer) GetResults(_ context.Context, p policy.Policy) (policy.PVPResult, error) {
	logger := hclog.Default()

	if err := checkRequiredTools(logger); err != nil {
		return policy.PVPResult{}, err
	}

	logger.Info("scanning target repositories")

	targetsPath := s.Config.TargetsFilePath()
	targetConfig, warnings, err := targets.LoadTargets(targetsPath)
	if err != nil {
		return policy.PVPResult{}, fmt.Errorf("loading targets: %w", err)
	}
	for _, w := range warnings {
		logger.Warn(w)
	}

	policyDir := s.Config.PolicyDirPath()
	resultsDir := s.Config.ResultsDirPath()
	scanCfg := scan.ScanConfig{
		PolicyPath: policyDir + "/" + convert.PolicyFileName,
		OutputDir:  resultsDir,
	}

	var repoResults []*results.PerRepoResult

	for _, repo := range targetConfig.Repositories {
		for _, branch := range repo.Branches {
			logger.Info("scanning repository", "url", repo.URL, "branch", branch)

			rawResult, err := scan.ScanRepository(repo, branch, scanCfg, ScanRunner)
			if err != nil {
				logger.Error("scan failed", "repo", repo.URL, "branch", branch, "error", err)
				errResult := &results.PerRepoResult{
					Repository: repo.URL,
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

			parsed, err := results.ParseAmpelOutput(rawResult.Output, repo.URL, branch)
			if err != nil {
				logger.Error("failed to parse scan output", "repo", repo.URL, "error", err)
				errResult := &results.PerRepoResult{
					Repository: repo.URL,
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
			if err := results.WritePerRepoResult(parsed, resultsDir); err != nil {
				logger.Error("failed to write result", "repo", repo.URL, "error", err)
			}
		}
	}

	pvpResult := results.ToPVPResult(repoResults)
	logger.Info("scan complete", "repositories_scanned", len(repoResults))
	return pvpResult, nil
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
