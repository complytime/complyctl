// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2"
	ocistore "oras.land/oras-go/v2/content/oci"
)

func TestLocalPolicySourceCopyPolicy_ContextDeadlineLeavesDestinationEmpty(t *testing.T) {
	const maximumWait = time.Second

	layoutPath, manifestDigest := createBlockingTransferLayout(t)
	cacheDir := t.TempDir()
	dataDir := t.TempDir()
	state, err := LoadState(dataDir)
	require.NoError(t, err)

	transferStarted := make(chan struct{})
	source := newLocalPolicySource(layoutPath, func(ctx context.Context, layoutFS fs.FS) (oras.ReadOnlyTarget, error) {
		store, storeErr := ocistore.NewFromFS(ctx, layoutFS)
		if storeErr != nil {
			return nil, storeErr
		}
		return &blockingTransferTarget{ReadOnlyTarget: store, transferStarted: transferStarted}, nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	syncer := NewSync(NewCache(cacheDir), state, source, dataDir, "")

	started := time.Now()
	errCh := make(chan error, 1)
	go func() {
		_, syncErr := syncer.SyncPolicy(ctx, "policies/test", manifestDigest)
		errCh <- syncErr
	}()

	select {
	case <-transferStarted:
	case <-time.After(maximumWait):
		t.Fatal("local OCI copy did not begin transfer")
	}
	err = <-errCh
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(started), maximumWait)

	assertNoOCIContent(t, filepath.Join(cacheDir, "policies", "policies", "test"))
	_, stateExists := state.GetPolicyState("policies/test")
	assert.False(t, stateExists)
}

func TestSyncPolicy_ContextCancellationPreservesExistingCacheAndState(t *testing.T) {
	ctx := context.Background()
	cacheDir := t.TempDir()
	dataDir := t.TempDir()
	policyID := "policies/test"
	cacheStore, err := NewCache(cacheDir).NewPolicyStore(policyID)
	require.NoError(t, err)
	previousContent := []byte("previous policy content")
	previousDescriptor := ocispec.Descriptor{
		MediaType: "application/vnd.complytime.policy.layer.v1",
		Digest:    digest.FromBytes(previousContent),
		Size:      int64(len(previousContent)),
	}
	require.NoError(t, cacheStore.Push(ctx, previousDescriptor, bytes.NewReader(previousContent)))

	state, err := LoadState(dataDir)
	require.NoError(t, err)
	previousStateDigest := digest.FromString("previous policy state").String()
	require.NoError(t, state.UpdatePolicyStateWithVerification(policyID, "previous", previousStateDigest, nil))
	require.NoError(t, SaveState(state, dataDir))
	previousState, exists := state.GetPolicyState(policyID)
	require.True(t, exists)

	transferStarted := make(chan struct{})
	source := cancelDuringFinalTransferSource{transferStarted: transferStarted}
	cancelCtx, cancel := context.WithCancel(t.Context())
	syncer := NewSync(NewCache(cacheDir), state, source, dataDir, "")
	errCh := make(chan error, 1)
	go func() {
		_, syncErr := syncer.SyncPolicy(cancelCtx, policyID, source.version())
		errCh <- syncErr
	}()

	<-transferStarted
	cancel()
	require.ErrorIs(t, <-errCh, context.Canceled)

	cacheStore, err = NewCache(cacheDir).NewPolicyStore(policyID)
	require.NoError(t, err)
	exists, err = cacheStore.Exists(ctx, previousDescriptor)
	require.NoError(t, err)
	assert.True(t, exists)
	updatedState, exists := state.GetPolicyState(policyID)
	require.True(t, exists)
	assert.Equal(t, previousState, updatedState)
	persistedState, err := LoadState(dataDir)
	require.NoError(t, err)
	persistedPolicyState, exists := persistedState.GetPolicyState(policyID)
	require.True(t, exists)
	assert.Equal(t, previousState.Version, persistedPolicyState.Version)
	assert.Equal(t, previousState.Digest, persistedPolicyState.Digest)
	assert.Equal(t, previousState.LastUpdated.UTC(), persistedPolicyState.LastUpdated)
}

type cancelDuringFinalTransferSource struct {
	transferStarted chan<- struct{}
}

func (s cancelDuringFinalTransferSource) DefinitionVersion(_ context.Context, _ string) (string, string, error) {
	return s.version(), s.version(), nil
}

func (s cancelDuringFinalTransferSource) CopyPolicy(
	ctx context.Context,
	_ string,
	_ string,
	dst *ocistore.Store,
) (ocispec.Descriptor, error) {
	partialContent := []byte("partial policy content")
	partialDescriptor := ocispec.Descriptor{
		MediaType: "application/vnd.complytime.policy.layer.v1",
		Digest:    digest.FromBytes(partialContent),
		Size:      int64(len(partialContent)),
	}
	if err := dst.Push(ctx, partialDescriptor, bytes.NewReader(partialContent)); err != nil {
		return ocispec.Descriptor{}, err
	}
	close(s.transferStarted)
	<-ctx.Done()
	return ocispec.Descriptor{}, ctx.Err()
}

func (s cancelDuringFinalTransferSource) version() string {
	return digest.FromString("new policy state").String()
}

func createBlockingTransferLayout(t *testing.T) (string, string) {
	t.Helper()
	layoutPath := t.TempDir()
	store, err := ocistore.New(layoutPath)
	require.NoError(t, err)
	content := []byte("policy content")
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
	return layoutPath, manifest.Digest.String()
}

type blockingTransferTarget struct {
	oras.ReadOnlyTarget
	transferStarted chan struct{}
	fetches         int
}

func (t *blockingTransferTarget) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	t.fetches++
	reader, err := t.ReadOnlyTarget.Fetch(ctx, target)
	if err != nil || t.fetches == 1 {
		return reader, err
	}
	return &blockingReadCloser{ReadCloser: reader, ctx: ctx, started: t.transferStarted, closed: make(chan struct{})}, nil
}

type blockingReadCloser struct {
	io.ReadCloser
	ctx     context.Context
	started chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	case <-r.closed:
		return 0, context.Canceled
	}
}

func (r *blockingReadCloser) Close() error {
	r.once.Do(func() { close(r.started) })
	select {
	case <-r.closed:
	default:
		close(r.closed)
	}
	return r.ReadCloser.Close()
}

func assertNoOCIContent(t *testing.T, layoutPath string) {
	t.Helper()

	blobRoot := filepath.Join(layoutPath, ocispec.ImageBlobsDir)
	err := filepath.WalkDir(blobRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != blobRoot && !entry.IsDir() {
			return errors.New("OCI content exists")
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	require.NoError(t, err)
}
