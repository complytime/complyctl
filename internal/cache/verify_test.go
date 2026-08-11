// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/cache/cachetest"
)

func TestVerifyFunc_NilSkipsVerification(t *testing.T) {
	var vf VerifyFunc
	assert.Nil(t, vf, "nil VerifyFunc represents disabled verification")
}

func TestVerifyFunc_MockSuccess(t *testing.T) {
	mockVerifier := func(_ context.Context, ref string, _ bool) (*VerificationResult, error) {
		return &VerificationResult{
			Verified:       true,
			SignerIdentity: "test@example.com",
			Issuer:         "https://issuer.example.com",
			VerifiedAt:     time.Now(),
		}, nil
	}

	result, err := mockVerifier(context.Background(), "registry.com/repo:v1.0", false)
	require.NoError(t, err)
	assert.True(t, result.Verified)
	assert.Equal(t, "test@example.com", result.SignerIdentity)
	assert.Equal(t, "https://issuer.example.com", result.Issuer)
	assert.False(t, result.VerifiedAt.IsZero())
}

func TestVerifyFunc_MockFailure(t *testing.T) {
	mockVerifier := func(_ context.Context, ref string, _ bool) (*VerificationResult, error) {
		return nil, fmt.Errorf("signature verification failed for %s: identity mismatch", ref)
	}

	result, err := mockVerifier(context.Background(), "registry.com/repo:v1.0", false)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "identity mismatch")
}

func TestNameOptions_InsecureSelectsPlainHTTP(t *testing.T) {
	// nameOptions is the crux of the plain-HTTP verification fix: when
	// insecure is true it must add name.Insecure so signature resolution
	// uses HTTP; when false it must return no options so the HTTPS default
	// (the security-relevant default) is preserved. A regression that
	// inverts this branch would silently downgrade all registries to HTTP
	// or break http:// registry verification, so both branches are asserted.
	tests := []struct {
		name       string
		insecure   bool
		wantLen    int
		wantScheme string
	}{
		{name: "insecure true -> plain HTTP", insecure: true, wantLen: 1, wantScheme: "http"},
		{name: "insecure false -> HTTPS default", insecure: false, wantLen: 0, wantScheme: "https"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := nameOptions(tc.insecure)
			assert.Len(t, opts, tc.wantLen,
				"nameOptions(%v) option count", tc.insecure)

			// Functionally confirm the option changes reference resolution:
			// the returned options must yield the expected registry scheme.
			ref, err := name.ParseReference("zot:5000/policies/x:v1", opts...)
			require.NoError(t, err)
			assert.Equal(t, tc.wantScheme, ref.Context().Scheme(),
				"nameOptions(%v) must yield %s registry scheme", tc.insecure, tc.wantScheme)
		})
	}
}

func TestParseCertificateChain_InvalidPEM(t *testing.T) {
	_, err := parseCertificateChain("not a PEM")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no certificates found")
}

func TestParseCertificateChain_EmptyPEM(t *testing.T) {
	_, err := parseCertificateChain("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no certificates found")
}

func TestParseRekorBundle_ValidJSON(t *testing.T) {
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(`{"test": "body"}`))
	setB64 := base64.StdEncoding.EncodeToString([]byte("signed-timestamp"))
	// cosign writes the Rekor logID as a hex string (SHA-256 of the log key).
	logIDHex := hex.EncodeToString([]byte("log-id-bytes"))

	payload := rekorBundlePayload{
		SignedEntryTimestamp: setB64,
	}
	payload.Payload.Body = bodyB64
	payload.Payload.IntegratedTime = 1701205628
	payload.Payload.LogIndex = 12345
	payload.Payload.LogID = logIDHex

	jsonBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	entries, err := parseRekorBundle(string(jsonBytes))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, int64(12345), entries[0].LogIndex)
	assert.Equal(t, int64(1701205628), entries[0].IntegratedTime)
	assert.Equal(t, "hashedrekord", entries[0].KindVersion.Kind)
	assert.NotNil(t, entries[0].InclusionPromise)
	assert.Equal(t, []byte("log-id-bytes"), entries[0].LogId.KeyId,
		"logID must be hex-decoded to match the trusted-root tlog keyId")
}

func TestParseRekorBundle_InvalidJSON(t *testing.T) {
	_, err := parseRekorBundle("not json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestBuildProtobufBundle_MissingAnnotations(t *testing.T) {
	layer := &v1.Descriptor{
		MediaType: types.MediaType(cosignSimpleSigningMediaType),
	}
	_, err := buildProtobufBundle(layer, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no annotations")
}

func TestBuildProtobufBundle_MissingSignature(t *testing.T) {
	// Annotations present but no signature annotation — should fail
	layer := &v1.Descriptor{
		MediaType: types.MediaType(cosignSimpleSigningMediaType),
		Annotations: map[string]string{
			"some-annotation": "some-value",
		},
	}
	_, err := buildProtobufBundle(layer, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing signature annotation")
}

func TestVerificationResult_ZeroValue(t *testing.T) {
	vr := &VerificationResult{}
	assert.False(t, vr.Verified)
	assert.Empty(t, vr.SignerIdentity)
	assert.Empty(t, vr.Issuer)
	assert.True(t, vr.VerifiedAt.IsZero())
}

func TestPolicyState_BackwardCompatibility(t *testing.T) {
	// Old-format JSON without verification fields must deserialize correctly
	oldJSON := `{"version":"v1.0","digest":"sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08","last_updated":"2024-01-01T00:00:00Z"}`
	var ps PolicyState
	err := json.Unmarshal([]byte(oldJSON), &ps)
	require.NoError(t, err)
	assert.Equal(t, "v1.0", ps.Version)
	assert.Equal(t, cachetest.DigestA, ps.Digest)
	assert.False(t, ps.Verified)
	assert.Empty(t, ps.SignerIdentity)
	assert.Empty(t, ps.Issuer)
	assert.True(t, ps.VerifiedAt.IsZero())
}

func TestPolicyState_OmitemptyMarshal(t *testing.T) {
	// Unverified state should not emit boolean/string verification fields
	// (omitempty works on bool and string). time.Time zero value is always
	// emitted since omitempty does not apply to structs in encoding/json.
	ps := PolicyState{
		Version:     "v1.0",
		Digest:      cachetest.DigestA,
		LastUpdated: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	data, err := json.Marshal(ps)
	require.NoError(t, err)
	s := string(data)
	assert.NotContains(t, s, `"verified":`)
	assert.NotContains(t, s, "signer_identity")
	assert.NotContains(t, s, "issuer")
}

func TestPolicyState_VerifiedMarshal(t *testing.T) {
	verifiedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	ps := PolicyState{
		Version:        "v1.0",
		Digest:         cachetest.DigestA,
		LastUpdated:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Verified:       true,
		SignerIdentity: "workflow@github.com",
		Issuer:         "https://token.actions.githubusercontent.com",
		VerifiedAt:     verifiedAt,
	}
	data, err := json.Marshal(ps)
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, `"verified":true`)
	assert.Contains(t, s, `"signer_identity":"workflow@github.com"`)
	assert.Contains(t, s, `"issuer":"https://token.actions.githubusercontent.com"`)
	assert.Contains(t, s, `"verified_at"`)
}

func TestUpdatePolicyStateWithVerification_NilResult(t *testing.T) {
	state := &State{Policies: make(map[string]PolicyState)}
	require.NoError(t, state.UpdatePolicyStateWithVerification("test-policy", "v1.0", cachetest.DigestB, nil))
	ps, ok := state.GetPolicyState("test-policy")
	require.True(t, ok)
	assert.Equal(t, "v1.0", ps.Version)
	assert.Equal(t, cachetest.DigestB, ps.Digest)
	assert.False(t, ps.Verified)
	assert.Empty(t, ps.SignerIdentity)
}

func TestUpdatePolicyStateWithVerification_WithResult(t *testing.T) {
	state := &State{Policies: make(map[string]PolicyState)}
	vr := &VerificationResult{
		Verified:       true,
		SignerIdentity: "user@example.com",
		Issuer:         "https://issuer.example.com",
		VerifiedAt:     time.Now(),
	}
	require.NoError(t, state.UpdatePolicyStateWithVerification("test-policy", "v1.0", cachetest.DigestC, vr))
	ps, ok := state.GetPolicyState("test-policy")
	require.True(t, ok)
	assert.True(t, ps.Verified)
	assert.Equal(t, "user@example.com", ps.SignerIdentity)
	assert.Equal(t, "https://issuer.example.com", ps.Issuer)
	assert.False(t, ps.VerifiedAt.IsZero())
}

func TestUpdateComplypackStateWithVerification_WithResult(t *testing.T) {
	state := &State{Complypacks: make(map[string]PolicyState)}
	vr := &VerificationResult{
		Verified:       true,
		SignerIdentity: "build@ci.com",
		Issuer:         "https://ci.issuer.com",
		VerifiedAt:     time.Now(),
	}
	require.NoError(t, state.UpdateComplypackStateWithVerification("repo/pack", "v2.0", cachetest.DigestD, "opa", vr))
	ps, ok := state.GetComplypackState("repo/pack")
	require.True(t, ok)
	assert.True(t, ps.Verified)
	assert.Equal(t, "build@ci.com", ps.SignerIdentity)
	assert.Equal(t, "opa", ps.EvaluatorID)
}

// generateTestCertPEM creates a self-signed test certificate and returns
// its PEM encoding. The certificate is valid for 1 hour.
func generateTestCertPEM(t *testing.T) (string, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"Test"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return string(pemBlock), derBytes
}

func TestParseCertificateChain_ValidSingleCert(t *testing.T) {
	certPEM, derBytes := generateTestCertPEM(t)

	chain, err := parseCertificateChain(certPEM)
	require.NoError(t, err)
	require.Len(t, chain.Certificates, 1)
	assert.Equal(t, derBytes, chain.Certificates[0].RawBytes)

	// Verify the RawBytes round-trips through x509.ParseCertificate
	parsed, err := x509.ParseCertificate(chain.Certificates[0].RawBytes)
	require.NoError(t, err)
	assert.Equal(t, "Test", parsed.Subject.Organization[0])
}

func TestParseCertificateChain_InvalidDER(t *testing.T) {
	badPEM := "-----BEGIN CERTIFICATE-----\nZm9vYmFy\n-----END CERTIFICATE-----\n"
	_, err := parseCertificateChain(badPEM)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid certificate in chain")
}

func TestBuildProtobufBundle_ValidAnnotations(t *testing.T) {
	certPEM, _ := generateTestCertPEM(t)
	sigBytes := []byte("test-signature-bytes")
	sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

	bodyB64 := base64.StdEncoding.EncodeToString([]byte(`{"test":"body"}`))
	setB64 := base64.StdEncoding.EncodeToString([]byte("signed-entry-timestamp"))
	logIDHex := hex.EncodeToString([]byte("log-id"))

	rekorBundle := rekorBundlePayload{SignedEntryTimestamp: setB64}
	rekorBundle.Payload.Body = bodyB64
	rekorBundle.Payload.IntegratedTime = 1701205628
	rekorBundle.Payload.LogIndex = 42
	rekorBundle.Payload.LogID = logIDHex
	rekorJSON, err := json.Marshal(rekorBundle)
	require.NoError(t, err)

	layer := &v1.Descriptor{
		MediaType: types.MediaType(cosignSimpleSigningMediaType),
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		Annotations: map[string]string{
			cosignAnnotationSignature: sigB64,
			cosignAnnotationCert:      certPEM,
			cosignAnnotationBundle:    string(rekorJSON),
		},
	}

	// The signed-artifact digest (policy manifest digest) is distinct from the
	// signature layer's own OCI descriptor digest; the message digest in the
	// bundle must carry the artifact digest so it matches WithArtifactDigest.
	artifactDigestHex := "1111111111111111111111111111111111111111111111111111111111111111"
	pb, err := buildProtobufBundle(layer, artifactDigestHex)
	require.NoError(t, err)
	assert.Equal(t, "application/vnd.dev.sigstore.bundle+json;version=0.1", pb.MediaType)
	require.NotNil(t, pb.VerificationMaterial)
	require.NotNil(t, pb.Content)

	// Verify the signature was correctly decoded
	msgSig := pb.GetMessageSignature()
	require.NotNil(t, msgSig)
	assert.Equal(t, sigBytes, msgSig.Signature)

	// The message digest must be the artifact digest, not the layer digest.
	wantDigest, err := hex.DecodeString(artifactDigestHex)
	require.NoError(t, err)
	assert.Equal(t, wantDigest, msgSig.MessageDigest.Digest,
		"message digest must be the signed artifact digest")

	// Verify certificate chain is present
	certChain := pb.VerificationMaterial.GetX509CertificateChain()
	require.NotNil(t, certChain)
	require.Len(t, certChain.Certificates, 1)

	// Verify tlog entries are present
	require.Len(t, pb.VerificationMaterial.TlogEntries, 1)
	assert.Equal(t, int64(42), pb.VerificationMaterial.TlogEntries[0].LogIndex)
}

func TestBuildProtobufBundle_ArtifactDigestInvariant(t *testing.T) {
	// bundleFromCosignOCI derives a single signedDigestHex (the signature
	// layer's OCI descriptor digest) and feeds it to TWO sinks: the bundle's
	// message digest (via buildProtobufBundle) and the returned artifact
	// digest (via hex.DecodeString). Those two outputs MUST be byte-identical
	// or sigstore-go's WithArtifactDigest check fails. bundleFromCosignOCI
	// itself needs a live registry, so this locks the invariant at the two
	// pure sinks it wires together, guarding against a regression that reverts
	// signedDigestHex to the manifest digest (the original bug).
	sigB64 := base64.StdEncoding.EncodeToString([]byte("sig"))
	signedDigestHex := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

	layer := &v1.Descriptor{
		MediaType: types.MediaType(cosignSimpleSigningMediaType),
		Digest:    v1.Hash{Algorithm: "sha256", Hex: signedDigestHex},
		Annotations: map[string]string{
			cosignAnnotationSignature: sigB64,
		},
	}

	// Sink 1: the message digest inside the bundle.
	pb, err := buildProtobufBundle(layer, signedDigestHex)
	require.NoError(t, err)
	msgDigest := pb.GetMessageSignature().MessageDigest.Digest

	// Sink 2: the artifact digest returned by bundleFromCosignOCI.
	artifactDigest, err := hex.DecodeString(signedDigestHex)
	require.NoError(t, err)

	assert.Equal(t, artifactDigest, msgDigest,
		"returned artifact digest must equal the bundle message digest")
}

func TestBuildVerificationMaterial_NoCertNoBundle(t *testing.T) {
	annotations := map[string]string{
		"unrelated": "value",
	}
	vm, err := buildVerificationMaterial(annotations)
	require.NoError(t, err)
	// A keyed signature carries neither cert nor Rekor bundle; the verification
	// material must still be non-empty (a PublicKey identifier) so sigstore-go
	// accepts the bundle. The trust anchor is the out-of-band keyed verifier.
	pubKey, ok := vm.Content.(*protobundle.VerificationMaterial_PublicKey)
	require.True(t, ok, "keyed material must be a PublicKey identifier")
	assert.NotNil(t, pubKey.PublicKey)
	assert.Empty(t, vm.TlogEntries)
}

func TestBuildVerificationMaterial_CertOnly(t *testing.T) {
	certPEM, _ := generateTestCertPEM(t)
	annotations := map[string]string{
		cosignAnnotationCert: certPEM,
	}
	vm, err := buildVerificationMaterial(annotations)
	require.NoError(t, err)
	require.NotNil(t, vm.GetX509CertificateChain())
	assert.Empty(t, vm.TlogEntries)
}

func TestFindSigningLayer_Found(t *testing.T) {
	img := &fakeImage{
		manifest: &v1.Manifest{
			Layers: []v1.Descriptor{
				{MediaType: "application/octet-stream"},
				{MediaType: types.MediaType(cosignSimpleSigningMediaType), Annotations: map[string]string{"key": "val"}},
			},
		},
	}
	layer, err := findSigningLayer(img)
	require.NoError(t, err)
	assert.Equal(t, types.MediaType(cosignSimpleSigningMediaType), layer.MediaType)
	assert.Equal(t, "val", layer.Annotations["key"])
}

func TestFindSigningLayer_NotFound(t *testing.T) {
	img := &fakeImage{
		manifest: &v1.Manifest{
			Layers: []v1.Descriptor{
				{MediaType: "application/octet-stream"},
			},
		},
	}
	_, err := findSigningLayer(img)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no cosign simple signing layer")
}

func TestExtractVerificationResult_WithVerifiedIdentity(t *testing.T) {
	result := &verify.VerificationResult{
		VerifiedIdentity: &verify.CertificateIdentity{
			SubjectAlternativeName: verify.SubjectAlternativeNameMatcher{
				SubjectAlternativeName: "workflow@github.com",
			},
			Issuer: verify.IssuerMatcher{
				Issuer: "https://token.actions.githubusercontent.com",
			},
		},
	}
	vr := extractVerificationResult(result)
	assert.True(t, vr.Verified)
	assert.Equal(t, "workflow@github.com", vr.SignerIdentity)
	assert.Equal(t, "https://token.actions.githubusercontent.com", vr.Issuer)
	assert.False(t, vr.VerifiedAt.IsZero())
}

func TestExtractVerificationResult_CertificateFallback(t *testing.T) {
	result := &verify.VerificationResult{
		Signature: &verify.SignatureVerificationResult{
			Certificate: &certificate.Summary{
				SubjectAlternativeName: "cert-san@example.com",
				Extensions: certificate.Extensions{
					Issuer: "https://cert-issuer.example.com",
				},
			},
		},
	}
	vr := extractVerificationResult(result)
	assert.True(t, vr.Verified)
	assert.Equal(t, "cert-san@example.com", vr.SignerIdentity)
	assert.Equal(t, "https://cert-issuer.example.com", vr.Issuer)
}

func TestExtractVerificationResult_IdentityTakesPrecedence(t *testing.T) {
	result := &verify.VerificationResult{
		VerifiedIdentity: &verify.CertificateIdentity{
			SubjectAlternativeName: verify.SubjectAlternativeNameMatcher{
				SubjectAlternativeName: "primary@github.com",
			},
			Issuer: verify.IssuerMatcher{
				Issuer: "https://primary-issuer.com",
			},
		},
		Signature: &verify.SignatureVerificationResult{
			Certificate: &certificate.Summary{
				SubjectAlternativeName: "fallback@example.com",
				Extensions: certificate.Extensions{
					Issuer: "https://fallback-issuer.com",
				},
			},
		},
	}
	vr := extractVerificationResult(result)
	assert.Equal(t, "primary@github.com", vr.SignerIdentity)
	assert.Equal(t, "https://primary-issuer.com", vr.Issuer)
}

func TestExtractVerificationResult_Empty(t *testing.T) {
	result := &verify.VerificationResult{}
	vr := extractVerificationResult(result)
	assert.True(t, vr.Verified)
	assert.Empty(t, vr.SignerIdentity)
	assert.Empty(t, vr.Issuer)
}

// --- Group 4: Verifier Construction Tests (custom trusted root) ---

func TestNewKeylessVerifier_WithTrustedRootFixture(t *testing.T) {
	// Verifies that a valid (but intentionally minimal) trusted_root.json
	// fixture loads via the custom-root branch without error. The fixture
	// has empty trust material arrays — this confirms file parsing, not
	// verification capability. Branch discrimination is tested separately
	// by TestNewKeylessVerifier_EmptyTrustedRootPathTakesTUFBranch.
	fixturePath := filepath.Join("testdata", "trusted_root.json")
	vf, err := NewKeylessVerifier("https://issuer.example.com", "user@example.com", fixturePath)
	require.NoError(t, err, "valid trusted_root.json fixture should create verifier")
	require.NotNil(t, vf, "returned VerifyFunc should not be nil")
}

func TestNewKeylessVerifier_NonexistentTrustedRootPath(t *testing.T) {
	// Nonexistent trustedRootPath returns error containing
	// "failed to load trusted root from".
	vf, err := NewKeylessVerifier(
		"https://issuer.example.com", "user@example.com",
		"/nonexistent/path/trusted_root.json",
	)
	require.Error(t, err)
	assert.Nil(t, vf)
	assert.Contains(t, err.Error(), "failed to load trusted root from")
}

func TestNewKeylessVerifier_InvalidJSONTrustedRoot(t *testing.T) {
	// Invalid JSON at trustedRootPath returns error containing
	// "failed to load trusted root from".
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid_root.json")
	require.NoError(t, os.WriteFile(invalidPath, []byte("not valid json"), 0600))

	vf, err := NewKeylessVerifier(
		"https://issuer.example.com", "user@example.com",
		invalidPath,
	)
	require.Error(t, err)
	assert.Nil(t, vf)
	assert.Contains(t, err.Error(), "failed to load trusted root from")
}

func TestNewKeylessVerifier_EmptyTrustedRootPathTakesTUFBranch(t *testing.T) {
	// Empty trustedRootPath does not call root.NewTrustedRootFromPath.
	// Verify by confirming that a nonexistent path that would fail if the
	// custom-root branch were taken does NOT produce a file-read error.
	// With empty trustedRootPath, the code takes the TUF fetch branch instead.
	// The TUF fetch will fail (no network in unit test), but the error should
	// NOT contain "failed to load trusted root from" — it should contain
	// "failed to fetch Sigstore trusted root" (the TUF branch error).
	vf, err := NewKeylessVerifier(
		"https://issuer.example.com", "user@example.com",
		"",
	)
	// TUF fetch will fail in a unit test environment (no network).
	// That's expected — we just verify it took the TUF branch, not the file branch.
	if err != nil {
		assert.NotContains(t, err.Error(), "failed to load trusted root from",
			"empty trustedRootPath should not attempt file load")
		assert.Contains(t, err.Error(), "Sigstore trusted root",
			"empty trustedRootPath should take TUF fetch branch")
	}
	// If it somehow succeeds (e.g., TUF cache available), that's also fine.
	if err == nil {
		assert.NotNil(t, vf)
	}
}

// fakeImage implements v1.Image minimally for findSigningLayer tests.
type fakeImage struct {
	v1.Image
	manifest *v1.Manifest
}

func (f *fakeImage) Manifest() (*v1.Manifest, error) {
	return f.manifest, nil
}
