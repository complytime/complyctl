# Task 9 implementation approach

Follow the supplied Task 9 brief using the existing SDK `bundle.Unpack`.
`BundleArtifacts` holds `Source bundle.File` and `Imports []bundle.File`.
`FilesByType` includes the source; `ImportsByType` excludes it.
The loader normalizes missing types using `Manifest.Artifacts` and
shares unpacking with the compatibility wrapper. Existing validation
and unrelated split-layer behavior remain in place.

The resolver parses the root Policy, then each catalog and guidance
document independently. It appends imported Policy assessments and
mapping references in inventory order and fills absent root defaults.
Catalogs are neither merged nor rewritten. Indexed IDs apply only to
catalogs without IDs. This follows constitution principles I–VII as
assessed in [the proposal](proposal.md).

Tests exercise real SDK packing and cached unpacking, resolver outputs,
malformed supporting documents, root precedence, and type filtering.
Verification includes the brief's focused tests, policy race tests,
the unit suite, formatting, vet, lint, and applicable quality checks.

## Resolved dependency tree freshness

Use a SHA-256 identifier of the resolved Gemara dependency tree to
regenerate only when resolved content changes. Replace the manual
projection version and root OCI digest freshness comparison with this
identifier. Keep the policy digest as provenance in generation state.
Keep complypack digest comparison and missing-artifact checks separate.

Compute the identifier from `DependencyGraph` after resolution, before
workspace variables or targets are applied. Include the policy ID,
parsed control and guidance catalogs with their resolved IDs, resolved
assessments and parameters, effective evaluator, timeline, and mapping
references. Include imported assessments in the same representation as
root assessments. Hash resolved content rather than artifact digests,
raw YAML formatting, paths, or timestamps.

Build a dedicated canonical representation without mutating the graph.
Normalize unordered maps and catalog inventories; preserve sequence
order where it affects precedence or generation behavior. Equivalent
resolved trees MUST produce the same identifier across processes.
Use standard-library encoding and hashing without new dependencies.

Persist the identifier as `resolved_tree_digest` using the
`sha256:<hex>` form. Explicit generate and scan-triggered generation
MUST use the same computation and persist only after successful
generation. Missing identifiers make legacy state stale, including
state with the former `projection_version` field. Loading or saving
legacy state MUST NOT assign a current identifier without generation.
Identifier calculation errors MUST stop generation-state promotion.

The identifier excludes workspace variables, targets, complypack
content, provider versions, and complyctl versions. Complypack changes
continue to trigger generation through their existing digest check.
Explicit `generate` remains the way to rebuild on demand.

Regression tests MUST cover canonical stability, imported assessment
addition/removal/modification with unchanged OCI digests, and changes
to each included graph component. CLI tests MUST show successful
generation followed by reuse, regeneration after a tree or complypack
change, missing-artifact regeneration, legacy-state upgrade, and no
state promotion after generation failure.
