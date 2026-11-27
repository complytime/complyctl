// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package workspacecoord

import (
	"errors"
	"fmt"
	"os"
	"runtime"
)

func openDirectory(string) (*os.File, error) {
	return nil, platformError()
}

func lockDirectory(*os.File, Mode) error {
	return platformError()
}

func checkFilesystem(*os.File) error {
	return platformError()
}

func platformError() error {
	return fmt.Errorf("workspace directory coordination on %s: %w; use Linux or macOS",
		runtime.GOOS, errors.ErrUnsupported)
}
