// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/internal/terminal"
)

type listOptions struct {
	*Common
	policyID string
	cacheDir string
}

func listCmd(common *Common) *cobra.Command {
	o := &listOptions{
		Common: common,
	}
	cmd := &cobra.Command{
		Use:               "list [flags]",
		Short:             "List cached Gemara policies",
		SilenceUsage:      true,
		Example:           "complyctl list\n  complyctl list --policy-id nist-800-53-r5",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := o.validate(); err != nil {
				return err
			}
			if err := o.complete(); err != nil {
				return err
			}
			return o.run(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&o.policyID, "policy-id", "", "Filter by policy ID")
	if err := cmd.RegisterFlagCompletionFunc("policy-id", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		logger.Error("Failed to register policy-id completion", "error", err)
	}
	return cmd
}

func (o *listOptions) validate() error {
	return nil
}

func (o *listOptions) complete() error {
	var err error
	o.cacheDir, err = complytime.ResolveCacheDir()
	if err != nil {
		return fmt.Errorf("failed to resolve cache directory: %w", err)
	}
	return nil
}

func (o *listOptions) run(_ context.Context) error {
	baseDir, err := o.ResolveWorkspace()
	if err != nil {
		return err
	}
	ws := complytime.NewWorkspace(baseDir)
	if err := ws.LoadAndValidate(); err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	cfg := ws.Config()

	cacheMgr := cache.NewCache(o.cacheDir)
	loader := policy.NewLoader(cacheMgr)

	state, err := cache.LoadState(o.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to load cache state: %w", err)
	}

	var rows [][]string
	for _, p := range cfg.Policies {
		eid := p.EffectiveID()
		if o.policyID != "" && eid != o.policyID {
			continue
		}

		ref, err := complytime.ParsePolicyRef(p.URL)
		if err != nil {
			return fmt.Errorf("invalid policy reference %q: %w", p.URL, err)
		}
		versions, _ := loader.GetCachedVersions(ref.Repository)

		var versionStr string
		if len(versions) > 0 {
			sort.Strings(versions)
			versionStr = strings.Join(versions, ", ")
		} else {
			versionStr = "-"
		}

		evaluatorStr, controlsStr := policyMetadataFields(
			state, ref.Repository,
		)
		digestStr := policyDigestField(state, ref.Repository)
		verifiedStr := verificationStatus(state, ref.Repository)
		rows = append(rows, []string{
			eid, versionStr, evaluatorStr, controlsStr,
			digestStr, verifiedStr,
		})
	}

	return printGemaraPolicyTable(o.Out, rows)
}

// policyMetadataFields returns the EVALUATOR and CONTROLS column
// values for a policy from the cache state. Returns "-" for both
// when the policy has no cached state or metadata has not been
// populated (pre-upgrade cache without metadata fields).
func policyMetadataFields(
	state *cache.State, repository string,
) (evaluator, controls string) {
	ps, ok := state.GetPolicyState(repository)
	if !ok {
		return "-", "-"
	}
	if !hasMetadata(ps) {
		return "-", "-"
	}
	evaluator = ps.PolicyEvaluator
	if evaluator == "" {
		evaluator = "-"
	}
	controls = strconv.Itoa(ps.ControlCount)
	return evaluator, controls
}

// hasMetadata returns true when any display-oriented metadata field
// on the PolicyState has been populated. This distinguishes a
// pre-upgrade cache (all zero values) from a policy that was synced
// with metadata extraction.
func hasMetadata(ps cache.PolicyState) bool {
	return ps.PolicyTitle != "" ||
		ps.PolicyEvaluator != "" ||
		ps.ControlCount > 0 ||
		ps.AssessmentCount > 0
}

// policyDigestField returns the abbreviated digest for a policy from the
// cache state. Returns "-" when the policy has no cached state.
func policyDigestField(state *cache.State, repository string) string {
	ps, ok := state.GetPolicyState(repository)
	if !ok {
		return "-"
	}
	return abbreviateDigest(ps.Digest)
}

// abbreviateDigest shortens an OCI digest to the algorithm prefix
// plus the first 12 hex characters (e.g., "sha256:a1b2c3d4e5f6").
// Returns "-" for empty digests.
func abbreviateDigest(dgst string) string {
	if dgst == "" {
		return "-"
	}
	parts := strings.SplitN(dgst, ":", 2)
	if len(parts) != 2 {
		return dgst
	}
	hex := parts[1]
	if len(hex) > 12 {
		hex = hex[:12]
	}
	return parts[0] + ":" + hex
}

func printGemaraPolicyTable(w io.Writer, rows [][]string) error {
	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	headers := []string{
		"POLICY ID", "VERSION", "EVALUATOR", "CONTROLS",
		"DIGEST", "VERIFIED",
	}
	terminal.ShowPlainTable(w, headers, rows)
	return nil
}

// verificationStatus returns "Yes", "No", or "-" based on the PolicyState
// verification status for a given policy repository.
func verificationStatus(state *cache.State, repository string) string {
	ps, exists := state.GetPolicyState(repository)
	if !exists {
		return "-"
	}
	if ps.Verified {
		return "Yes"
	}
	return "No"
}
