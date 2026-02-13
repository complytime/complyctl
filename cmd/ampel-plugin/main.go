package main

import (
	"os"

	"github.com/hashicorp/go-hclog"
	hplugin "github.com/hashicorp/go-plugin"
	"github.com/oscal-compass/compliance-to-policy-go/v2/plugin"

	"github.com/complytime/complyctl/cmd/ampel-plugin/server"
)

var logger hclog.Logger

func init() {
	logger = hclog.New(&hclog.LoggerOptions{
		Name:       "ampel-plugin",
		Level:      hclog.Debug,
		Output:     os.Stderr,
		JSONFormat: true,
	})
	hclog.SetDefault(logger)
}

func main() {
	ampelPlugin := server.New()
	pluginByType := map[string]hplugin.Plugin{
		plugin.PVPPluginName: &plugin.PVPPlugin{Impl: ampelPlugin},
	}
	config := plugin.ServeConfig{
		PluginSet: pluginByType,
		Logger:    logger,
	}
	plugin.Register(config)
}
