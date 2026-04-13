package gemaraconv

import (
	"context"
	"fmt"

	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/gemaraconv/markdown"
)

// InlineLexiconTerm is an alias for the markdown subpackage type so callers can use gemaraconv.InlineLexiconTerm.
type InlineLexiconTerm = markdown.InlineLexiconTerm

// CatalogToMarkdown renders a ControlCatalog as Markdown using embedded templates.
// Only controls whose state is LifecycleActive are included (TOC, body, and summary counts).
func CatalogToMarkdown(ctx context.Context, catalog *gemara.ControlCatalog, opts ...MarkdownOption) ([]byte, error) {
	if catalog == nil {
		return nil, fmt.Errorf("catalog is nil")
	}

	o := defaultMarkdownOpts()
	o.apply(opts...)

	cfg := markdown.Config{
		TOC:                 o.toc,
		LineEnding:          o.lineEnding,
		Metadata:            o.metadata,
		ApplicabilityMatrix: o.applicabilityMatrix,
		LexiconAutolink:     o.lexiconAutolink,
		InlineLexicon:       o.inlineLexicon,
	}
	return markdown.CatalogToMarkdown(ctx, catalog, cfg)
}
