package markdown

// Config holds Markdown rendering options for CatalogToMarkdown.
type Config struct {
	TOC                 bool
	LineEnding          string
	Metadata            bool
	ApplicabilityMatrix bool
	LexiconAutolink     bool
	InlineLexicon       []InlineLexiconTerm
}
