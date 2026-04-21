// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPolicyYAML is a valid Gemara Policy with two assessment-plans bound to
// the "test" evaluator.  Used by all mock registry seeds.
var testPolicyYAML = []byte(`title: Test Policy
metadata:
  id: test-policy
  version: "1.0.0"
contacts:
  responsible:
    - id: test-team
      name: Test Team
  accountable:
    - id: test-lead
      name: Test Lead
scope:
  in:
    technologies:
      - linux
imports:
  catalogs:
    - reference-id: nist-800-53-r5
adherence:
  evaluation-methods:
    - type: Behavioral
      executor:
        id: test
        name: test-evaluator
        type: Software
  assessment-plans:
    - id: AC-1-impl
      requirement-id: AC-1
      frequency: continuous
      evaluation-methods:
        - type: Behavioral
          executor:
            id: test
            name: test-evaluator
            type: Software
    - id: AC-2-impl
      requirement-id: AC-2
      frequency: continuous
      evaluation-methods:
        - type: Behavioral
          executor:
            id: test
            name: test-evaluator
            type: Software
`)

// --- Mock OCI Registry ---

type ociBlob struct {
	data      []byte
	mediaType string
}

type ociArtifact struct {
	manifestBytes  []byte
	manifestDigest string
}

type ociRepo struct {
	tags  map[string]*ociArtifact
	blobs map[string]*ociBlob
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

func ociDigest(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash)
}

// startMockRegistry creates an in-process mock OCI registry implementing
// the OCI Distribution Spec v2 endpoints. Serves proper manifests with
// computed SHA256 digests so oras-go works directly.
//
// Seeded policies:
//   - nist-800-53-r5 (catalog + policy layers, evaluator_id: test)
//   - cis-benchmark  (catalog layer only)
func startMockRegistry(t *testing.T) *httptest.Server {
	t.Helper()

	repos := make(map[string]*ociRepo)

	addArtifact := func(repoName string, tags []string, layers []struct {
		mt   string
		data []byte
	}) {
		repo := &ociRepo{
			tags:  make(map[string]*ociArtifact),
			blobs: make(map[string]*ociBlob),
		}

		emptyConfig := []byte("{}")
		emptyDigest := ociDigest(emptyConfig)
		repo.blobs[emptyDigest] = &ociBlob{
			data:      emptyConfig,
			mediaType: "application/vnd.oci.empty.v1+json",
		}

		layerDescs := make([]ociDescriptor, 0, len(layers))
		for _, l := range layers {
			d := ociDigest(l.data)
			repo.blobs[d] = &ociBlob{data: l.data, mediaType: l.mt}
			layerDescs = append(layerDescs, ociDescriptor{
				MediaType: l.mt,
				Digest:    d,
				Size:      int64(len(l.data)),
			})
		}

		manifest := map[string]interface{}{
			"schemaVersion": 2,
			"mediaType":     "application/vnd.oci.image.manifest.v1+json",
			"config": ociDescriptor{
				MediaType: "application/vnd.oci.empty.v1+json",
				Digest:    emptyDigest,
				Size:      int64(len(emptyConfig)),
			},
			"layers": layerDescs,
		}
		manifestData, _ := json.Marshal(manifest)
		art := &ociArtifact{
			manifestBytes:  manifestData,
			manifestDigest: ociDigest(manifestData),
		}

		for _, tag := range tags {
			repo.tags[tag] = art
		}
		repos[repoName] = repo
	}

	// Seed: nist-800-53-r5 with catalog + policy layers
	addArtifact("nist-800-53-r5", []string{"v1.0.0", "latest"}, []struct {
		mt   string
		data []byte
	}{
		{
			mt: "application/vnd.gemara.catalog.v1+yaml",
			data: []byte(`id: nist-800-53-r5
title: NIST SP 800-53 Rev 5
controls:
  - id: AC-1
    title: Access Control Policy
  - id: AC-2
    title: Account Management
`),
		},
		{
			mt:   "application/vnd.gemara.policy.v1+yaml",
			data: testPolicyYAML,
		},
	})

	// Seed: policies/nist-800-53-r5 — nested (slashed) policy ID matching
	// the standalone mock-oci-registry format
	addArtifact("policies/nist-800-53-r5", []string{"v1.0.0", "latest"}, []struct {
		mt   string
		data []byte
	}{
		{
			mt: "application/vnd.gemara.catalog.v1+yaml",
			data: []byte(`id: nist-800-53-r5
title: NIST SP 800-53 Rev 5
controls:
  - id: AC-1
    title: Access Control Policy
  - id: AC-2
    title: Account Management
`),
		},
		{
			mt:   "application/vnd.gemara.policy.v1+yaml",
			data: testPolicyYAML,
		},
	})

	// Seed: cis-benchmark with catalog layer only
	addArtifact("cis-benchmark", []string{"v2.0.0", "latest"}, []struct {
		mt   string
		data []byte
	}{
		{
			mt: "application/vnd.gemara.catalog.v1+yaml",
			data: []byte(`id: cis-benchmark
title: CIS Benchmark
controls:
  - id: CIS-1.1
    title: Filesystem Configuration
`),
		},
	})

	mux := http.NewServeMux()
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
			names := make([]string, 0, len(repos))
			for n := range repos {
				names = append(names, n)
			}
			sort.Strings(names)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"repositories": names})
			return
		}

		// /v2/{name}/tags/list
		if idx := strings.LastIndex(path, "/tags/list"); idx > 0 {
			repoName := path[:idx]
			repo, ok := repos[repoName]
			if !ok {
				writeRegistryError(w, http.StatusNotFound, "NAME_UNKNOWN", "not found")
				return
			}
			tags := make([]string, 0, len(repo.tags))
			for tag := range repo.tags {
				tags = append(tags, tag)
			}
			sort.Strings(tags)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"name": repoName, "tags": tags})
			return
		}

		// /v2/{name}/manifests/{reference}
		if idx := strings.LastIndex(path, "/manifests/"); idx > 0 {
			repoName := path[:idx]
			ref := path[idx+len("/manifests/"):]

			repo, ok := repos[repoName]
			if !ok {
				writeRegistryError(w, http.StatusNotFound, "NAME_UNKNOWN", "not found")
				return
			}

			art, ok := repo.tags[ref]
			if !ok {
				for _, a := range repo.tags {
					if a.manifestDigest == ref {
						art = a
						ok = true
						break
					}
				}
			}
			if !ok {
				writeRegistryError(w, http.StatusNotFound, "MANIFEST_UNKNOWN", "manifest not found")
				return
			}

			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", art.manifestDigest)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(art.manifestBytes)))
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")

			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(art.manifestBytes)
			return
		}

		// /v2/{name}/blobs/{digest}
		if idx := strings.LastIndex(path, "/blobs/"); idx > 0 {
			repoName := path[:idx]
			digest := path[idx+len("/blobs/"):]

			repo, ok := repos[repoName]
			if !ok {
				writeRegistryError(w, http.StatusNotFound, "NAME_UNKNOWN", "not found")
				return
			}

			for _, art := range repo.tags {
				if b, found := repo.blobs[digest]; found {
					_ = art // suppress unused
					w.Header().Set("Content-Type", "application/octet-stream")
					w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b.data)))
					w.Header().Set("Docker-Content-Digest", digest)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(b.data)
					return
				}
			}
			writeRegistryError(w, http.StatusNotFound, "BLOB_UNKNOWN", "blob not found")
			return
		}

		writeRegistryError(w, http.StatusBadRequest, "NAME_UNKNOWN", "invalid path")
	})

	return httptest.NewServer(mux)
}

func writeRegistryError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"errors": []map[string]string{{"code": code, "message": message}},
	})
}

// --- Binary & Environment Helpers ---

// locateBinary finds the built complyctl binary under bin/.
func locateBinary(t *testing.T) string {
	t.Helper()
	root := findRepoRoot(t)
	binary := filepath.Join(root, "bin", "complyctl")
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("complyctl binary not found at %s — run 'make build' first", binary)
	}
	return binary
}

// findRepoRoot walks up from CWD to find the directory containing go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for dir != "/" {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find repo root (no go.mod found)")
	return ""
}

// installTestPlugin copies the test provider binary into the provider directory
// for the given home directory, matching the complyctl-provider-* naming convention.
func installTestPlugin(t *testing.T, homeDir string) {
	t.Helper()
	root := findRepoRoot(t)
	srcBinary := filepath.Join(root, "bin", "complyctl-provider-test")
	if _, err := os.Stat(srcBinary); err != nil {
		t.Fatalf("test provider binary not found at %s — run 'make build-test-provider' first", srcBinary)
	}

	providerDir := filepath.Join(homeDir, ".complytime", "providers")
	require.NoError(t, os.MkdirAll(providerDir, 0755))

	dstBinary := filepath.Join(providerDir, "complyctl-provider-test")
	data, err := os.ReadFile(srcBinary)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dstBinary, data, 0755))
}

// writeWorkspaceConfig creates a complytime.yaml in dir with the given registry URL.
func writeWorkspaceConfig(t *testing.T, dir, registryURL, policyID string) {
	t.Helper()
	yaml := fmt.Sprintf(`policies:
  - url: %s/%s
    id: %s
variables:
  workspace: /tmp/e2e-workspace
targets:
  - id: e2e-target
    policies:
      - %s
    variables:
      env: test
`, registryURL, policyID, policyID, policyID)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "complytime.yaml"), []byte(yaml), 0644))
}

// copyWorkspaceConfig copies complytime.yaml from src to dst directory.
func copyWorkspaceConfig(t *testing.T, srcDir, dstDir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(srcDir, "complytime.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dstDir, "complytime.yaml"), data, 0644))
}

// buildEnv creates an isolated environment with a custom HOME directory.
func buildEnv(homeDir string) []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "HOME=") {
			continue
		}
		filtered = append(filtered, e)
	}
	return append(filtered, "HOME="+homeDir)
}

// runComplytime executes the complyctl binary with given args and returns combined output.
// Fails the test on non-zero exit code.
func runComplytime(t *testing.T, binary, workDir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = workDir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("complytime %s failed:\n%s\nerror: %v",
			strings.Join(args, " "), string(out), err)
	}
	return string(out)
}

// assertOutputFile finds a file matching prefix+suffix in dir. Returns its path.
// Fails the test if no matching file exists or the file is empty.
func assertOutputFile(t *testing.T, dir, prefix, suffix string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "output directory %s must be readable", dir)

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			path := filepath.Join(dir, name)
			assert.FileExists(t, path)
			info, statErr := os.Stat(path)
			require.NoError(t, statErr)
			assert.Greater(t, info.Size(), int64(0), "%s must not be empty", name)
			return path
		}
	}
	t.Fatalf("no file matching %s*%s found in %s (contents: %v)", prefix, suffix, dir, entries)
	return ""
}
