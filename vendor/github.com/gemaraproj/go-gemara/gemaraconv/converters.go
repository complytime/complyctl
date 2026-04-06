package gemaraconv

import (
	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/gemaraproj/go-gemara"
)

// EvaluationLogConverter define a converter object for converting EvaluationLog.
type EvaluationLogConverter struct {
	log *gemara.EvaluationLog
}

// EvaluationLog creates a new EvaluationLogConverter struct.
func EvaluationLog(log *gemara.EvaluationLog) *EvaluationLogConverter {
	return &EvaluationLogConverter{log: log}
}

// ToSARIF converts the EvaluationLog to SARIF format.
func (c *EvaluationLogConverter) ToSARIF(artifactURI string, catalog *gemara.ControlCatalog) ([]byte, error) {
	return ToSARIF(*c.log, artifactURI, catalog)
}

// ControlCatalogConverter defines a converter for converting ControlCatalog.
type ControlCatalogConverter struct {
	catalog *gemara.ControlCatalog
}

// ControlCatalog creates a new ControlCatalogConverter struct.
func ControlCatalog(catalog *gemara.ControlCatalog) *ControlCatalogConverter {
	return &ControlCatalogConverter{catalog: catalog}
}

// ToOSCAL converts the ControlCatalog to OSCAL format.
func (c *ControlCatalogConverter) ToOSCAL(opts ...GenerateOption) (oscal.Catalog, error) {
	return CatalogToOSCAL(c.catalog, opts...)
}

// ToMarkdown converts the ControlCatalog to Markdown format.
func (c *ControlCatalogConverter) ToMarkdown(opts ...MarkdownOption) ([]byte, error) {
	return CatalogToMarkdown(c.catalog, opts...)
}

// GuidanceCatalogConverter defines a converter for converting GuidanceCatalog.
type GuidanceCatalogConverter struct {
	guidance *gemara.GuidanceCatalog
}

// GuidanceCatalog creates a new GuidanceCatalogConverter struct.
func GuidanceCatalog(guidance *gemara.GuidanceCatalog) *GuidanceCatalogConverter {
	return &GuidanceCatalogConverter{guidance: guidance}
}

// ToOSCAL converts the GuidanceCatalog to an OSCAL Catalog and Profile.
func (c *GuidanceCatalogConverter) ToOSCAL(guidanceDocHref string, opts ...GenerateOption) (oscal.Catalog, oscal.Profile, error) {
	return GuidanceToOSCAL(c.guidance, guidanceDocHref, opts...)
}
