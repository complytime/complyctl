// SPDX-License-Identifier: Apache-2.0

package complytime

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/oscal-compass/oscal-sdk-go/models"
	"github.com/oscal-compass/oscal-sdk-go/settings"
	"github.com/oscal-compass/oscal-sdk-go/validation"
)

// Framework represents an implemented compliance framework across
// component sources.
type Framework struct {
	// ID is the short-name identifier that is used to consistently
	// represent a framework.
	ID string
	// Title is the human-readable name for a framework
	Title string
	// SupportedComponents define the component titles that implement the
	// framework.
	SupportedComponents []string
}

// LoadFrameworks returns all loaded framework information from a given application directory.
func LoadFrameworks(appDir ApplicationDirectory, validator validation.Validator) ([]Framework, error) {
	definitions, err := FindComponentDefinitions(appDir.BundleDir(), validator)
	if err != nil {
		return nil, fmt.Errorf("error finding component defintions in %s: %w", appDir.BundleDir(), err)
	}

	byFramework := make(map[string]Framework)
	for _, definition := range definitions {
		if definition.Components != nil {
			for _, comp := range *definition.Components {
				// The goal is to only display the target component and abstract away the
				// validation implementation details, so skipping.
				if comp.Type == "validation" {
					continue
				}
				frameworks, err := processComponent(appDir, comp, validator)
				if err != nil {
					return nil, err
				}
				for _, inputFramework := range frameworks {
					framework, ok := byFramework[inputFramework.ID]
					if !ok {
						framework = inputFramework
						framework.SupportedComponents = []string{}
					}
					framework.SupportedComponents = append(framework.SupportedComponents, comp.Title)
					byFramework[framework.ID] = framework
				}
			}
		}
	}

	var frameworks []Framework
	for _, framework := range byFramework {
		frameworks = append(frameworks, framework)
	}
	return frameworks, nil
}

func processComponent(appDir ApplicationDirectory, component oscalTypes.DefinedComponent, validator validation.Validator) ([]Framework, error) {
	if component.ControlImplementations == nil {
		return nil, nil
	}
	var frameworks []Framework
	for _, implementation := range *component.ControlImplementations {
		frameworkShortName, found := settings.GetFrameworkShortName(implementation)
		if !found {
			return nil, fmt.Errorf("no framework information found for implemenation %q", implementation.Description)
		}

		// Load profile of local and get more information for the description
		profile, err := LoadProfile(appDir, implementation.Source, validator)
		if err != nil {
			return nil, fmt.Errorf("error loading control source %s for component %s: %w", frameworkShortName, component.Title, err)
		}

		newFramework := Framework{
			ID:    frameworkShortName,
			Title: profile.Metadata.Title,
		}
		frameworks = append(frameworks, newFramework)

	}
	return frameworks, nil
}

// LoadProfile returns an OSCAL profiles from a given application directory and a found profile source.
func LoadProfile(appDir ApplicationDirectory, controlSource string, validator validation.Validator) (*oscalTypes.Profile, error) {
	sourceFile, err := findControlSource(appDir, controlSource)
	if err != nil {
		return nil, err
	}
	defer sourceFile.Close()
	return models.NewProfile(sourceFile, validator)
}

// GetControlTitle retrieves the title for a control from the catalog
func GetControlTitle(controlID string, controlSource string, appDir ApplicationDirectory, validator validation.Validator) (string, error) {
	profile, err := LoadProfile(appDir, controlSource, validator)
	if err != nil {
		return "", fmt.Errorf("failed to load profile from source '%s': %w", controlSource, err)
	}

	if profile.Imports == nil {
		return "", fmt.Errorf("profile '%s' has no imports", controlSource)
	}

	for _, imp := range profile.Imports {
		catalog, err := LoadCatalogSource(appDir, imp.Href, validator)
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

// findControlSource returns the correct control source file from the given control source or imported source.
func findControlSource(appDir ApplicationDirectory, controlSource string) (io.ReadCloser, error) {
	uri, err := url.ParseRequestURI(controlSource)
	if err != nil {
		return nil, err
	}

	path := uri.Host + uri.Path
	appDirPath := appDir.AppDir()

	// Handle the two supported cases:
	// An absolute path or
	// A path relative to the root of the complytime application directory
	if !filepath.IsAbs(path) {
		if !strings.HasPrefix(path, ControlsDir+string(os.PathSeparator)) {
			return nil, fmt.Errorf("got path %s, control source is expected to be under path %s", path, appDir.ControlDir())
		}
		path = filepath.Join(appDirPath, path)
	}
	cleanedPath := filepath.Clean(path)
	sourceFile, err := os.Open(cleanedPath)
	if err != nil {
		return nil, err
	}
	return sourceFile, nil
}
