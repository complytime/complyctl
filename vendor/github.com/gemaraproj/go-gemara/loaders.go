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
func Load[T any](ctx context.Context, f Fetcher, source string) (*T, error) {
	u, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("invalid source %q: %w", source, err)
	}
	ext := path.Ext(u.Path)
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
func (g *GuidanceCatalog) LoadFiles(ctx context.Context, f Fetcher, sources []string) error {
	for _, source := range sources {
		doc, err := Load[GuidanceCatalog](ctx, f, source)
		if err != nil {
			return err
		}
		if g.Metadata.Id == "" {
			g.Metadata = doc.Metadata
		}
		g.Groups = append(g.Groups, doc.Groups...)
		g.Guidelines = append(g.Guidelines, doc.Guidelines...)
	}
	return nil
}

// LoadFiles loads and merges data from multiple sources into the ControlCatalog.
func (c *ControlCatalog) LoadFiles(ctx context.Context, f Fetcher, sources []string) error {
	for _, source := range sources {
		catalog, err := Load[ControlCatalog](ctx, f, source)
		if err != nil {
			return err
		}
		if c.Metadata.Id == "" {
			c.Metadata = catalog.Metadata
		}
		c.Groups = append(c.Groups, catalog.Groups...)
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
