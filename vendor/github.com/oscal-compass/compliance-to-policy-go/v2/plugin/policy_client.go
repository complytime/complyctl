/*
 Copyright 2024 The OSCAL Compass Authors
 SPDX-License-Identifier: Apache-2.0
*/

package plugin

import (
	"context"

	"github.com/oscal-compass/compliance-to-policy-go/v2/api/proto"
	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
)

// Client must return an implementation of the corresponding interface that communicates over an RPC client.
var _ policy.Provider = (*pvpClient)(nil)

type pvpClient struct {
	client proto.PolicyEngineClient
}

func (pvp *pvpClient) Configure(ctx context.Context, configuration map[string]string) error {
	request := proto.ConfigureRequest{
		Settings: configuration,
	}
	_, err := pvp.client.Configure(ctx, &request)
	if err != nil {
		return err
	}
	return nil
}

func (pvp *pvpClient) Generate(ctx context.Context, p policy.Policy) error {
	request := PolicyToProto(p)
	_, err := pvp.client.Generate(ctx, request)
	if err != nil {
		return err
	}
	return nil
}

func (pvp *pvpClient) GetResults(ctx context.Context, p policy.Policy) (policy.PVPResult, error) {
	request := PolicyToProto(p)
	resp, err := pvp.client.GetResults(ctx, request)
	if err != nil {
		return policy.PVPResult{}, err
	}
	pvpResult := NewResultFromProto(resp.Result)
	return pvpResult, nil
}
