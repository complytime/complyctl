/*
 Copyright 2024 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package plugin

import (
	"context"

	"google.golang.org/grpc/status"

	"google.golang.org/grpc/codes"

	"github.com/oscal-compass/compliance-to-policy-go/v2/api/proto"
	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
)

// Plugin must return an RPC server for this plugin type.
var _ proto.PolicyEngineServer = (*pvpService)(nil)

type pvpService struct {
	proto.UnimplementedPolicyEngineServer
	Impl policy.Provider
}

func FromPVP(pe policy.Provider) proto.PolicyEngineServer {
	return &pvpService{
		Impl: pe,
	}
}

func (p *pvpService) Configure(ctx context.Context, request *proto.ConfigureRequest) (*proto.ConfigureResponse, error) {
	if err := p.Impl.Configure(ctx, request.Settings); err != nil {
		return &proto.ConfigureResponse{}, status.Error(codes.Internal, err.Error())
	}

	// policy.Provider.Configure currently only returns an error, so using an empty proto.ConifgureResponse
	return &proto.ConfigureResponse{}, nil
}

func (p *pvpService) Generate(ctx context.Context, request *proto.PolicyRequest) (*proto.GenerateResponse, error) {
	policy := NewPolicyFromProto(request)
	if err := p.Impl.Generate(ctx, policy); err != nil {
		return &proto.GenerateResponse{}, status.Error(codes.Internal, err.Error())
	}

	// policy.Provider.Generate currently only returns an error, so using an empty proto.GenerateResponse
	return &proto.GenerateResponse{}, nil
}

func (p *pvpService) GetResults(ctx context.Context, request *proto.PolicyRequest) (*proto.ResultsResponse, error) {
	policy := NewPolicyFromProto(request)
	result, err := p.Impl.GetResults(ctx, policy)
	if err != nil {
		return &proto.ResultsResponse{}, status.Error(codes.Internal, err.Error())
	}
	return &proto.ResultsResponse{Result: ResultsToProto(result)}, nil
}
