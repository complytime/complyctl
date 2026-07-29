// SPDX-License-Identifier: Apache-2.0

package complytime_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache/cachetest"
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
	dgst := cachetest.DigestA
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
	dgst := cachetest.DigestA
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
	dgst := cachetest.DigestA
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

func TestValidateVerificationConfig_Nil(t *testing.T) {
	err := complytime.ValidateVerificationConfig(nil)
	assert.NoError(t, err)
}

func TestValidateVerificationConfig_ValidKeyless(t *testing.T) {
	v := &complytime.VerificationConfig{
		Issuer:   "https://token.actions.githubusercontent.com",
		Identity: "https://github.com/complytime/complyctl/.github/workflows/release.yml@refs/tags/*",
	}
	err := complytime.ValidateVerificationConfig(v)
	assert.NoError(t, err)
}

func TestValidateVerificationConfig_ValidKeyed(t *testing.T) {
	v := &complytime.VerificationConfig{
		Key: "/path/to/cosign.pub",
	}
	err := complytime.ValidateVerificationConfig(v)
	assert.NoError(t, err)
}

func TestValidateVerificationConfig_MutuallyExclusive(t *testing.T) {
	v := &complytime.VerificationConfig{
		Issuer:   "https://token.actions.githubusercontent.com",
		Identity: "https://github.com/complytime/complyctl/.github/workflows/release.yml@refs/tags/*",
		Key:      "/path/to/cosign.pub",
	}
	err := complytime.ValidateVerificationConfig(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidateVerificationConfig_IssuerWithoutIdentity(t *testing.T) {
	v := &complytime.VerificationConfig{
		Issuer: "https://token.actions.githubusercontent.com",
	}
	err := complytime.ValidateVerificationConfig(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issuer requires identity")
}

func TestValidateVerificationConfig_IdentityWithoutIssuer(t *testing.T) {
	v := &complytime.VerificationConfig{
		Identity: "https://github.com/complytime/complyctl/.github/workflows/release.yml@refs/tags/*",
	}
	err := complytime.ValidateVerificationConfig(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity requires issuer")
}

func TestVerificationConfig_IsConfigured(t *testing.T) {
	tests := []struct {
		name   string
		config *complytime.VerificationConfig
		want   bool
	}{
		{"nil config", nil, false},
		{"empty config", &complytime.VerificationConfig{}, false},
		{"keyless configured", &complytime.VerificationConfig{
			Issuer:   "https://token.actions.githubusercontent.com",
			Identity: "user@example.com",
		}, true},
		{"keyed configured", &complytime.VerificationConfig{
			Key: "/path/to/key.pub",
		}, true},
		{"issuer only (incomplete)", &complytime.VerificationConfig{
			Issuer: "https://token.actions.githubusercontent.com",
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.config.IsConfigured())
		})
	}
}

func TestLoadFrom_WithVerificationKeyless(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "complytime.yml")

	yamlContent := `policies:
  - url: registry.com/policies/nist:v1.0
    id: nist
targets:
  - id: local
    policies:
      - nist
verification:
  issuer: "https://token.actions.githubusercontent.com"
  identity: "https://github.com/complytime/complyctl/.github/workflows/release.yml@refs/tags/*"
`
	require.NoError(t, os.WriteFile(configPath, []byte(yamlContent), 0600))

	cfg, err := complytime.LoadFrom(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Verification)
	assert.Equal(t, "https://token.actions.githubusercontent.com", cfg.Verification.Issuer)
	assert.Equal(t, "https://github.com/complytime/complyctl/.github/workflows/release.yml@refs/tags/*", cfg.Verification.Identity)
	assert.Empty(t, cfg.Verification.Key)
}

func TestLoadFrom_WithVerificationKeyed(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "complytime.yml")

	yamlContent := `policies:
  - url: registry.com/policies/nist:v1.0
    id: nist
targets:
  - id: local
    policies:
      - nist
verification:
  key: "/path/to/cosign.pub"
`
	require.NoError(t, os.WriteFile(configPath, []byte(yamlContent), 0600))

	cfg, err := complytime.LoadFrom(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Verification)
	assert.Equal(t, "/path/to/cosign.pub", cfg.Verification.Key)
	assert.Empty(t, cfg.Verification.Issuer)
}

func TestLoadFrom_WithoutVerification(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "complytime.yml")

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
	assert.Nil(t, cfg.Verification)
}

func TestValidate_VerificationMutuallyExclusive(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{URL: "registry.com/policies/nist:v1.0", ID: "nist"}},
		Targets:  []complytime.TargetConfig{{ID: "local", Policies: []string{"nist"}}},
		Verification: &complytime.VerificationConfig{
			Issuer:   "https://token.actions.githubusercontent.com",
			Identity: "user@example.com",
			Key:      "/path/to/key.pub",
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

// Per-entry verification tests (FR-001, FR-002, FR-004)

func TestLoadFrom_PerEntryVerification(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "complytime.yml")

	yamlContent := `policies:
  - url: registry.com/policies/nist:v1.0
    id: nist
    verification:
      key: "/path/to/vendor.pub"
  - url: ghcr.io/org/cis:v2.0
    id: cis
    skip_verify: true
  - url: quay.io/org/pci:v3.0
    id: pci
    verification:
      issuer: "https://token.actions.githubusercontent.com"
      identity: "https://github.com/org/.github/workflows/release.yml@refs/tags/*"
  - url: registry.com/policies/soc2:v1.0
    id: soc2
targets:
  - id: local
    policies:
      - nist
`
	require.NoError(t, os.WriteFile(
		configPath, []byte(yamlContent), 0600,
	))

	cfg, err := complytime.LoadFrom(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Policies, 4)

	// Entry with keyed verification
	require.NotNil(t, cfg.Policies[0].Verification)
	assert.Equal(t,
		"/path/to/vendor.pub",
		cfg.Policies[0].Verification.Key,
	)
	assert.False(t, cfg.Policies[0].SkipVerify)

	// Entry with skip_verify
	assert.Nil(t, cfg.Policies[1].Verification)
	assert.True(t, cfg.Policies[1].SkipVerify)

	// Entry with keyless verification
	require.NotNil(t, cfg.Policies[2].Verification)
	assert.Equal(t,
		"https://token.actions.githubusercontent.com",
		cfg.Policies[2].Verification.Issuer,
	)
	assert.Equal(t,
		"https://github.com/org/.github/workflows/release.yml@refs/tags/*",
		cfg.Policies[2].Verification.Identity,
	)
	assert.False(t, cfg.Policies[2].SkipVerify)

	// Entry without new fields (backward compat)
	assert.Nil(t, cfg.Policies[3].Verification)
	assert.False(t, cfg.Policies[3].SkipVerify)
}

func TestValidate_PerEntryVerificationMutualExclusivity(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{
				URL:        "registry.com/policies/nist:v1.0",
				ID:         "nist",
				SkipVerify: true,
				Verification: &complytime.VerificationConfig{
					Key: "/path/to/key.pub",
				},
			},
		},
		Targets: []complytime.TargetConfig{{
			ID:       "local",
			Policies: []string{"nist"},
		}},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
	assert.Contains(t, err.Error(), "nist")
}

func TestValidate_PerEntryVerificationConfig(t *testing.T) {
	tests := []struct {
		name    string
		entry   complytime.PolicyEntry
		wantErr string
	}{
		{
			name: "both key and issuer",
			entry: complytime.PolicyEntry{
				URL: "registry.com/policies/nist:v1.0",
				ID:  "nist",
				Verification: &complytime.VerificationConfig{
					Key:      "/path/to/key.pub",
					Issuer:   "https://issuer.example.com",
					Identity: "user@example.com",
				},
			},
			wantErr: "mutually exclusive",
		},
		{
			name: "issuer without identity",
			entry: complytime.PolicyEntry{
				URL: "registry.com/policies/cis:v1.0",
				ID:  "cis",
				Verification: &complytime.VerificationConfig{
					Issuer: "https://issuer.example.com",
				},
			},
			wantErr: "issuer requires identity",
		},
		{
			name: "identity without issuer",
			entry: complytime.PolicyEntry{
				URL: "registry.com/policies/pci:v1.0",
				ID:  "pci",
				Verification: &complytime.VerificationConfig{
					Identity: "user@example.com",
				},
			},
			wantErr: "identity requires issuer",
		},
		{
			name: "trusted_root with key mutual exclusivity",
			entry: complytime.PolicyEntry{
				URL: "registry.com/policies/hipaa:v1.0",
				ID:  "hipaa",
				Verification: &complytime.VerificationConfig{
					TrustedRoot: "/path/to/trusted_root.json",
					Key:         "/path/to/cosign.pub",
				},
			},
			wantErr: "trusted_root cannot be used with key-based verification",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &complytime.WorkspaceConfig{
				Policies: []complytime.PolicyEntry{tt.entry},
				Targets: []complytime.TargetConfig{{
					ID:       "local",
					Policies: []string{tt.entry.ID},
				}},
			}
			err := complytime.Validate(cfg)
			require.Error(t, err)
			// Error must identify the entry
			assert.Contains(t, err.Error(), tt.entry.ID)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidate_PerEntryVerificationBackwardCompat(t *testing.T) {
	// PolicyEntry without Verification or SkipVerify must pass
	// validation unchanged — verifies no regressions.
	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/policies/nist:v1.0", ID: "nist"},
			{URL: "ghcr.io/org/cis:v2.0", ID: "cis"},
		},
		Targets: []complytime.TargetConfig{{
			ID:       "local",
			Policies: []string{"nist"},
		}},
	}
	assert.NoError(t, complytime.Validate(cfg))

	// Verify fields are zero-valued
	assert.Nil(t, cfg.Policies[0].Verification)
	assert.False(t, cfg.Policies[0].SkipVerify)
	assert.Nil(t, cfg.Policies[1].Verification)
	assert.False(t, cfg.Policies[1].SkipVerify)
}

func TestValidate_ValidMixedVerificationConfigs(t *testing.T) {
	// FR-004 scenario: policies from multiple publishers with
	// different verification configs all pass validation, including
	// an entry with a custom trusted root for a private Sigstore instance.
	rootFile := filepath.Join(t.TempDir(), "trusted_root.json")
	require.NoError(t, os.WriteFile(rootFile, []byte(`{}`), 0600))

	cfg := &complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{
			{
				URL: "registry.com/policies/nist:v1.0",
				ID:  "nist",
				Verification: &complytime.VerificationConfig{
					Key: "/path/to/vendor-a.pub",
				},
			},
			{
				URL: "ghcr.io/org/cis:v2.0",
				ID:  "cis",
				Verification: &complytime.VerificationConfig{
					Issuer:   "https://token.actions.githubusercontent.com",
					Identity: "https://github.com/org/.github/workflows/release.yml@refs/tags/*",
				},
			},
			{
				// No entry-level verification — inherits workspace
				URL: "quay.io/org/pci:v3.0",
				ID:  "pci",
			},
			{
				// Explicit skip — no verification for this entry
				URL:        "registry.com/policies/soc2:v1.0",
				ID:         "soc2",
				SkipVerify: true,
			},
			{
				// Per-entry trusted root for private Sigstore deployment
				URL: "registry.internal/policies/hipaa:v1.0",
				ID:  "hipaa",
				Verification: &complytime.VerificationConfig{
					Issuer:      "https://fulcio.internal",
					Identity:    "builder@internal.example.com",
					TrustedRoot: rootFile,
				},
			},
		},
		Targets: []complytime.TargetConfig{{
			ID:       "local",
			Policies: []string{"nist"},
		}},
		Verification: &complytime.VerificationConfig{
			Issuer:   "https://token.actions.githubusercontent.com",
			Identity: "https://github.com/complytime/complyctl/.github/workflows/release.yml@refs/tags/*",
		},
	}
	assert.NoError(t, complytime.Validate(cfg))
}

// --- Custom trusted root config validation tests ---

func TestValidateVerificationConfig_TrustedRootKeyless(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "trusted_root.json")
	require.NoError(t, os.WriteFile(rootPath, []byte(`{}`), 0600))

	v := &complytime.VerificationConfig{
		Issuer:      "https://token.actions.githubusercontent.com",
		Identity:    "https://github.com/org/.github/workflows/release.yml@refs/tags/*",
		TrustedRoot: rootPath,
	}
	err := complytime.ValidateVerificationConfig(v)
	assert.NoError(t, err)
}

func TestValidateVerificationConfig_TrustedRootWithKey(t *testing.T) {
	v := &complytime.VerificationConfig{
		TrustedRoot: "/path/to/trusted_root.json",
		Key:         "/path/to/cosign.pub",
	}
	err := complytime.ValidateVerificationConfig(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(),
		"trusted_root cannot be used with key-based verification")
}

func TestValidateVerificationConfig_TrustedRootWithoutKeyless(t *testing.T) {
	v := &complytime.VerificationConfig{
		TrustedRoot: "/path/to/trusted_root.json",
	}
	err := complytime.ValidateVerificationConfig(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(),
		"trusted_root requires issuer and identity for keyless verification")
}

func TestValidateVerificationConfig_TrustedRootFileNotFound(t *testing.T) {
	v := &complytime.VerificationConfig{
		Issuer:      "https://issuer.example.com",
		Identity:    "user@example.com",
		TrustedRoot: "/nonexistent/path/trusted_root.json",
	}
	err := complytime.ValidateVerificationConfig(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trusted_root file")
}

func TestValidateVerificationConfig_TrustedRootFileExists(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "trusted_root.json")
	require.NoError(t, os.WriteFile(rootPath, []byte(`{}`), 0600))

	v := &complytime.VerificationConfig{
		Issuer:      "https://issuer.example.com",
		Identity:    "user@example.com",
		TrustedRoot: rootPath,
	}
	err := complytime.ValidateVerificationConfig(v)
	assert.NoError(t, err)
}

func TestValidateVerificationConfig_TrustedRootPathCleaned(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "trusted_root.json")
	require.NoError(t, os.WriteFile(rootPath, []byte(`{}`), 0600))

	// Path with non-canonical elements that filepath.Clean resolves
	dirtyPath := filepath.Join(dir, "..", filepath.Base(dir), "trusted_root.json")

	v := &complytime.VerificationConfig{
		Issuer:      "https://issuer.example.com",
		Identity:    "user@example.com",
		TrustedRoot: dirtyPath,
	}
	err := complytime.ValidateVerificationConfig(v)
	assert.NoError(t, err)
	// Validation stores the cleaned path so downstream consumers
	// use the same value that was validated.
	assert.Equal(t, rootPath, v.TrustedRoot,
		"TrustedRoot should be cleaned in-place after validation")
}

func TestLoadFrom_WithTrustedRoot(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "trusted_root.json")
	require.NoError(t, os.WriteFile(rootPath, []byte(`{}`), 0600))
	configPath := filepath.Join(dir, "complytime.yml")

	yamlContent := fmt.Sprintf(`policies:
  - url: registry.com/policies/nist:v1.0
    id: nist
targets:
  - id: local
    policies:
      - nist
verification:
  issuer: "https://token.actions.githubusercontent.com"
  identity: "https://github.com/org/.github/workflows/release.yml@refs/tags/*"
  trusted_root: %q
`, rootPath)
	require.NoError(t, os.WriteFile(configPath, []byte(yamlContent), 0600))

	cfg, err := complytime.LoadFrom(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Verification)
	assert.Equal(t, rootPath, cfg.Verification.TrustedRoot)
	assert.Equal(t, "https://token.actions.githubusercontent.com", cfg.Verification.Issuer)
	assert.Equal(t, "https://github.com/org/.github/workflows/release.yml@refs/tags/*", cfg.Verification.Identity)
}

// TestLoadFrom_PerEntryTrustedRoot verifies YAML round-trip for per-entry
// trusted_root. The path is intentionally nonexistent because LoadFrom only
// deserializes YAML; file existence is validated later by Validate().
func TestLoadFrom_PerEntryTrustedRoot(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "complytime.yml")

	yamlContent := `policies:
  - url: registry.com/policies/nist:v1.0
    id: nist
    verification:
      issuer: "https://issuer.example.com"
      identity: "user@example.com"
      trusted_root: "/path/to/entry_root.json"
targets:
  - id: local
    policies:
      - nist
`
	require.NoError(t, os.WriteFile(configPath, []byte(yamlContent), 0600))

	cfg, err := complytime.LoadFrom(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Policies, 1)
	require.NotNil(t, cfg.Policies[0].Verification)
	assert.Equal(t, "/path/to/entry_root.json", cfg.Policies[0].Verification.TrustedRoot)
}

func TestFilenameSafe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no slashes", "test-policy", "test-policy"},
		{"single slash", "org/policy", "org-policy"},
		{"multiple slashes", "org/sub/policy", "org-sub-policy"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, complytime.FilenameSafe(tt.input))
		})
	}
}

func TestExpandPath(t *testing.T) {
	t.Run("tilde prefix", func(t *testing.T) {
		result := complytime.ExpandPath("~/test")
		assert.NotEqual(t, "~/test", result)
		assert.True(t, strings.HasSuffix(result, "/test"))
	})
	t.Run("no tilde", func(t *testing.T) {
		assert.Equal(t, "/absolute/path", complytime.ExpandPath("/absolute/path"))
	})
	t.Run("relative path", func(t *testing.T) {
		assert.Equal(t, "relative/path", complytime.ExpandPath("relative/path"))
	})
}

func TestResolveCacheDir(t *testing.T) {
	dir, err := complytime.ResolveCacheDir()
	require.NoError(t, err)
	assert.NotEmpty(t, dir)
	assert.Contains(t, dir, "complytime")
}

func TestResolveProviderDir(t *testing.T) {
	dir, err := complytime.ResolveProviderDir()
	require.NoError(t, err)
	assert.NotEmpty(t, dir)
	assert.Contains(t, dir, "complytime")
	assert.Contains(t, dir, "providers")
}

func TestValidate_DuplicatePolicyInTarget(t *testing.T) {
	cfg := &complytime.WorkspaceConfig{
		Version: 1,
		Policies: []complytime.PolicyEntry{
			{URL: "registry.com/policies/nist:v1.0", ID: "nist"},
		},
		Targets: []complytime.TargetConfig{
			{
				ID:       "prod",
				Policies: []string{"nist", "nist"},
			},
		},
	}
	err := complytime.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate policy")
}
