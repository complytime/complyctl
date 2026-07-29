// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	sigstorecrypto "github.com/sigstore/sigstore/pkg/cryptoutils"
	sigsig "github.com/sigstore/sigstore/pkg/signature"
)

// VerifyFunc verifies an OCI artifact identified by a registry reference.
// Returns a VerificationResult on success or an error on failure.
// A nil VerifyFunc means verification is disabled.
type VerifyFunc func(ctx context.Context, registryRef string) (*VerificationResult, error)

// VerificationResult captures sigstore-go verification output.
type VerificationResult struct {
	Verified       bool
	SignerIdentity string
	Issuer         string
	VerifiedAt     time.Time
}

// cosignAnnotationSignature is the annotation key for the base64-encoded signature.
const cosignAnnotationSignature = "dev.cosignproject.cosign/signature"

// cosignAnnotationCert is the annotation key for the PEM-encoded signing certificate.
const cosignAnnotationCert = "dev.sigstore.cosign/certificate"

// cosignAnnotationBundle is the annotation key for the Rekor tlog entry JSON.
const cosignAnnotationBundle = "dev.sigstore.cosign/bundle"

// cosignSimpleSigningMediaType is the media type for cosign simple signing layers.
const cosignSimpleSigningMediaType = "application/vnd.dev.cosign.simplesigning.v1+json"

// rekorBundlePayload represents the JSON structure in the cosign bundle annotation.
type rekorBundlePayload struct {
	SignedEntryTimestamp string `json:"SignedEntryTimestamp"`
	Payload              struct {
		Body           string `json:"body"`
		IntegratedTime int64  `json:"integratedTime"`
		LogIndex       int64  `json:"logIndex"`
		LogID          string `json:"logID"`
	} `json:"Payload"`
}

// tufFetchTimeout is the maximum time allowed for fetching the Sigstore
// trusted root via TUF. This prevents complyctl get from hanging in
// restricted network environments.
const tufFetchTimeout = 30 * time.Second

// NewKeylessVerifier creates a VerifyFunc that verifies cosign signatures using
// Sigstore keyless verification (Fulcio + Rekor). The issuer and identity
// parameters match the OIDC issuer and SAN identity in the signing certificate.
// When trustedRootPath is non-empty, the trusted root is loaded from the
// specified file instead of fetching it via TUF from the public Sigstore
// infrastructure. This supports private Sigstore deployments.
func NewKeylessVerifier(issuer, identity, trustedRootPath string) (VerifyFunc, error) {
	var trustedRoot *root.TrustedRoot
	var err error

	if trustedRootPath != "" {
		trustedRoot, err = root.NewTrustedRootFromPath(trustedRootPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load trusted root from %s: %w", trustedRootPath, err)
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), tufFetchTimeout)
		defer cancel()

		fmt.Fprintln(os.Stderr, "Fetching Sigstore trusted root...")
		opts := tuf.DefaultOptions().WithContext(ctx)
		trustedRoot, err = root.FetchTrustedRootWithOptions(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Sigstore trusted root: %w", err)
		}
	}

	certID, err := verify.NewShortCertificateIdentity(issuer, "", identity, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate identity: %w", err)
	}

	verifier, err := verify.NewVerifier(trustedRoot,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sigstore verifier: %w", err)
	}

	return func(ctx context.Context, registryRef string) (*VerificationResult, error) {
		bun, artifactDigest, err := bundleFromCosignOCI(ctx, registryRef)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve cosign signature for %s: %w", registryRef, err)
		}

		policy := verify.NewPolicy(
			verify.WithArtifactDigest("sha256", artifactDigest),
			verify.WithCertificateIdentity(certID),
		)

		result, err := verifier.Verify(bun, policy)
		if err != nil {
			return nil, fmt.Errorf("signature verification failed for %s: %w", registryRef, err)
		}

		return extractVerificationResult(result), nil
	}, nil
}

// NewKeyedVerifier creates a VerifyFunc that verifies cosign signatures using
// a PEM-encoded public key.
func NewKeyedVerifier(keyPath string) (VerifyFunc, error) {
	keyPath = filepath.Clean(keyPath)
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key %s: %w", keyPath, err)
	}

	pubKey, err := sigstorecrypto.UnmarshalPEMToPublicKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key %s: %w", keyPath, err)
	}

	sigVerifier, err := sigsig.LoadVerifier(pubKey, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("failed to create signature verifier from %s: %w", keyPath, err)
	}

	// For keyed verification, accept any key ID since we have a single
	// trusted public key. The ExpiringKey with zero times means the key
	// is valid for all time.
	key := root.NewExpiringKey(sigVerifier, time.Time{}, time.Time{})
	trustedKeyMaterial := root.NewTrustedPublicKeyMaterial(
		func(_ string) (root.TimeConstrainedVerifier, error) {
			return key, nil
		},
	)

	verifier, err := verify.NewVerifier(trustedKeyMaterial,
		verify.WithNoObserverTimestamps(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create keyed verifier: %w", err)
	}

	return func(ctx context.Context, registryRef string) (*VerificationResult, error) {
		bun, artifactDigest, err := bundleFromCosignOCI(ctx, registryRef)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve cosign signature for %s: %w", registryRef, err)
		}

		policy := verify.NewPolicy(
			verify.WithArtifactDigest("sha256", artifactDigest),
			verify.WithKey(),
		)

		_, err = verifier.Verify(bun, policy)
		if err != nil {
			return nil, fmt.Errorf("signature verification failed for %s: %w", registryRef, err)
		}

		return &VerificationResult{
			Verified:   true,
			VerifiedAt: time.Now(),
		}, nil
	}, nil
}

// bundleFromCosignOCI resolves a cosign signature from the OCI registry for the
// given image reference, constructs a sigstore-go bundle, and returns it along
// with the artifact digest bytes.
func bundleFromCosignOCI(ctx context.Context, registryRef string) (*bundle.Bundle, []byte, error) {
	ref, digestHex, err := resolveArtifactDigest(ctx, registryRef)
	if err != nil {
		return nil, nil, err
	}

	sigLayer, err := fetchCosignSignature(ctx, ref, digestHex, registryRef)
	if err != nil {
		return nil, nil, err
	}

	pb, err := buildProtobufBundle(sigLayer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build sigstore bundle from cosign annotations: %w", err)
	}

	bun, err := bundle.NewBundle(pb)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create sigstore bundle: %w", err)
	}

	artifactDigest, err := hex.DecodeString(digestHex)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode artifact digest: %w", err)
	}

	return bun, artifactDigest, nil
}

// resolveArtifactDigest parses the OCI reference and resolves the image
// manifest digest from the registry.
func resolveArtifactDigest(ctx context.Context, registryRef string) (name.Reference, string, error) {
	ref, err := name.ParseReference(registryRef)
	if err != nil {
		return nil, "", fmt.Errorf("invalid OCI reference %q: %w", registryRef, err)
	}

	desc, err := remote.Get(ref,
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithContext(ctx),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve %s: %w", registryRef, err)
	}

	return ref, desc.Digest.Hex, nil
}

// fetchCosignSignature resolves the cosign signature manifest from the
// registry using the tag convention (sha256-<hex>.sig) and returns the
// simple signing layer descriptor containing verification annotations.
func fetchCosignSignature(ctx context.Context, ref name.Reference, digestHex, registryRef string) (*v1.Descriptor, error) {
	sigTagStr := fmt.Sprintf("%s:sha256-%s.sig", ref.Context().Name(), digestHex)
	sigTag, err := name.NewTag(sigTagStr)
	if err != nil {
		return nil, fmt.Errorf("failed to construct cosign signature tag: %w", err)
	}

	sigDesc, err := remote.Get(sigTag,
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("no cosign signature found for %s: %w", registryRef, err)
	}

	sigImage, err := sigDesc.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to read cosign signature image: %w", err)
	}

	return findSigningLayer(sigImage)
}

// findSigningLayer locates the cosign simple signing layer in a signature
// image manifest. Returns an error if no matching layer is found.
func findSigningLayer(sigImage v1.Image) (*v1.Descriptor, error) {
	manifest, err := sigImage.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to read cosign signature manifest: %w", err)
	}

	for i := range manifest.Layers {
		layer := &manifest.Layers[i]
		if layer.MediaType == cosignSimpleSigningMediaType {
			return layer, nil
		}
	}
	return nil, fmt.Errorf("no cosign simple signing layer found in signature manifest")
}

// buildProtobufBundle constructs a sigstore protobuf bundle from cosign
// layer descriptor annotations.
func buildProtobufBundle(layer *v1.Descriptor) (*protobundle.Bundle, error) {
	annotations := layer.Annotations
	if annotations == nil {
		return nil, fmt.Errorf("cosign layer has no annotations")
	}

	pb := &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle+json;version=0.1",
	}

	// Extract verification material (certificate chain + tlog entries)
	verificationMaterial, err := buildVerificationMaterial(annotations)
	if err != nil {
		return nil, err
	}
	pb.VerificationMaterial = verificationMaterial

	// Extract message signature
	sigB64, ok := annotations[cosignAnnotationSignature]
	if !ok {
		return nil, fmt.Errorf("cosign layer missing signature annotation")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cosign signature: %w", err)
	}

	digestBytes, err := hex.DecodeString(layer.Digest.Hex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode layer digest: %w", err)
	}

	pb.Content = &protobundle.Bundle_MessageSignature{
		MessageSignature: &protocommon.MessageSignature{
			MessageDigest: &protocommon.HashOutput{
				Algorithm: protocommon.HashAlgorithm_SHA2_256,
				Digest:    digestBytes,
			},
			Signature: sigBytes,
		},
	}

	return pb, nil
}

// buildVerificationMaterial extracts the X.509 certificate chain and Rekor tlog
// entries from cosign annotations.
func buildVerificationMaterial(annotations map[string]string) (*protobundle.VerificationMaterial, error) {
	vm := &protobundle.VerificationMaterial{}

	// Extract certificate chain
	certPEM, hasCert := annotations[cosignAnnotationCert]
	if hasCert {
		certs, err := parseCertificateChain(certPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cosign certificate: %w", err)
		}
		vm.Content = &protobundle.VerificationMaterial_X509CertificateChain{
			X509CertificateChain: certs,
		}
	}

	// Extract Rekor tlog entry
	bundleJSON, hasBundle := annotations[cosignAnnotationBundle]
	if hasBundle {
		entries, err := parseRekorBundle(bundleJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cosign Rekor bundle: %w", err)
		}
		vm.TlogEntries = entries
	}

	return vm, nil
}

// parseCertificateChain parses a PEM-encoded certificate (possibly with chain)
// into the protobuf X509CertificateChain format.
func parseCertificateChain(certPEM string) (*protocommon.X509CertificateChain, error) {
	var certs []*protocommon.X509Certificate
	remaining := []byte(certPEM)
	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			// Validate the certificate parses correctly
			if _, err := x509.ParseCertificate(block.Bytes); err != nil {
				return nil, fmt.Errorf("invalid certificate in chain: %w", err)
			}
			certs = append(certs, &protocommon.X509Certificate{
				RawBytes: block.Bytes,
			})
		}
		remaining = rest
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates found in PEM data")
	}
	return &protocommon.X509CertificateChain{Certificates: certs}, nil
}

// parseRekorBundle parses the cosign Rekor bundle annotation JSON into
// protobuf TransparencyLogEntry format.
func parseRekorBundle(bundleJSON string) ([]*protorekor.TransparencyLogEntry, error) {
	var payload rekorBundlePayload
	if err := json.Unmarshal([]byte(bundleJSON), &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Rekor bundle: %w", err)
	}

	setBytes, err := base64.StdEncoding.DecodeString(payload.SignedEntryTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode SignedEntryTimestamp: %w", err)
	}

	logIDBytes, err := base64.StdEncoding.DecodeString(payload.Payload.LogID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode logID: %w", err)
	}

	canonicalBody, err := base64.StdEncoding.DecodeString(payload.Payload.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode body: %w", err)
	}

	entry := &protorekor.TransparencyLogEntry{
		LogIndex: payload.Payload.LogIndex,
		LogId: &protocommon.LogId{
			KeyId: logIDBytes,
		},
		KindVersion: &protorekor.KindVersion{
			Kind:    "hashedrekord",
			Version: "0.0.1",
		},
		IntegratedTime: payload.Payload.IntegratedTime,
		InclusionPromise: &protorekor.InclusionPromise{
			SignedEntryTimestamp: setBytes,
		},
		CanonicalizedBody: canonicalBody,
	}

	return []*protorekor.TransparencyLogEntry{entry}, nil
}

// extractVerificationResult maps sigstore-go's VerificationResult to our
// VerificationResult type.
func extractVerificationResult(result *verify.VerificationResult) *VerificationResult {
	vr := &VerificationResult{
		Verified:   true,
		VerifiedAt: time.Now(),
	}

	if result.VerifiedIdentity != nil {
		vr.Issuer = result.VerifiedIdentity.Issuer.Issuer
		vr.SignerIdentity = result.VerifiedIdentity.SubjectAlternativeName.SubjectAlternativeName
	}

	if result.Signature != nil && result.Signature.Certificate != nil {
		cert := result.Signature.Certificate
		if vr.Issuer == "" {
			vr.Issuer = cert.Issuer
		}
		if vr.SignerIdentity == "" {
			vr.SignerIdentity = cert.SubjectAlternativeName
		}
	}

	return vr
}
