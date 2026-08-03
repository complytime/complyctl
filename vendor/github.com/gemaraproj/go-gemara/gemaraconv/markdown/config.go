package markdown

import "github.com/gemaraproj/go-gemara"

// Config holds Markdown rendering options for CatalogToMarkdown.
type Config struct {
	TOC                 bool
	LineEnding          string
	Metadata            bool
	ApplicabilityMatrix bool
	LexiconAutolink     bool
	InlineLexicon       []InlineLexiconTerm
	Fetcher             gemara.Fetcher
}
