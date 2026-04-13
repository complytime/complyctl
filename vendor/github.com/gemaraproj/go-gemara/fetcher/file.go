// SPDX-License-Identifier: Apache-2.0

package fetcher

import (
	"context"
	"fmt"
	"io"
	"os"
)

// File reads from local filesystem paths.
type File struct{}

func (f *File) Fetch(_ context.Context, source string) (io.ReadCloser, error) {
	file, err := os.Open(source)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	return file, nil
}
