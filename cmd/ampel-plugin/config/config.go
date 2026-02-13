package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
)

const (
	// PluginDir is the subdirectory name for ampel artifacts within the workspace.
	PluginDir = "ampel"
	// DefaultPolicyDir is the default directory name for AMPEL policy files.
	DefaultPolicyDir = "policy"
	// DefaultResultsDir is the default directory name for per-repository result files.
	DefaultResultsDir = "results"
	// DefaultTargetsFile is the default filename for the target repository configuration.
	DefaultTargetsFile = "ampel-targets.yaml"
)

// Config holds the plugin configuration received from complyctl.
type Config struct {
	Workspace   string
	Profile     string
	PolicyDir   string
	ResultsDir  string
	TargetsFile string
}

// NewConfig returns a new Config with default values for optional fields.
func NewConfig() *Config {
	return &Config{
		PolicyDir:   DefaultPolicyDir,
		ResultsDir:  DefaultResultsDir,
		TargetsFile: DefaultTargetsFile,
	}
}

// LoadSettings populates the config from the map provided by complyctl
// via the plugin manifest. Required fields are validated, defaults
// are applied for optional fields, and all paths are resolved to
// absolute paths.
func (c *Config) LoadSettings(configMap map[string]string) error {
	workspace, ok := configMap["workspace"]
	if !ok || workspace == "" {
		return fmt.Errorf("missing required configuration: workspace")
	}
	c.Workspace = workspace

	profile, ok := configMap["profile"]
	if !ok || profile == "" {
		return fmt.Errorf("missing required configuration: profile")
	}
	c.Profile = profile

	if v, ok := configMap["policy_dir"]; ok && v != "" {
		c.PolicyDir = v
	}
	if v, ok := configMap["results_dir"]; ok && v != "" {
		c.ResultsDir = v
	}
	if v, ok := configMap["targets_file"]; ok && v != "" {
		c.TargetsFile = v
	}

	if err := c.createWorkspaceDirs(); err != nil {
		return fmt.Errorf("creating workspace directories: %w", err)
	}

	if err := c.ResolvePaths(); err != nil {
		return fmt.Errorf("resolving paths: %w", err)
	}

	return nil
}

// Validate checks that all required fields are populated.
func (c *Config) Validate() error {
	if c.Workspace == "" {
		return fmt.Errorf("workspace is required")
	}
	if c.Profile == "" {
		return fmt.Errorf("profile is required")
	}
	return nil
}

// createWorkspaceDirs creates the workspace subdirectory for ampel artifacts.
func (c *Config) createWorkspaceDirs() error {
	logger := hclog.Default()
	ampelDir := c.ampelDir()
	logger.Debug("creating workspace directory", "path", ampelDir)
	return os.MkdirAll(ampelDir, 0750)
}

// ampelDir returns the path to the ampel subdirectory within the workspace.
func (c *Config) ampelDir() string {
	return filepath.Join(c.Workspace, PluginDir)
}

// ResolvePaths resolves PolicyDir, ResultsDir, and TargetsFile to absolute
// paths. Relative paths are resolved against {Workspace}/ampel/. Directories
// for PolicyDir and ResultsDir are created if they do not exist.
func (c *Config) ResolvePaths() error {
	logger := hclog.Default()

	if !filepath.IsAbs(c.PolicyDir) {
		c.PolicyDir = filepath.Join(c.ampelDir(), c.PolicyDir)
	}
	if !filepath.IsAbs(c.ResultsDir) {
		c.ResultsDir = filepath.Join(c.ampelDir(), c.ResultsDir)
	}
	if !filepath.IsAbs(c.TargetsFile) {
		c.TargetsFile = filepath.Join(c.ampelDir(), c.TargetsFile)
	}

	logger.Debug("resolved paths", "policy_dir", c.PolicyDir, "results_dir", c.ResultsDir, "targets_file", c.TargetsFile)

	if err := os.MkdirAll(c.PolicyDir, 0750); err != nil {
		return fmt.Errorf("creating policy directory %s: %w", c.PolicyDir, err)
	}
	if err := os.MkdirAll(c.ResultsDir, 0750); err != nil {
		return fmt.Errorf("creating results directory %s: %w", c.ResultsDir, err)
	}

	return nil
}

// PolicyDirPath returns the resolved absolute path for the policy directory.
func (c *Config) PolicyDirPath() string {
	return c.PolicyDir
}

// ResultsDirPath returns the resolved absolute path for the results directory.
func (c *Config) ResultsDirPath() string {
	return c.ResultsDir
}

// TargetsFilePath returns the resolved absolute path for the targets file.
func (c *Config) TargetsFilePath() string {
	return c.TargetsFile
}
