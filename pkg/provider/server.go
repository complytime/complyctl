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
		GlobalVariables:       req.GetGlobalVariables(),
		Configuration:         configs,
		TargetVariables:       req.GetTargetVariables(),
		ComplypackContentPath: req.GetComplypackContentPath(),
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
			RequirementId:  a.RequirementID,
			Steps:          protoSteps,
			Message:        a.Message,
			Confidence:     internalConfidenceToProto(a.Confidence),
			Evidence:       internalEvidenceToProto(a.Evidence),
			Recommendation: a.Recommendation,
		})
	}

	return &proto.ScanResponse{
		Assessments: protoAssessments,
		Errors:      resp.Errors,
	}, nil
}

func internalEvidenceToProto(evidence []Evidence) []*proto.Evidence {
	if len(evidence) == 0 {
		return nil
	}
	pe := make([]*proto.Evidence, len(evidence))
	for i, ev := range evidence {
		pe[i] = &proto.Evidence{
			Id:          ev.ID,
			Type:        ev.Type,
			Description: ev.Description,
			Payload:     ev.Payload,
			CollectedAt: ev.CollectedAt,
		}
	}
	return pe
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
