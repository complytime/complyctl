// SPDX-License-Identifier: Apache-2.0

package output

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gemaraproj/go-gemara"

	"github.com/complytime/complyctl/internal/complytime"
)

// Markdown generates a human-readable compliance report in Markdown format.
type Markdown struct {
	policyID      string
	evalLog       *gemara.EvaluationLog
	embedLog      bool
	evaluationLog string
}

// NewMarkdown creates a Markdown report generator for the given policy and evaluation log.
func NewMarkdown(policyID string, evalLog *gemara.EvaluationLog) *Markdown {
	return &Markdown{
		policyID: policyID,
		evalLog:  evalLog,
		embedLog: false,
	}
}

// SetEmbedEvaluationLog enables embedding of the raw evaluation log YAML in the report.
func (m *Markdown) SetEmbedEvaluationLog(path string) {
	m.embedLog = true
	m.evaluationLog = path
}

type summaryCounts struct {
	passed        int
	failed        int
	needsReview   int
	notApplicable int
	notRun        int
	unknown       int
}

func (s summaryCounts) total() int {
	return s.passed + s.failed + s.needsReview + s.notApplicable + s.notRun + s.unknown
}

func (s summaryCounts) applicable() int {
	return s.total() - s.notApplicable - s.notRun
}

func computeSummaryCounts(evals []*gemara.ControlEvaluation) summaryCounts {
	var c summaryCounts
	for _, ce := range evals {
		for _, al := range ce.AssessmentLogs {
			switch al.Result {
			case gemara.Passed:
				c.passed++
			case gemara.Failed:
				c.failed++
			case gemara.NeedsReview:
				c.needsReview++
			case gemara.NotApplicable:
				c.notApplicable++
			case gemara.NotRun:
				c.notRun++
			case gemara.Unknown:
				c.unknown++
			}
		}
	}
	return c
}

type finding struct {
	controlID      string
	requirementID  string
	result         gemara.Result
	message        string
	recommendation string
	confidence     gemara.ConfidenceLevel
	evidence       []gemara.Evidence
}

func collectFindings(evals []*gemara.ControlEvaluation) []finding {
	var findings []finding
	for _, ce := range evals {
		for _, al := range ce.AssessmentLogs {
			if al.Result == gemara.Passed {
				continue
			}
			findings = append(findings, finding{
				controlID:      ce.Name,
				requirementID:  al.Requirement.EntryId,
				result:         al.Result,
				message:        al.Message,
				recommendation: al.Recommendation,
				confidence:     al.ConfidenceLevel,
				evidence:       al.Evidence,
			})
		}
	}
	slices.SortStableFunc(findings, func(a, b finding) int {
		return sortPriority(a.result) - sortPriority(b.result)
	})
	return findings
}

// Write generates the markdown report and writes it to outDir.
func (m *Markdown) Write(outDir string) (string, error) {
	now := time.Now()
	var sb strings.Builder

	m.writeSummary(&sb, now)
	m.writeControlsTable(&sb)
	m.writeFindings(&sb)
	m.writeEvaluationLog(&sb)

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("report-%s-%s.md",
		complytime.FilenameSafe(m.policyID), now.Format("20060102-150405"))
	path := filepath.Join(outDir, filename)
	if err := os.WriteFile(path, []byte(sb.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write markdown report: %w", err)
	}

	return path, nil
}

func (m *Markdown) writeSummary(sb *strings.Builder, now time.Time) {
	fmt.Fprintf(sb, "# Compliance Scan Report: %s\n\n", m.policyID)

	targetID := m.evalLog.Target.Id
	if targetID == "" {
		targetID = "-"
	}

	toolName := m.evalLog.Metadata.Author.Name
	if toolName == "" {
		toolName = "-"
	}
	toolVersion := m.evalLog.Metadata.Author.Version
	if toolVersion != "" {
		toolName = toolName + " " + toolVersion
	}

	fmt.Fprintf(sb, "| Field | Value |\n")
	fmt.Fprintf(sb, "|-------|-------|\n")
	fmt.Fprintf(sb, "| Policy | %s |\n", m.policyID)
	fmt.Fprintf(sb, "| Target | %s |\n", targetID)
	fmt.Fprintf(sb, "| Tool | %s |\n", toolName)
	fmt.Fprintf(sb, "| Result | %s |\n", m.evalLog.Result.String())
	fmt.Fprintf(sb, "| Date | %s |\n\n", now.Format(time.RFC3339))

	counts := computeSummaryCounts(m.evalLog.Evaluations)
	applicable := counts.applicable()
	passRate := 0
	if applicable > 0 {
		passRate = counts.passed * 100 / applicable
	}

	fmt.Fprintf(sb, "**Overall: %s -- %d%% pass rate (%d/%d applicable)**\n\n",
		m.evalLog.Result.String(), passRate, counts.passed, applicable)

	fmt.Fprintf(sb, "| Passed | Failed | Needs Review | Unknown | N/A | Not Run | Total |\n")
	fmt.Fprintf(sb, "|--------|--------|--------------|---------|-----|---------|-------|\n")
	fmt.Fprintf(sb, "| %d | %d | %d | %d | %d | %d | %d |\n\n",
		counts.passed, counts.failed, counts.needsReview,
		counts.unknown, counts.notApplicable, counts.notRun, counts.total())
}

func (m *Markdown) writeControlsTable(sb *strings.Builder) {
	fmt.Fprintf(sb, "## Controls\n\n")
	fmt.Fprintf(sb, "| Control / Requirement | Result | Message |\n")
	fmt.Fprintf(sb, "|-----------------------|--------|---------|\n")

	for _, ce := range m.evalLog.Evaluations {
		fmt.Fprintf(sb, "| **%s** | **%s** | %s |\n", ce.Name, ce.Result.String(), ce.Message)
		for _, al := range ce.AssessmentLogs {
			fmt.Fprintf(sb, "| &nbsp;&nbsp;%s | %s | |\n",
				al.Requirement.EntryId, al.Result.String())
		}
	}
	sb.WriteString("\n")
}

func (m *Markdown) writeFindings(sb *strings.Builder) {
	findings := collectFindings(m.evalLog.Evaluations)

	fmt.Fprintf(sb, "## Findings\n\n")

	if len(findings) == 0 {
		sb.WriteString("No findings.\n\n")
		return
	}

	var currentResult gemara.Result
	firstGroup := true
	for _, f := range findings {
		if firstGroup || f.result != currentResult {
			currentResult = f.result
			firstGroup = false
			fmt.Fprintf(sb, "### %s\n\n", currentResult.String())
		}

		fmt.Fprintf(sb, "#### %s -- %s\n\n", f.requirementID, f.result.String())
		fmt.Fprintf(sb, "- **Control**: %s\n", f.controlID)
		fmt.Fprintf(sb, "- **Message**: %s\n", f.message)

		if f.confidence != gemara.Undetermined {
			fmt.Fprintf(sb, "- **Confidence**: %s\n", f.confidence.String())
		}

		if f.recommendation != "" {
			fmt.Fprintf(sb, "- **Recommendation**: %s\n", f.recommendation)
		}

		if len(f.evidence) > 0 {
			fmt.Fprintf(sb, "\n<details>\n<summary>Evidence (%d items)</summary>\n\n",
				len(f.evidence))
			for _, ev := range f.evidence {
				label := ev.Description
				if label == "" {
					label = ev.Id
				}
				meta := formatEvidenceMeta(ev)
				if meta != "" {
					fmt.Fprintf(sb, "- %s (%s)\n", label, meta)
				} else {
					fmt.Fprintf(sb, "- %s\n", label)
				}
				if p, ok := ev.Payload.(string); ok && p != "" {
					fmt.Fprintf(sb, "\n  ```\n  %s\n  ```\n\n", p)
				}
			}
			sb.WriteString("\n</details>\n")
		}

		sb.WriteString("\n")
	}
}

func formatEvidenceMeta(ev gemara.Evidence) string {
	var parts []string
	if string(ev.Type) != "" {
		parts = append(parts, string(ev.Type))
	}
	if string(ev.CollectedAt) != "" {
		parts = append(parts, "collected: "+string(ev.CollectedAt))
	}
	return strings.Join(parts, ", ")
}

func (m *Markdown) writeEvaluationLog(sb *strings.Builder) {
	if !m.embedLog || m.evaluationLog == "" {
		return
	}

	data, err := os.ReadFile(m.evaluationLog)
	if err != nil {
		log.Warn("failed to read evaluation log for embedding", "path", m.evaluationLog, "error", err)
		return
	}

	sb.WriteString("---\n\n")
	sb.WriteString("<details>\n<summary>Evaluation Log</summary>\n\n```yaml\n")
	sb.WriteString(string(data))
	sb.WriteString("\n```\n\n</details>\n")
}
