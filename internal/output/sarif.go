// SPDX-License-Identifier: Apache-2.0

package output

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/gemaraconv"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/pkg/provider"
)

// ToSARIF converts a gemara.EvaluationLog to SARIF using go-gemara gemaraconv.
func ToSARIF(log *gemara.EvaluationLog, artifactURI, outDir string) (string, error) {
	sarifBytes, err := gemaraconv.ToSARIF(*log, gemaraconv.WithArtifactURI(artifactURI))
	if err != nil {
		return "", fmt.Errorf("SARIF conversion failed: %w", err)
	}

	if outDir == "" {
		outDir = "."
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	policyID := log.Metadata.Id
	filename := fmt.Sprintf("scan-%s-%s.sarif.json",
		complytime.FilenameSafe(policyID), time.Now().Format("20060102-150405"))
	path := filepath.Join(outDir, filename)
	if err := os.WriteFile(path, sarifBytes, 0600); err != nil {
		return "", fmt.Errorf("failed to write SARIF file: %w", err)
	}
	return path, nil
}

func providerResultToGemara(r provider.Result) gemara.Result {
	switch r {
	case provider.ResultPassed:
		return gemara.Passed
	case provider.ResultFailed:
		return gemara.Failed
	case provider.ResultSkipped:
		return gemara.NotApplicable
	case provider.ResultError:
		return gemara.Unknown
	default:
		return gemara.NotRun
	}
}

func aggregateResultFromSteps(steps []provider.Step) gemara.Result {
	agg := gemara.NotRun
	for _, s := range steps {
		g := providerResultToGemara(s.Result)
		agg = gemara.UpdateAggregateResult(agg, g)
	}
	return agg
}
