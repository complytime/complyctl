package markdown

import (
	"context"
	"fmt"
	"strings"

	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/fetcher"
	"github.com/gemaraproj/go-gemara/internal/codec"
)

// resolveLexiconURL returns the source string (URL or local path) for the lexicon artifact.
// Precedence: metadata.mapping-references entry whose id matches metadata.lexicon.reference-id;
// else metadata.lexicon.remarks if it is a fetchable URL (must use http://, https://, or file://).
func resolveLexiconURL(meta gemara.Metadata) (string, error) {
	if meta.Lexicon == nil {
		return "", fmt.Errorf("lexicon mapping is nil")
	}
	refID := strings.TrimSpace(meta.Lexicon.ReferenceId)
	for _, mappingRef := range meta.MappingReferences {
		if mappingRef.Id == refID && refID != "" {
			mappedURL := strings.TrimSpace(mappingRef.Url)
			if mappedURL == "" {
				return "", fmt.Errorf("mapping-references entry %q has empty url", refID)
			}
			return mappedURL, nil
		}
	}
	remarks := strings.TrimSpace(meta.Lexicon.Remarks)
	if isLexiconFetchURL(remarks) {
		return remarks, nil
	}
	if refID == "" {
		return "", fmt.Errorf("metadata.lexicon has empty reference-id and remarks is not a fetchable URL")
	}
	return "", fmt.Errorf("no mapping-references entry with id %q for metadata.lexicon", refID)
}

// loadLexiconFromURI fetches a Lexicon from an http(s):// URL, a file:// URI,
// or a local file path, and returns normalized entries.
func loadLexiconFromURI(ctx context.Context, uri string) ([]lexiconEntry, error) {
	doc, err := gemara.Load[gemara.Lexicon](ctx, &fetcher.URI{}, uri)
	if err != nil {
		return nil, fmt.Errorf("load lexicon: %w", err)
	}
	return parseLexiconDocument(doc)
}

func parseLexiconDocument(doc *gemara.Lexicon) ([]lexiconEntry, error) {
	if err := validateLexicon(doc); err != nil {
		return nil, err
	}
	return normalizeLexicon(doc)
}

// parseLexiconYAML decodes bytes as a single Gemara Lexicon document and returns normalized entries.
func parseLexiconYAML(data []byte) ([]lexiconEntry, error) {
	var doc gemara.Lexicon
	if err := codec.UnmarshalYAML(data, &doc); err != nil {
		return nil, fmt.Errorf("decode lexicon YAML: %w", err)
	}
	return parseLexiconDocument(&doc)
}

func validateLexicon(lexDoc *gemara.Lexicon) error {
	if lexDoc == nil {
		return fmt.Errorf("lexicon is nil")
	}
	if len(lexDoc.Terms) == 0 {
		return fmt.Errorf("lexicon has no terms")
	}
	for termIdx, term := range lexDoc.Terms {
		if strings.TrimSpace(term.Title) == "" && strings.TrimSpace(term.Id) == "" {
			return fmt.Errorf("lexicon terms[%d]: title and id are both empty", termIdx)
		}
		if strings.TrimSpace(term.Definition) == "" {
			return fmt.Errorf("lexicon terms[%d]: definition is empty", termIdx)
		}
		for refIdx, refLine := range term.References {
			if strings.TrimSpace(refLine.Citation) == "" {
				return fmt.Errorf("lexicon terms[%d].references[%d]: citation is empty", termIdx, refIdx)
			}
		}
	}
	return nil
}

func normalizeLexicon(lexDoc *gemara.Lexicon) ([]lexiconEntry, error) {
	seen := make(map[string]struct{})
	out := make([]lexiconEntry, 0, len(lexDoc.Terms))
	for termIdx, term := range lexDoc.Terms {
		canonical := strings.TrimSpace(term.Title)
		if canonical == "" {
			canonical = strings.TrimSpace(term.Id)
		}
		if err := markGemaraCanonicalSeen(seen, canonical); err != nil {
			return nil, err
		}

		syns, err := trimSynonyms(term.Synonyms, termIdx, "lexicon terms")
		if err != nil {
			return nil, err
		}

		out = append(out, lexiconEntry{
			Canonical:  canonical,
			Definition: strings.TrimSpace(term.Definition),
			Synonyms:   syns,
			Refs:       refLinesFromGemara(term.References),
		})
	}
	return out, nil
}
