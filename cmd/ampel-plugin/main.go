package main

import (
	"github.com/complytime/complyctl/cmd/ampel-plugin/server"
	"github.com/complytime/complyctl/pkg/provider"
)

func main() {
	ampelPlugin := server.New()
	provider.Serve(ampelPlugin)
}
