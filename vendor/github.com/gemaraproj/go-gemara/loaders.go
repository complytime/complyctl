// SPDX-License-Identifier: Apache-2.0

package gemara

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"

	"github.com/gemaraproj/go-gemara/internal/codec"
)

// TypeMeta is the minimal discriminator parsed from raw YAML/JSON to
// determine which concrete Gemara type to unmarshal into
type TypeMeta struct {
	Metadata struct {
		Type ArtifactType `json:"type" yaml:"type"`
	} `json:"metadata" yaml:"metadata"`
}

// DetectType parses only the metadata.type field from raw artifact data,
// returning the ArtifactType without requiring a full unmarshal.
func DetectType(data []byte) (ArtifactType, error) {
	var tm TypeMeta
	if err := codec.UnmarshalYAML(data, &tm); err != nil {
		return InvalidArtifact, fmt.Errorf("reading metadata.type: %w", err)
	}
	return tm.Metadata.Type, nil
}

// Decode unmarshals raw YAML or JSON bytes into a Gemara type. It is the
// in-memory counterpart to [Load], for callers that already hold the bytes
// (for example from bundle.Unpack) and so have no source URL to fetch.
//
// JSON is decoded as a subset of YAML, so no format detection is needed.
// Unknown fields are ignored, letting artifacts written against a newer
// Gemara schema decode into the types this version supports; the unknown
// keys are dropped rather than preserved, so re-serializing writes only
// the schema version this library is at.
func Decode[T any](data []byte) (*T, error) {
	var target T
	if err := codec.UnmarshalYAML(data, &target); err != nil {
		return nil, err
	}
	return &target, nil
}

// Fetcher retrieves content from a source location.
//
// Network-capable implementations (e.g. [fetcher.HTTP], [fetcher.URI])
// will follow any URL without filtering; see their documentation for
// details.
type Fetcher interface {
	Fetch(ctx context.Context, source string) (io.ReadCloser, error)
}

// Load fetches and decodes a single artifact from the given source.
// Format (YAML or JSON) is detected from the file extension.
//
// Unknown fields are ignored, letting artifacts written against a newer
// Gemara schema decode into the types this version supports; the unknown
// keys are dropped rather than preserved, so re-serializing writes only
// the schema version this library is at.
func Load[T any](ctx context.Context, f Fetcher, source string) (*T, error) {
	u, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("invalid source %q: %w", source, err)
	}
	p := u.Path
	if p == "" {
		p = source
	}
	ext := path.Ext(p)
	switch ext {
	case ".yaml", ".yml", ".json":
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	reader, err := f.Fetch(ctx, source)
	if err != nil {
		return nil, err
	}
	defer reader.Close() //nolint:errcheck

	var target T
	switch ext {
	case ".yaml", ".yml":
		if err := codec.DecodeYAML(reader, &target); err != nil {
			return nil, err
		}
	case ".json":
		if err := codec.DecodeJSON(reader, &target); err != nil {
			return nil, fmt.Errorf("error loading json: %w", err)
		}
	}
	return &target, nil
}

// LoadFiles loads and merges data from multiple sources into the GuidanceCatalog.
//
// The sources are fragments of one catalog. The receiver supplies the catalog's
// identity (metadata, title, type and front matter); when the receiver has no
// metadata.id, the first source supplies it instead. Groups, extends, imports,
// guidelines and exemptions are appended from every source without
// deduplication. A source may omit metadata.id; a source whose metadata.id
// differs from the catalog's is an error, because a catalog with a different
// identity belongs in imports or extends.
func (g *GuidanceCatalog) LoadFiles(ctx context.Context, f Fetcher, sources []string) error {
	hasIdentity := g.Metadata.Id != ""
	for _, source := range sources {
		doc, err := Load[GuidanceCatalog](ctx, f, source)
		if err != nil {
			return err
		}
		if !hasIdentity {
			g.Metadata = doc.Metadata
			g.Title = doc.Title
			g.GuidanceType = doc.GuidanceType
			g.FrontMatter = doc.FrontMatter
			hasIdentity = true
		} else if err := checkFragmentId(source, doc.Metadata.Id, g.Metadata.Id); err != nil {
			return err
		}
		g.Groups = append(g.Groups, doc.Groups...)
		g.Extends = append(g.Extends, doc.Extends...)
		g.Imports = append(g.Imports, doc.Imports...)
		g.Guidelines = append(g.Guidelines, doc.Guidelines...)
		g.Exemptions = append(g.Exemptions, doc.Exemptions...)
	}
	return nil
}

// LoadFiles loads and merges data from multiple sources into the ControlCatalog.
//
// The sources are fragments of one catalog. The receiver supplies the catalog's
// identity (metadata and title); when the receiver has no metadata.id, the
// first source supplies it instead. Groups, extends, controls and imports are
// appended from every source without deduplication. A source may omit
// metadata.id; a source whose metadata.id differs from the catalog's is an
// error, because a catalog with a different identity belongs in imports or
// extends.
func (c *ControlCatalog) LoadFiles(ctx context.Context, f Fetcher, sources []string) error {
	hasIdentity := c.Metadata.Id != ""
	for _, source := range sources {
		catalog, err := Load[ControlCatalog](ctx, f, source)
		if err != nil {
			return err
		}
		if !hasIdentity {
			c.Metadata = catalog.Metadata
			c.Title = catalog.Title
			hasIdentity = true
		} else if err := checkFragmentId(source, catalog.Metadata.Id, c.Metadata.Id); err != nil {
			return err
		}
		c.Groups = append(c.Groups, catalog.Groups...)
		c.Extends = append(c.Extends, catalog.Extends...)
		c.Controls = append(c.Controls, catalog.Controls...)
		if catalog.Imports != nil {
			if c.Imports == nil {
				c.Imports = []MultiEntryMapping{}
			}
			c.Imports = append(c.Imports, catalog.Imports...)
		}
	}
	return nil
}

// checkFragmentId rejects a source that declares a catalog identity other
// than the one being loaded. Sources without a metadata.id are fragments.
func checkFragmentId(source, sourceId, catalogId string) error {
	if sourceId == "" || sourceId == catalogId {
		return nil
	}
	return fmt.Errorf("%s has metadata.id %q but the catalog being loaded is %q; load a catalog with a different identity through imports or extends", source, sourceId, catalogId)
}

// LoadNestedCatalog loads a YAML file where the ControlCatalog is nested
// under a single wrapper key (e.g. "catalog:"). Only supports one layer
// of nesting.
func (c *ControlCatalog) LoadNestedCatalog(ctx context.Context, f Fetcher, source, fieldName string) error {
	if fieldName == "" {
		return fmt.Errorf("fieldName cannot be empty")
	}

	data, err := Load[map[string]interface{}](ctx, f, source)
	if err != nil {
		return fmt.Errorf("error decoding source: %w (%s)", err, source)
	}

	fieldData, exists := (*data)[fieldName]
	if !exists {
		return fmt.Errorf("field %q not found in %s", fieldName, source)
	}

	fieldBytes, err := codec.MarshalYAML(fieldData)
	if err != nil {
		return fmt.Errorf("error marshaling nested field: %w", err)
	}
	if err := codec.UnmarshalYAML(fieldBytes, c); err != nil {
		return fmt.Errorf("error decoding field %q into ControlCatalog: %w", fieldName, err)
	}
	return nil
}
