// SPDX-License-Identifier: Apache-2.0

package behavioral

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
)

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

// StartMockRegistry creates an in-process mock OCI registry implementing
// the OCI Distribution Spec v2 endpoints with preseeded policy artifacts.
func StartMockRegistry() *httptest.Server {
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

	policyLayer := []byte(`- id: AC-1-impl
  evaluator_id: test
  parameters:
    control_id: AC-1
- id: AC-2-impl
  evaluator_id: test
  parameters:
    control_id: AC-2
`)
	catalogLayer := []byte(`id: nist-800-53-r5
title: NIST SP 800-53 Rev 5
controls:
  - id: AC-1
    title: Access Control Policy
  - id: AC-2
    title: Account Management
`)

	addArtifact("nist-800-53-r5", []string{"v1.0.0", "latest"}, []struct {
		mt   string
		data []byte
	}{
		{mt: "application/vnd.gemara.catalog.v1+yaml", data: catalogLayer},
		{mt: "application/vnd.gemara.policy.v1+yaml", data: policyLayer},
	})

	addArtifact("policies/nist-800-53-r5", []string{"v1.0.0", "latest"}, []struct {
		mt   string
		data []byte
	}{
		{mt: "application/vnd.gemara.catalog.v1+yaml", data: catalogLayer},
		{mt: "application/vnd.gemara.policy.v1+yaml", data: policyLayer},
	})

	addArtifact("cis-benchmark", []string{"v2.0.0", "latest"}, []struct {
		mt   string
		data []byte
	}{
		{mt: "application/vnd.gemara.catalog.v1+yaml", data: []byte(`id: cis-benchmark
title: CIS Benchmark
controls:
  - id: CIS-1.1
    title: Filesystem Configuration
`)},
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/v2/")
		path = strings.TrimSuffix(path, "/")

		if path == "" {
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
			return
		}

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

		if idx := strings.LastIndex(path, "/blobs/"); idx > 0 {
			repoName := path[:idx]
			digest := path[idx+len("/blobs/"):]

			repo, ok := repos[repoName]
			if !ok {
				writeRegistryError(w, http.StatusNotFound, "NAME_UNKNOWN", "not found")
				return
			}

			if b, found := repo.blobs[digest]; found {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b.data)))
				w.Header().Set("Docker-Content-Digest", digest)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(b.data)
				return
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

// BuildEnv creates an isolated environment with a custom HOME directory.
// XDG env vars are cleared so the CLI falls back to HOME-relative defaults:
// cache → $HOME/.cache/complytime, data → $HOME/.local/share/complytime.
func BuildEnv(homeDir string) []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "HOME=") ||
			strings.HasPrefix(e, "XDG_CACHE_HOME=") ||
			strings.HasPrefix(e, "XDG_DATA_HOME=") {
			continue
		}
		filtered = append(filtered, e)
	}
	return append(filtered, "HOME="+homeDir)
}
