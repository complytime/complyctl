// Package gemaraconv provides conversion functions to transform Gemara documents
// into various standard formats.
//
// Primary API (strconv-style):
//   - Direct functions: ToSARIF(), CatalogToOSCAL(), CatalogToMarkdown(), GuidanceToOSCAL()
//
// Fluent Wrappers (for IDE discoverability):
//   - EvaluationLog(), ControlCatalog(), GuidanceCatalog()
//   - Thin wrappers that delegate to the primary functions
//
// Examples:
//
//	sarifBytes, err := gemaraconv.ToSARIF(log, gemaraconv.WithArtifactURI("file.md"), gemaraconv.WithCatalog(catalog))
//	oscalCatalog, err := gemaraconv.CatalogToOSCAL(catalog, gemaraconv.WithVersion("1.0"))
//	md, err := gemaraconv.CatalogToMarkdown(ctx, catalog, gemaraconv.WithTOC(true), gemaraconv.WithLexiconAutolink(true))
//	// Or pass list-shaped entries: WithInlineLexicon([]gemaraconv.InlineLexiconTerm{...})
//	converter := gemaraconv.EvaluationLog(log)
//	sarifBytes, err := converter.ToSARIF(gemaraconv.WithArtifactURI("file.md"), gemaraconv.WithCatalog(catalog))
//	md, err := gemaraconv.ControlCatalog(catalog).ToMarkdown(ctx)
package gemaraconv
