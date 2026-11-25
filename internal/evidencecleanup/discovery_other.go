// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package evidencecleanup

import (
	"fmt"
	"os"
)

func openDiscoveryWorkspace(string) (*os.File, func() error, error) {
	return nil, nil, fmt.Errorf("confined evaluation discovery requires Linux or macOS")
}

func openDiscoveryDirectory(*os.File, string) (*os.File, error) {
	return nil, fmt.Errorf("confined evaluation discovery requires Linux or macOS")
}

func inspectDiscoveryEntry(*os.File, string) (discoveryEntry, error) {
	return discoveryEntry{}, fmt.Errorf("confined evaluation discovery requires Linux or macOS")
}
