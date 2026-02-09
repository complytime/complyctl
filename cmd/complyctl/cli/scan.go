// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/oscal-compass/compliance-to-policy-go/v2/framework"
	"github.com/oscal-compass/compliance-to-policy-go/v2/framework/actions"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/oscal-compass/oscal-sdk-go/settings"
	"github.com/oscal-compass/oscal-sdk-go/validation"
	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/cmd/complyctl/option"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/terminal"
)

const assessmentResultsLocationJson = "assessment-results.json"
const assessmentResultsLocationMd = "assessment-results.md"

// scanOptions defined options for the scan subcommand.
type scanOptions struct {
	*option.Common
	complyTimeOpts   *option.ComplyTime
	withPluginConfig string
}

// scanCmd creates a new cobra.Command for the version subcommand.
func scanCmd(common *option.Common) *cobra.Command {
	scanOpts := &scanOptions{
		Common:         common,
		complyTimeOpts: &option.ComplyTime{},
	}
	cmd := &cobra.Command{
		Use:          "scan [flags]",
		Short:        "Scan environment with assessment plan",
		Example:      "complyctl scan",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Ensure user workspace exists before proceeding
			return complytime.EnsureUserWorkspace(scanOpts.complyTimeOpts.UserWorkspace)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runScan(cmd, scanOpts)
		},
	}
	cmd.Flags().StringVarP(&scanOpts.withPluginConfig, "plugin-config", "c", "", "Directory where user customized plugin manifests are located")
	cmd.Flags().BoolP("with-md", "m", false, "If true, assessment-result markdown will be generated")
	scanOpts.complyTimeOpts.BindFlags(cmd.Flags())
	return cmd
}

func runScan(cmd *cobra.Command, opts *scanOptions) error {
	validator := validation.NewSchemaValidator()
	// Load settings from assessment plan
	ap, apCleanedPath, err := loadPlan(opts.complyTimeOpts, validator)
	if err != nil {
		return err
	}

	inputContext, err := complytime.ActionsContextFromPlan(ap)
	if err != nil {
		return err
	}

	// Create the application directory if it does not exist
	appDir, err := complytime.NewApplicationDirectory(true, logger)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("Using application directory: %s", appDir.AppDir()))

	cfg, err := complytime.Config(appDir)
	if err != nil {
		return err
	}

	// set config logger to CLI charm logger
	cfg.Logger = logger

	manager, err := framework.NewPluginManager(cfg)
	if err != nil {
		return fmt.Errorf("error initializing plugin manager: %w", err)
	}

	// Determine what profile to load from framework information captured
	// from state (assessment plan). This is required to populate complyTime required plugin options.
	frameworkProp, valid := extensions.GetTrestleProp(extensions.FrameworkProp, *ap.Metadata.Props)
	if !valid {
		return fmt.Errorf("error reading framework property from assessment plan")
	}
	opts.complyTimeOpts.FrameworkID = frameworkProp.Value
	logger.Debug(fmt.Sprintf("Framework property was successfully read from the assessment plan: %v.", frameworkProp))

	pluginOptions := opts.complyTimeOpts.ToPluginOptions()
	pluginOptions.UserConfigRoot = opts.withPluginConfig
	plugins, cleanup, err := complytime.Plugins(manager, inputContext, pluginOptions, logger)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return fmt.Errorf("errors launching plugins: %w", err)
	}
	logger.Info(fmt.Sprintf("Successfully loaded %v plugin(s).", len(plugins)))

	logger.Info("Scanning (this may take some time depending on the system and controls)...")
	stopSpinner := make(chan int)
	go terminal.ShowSpinner(stopSpinner)

	allResults, err := actions.AggregateResults(cmd.Context(), inputContext, plugins)
	stopSpinner <- 1

	if err != nil {
		return err
	}
	logger.Info("Scan completed successfully.")

	// Collect results in a single report
	planHref := fmt.Sprintf("file://%s", apCleanedPath)
	assessmentResults, err := actions.Report(cmd.Context(), inputContext, planHref, *ap, allResults)
	if err != nil {
		return err
	}
	arJsonPath := filepath.Join(opts.complyTimeOpts.UserWorkspace, assessmentResultsLocationJson)
	err = complytime.WriteAssessmentResults(assessmentResults, arJsonPath)
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("The assessment results in JSON were successfully written to %v.", arJsonPath))

	outputFlag, _ := cmd.Flags().GetBool("with-md")
	if outputFlag {
		var profileHref string
		compDefs, err := complytime.FindComponentDefinitions(appDir.BundleDir(), validator)
		if err != nil {
			return err
		}
		for _, compDef := range compDefs {
			if compDef.Components == nil {
				continue
			}
			for _, component := range *compDef.Components {
				if component.ControlImplementations == nil {
					continue
				}
				for _, implementation := range *component.ControlImplementations {
					frameworkShortName, found := settings.GetFrameworkShortName(implementation)
					// If the framework property value match the assessment plan framework property values
					// this is the correct control source.
					if found && frameworkShortName == frameworkProp.Value {
						profileHref = implementation.Source
						break
					}
				}
				if profileHref != "" {
					break
				}
			}
		}

		profile, err := complytime.LoadProfile(appDir, profileHref, validator)
		if err != nil {
			return err
		}

		if len(profile.Imports) != 1 {
			return errors.New("profile imports must be one")
		}
		catalog, err := complytime.LoadCatalogSource(appDir, profile.Imports[0].Href, validator)
		if err != nil {
			return err
		}
		arMarkdownPath := filepath.Join(opts.complyTimeOpts.UserWorkspace, assessmentResultsLocationMd)

		posture := framework.NewPosture(assessmentResults, catalog, ap, logger)
		assessmentResultsMd, err := posture.Generate(arMarkdownPath)
		if err != nil {
			return err
		}
		err = os.WriteFile(arMarkdownPath, assessmentResultsMd, 0600)
		if err != nil {
			return err
		}
		logger.Info(fmt.Sprintf("The assessment results in markdown were successfully written to %v.", arMarkdownPath))
	} else {
		logger.Info("No assessment result in markdown will be generated.")
	}
	return nil
}
