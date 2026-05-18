// SPDX-License-Identifier: Apache-2.0

package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// URI routes to File or HTTP based on the source string.
//
// Recognized forms:
//   - http:// or https:// URLs are fetched via [HTTP].
//   - file:// URIs are fetched via [File].
//   - Any other input without a scheme (absolute or relative local paths,
//     including Windows drive paths) is treated as a local file path.
//   - Inputs with any other <scheme>:// prefix return an unsupported-scheme error.
//
// For HTTP(S) sources it delegates to [HTTP]; see that type's
// documentation for security considerations.
type URI struct {
	Client *http.Client
}

// schemePrefix matches a leading "<scheme>://" per RFC 3986 scheme syntax.
var schemePrefix = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*://`)

func (u *URI) Fetch(ctx context.Context, source string) (io.ReadCloser, error) {
	switch {
	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		return (&HTTP{Client: u.Client}).Fetch(ctx, source)
	case strings.HasPrefix(source, "file://"):
		parsed, err := url.Parse(source)
		if err != nil {
			return nil, fmt.Errorf("invalid file URI %q: %w", source, err)
		}
		return (&File{}).Fetch(ctx, parsed.Path)
	case schemePrefix.MatchString(source):
		return nil, fmt.Errorf("unsupported URI scheme in %q", source)
	default:
		return (&File{}).Fetch(ctx, source)
	}
}
