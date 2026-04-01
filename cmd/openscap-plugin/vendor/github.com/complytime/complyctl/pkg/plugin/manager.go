// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

const describeTimeout = 30 * time.Second

// Plugin is the interface that plugin authors implement for evaluation RPCs.
type Plugin interface {
	Describe(ctx context.Context, req *DescribeRequest) (*DescribeResponse, error)
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
	Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error)
}

// Manager handles plugin discovery, lifecycle, and request routing.
type Manager struct {
	discovery *Discovery
	plugins   map[string]*LoadedPlugin
	logger    hclog.Logger
}

// LoadedPlugin pairs discovery metadata with a live gRPC client.
type LoadedPlugin struct {
	Info   PluginInfo
	Client *Client
}

func NewManager(pluginDir string, logger hclog.Logger) (*Manager, error) {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}
	discovery := NewDiscovery(pluginDir)
	return &Manager{
		discovery: discovery,
		plugins:   make(map[string]*LoadedPlugin),
		logger:    logger,
	}, nil
}

// LoadPlugins discovers plugins via executable naming convention and verifies
// each via Describe RPC before registering.
func (m *Manager) LoadPlugins() error {
	pluginInfos, err := m.discovery.DiscoverPlugins()
	if err != nil {
		return fmt.Errorf("failed to discover plugins: %w", err)
	}

	goPluginLogger := m.logger.Named("go-plugin")

	for _, info := range pluginInfos {
		client, err := NewClient(info.ExecutablePath, goPluginLogger)
		if err != nil {
			return fmt.Errorf("failed to create client for plugin %s: %w", info.PluginID, err)
		}

		descCtx, descCancel := context.WithTimeout(context.Background(), describeTimeout)
		descResp, descErr := client.Describe(descCtx, &DescribeRequest{})
		descCancel()
		if descErr != nil {
			client.Close()
			fmt.Fprintf(os.Stderr, "WARNING: plugin %s Describe failed: %v (skipping)\n",
				info.PluginID, descErr)
			continue
		}
		if !descResp.Healthy {
			client.Close()
			fmt.Fprintf(os.Stderr, "WARNING: plugin %s is unhealthy: %s (skipping)\n",
				info.PluginID, descResp.ErrorMessage)
			continue
		}

		lp := &LoadedPlugin{
			Info:   info,
			Client: client,
		}

		m.plugins[info.EvaluatorID] = lp
	}

	return nil
}

func (p *LoadedPlugin) GetClient() Plugin {
	return p.Client
}

func (m *Manager) GetPlugin(evaluatorID string) (*LoadedPlugin, error) {
	lp, exists := m.plugins[evaluatorID]
	if !exists {
		available := make([]string, 0, len(m.plugins))
		for id := range m.plugins {
			available = append(available, id)
		}
		return nil, fmt.Errorf("plugin not found for evaluator ID %q (available: %v)", evaluatorID, available)
	}
	return lp, nil
}

func (m *Manager) ListPlugins() []*LoadedPlugin {
	plugins := make([]*LoadedPlugin, 0, len(m.plugins))
	seen := make(map[string]bool)
	for _, lp := range m.plugins {
		if !seen[lp.Info.PluginID] {
			plugins = append(plugins, lp)
			seen[lp.Info.PluginID] = true
		}
	}
	return plugins
}

// Cleanup kills all managed plugin subprocesses. Call via defer after LoadPlugins.
func (m *Manager) Cleanup() {
	goplugin.CleanupClients()
}

// RouteGenerate dispatches a GenerateRequest to the plugin matching evaluatorID.
// globalVars carries workspace-level variables; targetVars carries per-target
// variables from the three-tier model (R48).
func (m *Manager) RouteGenerate(ctx context.Context, evaluatorID string, globalVars, targetVars map[string]string, configs []AssessmentConfiguration) error {
	req := &GenerateRequest{
		GlobalVariables: globalVars,
		Configuration:   configs,
		TargetVariables: targetVars,
	}

	if evaluatorID != "" {
		p, err := m.GetPlugin(evaluatorID)
		if err != nil {
			return fmt.Errorf("no plugin registered for evaluator %q: %w", evaluatorID, err)
		}
		m.logger.Info("Invoking plugin Generate", "plugin_id", p.Info.PluginID, "evaluator_id", evaluatorID)
		resp, genErr := p.GetClient().Generate(ctx, req)
		if genErr != nil {
			return fmt.Errorf("plugin %s generate failed: %w", p.Info.PluginID, genErr)
		}
		if !resp.Success {
			return fmt.Errorf("plugin %s (evaluator %q): %s", p.Info.PluginID, evaluatorID, resp.ErrorMessage)
		}
		return nil
	}

	for _, p := range m.ListPlugins() {
		m.logger.Info("Invoking plugin Generate (broadcast)", "plugin_id", p.Info.PluginID)
		resp, genErr := p.GetClient().Generate(ctx, req)
		if genErr != nil {
			return fmt.Errorf("plugin %s generate failed: %w", p.Info.PluginID, genErr)
		}
		if !resp.Success {
			return fmt.Errorf("plugin %s (evaluator %q): %s", p.Info.PluginID, p.Info.EvaluatorID, resp.ErrorMessage)
		}
	}
	return nil
}

// RouteScan dispatches a ScanRequest to the plugin matching evaluatorID.
// The provider evaluates all requirements from Generate-time state — no
// requirement IDs are sent over the wire.
// See R47: specs/001-gemara-native-workflow/research.md
func (m *Manager) RouteScan(ctx context.Context, evaluatorID string, targets []Target) ([]AssessmentLog, error) {
	req := &ScanRequest{
		Targets: targets,
	}

	if evaluatorID != "" {
		p, err := m.GetPlugin(evaluatorID)
		if err != nil {
			return nil, fmt.Errorf("no plugin registered for evaluator %q: %w", evaluatorID, err)
		}
		m.logger.Info("Scanning via plugin", "plugin_id", p.Info.PluginID, "evaluator_id", evaluatorID)
		resp, scanErr := p.GetClient().Scan(ctx, req)
		if scanErr != nil {
			msg := m.scanErrorMessage(p.Info.PluginID, scanErr, ctx)
			m.logger.Error("Plugin Scan failed",
				"plugin_id", p.Info.PluginID, "error", scanErr)
			return errorAssessments(evaluatorID, msg), nil
		}
		return resp.Assessments, nil
	}

	var all []AssessmentLog
	for _, p := range m.ListPlugins() {
		m.logger.Info("Scanning via plugin (broadcast)", "plugin_id", p.Info.PluginID)
		resp, scanErr := p.GetClient().Scan(ctx, req)
		if scanErr != nil {
			msg := m.scanErrorMessage(p.Info.PluginID, scanErr, ctx)
			m.logger.Error("Plugin Scan failed",
				"plugin_id", p.Info.PluginID, "error", scanErr)
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
func (m *Manager) scanErrorMessage(pluginID string, scanErr error, ctx context.Context) string {
	msg := fmt.Sprintf("plugin %s failed: %v", pluginID, scanErr)
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
			Name:    "plugin-error",
			Result:  ResultError,
			Message: message,
		}},
		Message:    message,
		Confidence: 0,
	}}
}
