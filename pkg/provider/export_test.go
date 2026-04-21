// SPDX-License-Identifier: Apache-2.0

package provider

// RegisterProviderForTest allows test code to register a provider directly
// into the Manager's provider map. This is only available during testing.
func (m *Manager) RegisterProviderForTest(evaluatorID string, p *LoadedProvider) {
	m.plugins[evaluatorID] = p
}

// NewMockLoadedProvider creates a LoadedProvider backed by a mock Provider for tests.
func NewMockLoadedProvider(providerID, evaluatorID string, mock Provider) *LoadedProvider {
	return &LoadedProvider{
		Info: ProviderInfo{
			ProviderID:     providerID,
			EvaluatorID:    evaluatorID,
			ExecutablePath: "(test)",
		},
		Client: mock,
	}
}
