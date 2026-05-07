// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"

	"github.com/gemaraproj/go-gemara"
	"github.com/gemaraproj/go-gemara/internal/codec"
)

// Assembler walks the full import graph of Gemara artifacts and produces
// a self-contained Bundle with all transitive dependencies fetched.
type Assembler struct {
	fetcher gemara.Fetcher
}

// NewAssembler creates an Assembler backed by the given Fetcher.
func NewAssembler(f gemara.Fetcher) *Assembler {
	return &Assembler{fetcher: f}
}

// parsedFile pairs a bundle File with its parsed outbound reference data.
type parsedFile struct {
	File
	id      string
	artType gemara.ArtifactType
	refIDs  []string
	refURLs map[string]string
}

// parseFile decodes a Gemara YAML file into either a gemara.Catalog
// or gemara.Policy and extracts the identity and outbound reference
// information needed for dependency resolution.
func parseFile(f File) (*parsedFile, error) {
	pf := &parsedFile{File: f}
	artType, err := gemara.DetectType(f.Data)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", f.Name, err)
	}

	switch artType {
	case gemara.PolicyArtifact:
		var pol gemara.Policy
		if err := codec.UnmarshalYAML(f.Data, &pol); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", f.Name, err)
		}
		pf.id = pol.Metadata.Id
		pf.artType = pol.Metadata.Type
		pf.refURLs = mappingRefURLs(pol.Metadata.MappingReferences)
		pf.refIDs = policyRefIDs(pol.Imports)
	default:
		var cat gemara.Catalog
		if err := codec.UnmarshalYAML(f.Data, &cat); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", f.Name, err)
		}
		pf.id = cat.Metadata.Id
		pf.artType = cat.Metadata.Type
		pf.refURLs = mappingRefURLs(cat.Metadata.MappingReferences)
		pf.refIDs = catalogRefIDs(cat.Extends, cat.Imports)
	}

	return pf, nil
}

// Assemble parses each source file, fetches every artifact referenced in
// their extends and imports via mapping-references URLs, then
// recursively parses fetched artifacts for their own references until the
// full dependency tree is resolved.
func (a *Assembler) Assemble(ctx context.Context, m Manifest, sources ...File) (*Bundle, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("at least one source file is required")
	}

	seen := make(map[string]bool)
	depMap := make(map[string][]string)

	var sourceParsed []*parsedFile
	var queue []fetchRef

	for _, f := range sources {
		pf, err := parseFile(f)
		if err != nil {
			return nil, err
		}
		sourceParsed = append(sourceParsed, pf)
		queue = append(queue, enqueueRefs(pf, seen, depMap)...)
	}

	var importParsed []*parsedFile

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if seen[item.url] {
			continue
		}
		seen[item.url] = true

		data, err := a.fetchAll(ctx, item.url)
		if err != nil {
			return nil, fmt.Errorf("fetching dependency %q from %s: %w", item.refID, item.url, err)
		}

		f := File{
			Name: importFileName(item.refID, item.url),
			Data: data,
		}

		pf, err := parseFile(f)
		if err != nil {
			return nil, fmt.Errorf("parsing transitive refs from %s: %w", f.Name, err)
		}
		importParsed = append(importParsed, pf)
		queue = append(queue, enqueueRefs(pf, seen, depMap)...)
	}

	files := make([]File, len(sources))
	copy(files, sources)

	var imports []File
	for _, pf := range importParsed {
		imports = append(imports, pf.File)
	}

	m.Artifacts = buildArtifactTree(sourceParsed, importParsed, depMap)

	return &Bundle{
		Manifest: m,
		Files:    files,
		Imports:  imports,
	}, nil
}

// fetchRef is a URL-keyed reference waiting to be fetched.
type fetchRef struct {
	refID string
	url   string
}

// enqueueRefs builds fetchRef items for each outbound reference that
// has a resolvable URL and hasn't been seen yet. It also records
// dependency edges in depMap.
func enqueueRefs(pf *parsedFile, seen map[string]bool, depMap map[string][]string) []fetchRef {
	var refs []fetchRef
	for _, refID := range pf.refIDs {
		fetchURL, ok := pf.refURLs[refID]
		if !ok {
			continue
		}

		depName := importFileName(refID, fetchURL)
		depMap[pf.Name] = append(depMap[pf.Name], depName)

		if seen[fetchURL] {
			continue
		}
		refs = append(refs, fetchRef{refID: refID, url: fetchURL})
	}
	return refs
}

// mappingRefURLs builds an id -> url lookup from MappingReferences.
func mappingRefURLs(refs []gemara.MappingReference) map[string]string {
	m := make(map[string]string, len(refs))
	for _, ref := range refs {
		if ref.Url != "" {
			m[ref.Id] = ref.Url
		}
	}
	return m
}

// catalogRefIDs collects all reference IDs from extends and catalog-style imports.
func catalogRefIDs(extends []gemara.ArtifactMapping, imports []gemara.MultiEntryMapping) []string {
	var ids []string
	for _, ext := range extends {
		if ext.ReferenceId != "" {
			ids = append(ids, ext.ReferenceId)
		}
	}
	for _, imp := range imports {
		if imp.ReferenceId != "" {
			ids = append(ids, imp.ReferenceId)
		}
	}
	return ids
}

// policyRefIDs collects all reference IDs from a Policy's typed imports.
func policyRefIDs(imports gemara.Imports) []string {
	var ids []string
	for _, p := range imports.Policies {
		if p.ReferenceId != "" {
			ids = append(ids, p.ReferenceId)
		}
	}
	for _, c := range imports.Catalogs {
		if c.ReferenceId != "" {
			ids = append(ids, c.ReferenceId)
		}
	}
	for _, g := range imports.Guidance {
		if g.ReferenceId != "" {
			ids = append(ids, g.ReferenceId)
		}
	}
	return ids
}

// buildArtifactTree constructs the Manifest.Artifacts slice from the
// already-parsed files and their dependency relationships.
func buildArtifactTree(files, imports []*parsedFile, depMap map[string][]string) []Artifact {
	artifacts := make([]Artifact, 0, len(files)+len(imports))
	for _, pf := range files {
		artifacts = append(artifacts, Artifact{
			Name:         pf.Name,
			Type:         pf.artType.String(),
			ID:           pf.id,
			Role:         roleArtifact,
			Dependencies: depMap[pf.Name],
		})
	}
	for _, pf := range imports {
		artifacts = append(artifacts, Artifact{
			Name:         pf.Name,
			Type:         pf.artType.String(),
			ID:           pf.id,
			Role:         roleImport,
			Dependencies: depMap[pf.Name],
		})
	}
	return artifacts
}

func (a *Assembler) fetchAll(ctx context.Context, source string) ([]byte, error) {
	rc, err := a.fetcher.Fetch(ctx, source)
	if err != nil {
		return nil, err
	}
	defer rc.Close() //nolint:errcheck
	return io.ReadAll(rc)
}

// importFileName derives a bundle-relative filename for a resolved import.
func importFileName(refID, rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		if base := path.Base(u.Path); base != "" && base != "." && base != "/" {
			return base
		}
	}
	return refID + ".yaml"
}
