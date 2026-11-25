// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gemaraproj/go-gemara"
)

func resolveReportEvidence(workspace *os.File, canonical string, parsed *parsedReport,
	evaluations map[string]*Evaluation, report *ManagedReport) error {
	references, err := indexMappingReferences(parsed.references)
	if err != nil {
		return err
	}
	for _, evidence := range parsed.evidence {
		if evidence.Source.ReferenceId != "" && !references[evidence.Source.ReferenceId] {
			report.Reasons = append(report.Reasons,
				fmt.Sprintf("evidence %q has an unresolved mapping-reference ID %q",
					evidence.Id, evidence.Source.ReferenceId))
		}
		if err := resolveEvidence(workspace, canonical, evidence, evaluations, report); err != nil {
			return err
		}
	}
	slices.Sort(report.Evaluations)
	report.Evaluations = slices.Compact(report.Evaluations)
	slices.Sort(report.Reasons)
	report.Reasons = slices.Compact(report.Reasons)
	if len(report.Evaluations) == 0 {
		report.Reasons = append(report.Reasons, "report has no establishable evaluation age")
	}
	return nil
}

func indexMappingReferences(references []gemara.MappingReference) (map[string]bool, error) {
	seen := make(map[string]bool, len(references))
	for _, reference := range references {
		if reference.Id == "" || seen[reference.Id] {
			return nil, fmt.Errorf("missing/duplicate mapping-reference ID %q", reference.Id)
		}
		seen[reference.Id] = true
	}
	return seen, nil
}

func resolveEvidence(workspace *os.File, canonical string, evidence gemara.Evidence,
	evaluations map[string]*Evaluation, report *ManagedReport) error {
	source := evidence.Source
	if source == (gemara.EvidenceMapping{}) {
		// Inline payloads are never interpreted as paths or deletion targets.
		return nil
	}
	if source.ReferenceId == "" || source.Coordinate == "" || source.EntryId != "" {
		return fmt.Errorf("evidence %q has an unresolved source; possible scope is unbounded", evidence.Id)
	}
	path, err := localEvidencePath(canonical, source.Coordinate)
	if err != nil {
		return fmt.Errorf("evidence %q: %w", evidence.Id, err)
	}
	if !filepath.IsLocal(path) {
		path, err = workspaceAliasEvidencePath(workspace, canonical, path)
		if err != nil {
			return fmt.Errorf("evidence %q: %w", evidence.Id, err)
		}
	}
	if !filepath.IsLocal(path) {
		// An external spelling can alias a managed unit without passing through
		// the workspace root. Its unknown relationship cannot establish an orphan.
		return fmt.Errorf("evidence %q is outside the established workspace boundary; "+
			"possible relationships are unbounded", evidence.Id)
	}
	unit := findEvidenceEvaluation(path, evaluations)
	if unit == nil {
		// A different spelling may resolve into a recognized evaluation on a
		// case-insensitive filesystem, even when its final file is missing.
		return fmt.Errorf("evidence %q has no established evaluation relationship: %q; "+
			"possible relationships are unbounded", evidence.Id, path)
	}
	// An edge can only name an already discovered unit; the coordinate cannot
	// create authority. Keep the edge even if its required file is now missing,
	// so later grouping preserves every other reference to that evaluation.
	report.Evaluations = append(report.Evaluations, unit.Path)
	if err := inspectEvidenceFile(workspace, path, *unit); err != nil {
		// A rejected traversal may conceal a reference to another evaluation.
		// Do not follow it, or misclassify its unknown target as reportless.
		// A missing file or the unit directory itself still has a bounded edge.
		if path != unit.Path && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("evidence %q has unsafe or inaccessible scope; "+
				"possible relationships are unbounded: %w", evidence.Id, err)
		}
		report.Reasons = append(report.Reasons, fmt.Sprintf("evidence %q (%q): %v", evidence.Id, path, err))
	}
	return nil
}

func localEvidencePath(workspace, coordinate string) (string, error) {
	parsed, err := url.Parse(coordinate)
	if err != nil {
		return "", fmt.Errorf("invalid evidence coordinate: %w", err)
	}
	if parsed.Scheme != "" {
		if parsed.Scheme != "file" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", fmt.Errorf("unsupported evidence coordinate; possible scope is unbounded: %q", coordinate)
		}
		coordinate = parsed.Path
	} else if strings.ContainsAny(coordinate, "?#$[]\x00\\") {
		return "", fmt.Errorf("ambiguous evidence coordinate; possible scope is unbounded: %q", coordinate)
	}
	// Cleaning before traversal would erase a symlink/.. component and could
	// assign an edge to a different unit than the original coordinate resolves.
	for _, component := range strings.Split(coordinate, string(filepath.Separator)) {
		if component == ".." {
			return "", fmt.Errorf("parent traversal in evidence coordinate; possible scope is unbounded")
		}
	}
	if filepath.IsAbs(coordinate) {
		relative, err := filepath.Rel(workspace, coordinate)
		if err != nil {
			return "", fmt.Errorf("resolve absolute evidence coordinate: %w", err)
		}
		return relative, nil
	}
	return filepath.Clean(coordinate), nil
}

func workspaceAliasEvidencePath(workspace *os.File, canonical, relative string) (string, error) {
	identity, err := workspace.Stat()
	if err != nil {
		return "", fmt.Errorf("inspect evidence workspace identity: %w", err)
	}
	absolute := filepath.Join(canonical, relative)
	components := strings.Split(absolute, string(filepath.Separator))
	prefix := string(filepath.Separator)
	// Workspace aliases (including Darwin's /var) are allowed only before the
	// pinned root. Per Composability, reuse confined evidence inspection below
	// that boundary instead of EvalSymlinks on the entire untrusted coordinate.
	for index, component := range components {
		prefix = filepath.Join(prefix, component)
		current, err := os.Stat(prefix)
		if errors.Is(err, os.ErrNotExist) {
			return relative, nil
		}
		if err != nil {
			return "", fmt.Errorf("inspect evidence workspace alias: %w", err)
		}
		if os.SameFile(identity, current) {
			return filepath.Join(components[index+1:]...), nil
		}
		if !current.IsDir() {
			return relative, nil
		}
	}
	return relative, nil
}

func findEvidenceEvaluation(path string, evaluations map[string]*Evaluation) *Evaluation {
	// A coordinate naming the unit itself still establishes a relationship.
	// File inspection can reject it, but grouping must retain that entire unit.
	for parent := path; parent != "."; parent = filepath.Dir(parent) {
		if unit := evaluations[parent]; unit != nil {
			return unit
		}
	}
	return nil
}

func inspectEvidenceFile(workspace *os.File, path string, evaluation Evaluation) (err error) {
	unit, err := openDiscoveryDirectory(workspace, evaluation.Path)
	if err != nil {
		return fmt.Errorf("open evidence evaluation: %w", err)
	}
	defer func() { err = errors.Join(err, unit.Close()) }()
	current, err := unit.Stat()
	if err != nil {
		return fmt.Errorf("stat evidence evaluation: %w", err)
	}
	if !os.SameFile(evaluation.Identity, current) {
		return fmt.Errorf("evidence evaluation changed since discovery")
	}
	relative, err := filepath.Rel(evaluation.Path, path)
	if err != nil {
		return fmt.Errorf("resolve contained evidence path: %w", err)
	}
	return inspectContainedEvidence(unit, relative)
}

func inspectContainedEvidence(unit *os.File, relative string) (err error) {
	parent := unit
	if dir := filepath.Dir(relative); dir != "." {
		parent, err = openDiscoveryDirectory(unit, dir)
		if err != nil {
			return fmt.Errorf("open contained evidence directory: %w", err)
		}
		defer func() { err = errors.Join(err, parent.Close()) }()
	}
	file, err := openInventoryRegular(parent, filepath.Base(relative))
	if err != nil {
		return fmt.Errorf("open contained evidence: %w", err)
	}
	return file.Close()
}
