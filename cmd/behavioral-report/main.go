// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/complytime/complyctl/tests/behavioral"
	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/fetcher"
	"github.com/gemaraproj/go-gemara/gemaraconv"
	"github.com/goccy/go-yaml"
)

const (
	policyID    = "CT.COMPLYCTL.CTRL"
	toolName    = "complytime-behavioral"
	toolURI     = "https://github.com/complytime/complytime"
	toolVersion = "0.1.0"
)

func main() {
	binary := flag.String("binary", "bin/complyctl", "Path to the complyctl binary")
	testProvider := flag.String("test-provider", "bin/complyctl-provider-test", "Path to the test provider binary")
	catalogPath := flag.String("catalog", "governance/controls/complytime-controls.yaml", "Path to the Gemara control catalog YAML")
	outDir := flag.String("out", "governance/reports", "Output directory for generated artifacts")
	artifactURI := flag.String("artifact-uri", "governance/controls/complytime-controls.yaml", "SARIF artifact URI")
	targetPolicyID := flag.String("policy-id", "nist-800-53-r5", "Policy ID to evaluate against")
	flag.Parse()

	binaryAbs, err := filepath.Abs(*binary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve binary path: %v\n", err)
		os.Exit(1)
	}
	testProviderAbs, err := filepath.Abs(*testProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve test provider path: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(binaryAbs); err != nil {
		fmt.Fprintf(os.Stderr, "binary not found at %s — run 'make build' first\n", binaryAbs)
		os.Exit(1)
	}

	var catalog *gemara.ControlCatalog
	if *catalogPath != "" {
		uri := toFileURI(*catalogPath)
		c, err := gemara.Load[gemara.ControlCatalog](context.Background(), &fetcher.URI{}, uri)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load catalog %s: %v\n", *catalogPath, err)
		} else {
			catalog = c
		}
	}

	srv := behavioral.StartMockRegistry()
	defer srv.Close()
	fmt.Fprintf(os.Stderr, "mock registry started at %s\n", srv.URL)

	controlEvals := runEvaluations(binaryAbs, testProviderAbs, srv.URL, *targetPolicyID)

	evalLog := gemara.EvaluationLog{
		Evaluations: controlEvals,
		Metadata: gemara.Metadata{
			Id:          policyID,
			Description: "Behavioral test evaluation of CT.COMPLYCTL control catalog",
			Author: gemara.Actor{
				Id:      toolName,
				Name:    toolName,
				Type:    gemara.Software,
				Version: toolVersion,
				Uri:     toolURI,
			},
		},
	}

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	if err := writeEvaluationLog(evalLog, *outDir); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write evaluation log: %v\n", err)
		os.Exit(1)
	}

	if err := writeSARIF(evalLog, *artifactURI, *outDir, catalog); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write SARIF: %v\n", err)
		os.Exit(1)
	}

	printSummary(controlEvals)
}

func runEvaluations(binary, testProvider, registryURL, targetPolicyID string) []*gemara.ControlEvaluation {
	requirementIDs := sortedKeys(behavioral.Plans)
	evals := make([]*gemara.ControlEvaluation, 0, len(requirementIDs))

	for _, reqID := range requirementIDs {
		steps := behavioral.Plans[reqID]
		controlID := behavioral.ControlForRequirement(reqID)

		homeDir, err := os.MkdirTemp("", "behavioral-home-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not create temp home for %s: %v\n", reqID, err)
			continue
		}
		workDir, err := os.MkdirTemp("", "behavioral-work-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not create temp workdir for %s: %v\n", reqID, err)
			continue
		}

		ctx := &behavioral.BehavioralContext{
			Binary:             binary,
			TestProviderBinary: testProvider,
			HomeDir:            homeDir,
			WorkDir:            workDir,
			Env:                behavioral.BuildEnv(homeDir),
			PolicyID:           targetPolicyID,
			RegistryURL:        registryURL,
		}

		eval := &gemara.ControlEvaluation{
			Name:    reqID,
			Control: gemara.EntryMapping{EntryId: controlID},
		}
		eval.AddAssessment(
			reqID,
			"Behavioral assessment of "+reqID,
			[]string{"behavioral"},
			steps,
		)
		eval.Evaluate(ctx, []string{"behavioral"})
		evals = append(evals, eval)

		status := eval.Result.String()
		fmt.Fprintf(os.Stderr, "  %s: %s — %s\n", reqID, status, eval.Message)

		_ = os.RemoveAll(homeDir)
		_ = os.RemoveAll(workDir)
	}

	return evals
}

func writeEvaluationLog(evalLog gemara.EvaluationLog, outDir string) error {
	data, err := yaml.Marshal(evalLog)
	if err != nil {
		return fmt.Errorf("marshal evaluation log: %w", err)
	}
	path := filepath.Join(outDir, "evaluation-log.yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write evaluation log: %w", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	return nil
}

func writeSARIF(evalLog gemara.EvaluationLog, artifactURI, outDir string, catalog *gemara.ControlCatalog) error {
	sarifBytes, err := gemaraconv.ToSARIF(evalLog, gemaraconv.WithArtifactURI(artifactURI), gemaraconv.WithCatalog(catalog))
	if err != nil {
		return fmt.Errorf("SARIF conversion: %w", err)
	}
	path := filepath.Join(outDir, "behavioral-report.sarif.json")
	if err := os.WriteFile(path, sarifBytes, 0600); err != nil {
		return fmt.Errorf("write SARIF: %w", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	return nil
}

func printSummary(evals []*gemara.ControlEvaluation) {
	var passed, failed, other int
	for _, e := range evals {
		switch e.Result {
		case gemara.Passed:
			passed++
		case gemara.Failed:
			failed++
		default:
			other++
		}
	}
	total := passed + failed + other
	fmt.Fprintf(os.Stderr,
		"\nbehavioral assessment: %d total, %d passed, %d failed, %d other\n",
		total, passed, failed, other)
}

func sortedKeys(m map[string][]gemara.AssessmentStep) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func toFileURI(path string) string {
	if strings.HasPrefix(path, "file:///") || strings.HasPrefix(path, "https://") {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return "file://" + abs
}
