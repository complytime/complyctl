// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/complytime/complyctl/cmd/openscap-plugin/server"
	"github.com/complytime/complyctl/pkg/provider"
)

func main() {
	openSCAPPlugin := server.New()
	provider.Serve(openSCAPPlugin)
}
