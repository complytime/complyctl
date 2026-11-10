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
