// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/complytime/complyctl/cmd/complyctl/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	fmt.Println("complyctl is running")
	complyctl := cli.New()
	if err := complyctl.ExecuteContext(ctx); err != nil {
		cli.Error(fmt.Sprintf("error running complyctl: %v", err))
		os.Exit(1)
	}
}
