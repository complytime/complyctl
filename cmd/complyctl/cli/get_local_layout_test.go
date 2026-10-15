// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/complytime/complypack/pkg/complypack"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2"
	ocistore "oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/registry"
)

func TestGetCmd_FromOCILayoutFlag(t *testing.T) {
	command := getCmd(&Common{})

	flag := command.Flags().Lookup("from-oci-layout")

	require.NotNil(t, flag)
	assert.Equal(t, "Import configured artifacts from a local OCI image layout", flag.Usage)
}

func TestValidateLocalOCILayout_InvalidPaths(t *testing.T) {
	tests := []struct {
		name      string
		prepare   func(t *testing.T) string
		errorPart string
	}{
		{
			name: "missing path",
			prepare: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "missing")
			},
			errorPart: "no such file or directory",
		},
		{
			name: "missing index",
			prepare: func(t *testing.T) string {
				path := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(path, "oci-layout"),
					[]byte(`{"imageLayoutVersion":"1.0.0"}`),
					0o600,
				))
				return path
			},
			errorPart: "readable index.json",
		},
		{
			name: "invalid marker",
			prepare: func(t *testing.T) string {
				path := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(path, "oci-layout"), []byte("not json"), 0o600,
				))
				require.NoError(t, os.WriteFile(
					filepath.Join(path, "index.json"), []byte(`{"schemaVersion":2}`), 0o600,
				))
				return path
			},
			errorPart: "invalid local OCI layout",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateLocalOCILayout(context.Background(), test.prepare(t))

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.errorPart)
		})
	}
}

func TestValidateLocalOCILayout_ContextCancellationDuringMetadataRead(t *testing.T) {
	originalReadFile := localLayoutReadFile
	t.Cleanup(func() { localLayoutReadFile = originalReadFile })
	readStarted := make(chan struct{})
	releaseRead := make(chan struct{})
	localLayoutReadFile = func(fs.FS, string) ([]byte, error) {
		close(readStarted)
		<-releaseRead
		return nil, context.Canceled
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	layoutPath := t.TempDir()

	errCh := make(chan error, 1)
	go func() { errCh <- validateLocalOCILayout(ctx, layoutPath) }()
	<-readStarted
	cancel()

	require.ErrorIs(t, <-errCh, context.Canceled)
	close(releaseRead)
}

func TestValidateLocalOCILayout_RejectsOversizedMetadata(t *testing.T) {
	layoutPath := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(layoutPath, "oci-layout"),
		[]byte(`{"imageLayoutVersion":"1.0.0"}`),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(layoutPath, "index.json"),
		bytes.Repeat([]byte("x"), complytime.MaxOCILayoutMetadataSize+1),
		0o600,
	))

	err := validateLocalOCILayout(context.Background(), layoutPath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestValidateLocalOCILayout_RejectsEscapingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}
	layoutPath := t.TempDir()
	externalPath := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(layoutPath, "oci-layout"),
		[]byte(`{"imageLayoutVersion":"1.0.0"}`),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(externalPath, "index.json"), []byte(`{"schemaVersion":2}`), 0o600,
	))
	require.NoError(t, os.Symlink(
		filepath.Join(externalPath, "index.json"),
		filepath.Join(layoutPath, "index.json"),
	))

	err := validateLocalOCILayout(context.Background(), layoutPath)

	require.Error(t, err)
}

func TestValidateLocalOCILayout_IgnoresUnrelatedManifestContent(t *testing.T) {
	layoutPath, digestValue := createLocalPolicyLayoutForCLI(t)
	indexPath := filepath.Join(layoutPath, "index.json")
	indexBytes, err := os.ReadFile(indexPath)
	require.NoError(t, err)
	var index ocispec.Index
	require.NoError(t, json.Unmarshal(indexBytes, &index))
	index.Manifests = append(index.Manifests, ocispec.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    digest.FromString("unrelated"),
		Size:      1,
	})
	indexBytes, err = json.Marshal(index)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(indexPath, indexBytes, 0o600))

	require.NoError(t, validateLocalOCILayout(context.Background(), layoutPath))
	options := &getOptions{cacheDir: t.TempDir(), dataDir: t.TempDir(), fromOCILayout: layoutPath}
	config := &complytime.WorkspaceConfig{Policies: []complytime.PolicyEntry{{
		ID: "selected", URL: "registry.invalid/policies/selected@" + digestValue,
	}}}
	require.NoError(t, options.syncPolicies(context.Background(), config, nil, nil))
}

func TestSyncPolicies_LocalLayoutRejectsVerifiedEntryAndImportsEligibleEntry(t *testing.T) {
	layoutPath, digestValue := createLocalPolicyLayoutForCLI(t)
	guard := installLocalModeInitializerGuards(t)
	dataDir := t.TempDir()
	options := &getOptions{
		cacheDir:      t.TempDir(),
		dataDir:       dataDir,
		fromOCILayout: layoutPath,
	}
	config := &complytime.WorkspaceConfig{Policies: []complytime.PolicyEntry{
		{
			ID:  "rejected",
			URL: "registry.example.test/policies/rejected@" + digestValue,
			Verification: &complytime.VerificationConfig{
				Key: "/keys/rejected.pub",
			},
		},
		{
			ID:  "eligible",
			URL: "registry.example.test/policies/eligible@" + digestValue,
		},
	}}

	err := options.syncPolicies(context.Background(), config, nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "offline signature verification is not supported")
	assertLocalModeInitializersUnused(t, guard)
	state, stateErr := cache.LoadState(dataDir)
	require.NoError(t, stateErr)
	_, rejected := state.GetPolicyState("policies/rejected")
	eligible, imported := state.GetPolicyState("policies/eligible")
	assert.False(t, rejected)
	require.True(t, imported)
	assert.Equal(t, digestValue, eligible.Digest)
	assert.False(t, eligible.Verified)
}

func TestSyncComplypacks_LocalLayoutRejectsVerifiedEntryAndImportsEligibleEntry(t *testing.T) {
	layoutPath, digestValue := createLocalComplypackLayoutForCLI(t)
	guard := installLocalModeInitializerGuards(t)
	options := &getOptions{
		cacheDir:      t.TempDir(),
		dataDir:       t.TempDir(),
		fromOCILayout: layoutPath,
	}
	config := &complytime.WorkspaceConfig{Complypacks: []complytime.PolicyEntry{
		{
			ID:  "rejected",
			URL: "registry.example.test/packs/rejected@" + digestValue,
			Verification: &complytime.VerificationConfig{
				Key: "/keys/rejected.pub",
			},
		},
		{
			ID:  "eligible",
			URL: "registry.example.test/packs/eligible@" + digestValue,
		},
	}}

	err := options.syncComplypacks(context.Background(), config, t.TempDir(), nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "offline signature verification is not supported")
	assertLocalModeInitializersUnused(t, guard)
	state, stateErr := cache.LoadState(options.dataDir)
	require.NoError(t, stateErr)
	_, rejected := state.GetComplypackState("packs/rejected")
	eligible, imported := state.GetComplypackState("packs/eligible")
	assert.False(t, rejected)
	require.True(t, imported)
	assert.Equal(t, digestValue, eligible.Digest)
	assert.False(t, eligible.Verified)
}

func TestSyncLocalLayout_TagOnlyEntriesFailWhileDigestPinnedEntriesImport(t *testing.T) {
	t.Run("policy", func(t *testing.T) {
		layoutPath, digestValue := createLocalPolicyLayoutForCLI(t)
		options := &getOptions{
			cacheDir:      t.TempDir(),
			dataDir:       t.TempDir(),
			fromOCILayout: layoutPath,
		}
		config := &complytime.WorkspaceConfig{Policies: []complytime.PolicyEntry{
			{ID: "pinned", URL: "registry.invalid/policies/pinned@" + digestValue},
			{ID: "tag-only", URL: "registry.invalid/policies/tag-only:latest"},
		}}

		err := options.syncPolicies(context.Background(), config, nil, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "digest-pinned")
		state, stateErr := cache.LoadState(options.dataDir)
		require.NoError(t, stateErr)
		pinned, found := state.GetPolicyState("policies/pinned")
		require.True(t, found)
		assert.Equal(t, digestValue, pinned.Digest)
		_, found = state.GetPolicyState("policies/tag-only")
		assert.False(t, found)
		assert.NoDirExists(t, filepath.Join(options.cacheDir, "policies", "policies", "tag-only"))
	})

	t.Run("complypack", func(t *testing.T) {
		layoutPath, digestValue := createLocalComplypackLayoutForCLI(t)
		options := &getOptions{
			cacheDir:      t.TempDir(),
			dataDir:       t.TempDir(),
			fromOCILayout: layoutPath,
		}
		config := &complytime.WorkspaceConfig{Complypacks: []complytime.PolicyEntry{
			{ID: "pinned", URL: "registry.invalid/packs/pinned@" + digestValue},
			{ID: "tag-only", URL: "registry.invalid/packs/tag-only:latest"},
		}}

		err := options.syncComplypacks(context.Background(), config, t.TempDir(), nil, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "digest-pinned")
		state, stateErr := cache.LoadState(options.dataDir)
		require.NoError(t, stateErr)
		pinned, found := state.GetComplypackState("packs/pinned")
		require.True(t, found)
		assert.Equal(t, digestValue, pinned.Digest)
		_, found = state.GetComplypackState("packs/tag-only")
		assert.False(t, found)
	})
}

func TestSyncComplypacks_LocalLayoutDoesNotServeStaleCacheAfterSourceError(t *testing.T) {
	layoutPath, digestValue := createLocalComplypackLayoutForCLI(t)
	options := &getOptions{cacheDir: t.TempDir(), dataDir: t.TempDir(), fromOCILayout: layoutPath}
	validConfig := &complytime.WorkspaceConfig{Complypacks: []complytime.PolicyEntry{{
		ID: "pack", URL: "registry.invalid/packs/pack@" + digestValue,
	}}}
	require.NoError(t, options.syncComplypacks(context.Background(), validConfig, t.TempDir(), nil, nil))

	for _, test := range []struct {
		name string
		url  string
	}{
		{name: "missing digest", url: "registry.invalid/packs/pack@" + digest.FromString("missing").String()},
		{name: "tag only", url: "registry.invalid/packs/pack:latest"},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := &complytime.WorkspaceConfig{Complypacks: []complytime.PolicyEntry{{ID: "pack", URL: test.url}}}
			err := options.syncComplypacks(context.Background(), config, t.TempDir(), nil, nil)
			require.Error(t, err)
		})
	}
}

func TestSyncPolicies_LocalLayoutSkipVerifyImportsConfiguredEntryWithoutVerifier(t *testing.T) {
	layoutPath, digestValue := createLocalPolicyLayoutForCLI(t)
	guard := installLocalModeInitializerGuards(t)
	options := &getOptions{cacheDir: t.TempDir(), dataDir: t.TempDir(), fromOCILayout: layoutPath, skipVerify: true}
	config := &complytime.WorkspaceConfig{Policies: []complytime.PolicyEntry{{
		ID:           "verified-policy",
		URL:          "registry.invalid/policies/verified-policy@" + digestValue,
		Verification: &complytime.VerificationConfig{Key: "/keys/verified.pub"},
	}}}

	require.NoError(t, options.syncAll(context.Background(), config, t.TempDir(), nil, nil))
	assertLocalModeInitializersUnused(t, guard)
	state, err := cache.LoadState(options.dataDir)
	require.NoError(t, err)
	entry, found := state.GetPolicyState("policies/verified-policy")
	require.True(t, found)
	assert.False(t, entry.Verified)
}

func TestSyncPolicies_LocalLayout_SkipsUnchangedDigestAndReplacesChangedDigest(t *testing.T) {
	layoutPath := t.TempDir()
	digestA := addLocalPolicyArtifactForCLI(t, layoutPath, []byte("policy content A"))
	digestB := addLocalPolicyArtifactForCLI(t, layoutPath, []byte("policy content B"))
	options := &getOptions{
		cacheDir:      t.TempDir(),
		dataDir:       t.TempDir(),
		fromOCILayout: layoutPath,
	}

	config := func(digestValue string) *complytime.WorkspaceConfig {
		return &complytime.WorkspaceConfig{Policies: []complytime.PolicyEntry{{
			ID:  "local-policy",
			URL: "registry.invalid/policies/local-policy@" + digestValue,
		}}}
	}
	repository := "policies/local-policy"
	storePath := filepath.Join(options.cacheDir, "policies", repository)

	require.NoError(t, options.syncPolicies(context.Background(), config(digestA), nil, nil))
	statePath := filepath.Join(options.dataDir, complytime.StateFileName)
	stateBeforeSkip, err := os.ReadFile(statePath)
	require.NoError(t, err)
	indexBeforeSkip, err := os.ReadFile(filepath.Join(storePath, "index.json"))
	require.NoError(t, err)

	require.NoError(t, options.syncPolicies(context.Background(), config(digestA), nil, nil))
	stateAfterSkip, err := os.ReadFile(statePath)
	require.NoError(t, err)
	indexAfterSkip, err := os.ReadFile(filepath.Join(storePath, "index.json"))
	require.NoError(t, err)
	assert.Equal(t, stateBeforeSkip, stateAfterSkip)
	assert.Equal(t, indexBeforeSkip, indexAfterSkip)

	require.NoError(t, options.syncPolicies(context.Background(), config(digestB), nil, nil))
	state, err := cache.LoadState(options.dataDir)
	require.NoError(t, err)
	policyState, found := state.GetPolicyState(repository)
	require.True(t, found)
	assert.Equal(t, digestB, policyState.Digest)
	assert.Equal(t, digestB, policyState.Version)
	assert.NotEqual(t, digestA, policyState.Digest)
	replacementIndex, err := os.ReadFile(filepath.Join(storePath, "index.json"))
	require.NoError(t, err)
	assert.Contains(t, string(replacementIndex), digestB)
	assert.NotEqual(t, indexBeforeSkip, replacementIndex)
}

type localModeInitializerGuard struct {
	credentialCalls int
	registryCalls   int
	verifierCalls   int
}

func installLocalModeInitializerGuards(t *testing.T) *localModeInitializerGuard {
	t.Helper()
	guard := &localModeInitializerGuard{}
	originalCredentialBuilder := defaultCredentialFuncBuilder
	originalRegistryBuilder := defaultRegistryClientBuilder
	originalVerifierBuilder := defaultVerifierBuilder
	t.Cleanup(func() {
		defaultCredentialFuncBuilder = originalCredentialBuilder
		defaultRegistryClientBuilder = originalRegistryBuilder
		defaultVerifierBuilder = originalVerifierBuilder
	})
	defaultCredentialFuncBuilder = func() (auth.CredentialFunc, error) {
		guard.credentialCalls++
		return func(context.Context, string) (auth.Credential, error) {
			return auth.EmptyCredential, nil
		}, nil
	}
	defaultRegistryClientBuilder = func(host string, credentials auth.CredentialFunc) *registry.Client {
		guard.registryCalls++
		return registry.NewClient(host, credentials)
	}
	defaultVerifierBuilder = func(complytime.VerificationConfig) (cache.VerifyFunc, error) {
		guard.verifierCalls++
		return mockVerifyFunc(), nil
	}
	return guard
}

func assertLocalModeInitializersUnused(t *testing.T, guard *localModeInitializerGuard) {
	t.Helper()
	assert.Zero(t, guard.credentialCalls)
	assert.Zero(t, guard.registryCalls)
	assert.Zero(t, guard.verifierCalls)
}

func createLocalPolicyLayoutForCLI(t *testing.T) (string, string) {
	t.Helper()
	layoutPath := t.TempDir()
	return layoutPath, addLocalPolicyArtifactForCLI(t, layoutPath, []byte("policy content"))
}

func addLocalPolicyArtifactForCLI(t *testing.T, layoutPath string, content []byte) string {
	t.Helper()
	store, err := ocistore.New(layoutPath)
	require.NoError(t, err)
	layer := ocispec.Descriptor{
		MediaType: "application/vnd.complytime.policy.layer.v1",
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
	}
	require.NoError(t, store.Push(context.Background(), layer, bytes.NewReader(content)))
	manifest, err := oras.PackManifest(
		context.Background(), store, oras.PackManifestVersion1_1,
		"application/vnd.complytime.policy.v1", oras.PackManifestOptions{Layers: []ocispec.Descriptor{layer}},
	)
	require.NoError(t, err)
	return manifest.Digest.String()
}

func createLocalComplypackLayoutForCLI(t *testing.T) (string, string) {
	t.Helper()
	layoutPath := t.TempDir()
	store, err := ocistore.New(layoutPath)
	require.NoError(t, err)
	manifest, err := complypack.Pack(
		context.Background(), store,
		complypack.Config{ID: "io.complytime.test-pack", EvaluatorID: "io.complytime.test", Version: "1.0.0"},
		bytes.NewReader([]byte("complypack content")),
	)
	require.NoError(t, err)
	return layoutPath, manifest.Digest.String()
}
