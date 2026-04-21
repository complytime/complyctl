// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

const describeTimeout = 30 * time.Second

// Provider is the interface that provider authors implement for evaluation RPCs.
type Provider interface {
	Describe(ctx context.Context, req *DescribeRequest) (*DescribeResponse, error)
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
	Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error)
}

// Exporter is an optional interface for providers that support shipping
// evidence to a Beacon collector. Provider authors opt in by implementing
// this interface and declaring supports_export=true in DescribeResponse.
type Exporter interface {
	Export(ctx context.Context, req *ExportRequest) (*ExportResponse, error)
}

// Manager handles provider discovery, lifecycle, and request routing.
type Manager struct {
	discovery *Discovery
	plugins   map[string]*LoadedProvider
	logger    hclog.Logger
}

// LoadedProvider pairs discovery metadata with a live gRPC client.
type LoadedProvider struct {
	Info           ProviderInfo
	Client         Provider
	SupportsExport bool
}

func NewManager(providerDir string, logger hclog.Logger) (*Manager, error) {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}
	discovery := NewDiscovery(providerDir)
	return &Manager{
		discovery: discovery,
		plugins:   make(map[string]*LoadedProvider),
		logger:    logger,
	}, nil
}

// LoadProviders discovers providers via executable naming convention and verifies
// each via Describe RPC before registering.
func (m *Manager) LoadProviders() error {
	providerInfos, err := m.discovery.DiscoverProviders()
	if err != nil {
		return fmt.Errorf("failed to discover providers: %w", err)
	}

	goPluginLogger := m.logger.Named("go-plugin")

	for _, info := range providerInfos {
		client, err := NewClient(info.ExecutablePath, goPluginLogger)
		if err != nil {
			return fmt.Errorf("failed to create client for provider %s: %w", info.ProviderID, err)
		}

		descCtx, descCancel := context.WithTimeout(context.Background(), describeTimeout)
		descResp, descErr := client.Describe(descCtx, &DescribeRequest{})
		descCancel()
		if descErr != nil {
			client.Close()
			fmt.Fprintf(os.Stderr, "WARNING: provider %s Describe failed: %v (skipping)\n",
				info.ProviderID, descErr)
			continue
		}
		if !descResp.Healthy {
			client.Close()
			fmt.Fprintf(os.Stderr, "WARNING: provider %s is unhealthy: %s (skipping)\n",
				info.ProviderID, descResp.ErrorMessage)
			continue
		}

		lp := &LoadedProvider{
			Info:           info,
			Client:         client,
			SupportsExport: descResp.SupportsExport,
		}

		m.plugins[info.EvaluatorID] = lp
	}

	return nil
}

func (p *LoadedProvider) GetClient() Provider {
	return p.Client
}

func (m *Manager) GetProvider(evaluatorID string) (*LoadedProvider, error) {
	lp, exists := m.plugins[evaluatorID]
	if !exists {
		available := make([]string, 0, len(m.plugins))
		for id := range m.plugins {
			available = append(available, id)
		}
		return nil, fmt.Errorf("provider not found for evaluator ID %q (available: %v)", evaluatorID, available)
	}
	return lp, nil
}

func (m *Manager) ListProviders() []*LoadedProvider {
	providers := make([]*LoadedProvider, 0, len(m.plugins))
	seen := make(map[string]bool)
	for _, lp := range m.plugins {
		if !seen[lp.Info.ProviderID] {
			providers = append(providers, lp)
			seen[lp.Info.ProviderID] = true
		}
	}
	return providers
}

// Cleanup kills all managed provider subprocesses. Call via defer after LoadProviders.
func (m *Manager) Cleanup() {
	goplugin.CleanupClients()
}

// RouteGenerate dispatches a GenerateRequest to the provider matching evaluatorID.
// globalVars carries workspace-level variables; targetVars carries per-target
// variables from the three-tier model (R48).
func (m *Manager) RouteGenerate(ctx context.Context, evaluatorID string, globalVars, targetVars map[string]string, configs []AssessmentConfiguration) error {
	req := &GenerateRequest{
		GlobalVariables: globalVars,
		Configuration:   configs,
		TargetVariables: targetVars,
	}

	if evaluatorID != "" {
		p, err := m.GetProvider(evaluatorID)
		if err != nil {
			return fmt.Errorf("no provider registered for evaluator %q: %w", evaluatorID, err)
		}
		m.logger.Info("Invoking provider Generate", "provider_id", p.Info.ProviderID, "evaluator_id", evaluatorID)
		resp, genErr := p.GetClient().Generate(ctx, req)
		if genErr != nil {
			return fmt.Errorf("provider %s generate failed: %w", p.Info.ProviderID, genErr)
		}
		if !resp.Success {
			return fmt.Errorf("provider %s (evaluator %q): %s", p.Info.ProviderID, evaluatorID, resp.ErrorMessage)
		}
		return nil
	}

	for _, p := range m.ListProviders() {
		m.logger.Info("Invoking provider Generate (broadcast)", "provider_id", p.Info.ProviderID)
		resp, genErr := p.GetClient().Generate(ctx, req)
		if genErr != nil {
			return fmt.Errorf("provider %s generate failed: %w", p.Info.ProviderID, genErr)
		}
		if !resp.Success {
			return fmt.Errorf("provider %s (evaluator %q): %s", p.Info.ProviderID, p.Info.EvaluatorID, resp.ErrorMessage)
		}
	}
	return nil
}

// RouteScan dispatches a ScanRequest to the provider matching evaluatorID.
// The provider evaluates all requirements from Generate-time state — no
// requirement IDs are sent over the wire.
// See R47: specs/001-gemara-native-workflow/research.md
func (m *Manager) RouteScan(ctx context.Context, evaluatorID string, targets []Target) ([]AssessmentLog, error) {
	req := &ScanRequest{
		Targets: targets,
	}

	if evaluatorID != "" {
		p, err := m.GetProvider(evaluatorID)
		if err != nil {
			return nil, fmt.Errorf("no provider registered for evaluator %q: %w", evaluatorID, err)
		}
		m.logger.Info("Scanning via provider", "provider_id", p.Info.ProviderID, "evaluator_id", evaluatorID)
		resp, scanErr := p.GetClient().Scan(ctx, req)
		if scanErr != nil {
			msg := m.scanErrorMessage(p.Info.ProviderID, scanErr, ctx)
			m.logger.Error("Provider Scan failed",
				"provider_id", p.Info.ProviderID, "error", scanErr)
			return errorAssessments(evaluatorID, msg), nil
		}
		return resp.Assessments, nil
	}

	var all []AssessmentLog
	for _, p := range m.ListProviders() {
		m.logger.Info("Scanning via provider (broadcast)", "provider_id", p.Info.ProviderID)
		resp, scanErr := p.GetClient().Scan(ctx, req)
		if scanErr != nil {
			msg := m.scanErrorMessage(p.Info.ProviderID, scanErr, ctx)
			m.logger.Error("Provider Scan failed",
				"provider_id", p.Info.ProviderID, "error", scanErr)
			all = append(all, errorAssessments(p.Info.EvaluatorID, msg)...)
			continue
		}
		all = append(all, resp.Assessments...)
	}
	return all, nil
}

// scanErrorMessage builds an error string for a failed Scan RPC. When the
// failure is a deadline timeout, extra guidance is appended so the operator
// can increase the timeout and find the exact command in the log file.
func (m *Manager) scanErrorMessage(providerID string, scanErr error, ctx context.Context) string {
	msg := fmt.Sprintf("provider %s failed: %v", providerID, scanErr)
	if ctx.Err() == context.DeadlineExceeded {
		msg += "\n\nThe scan exceeded the deadline." +
			"\n  - Increase the timeout: complyctl scan --timeout 15m ..." +
			"\n  - Check .complytime/complyctl.log"
	}
	return msg
}

// RouteExport dispatches an ExportRequest to the provider matching evaluatorID.
// Only providers that declared supports_export=true and implement Exporter are eligible.
func (m *Manager) RouteExport(ctx context.Context, evaluatorID string, req *ExportRequest) (*ExportResponse, error) {
	exporter, providerID, err := m.resolveExporter(evaluatorID)
	if err != nil {
		return nil, err
	}
	m.logger.Info("Exporting via provider", "provider_id", providerID, "evaluator_id", evaluatorID)
	resp, exportErr := exporter.Export(ctx, req)
	if exportErr != nil {
		return nil, fmt.Errorf("provider %s export failed: %w", providerID, exportErr)
	}
	return resp, nil
}

func (m *Manager) resolveExporter(evaluatorID string) (Exporter, string, error) {
	p, err := m.GetProvider(evaluatorID)
	if err != nil {
		return nil, "", fmt.Errorf("no provider registered for evaluator %q: %w", evaluatorID, err)
	}
	if !p.SupportsExport {
		return nil, "", fmt.Errorf("provider %s does not support export", p.Info.ProviderID)
	}
	exporter, ok := p.GetClient().(Exporter)
	if !ok {
		return nil, "", fmt.Errorf("provider %s declared supports_export but does not implement Exporter", p.Info.ProviderID)
	}
	return exporter, p.Info.ProviderID, nil
}

func errorAssessments(evaluatorID string, message string) []AssessmentLog {
	return []AssessmentLog{{
		RequirementID: evaluatorID + "-error",
		Steps: []Step{{
			Name:    "provider-error",
			Result:  ResultError,
			Message: message,
		}},
		Message:    message,
		Confidence: 0,
	}}
}
