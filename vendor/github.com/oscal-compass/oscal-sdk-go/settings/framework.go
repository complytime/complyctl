/*
 Copyright 2024 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package settings

import (
	"fmt"
	"path/filepath"
	"strings"

	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"

	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/oscal-compass/oscal-sdk-go/models/components"
)

// GetFrameworkShortName returns the human-readable short name for the control source in a
// control implementation set and whether this value is populated.
//
// This function checks the associated properties and falls back to the implementation
// Source reference.
func GetFrameworkShortName(implementation oscalTypes.ControlImplementationSet) (string, bool) {
	const (
		expectedPathParts = 3
		modelIDIndex      = 1
		filenameIndex     = 2
	)
	// Looks for the property, fallback to parsing it out of the control source href.
	if implementation.Props != nil {
		property, found := extensions.GetTrestleProp(extensions.FrameworkProp, *implementation.Props)
		if found {
			return property.Value, true
		}
	}

	// Fallback to the control source string based on trestle
	// workspace conventions of $MODEL/$MODEL_ID/$MODEL.json.
	cleanedSource := filepath.Clean(implementation.Source)
	parts := strings.Split(cleanedSource, "/")
	if len(parts) == expectedPathParts && strings.HasSuffix(parts[filenameIndex], ".json") {
		return parts[modelIDIndex], true
	}

	return "", false
}

// FrameworkSource defines data for a control source or framework.
type FrameworkSource struct {
	Title       string
	Description string
	Href        string
}

// ByFramework returns ImplementationSettings and FrameworkSource from a list of OSCAL Control Implementations for a given framework. If multiple matches are found, the
// implementation settings are merged together.
func ByFramework(framework string, controlImplementations []oscalTypes.ControlImplementationSet) (*ImplementationSettings, FrameworkSource, error) {
	var implementationSettings *ImplementationSettings
	var frameworkSource FrameworkSource

	for _, controlImplementation := range controlImplementations {
		frameworkShortName, found := GetFrameworkShortName(controlImplementation)
		implementationAdapter := components.NewControlImplementationSetAdapter(controlImplementation)
		if found && frameworkShortName == framework {
			if implementationSettings == nil {
				implementationSettings = NewImplementationSettings(implementationAdapter)
				frameworkSource.Description = controlImplementation.Description
				frameworkSource.Href = controlImplementation.Source
				frameworkSource.Title = framework
			} else {
				implementationSettings.merge(implementationAdapter)
			}
		}
	}

	if implementationSettings == nil {
		return implementationSettings, frameworkSource, fmt.Errorf("framework %s is not in control implementations", framework)
	}
	return implementationSettings, frameworkSource, nil
}
