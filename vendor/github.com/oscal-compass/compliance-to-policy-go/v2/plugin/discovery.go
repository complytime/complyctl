/*
 Copyright 2024 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	manifestPrefix = "c2p-"
	manifestSuffix = "-manifest.json"
)

type findOptions struct {
	providerIds []ID
	pluginType  string
}

// FindOption represents a filtering criteria for plugin discovery in plugin.FindPlugins.
type FindOption func(options *findOptions)

// WithProviderIds filters plugins by their provider IDs.
func WithProviderIds(providerIds []ID) FindOption {
	return func(options *findOptions) {
		options.providerIds = providerIds
	}
}

// WithPluginType filters available plugins based on the plugin type
// implemented.
func WithPluginType(pluginType string) FindOption {
	return func(options *findOptions) {
		options.pluginType = pluginType
	}
}

// FindPlugins searches for plugins in the specified directory, optionally applying filters.
//
// The function expects plugin manifests in the format "c2p-$PLUGIN-ID-manifest.json".
//
// Available filters:
//   - `WithProviderIds`: Filters by a list of provider IDs.
//   - `WithPluginType`: Filters by plugin type.
//
// If no filters are applied, all discovered plugins are returned.
func FindPlugins(pluginDir, pluginManifestDir string, opts ...FindOption) (Manifests, error) {
	config := &findOptions{}
	for _, opt := range opts {
		opt(config)
	}

	matchingPlugins, err := findAllPluginMatches(pluginManifestDir)
	if err != nil {
		return nil, err
	}

	if len(matchingPlugins) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrPluginsNotFound, pluginDir)
	}

	collectedManifests := make(Manifests)
	var errs []error

	// Filter plugins by provider IDs if provided
	if len(config.providerIds) != 0 {
		filteredIds := make(map[ID]string)
		for _, providerId := range config.providerIds {
			if _, ok := matchingPlugins[providerId]; !ok {
				errs = append(errs, &NotFoundError{providerId.String()})
			}
			filteredIds[providerId] = matchingPlugins[providerId]
		}
		matchingPlugins = filteredIds

		// Return early if there are errors to avoid unnecessary processing
		if len(errs) > 0 {
			return nil, errors.Join(errs...)
		}
	}

	// Process remaining plugins, filtering by plugin type if necessary
	for id, manifestName := range matchingPlugins {
		manifestPath := filepath.Join(pluginManifestDir, manifestName)
		manifest, err := readManifestFile(id, manifestPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// Ensure consistent naming for the plugin identifier and
		// that the name meets identifier criteria.
		if !manifest.ID.Validate() || manifest.ID != id {
			errs = append(errs, fmt.Errorf("invalid plugin id %q in manifest %s", manifest.ID, manifestName))
			continue
		}

		if config.pluginType != "" && !manifestMatchesType(manifest, config.pluginType) {
			continue
		}

		// sanitize the executable path in the manifest
		if manifestErr := manifest.ResolvePath(pluginDir); manifestErr != nil {
			errs = append(errs, manifestErr)
			continue
		}
		collectedManifests[id] = manifest
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	if len(collectedManifests) == 0 {
		return nil, fmt.Errorf("%w in %s with matching criteria", ErrPluginsNotFound, pluginDir)
	}

	return collectedManifests, nil
}

// findAllPluginsMatches locates the manifests in the plugin manifest directory that match
// the prefix naming scheme and returns the plugin ID and file name.
func findAllPluginMatches(pluginManifestDir string) (map[ID]string, error) {
	items, err := os.ReadDir(pluginManifestDir)
	if err != nil {
		return nil, err
	}

	matchingPlugins := make(map[ID]string)
	for _, item := range items {
		name := item.Name()
		if !strings.HasPrefix(name, manifestPrefix) {
			continue
		}
		trimmedName := strings.TrimPrefix(name, manifestPrefix)
		trimmedName = strings.TrimSuffix(trimmedName, manifestSuffix)
		id := ID(trimmedName)
		matchingPlugins[id] = name
	}
	return matchingPlugins, nil
}

// manifestMatchesType checks if the plugin manifest defines
// the plugin type being searched for.
func manifestMatchesType(manifest Manifest, pluginType string) bool {
	for _, typ := range manifest.Types {
		if typ == pluginType {
			return true
		}
	}
	return false
}

// readManifestFile reads and parses the manifest from JSON.
func readManifestFile(pluginName ID, manifestPath string) (Manifest, error) {
	cleanedPath := filepath.Clean(manifestPath)
	manifestFile, err := os.Open(cleanedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, &ManifestNotFoundError{File: manifestPath, PluginID: pluginName.String()}
		}
		return Manifest{}, err
	}
	defer manifestFile.Close()

	jsonParser := json.NewDecoder(manifestFile)
	var manifest Manifest
	err = jsonParser.Decode(&manifest)
	if err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
