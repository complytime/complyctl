// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"

	proto "github.com/complytime/complyctl/api/plugin"
)

var _ proto.PluginServer = (*grpcServer)(nil)

// grpcServer adapts the Provider domain interface to the proto-generated
// PluginServer for registration on the provider process side.
type grpcServer struct {
	proto.UnimplementedPluginServer
	impl Provider
}

func (s *grpcServer) Describe(ctx context.Context, _ *proto.DescribeRequest) (*proto.DescribeResponse, error) {
	resp, err := s.impl.Describe(ctx, &DescribeRequest{})
	if err != nil {
		return nil, err
	}
	return &proto.DescribeResponse{
		Healthy:                 resp.Healthy,
		Version:                 resp.Version,
		ErrorMessage:            resp.ErrorMessage,
		RequiredGlobalVariables: resp.RequiredGlobalVariables,
		RequiredTargetVariables: resp.RequiredTargetVariables,
		SupportsExport:          resp.SupportsExport,
	}, nil
}

func (s *grpcServer) Generate(ctx context.Context, req *proto.GenerateRequest) (*proto.GenerateResponse, error) {
	configs := make([]AssessmentConfiguration, 0, len(req.GetConfigurations()))
	for _, c := range req.GetConfigurations() {
		configs = append(configs, AssessmentConfiguration{
			PlanID:        c.GetPlanId(),
			RequirementID: c.GetRequirementId(),
			Parameters:    c.GetParameters(),
		})
	}

	resp, err := s.impl.Generate(ctx, &GenerateRequest{
		GlobalVariables: req.GetGlobalVariables(),
		Configuration:   configs,
		TargetVariables: req.GetTargetVariables(),
	})
	if err != nil {
		return nil, err
	}

	return &proto.GenerateResponse{
		Success:      resp.Success,
		ErrorMessage: resp.ErrorMessage,
	}, nil
}

func (s *grpcServer) Scan(ctx context.Context, req *proto.ScanRequest) (*proto.ScanResponse, error) {
	targets := make([]Target, 0, len(req.GetTargets()))
	for _, t := range req.GetTargets() {
		targets = append(targets, Target{
			TargetID:  t.GetTargetId(),
			Variables: t.GetVariables(),
		})
	}

	resp, err := s.impl.Scan(ctx, &ScanRequest{
		Targets: targets,
	})
	if err != nil {
		return nil, err
	}

	protoAssessments := make([]*proto.AssessmentLog, 0, len(resp.Assessments))
	for _, a := range resp.Assessments {
		protoSteps := make([]*proto.Step, 0, len(a.Steps))
		for _, step := range a.Steps {
			protoSteps = append(protoSteps, &proto.Step{
				Name:    step.Name,
				Result:  internalResultToProto(step.Result),
				Message: step.Message,
			})
		}
		protoAssessments = append(protoAssessments, &proto.AssessmentLog{
			RequirementId: a.RequirementID,
			Steps:         protoSteps,
			Message:       a.Message,
			Confidence:    internalConfidenceToProto(a.Confidence),
		})
	}

	return &proto.ScanResponse{Assessments: protoAssessments}, nil
}

func (s *grpcServer) Export(ctx context.Context, req *proto.ExportRequest) (*proto.ExportResponse, error) {
	exporter, ok := s.impl.(Exporter)
	if !ok {
		return &proto.ExportResponse{
			Success:      false,
			ErrorMessage: "plugin does not implement export",
		}, nil
	}

	var collector CollectorConfig
	if req.GetCollector() != nil {
		collector = CollectorConfig{
			Endpoint:  req.GetCollector().GetEndpoint(),
			AuthToken: req.GetCollector().GetAuthToken(),
		}
	}

	resp, err := exporter.Export(ctx, &ExportRequest{
		Collector: collector,
	})
	if err != nil {
		return nil, err
	}

	return &proto.ExportResponse{
		Success:       resp.Success,
		ExportedCount: resp.ExportedCount,
		FailedCount:   resp.FailedCount,
		ErrorMessage:  resp.ErrorMessage,
	}, nil
}

func internalResultToProto(r Result) proto.Result {
	switch r {
	case ResultPassed:
		return proto.Result_RESULT_PASSED
	case ResultFailed:
		return proto.Result_RESULT_FAILED
	case ResultSkipped:
		return proto.Result_RESULT_SKIPPED
	case ResultError:
		return proto.Result_RESULT_ERROR
	default:
		return proto.Result_RESULT_UNSPECIFIED
	}
}

func internalConfidenceToProto(c ConfidenceLevel) proto.ConfidenceLevel {
	switch c {
	case ConfidenceLevelUndetermined:
		return proto.ConfidenceLevel_CONFIDENCE_LEVEL_UNDETERMINED
	case ConfidenceLevelLow:
		return proto.ConfidenceLevel_CONFIDENCE_LEVEL_LOW
	case ConfidenceLevelMedium:
		return proto.ConfidenceLevel_CONFIDENCE_LEVEL_MEDIUM
	case ConfidenceLevelHigh:
		return proto.ConfidenceLevel_CONFIDENCE_LEVEL_HIGH
	default:
		return proto.ConfidenceLevel_CONFIDENCE_LEVEL_NOT_SET
	}
}
