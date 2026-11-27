// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/evidencecleanup"
	"github.com/complytime/complyctl/pkg/log"
)

const cleanAgeFlag = "older-than"

const cleanConsequences = `Expiry irreversibly deletes managed reports and their supporting evaluations together.
Whole-group preflight is not a transaction: unexpected I/O failures can leave partial
deletion; no rollback is promised. Evidence is removed before reports.
Stop older hosts and direct provider calls before cleanup; host locks do not stop them.
Supported: Linux (ext4, XFS, Btrfs, tmpfs) and macOS (APFS), amd64 and arm64.
Other filesystems or unavailable coordination cause refusal before deletion.
`

type cleanClock func() time.Time

func (c cleanClock) Now() time.Time { return c() }

type cleanOptions struct {
	*Common
	olderThan string
	dryRun    bool
	clock     evidencecleanup.Clock
	// Keep fault injection at the service boundary (dependency inversion), not in
	// public deletion options. Inventory, grouping and removal remain library-owned.
	runCleanup func(evidencecleanup.Options) (*evidencecleanup.Result, error)
}

func cleanCmd(common *Common) *cobra.Command {
	return cleanCmdWithOptions(&cleanOptions{
		Common: common, clock: cleanClock(time.Now), runCleanup: evidencecleanup.Run,
	})
}

// Keep dependencies instance-scoped (dependency inversion); tests can control the
// clock without replacing the command hooks, cleanup service, or workspace lock.
func cleanCmdWithOptions(o *cleanOptions) *cobra.Command {
	common := o.Common
	cmd := &cobra.Command{
		Use:   "clean --older-than <days>d [--dry-run]",
		Short: "Expire workspace reports and their supporting evidence",
		Long: "Expire managed report/evaluation groups beneath .complytime in the selected workspace.\n" +
			"Age uses evaluation collection-start time, strictly before the cutoff.\n" +
			"No configuration, registry credentials, policy resolution or providers are required.\n\n" +
			cleanConsequences + "\nPreview first with --dry-run; it does not reserve a later cleanup.",
		Example: "complyctl clean --older-than 30d --dry-run\n" +
			"complyctl clean --workspace /path/to/workspace --older-than 30d",
		ValidArgsFunction: cobra.NoFileCompletions,
		// Cobra checks arguments before pre-run hooks. Select stream logging here
		// so the executable's Error call cannot mutate logs on validation failures.
		Args: func(cmd *cobra.Command, args []string) error {
			configureCleanDiagnostics(cmd, common.Debug)
			return cleanError(cobra.NoArgs(cmd, args))
		},
		// FR-007 forbids preview writes; FR-012 also preserves logs during deletion.
		// Override both root hooks rather than initializing or closing its file logger.
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			complytime.CheckLegacyDir(cmd.ErrOrStderr())
		},
		PersistentPostRun: func(_ *cobra.Command, _ []string) {},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cleanError(o.run(cmd.OutOrStdout()))
		},
	}
	// Flag parsing precedes even Args, so its errors need the same routing.
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		configureCleanDiagnostics(cmd, common.Debug)
		return cleanError(err)
	})
	cmd.Flags().StringVar(&o.olderThan, cleanAgeFlag, "",
		"Required positive whole days, e.g. 30d (24 hours/day; no leading zeroes)")
	cmd.Flags().BoolVar(&o.dryRun, "dry-run", false, "Preview complete groups without deleting them")
	if err := cmd.MarkFlagRequired(cleanAgeFlag); err != nil {
		logger.Error("Failed to mark cleanup age flag required", "error", err)
	}
	return cmd
}

func configureCleanDiagnostics(cmd *cobra.Command, debug bool) {
	logger = log.NewLogger(cmd.ErrOrStderr())
	if debug {
		logger.SetLevel(hclog.Debug)
	}
}

func (o *cleanOptions) run(out io.Writer) error {
	if _, err := evidencecleanup.ParseDays(o.olderThan); err != nil {
		return fmt.Errorf("validate --%s: %w", cleanAgeFlag, err)
	}
	workspace, err := o.ResolveWorkspace()
	if err != nil {
		return fmt.Errorf("resolve cleanup workspace: %w", err)
	}
	if o.dryRun {
		result, err := evidencecleanup.Preview(evidencecleanup.PreviewOptions{
			Workspace: workspace, OlderThan: o.olderThan, Clock: o.clock,
		})
		return errors.Join(err, writeCleanText(out, cleanPreviewText(result)))
	}
	result, err := o.runCleanup(evidencecleanup.Options{
		Workspace: workspace, OlderThan: o.olderThan, Clock: o.clock,
	})
	// A failed execution can still carry exact progress. Print it before returning
	// the original service error so the caller keeps its nonzero exit behavior.
	return errors.Join(err, writeCleanText(out, cleanExecutionText(result)))
}

func writeCleanText(out io.Writer, text string) error {
	if text == "" {
		return nil
	}
	if _, err := io.WriteString(out, text); err != nil {
		return fmt.Errorf("write cleanup results: %w", err)
	}
	return nil
}

func cleanHeader(workspace string, age evidencecleanup.AgeWindow) *strings.Builder {
	var text strings.Builder
	fmt.Fprintf(&text, "Workspace: %q\nCutoff (UTC, exclusive): %s\n%s",
		workspace, age.Cutoff().Format(time.RFC3339Nano), cleanConsequences)
	return &text
}

func cleanPreviewText(result *evidencecleanup.PreviewResult) string {
	if result == nil {
		return ""
	}
	text := cleanHeader(result.Workspace, result.Age)
	fmt.Fprintln(text, "Preview only; a later cleanup independently repeats discovery and preflight.")
	for index, group := range result.Groups {
		fmt.Fprintf(text, "Group %d:\n", index+1)
		cleanBlockerText(text, group.Blockers)
		for _, member := range group.Members {
			fmt.Fprintf(text, "  %s %s %q: %s\n", member.Status, member.Kind,
				member.Path, cleanDiagnosticText(strings.Join(member.Reasons, "; ")))
		}
	}
	cleanExclusionText(text, result.Exclusions)
	totals := result.Totals
	fmt.Fprintf(text, "Totals: groups would-delete=%d retained=%d; "+
		"members would-delete=%d retained=%d; excluded=%d\n",
		totals.WouldDeleteGroups, totals.RetainedGroups, totals.WouldDeleteMembers,
		totals.RetainedMembers, totals.ExcludedEntries)
	return text.String()
}

func cleanExecutionText(result *evidencecleanup.Result) string {
	if result == nil {
		return ""
	}
	text := cleanHeader(result.Workspace, result.Age)
	for index, group := range result.Groups {
		fmt.Fprintf(text, "Group %d:\n", index+1)
		cleanBlockerText(text, group.Blockers)
		if group.PartialLoss {
			fmt.Fprintln(text, "  Partial deletion; remaining members retained; no rollback.")
		}
		if group.Failure != "" {
			fmt.Fprintf(text, "  Group failure: %s\n", cleanDiagnosticText(group.Failure))
		}
		for _, member := range group.Members {
			cleanMemberText(text, member)
		}
	}
	cleanExclusionText(text, result.Exclusions)
	totals := result.Totals
	fmt.Fprintf(text, "Totals: groups deleted=%d retained=%d failed=%d; "+
		"members deleted=%d retained=%d partially-deleted=%d; excluded=%d\n",
		totals.DeletedGroups, totals.RetainedGroups, totals.FailedGroups,
		totals.DeletedMembers, totals.RetainedMembers, totals.PartialMembers, totals.ExcludedEntries)
	return text.String()
}

func cleanMemberText(text *strings.Builder, member evidencecleanup.MemberResult) {
	fmt.Fprintf(text, "  %s %s %q: %s\n", member.Status, member.Kind,
		member.Path, cleanDiagnosticText(strings.Join(member.Reasons, "; ")))
	for _, path := range member.RemovedPaths {
		fmt.Fprintf(text, "    removed %q\n", path)
	}
	if member.Failure != "" {
		fmt.Fprintf(text, "    Failure: %s\n", cleanDiagnosticText(member.Failure))
	}
}

func cleanBlockerText(text *strings.Builder, blockers []evidencecleanup.GroupBlocker) {
	for _, blocker := range blockers {
		fmt.Fprintf(text, "  Blocker %q: %s; entire group retained\n",
			blocker.Path, cleanDiagnosticText(blocker.Reason))
	}
}

func cleanExclusionText(text *strings.Builder, exclusions []evidencecleanup.DiscoveryExclusion) {
	for _, excluded := range exclusions {
		fmt.Fprintf(text, "Excluded %q: %s\n", excluded.Path, cleanDiagnosticText(excluded.Reason))
	}
}

func cleanDiagnosticText(value string) string {
	// Diagnostics can include report-supplied paths and operating-system errors.
	// Keep their controls visible without letting them alter the terminal or rows.
	var safe strings.Builder
	for _, character := range value {
		if unicode.IsPrint(character) {
			safe.WriteRune(character)
			continue
		}
		quoted := strconv.QuoteRuneToASCII(character)
		safe.WriteString(quoted[1 : len(quoted)-1])
	}
	return safe.String()
}

// Sanitize at the command boundary because parser errors can contain raw report
// excerpts. Keep the underlying error available for errors.Is and errors.As.
type cleanCommandError struct{ cause error }

func (e cleanCommandError) Error() string { return cleanDiagnosticText(e.cause.Error()) }

func (e cleanCommandError) Unwrap() error { return e.cause }

func cleanError(err error) error {
	if err == nil {
		return nil
	}
	return cleanCommandError{cause: err}
}
