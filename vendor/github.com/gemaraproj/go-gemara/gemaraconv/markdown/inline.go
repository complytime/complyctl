package markdown

import (
	"fmt"
	"strings"
)

// InlineLexiconTerm carries list-shaped lexicon rows (e.g. OSPS baseline/lexicon.yaml:
// term, definition, synonyms, string references) for Markdown autolink + glossary without
// fetching a Gemara Lexicon YAML document.
type InlineLexiconTerm struct {
	Term       string
	Definition string
	Synonyms   []string
	References []string
}

func normalizeInlineLexicon(terms []InlineLexiconTerm) ([]lexiconEntry, error) {
	if len(terms) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	out := make([]lexiconEntry, 0, len(terms))
	for rowIdx, row := range terms {
		canonical := strings.TrimSpace(row.Term)
		if canonical == "" {
			return nil, fmt.Errorf("inline lexicon[%d]: empty term", rowIdx)
		}
		def := strings.TrimSpace(row.Definition)
		if def == "" {
			return nil, fmt.Errorf("inline lexicon[%d]: empty definition", rowIdx)
		}
		if err := markInlineLexiconTermSeen(seen, canonical); err != nil {
			return nil, err
		}

		syns, err := trimSynonyms(row.Synonyms, rowIdx, "inline lexicon")
		if err != nil {
			return nil, err
		}

		out = append(out, lexiconEntry{
			Canonical:  canonical,
			Definition: def,
			Synonyms:   syns,
			Refs:       refLinesFromStrings(row.References),
		})
	}
	return out, nil
}
