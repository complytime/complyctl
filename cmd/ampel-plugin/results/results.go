package results

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
)

const maxFieldSize = 10 * 1024 // 10KB per field

// AmpelVerifyOutput represents the JSON output from ampel verify.
type AmpelVerifyOutput struct {
	PolicyID string              `json:"policy_id"`
	Passed   bool                `json:"passed"`
	Error    string              `json:"error,omitempty"`
	Results  []AmpelVerifyResult `json:"results,omitempty"`
}

// AmpelVerifyResult represents a single tenet result from ampel verify.
type AmpelVerifyResult struct {
	TenetID string `json:"tenet_id"`
	Title   string `json:"title"`
	Passed  bool   `json:"passed"`
	Reason  string `json:"reason"`
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

// ParseAmpelOutput parses raw ampel verify JSON output into a PerRepoResult.
// It validates field sizes and strips control characters per security requirements.
func ParseAmpelOutput(raw []byte, repo, branch string) (*PerRepoResult, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty ampel verify output")
	}

	var output AmpelVerifyOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, fmt.Errorf("parsing ampel verify output: %w", err)
	}

	result := &PerRepoResult{
		Repository: repo,
		Branch:     branch,
		ScannedAt:  time.Now(),
	}

	// Handle error case
	if output.Error != "" {
		if len(output.Error) > maxFieldSize {
			return nil, fmt.Errorf("ampel output error field exceeds maximum size")
		}
		result.Status = "error"
		result.Error = stripControlChars(output.Error)
		return result, nil
	}

	// Map results to findings
	for _, r := range output.Results {
		if err := validateFieldSizes(r); err != nil {
			return nil, err
		}
		if !isPrintableASCII(r.TenetID) {
			return nil, fmt.Errorf("tenet ID %q contains non-printable characters", r.TenetID)
		}

		finding := Finding{
			TenetID: stripControlChars(r.TenetID),
			Title:   stripControlChars(r.Title),
			Reason:  stripControlChars(r.Reason),
		}
		if r.Passed {
			finding.Result = "pass"
		} else {
			finding.Result = "fail"
		}
		result.Findings = append(result.Findings, finding)
	}

	if output.Passed {
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

	filename := sanitizeForFilename(result.Repository) + "-" + result.Branch + ".json"
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
func ToPVPResult(repoResults []*PerRepoResult) policy.PVPResult {
	var observations []policy.ObservationByCheck

	for _, rr := range repoResults {
		for _, f := range rr.Findings {
			obs := policy.ObservationByCheck{
				Title:   f.Title,
				CheckID: f.TenetID,
				Methods: []string{"AUTOMATED"},
				Subjects: []policy.Subject{
					{
						Title:       repoDisplayName(rr.Repository),
						Type:        "inventory-item",
						ResourceID:  rr.Repository,
						Result:      mapResult(f.Result, rr.Status),
						EvaluatedOn: rr.ScannedAt,
						Reason:      f.Reason,
					},
				},
				Collected: rr.ScannedAt,
			}
			observations = append(observations, obs)
		}

		// For error status with no findings, add an error observation
		if rr.Status == "error" && len(rr.Findings) == 0 {
			obs := policy.ObservationByCheck{
				Title:   "Scan Error",
				CheckID: "scan-error",
				Methods: []string{"AUTOMATED"},
				Subjects: []policy.Subject{
					{
						Title:       repoDisplayName(rr.Repository),
						Type:        "inventory-item",
						ResourceID:  rr.Repository,
						Result:      policy.ResultError,
						EvaluatedOn: rr.ScannedAt,
						Reason:      rr.Error,
					},
				},
				Collected: rr.ScannedAt,
			}
			observations = append(observations, obs)
		}
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

func validateFieldSizes(r AmpelVerifyResult) error {
	if len(r.TenetID) > maxFieldSize {
		return fmt.Errorf("tenet_id field exceeds maximum size")
	}
	if len(r.Title) > maxFieldSize {
		return fmt.Errorf("title field exceeds maximum size")
	}
	if len(r.Reason) > maxFieldSize {
		return fmt.Errorf("reason field exceeds maximum size")
	}
	return nil
}

func sanitizeForFilename(repoURL string) string {
	name := repoURL
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(name, prefix) {
			name = name[len(prefix):]
			break
		}
	}
	var result []rune
	for _, r := range name {
		if r == '/' || r == '.' || r == ':' {
			result = append(result, '-')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

func repoDisplayName(repoURL string) string {
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return repoURL
}
