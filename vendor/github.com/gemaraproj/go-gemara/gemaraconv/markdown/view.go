package markdown

import (
	"sort"
	"strings"

	"github.com/gemaraproj/go-gemara"
)

// sanitizeMarkdownTableCell flattens whitespace and escapes '|' so pipe-table rows
// stay on one line (YAML block scalars often end with a newline).
func sanitizeMarkdownTableCell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(s, "|", `\|`)
}

const ungroupedSectionTitle = "Ungrouped"

// markdownCatalogView is the template root: deterministic ordering and explicit Ungrouped bucket.
type markdownCatalogView struct {
	Title        string
	Metadata     gemara.Metadata
	ShowMetadata bool
	// ShowApplicabilityMatrix is true when WithApplicabilityMatrix is set and the matrix can be built.
	ShowApplicabilityMatrix    bool
	ApplicabilityMatrixColumns []markdownApplicabilityColumn
	ApplicabilityMatrixRows    []markdownApplicabilityMatrixRow
	Extends                    []gemara.ArtifactMapping
	Imports                    []gemara.MultiEntryMapping
	TOC                        bool
	LineEnding                 string
	Groups                     []markdownGroupView
	TOCItems                   []markdownTOCItem
	NumControls                int
	NumARs                     int
	// LexiconGlossary is non-empty when lexicon autolink loaded a valid document.
	LexiconGlossary []markdownLexiconGlossaryEntry
}

// markdownLexiconGlossaryEntry is one term in the rendered ## Lexicon section.
type markdownLexiconGlossaryEntry struct {
	Canonical  string
	Definition string
	// RefTarget is the link target for reference-style definitions (includes leading '#').
	RefTarget string
	Refs      []lexiconRefLine
}

// markdownApplicabilityColumn is one applicability dimension in the matrix header.
type markdownApplicabilityColumn struct {
	ID    string
	Label string
}

// markdownApplicabilityMatrixRow is one assessment-requirement row; Cells align with ApplicabilityMatrixColumns.
type markdownApplicabilityMatrixRow struct {
	RequirementID string
	Cells         []string // "X" or empty
}

// markdownTOCItem is one line in the table of contents (group or control).
type markdownTOCItem struct {
	Label   string
	Anchor  string
	Indent  int // 0 = group, 1 = control under group
	Control bool
}

type markdownGroupView struct {
	ID          string
	Title       string
	Description string
	Anchor      string
	IsUngrouped bool
	Controls    []gemara.Control
}

func buildLexiconGlossaryView(entries []lexiconEntry) []markdownLexiconGlossaryEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]markdownLexiconGlossaryEntry, len(entries))
	for entryIdx, entry := range entries {
		out[entryIdx] = markdownLexiconGlossaryEntry{
			Canonical:  entry.Canonical,
			Definition: entry.Definition,
			RefTarget:  lexiconRefSlug(entry.Canonical),
			Refs:       append([]lexiconRefLine(nil), entry.Refs...),
		}
	}
	return out
}

func buildMarkdownCatalogView(catalog *gemara.ControlCatalog, cfg Config, lexGlossary []markdownLexiconGlossaryEntry) markdownCatalogView {
	if catalog == nil {
		return markdownCatalogView{LineEnding: cfg.LineEnding, LexiconGlossary: lexGlossary}
	}

	known := make(map[string]struct{}, len(catalog.Groups))
	for _, g := range catalog.Groups {
		known[g.Id] = struct{}{}
	}

	byGroup := make(map[string][]gemara.Control)
	var orphans []gemara.Control
	numARs := 0
	numControlsShown := 0
	for _, c := range catalog.Controls {
		if c.State != gemara.LifecycleActive {
			continue
		}
		numControlsShown++
		numARs += len(c.AssessmentRequirements)
		if _, ok := known[c.Group]; ok {
			byGroup[c.Group] = append(byGroup[c.Group], c)
		} else {
			orphans = append(orphans, c)
		}
	}

	var groups []markdownGroupView
	var toc []markdownTOCItem

	appendGroup := func(gv markdownGroupView) {
		groups = append(groups, gv)
		if !cfg.TOC {
			return
		}
		toc = append(toc, markdownTOCItem{Label: gv.Title, Anchor: gv.Anchor, Indent: 0, Control: false})
		for _, ctl := range gv.Controls {
			toc = append(toc, markdownTOCItem{
				Label:   ctl.Id + ": " + ctl.Title,
				Anchor:  Anchor(ctl.Id + ": " + ctl.Title),
				Indent:  1,
				Control: true,
			})
		}
	}

	for _, g := range catalog.Groups {
		ctrls := append([]gemara.Control(nil), byGroup[g.Id]...)
		if len(ctrls) == 0 {
			continue
		}
		sort.Slice(ctrls, func(i, j int) bool { return ctrls[i].Id < ctrls[j].Id })
		ctrls = copyControlsWithSortedARs(ctrls)
		anchor := Anchor(g.Id)
		if anchor == "" {
			anchor = Anchor(g.Title)
		}
		appendGroup(markdownGroupView{
			ID:          g.Id,
			Title:       g.Title,
			Description: g.Description,
			Anchor:      anchor,
			IsUngrouped: false,
			Controls:    ctrls,
		})
	}

	if len(orphans) > 0 {
		ctrls := append([]gemara.Control(nil), orphans...)
		sort.Slice(ctrls, func(i, j int) bool { return ctrls[i].Id < ctrls[j].Id })
		ctrls = copyControlsWithSortedARs(ctrls)
		uAnchor := Anchor(ungroupedSectionTitle)
		appendGroup(markdownGroupView{
			ID:          "",
			Title:       ungroupedSectionTitle,
			Description: "Controls whose group id is not listed in the catalog groups.",
			Anchor:      uAnchor,
			IsUngrouped: true,
			Controls:    ctrls,
		})
	}

	applicabilityCols, applicabilityRows, showMatrix := buildApplicabilityMatrix(catalog, groups, cfg.ApplicabilityMatrix)

	return markdownCatalogView{
		Title:                      catalog.Title,
		Metadata:                   catalog.Metadata,
		ShowMetadata:               cfg.Metadata,
		ShowApplicabilityMatrix:    showMatrix,
		ApplicabilityMatrixColumns: applicabilityCols,
		ApplicabilityMatrixRows:    applicabilityRows,
		Extends:                    catalog.Extends,
		Imports:                    catalog.Imports,
		TOC:                        cfg.TOC,
		LineEnding:                 cfg.LineEnding,
		Groups:                     groups,
		TOCItems:                   toc,
		NumControls:                numControlsShown,
		NumARs:                     numARs,
		LexiconGlossary:            lexGlossary,
	}
}

// copyControlsWithSortedARs returns a deep copy of ctrls with AssessmentRequirements
// sorted by id for stable Markdown output. The source slice and catalog are not modified.
func copyControlsWithSortedARs(ctrls []gemara.Control) []gemara.Control {
	out := make([]gemara.Control, len(ctrls))
	for i, c := range ctrls {
		out[i] = c
		ars := append([]gemara.AssessmentRequirement(nil), c.AssessmentRequirements...)
		sort.Slice(ars, func(a, b int) bool { return ars[a].Id < ars[b].Id })
		out[i].AssessmentRequirements = ars
	}
	return out
}

func arIncludedInApplicabilityMatrix(ar gemara.AssessmentRequirement) bool {
	return ar.State != gemara.LifecycleRetired
}

// applicabilityColumnIDs returns ordered applicability ids for matrix columns:
// metadata applicability-groups order if present, else sorted union of applicability
// on non-retired assessment requirements under active controls.
func applicabilityColumnIDs(catalog *gemara.ControlCatalog) []string {
	if len(catalog.Metadata.ApplicabilityGroups) > 0 {
		out := make([]string, 0, len(catalog.Metadata.ApplicabilityGroups))
		for _, g := range catalog.Metadata.ApplicabilityGroups {
			out = append(out, g.Id)
		}
		return out
	}
	seen := make(map[string]struct{})
	for _, c := range catalog.Controls {
		if c.State != gemara.LifecycleActive {
			continue
		}
		for _, ar := range c.AssessmentRequirements {
			if !arIncludedInApplicabilityMatrix(ar) {
				continue
			}
			for _, a := range ar.Applicability {
				seen[a] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func buildApplicabilityMatrix(catalog *gemara.ControlCatalog, groups []markdownGroupView, enabled bool) (cols []markdownApplicabilityColumn, rows []markdownApplicabilityMatrixRow, show bool) {
	if !enabled || catalog == nil {
		return nil, nil, false
	}
	colIDs := applicabilityColumnIDs(catalog)
	if len(colIDs) == 0 {
		return nil, nil, false
	}
	idToLabel := make(map[string]string, len(catalog.Metadata.ApplicabilityGroups))
	for _, g := range catalog.Metadata.ApplicabilityGroups {
		label := g.Title
		if label == "" {
			label = g.Id
		}
		idToLabel[g.Id] = label
	}
	cols = make([]markdownApplicabilityColumn, len(colIDs))
	for i, id := range colIDs {
		label := idToLabel[id]
		if label == "" {
			label = id
		}
		cols[i] = markdownApplicabilityColumn{ID: id, Label: sanitizeMarkdownTableCell(label)}
	}

	var flat []gemara.Control
	for _, gv := range groups {
		flat = append(flat, gv.Controls...)
	}
	if len(flat) == 0 {
		return nil, nil, false
	}

	for _, c := range flat {
		for _, ar := range c.AssessmentRequirements {
			if !arIncludedInApplicabilityMatrix(ar) {
				continue
			}
			set := make(map[string]struct{}, len(ar.Applicability))
			for _, a := range ar.Applicability {
				set[a] = struct{}{}
			}
			cells := make([]string, len(colIDs))
			for i, id := range colIDs {
				if _, ok := set[id]; ok {
					cells[i] = "X"
				}
			}
			rows = append(rows, markdownApplicabilityMatrixRow{
				RequirementID: sanitizeMarkdownTableCell(ar.Id),
				Cells:         cells,
			})
		}
	}
	if len(rows) == 0 {
		return nil, nil, false
	}
	return cols, rows, true
}
