/*
 Copyright 2024 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package policy

import "context"

// Provider defines methods for a policy engine C2P plugin.
type Provider interface {
	// Configure send configuration options and selected values to the
	// plugin.
	Configure(context.Context, map[string]string) error
	// Generate policy artifacts for a specific policy engine.
	Generate(context.Context, Policy) error
	// GetResults from a specific policy engine and transform into
	// PVPResults.
	GetResults(context.Context, Policy) (PVPResult, error)
}
