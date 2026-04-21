package convert

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

func loadConfigurations(t *testing.T, path string) []provider.AssessmentConfiguration {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading fixture %s", path)
	var configs []provider.AssessmentConfiguration
	require.NoError(t, json.Unmarshal(data, &configs), "unmarshaling fixture %s", path)
	return configs
}

func loadExpectedBundle(t *testing.T, path string) *AmpelPolicyBundle {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading expected fixture %s", path)
	var expected AmpelPolicyBundle
	require.NoError(t, json.Unmarshal(data, &expected), "unmarshaling expected fixture %s", path)
	return &expected
}

// --- LoadGranularPolicies tests ---

func TestLoadGranularPolicies(t *testing.T) {
	policies, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)
	require.Len(t, policies, 5)

	expectedIDs := []string{
		"BP-1.01",
		"BP-2.01",
		"BP-3.01",
		"BP-4.01",
		"BP-5.01",
	}
	for _, id := range expectedIDs {
		p, ok := policies[id]
		require.True(t, ok, "expected policy %q to be loaded", id)
		require.Equal(t, id, p.ID)
		require.NotEmpty(t, p.Tenets, "policy %q should have tenets", id)
	}
}

func TestLoadGranularPolicies_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Empty(t, policies)
}

func TestLoadGranularPolicies_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid"), 0600))

	_, err := LoadGranularPolicies(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing policy file")
}

func TestLoadGranularPolicies_EmptyPolicyID(t *testing.T) {
	dir := t.TempDir()
	data := `{"id": "", "meta": {"description": "test"}, "tenets": []}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty-id.json"), []byte(data), 0600))

	_, err := LoadGranularPolicies(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty id field")
}

func TestLoadGranularPolicies_SkipsOutputFile(t *testing.T) {
	dir := t.TempDir()

	// Write a valid granular policy
	p := AmpelPolicy{ID: "test-01", Meta: PolicyMeta{Description: "test"}, Tenets: []AmpelTenet{}}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test-01.json"), data, 0600))

	// Write the output file (should be skipped)
	bundle := AmpelPolicyBundle{ID: "complytime-ampel-policy"}
	bdata, err := json.Marshal(bundle)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, PolicyFileName), bdata, 0600))

	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Len(t, policies, 1)
	require.Contains(t, policies, "test-01")
}

func TestLoadGranularPolicies_SkipsNonJSON(t *testing.T) {
	dir := t.TempDir()

	// Write a valid granular policy
	p := AmpelPolicy{ID: "test-01", Meta: PolicyMeta{Description: "test"}, Tenets: []AmpelTenet{}}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test-01.json"), data, 0600))

	// Write a non-JSON file (should be skipped)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0600))

	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Len(t, policies, 1)
}

// --- MatchPolicies tests ---

func TestMatchPolicies(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := loadConfigurations(t, "testdata/assessment-plan-full.json")
	matched, warnings := MatchPolicies(input, granular)

	require.Len(t, matched, 5)
	require.Empty(t, warnings)

	// Verify sorted order
	for i := 1; i < len(matched); i++ {
		require.True(t, matched[i-1].ID < matched[i].ID,
			"expected sorted order, got %s before %s", matched[i-1].ID, matched[i].ID)
	}
}

func TestMatchPolicies_Subset(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := loadConfigurations(t, "testdata/assessment-plan-subset.json")
	matched, warnings := MatchPolicies(input, granular)

	require.Len(t, matched, 2)
	require.Empty(t, warnings)
	require.Equal(t, "BP-1.01", matched[0].ID)
	require.Equal(t, "BP-3.01", matched[1].ID)
}

func TestMatchPolicies_UnmatchedRule(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := []provider.AssessmentConfiguration{
		{RequirementID: "BP-1.01"},
		{RequirementID: "nonexistent-rule"},
	}

	matched, warnings := MatchPolicies(input, granular)
	require.Len(t, matched, 1)
	require.Equal(t, "BP-1.01", matched[0].ID)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "nonexistent-rule")
}

func TestMatchPolicies_AllUnmatched(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := []provider.AssessmentConfiguration{
		{RequirementID: "no-such-rule-1"},
		{RequirementID: "no-such-rule-2"},
	}

	matched, warnings := MatchPolicies(input, granular)
	require.Empty(t, matched)
	require.Len(t, warnings, 2)
}

func TestMatchPolicies_EmptyInput(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	matched, warnings := MatchPolicies([]provider.AssessmentConfiguration{}, granular)
	require.Empty(t, matched)
	require.Empty(t, warnings)
}

func TestMatchPolicies_DuplicateRequirements(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := []provider.AssessmentConfiguration{
		{RequirementID: "BP-1.01"},
		{RequirementID: "BP-1.01"},
	}

	matched, warnings := MatchPolicies(input, granular)
	require.Len(t, matched, 1, "duplicate requirements should be deduplicated")
	require.Empty(t, warnings)
}

// --- MergeToBundle tests ---

func TestMergeToBundle(t *testing.T) {
	policies := []*AmpelPolicy{
		{ID: "BP-1.01", Meta: PolicyMeta{Description: "PR required"}, Tenets: []AmpelTenet{{ID: "01"}}},
		{ID: "BP-3.01", Meta: PolicyMeta{Description: "Force push"}, Tenets: []AmpelTenet{{ID: "01"}}},
	}

	bundle := MergeToBundle(policies)
	require.Equal(t, "complytime-ampel-policy", bundle.ID)
	require.Len(t, bundle.Meta.Frameworks, 1)
	require.Equal(t, "ComplyTime-AMPEL-Policy", bundle.Meta.Frameworks[0].ID)
	require.Len(t, bundle.Policies, 2)
	require.Equal(t, "BP-1.01", bundle.Policies[0].ID)
	require.Equal(t, "BP-3.01", bundle.Policies[1].ID)
}

func TestMergeToBundle_Empty(t *testing.T) {
	bundle := MergeToBundle(nil)
	require.Equal(t, "complytime-ampel-policy", bundle.ID)
	require.Empty(t, bundle.Policies)
}

// --- End-to-end: load, match, merge, compare to expected ---

func TestEndToEnd_FullPlan(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := loadConfigurations(t, "testdata/assessment-plan-full.json")
	matched, warnings := MatchPolicies(input, granular)
	require.Empty(t, warnings)
	require.Len(t, matched, 5)

	bundle := MergeToBundle(matched)
	expected := loadExpectedBundle(t, "testdata/ampel-bundle-expected-full.json")

	require.Equal(t, expected.ID, bundle.ID)
	require.Equal(t, expected.Meta, bundle.Meta)
	require.Len(t, bundle.Policies, len(expected.Policies))
	for i, ep := range expected.Policies {
		require.Equal(t, ep.ID, bundle.Policies[i].ID, "policy %d ID mismatch", i)
		require.Equal(t, ep.Meta, bundle.Policies[i].Meta, "policy %d meta mismatch", i)
		require.Equal(t, len(ep.Tenets), len(bundle.Policies[i].Tenets), "policy %d tenet count mismatch", i)
		for j, et := range ep.Tenets {
			require.Equal(t, et.ID, bundle.Policies[i].Tenets[j].ID, "policy %d tenet %d ID mismatch", i, j)
			require.Equal(t, et.Code, bundle.Policies[i].Tenets[j].Code, "policy %d tenet %d Code mismatch", i, j)
		}
	}
}

func TestEndToEnd_SubsetPlan(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := loadConfigurations(t, "testdata/assessment-plan-subset.json")
	matched, warnings := MatchPolicies(input, granular)
	require.Empty(t, warnings)
	require.Len(t, matched, 2)

	bundle := MergeToBundle(matched)
	expected := loadExpectedBundle(t, "testdata/ampel-bundle-expected-subset.json")

	require.Equal(t, expected.ID, bundle.ID)
	require.Len(t, bundle.Policies, len(expected.Policies))
	for i, ep := range expected.Policies {
		require.Equal(t, ep.ID, bundle.Policies[i].ID, "policy %d ID mismatch", i)
	}
}

// --- WritePolicy tests ---

func TestWritePolicy(t *testing.T) {
	t.Run("writes bundle file", func(t *testing.T) {
		dir := t.TempDir()
		bundle := &AmpelPolicyBundle{
			ID: "test-bundle",
			Meta: BundleMeta{
				Frameworks: []Framework{{ID: "test", Name: "Test"}},
			},
			Policies: []*AmpelPolicy{
				{
					ID:   "policy-1",
					Meta: PolicyMeta{Description: "Test policy"},
					Tenets: []AmpelTenet{
						{ID: "01", Code: "true", Predicates: PredicateSpec{Types: []string{"type"}}},
					},
				},
			},
		}
		err := WritePolicy(bundle, dir)
		require.NoError(t, err)

		path := filepath.Join(dir, PolicyFileName)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(data), "test-bundle")
		require.Contains(t, string(data), "policy-1")
	})

	t.Run("nil bundle writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		err := WritePolicy(nil, dir)
		require.NoError(t, err)

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("empty policies writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		bundle := &AmpelPolicyBundle{ID: "empty", Policies: nil}
		err := WritePolicy(bundle, dir)
		require.NoError(t, err)

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("creates directory if missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "subdir", "nested")
		bundle := &AmpelPolicyBundle{
			ID: "test",
			Policies: []*AmpelPolicy{
				{ID: "p1", Tenets: []AmpelTenet{{ID: "01", Code: "true", Predicates: PredicateSpec{Types: []string{"type"}}}}},
			},
		}
		err := WritePolicy(bundle, dir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(dir, PolicyFileName))
		require.NoError(t, err)
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		dir := t.TempDir()
		b1 := &AmpelPolicyBundle{
			ID: "first",
			Policies: []*AmpelPolicy{
				{ID: "p1", Tenets: []AmpelTenet{{ID: "01", Code: "v1", Predicates: PredicateSpec{Types: []string{"type"}}}}},
			},
		}
		b2 := &AmpelPolicyBundle{
			ID: "second",
			Policies: []*AmpelPolicy{
				{ID: "p2", Tenets: []AmpelTenet{{ID: "01", Code: "v2", Predicates: PredicateSpec{Types: []string{"type"}}}}},
			},
		}

		require.NoError(t, WritePolicy(b1, dir))
		require.NoError(t, WritePolicy(b2, dir))

		data, err := os.ReadFile(filepath.Join(dir, PolicyFileName))
		require.NoError(t, err)
		require.Contains(t, string(data), "second")
		require.NotContains(t, string(data), "first")
	})
}
