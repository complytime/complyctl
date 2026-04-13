// SPDX-License-Identifier: Apache-2.0

package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// URI routes to File or HTTP based on the URI scheme.
// Supported schemes: file://, http://, https://.
//
// For HTTP(S) sources it delegates to [HTTP]; see that type's
// documentation for security considerations.
type URI struct {
	Client *http.Client
}

func (u *URI) Fetch(ctx context.Context, source string) (io.ReadCloser, error) {
	parsed, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("invalid URI %q: %w", source, err)
	}
	switch parsed.Scheme {
	case "file":
		return (&File{}).Fetch(ctx, parsed.Path)
	case "http", "https":
		return (&HTTP{Client: u.Client}).Fetch(ctx, source)
	default:
		return nil, fmt.Errorf("unsupported URI scheme in %q", source)
	}
}
