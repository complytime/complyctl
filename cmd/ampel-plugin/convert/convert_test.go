package convert

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/oscal-compass/compliance-to-policy-go/v2/policy"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/stretchr/testify/require"
)

func loadPolicy(t *testing.T, path string) policy.Policy {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading fixture %s", path)
	var ruleSets []extensions.RuleSet
	require.NoError(t, json.Unmarshal(data, &ruleSets), "unmarshaling fixture %s", path)
	return policy.Policy(ruleSets)
}

func loadExpectedPolicy(t *testing.T, path string) *AmpelPolicy {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading expected fixture %s", path)
	var expected AmpelPolicy
	require.NoError(t, json.Unmarshal(data, &expected), "unmarshaling expected fixture %s", path)
	return &expected
}

func TestPolicyToAmpel(t *testing.T) {
	cfg := ConvertConfig{Profile: "branch-protection-baseline"}

	tests := []struct {
		name           string
		inputFixture   string
		expectedFile   string
		expectedTenets int
		wantNil        bool
		wantErr        bool
	}{
		{
			name:           "full plan produces full policy",
			inputFixture:   "testdata/assessment-plan-full.json",
			expectedFile:   "testdata/ampel-policy-expected-full.json",
			expectedTenets: 5,
		},
		{
			name:           "subset plan produces subset policy",
			inputFixture:   "testdata/assessment-plan-subset.json",
			expectedFile:   "testdata/ampel-policy-expected-subset.json",
			expectedTenets: 2,
		},
		{
			name:    "empty policy input returns nil",
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var input policy.Policy
			if tc.inputFixture != "" {
				input = loadPolicy(t, tc.inputFixture)
			} else {
				input = policy.Policy{}
			}

			result, err := PolicyToAmpel(input, cfg)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tc.wantNil {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			require.Len(t, result.Tenets, tc.expectedTenets)

			if tc.expectedFile != "" {
				expected := loadExpectedPolicy(t, tc.expectedFile)
				require.Equal(t, expected.ID, result.ID)
				require.Equal(t, expected.Meta.AssertMode, result.Meta.AssertMode)
				require.Equal(t, expected.Meta.Runtime, result.Meta.Runtime)
				require.Equal(t, len(expected.Tenets), len(result.Tenets))
				for i, tenet := range expected.Tenets {
					require.Equal(t, tenet.ID, result.Tenets[i].ID, "tenet %d ID mismatch", i)
					require.Equal(t, tenet.Title, result.Tenets[i].Title, "tenet %d Title mismatch", i)
					require.Equal(t, tenet.Code, result.Tenets[i].Code, "tenet %d Code mismatch", i)
					require.Equal(t, tenet.Predicates, result.Tenets[i].Predicates, "tenet %d Predicates mismatch", i)
				}
			}
		})
	}
}

func TestPolicyToAmpel_NilInput(t *testing.T) {
	cfg := ConvertConfig{Profile: "test"}
	_, err := PolicyToAmpel(nil, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not be nil")
}

func TestPolicyToAmpel_RuleWithNoChecks(t *testing.T) {
	cfg := ConvertConfig{Profile: "test"}
	input := policy.Policy{
		{
			Rule: extensions.Rule{
				ID:          "rule-no-checks",
				Description: "Rule with no checks",
			},
			Checks: nil,
		},
	}
	result, err := PolicyToAmpel(input, cfg)
	require.NoError(t, err)
	require.Nil(t, result, "rule with no checks should produce nil output")
}

func TestPolicyToAmpel_EmptyCheckID(t *testing.T) {
	cfg := ConvertConfig{Profile: "test"}
	input := policy.Policy{
		{
			Rule: extensions.Rule{
				ID:          "rule-empty-check",
				Description: "Rule with empty check ID",
			},
			Checks: []extensions.Check{
				{ID: "", Description: "Empty ID check"},
			},
		},
	}
	_, err := PolicyToAmpel(input, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "check ID must not be empty")
}

func TestPolicyToAmpel_ChangingParameterChangesOutput(t *testing.T) {
	cfg := ConvertConfig{Profile: "test"}

	makePolicy := func(value string) policy.Policy {
		return policy.Policy{
			{
				Rule: extensions.Rule{
					ID:          "min-approvals",
					Description: "Minimum approvals",
					Parameters: []extensions.Parameter{
						{ID: "min_approvals", Description: "Min approvals", Value: value},
					},
				},
				Checks: []extensions.Check{
					{ID: "check-min-approvals", Description: "Check min approvals"},
				},
			},
		}
	}

	result1, err := PolicyToAmpel(makePolicy("2"), cfg)
	require.NoError(t, err)
	require.NotNil(t, result1)

	result2, err := PolicyToAmpel(makePolicy("3"), cfg)
	require.NoError(t, err)
	require.NotNil(t, result2)

	require.NotEqual(t, result1.Tenets[0].Code, result2.Tenets[0].Code,
		"changing parameter value should change CEL expression")
	require.Contains(t, result1.Tenets[0].Code, "2")
	require.Contains(t, result2.Tenets[0].Code, "3")
}

func TestWritePolicy(t *testing.T) {
	t.Run("writes policy file", func(t *testing.T) {
		dir := t.TempDir()
		p := &AmpelPolicy{
			ID:   "test-policy",
			Meta: AmpelMeta{Runtime: "cel@v14.0", AssertMode: "AND", Enforce: "ON", Controls: []Control{}},
			Tenets: []AmpelTenet{
				{ID: "t1", Title: "Test", Predicates: PredicateSpec{Types: []string{"type"}}, Code: "true"},
			},
		}
		err := WritePolicy(p, dir)
		require.NoError(t, err)

		path := filepath.Join(dir, PolicyFileName)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(data), "test-policy")
	})

	t.Run("nil policy writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		err := WritePolicy(nil, dir)
		require.NoError(t, err)

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("creates directory if missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "subdir", "nested")
		p := &AmpelPolicy{
			ID:     "test",
			Meta:   AmpelMeta{Controls: []Control{}},
			Tenets: []AmpelTenet{{ID: "t1", Code: "true", Predicates: PredicateSpec{Types: []string{"type"}}}},
		}
		err := WritePolicy(p, dir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(dir, PolicyFileName))
		require.NoError(t, err)
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		dir := t.TempDir()
		p1 := &AmpelPolicy{ID: "first", Meta: AmpelMeta{Controls: []Control{}}, Tenets: []AmpelTenet{{ID: "t1", Code: "v1", Predicates: PredicateSpec{Types: []string{"type"}}}}}
		p2 := &AmpelPolicy{ID: "second", Meta: AmpelMeta{Controls: []Control{}}, Tenets: []AmpelTenet{{ID: "t1", Code: "v2", Predicates: PredicateSpec{Types: []string{"type"}}}}}

		require.NoError(t, WritePolicy(p1, dir))
		require.NoError(t, WritePolicy(p2, dir))

		data, err := os.ReadFile(filepath.Join(dir, PolicyFileName))
		require.NoError(t, err)
		require.Contains(t, string(data), "second")
		require.NotContains(t, string(data), "first")
	})
}
