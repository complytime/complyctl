// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"fmt"

	pluginv2 "github.com/complytime/complyctl/api/plugin"
	goplugin "github.com/hashicorp/go-plugin"
)

var (
	_ Provider = (*Client)(nil)
	_ Exporter = (*Client)(nil)
)

// GenerateRequest carries assessment plan configuration to a provider.
// See R48: three-tier variable model.
type GenerateRequest struct {
	GlobalVariables map[string]string
	Configuration   []AssessmentConfiguration
	TargetVariables map[string]string
}

// AssessmentConfiguration binds a requirement ID to its plan and parameters.
type AssessmentConfiguration struct {
	PlanID        string
	RequirementID string
	Parameters    map[string]string
	// EvaluatorID is used for routing to the correct provider. It is not
	// serialized over gRPC — routing is handled by the provider manager.
	EvaluatorID string
}

// GenerateResponse confirms whether policy preparation succeeded.
type GenerateResponse struct {
	Success      bool
	ErrorMessage string
}

// ScanRequest carries targets to evaluate.
// The scanning provider evaluates all requirements from Generate-time state.
// See R47: specs/001-gemara-native-workflow/research.md
type ScanRequest struct {
	Targets []Target
}

// Target identifies a system or environment to scan, with provider-specific variables.
type Target struct {
	TargetID  string
	Variables map[string]string
}

// ScanResponse carries assessment results from a provider scan.
type ScanResponse struct {
	Assessments []AssessmentLog
}

// AssessmentLog holds the evaluation result for a single requirement.
type AssessmentLog struct {
	RequirementID string
	Steps         []Step
	Message       string
	Confidence    ConfidenceLevel
}

// Step is one discrete check within an assessment.
type Step struct {
	Name    string
	Result  Result
	Message string
}

// Result is the outcome of a single assessment step.
type Result int32

const (
	ResultUnspecified Result = 0
	ResultPassed      Result = 1
	ResultFailed      Result = 2
	ResultSkipped     Result = 3
	ResultError       Result = 4
)

// ConfidenceLevel indicates the evaluator's confidence in an assessment result.
// Mirrors go-gemara ConfidenceLevel enum values (1:1 mapping).
type ConfidenceLevel int32

const (
	ConfidenceLevelNotSet       ConfidenceLevel = 0
	ConfidenceLevelUndetermined ConfidenceLevel = 1
	ConfidenceLevelLow          ConfidenceLevel = 2
	ConfidenceLevelMedium       ConfidenceLevel = 3
	ConfidenceLevelHigh         ConfidenceLevel = 4
)

// DescribeRequest is sent to discover provider identity and requirements.
type DescribeRequest struct{}

// DescribeResponse reports provider identity, health, version, and declared
// variable requirements used by doctor diagnostics (R51).
type DescribeResponse struct {
	Healthy                 bool
	Version                 string
	ErrorMessage            string
	RequiredGlobalVariables []string
	RequiredTargetVariables []string
	SupportsExport          bool
}

// ExportRequest carries collector configuration for evidence export.
type ExportRequest struct {
	Collector CollectorConfig
}

// CollectorConfig holds the Beacon collector endpoint and auth credentials.
type CollectorConfig struct {
	Endpoint  string
	AuthToken string //nolint:gosec // not a hardcoded credential
}

// ExportResponse reports the outcome of evidence export.
type ExportResponse struct {
	Success       bool
	ExportedCount int32
	FailedCount   int32
	ErrorMessage  string
}

// Client provides gRPC communication with a provider subprocess managed by
// hashicorp/go-plugin.
type Client struct {
	executablePath string
	gopluginClient *goplugin.Client
	grpcClient     pluginv2.PluginClient
}

func (c *Client) Close() {
	if c.gopluginClient != nil {
		c.gopluginClient.Kill()
	}
}

func (c *Client) Describe(ctx context.Context, req *DescribeRequest) (*DescribeResponse, error) {
	_ = req

	protoResp, err := c.grpcClient.Describe(ctx, &pluginv2.DescribeRequest{})
	if err != nil {
		return nil, fmt.Errorf("Describe RPC failed: %w", err)
	}

	return &DescribeResponse{
		Healthy:                 protoResp.GetHealthy(),
		Version:                 protoResp.GetVersion(),
		ErrorMessage:            protoResp.GetErrorMessage(),
		RequiredGlobalVariables: protoResp.GetRequiredGlobalVariables(),
		RequiredTargetVariables: protoResp.GetRequiredTargetVariables(),
		SupportsExport:          protoResp.GetSupportsExport(),
	}, nil
}

func (c *Client) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	protoConfigs := make([]*pluginv2.AssessmentConfiguration, 0, len(req.Configuration))
	for _, cfg := range req.Configuration {
		protoConfigs = append(protoConfigs, &pluginv2.AssessmentConfiguration{
			PlanId:        cfg.PlanID,
			RequirementId: cfg.RequirementID,
			Parameters:    cfg.Parameters,
		})
	}

	protoResp, err := c.grpcClient.Generate(ctx, &pluginv2.GenerateRequest{
		GlobalVariables: req.GlobalVariables,
		Configurations:  protoConfigs,
		TargetVariables: req.TargetVariables,
	})
	if err != nil {
		return nil, fmt.Errorf("Generate RPC failed: %w", err)
	}

	return &GenerateResponse{
		Success:      protoResp.GetSuccess(),
		ErrorMessage: protoResp.GetErrorMessage(),
	}, nil
}

func (c *Client) Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error) {
	protoTargets := make([]*pluginv2.Target, 0, len(req.Targets))
	for _, t := range req.Targets {
		protoTargets = append(protoTargets, &pluginv2.Target{
			TargetId:  t.TargetID,
			Variables: t.Variables,
		})
	}

	protoResp, err := c.grpcClient.Scan(ctx, &pluginv2.ScanRequest{
		Targets: protoTargets,
	})
	if err != nil {
		return nil, fmt.Errorf("Scan RPC failed: %w", err)
	}

	assessments := make([]AssessmentLog, 0, len(protoResp.GetAssessments()))
	for _, pa := range protoResp.GetAssessments() {
		steps := make([]Step, 0, len(pa.GetSteps()))
		for _, ps := range pa.GetSteps() {
			steps = append(steps, Step{
				Name:    ps.GetName(),
				Result:  protoResultToInternal(ps.GetResult()),
				Message: ps.GetMessage(),
			})
		}
		assessments = append(assessments, AssessmentLog{
			RequirementID: pa.GetRequirementId(),
			Steps:         steps,
			Message:       pa.GetMessage(),
			Confidence:    protoConfidenceToInternal(pa.GetConfidence()),
		})
	}

	return &ScanResponse{Assessments: assessments}, nil
}

func (c *Client) Export(ctx context.Context, req *ExportRequest) (*ExportResponse, error) {
	protoResp, err := c.grpcClient.Export(ctx, &pluginv2.ExportRequest{
		Collector: &pluginv2.CollectorConfig{
			Endpoint:  req.Collector.Endpoint,
			AuthToken: req.Collector.AuthToken,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Export RPC failed: %w", err)
	}

	return &ExportResponse{
		Success:       protoResp.GetSuccess(),
		ExportedCount: protoResp.GetExportedCount(),
		FailedCount:   protoResp.GetFailedCount(),
		ErrorMessage:  protoResp.GetErrorMessage(),
	}, nil
}

func protoResultToInternal(r pluginv2.Result) Result {
	switch r {
	case pluginv2.Result_RESULT_PASSED:
		return ResultPassed
	case pluginv2.Result_RESULT_FAILED:
		return ResultFailed
	case pluginv2.Result_RESULT_SKIPPED:
		return ResultSkipped
	case pluginv2.Result_RESULT_ERROR:
		return ResultError
	default:
		return ResultUnspecified
	}
}

func protoConfidenceToInternal(c pluginv2.ConfidenceLevel) ConfidenceLevel {
	switch c {
	case pluginv2.ConfidenceLevel_CONFIDENCE_LEVEL_UNDETERMINED:
		return ConfidenceLevelUndetermined
	case pluginv2.ConfidenceLevel_CONFIDENCE_LEVEL_LOW:
		return ConfidenceLevelLow
	case pluginv2.ConfidenceLevel_CONFIDENCE_LEVEL_MEDIUM:
		return ConfidenceLevelMedium
	case pluginv2.ConfidenceLevel_CONFIDENCE_LEVEL_HIGH:
		return ConfidenceLevelHigh
	default:
		return ConfidenceLevelNotSet
	}
}
