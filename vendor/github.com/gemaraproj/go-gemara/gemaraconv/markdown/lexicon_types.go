package markdown

// lexiconRefLine is one reference for glossary rendering.
type lexiconRefLine struct {
	Citation string
	URL      string
}

// lexiconEntry is normalized lexicon data for autolinking and the glossary.
type lexiconEntry struct {
	Canonical  string
	Definition string
	Synonyms   []string
	Refs       []lexiconRefLine
}
