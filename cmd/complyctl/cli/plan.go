// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/goccy/go-yaml"
	"github.com/oscal-compass/oscal-sdk-go/transformers"
	"github.com/oscal-compass/oscal-sdk-go/validation"
	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/cmd/complyctl/option"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/complytime/plan"
)

const assessmentPlanLocation = "assessment-plan.json"

// complytimeProfileLoader implements the ProfileLoader interface
type complytimeProfileLoader struct{}

func (c complytimeProfileLoader) LoadProfile(appDir plan.ApplicationDirectory, controlSource string, validator validation.Validator) (*oscalTypes.Profile, error) {
	// Type assertion to convert interface back to concrete type
	concreteAppDir, ok := appDir.(complytime.ApplicationDirectory)
	if !ok {
		return nil, fmt.Errorf("invalid application directory type")
	}
	return complytime.LoadProfile(concreteAppDir, controlSource, validator)
}

func (c complytimeProfileLoader) LoadCatalogSource(appDir plan.ApplicationDirectory, catalogSource string, validator validation.Validator) (*oscalTypes.Catalog, error) {
	// Type assertion to convert interface back to concrete type
	concreteAppDir, ok := appDir.(complytime.ApplicationDirectory)
	if !ok {
		return nil, fmt.Errorf("invalid application directory type")
	}
	return complytime.LoadCatalogSource(concreteAppDir, catalogSource, validator)
}

// PlanOptions defines options for the "plan" subcommand
type planOptions struct {
	*option.Common
	complyTimeOpts *option.ComplyTime

	// dryRun loads the defaults and prints the config to stdout
	dryRun bool

	// WithScopeConfig "config.yml" to customize the generated assessment plan
	withScopeConfig string

	// Out
	output string
}

var planExample = `
# The default behavior is to prepare a default assessment plan with all defined controls within the framework in scope.
complytime plan myframework

# To see the default contents of the assessment plan, run in dry-run mode.
complytime plan myframework --dry-run

# To customize the assessment plan and write to a file, run in dry-run mode with out.
complytime plan myframework --dry-run --out config.yml

# Alter the configuration and use it as input for plan customization.
complytime plan myframework --scope-config config.yml
`

// planCmd creates a new cobra.Command for the "plan" subcommand
func planCmd(common *option.Common) *cobra.Command {
	planOpts := &planOptions{
		Common:         common,
		complyTimeOpts: &option.ComplyTime{},
	}
	cmd := &cobra.Command{
		Use:     "plan [flags] id",
		Short:   "Generate a new assessment plan for a given compliance framework id.",
		Example: planExample,
		Args:    cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			completePlan(planOpts, args)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validatePlan(planOpts); err != nil {
				return err
			}
			return runPlan(cmd, planOpts)
		},
	}
	cmd.Flags().BoolVar(&planOpts.dryRun, "dry-run", false, "load the defaults and print the config to stdout")
	cmd.Flags().StringVarP(&planOpts.withScopeConfig, "scope-config", "s", "", "load config.yml to customize the generated assessment plan")
	cmd.Flags().StringVarP(&planOpts.output, "out", "o", "-", "path to output file. Use '-' for stdout. Default '-'.")
	planOpts.complyTimeOpts.BindFlags(cmd.Flags())
	return cmd
}

func completePlan(opts *planOptions, args []string) {
	if len(args) == 1 {
		opts.complyTimeOpts.FrameworkID = filepath.Clean(args[0])
	}
}

func validatePlan(opts *planOptions) error {
	if opts.output != "-" && !opts.dryRun {
		return errors.New("invalid command flags: \"--dry-run\" must be used with \"--out\"")
	}
	return nil
}

func runPlan(cmd *cobra.Command, opts *planOptions) error {
	// Create the application directory if it does not exist
	appDir, err := complytime.NewApplicationDirectory(true)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("Using application directory: %s", appDir.AppDir()))

	validator := validation.NewSchemaValidator()
	componentDefs, err := complytime.FindComponentDefinitions(appDir.BundleDir(), validator)
	if err != nil {
		return err
	}

	if opts.dryRun {
		// Write the plan configuration to stdout
		return planDryRun(opts.complyTimeOpts.FrameworkID, componentDefs, opts.output)
	}

	logger.Debug(fmt.Sprintf("Using bundle directory: %s for component definitions.", appDir.BundleDir()))
	assessmentPlan, err := transformers.ComponentDefinitionsToAssessmentPlan(cmd.Context(), componentDefs, opts.complyTimeOpts.FrameworkID)
	if err != nil {
		return err
	}

	if opts.withScopeConfig != "" {
		configBytes, err := os.ReadFile(filepath.Clean(opts.withScopeConfig))
		if err != nil {
			return fmt.Errorf("error reading assessment plan: %w", err)
		}
		assessmentScope := plan.AssessmentScope{}
		if err := yaml.Unmarshal(configBytes, &assessmentScope); err != nil {
			return fmt.Errorf("error unmarshaling assessment plan: %w", err)
		}
		assessmentScope.ApplyScope(assessmentPlan, logger)
	}

	filePath := filepath.Join(opts.complyTimeOpts.UserWorkspace, assessmentPlanLocation)
	cleanedPath := filepath.Clean(filePath)

	if err := plan.WritePlan(assessmentPlan, opts.complyTimeOpts.FrameworkID, cleanedPath); err != nil {
		return fmt.Errorf("error writing assessment plan to %s: %w", cleanedPath, err)
	}
	logger.Info(fmt.Sprintf("Assessment plan written to %s\n", cleanedPath))
	return nil
}

// loadPlan returns the loaded assessment plan and path from the workspace.
func loadPlan(opts *option.ComplyTime, validator validation.Validator) (*oscalTypes.AssessmentPlan, string, error) {
	apPath := filepath.Join(opts.UserWorkspace, assessmentPlanLocation)
	apCleanedPath := filepath.Clean(apPath)
	assessmentPlan, err := plan.ReadPlan(apCleanedPath, validator)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", fmt.Errorf("error: assessment plan does not exist in workspace %s: %w\n\nDid you run the plan command?",
				opts.UserWorkspace,
				err)
		}
		return nil, "", err
	}
	return assessmentPlan, apCleanedPath, nil
}

// planDryRun leverages the AssessmentScope structure to populate tailoring config.
// The config is written to stdout.
func planDryRun(frameworkId string, cds []oscalTypes.ComponentDefinition, output string) error {
	// Create the application directory and validator to retrieve control titles
	appDir, err := complytime.NewApplicationDirectory(true)
	if err != nil {
		return fmt.Errorf("failed to initialize application directory: %w", err)
	}

	validator := validation.NewSchemaValidator()
	profileLoader := complytimeProfileLoader{}

	// Retrieve control titles
	scope, err := plan.NewAssessmentScopeFromCDsWithTitles(frameworkId, appDir, validator, profileLoader, cds...)
	if err != nil {
		return fmt.Errorf("error creating assessment scope for %s: %w", frameworkId, err)
	}

	data, err := yaml.Marshal(&scope)
	if err != nil {
		return fmt.Errorf("error marshalling yaml content: %v", err)
	}

	if output == "-" {
		fmt.Fprintln(os.Stdout, string(data))
	} else {
		return os.WriteFile(output, data, 0600)
	}
	return nil
}
