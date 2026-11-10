# Preserve every Gemara bundle artifact

## Why

The bundle consumer currently keeps only the first artifact of each
type. Multi-artifact bundles therefore lose supporting catalogs and
imported Policy assessments during resolution.

## What Changes

Task 9 adds a source/import inventory through `LoadBundleArtifacts`.
The resolver consumes all catalogs, guidance, and imported Policies.
`LoadBundleFiles` remains a first-per-type compatibility wrapper.

## Capabilities

### Added Capabilities

- `bundle-artifact-inventory`: lossless loading and graph resolution.

## Impact

Only `internal/policy` loading and resolution change. The existing SDK
unpacks standard OCI layouts; dependencies and formats stay unchanged.
The changelog records the user-visible correction.

## Constitution Alignment

The change follows the ComplyTime constitution: reuse SDK unpacking
(V), preserve composable artifacts (VI), and scope work to Task 9 (III).
Shared artifact-type constants support a single source of truth (I).
Focused functions and regression tests support simplicity and
readability (II, IV); existing CLI configuration remains sufficient
(VII). No governance exceptions are required.
