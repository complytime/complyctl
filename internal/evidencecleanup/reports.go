// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	reportDirectory   = ".complytime/scan"
	maxReportBytes    = 8 << 20
	maxInventoryBytes = 256 << 20
	maxReportEntries  = 10000
	formatEvaluation  = "evaluation-log"
	formatMarkdown    = "markdown"
	formatOSCAL       = "oscal"
	formatSARIF       = "sarif"
)

// ManagedReport records content-derived relationships, not deletion authority.
// Paths are workspace-relative. Reasons retain this report and every connected
// evaluation during later group preflight. Identity must be revalidated there.
type ManagedReport struct {
	Path        string
	Format      string
	PolicyID    string
	TargetID    string
	Evaluations []string
	Reasons     []string
	Identity    os.FileInfo
}

// ReportInventory is a complete managed inventory, including reportless units.
// It contains no age selection, transitive groups, or removal authorization.
type ReportInventory struct {
	Evaluations []Evaluation
	Reports     []ManagedReport
	Exclusions  []DiscoveryExclusion
}

// InventoryReports inventories existing evaluations and all managed reports.
// The caller must hold exclusive workspace access and check activity/support,
// just as for Discover. Unbounded uncertainty returns nil and a contextual error;
// a partial inventory must never authorize any workspace mutation. Bounded
// missing/unsafe references instead populate the affected report's Reasons.
func InventoryReports(opts DiscoveryOptions) (result *ReportInventory, err error) {
	discovered, err := Discover(opts)
	if err != nil {
		return nil, fmt.Errorf("inventory reports: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(opts.Workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve report workspace: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute report workspace: %w", err)
	}
	workspace, validate, err := openDiscoveryWorkspace(canonical)
	if err != nil {
		return nil, fmt.Errorf("open report workspace: %w", err)
	}
	defer func() {
		err = errors.Join(err, workspace.Close())
		if err != nil {
			result = nil
		}
	}()
	result = &ReportInventory{Evaluations: discovered.Evaluations, Exclusions: discovered.Exclusions}
	if err := inventoryReportDirectory(workspace, canonical, result); err != nil {
		return nil, fmt.Errorf("inventory managed reports: %w", err)
	}
	if err := validate(); err != nil {
		return nil, fmt.Errorf("revalidate report workspace: %w", err)
	}
	return result, nil
}

func inventoryReportDirectory(workspace *os.File, canonical string, result *ReportInventory) (err error) {
	dir, err := openDiscoveryDirectory(workspace, filepath.FromSlash(reportDirectory))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, dir.Close()) }()
	remaining := int64(maxInventoryBytes)
	count := 0
	evaluations := indexEvaluations(result.Evaluations)
	for {
		names, readErr := dir.Readdirnames(readDirectoryBatch)
		for _, name := range names {
			count++
			if count > maxReportEntries {
				return fmt.Errorf("report directory entry limit exceeded")
			}
			if err := inventoryReportEntry(workspace, dir, canonical, name, &remaining, evaluations, result); err != nil {
				return fmt.Errorf("report %q: %w", filepath.Join(reportDirectory, name), err)
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read report directory: %w", readErr)
		}
	}
	slices.SortFunc(result.Reports, func(a, b ManagedReport) int { return strings.Compare(a.Path, b.Path) })
	return nil
}

func inventoryReportEntry(workspace, dir *os.File, canonical, name string,
	remaining *int64, evaluations map[string]*Evaluation, result *ReportInventory) error {
	format, reportLike := classifyReportName(name)
	if !reportLike {
		result.Exclusions = append(result.Exclusions, DiscoveryExclusion{
			Path: filepath.Join(reportDirectory, name), Reason: "unrelated report-directory entry",
		})
		return nil
	}
	if format == "" {
		return fmt.Errorf("report-like entry has an unsupported managed format")
	}
	data, identity, err := readManagedReport(dir, name)
	if err != nil {
		return err
	}
	*remaining -= int64(len(data))
	if *remaining < 0 {
		return fmt.Errorf("aggregate report byte limit exceeded")
	}
	parsed, err := parseManagedReport(format, data)
	if err != nil {
		return err
	}
	report := ManagedReport{Path: filepath.Join(reportDirectory, name), Format: format,
		PolicyID: parsed.policy, TargetID: parsed.target, Identity: identity}
	if err := resolveReportEvidence(workspace, canonical, parsed, evaluations, &report); err != nil {
		return err
	}
	result.Reports = append(result.Reports, report)
	return nil
}

func indexEvaluations(evaluations []Evaluation) map[string]*Evaluation {
	byPath := make(map[string]*Evaluation, len(evaluations))
	for i := range evaluations {
		byPath[evaluations[i].Path] = &evaluations[i]
	}
	return byPath
}

// Names select an adapter only. They never supply policy, target, age, or an edge.
// Treat all structured-report extensions and known prefixes as potentially
// significant: an unknown variant must not make evidence appear reportless.
func classifyReportName(name string) (string, bool) {
	switch {
	case strings.HasPrefix(name, "evaluation-log-"):
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".json") {
			return formatEvaluation, true
		}
	case strings.HasPrefix(name, "report-"):
		if strings.HasSuffix(name, ".md") {
			return formatMarkdown, true
		}
	case strings.HasPrefix(name, "assessment-results-"):
		if strings.HasSuffix(name, ".json") {
			return formatOSCAL, true
		}
	case strings.HasPrefix(name, "scan-"):
		if strings.HasSuffix(name, ".sarif.json") {
			return formatSARIF, true
		}
	default:
		return "", slices.Contains([]string{".json", ".yaml", ".yml", ".md", ".sarif"},
			strings.ToLower(filepath.Ext(name)))
	}
	return "", true
}

func readManagedReport(dir *os.File, name string) (data []byte, identity os.FileInfo, err error) {
	file, err := openInventoryRegular(dir, name)
	if err != nil {
		return nil, nil, fmt.Errorf("open confined report: %w", err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	identity, err = file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat report: %w", err)
	}
	data, err = io.ReadAll(io.LimitReader(file, maxReportBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read report: %w", err)
	}
	if len(data) > maxReportBytes {
		return nil, nil, fmt.Errorf("report byte limit exceeded")
	}
	return data, identity, nil
}
