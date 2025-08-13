/*
 Copyright 2024 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
)

// Manifest is metadata about a plugin to support discovering and
// launching plugins. This should be provided with the plugin on-disk.
type Manifest struct {
	// Metadata has required information for plugin launch and discovery.
	Metadata `json:"metadata"`
	// ExecutablePath is the path to the plugin binary.
	ExecutablePath string `json:"executablePath"`
	// Checksum is the SHA256 hash of the content.
	// This checked against the calculated value at plugin launch.
	Checksum string `json:"sha256"`
	// Configuration is an optional section to add plugin
	// configuration options and default values.
	Configuration []ConfigurationOption `json:"configuration,omitempty"`
}

// ResolvePath validates and sanitizes the Manifest.ExecutablePath.
//
// If the path is not absolute, it updates Manifest.ExecutablePath field
// to a location under the given plugin directory. If the path is absolute,
// it validates it is under the given plugin directory.
func (m *Manifest) ResolvePath(pluginDir string) error {
	absPluginDir, err := filepath.Abs(pluginDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute plugin directory: %w", err)
	}

	// Handle different path types (relative and absolute)
	var cleanedPath string
	if filepath.IsAbs(m.ExecutablePath) {
		cleanedPath = filepath.Clean(m.ExecutablePath)
		if !strings.HasPrefix(cleanedPath, absPluginDir+string(os.PathSeparator)) {
			return fmt.Errorf("absolute path %s is not under the plugin directory %s", m.ExecutablePath, absPluginDir)
		}
	} else {
		cleanedPath = filepath.Clean(filepath.Join(absPluginDir, m.ExecutablePath))
	}

	fileInfo, err := os.Stat(cleanedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("plugin executable %s does not exist: %w", cleanedPath, err)
		}
		return fmt.Errorf("failed to stat plugin executable: %w", err)
	}

	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("plugin executable %s is not a file", cleanedPath)
	}

	if fileInfo.Mode()&0100 == 0 {
		return fmt.Errorf("plugin file %s is not executable", cleanedPath)
	}
	m.ExecutablePath = cleanedPath

	return nil
}

// ResolveOptions validates and applies given configuration selections against the manifest
// declared configuration and returns the resolved options.
func (m *Manifest) ResolveOptions(configSelections map[string]string, log hclog.Logger) (map[string]string, error) {
	configMap := make(map[string]string)
	processedOptions := make(map[string]struct{})
	for _, option := range m.Configuration {
		// Grab the defaults for each
		if option.Default != nil {
			configMap[option.Name] = *option.Default
		}

		// Apply overrides, if they do not exist for required options,
		// fail.
		selected, ok := configSelections[option.Name]
		if ok {
			configMap[option.Name] = selected
			processedOptions[option.Name] = struct{}{}
		} else if option.Required {
			return nil,
				fmt.Errorf("required value not supplied for option %q", option.Name)
		}
	}
	var unknownKeys []string
	for key := range configSelections {
		if _, found := processedOptions[key]; !found {
			unknownKeys = append(unknownKeys, key)
		}
	}
	if len(unknownKeys) > 0 {
		log.Warn(fmt.Sprintf("Unknown configuration options found: %s", strings.Join(unknownKeys, ", ")))
	}
	return configMap, nil
}

// Metadata has required information for plugin launch and discovery.
type Metadata struct {
	// ID is the name of the plugin. This is the information used
	// when a plugin is requested.
	ID ID `json:"id"`
	// Description is a short description for the plugin.
	Description string `json:"description"`
	// Version is the semantic version of the
	// plugin.
	Version string `json:"version"`
	// Type defined which supported plugin types
	// are implemented by this plugin. It should match
	// on or more of the values in plugin.SupportedPlugin.
	Types []string `json:"types"`
}

// ConfigurationOption defines an option for configuring plugin behavior.
type ConfigurationOption struct {
	// Name is the human-readable name of the option.
	Name string `json:"name"`
	// Description is a short description of the option.
	Description string `json:"description"`
	// Required is whether the option is required to be set
	Required bool `json:"required"`
	// Default is an optional parameter with the default selected value.
	Default *string `json:"default,omitempty"`
}

// Manifests defines the Manifest by plugin id.
type Manifests map[ID]Manifest
