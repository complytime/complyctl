// SPDX-License-Identifier: Apache-2.0

package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
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
// When BasePath is set, relative file:// URIs and bare relative paths are
// resolved against it instead of the process working directory. Absolute
// paths are unaffected. This lets callers anchor resolution to the source
// artifact's directory rather than relying on the working directory.
//
// For HTTP(S) sources it delegates to [HTTP]; see that type's
// documentation for security considerations.
type URI struct {
	Client *http.Client

	// BasePath is the directory used to resolve relative file:// URIs and
	// bare relative paths. When empty, paths resolve against the process
	// working directory (the default, backward-compatible behavior).
	BasePath string
}

// schemePrefix matches a leading "<scheme>://" per RFC 3986 scheme syntax.
var schemePrefix = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*://`)

func (u *URI) Fetch(ctx context.Context, source string) (io.ReadCloser, error) {
	switch {
	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		return (&HTTP{Client: u.Client}).Fetch(ctx, source)
	case strings.HasPrefix(source, "file://"):
		path, err := url.PathUnescape(strings.TrimPrefix(source, "file://"))
		if err != nil {
			return nil, fmt.Errorf("invalid file URI %q: %w", source, err)
		}
		return (&File{}).Fetch(ctx, u.resolveLocal(path))
	case schemePrefix.MatchString(source):
		return nil, fmt.Errorf("unsupported URI scheme in %q", source)
	default:
		return (&File{}).Fetch(ctx, u.resolveLocal(source))
	}
}

// resolveLocal joins path with BasePath when the path is relative and
// BasePath is set. Absolute paths are returned unchanged.
func (u *URI) resolveLocal(path string) string {
	if u.BasePath != "" && !filepath.IsAbs(path) {
		return filepath.Join(u.BasePath, path)
	}
	return path
}
