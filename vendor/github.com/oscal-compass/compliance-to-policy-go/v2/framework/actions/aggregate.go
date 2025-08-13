/*
 Copyright 2025 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/oscal-compass/oscal-sdk-go/settings"
	"golang.org/x/sync/errgroup"

	"github.com/oscal-compass/compliance-to-policy-go/v2/logging"
	"github.com/oscal-compass/compliance-to-policy-go/v2/plugin"
	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
)

// AggregateResults action identifies policy configuration for each provider in the given pluginSet to execute the GetResults() method
// each policy.Provider.
//
// The rule set passed to each plugin can be configured with compliance specific settings based on the InputContext.
func AggregateResults(ctx context.Context, inputContext *InputContext, pluginSet map[plugin.ID]policy.Provider) ([]policy.PVPResult, error) {
	log := logging.GetLogger("aggregator")

	var allResults []policy.PVPResult
	resultChan := make(chan policy.PVPResult, len(pluginSet))

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(inputContext.MaxConcurrency)
	for providerId, policyPlugin := range pluginSet {
		func(providerId plugin.ID, plugin policy.Provider) {
			eg.Go(func() error {
				select {
				case <-egCtx.Done():
					return fmt.Errorf("%s skipped due to context cancellation/timeout: %w", providerId.String(), egCtx.Err())
				default:
				}

				componentTitle, err := inputContext.ProviderTitle(providerId)
				if err != nil {
					if errors.Is(err, ErrMissingProvider) {
						log.Warn(fmt.Sprintf("skipping %s provider: missing validation component", providerId))
						return nil
					}
					return err
				}
				log.Debug(fmt.Sprintf("Aggregating results for provider %s", providerId))

				appliedRuleSet, err := settings.ApplyToComponent(egCtx, componentTitle, inputContext.Store(), inputContext.Settings)
				if err != nil {
					return fmt.Errorf("failed to get rule sets for component %s: %w", componentTitle, err)
				}

				pluginResults, err := policyPlugin.GetResults(egCtx, appliedRuleSet)
				if err != nil {
					return err
				}
				resultChan <- pluginResults
				return nil
			})
		}(providerId, policyPlugin)
	}

	go func() {
		_ = eg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		allResults = append(allResults, result)
	}

	// Calling Wait() again to avoid data races
	if err := eg.Wait(); err != nil {
		return allResults, err
	}

	return allResults, nil
}
