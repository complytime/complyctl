# Preserve every Gemara bundle artifact

## Why

The bundle consumer currently keeps only the first artifact of each
type. Multi-artifact bundles therefore lose supporting catalogs and
imported Policy assessments during resolution.

## What Changes

Task 9 adds a source/import inventory through `LoadBundleArtifacts`.
The resolver consumes all catalogs, guidance, and imported Policies.
`LoadBundleFiles` remains a first-per-type compatibility wrapper.

Generation freshness will use a content-derived identifier of the
resolved Gemara dependency tree instead of a manually incremented
projection version or the root policy OCI digest. Complypack digest
and missing-artifact checks remain separate freshness conditions.

## Capabilities

### Added Capabilities

- `bundle-artifact-inventory`: lossless loading and graph resolution.

## Impact

Changes cover `internal/policy` loading, resolution, and generation
state, plus CLI generate/scan freshness integration. The existing SDK
unpacks standard OCI layouts; no new dependency is required. Existing
generation state without a tree identifier regenerates once after
upgrade. The changelog records the user-visible correction.

## Constitution Alignment

The change follows the ComplyTime constitution: reuse SDK unpacking
(V), preserve composable artifacts (VI), and scope work to Task 9 (III).
Shared artifact-type constants support a single source of truth (I).
Focused functions and regression tests support simplicity and
readability (II, IV); existing CLI configuration remains sufficient
(VII). No governance exceptions are required.
