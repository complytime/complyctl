package markdown

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"strings"
	"text/template"
	"unicode"

	"github.com/gemaraproj/go-gemara"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// CatalogToMarkdown renders a ControlCatalog as Markdown using embedded templates.
// Only controls whose state is LifecycleActive are included (TOC, body, and summary counts).
func CatalogToMarkdown(ctx context.Context, catalog gemara.ControlCatalog, cfg Config) ([]byte, error) {
	lineEnding := cfg.LineEnding
	if lineEnding == "" {
		lineEnding = "\n"
	}
	cfg.LineEnding = lineEnding

	var lexEntries []lexiconEntry
	switch {
	case cfg.LexiconAutolink && catalog.Metadata.Lexicon != nil:
		lexiconURI, err := resolveLexiconURL(catalog.Metadata)
		if err != nil {
			return nil, fmt.Errorf("lexicon: resolve URL: %w", err)
		}
		loaded, err := loadLexiconFromURI(ctx, lexiconURI)
		if err != nil {
			return nil, fmt.Errorf("lexicon: %w", err)
		}
		lexEntries = loaded
	case len(cfg.InlineLexicon) > 0:
		loaded, err := normalizeInlineLexicon(cfg.InlineLexicon)
		if err != nil {
			return nil, fmt.Errorf("lexicon: %w", err)
		}
		lexEntries = loaded
	}

	lexGlossary := buildLexiconGlossaryView(lexEntries)
	view := buildMarkdownCatalogView(catalog, cfg, lexGlossary)

	linker := newLexiconLinker(lexEntries)
	t, err := template.New("").Funcs(markdownFuncMap(linker)).ParseFS(templatesFS, "templates/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse markdown templates: %w", err)
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "catalog", view); err != nil {
		return nil, fmt.Errorf("execute markdown template: %w", err)
	}

	text := collapseExtraNewlines(buf.String())
	out := []byte(text)
	if lineEnding != "\n" {
		out = []byte(strings.ReplaceAll(string(out), "\n", lineEnding))
	}
	return out, nil
}

// collapseExtraNewlines replaces every run of three or more consecutive newlines
// with exactly two newlines, repeating until stable (one blank line between blocks).
func collapseExtraNewlines(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

func markdownFuncMap(lexiconLink func(string) string) template.FuncMap {
	return template.FuncMap{
		"lexiconLink":  lexiconLink,
		"anchor":       Anchor,
		"lifecycle":    func(l gemara.Lifecycle) string { return l.String() },
		"isRetired":    func(l gemara.Lifecycle) bool { return l == gemara.LifecycleRetired },
		"artifactType": func(a gemara.ArtifactType) string { return a.String() },
		"entityType":   func(e gemara.EntityType) string { return e.String() },
		"datetime":     func(d gemara.Datetime) string { return string(d) },
		"joinStrings":  func(ss []string, sep string) string { return strings.Join(ss, sep) },
		"joinArtifactEntries": func(entries []gemara.ArtifactMapping, sep string) string {
			if len(entries) == 0 {
				return ""
			}
			parts := make([]string, 0, len(entries))
			for _, e := range entries {
				s := e.ReferenceId
				if e.Remarks != "" {
					s += " — " + e.Remarks
				}
				parts = append(parts, s)
			}
			return strings.Join(parts, sep)
		},
		"artifactMapping": func(m gemara.ArtifactMapping) string {
			s := m.ReferenceId
			if m.Remarks != "" {
				s += " — " + m.Remarks
			}
			return s
		},
	}
}

// Anchor returns a GitHub-style fragment id for heading text (lowercase, hyphen-separated).
func Anchor(s string) string {
	if s == "" {
		return "section"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if b.Len() > 0 && !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "section"
	}
	return out
}
