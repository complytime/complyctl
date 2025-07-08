// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/oscal-compass/oscal-sdk-go/validation"

	"github.com/complytime/complyctl/internal/complytime"
)

// getControlTitle retrieves the title for a control from the catalog
func getControlTitle(controlID string, controlSource string, appDir complytime.ApplicationDirectory, validator validation.Validator) (string, error) {
	profile, err := complytime.LoadProfile(appDir, controlSource, validator)
	if err != nil {
		return "", fmt.Errorf("failed to load profile from source '%s': %w", controlSource, err)
	}

	if profile.Imports == nil {
		return "", fmt.Errorf("profile '%s' has no imports", controlSource)
	}

	for _, imp := range profile.Imports {
		catalog, err := complytime.LoadCatalogSource(appDir, imp.Href, validator)
		if err != nil {
			continue
		}
		if catalog.Groups == nil {
			continue
		}
		for _, group := range *catalog.Groups {
			if group.Controls == nil {
				continue
			}
			for _, control := range *group.Controls {
				if control.ID == controlID && control.Title != "" {
					return control.Title, nil
				}
			}
		}
	}
	return "", fmt.Errorf("title for control '%s' not found in catalog", controlID)
}
