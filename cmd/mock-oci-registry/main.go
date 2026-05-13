// SPDX-License-Identifier: Apache-2.0

// Gemara Content Service — OCI-compliant registry and compliance enrichment server for testing.
// Implements the Gemara Content Service API (OpenAPI 3.0.3):
//   - OCI Distribution Spec v2 endpoints (/v2/, _catalog, tags/list, manifests, blobs)
//   - Compliance enrichment endpoint (/v1/enrich)
//
// Start: go run ./cmd/mock-oci-registry
// Usage: COMPLYCTL_REGISTRY_URL=http://localhost:8765 complytime get

package main

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

//go:embed testdata/*.yaml
var seedData embed.FS

const defaultPort = "8765"

// OCI media types
const (
	ociManifestMediaType = "application/vnd.oci.image.manifest.v1+json"
	ociEmptyConfigType   = "application/vnd.oci.empty.v1+json"
)

// Gemara layer media types
const (
	gemaraGuidanceType = "application/vnd.gemara.guidance.v1+yaml"
	gemaraCatalogType  = "application/vnd.gemara.catalog.v1+yaml"
	gemaraPolicyType   = "application/vnd.gemara.policy.v1+yaml"
)

func main() {
	port := os.Getenv("GEMARA_SERVICE_PORT")
	if port == "" {
		port = defaultPort
	}

	store := newContentStore()
	store.seedDefaults()

	mux := http.NewServeMux()
	registerOCIRoutes(mux, store)
	registerEnrichRoute(mux, store)

	addr := ":" + port
	log.Printf("Gemara Content Service listening on http://localhost%s", addr) //nolint:gosec // G706: addr is from a hardcoded port, not user input
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// registerOCIRoutes adds OCI Distribution Spec v2 endpoints.
func registerOCIRoutes(mux *http.ServeMux, store *contentStore) {
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/v2/")
		path = strings.TrimSuffix(path, "/")

		// GET /v2/ — API version check
		if path == "" {
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
			return
		}

		// GET /v2/_catalog
		if path == "_catalog" {
			serveCatalog(w, store)
			return
		}

		// Route: /v2/{name}/tags/list
		if idx := strings.LastIndex(path, "/tags/list"); idx > 0 {
			repoName := path[:idx]
			serveTagsList(w, store, repoName)
			return
		}

		// Route: /v2/{name}/manifests/{reference}
		if idx := strings.LastIndex(path, "/manifests/"); idx > 0 {
			repoName := path[:idx]
			reference := path[idx+len("/manifests/"):]
			serveManifest(w, r, store, repoName, reference)
			return
		}

		// Route: /v2/{name}/blobs/{digest}
		if idx := strings.LastIndex(path, "/blobs/"); idx > 0 {
			repoName := path[:idx]
			digest := path[idx+len("/blobs/"):]
			serveBlob(w, store, repoName, digest)
			return
		}

		writeOCIError(w, http.StatusBadRequest, "NAME_UNKNOWN", "invalid API path")
	})
}

// serveCatalog handles GET /v2/_catalog
func serveCatalog(w http.ResponseWriter, store *contentStore) {
	repos := store.listRepositories()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"repositories": repos,
	})
}

// serveTagsList handles GET /v2/{name}/tags/list
func serveTagsList(w http.ResponseWriter, store *contentStore, repoName string) {
	repo, ok := store.repos[repoName]
	if !ok {
		writeOCIError(w, http.StatusNotFound, "NAME_UNKNOWN", fmt.Sprintf("repository %q not found", repoName))
		return
	}

	tags := make([]string, 0, len(repo.tags))
	for tag := range repo.tags {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"name": repoName,
		"tags": tags,
	})
}

// serveManifest handles GET/HEAD /v2/{name}/manifests/{reference}
func serveManifest(w http.ResponseWriter, r *http.Request, store *contentStore, repoName, reference string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repo, ok := store.repos[repoName]
	if !ok {
		writeOCIError(w, http.StatusNotFound, "NAME_UNKNOWN", fmt.Sprintf("repository %q not found", repoName))
		return
	}

	art, ok := repo.resolve(reference)
	if !ok {
		writeOCIError(w, http.StatusNotFound, "MANIFEST_UNKNOWN", fmt.Sprintf("manifest %q not found", reference))
		return
	}

	w.Header().Set("Content-Type", ociManifestMediaType)
	w.Header().Set("Docker-Content-Digest", art.manifestDigest)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(art.manifestBytes)))
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")

	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(art.manifestBytes) //nolint:gosec // G705: internal test mock data, not user-tainted
}

// serveBlob handles GET /v2/{name}/blobs/{digest}
func serveBlob(w http.ResponseWriter, store *contentStore, repoName, digest string) {
	repo, ok := store.repos[repoName]
	if !ok {
		writeOCIError(w, http.StatusNotFound, "NAME_UNKNOWN", fmt.Sprintf("repository %q not found", repoName))
		return
	}

	blob, ok := repo.blobs[digest]
	if !ok {
		writeOCIError(w, http.StatusNotFound, "BLOB_UNKNOWN", fmt.Sprintf("blob %q not found", digest))
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(blob.data)))
	w.Header().Set("Docker-Content-Digest", digest)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(blob.data)
}

// registerEnrichRoute adds the POST /v1/enrich endpoint.
func registerEnrichRoute(mux *http.ServeMux, store *contentStore) {
	mux.HandleFunc("/v1/enrich", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req enrichmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Policy.PolicyEngineName == "" || req.Policy.PolicyRuleID == "" {
			writeJSONError(w, http.StatusBadRequest, "policy.policyEngineName and policy.policyRuleId are required")
			return
		}

		resp := store.enrich(req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// writeOCIError writes an OCI Distribution Spec error response.
func writeOCIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"errors": []map[string]string{
			{"code": code, "message": message},
		},
	})
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    status,
		"message": message,
	})
}

// --- Content Store ---

type contentStore struct {
	repos       map[string]*repository
	enrichments map[string]*enrichmentMapping
}

type repository struct {
	tags  map[string]*artifact
	blobs map[string]*blob
}

type artifact struct {
	manifestBytes  []byte
	manifestDigest string
}

type blob struct {
	data      []byte
	mediaType string
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type ociManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Config        ociDescriptor   `json:"config"`
	Layers        []ociDescriptor `json:"layers"`
}

func (r *repository) resolve(reference string) (*artifact, bool) {
	art, ok := r.tags[reference]
	if ok {
		return art, true
	}
	// Try resolving by digest across all tags
	for _, a := range r.tags {
		if a.manifestDigest == reference {
			return a, true
		}
	}
	return nil, false
}

func newContentStore() *contentStore {
	return &contentStore{
		repos:       make(map[string]*repository),
		enrichments: make(map[string]*enrichmentMapping),
	}
}

func (s *contentStore) listRepositories() []string {
	repos := make([]string, 0, len(s.repos))
	for name := range s.repos {
		repos = append(repos, name)
	}
	sort.Strings(repos)
	return repos
}

// addArtifact creates a repository with OCI manifest, layers, and tags.
func (s *contentStore) addArtifact(repoName string, tags []string, layers []layerDef) {
	repo := &repository{
		tags:  make(map[string]*artifact),
		blobs: make(map[string]*blob),
	}

	// Empty complytime blob (OCI spec)
	emptyConfig := []byte("{}")
	emptyConfigDigest := computeDigest(emptyConfig)
	repo.blobs[emptyConfigDigest] = &blob{data: emptyConfig, mediaType: ociEmptyConfigType}

	// Store each layer as a blob
	layerDescs := make([]ociDescriptor, 0, len(layers))
	for _, l := range layers {
		digest := computeDigest(l.data)
		repo.blobs[digest] = &blob{data: l.data, mediaType: l.mediaType}
		layerDescs = append(layerDescs, ociDescriptor{
			MediaType: l.mediaType,
			Digest:    digest,
			Size:      int64(len(l.data)),
		})
	}

	manifest := ociManifest{
		SchemaVersion: 2,
		MediaType:     ociManifestMediaType,
		Config: ociDescriptor{
			MediaType: ociEmptyConfigType,
			Digest:    emptyConfigDigest,
			Size:      int64(len(emptyConfig)),
		},
		Layers: layerDescs,
	}

	manifestBytes, _ := json.Marshal(manifest)
	manifestDigest := computeDigest(manifestBytes)

	art := &artifact{
		manifestBytes:  manifestBytes,
		manifestDigest: manifestDigest,
	}

	for _, tag := range tags {
		repo.tags[tag] = art
	}

	s.repos[repoName] = repo
}

type layerDef struct {
	mediaType string
	data      []byte
}

func computeDigest(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash)
}

// --- Enrichment ---

type enrichmentRequest struct {
	Policy enrichmentPolicy `json:"policy"`
}

type enrichmentPolicy struct {
	PolicyEngineName string `json:"policyEngineName"`
	PolicyRuleID     string `json:"policyRuleId"`
}

type enrichmentResponse struct {
	Compliance enrichmentCompliance `json:"compliance"`
}

type enrichmentCompliance struct {
	Control          enrichmentControl    `json:"control"`
	Frameworks       enrichmentFrameworks `json:"frameworks"`
	Risk             enrichmentRisk       `json:"risk"`
	EnrichmentStatus string               `json:"enrichmentStatus"`
}

type enrichmentControl struct {
	ID                     string   `json:"id"`
	Category               string   `json:"category"`
	CatalogID              string   `json:"catalogId"`
	Applicability          []string `json:"applicability,omitempty"`
	RemediationDescription string   `json:"remediationDescription,omitempty"`
}

type enrichmentFrameworks struct {
	Frameworks   []string `json:"frameworks"`
	Requirements []string `json:"requirements"`
}

type enrichmentRisk struct {
	Level string `json:"level"`
}

type enrichmentMapping struct {
	control    enrichmentControl
	frameworks enrichmentFrameworks
	risk       enrichmentRisk
}

func (s *contentStore) enrich(req enrichmentRequest) enrichmentResponse {
	key := req.Policy.PolicyEngineName + ":" + req.Policy.PolicyRuleID
	mapping, ok := s.enrichments[key]
	if !ok {
		return enrichmentResponse{
			Compliance: enrichmentCompliance{
				Control: enrichmentControl{
					ID:        req.Policy.PolicyRuleID,
					Category:  "Unknown",
					CatalogID: "Unknown",
				},
				Frameworks: enrichmentFrameworks{
					Frameworks:   []string{},
					Requirements: []string{},
				},
				Risk:             enrichmentRisk{Level: "Informational"},
				EnrichmentStatus: "Unmapped",
			},
		}
	}
	return enrichmentResponse{
		Compliance: enrichmentCompliance{
			Control:          mapping.control,
			Frameworks:       mapping.frameworks,
			Risk:             mapping.risk,
			EnrichmentStatus: "Success",
		},
	}
}

// --- Seed Data ---

// seedPolicyFromFiles loads a catalog and policy YAML from embedded testdata
// files and registers them as a versioned OCI artifact in the content store.
func (s *contentStore) seedPolicyFromFiles(
	repoPath, catalogFile, policyFile string, tags []string,
) {
	catalog, err := seedData.ReadFile(catalogFile)
	if err != nil {
		log.Fatalf("failed to load catalog seed data from %s: %v", catalogFile, err)
	}
	policy, err := seedData.ReadFile(policyFile)
	if err != nil {
		log.Fatalf("failed to load policy seed data from %s: %v", policyFile, err)
	}
	s.addArtifact(repoPath, tags, []layerDef{
		{mediaType: gemaraCatalogType, data: catalog},
		{mediaType: gemaraPolicyType, data: policy},
	})
}

func (s *contentStore) seedDefaults() {
	// policies/nist-800-53-r5
	s.addArtifact("policies/nist-800-53-r5", []string{"v1.0.0", "latest"}, []layerDef{
		{mediaType: gemaraCatalogType, data: []byte(`title: NIST SP 800-53 Rev 5
metadata:
  id: nist-800-53-r5
  description: Security and privacy controls for information systems
  author:
    id: nist
    name: NIST
    type: Human
families:
  - id: access-control
    title: Access Control
    description: Controls related to access management
controls:
  - id: AC-1
    title: Access Control Policy
    objective: Establish and maintain access control policy
    family: access-control
    assessment-requirements:
      - id: AC-1-ar
        text: Access control policy MUST be documented and maintained
        applicability:
          - All systems
  - id: AC-2
    title: Account Management
    objective: Manage information system accounts
    family: access-control
    assessment-requirements:
      - id: AC-2-ar
        text: System accounts MUST be properly managed
        applicability:
          - All systems
`)},
		{mediaType: gemaraPolicyType, data: []byte(`title: NIST SP 800-53 Rev 5 Policy
metadata:
  id: nist-800-53-r5-policy
  description: Automated evaluation policy for NIST SP 800-53 Rev 5
  author:
    id: complytime
    name: ComplyTime
    type: Software
  mapping-references:
    - id: nist-800-53-r5
      title: NIST SP 800-53 Rev 5
      version: "5.0"
contacts:
  responsible:
    - name: System Administrator
  accountable:
    - name: Security Team
scope:
  in:
    technologies:
      - Information Systems
imports:
  catalogs:
    - reference-id: nist-800-53-r5
adherence:
  evaluation-methods:
    - type: Behavioral
      executor:
        id: test
        name: Test Evaluator
        type: Software
  assessment-plans:
    - id: AC-1-impl
      requirement-id: AC-1-ar
      frequency: on-demand
      evaluation-methods:
        - type: Behavioral
    - id: AC-2-impl
      requirement-id: AC-2-ar
      frequency: on-demand
      evaluation-methods:
        - type: Behavioral
`)},
	})

	// policies/cis-benchmark
	s.addArtifact("policies/cis-benchmark", []string{"v2.0.0", "latest"}, []layerDef{
		{mediaType: gemaraCatalogType, data: []byte(`title: CIS Benchmark
metadata:
  id: cis-benchmark
  description: Center for Internet Security Benchmark controls
  author:
    id: cis
    name: CIS
    type: Human
families:
  - id: filesystem
    title: Filesystem Configuration
    description: Controls for filesystem hardening
controls:
  - id: CIS-1.1
    title: Filesystem Configuration
    objective: Harden filesystem configuration
    family: filesystem
    assessment-requirements:
      - id: CIS-1.1-ar
        text: Filesystem MUST be properly configured
        applicability:
          - All systems
`)},
	})

	// catalogs/osps-b
	s.addArtifact("catalogs/osps-b", []string{"v1.0.0", "latest"}, []layerDef{
		{mediaType: gemaraCatalogType, data: []byte(`title: Open Source Project Security Baseline
metadata:
  id: osps-b
  description: Security baseline controls for open source projects
  author:
    id: openssf
    name: OpenSSF
    type: Human
families:
  - id: quality-assurance
    title: Quality Assurance
    description: Controls ensuring software quality and security
controls:
  - id: OSPS-QA-07.01
    title: Quality Assurance Control
    objective: Ensure quality assurance processes are in place
    family: quality-assurance
    assessment-requirements:
      - id: OSPS-QA-07.01-ar
        text: Quality assurance controls MUST be implemented
        applicability:
          - Open source projects
`)},
	})

	// guidance/nist
	s.addArtifact("guidance/nist", []string{"v1.0.0", "latest"}, []layerDef{
		{mediaType: gemaraGuidanceType, data: []byte(`title: NIST Security Guidance
metadata:
  id: nist-guidance
  description: NIST security guidance for information systems
  author:
    id: nist
    name: NIST
    type: Human
type: Standard
families:
  - id: access-control
    title: Access Control
    description: Guidelines related to access management
guidelines:
  - id: nist-guide-ac
    title: Access Control Guidance
    objective: Provide guidance on access control implementation
    family: access-control
`)},
	})

	// File-based policies — catalog + policy YAML loaded from testdata/
	s.seedPolicyFromFiles("policies/cis-fedora-l1-workstation",
		"testdata/cis-fedora-l1-workstation-catalog.yaml",
		"testdata/cis-fedora-l1-workstation-policy.yaml",
		[]string{"v1.0.0", "latest"})
	s.seedPolicyFromFiles("policies/ampel-branch-protection",
		"testdata/ampel-branch-protection-catalog.yaml",
		"testdata/ampel-branch-protection-policy.yaml",
		[]string{"v1.0.0", "latest"})
	s.seedPolicyFromFiles("policies/test-branch-protection",
		"testdata/test-branch-protection-catalog.yaml",
		"testdata/test-branch-protection-policy.yaml",
		[]string{"v1.0.0", "latest"})

	// Enrichment mappings
	s.enrichments["OPA:deny-root-user"] = &enrichmentMapping{
		control: enrichmentControl{
			ID:                     "OSPS-QA-07.01",
			Category:               "Access Control",
			CatalogID:              "OSPS-B",
			Applicability:          []string{"Production", "Staging"},
			RemediationDescription: "Remove root user access and implement proper IAM policies",
		},
		frameworks: enrichmentFrameworks{
			Frameworks:   []string{"NIST-800-53", "SOC-2"},
			Requirements: []string{"AC-1", "CC6.1"},
		},
		risk: enrichmentRisk{Level: "High"},
	}

	s.enrichments["Kyverno:require-labels"] = &enrichmentMapping{
		control: enrichmentControl{
			ID:                     "OSPS-QA-07.01",
			Category:               "Configuration Management",
			CatalogID:              "OSPS-B",
			Applicability:          []string{"Production"},
			RemediationDescription: "Ensure all resources have required labels",
		},
		frameworks: enrichmentFrameworks{
			Frameworks:   []string{"NIST-800-53"},
			Requirements: []string{"CM-2"},
		},
		risk: enrichmentRisk{Level: "Medium"},
	}
}
