// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/complytime/complyctl/internal/complytime"
)

// ProviderInfo holds the identity and filesystem path of a discovered provider.
type ProviderInfo struct {
	ProviderID     string
	EvaluatorID    string
	ExecutablePath string
}

// Discovery scans a directory for provider executables matching the naming convention.
type Discovery struct {
	providerDir string
}

func NewDiscovery(providerDir string) *Discovery {
	return &Discovery{
		providerDir: providerDir,
	}
}

// DiscoverProviders scans the user provider directory and the system-wide
// provider directory for executables matching the naming convention.
// User-directory providers take precedence over system-installed ones.
func (d *Discovery) DiscoverProviders() ([]ProviderInfo, error) {
	seen := make(map[string]bool)
	var providers []ProviderInfo

	userProviders, err := scanDir(expandPath(d.providerDir))
	if err != nil {
		return nil, err
	}
	for _, p := range userProviders {
		seen[p.EvaluatorID] = true
		providers = append(providers, p)
	}

	sysProviders, err := scanDir(complytime.SystemProviderDir)
	if err != nil {
		return nil, err
	}
	for _, p := range sysProviders {
		if !seen[p.EvaluatorID] {
			providers = append(providers, p)
		}
	}

	return providers, nil
}

func scanDir(dir string) ([]ProviderInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read provider directory %s: %w", dir, err)
	}

	var providers []ProviderInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasPrefix(entry.Name(), complytime.ProviderExecutablePrefix) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Mode()&0111 == 0 {
			continue
		}

		providerID := strings.TrimPrefix(entry.Name(), complytime.ProviderExecutablePrefix)
		providers = append(providers, ProviderInfo{
			ProviderID:     providerID,
			EvaluatorID:    providerID,
			ExecutablePath: filepath.Join(dir, entry.Name()),
		})
	}

	return providers, nil
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
