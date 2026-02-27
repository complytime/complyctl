package results

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/complytime/complyctl/cmd/ampel-plugin/intoto"
	"github.com/complytime/complyctl/cmd/ampel-plugin/targets"
	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
)

const maxFieldSize = 10 * 1024 // 10KB per field

// ampelResultStatement represents the in-toto attestation produced by ampel verify
// with --attest-results. The predicate contains the evaluation ResultSet.
type ampelResultStatement struct {
	Predicate ampelResultSetPred `json:"predicate"`
}

// ampelResultSetPred represents the ResultSet predicate from ampel verify.
type ampelResultSetPred struct {
	Status  string              `json:"status"`
	Results []ampelPolicyResult `json:"results"`
	Error   *ampelError         `json:"error,omitempty"`
}

// ampelPolicyResult represents a single policy evaluation result.
type ampelPolicyResult struct {
	Status      string            `json:"status"`
	Policy      ampelPolicyRef    `json:"policy"`
	EvalResults []ampelEvalResult `json:"eval_results"`
	Meta        ampelResultMeta   `json:"meta"`
}

// ampelPolicyRef identifies the policy that was evaluated.
type ampelPolicyRef struct {
	ID string `json:"id"`
}

// ampelEvalResult represents a single tenet evaluation result.
type ampelEvalResult struct {
	ID         string           `json:"id"`
	Status     string           `json:"status"`
	Assessment *ampelAssessment `json:"assessment,omitempty"`
	Error      *ampelError      `json:"error,omitempty"`
}

// ampelAssessment holds the message for a passing tenet.
type ampelAssessment struct {
	Message string `json:"message"`
}

// ampelError holds the message and guidance for a failing tenet or result set.
type ampelError struct {
	Message  string `json:"message"`
	Guidance string `json:"guidance"`
}

// ampelResultMeta holds metadata about a policy evaluation.
type ampelResultMeta struct {
	Description string `json:"description"`
}

// PerRepoResult holds scan findings for a single repository.
type PerRepoResult struct {
	Repository string    `json:"repository"`
	Branch     string    `json:"branch"`
	ScannedAt  time.Time `json:"scanned_at"`
	Findings   []Finding `json:"findings"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}

// Finding represents an individual rule evaluation result.
type Finding struct {
	TenetID string `json:"tenet_id"`
	Title   string `json:"title"`
	Result  string `json:"result"`
	Reason  string `json:"reason"`
}

// ParseAmpelOutput parses the in-toto attestation produced by ampel verify
// (with --attest-results) into a PerRepoResult. The attestation predicate
// contains the evaluation ResultSet with per-policy and per-tenet results.
func ParseAmpelOutput(raw []byte, repo, branch string) (*PerRepoResult, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty ampel verify output")
	}

	// Unwrap DSSE envelope if present (ampel --attest-results produces signed attestations)
	raw, err := intoto.UnwrapDSSE(raw)
	if err != nil {
		return nil, fmt.Errorf("unwrapping DSSE envelope in ampel result: %w", err)
	}

	var stmt ampelResultStatement
	if err := json.Unmarshal(raw, &stmt); err != nil {
		return nil, fmt.Errorf("parsing ampel verify attestation: %w", err)
	}

	result := &PerRepoResult{
		Repository: repo,
		Branch:     branch,
		ScannedAt:  time.Now(),
	}

	// Handle ResultSet-level error
	if stmt.Predicate.Error != nil && stmt.Predicate.Error.Message != "" {
		if len(stmt.Predicate.Error.Message) > maxFieldSize {
			return nil, fmt.Errorf("ampel output error field exceeds maximum size")
		}
		result.Status = "error"
		result.Error = stripControlChars(stmt.Predicate.Error.Message)
		return result, nil
	}

	// Extract findings from each policy result's tenet evaluations
	for _, policyResult := range stmt.Predicate.Results {
		policyID := stripControlChars(policyResult.Policy.ID)
		if !isPrintableASCII(policyID) {
			return nil, fmt.Errorf("policy ID %q contains non-printable characters", policyID)
		}

		description := stripControlChars(policyResult.Meta.Description)

		for _, er := range policyResult.EvalResults {
			checkID := "check-" + policyID
			if len(checkID) > maxFieldSize || len(description) > maxFieldSize {
				return nil, fmt.Errorf("field exceeds maximum size in policy %s", policyID)
			}

			finding := Finding{
				TenetID: checkID,
				Title:   description,
			}

			status := strings.ToUpper(er.Status)
			switch status {
			case "PASS":
				finding.Result = "pass"
				if er.Assessment != nil {
					finding.Reason = stripControlChars(er.Assessment.Message)
				}
			default:
				finding.Result = "fail"
				if er.Error != nil {
					finding.Reason = stripControlChars(er.Error.Message)
				}
			}

			result.Findings = append(result.Findings, finding)
		}
	}

	overallStatus := strings.ToUpper(stmt.Predicate.Status)
	if overallStatus == "PASS" {
		result.Status = "pass"
	} else {
		result.Status = "fail"
	}

	return result, nil
}

// WritePerRepoResult writes a PerRepoResult as JSON to the given directory.
func WritePerRepoResult(result *PerRepoResult, dir string) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating results directory: %w", err)
	}

	filename := targets.SanitizeRepoURL(result.Repository) + "-" + result.Branch + ".json"
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling per-repo result: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing per-repo result: %w", err)
	}

	return nil
}

// ToPVPResult maps a slice of PerRepoResults to a policy.PVPResult.
// Findings with the same CheckID are grouped into a single ObservationByCheck
// with multiple Subjects (one per repository). This matches the OSCAL pattern
// and prevents last-write-wins overwrites in the downstream observation manager.
func ToPVPResult(repoResults []*PerRepoResult) policy.PVPResult {
	type checkGroup struct {
		title     string
		checkID   string
		subjects  []policy.Subject
		collected time.Time
	}

	groups := make(map[string]*checkGroup)
	var order []string // track insertion order for deterministic output

	for _, rr := range repoResults {
		for _, f := range rr.Findings {
			g, ok := groups[f.TenetID]
			if !ok {
				g = &checkGroup{
					title:     f.Title,
					checkID:   f.TenetID,
					collected: rr.ScannedAt,
				}
				groups[f.TenetID] = g
				order = append(order, f.TenetID)
			}
			g.subjects = append(g.subjects, policy.Subject{
				Title:       targets.RepoDisplayName(rr.Repository),
				Type:        "inventory-item",
				ResourceID:  rr.Repository,
				Result:      mapResult(f.Result, rr.Status),
				EvaluatedOn: rr.ScannedAt,
				Reason:      f.Reason,
			})
			if rr.ScannedAt.After(g.collected) {
				g.collected = rr.ScannedAt
			}
		}

		// For error status with no findings, add an error subject
		if rr.Status == "error" && len(rr.Findings) == 0 {
			const errorCheckID = "scan-error"
			g, ok := groups[errorCheckID]
			if !ok {
				g = &checkGroup{
					title:     "Scan Error",
					checkID:   errorCheckID,
					collected: rr.ScannedAt,
				}
				groups[errorCheckID] = g
				order = append(order, errorCheckID)
			}
			g.subjects = append(g.subjects, policy.Subject{
				Title:       targets.RepoDisplayName(rr.Repository),
				Type:        "inventory-item",
				ResourceID:  rr.Repository,
				Result:      policy.ResultError,
				EvaluatedOn: rr.ScannedAt,
				Reason:      rr.Error,
			})
			if rr.ScannedAt.After(g.collected) {
				g.collected = rr.ScannedAt
			}
		}
	}

	observations := make([]policy.ObservationByCheck, 0, len(groups))
	for _, checkID := range order {
		g := groups[checkID]
		observations = append(observations, policy.ObservationByCheck{
			Title:     g.title,
			CheckID:   g.checkID,
			Methods:   []string{"AUTOMATED"},
			Subjects:  g.subjects,
			Collected: g.collected,
		})
	}

	return policy.PVPResult{
		ObservationsByCheck: observations,
	}
}

func mapResult(findingResult, repoStatus string) policy.Result {
	if repoStatus == "error" {
		return policy.ResultError
	}
	switch findingResult {
	case "pass":
		return policy.ResultPass
	case "fail":
		return policy.ResultFail
	default:
		return policy.ResultError
	}
}

func stripControlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

func isPrintableASCII(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7E {
			return false
		}
	}
	return true
}

