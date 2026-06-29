// SPDX-License-Identifier: Apache-2.0

package complytime_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
)

func TestParsePolicyRef_FullReference(t *testing.T) {
	ref, err := complytime.ParsePolicyRef("registry.com/policies/nist-800-53-r5:v1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "registry.com", ref.Registry)
	assert.Equal(t, "policies/nist-800-53-r5", ref.Repository)
	assert.Equal(t, "v1.2.3", ref.Tag)
	assert.Empty(t, ref.Digest)
}

func TestParsePolicyRef_NoVersion(t *testing.T) {
	ref, err := complytime.ParsePolicyRef("registry.com/policies/nist-800-53-r5")
	require.NoError(t, err)
	assert.Equal(t, "registry.com", ref.Registry)
	assert.Equal(t, "policies/nist-800-53-r5", ref.Repository)
	assert.Empty(t, ref.Tag)
	assert.Empty(t, ref.Digest)
}

func TestParsePolicyRef_WithHTTPScheme(t *testing.T) {
	ref, err := complytime.ParsePolicyRef("http://localhost:5000/policies/test:v1.0")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:5000", ref.Registry)
	assert.Equal(t, "policies/test", ref.Repository)
	assert.Equal(t, "v1.0", ref.Tag)
	assert.Empty(t, ref.Digest)
}

func TestParsePolicyRef_WithHTTPSScheme(t *testing.T) {
	ref, err := complytime.ParsePolicyRef("https://ghcr.io/org/policy:latest")
	require.NoError(t, err)
	assert.Equal(t, "https://ghcr.io", ref.Registry)
	assert.Equal(t, "org/policy", ref.Repository)
	assert.Equal(t, "latest", ref.Tag)
	assert.Empty(t, ref.Digest)
}

func TestParsePolicyRef_NoRegistry(t *testing.T) {
	ref, err := complytime.ParsePolicyRef("nist-800-53-r5@v1.0")
	require.NoError(t, err)
	assert.Empty(t, ref.Registry)
	assert.Equal(t, "nist-800-53-r5", ref.Repository)
	assert.Equal(t, "v1.0", ref.Tag)
	assert.Empty(t, ref.Digest)
}

func TestParsePolicyRef_BareID(t *testing.T) {
	ref, err := complytime.ParsePolicyRef("nist-800-53-r5")
	require.NoError(t, err)
	assert.Empty(t, ref.Registry)
	assert.Equal(t, "nist-800-53-r5", ref.Repository)
	assert.Empty(t, ref.Tag)
	assert.Empty(t, ref.Digest)
}

func TestParsePolicyRef_BareIDWithDigest(t *testing.T) {
	dgst := "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	ref, err := complytime.ParsePolicyRef("my-policy@" + dgst)
	require.NoError(t, err)
	assert.Empty(t, ref.Registry)
	assert.Equal(t, "my-policy", ref.Repository)
	assert.Empty(t, ref.Tag)
	assert.Equal(t, dgst, ref.Digest)
}

func TestParsePolicyRef_PortInRegistry(t *testing.T) {
	ref, err := complytime.ParsePolicyRef("localhost:5000/policy:v2")
	require.NoError(t, err)
	assert.Equal(t, "localhost:5000", ref.Registry)
	assert.Equal(t, "policy", ref.Repository)
	assert.Equal(t, "v2", ref.Tag)
	assert.Empty(t, ref.Digest)
}

func TestParsePolicyRef_ColonTag(t *testing.T) {
	ref, err := complytime.ParsePolicyRef("quay.io/complytime/complypack-ampel-bp:v0.4.0")
	require.NoError(t, err)
	assert.Equal(t, "quay.io", ref.Registry)
	assert.Equal(t, "complytime/complypack-ampel-bp", ref.Repository)
	assert.Equal(t, "v0.4.0", ref.Tag)
	assert.Empty(t, ref.Digest)
}

func TestParsePolicyRef_ColonLatest(t *testing.T) {
	ref, err := complytime.ParsePolicyRef("quay.io/complytime/complypack-ampel-bp:latest")
	require.NoError(t, err)
	assert.Equal(t, "quay.io", ref.Registry)
	assert.Equal(t, "complytime/complypack-ampel-bp", ref.Repository)
	assert.Equal(t, "latest", ref.Tag)
	assert.Empty(t, ref.Digest)
}

func TestParsePolicyRef_Digest(t *testing.T) {
	dgst := "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	ref, err := complytime.ParsePolicyRef("quay.io/complytime/complypack-ampel-bp@" + dgst)
	require.NoError(t, err)
	assert.Equal(t, "quay.io", ref.Registry)
	assert.Equal(t, "complytime/complypack-ampel-bp", ref.Repository)
	assert.Empty(t, ref.Tag)
	assert.Equal(t, dgst, ref.Digest)
}

func TestParsePolicyRef_HTTPSchemeWithColonTag(t *testing.T) {
	ref, err := complytime.ParsePolicyRef("http://localhost:5000/policies/test:v1.0")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:5000", ref.Registry)
	assert.Equal(t, "policies/test", ref.Repository)
	assert.Equal(t, "v1.0", ref.Tag)
	assert.Empty(t, ref.Digest)
}

func TestPolicyRef_VersionString_Tag(t *testing.T) {
	ref := complytime.PolicyRef{Tag: "v1.0"}
	assert.Equal(t, "v1.0", ref.VersionString())
}

func TestPolicyRef_VersionString_Digest(t *testing.T) {
	ref := complytime.PolicyRef{Digest: "sha256:abc123"}
	assert.Equal(t, "sha256:abc123", ref.VersionString())
}

func TestPolicyRef_VersionString_Empty(t *testing.T) {
	ref := complytime.PolicyRef{}
	assert.Empty(t, ref.VersionString())
}

func TestParsePolicyRef_ErrorOnEmpty(t *testing.T) {
	_, err := complytime.ParsePolicyRef("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestParsePolicyRef_ErrorOnWhitespace(t *testing.T) {
	_, err := complytime.ParsePolicyRef("   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestParsePolicyRef_DeprecationWarning_AtVersion(t *testing.T) {
	// Reset deduplication state so the warning fires for this test URL.
	complytime.ResetDeprecationWarnings()

	// Capture stderr to verify the deprecation warning.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	ref, parseErr := complytime.ParsePolicyRef("registry.example.com/policies/nist@v1.0")

	w.Close()
	output, readErr := io.ReadAll(r)
	r.Close()
	require.NoError(t, readErr)

	require.NoError(t, parseErr)
	assert.Equal(t, "v1.0", ref.Tag)
	assert.Empty(t, ref.Digest)
	assert.Contains(t, string(output), "DEPRECATED")
	assert.Contains(t, string(output), "registry.example.com/policies/nist@v1.0")
	assert.Contains(t, string(output), ":tag")
}

func TestParsePolicyRef_NoDeprecationWarning_Digest(t *testing.T) {
	dgst := "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	ref, parseErr := complytime.ParsePolicyRef("registry.example.com/policies/nist@" + dgst)

	w.Close()
	output, readErr := io.ReadAll(r)
	r.Close()
	require.NoError(t, readErr)

	require.NoError(t, parseErr)
	assert.Equal(t, dgst, ref.Digest)
	assert.Empty(t, ref.Tag)
	assert.Empty(t, string(output), "no deprecation warning expected for digest reference")
}

func TestParsePolicyRef_NoDeprecationWarning_BareID(t *testing.T) {
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	ref, parseErr := complytime.ParsePolicyRef("my-policy@v2.0")

	w.Close()
	output, readErr := io.ReadAll(r)
	r.Close()
	require.NoError(t, readErr)

	require.NoError(t, parseErr)
	assert.Equal(t, "v2.0", ref.Tag)
	assert.Empty(t, ref.Digest)
	assert.Empty(t, string(output), "no deprecation warning expected for bare policy ID")
}

func TestPolicyEntry_EffectiveID_ExplicitID(t *testing.T) {
	p := complytime.PolicyEntry{URL: "registry.com/policies/nist-800-53-r5:v1.0", ID: "nist"}
	assert.Equal(t, "nist", p.EffectiveID())
}

func TestPolicyEntry_EffectiveID_DerivedFromURL(t *testing.T) {
	p := complytime.PolicyEntry{URL: "registry.com/policies/nist-800-53-r5:v1.0"}
	assert.Equal(t, "nist-800-53-r5", p.EffectiveID())
}

func TestPolicyEntry_EffectiveID_NestedPath(t *testing.T) {
	p := complytime.PolicyEntry{URL: "registry.com/org/team/cis-fedora:v2.0"}
	assert.Equal(t, "cis-fedora", p.EffectiveID())
}

func TestFindPolicy_ByEffectiveID(t *testing.T) {
	policies := []complytime.PolicyEntry{
		{URL: "registry.com/policies/nist:v1.0", ID: "nist"},
		{URL: "ghcr.io/cis:v2.0"},
	}
	entry, ok := complytime.FindPolicy(policies, "nist")
	assert.True(t, ok)
	assert.Equal(t, "registry.com/policies/nist:v1.0", entry.URL)
}

func TestFindPolicy_ByDerivedID(t *testing.T) {
	policies := []complytime.PolicyEntry{
		{URL: "registry.com/policies/nist-800-53-r5:v1.0"},
	}
	entry, ok := complytime.FindPolicy(policies, "nist-800-53-r5")
	assert.True(t, ok)
	assert.Equal(t, "registry.com/policies/nist-800-53-r5:v1.0", entry.URL)
}

func TestFindPolicy_NotFound(t *testing.T) {
	policies := []complytime.PolicyEntry{
		{URL: "registry.com/policies/nist:v1.0"},
	}
	_, ok := complytime.FindPolicy(policies, "nonexistent")
	assert.False(t, ok)
}

func TestValidate_Valid(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/policies/nist:v1.0", ID: "nist"},
		},
		Targets: []complytime.TargetConfig{{
			ID:       "local",
			Policies: []string{"nist"},
		}},
	}
	assert.NoError(t, complytime.Validate(cfg))
}

func TestValidate_ValidWithDerivedID(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/policies/nist-800-53-r5:v1.0"},
		},
		Targets: []complytime.TargetConfig{{
			ID:       "local",
			Policies: []string{"nist-800-53-r5"},
		}},
	}
	assert.NoError(t, complytime.Validate(cfg))
}

func TestValidate_NoPolicies(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one policy")
}

func TestValidate_EmptyURL(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: ""}},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestValidate_DuplicateURL(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/a:v1"},
			{URL: "registry.com/a:v1"},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate url")
}

func TestValidate_DuplicateEffectiveID(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/a/nist:v1"},
			{URL: "ghcr.io/b/nist:v2"},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate id nist")
}

func TestValidate_DuplicateExplicitID(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/a:v1", ID: "same"},
			{URL: "ghcr.io/b:v2", ID: "same"},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate id same")
}

func TestValidate_TargetPolicyNotInList(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/a:v1", ID: "nist"},
		},
		Targets: []complytime.TargetConfig{{
			ID:       "local",
			Policies: []string{"nonexistent"},
		}},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in policies list")
}

func TestValidate_DuplicateTarget(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/a:v1", ID: "nist"},
		},
		Targets: []complytime.TargetConfig{
			{ID: "local", Policies: []string{"nist"}},
			{ID: "local", Policies: []string{"nist"}},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate local")
}

func TestValidate_TargetNoPolicies(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/a:v1"},
		},
		Targets: []complytime.TargetConfig{{ID: "local"}},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one required")
}

func TestPolicyIDs(t *testing.T) {
	policies := []complytime.PolicyEntry{
		{URL: "registry.com/policies/nist:v1.0", ID: "nist"},
		{URL: "ghcr.io/cis-fedora:v2.0"},
	}
	m := complytime.PolicyIDs(policies)
	assert.Len(t, m, 2)
	assert.NotNil(t, m["nist"])
	assert.NotNil(t, m["cis-fedora"])
}

//nolint:gosec // G101: test data, not real credentials
func TestResolveEnvVars_Substitution(t *testing.T) {
	t.Setenv("CT_TEST_TOKEN", "secret123")
	t.Setenv("CT_TEST_HOST", "db.example.com")

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "registry.com/p1:v1"}},
		Targets: []complytime.TargetConfig{{
			ID:       "target1",
			Policies: []string{"p1"},
			Variables: map[string]string{
				"api_token": "${CT_TEST_TOKEN}",
				"db_host":   "${CT_TEST_HOST}",
				"plain":     "no-env-ref",
			},
		}},
	}

	require.NoError(t, complytime.ResolveEnvVars(cfg))
	assert.Equal(t, "secret123", cfg.Targets[0].Variables["api_token"])
	assert.Equal(t, "db.example.com", cfg.Targets[0].Variables["db_host"])
	assert.Equal(t, "no-env-ref", cfg.Targets[0].Variables["plain"])
}

//nolint:gosec // G101: test data, not real credentials
func TestResolveEnvVars_UnsetVariableErrors(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "registry.com/p1:v1"}},
		Targets: []complytime.TargetConfig{{
			ID:       "target1",
			Policies: []string{"p1"},
			Variables: map[string]string{
				"token": "${CT_UNSET_VAR_12345}",
			},
		}},
	}

	err := complytime.ResolveEnvVars(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CT_UNSET_VAR_12345")
	assert.Contains(t, err.Error(), "unset environment variable")
}

func TestResolveEnvVars_MultipleRefsInOneValue(t *testing.T) {
	t.Setenv("CT_PROTO", "https")
	t.Setenv("CT_HOST", "example.com")

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "registry.com/p1:v1"}},
		Targets: []complytime.TargetConfig{{
			ID:       "target1",
			Policies: []string{"p1"},
			Variables: map[string]string{
				"endpoint": "${CT_PROTO}://${CT_HOST}/api",
			},
		}},
	}

	require.NoError(t, complytime.ResolveEnvVars(cfg))
	assert.Equal(t, "https://example.com/api", cfg.Targets[0].Variables["endpoint"])
}

func TestResolveEnvVars_NoVariables(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "registry.com/p1:v1"}},
		Targets: []complytime.TargetConfig{{
			ID:       "target1",
			Policies: []string{"p1"},
		}},
	}

	require.NoError(t, complytime.ResolveEnvVars(cfg))
}

func TestResolveEnvVars_EmptyValue(t *testing.T) {
	t.Setenv("CT_EMPTY", "")

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "registry.com/p1:v1"}},
		Targets: []complytime.TargetConfig{{
			ID:       "target1",
			Policies: []string{"p1"},
			Variables: map[string]string{
				"val": "${CT_EMPTY}",
			},
		}},
	}

	require.NoError(t, complytime.ResolveEnvVars(cfg))
	assert.Equal(t, "", cfg.Targets[0].Variables["val"])
}

func TestValidate_UnsupportedVersion(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Version: 99,
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/a:v1", ID: "a"},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported config version")
}

func TestValidate_CurrentVersion(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Version: complytime.CurrentWorkspaceVersion,
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/a:v1", ID: "a"},
		},
		Targets: []complytime.TargetConfig{{
			ID:       "local",
			Policies: []string{"a"},
		}},
	}
	assert.NoError(t, complytime.Validate(cfg))
}

func TestValidate_ZeroVersionAllowed(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Version: 0,
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/a:v1", ID: "a"},
		},
		Targets: []complytime.TargetConfig{{
			ID:       "local",
			Policies: []string{"a"},
		}},
	}
	assert.NoError(t, complytime.Validate(cfg))
}

// T254: ValidateOCIRef tests

func TestValidateOCIRef_ValidFullRef(t *testing.T) {
	assert.NoError(t, complytime.ValidateOCIRef("registry.com/policies/nist-800-53-r5:v1.0"))
}

func TestValidateOCIRef_ValidWithTag(t *testing.T) {
	assert.NoError(t, complytime.ValidateOCIRef("registry.com/repo:latest"))
}

func TestValidateOCIRef_ValidWithDigest(t *testing.T) {
	assert.NoError(t, complytime.ValidateOCIRef("registry.com/repo@sha256:abc123def"))
}

func TestValidateOCIRef_ValidWithPort(t *testing.T) {
	assert.NoError(t, complytime.ValidateOCIRef("localhost:5000/policies/test:v1.0"))
}

func TestValidateOCIRef_ValidHTTPS(t *testing.T) {
	assert.NoError(t, complytime.ValidateOCIRef("https://ghcr.io/org/policy:latest"))
}

func TestValidateOCIRef_ValidHTTP(t *testing.T) {
	assert.NoError(t, complytime.ValidateOCIRef("http://localhost:5000/policies/test:v1.0"))
}

func TestValidateOCIRef_ValidNestedPath(t *testing.T) {
	assert.NoError(t, complytime.ValidateOCIRef("registry.com/org/team/policy:v2.0"))
}

func TestValidateOCIRef_RejectEmpty(t *testing.T) {
	err := complytime.ValidateOCIRef("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestValidateOCIRef_RejectWhitespace(t *testing.T) {
	err := complytime.ValidateOCIRef("   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestValidateOCIRef_RejectShellInjection(t *testing.T) {
	err := complytime.ValidateOCIRef("ls;pwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestValidateOCIRef_RejectPipeInjection(t *testing.T) {
	err := complytime.ValidateOCIRef("foo|bar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestValidateOCIRef_RejectBacktickInjection(t *testing.T) {
	err := complytime.ValidateOCIRef("`whoami`/repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestValidateOCIRef_RejectDollarInjection(t *testing.T) {
	err := complytime.ValidateOCIRef("$(cat /etc/passwd)/repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestValidateOCIRef_RejectBareWord(t *testing.T) {
	err := complytime.ValidateOCIRef("nist-800-53-r5")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must include a registry")
}

func TestValidateOCIRef_RejectNoRegistryHost(t *testing.T) {
	err := complytime.ValidateOCIRef("plaindir/repo:v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must include a registry host")
}

// T256: Validate catches duplicate URLs (already tested above, but verify
// the OCI validation runs first for malformed entries)

func TestValidate_InvalidOCIRef(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "bare-name-no-registry"},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must include a registry")
}

func TestValidate_ShellInjectionInURL(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "ls;rm -rf /"},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

// Complypack validation tests

func TestValidate_ComplypackDuplicateURL(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/policies/nist:v1.0", ID: "nist"},
		},
		Complypacks: []complytime.PolicyEntry{
			{URL: "ghcr.io/org/pack-a:v1"},
			{URL: "ghcr.io/org/pack-a:v1"},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate url")
}

func TestValidate_ComplypackDuplicateEffectiveID(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/policies/nist:v1.0", ID: "nist"},
		},
		Complypacks: []complytime.PolicyEntry{
			{URL: "registry.com/org/mypack:v1"},
			{URL: "ghcr.io/other/mypack:v2"},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate id mypack")
}

func TestValidate_ComplypackAndPolicySameURL_Allowed(t *testing.T) {
	// Cross-list independence: the same URL can appear in both
	// policies and complypacks without conflict.
	sharedURL := "ghcr.io/org/shared-artifact:v1.0"
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: sharedURL, ID: "shared-policy"},
		},
		Complypacks: []complytime.PolicyEntry{
			{URL: sharedURL, ID: "shared-pack"},
		},
		Targets: []complytime.TargetConfig{{
			ID:       "local",
			Policies: []string{"shared-policy"},
		}},
	}
	assert.NoError(t, complytime.Validate(cfg))
}

func TestValidate_ComplypackInvalidOCIRef(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{
			name:    "shell injection semicolon",
			url:     "ls;pwd",
			wantErr: "invalid characters",
		},
		{
			name:    "bare word without registry",
			url:     "just-a-name",
			wantErr: "must include a registry",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &complytime.WorkspaceConfig{
				Policies: []complytime.PolicyEntry{
					{URL: "registry.com/policies/nist:v1.0", ID: "nist"},
				},
				Complypacks: []complytime.PolicyEntry{
					{URL: tt.url},
				},
			}
			err := complytime.Validate(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidate_ComplypackEmptyURL(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/policies/nist:v1.0", ID: "nist"},
		},
		Complypacks: []complytime.PolicyEntry{
			{URL: ""},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestValidate_ComplypackEmpty_Allowed(t *testing.T) {
	// No complypacks section is valid — complypacks are optional.
	tests := []struct {
		name        string
		complypacks []complytime.PolicyEntry
	}{
		{name: "nil slice", complypacks: nil},
		{name: "empty slice", complypacks: []complytime.PolicyEntry{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &complytime.WorkspaceConfig{
				Policies: []complytime.PolicyEntry{
					{URL: "registry.com/policies/nist:v1.0", ID: "nist"},
				},
				Complypacks: tt.complypacks,
				Targets: []complytime.TargetConfig{{
					ID:       "local",
					Policies: []string{"nist"},
				}},
			}
			assert.NoError(t, complytime.Validate(cfg))
		})
	}
}

func TestLoadFrom_WithComplypacks(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "complytime.yml")

	yamlContent := `policies:
  - url: registry.com/policies/nist:v1.0
    id: nist
complypacks:
  - url: ghcr.io/org/complypack-rhel9:v1.0
    id: rhel9
  - url: ghcr.io/org/complypack-ubuntu:v2.0
targets:
  - id: local
    policies:
      - nist
`
	require.NoError(t, os.WriteFile(configPath, []byte(yamlContent), 0600))

	cfg, err := complytime.LoadFrom(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify policies parsed correctly
	require.Len(t, cfg.Policies, 1)
	assert.Equal(t, "registry.com/policies/nist:v1.0", cfg.Policies[0].URL)

	// Verify complypacks parsed correctly
	require.Len(t, cfg.Complypacks, 2)
	assert.Equal(t, "ghcr.io/org/complypack-rhel9:v1.0", cfg.Complypacks[0].URL)
	assert.Equal(t, "rhel9", cfg.Complypacks[0].ID)
	assert.Equal(t, "ghcr.io/org/complypack-ubuntu:v2.0", cfg.Complypacks[1].URL)
	assert.Empty(t, cfg.Complypacks[1].ID)

	// Verify EffectiveID derivation works for the entry without explicit ID
	assert.Equal(t, "complypack-ubuntu", cfg.Complypacks[1].EffectiveID())
}

func TestLoadFrom_InvalidPolicyURL(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "complytime.yml")

	yamlContent := `policies:
  - url: ""
targets:
  - id: local
    policies:
      - nist
`
	require.NoError(t, os.WriteFile(configPath, []byte(yamlContent), 0600))

	_, err := complytime.LoadFrom(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config")
}

func TestLoadFrom_InvalidComplypackURL(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "complytime.yml")

	yamlContent := `policies:
  - url: registry.com/policies/nist:v1.0
    id: nist
complypacks:
  - url: "   "
targets:
  - id: local
    policies:
      - nist
`
	require.NoError(t, os.WriteFile(configPath, []byte(yamlContent), 0600))

	_, err := complytime.LoadFrom(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config")
}

// TestLoadFrom_ColonTagComplypack verifies the fix for issue #594:
// complypack URLs using :tag syntax must load and parse correctly.
func TestLoadFrom_ColonTagComplypack(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "complytime.yml")

	yamlContent := `policies:
  - url: quay.io/complytime/policies-ampel-bp:latest
    id: ampel-bp
complypacks:
  - url: quay.io/complytime/complypack-ampel-bp:v0.4.0
    id: ampel-bp-pack
targets:
  - id: local
    policies:
      - ampel-bp
`
	require.NoError(t, os.WriteFile(configPath, []byte(yamlContent), 0600))

	cfg, err := complytime.LoadFrom(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Complypacks, 1)
	assert.Equal(t, "quay.io/complytime/complypack-ampel-bp:v0.4.0", cfg.Complypacks[0].URL)
}

func TestLoadFrom_WithoutComplypacks(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "complytime.yml")

	// Existing config format without complypacks — must still work unchanged.
	yamlContent := `policies:
  - url: registry.com/policies/nist:v1.0
    id: nist
targets:
  - id: local
    policies:
      - nist
`
	require.NoError(t, os.WriteFile(configPath, []byte(yamlContent), 0600))

	cfg, err := complytime.LoadFrom(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Policies load as before
	require.Len(t, cfg.Policies, 1)
	assert.Equal(t, "registry.com/policies/nist:v1.0", cfg.Policies[0].URL)

	// Complypacks is nil/empty when not present in YAML
	assert.Empty(t, cfg.Complypacks)
}
