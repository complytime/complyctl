// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package evidencecleanup

import (
	"fmt"
	"os"
)

func openInventoryRegular(_ *os.File, _ string) (*os.File, error) {
	return nil, fmt.Errorf("confined report inventory unavailable on this platform")
}
