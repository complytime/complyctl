// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package evidencecleanup

import (
	"fmt"
	"os"
)

func walkPreflightParent(_ *os.File, _ string) (*os.File, error) {
	return nil, fmt.Errorf("confined group preflight requires Linux or macOS")
}

func checkPreflightAccess(_ *os.File) error {
	return fmt.Errorf("confined group preflight requires Linux or macOS")
}
