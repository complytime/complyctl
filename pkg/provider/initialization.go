// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"fmt"
	"os/exec"

	pluginv2 "github.com/complytime/complyctl/api/plugin"
	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

func NewClient(executablePath string, logger hclog.Logger) (*Client, error) {
	config := &goplugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Logger:           logger.Named(executablePath),
		Managed:          true,
		AutoMTLS:         true,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Cmd:              exec.Command(executablePath), /* #nosec G204 — path validated by discovery */
		Plugins:          SupportedProviders,
	}

	client := goplugin.NewClient(config)

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to connect to provider %s: %w", executablePath, err)
	}

	raw, err := rpcClient.Dispense("evaluator")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense evaluator from provider %s: %w", executablePath, err)
	}

	grpcClient, ok := raw.(pluginv2.PluginClient)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("provider %s does not implement evaluator interface", executablePath)
	}

	return &Client{
		executablePath: executablePath,
		gopluginClient: client,
		grpcClient:     grpcClient,
	}, nil
}
