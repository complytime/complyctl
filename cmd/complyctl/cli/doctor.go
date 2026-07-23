// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/doctor"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/internal/registry"
)

// diagnosticOutput is the top-level JSON structure for --format json.
type diagnosticOutput struct {
	Checks          []checkOutput     `json:"checks"`
	Summary         diagnosticSummary `json:"summary"`
	BlockingFailure bool              `json:"blocking_failure"`
}

// checkOutput is one entry in the JSON checks array.
type checkOutput struct {
	Name     string             `json:"name"`
	Label    string             `json:"label,omitempty"`
	Status   doctor.CheckStatus `json:"status"`
	Message  string             `json:"message"`
	Blocking bool               `json:"blocking"`
	Group    doctor.CheckGroup  `json:"group,omitempty"`
	Children []checkOutput      `json:"children,omitempty"`
}

// diagnosticSummary holds the counts for the JSON summary field.
type diagnosticSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Warnings int `json:"warnings"`
}

func doctorCmd(common *Common) *cobra.Command {
	var verbose bool
	var format string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run pre-flight diagnostics on the workspace",
		Long: `Run pre-flight diagnostics on the workspace.

Checks include provider discovery, policy cache integrity, workspace
configuration validation, and complypack availability. When complypacks are
configured, the doctor verifies that each referenced complypack is cached
and reports missing entries.`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(_ *cobra.Command, _ []string) error {
			baseDir, err := common.ResolveWorkspace()
			if err != nil {
				return err
			}
			resolvedFormat, err := resolveFormat(format)
			if err != nil {
				return err
			}
			return runDoctor(baseDir, verbose, resolvedFormat)
		},
	}
	cmd.Flags().BoolVar(&verbose, "verbose", false, "expand per-provider variable detail")
	cmd.Flags().StringVarP(&format, "format", "f", "", "output format: text, json")
	if err := cmd.RegisterFlagCompletionFunc("format", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{complytime.OutputFormatText, complytime.OutputFormatJSON}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		logger.Error("Failed to register format completion", "error", err)
	}
	return cmd
}

// registryVersionResolver adapts registry.Client to doctor.VersionResolver.
// See R55: specs/001-gemara-native-workflow/spec.md
type registryVersionResolver struct {
	timeout time.Duration
}

func (r *registryVersionResolver) ResolveLatestVersion(registryURL, repository string) (string, error) {
	return r.resolve(registryURL, repository, "")
}

func (r *registryVersionResolver) ResolveVersion(registryURL, repository, version string) (string, error) {
	return r.resolve(registryURL, repository, version)
}

func (r *registryVersionResolver) resolve(registryURL, repository, version string) (string, error) {
	credFunc, err := registry.NewCredentialFunc()
	if err != nil {
		credFunc = nil
	}
	client := registry.NewClient(registryURL, credFunc)
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	lookup := repository
	if version != "" {
		lookup = repository + ":" + version
	}
	_, resolved, err := client.DefinitionVersion(ctx, lookup)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

// See FR-039, R44, R51, R52, R55: specs/001-gemara-native-workflow/spec.md
func runDoctor(baseDir string, verbose bool, format string) error {
	providerDir, err := complytime.ResolveProviderDir()
	if err != nil {
		return fmt.Errorf("failed to resolve provider directory: %w", err)
	}

	cacheDir, err := complytime.ResolveCacheDir()
	if err != nil {
		return fmt.Errorf("failed to resolve cache directory: %w", err)
	}

	dataDir, err := complytime.ResolveDataDir()
	if err != nil {
		return fmt.Errorf("failed to resolve data directory: %w", err)
	}

	ws := complytime.NewWorkspace(baseDir)
	configPath := ws.Path()
	var cfg *complytime.WorkspaceConfig

	if loadErr := ws.Load(); loadErr == nil {
		cfg = ws.Config()
	}

	var resolver doctor.PolicyGraphResolver
	cacheMgr := cache.NewCache(cacheDir)
	loader := policy.NewLoader(cacheMgr)
	resolver = policy.NewResolver(loader)

	versionResolver := &registryVersionResolver{timeout: 5 * time.Second}

	results := doctor.Run(cfg, configPath, providerDir, cacheDir, dataDir, resolver, versionResolver, verbose, logger)
	return printDiagnostics(results, format)
}

// statusLabel maps a CheckStatus to a grep-stable bracketed label.
func statusLabel(s doctor.CheckStatus) string {
	switch s {
	case doctor.StatusPass:
		return "[PASS]"
	case doctor.StatusFail:
		return "[FAIL]"
	case doctor.StatusWarn:
		return "[WARN]"
	default:
		return "[UNKNOWN]"
	}
}

// resolveFormat returns the effective output format. An explicit --format flag
// value takes precedence; when the flag is empty, NO_COLOR triggers text mode.
// Returns an error for unrecognised flag values.
func resolveFormat(flagValue string) (string, error) {
	if flagValue != "" {
		switch flagValue {
		case complytime.OutputFormatText, complytime.OutputFormatJSON:
			return flagValue, nil
		default:
			return "", fmt.Errorf("invalid format %q: must be one of %s, %s",
				flagValue, complytime.OutputFormatText, complytime.OutputFormatJSON)
		}
	}
	if os.Getenv("NO_COLOR") != "" {
		return complytime.OutputFormatText, nil
	}
	return "", nil
}

// resultLabel returns the display label for a CheckResult. Uses Label
// when set, falls back to Name when Label is empty. No string parsing
// of Name occurs — display text is set at the source (D10).
func resultLabel(r doctor.CheckResult) string {
	if r.Label != "" {
		return r.Label
	}
	return r.Name
}

// statusEmoji maps a CheckStatus to its emoji indicator for human output.
func statusEmoji(s doctor.CheckStatus) string {
	switch s {
	case doctor.StatusPass:
		return complytime.StatusPassed
	case doctor.StatusFail:
		return complytime.StatusFailed
	case doctor.StatusWarn:
		return complytime.StatusError
	default:
		return "?"
	}
}

func countStatus(s doctor.CheckStatus, pass, fail, warn *int) {
	switch s {
	case doctor.StatusPass:
		*pass++
	case doctor.StatusFail:
		*fail++
	case doctor.StatusWarn:
		*warn++
	}
}

// printDiagnosticsHuman renders results with emoji status indicators followed
// by a grep-stable [STATUS] label. This is the default interactive mode.
func printDiagnosticsHuman(results []doctor.CheckResult, w io.Writer) error {
	fmt.Fprintln(w, "Running workspace diagnostics...")
	fmt.Fprintln(w)

	grouped := make(map[doctor.CheckGroup][]doctor.CheckResult)
	for _, r := range results {
		grouped[r.Group] = append(grouped[r.Group], r)
	}

	var passCount, failCount, warnCount int
	hasBlockingFailure := false
	firstSection := true

	for _, group := range doctor.GroupOrder() {
		checks, ok := grouped[group]
		if !ok {
			continue
		}

		if !firstSection {
			fmt.Fprintln(w)
		}
		firstSection = false
		fmt.Fprintln(w, string(group))

		for _, r := range checks {
			emoji := statusEmoji(r.Status)
			countStatus(r.Status, &passCount, &failCount, &warnCount)
			fmt.Fprintf(w, "  %s %s %s: %s\n", emoji, statusLabel(r.Status), resultLabel(r), r.Message)
			if r.Blocking && r.Status == doctor.StatusFail {
				hasBlockingFailure = true
			}

			for _, child := range r.Children {
				childEmoji := statusEmoji(child.Status)
				countStatus(child.Status, &passCount, &failCount, &warnCount)
				fmt.Fprintf(w, "      %s %s %s: %s\n", childEmoji, statusLabel(child.Status), resultLabel(child), child.Message)
				if child.Blocking && child.Status == doctor.StatusFail {
					hasBlockingFailure = true
				}

				// Grandchildren (verbose detail) are rendered but not
				// counted in the summary (D5 — stable count regardless
				// of --verbose).
				for _, gc := range child.Children {
					gcEmoji := statusEmoji(gc.Status)
					fmt.Fprintf(w, "          %s %s %s\n", gcEmoji, statusLabel(gc.Status), gc.Message)
				}
			}
		}
	}

	total := passCount + failCount + warnCount
	fmt.Fprintf(w, "\n%d checks: %d passed, %d failed, %d warnings\n", total, passCount, failCount, warnCount)

	if hasBlockingFailure {
		return fmt.Errorf("one or more blocking checks failed")
	}
	return nil
}

// printDiagnosticsText renders results with bracketed [STATUS] labels and no
// emoji. Used for --format text and when NO_COLOR is set.
func printDiagnosticsText(results []doctor.CheckResult, w io.Writer) error {
	fmt.Fprintln(w, "Running workspace diagnostics...")
	fmt.Fprintln(w)

	grouped := make(map[doctor.CheckGroup][]doctor.CheckResult)
	for _, r := range results {
		grouped[r.Group] = append(grouped[r.Group], r)
	}

	var passCount, failCount, warnCount int
	hasBlockingFailure := false
	firstSection := true

	for _, group := range doctor.GroupOrder() {
		checks, ok := grouped[group]
		if !ok {
			continue
		}

		if !firstSection {
			fmt.Fprintln(w)
		}
		firstSection = false
		fmt.Fprintln(w, string(group))

		for _, r := range checks {
			countStatus(r.Status, &passCount, &failCount, &warnCount)
			fmt.Fprintf(w, "  %s %s: %s\n", statusLabel(r.Status), resultLabel(r), r.Message)
			if r.Blocking && r.Status == doctor.StatusFail {
				hasBlockingFailure = true
			}

			for _, child := range r.Children {
				countStatus(child.Status, &passCount, &failCount, &warnCount)
				fmt.Fprintf(w, "      %s %s: %s\n", statusLabel(child.Status), resultLabel(child), child.Message)
				if child.Blocking && child.Status == doctor.StatusFail {
					hasBlockingFailure = true
				}

				for _, gc := range child.Children {
					fmt.Fprintf(w, "          %s %s\n", statusLabel(gc.Status), gc.Message)
				}
			}
		}
	}

	total := passCount + failCount + warnCount
	fmt.Fprintf(w, "\n%d checks: %d passed, %d failed, %d warnings\n", total, passCount, failCount, warnCount)

	if hasBlockingFailure {
		return fmt.Errorf("one or more blocking checks failed")
	}
	return nil
}

// convertResult converts a doctor.CheckResult to a checkOutput for JSON
// serialisation, recursively converting children.
func convertResult(r doctor.CheckResult) checkOutput {
	var children []checkOutput
	for _, child := range r.Children {
		children = append(children, convertResult(child))
	}
	return checkOutput{
		Name:     r.Name,
		Label:    r.Label,
		Status:   r.Status,
		Message:  r.Message,
		Blocking: r.Blocking,
		Group:    r.Group,
		Children: children,
	}
}

// printDiagnosticsJSON renders results as a single JSON object. No prose is
// written. Used for --format json.
func printDiagnosticsJSON(results []doctor.CheckResult, w io.Writer) error {
	checks := make([]checkOutput, 0, len(results))
	var passCount, failCount, warnCount int
	hasBlockingFailure := false

	for _, r := range results {
		countStatus(r.Status, &passCount, &failCount, &warnCount)
		if r.Blocking && r.Status == doctor.StatusFail {
			hasBlockingFailure = true
		}
		for _, child := range r.Children {
			countStatus(child.Status, &passCount, &failCount, &warnCount)
			if child.Blocking && child.Status == doctor.StatusFail {
				hasBlockingFailure = true
			}
		}
		checks = append(checks, convertResult(r))
	}

	out := diagnosticOutput{
		Checks: checks,
		Summary: diagnosticSummary{
			Total:    passCount + failCount + warnCount,
			Passed:   passCount,
			Failed:   failCount,
			Warnings: warnCount,
		},
		BlockingFailure: hasBlockingFailure,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encoding diagnostic output: %w", err)
	}

	if hasBlockingFailure {
		return fmt.Errorf("one or more blocking checks failed")
	}
	return nil
}

// printDiagnosticsTo dispatches to the appropriate renderer based on format,
// writing output to w. An empty format string selects the human renderer.
func printDiagnosticsTo(results []doctor.CheckResult, format string, w io.Writer) error {
	switch format {
	case complytime.OutputFormatText:
		return printDiagnosticsText(results, w)
	case complytime.OutputFormatJSON:
		return printDiagnosticsJSON(results, w)
	default:
		return printDiagnosticsHuman(results, w)
	}
}

// printDiagnostics dispatches to the appropriate renderer writing to os.Stdout.
func printDiagnostics(results []doctor.CheckResult, format string) error {
	return printDiagnosticsTo(results, format, os.Stdout)
}
