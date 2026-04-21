// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"os"

	proto "github.com/complytime/complyctl/api/plugin"
	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

// Handshake is the shared config that providers must match to connect.
// Wire values are frozen — do not change MagicCookieKey, MagicCookieValue, or ProtocolVersion.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion: 1,
	MagicCookieKey:  "COMPLYCTL_PLUGIN",
	// DO NOT CHANGE - UUID
	MagicCookieValue: "ddff478d-578e-4d9d-8253-35e8ebf548d2",
}

// SupportedProviders is the provider type map used when creating go-plugin clients.
var SupportedProviders = map[string]goplugin.Plugin{
	"evaluator": &GRPCEvaluatorPlugin{},
}

// GRPCEvaluatorPlugin implements hashicorp/go-plugin.GRPCPlugin for the
// evaluator service.
type GRPCEvaluatorPlugin struct {
	goplugin.Plugin
	Impl Provider
}

func (p *GRPCEvaluatorPlugin) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterPluginServer(s, &grpcServer{impl: p.Impl})
	return nil
}

func (p *GRPCEvaluatorPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return proto.NewPluginClient(c), nil
}

// Serve starts the provider process. Provider authors call this from main().
// A JSON logger is created at Trace level so every message reaches the
// client; the client-side logger level controls what is actually written.
func Serve(impl Provider) {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	})
	hclog.SetDefault(logger)

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]goplugin.Plugin{
			"evaluator": &GRPCEvaluatorPlugin{Impl: impl},
		},
		Logger:     logger,
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
