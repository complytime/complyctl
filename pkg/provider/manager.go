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

// Manager handles provider discovery, lifecycle, and request routing.
type Manager struct {
	discovery *Discovery
	plugins   map[string]*LoadedProvider
	logger    hclog.Logger
}

// LoadedProvider pairs discovery metadata with a live gRPC client.
type LoadedProvider struct {
	Info   ProviderInfo
	Client Provider
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
			Info:   info,
			Client: client,
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
// variables from the three-tier model (R48). complypackContentPath is the path
// to a cached complypack content archive for the evaluator; pass "" when no
// complypack is available (backward compatible).
func (m *Manager) RouteGenerate(ctx context.Context, evaluatorID string, globalVars, targetVars map[string]string, configs []AssessmentConfiguration, complypackContentPath string) error {
	req := &GenerateRequest{
		GlobalVariables:       globalVars,
		Configuration:         configs,
		TargetVariables:       targetVars,
		ComplypackContentPath: complypackContentPath,
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

// ScanResult holds the combined output of a RouteScanResult call, separating
// evaluation results (Assessments) from operational failures (Errors).
type ScanResult struct {
	Assessments []AssessmentLog
	Errors      []string
	// rpcFailures tracks RPC-level errors with their evaluator context.
	// RouteScan uses these to synthesize backward-compatible error assessments
	// without re-injecting provider-reported operational errors.
	rpcFailures []rpcFailure
}

type rpcFailure struct {
	evaluatorID string
	message     string
}

// HasErrors reports whether the scan encountered operational failures.
func (r *ScanResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// RouteScan dispatches a ScanRequest to the provider matching evaluatorID.
// The provider evaluates all requirements from Generate-time state — no
// requirement IDs are sent over the wire.
// See R47: specs/001-gemara-native-workflow/research.md
//
// Backward-compat: injects synthetic error assessments for operational
// failures so callers that only consume the assessment stream still see
// error entries. New callers should prefer RouteScanResult.
func (m *Manager) RouteScan(ctx context.Context, evaluatorID string, targets []Target) ([]AssessmentLog, error) {
	result, err := m.RouteScanResult(ctx, evaluatorID, targets)
	if err != nil {
		return nil, err
	}
	assessments := result.Assessments
	for _, f := range result.rpcFailures {
		assessments = append(assessments, errorAssessments(f.evaluatorID, f.message)...)
	}
	return assessments, nil
}

// RouteScanResult dispatches a ScanRequest and returns the full ScanResult
// including both assessments and operational errors. Callers that need to
// distinguish evaluation results from infrastructure failures should use
// this method instead of RouteScan.
func (m *Manager) RouteScanResult(ctx context.Context, evaluatorID string, targets []Target) (*ScanResult, error) {
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
			return &ScanResult{
				Errors:      []string{msg},
				rpcFailures: []rpcFailure{{evaluatorID: evaluatorID, message: msg}},
			}, nil
		}
		return &ScanResult{
			Assessments: resp.Assessments,
			Errors:      resp.Errors,
		}, nil
	}

	result := &ScanResult{}
	for _, p := range m.ListProviders() {
		m.logger.Info("Scanning via provider (broadcast)", "provider_id", p.Info.ProviderID)
		resp, scanErr := p.GetClient().Scan(ctx, req)
		if scanErr != nil {
			msg := m.scanErrorMessage(p.Info.ProviderID, scanErr, ctx)
			m.logger.Error("Provider Scan failed",
				"provider_id", p.Info.ProviderID, "error", scanErr)
			result.Errors = append(result.Errors, msg)
			result.rpcFailures = append(result.rpcFailures, rpcFailure{
				evaluatorID: p.Info.EvaluatorID, message: msg,
			})
			continue
		}
		result.Assessments = append(result.Assessments, resp.Assessments...)
		result.Errors = append(result.Errors, resp.Errors...)
	}
	return result, nil
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

func errorAssessments(evaluatorID string, message string) []AssessmentLog {
	return []AssessmentLog{{
		RequirementID: evaluatorID + "-error",
		Steps: []Step{{
			Name:    "provider-error",
			Result:  ResultError,
			Message: message,
		}},
		Message:    message,
		Confidence: ConfidenceLevelNotSet,
	}}
}
