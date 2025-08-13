/*
 Copyright 2024 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package framework

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"

	"github.com/oscal-compass/compliance-to-policy-go/v2/plugin"
	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
)

// PluginManager manages the plugin lifecycle and compliance-to-policy
// workflows.
type PluginManager struct {
	// pluginDir is the location to search for plugins.
	pluginDir string
	// pluginManifestDir is the location to search for plugin manifests.
	pluginManifestDir string
	// clientFactory is the function used to
	// create new plugin clients.
	clientFactory plugin.ClientFactoryFunc
	// logger for the PluginManager
	log hclog.Logger
}

// NewPluginManager creates a new instance of a PluginManager from a C2PConfig that can be used to
// manage supported plugins.
//
// It supports the plugin lifecycle with the following methods:
//   - Discover plugins: FindRequestedPlugins()
//   - Launching and initializing plugins: LaunchPolicyPlugins()
//   - Clean/Stop - Clean()
func NewPluginManager(cfg *C2PConfig) (*PluginManager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &PluginManager{
		pluginDir:         cfg.PluginDir,
		pluginManifestDir: cfg.PluginManifestDir,
		clientFactory:     plugin.ClientFactory(cfg.Logger),
		log:               cfg.Logger,
	}, nil
}

// FindRequestedPlugins retrieves information for the plugins that have been requested
// returns the plugin manifests for use with LaunchPolicyPlugins().
func (m *PluginManager) FindRequestedPlugins(requestedPlugins []plugin.ID) (plugin.Manifests, error) {
	m.log.Info(fmt.Sprintf("Searching for plugins in %s", m.pluginDir))

	pluginManifests, err := plugin.FindPlugins(
		m.pluginDir,
		m.pluginManifestDir,
		plugin.WithProviderIds(requestedPlugins),
		plugin.WithPluginType(plugin.PVPPluginName),
	)
	if err != nil {
		return pluginManifests, err
	}
	m.log.Debug(fmt.Sprintf("Found %d matching plugins", len(pluginManifests)))
	return pluginManifests, nil
}

// LaunchPolicyPlugins launches requested policy plugins and configures each plugin to make it ready for use with defined plugin workflows.
// The plugin is configured based on default options and given options.
// Given options are represented by config.PluginConfig.
func (m *PluginManager) LaunchPolicyPlugins(ctx context.Context, manifests plugin.Manifests, pluginConfig PluginConfig) (map[plugin.ID]policy.Provider, error) {
	pluginsByIds := make(map[plugin.ID]policy.Provider)
	for _, manifest := range manifests {
		policyPlugin, err := plugin.NewPolicyPlugin(manifest, m.clientFactory)
		if err != nil {
			return pluginsByIds, err
		}
		pluginsByIds[manifest.ID] = policyPlugin
		m.log.Debug(fmt.Sprintf("Launched plugin %s", manifest.ID))
		m.log.Debug(fmt.Sprintf("Gathering configuration options for %s", manifest.ID))

		// Get all the base configuration
		if len(manifest.Configuration) > 0 {
			if err := m.configurePlugin(ctx, policyPlugin, manifest, pluginConfig); err != nil {
				return pluginsByIds, fmt.Errorf("failed to configure plugin %s: %w", manifest.ID, err)
			}
		}
	}
	return pluginsByIds, nil
}

func (m *PluginManager) configurePlugin(ctx context.Context, policyPlugin policy.Provider, manifest plugin.Manifest, pluginConfig PluginConfig) error {
	selections := pluginConfig(manifest.ID)
	if selections == nil {
		selections = make(map[string]string)
		m.log.Debug("No overrides set for plugin %s, using defaults...", manifest.ID)
	}
	configMap, err := manifest.ResolveOptions(selections, m.log)
	if err != nil {
		return err
	}
	if err := policyPlugin.Configure(ctx, configMap); err != nil {
		return err
	}
	return nil
}

// Clean deletes managed instances of plugin clients that have been created using LaunchPolicyPlugins.
// This will remove all clients launched with the plugin.ClientFactoryFunc.
func (m *PluginManager) Clean() {
	m.log.Debug("Cleaning launched plugins")
	plugin.Cleanup()
}
