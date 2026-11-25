// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"errors"
	"fmt"
	"os"
)

func validateInventoryRegular(fd int, name string) (*os.File, error) {
	file, err := discoveryFile(fd, name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if !info.Mode().IsRegular() {
		return nil, errors.Join(fmt.Errorf("not a regular report/evidence file"), file.Close())
	}
	return file, nil
}
