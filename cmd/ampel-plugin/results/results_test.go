package results

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
	"github.com/stretchr/testify/require"
)

func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func TestParseAmpelOutput_Pass(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-pass.json")
	result, err := ParseAmpelOutput(data, "https://github.com/myorg/repo1", "main")
	require.NoError(t, err)
	require.Equal(t, "pass", result.Status)
	require.Len(t, result.Findings, 2)
	for _, f := range result.Findings {
		require.Equal(t, "pass", f.Result)
	}
	require.Equal(t, "check-SC-CODE-01.01", result.Findings[0].TenetID)
	require.Equal(t, "check-SC-CODE-03.01", result.Findings[1].TenetID)
}

func TestParseAmpelOutput_Fail(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-fail.json")
	result, err := ParseAmpelOutput(data, "https://github.com/myorg/repo1", "main")
	require.NoError(t, err)
	require.Equal(t, "fail", result.Status)
	require.Len(t, result.Findings, 2)

	var failCount int
	for _, f := range result.Findings {
		if f.Result == "fail" {
			failCount++
		}
	}
	require.Equal(t, 1, failCount)
}

func TestParseAmpelOutput_DSSEEnvelope(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-dsse-fail.json")
	result, err := ParseAmpelOutput(data, "https://github.com/myorg/repo1", "main")
	require.NoError(t, err)
	require.Equal(t, "fail", result.Status)
	require.Len(t, result.Findings, 2)

	var passCount, failCount int
	for _, f := range result.Findings {
		switch f.Result {
		case "pass":
			passCount++
		case "fail":
			failCount++
		}
	}
	require.Equal(t, 1, passCount, "expected 1 passing finding")
	require.Equal(t, 1, failCount, "expected 1 failing finding")
}

func TestParseAmpelOutput_Error(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-error.json")
	result, err := ParseAmpelOutput(data, "https://github.com/myorg/repo1", "main")
	require.NoError(t, err)
	require.Equal(t, "error", result.Status)
	require.NotEmpty(t, result.Error)
}

func TestParseAmpelOutput_Empty(t *testing.T) {
	_, err := ParseAmpelOutput([]byte{}, "repo", "main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}

func TestParseAmpelOutput_MalformedJSON(t *testing.T) {
	_, err := ParseAmpelOutput([]byte("{invalid json"), "repo", "main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing")
}

func TestParseAmpelOutput_ControlCharsStripped(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "PASS",
			Results: []ampelPolicyResult{
				{
					Status: "PASS",
					Policy: ampelPolicyRef{ID: "SC-CODE-01.01"},
					EvalResults: []ampelEvalResult{
						{
							ID:         "01",
							Status:     "PASS",
							Assessment: &ampelAssessment{Message: "OK\x07bell"},
						},
					},
					Meta: ampelResultMeta{Description: "Test\x00Title\x01With\x02Controls"},
				},
			},
		},
	}
	data, err := json.Marshal(stmt)
	require.NoError(t, err)

	result, err := ParseAmpelOutput(data, "repo", "main")
	require.NoError(t, err)
	require.Equal(t, "TestTitleWithControls", result.Findings[0].Title)
	require.Equal(t, "OKbell", result.Findings[0].Reason)
}

func TestParseAmpelOutput_OversizedField(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "PASS",
			Results: []ampelPolicyResult{
				{
					Status: "PASS",
					Policy: ampelPolicyRef{ID: "SC-CODE-01.01"},
					EvalResults: []ampelEvalResult{
						{
							ID:         "01",
							Status:     "PASS",
							Assessment: &ampelAssessment{Message: "OK"},
						},
					},
					Meta: ampelResultMeta{Description: strings.Repeat("x", maxFieldSize+1)},
				},
			},
		},
	}
	data, err := json.Marshal(stmt)
	require.NoError(t, err)

	_, err = ParseAmpelOutput(data, "repo", "main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds maximum size")
}

func TestParseAmpelOutput_NonPrintablePolicyID(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "PASS",
			Results: []ampelPolicyResult{
				{
					Status: "PASS",
					Policy: ampelPolicyRef{ID: "SC-CODE\x80-01"},
					EvalResults: []ampelEvalResult{
						{ID: "01", Status: "PASS", Assessment: &ampelAssessment{Message: "OK"}},
					},
					Meta: ampelResultMeta{Description: "Test"},
				},
			},
		},
	}
	data, err := json.Marshal(stmt)
	require.NoError(t, err)

	_, err = ParseAmpelOutput(data, "repo", "main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-printable")
}

func TestParseAmpelOutput_OversizedErrorField(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "ERROR",
			Error: &ampelError{
				Message: strings.Repeat("x", maxFieldSize+1),
			},
		},
	}
	data, err := json.Marshal(stmt)
	require.NoError(t, err)

	_, err = ParseAmpelOutput(data, "repo", "main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds maximum size")
}

func TestWritePerRepoResult(t *testing.T) {
	dir := t.TempDir()
	result := &PerRepoResult{
		Repository: "https://github.com/myorg/repo1",
		Branch:     "main",
		Status:     "pass",
		Findings: []Finding{
			{TenetID: "t1", Title: "Test", Result: "pass", Reason: "OK"},
		},
	}
	err := WritePerRepoResult(result, dir)
	require.NoError(t, err)

	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Contains(t, files[0].Name(), "myorg-repo1-main.json")
}

func TestWritePerRepoResult_Overwrites(t *testing.T) {
	dir := t.TempDir()
	r1 := &PerRepoResult{Repository: "https://github.com/org/repo", Branch: "main", Status: "pass"}
	r2 := &PerRepoResult{Repository: "https://github.com/org/repo", Branch: "main", Status: "fail"}

	require.NoError(t, WritePerRepoResult(r1, dir))
	require.NoError(t, WritePerRepoResult(r2, dir))

	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	data, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	require.NoError(t, err)
	require.Contains(t, string(data), `"fail"`)
}

func TestToPVPResult(t *testing.T) {
	results := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{TenetID: "check-1", Title: "Check 1", Result: "pass", Reason: "OK"},
			},
		},
		{
			Repository: "https://gitlab.com/myorg/repo2",
			Branch:     "main",
			Status:     "fail",
			Findings: []Finding{
				{TenetID: "check-1", Title: "Check 1", Result: "fail", Reason: "Not configured"},
			},
		},
	}

	pvp := ToPVPResult(results)
	// Same CheckID → grouped into one observation with two subjects
	require.Len(t, pvp.ObservationsByCheck, 1)
	obs := pvp.ObservationsByCheck[0]
	require.Equal(t, "check-1", obs.CheckID)
	require.Len(t, obs.Subjects, 2)

	// Sort by ResourceID for deterministic assertions
	subjects := obs.Subjects
	sort.Slice(subjects, func(i, j int) bool {
		return subjects[i].ResourceID < subjects[j].ResourceID
	})

	require.Equal(t, "https://github.com/myorg/repo1", subjects[0].ResourceID)
	require.Equal(t, policy.ResultPass, subjects[0].Result)

	require.Equal(t, "https://gitlab.com/myorg/repo2", subjects[1].ResourceID)
	require.Equal(t, policy.ResultFail, subjects[1].Result)
}

func TestToPVPResult_MultipleChecks(t *testing.T) {
	results := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{TenetID: "check-1", Title: "Check 1", Result: "pass", Reason: "OK"},
				{TenetID: "check-2", Title: "Check 2", Result: "pass", Reason: "OK"},
			},
		},
		{
			Repository: "https://github.com/myorg/repo2",
			Branch:     "main",
			Status:     "fail",
			Findings: []Finding{
				{TenetID: "check-1", Title: "Check 1", Result: "fail", Reason: "Not configured"},
				{TenetID: "check-2", Title: "Check 2", Result: "pass", Reason: "OK"},
			},
		},
	}

	pvp := ToPVPResult(results)
	// Two distinct CheckIDs → two observations
	require.Len(t, pvp.ObservationsByCheck, 2)

	// Each observation should have 2 subjects (one per repo)
	for _, obs := range pvp.ObservationsByCheck {
		require.Len(t, obs.Subjects, 2, "CheckID %s should have 2 subjects", obs.CheckID)
	}
}

func TestToPVPResult_ErrorRepo(t *testing.T) {
	results := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "error",
			Error:      "connection refused",
		},
	}

	pvp := ToPVPResult(results)
	require.Len(t, pvp.ObservationsByCheck, 1)
	require.Equal(t, policy.ResultError, pvp.ObservationsByCheck[0].Subjects[0].Result)
	require.Equal(t, "connection refused", pvp.ObservationsByCheck[0].Subjects[0].Reason)
}
