// SPDX-License-Identifier: Apache-2.0

package fetcher

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// DefaultHTTPTimeout is the timeout applied when no custom [http.Client]
// is provided.
const DefaultHTTPTimeout = 30 * time.Second

// HTTP reads from HTTP/HTTPS URLs.
//
// If Client is nil, a default client with [DefaultHTTPTimeout] is used.
// Callers may further bound individual requests via a deadline on the
// provided [context.Context].
//
// HTTP performs no URL filtering; it will follow any URL it receives,
// including internal or private network addresses. Applications that
// accept URLs from untrusted input should supply a Client whose
// [http.Transport] rejects private/loopback addresses at dial time;
// see [ExampleURI_ssrfSafe].
type HTTP struct {
	Client *http.Client
}

func (h *HTTP) httpClient() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: DefaultHTTPTimeout}
}

func (h *HTTP) Fetch(ctx context.Context, source string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := h.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("failed to close response body: %v", err)
			}
		}()
		return nil, fmt.Errorf("failed to fetch URL; response status: %v", resp.Status)
	}
	return resp.Body, nil
}
