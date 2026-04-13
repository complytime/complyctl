package markdown

import (
	"fmt"
	"strings"

	"github.com/gemaraproj/go-gemara"
)

// isLexiconFetchURL reports whether raw uses a scheme loaders accept for lexicon YAML (https, http, file).
func isLexiconFetchURL(raw string) bool {
	return strings.HasPrefix(raw, "https://") ||
		strings.HasPrefix(raw, "http://") ||
		strings.HasPrefix(raw, "file://")
}

func refLinesFromGemara(refs []gemara.LexiconReference) []lexiconRefLine {
	out := make([]lexiconRefLine, len(refs))
	for refIdx, refLine := range refs {
		out[refIdx] = lexiconRefLine{
			Citation: strings.TrimSpace(refLine.Citation),
			URL:      strings.TrimSpace(refLine.Url),
		}
	}
	return out
}

func refLinesFromStrings(refs []string) []lexiconRefLine {
	var out []lexiconRefLine
	for _, refStr := range refs {
		refStr = strings.TrimSpace(refStr)
		if refStr == "" {
			continue
		}
		if strings.HasPrefix(refStr, "http://") || strings.HasPrefix(refStr, "https://") {
			out = append(out, lexiconRefLine{Citation: refStr, URL: refStr})
		} else {
			out = append(out, lexiconRefLine{Citation: refStr})
		}
	}
	return out
}

// trimSynonyms returns trimmed non-empty synonyms or an error.
// scope is the message prefix, e.g. "lexicon terms" or "inline lexicon".
func trimSynonyms(synonyms []string, termIndex int, scope string) ([]string, error) {
	out := make([]string, 0, len(synonyms))
	for _, synonym := range synonyms {
		trimmed := strings.TrimSpace(synonym)
		if trimmed == "" {
			return nil, fmt.Errorf("%s[%d]: empty synonym", scope, termIndex)
		}
		out = append(out, trimmed)
	}
	return out, nil
}

func markGemaraCanonicalSeen(seen map[string]struct{}, canonical string) error {
	key := strings.ToLower(canonical)
	if _, already := seen[key]; already {
		return fmt.Errorf("duplicate lexicon canonical %q", canonical)
	}
	seen[key] = struct{}{}
	return nil
}

func markInlineLexiconTermSeen(seen map[string]struct{}, canonical string) error {
	key := strings.ToLower(canonical)
	if _, already := seen[key]; already {
		return fmt.Errorf("duplicate inline lexicon term %q", canonical)
	}
	seen[key] = struct{}{}
	return nil
}
