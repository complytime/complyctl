package gemaraconv

import "github.com/gemaraproj/go-gemara"

type generateOpts struct {
	version       string
	imports       map[string]string
	canonicalHref string
	controlHREF   string
}

func (g *generateOpts) completeFromGuidance(doc *gemara.GuidanceCatalog) {
	if g.version == "" {
		g.version = doc.Metadata.Version
	}
	if g.imports == nil {
		g.imports = make(map[string]string)
		for _, mappingRef := range doc.Metadata.MappingReferences {
			g.imports[mappingRef.Id] = mappingRef.Url
		}
	}
}

func (g *generateOpts) completeFromCatalog(catalog *gemara.ControlCatalog) {
	if g.version == "" {
		g.version = catalog.Metadata.Version
	}
}

// GenerateOption defines an option to tune the behavior of the OSCAL
// generation functions for both Layer 1 (GuidanceCatalog) and Layer 2 (ControlCatalog).
type GenerateOption func(opts *generateOpts)

// WithVersion is a GenerateOption that sets the version of the OSCAL Document. If set,
// this will be used instead of the version in GuidanceCatalog.
func WithVersion(version string) GenerateOption {
	return func(opts *generateOpts) {
		opts.version = version
	}
}

// WithOSCALImports is a GenerateOption that provides the `href` to guidance document mappings in OSCAL
// by mapping unique identifier. If unset, the mapping URL of the guidance document will be used.
func WithOSCALImports(imports map[string]string) GenerateOption {
	return func(opts *generateOpts) {
		opts.imports = imports
	}
}

// WithCanonicalHrefFormat is a GenerateOption that provides an `href` format string
// for the canonical version of the guidance document. If set, this will be added as a
// link in the mapping.cue with the rel="canonical" attribute. Ex - https://myguidance.org/versions/%s
func WithCanonicalHrefFormat(canonicalHref string) GenerateOption {
	return func(opts *generateOpts) {
		opts.canonicalHref = canonicalHref
	}
}

// WithControlHref is a GenerateOption that provides a URL template for linking to controls
// in Catalog conversion. Uses format: controlHREF(version, controlID)
// Example: "https://baseline.openssf.org/versions/%s#%s"
func WithControlHref(controlHref string) GenerateOption {
	return func(opts *generateOpts) {
		opts.controlHREF = controlHref
	}
}

type markdownOpts struct {
	toc                 bool
	lineEnding          string
	metadata            bool
	applicabilityMatrix bool
	lexiconAutolink     bool
	inlineLexicon       []InlineLexiconTerm
}

func defaultMarkdownOpts() markdownOpts {
	return markdownOpts{toc: true, lineEnding: "\n", metadata: true, applicabilityMatrix: false}
}

func (o *markdownOpts) apply(opts ...MarkdownOption) {
	for _, opt := range opts {
		opt(o)
	}
	if o.lineEnding == "" {
		o.lineEnding = "\n"
	}
}

// MarkdownOption configures ControlCatalog Markdown export.
type MarkdownOption func(*markdownOpts)

// WithTOC sets whether a table of contents is emitted (default true).
func WithTOC(toc bool) MarkdownOption {
	return func(o *markdownOpts) {
		o.toc = toc
	}
}

// WithLineEnding sets the line ending sequence (default "\n"). Use "\r\n" for Windows-style output.
func WithLineEnding(s string) MarkdownOption {
	return func(o *markdownOpts) {
		if s != "" {
			o.lineEnding = s
		}
	}
}

// WithMetadata sets whether the metadata section is emitted (default true).
func WithMetadata(enabled bool) MarkdownOption {
	return func(o *markdownOpts) {
		o.metadata = enabled
	}
}

// WithApplicabilityMatrix sets whether an assessment-requirement × applicability matrix is emitted (default false).
func WithApplicabilityMatrix(enabled bool) MarkdownOption {
	return func(o *markdownOpts) {
		o.applicabilityMatrix = enabled
	}
}

// WithLexiconAutolink enables loading metadata.lexicon from mapping-references (or remarks URL),
// strict Gemara Lexicon YAML, term autolinking in prose, and a trailing glossary (default false).
// When enabled and metadata.lexicon is set, this takes precedence over WithInlineLexicon.
func WithLexiconAutolink(enabled bool) MarkdownOption {
	return func(o *markdownOpts) {
		o.lexiconAutolink = enabled
	}
}

// WithInlineLexicon supplies list-shaped lexicon entries (term / definition / synonyms / string references)
// for autolinking and the trailing glossary without network I/O. Used when the catalog does not
// reference a remote Gemara Lexicon document.
func WithInlineLexicon(terms []InlineLexiconTerm) MarkdownOption {
	return func(o *markdownOpts) {
		o.inlineLexicon = terms
	}
}
