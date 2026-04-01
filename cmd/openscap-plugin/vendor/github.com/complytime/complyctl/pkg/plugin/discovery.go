// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/complytime/complyctl/internal/complytime"
)

// PluginInfo holds the identity and filesystem path of a discovered plugin.
type PluginInfo struct {
	PluginID       string
	EvaluatorID    string
	ExecutablePath string
}

// Discovery scans a directory for plugin executables matching the naming convention.
type Discovery struct {
	pluginDir string
}

func NewDiscovery(pluginDir string) *Discovery {
	return &Discovery{
		pluginDir: pluginDir,
	}
}

// DiscoverPlugins scans the user plugin directory and the system-wide
// provider directory for executables matching the naming convention.
// User-directory providers take precedence over system-installed ones.
func (d *Discovery) DiscoverPlugins() ([]PluginInfo, error) {
	seen := make(map[string]bool)
	var plugins []PluginInfo

	userPlugins, err := scanDir(expandPath(d.pluginDir))
	if err != nil {
		return nil, err
	}
	for _, p := range userPlugins {
		seen[p.EvaluatorID] = true
		plugins = append(plugins, p)
	}

	sysPlugins, err := scanDir(complytime.SystemProviderDir)
	if err != nil {
		return nil, err
	}
	for _, p := range sysPlugins {
		if !seen[p.EvaluatorID] {
			plugins = append(plugins, p)
		}
	}

	return plugins, nil
}

func scanDir(dir string) ([]PluginInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read plugin directory %s: %w", dir, err)
	}

	var plugins []PluginInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasPrefix(entry.Name(), complytime.PluginExecutablePrefix) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Mode()&0111 == 0 {
			continue
		}

		pluginID := strings.TrimPrefix(entry.Name(), complytime.PluginExecutablePrefix)
		plugins = append(plugins, PluginInfo{
			PluginID:       pluginID,
			EvaluatorID:    pluginID,
			ExecutablePath: filepath.Join(dir, entry.Name()),
		})
	}

	return plugins, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}
