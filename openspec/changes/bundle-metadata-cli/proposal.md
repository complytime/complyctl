## Why

After `complyctl get` pulls a Gemara bundle from an OCI registry, the
CLI discards all content metadata. Users cannot discover the policy
title, evaluator type, required target variables, or control count
without manually inspecting OCI blobs via `oras blob fetch`. The
`policies[].id` in `complytime.yaml` is a disconnected local alias
with no validation against bundle content -- `id: banana` works
identically to `id: cis-fedora-l1-workstation-policy`.

As reported in [#506](https://github.com/complytime/complyctl/issues/506),
this problem worsens with multiple providers (OPA, Ampel, OpenSCAP)
because users need to know which evaluator a policy requires to
configure the correct target variables.

The OCI config blob and policy YAML layers already contain all the
metadata needed (title, evaluator ID, assessment plans, control
catalog). This data is parsed during `scan` via `ResolvePolicyGraph()`
but is never surfaced to users outside of scan reports.

## What Changes

- **Option B**: Enrich `complyctl list` output with policy metadata
  columns (EVALUATOR, CONTROLS) sourced from cached state
- **Option C**: `complyctl get` prints a post-sync summary after
  each freshly fetched policy, showing title, evaluator, and counts

Option A (`complyctl inspect <url>` for pre-pull discovery) is
deferred to a follow-up issue.

## Capabilities

### New Capabilities
- `get-sync-summary`: After a fresh policy fetch, `complyctl get`
  prints a human-readable summary to stderr showing the policy
  title, evaluator, control count, and assessment count
- `policy-metadata-cache`: `PolicyState` in `state.json` gains
  metadata fields (`policy_title`, `policy_evaluator`,
  `control_count`, `assessment_count`) populated at sync time
- `extract-policy-metadata`: `policy.Resolver` gains
  `ExtractPolicyMetadata()` for lightweight metadata extraction
  without building a full `DependencyGraph`

### Modified Capabilities
- `list-output`: `complyctl list` table gains EVALUATOR and
  CONTROLS columns sourced from cached `PolicyState` metadata

### Removed Capabilities
None.

## Impact

- `internal/cache/state.go`: Add metadata fields to `PolicyState`
- `internal/policy/resolver.go`: Add `PolicyMetadata` type and
  `ExtractPolicyMetadata()` method
- `cmd/complyctl/cli/get.go`: Call metadata extraction after fresh
  sync, print summary to stderr, persist metadata to state
- `cmd/complyctl/cli/list.go`: Add EVALUATOR and CONTROLS columns
- Tests: New tests for metadata extraction, state round-trip,
  list output, and get summary output

## Documentation Impact

- `CHANGELOG.md`: Entry under `### Added` for enriched `list`
  output and `get` post-sync summary
- `AGENTS.md`: Update Recent Changes section

## Constitution Alignment

Assessed against the Unbound Force org constitution.

### I. Autonomous Collaboration

**Assessment**: PASS

The metadata is persisted in `state.json` (an existing
artifact) using the existing JSON serialization pattern.
Both `list` and `get` summary consume the same cached
metadata, producing self-describing output without
requiring interactive discovery.

### II. Composability First

**Assessment**: PASS

`ExtractPolicyMetadata()` is a standalone method on
`Resolver` that reuses existing YAML parsing functions
(`parsePolicyLayer`, `parseControlCatalog`) without
introducing new dependencies. The metadata cache is
additive to `PolicyState` -- no existing fields or methods
change. `list` reads metadata from state without needing
the Resolver or OCI store at display time.

### III. Observable Quality

**Assessment**: PASS

All metadata fields are machine-parseable (JSON in
`state.json`, plain-text table in `list`, structured
stderr lines in `get` summary). The `list` table format
is tab-aligned and consistent with existing output. No
provenance metadata is lost -- the existing digest,
version, and verification fields are preserved.

### IV. Testability

**Assessment**: PASS

Each component is testable in isolation:
- `PolicyState` JSON round-trip with new fields (unit test)
- `ExtractPolicyMetadata()` with mock `PolicyLoader`
  (unit test)
- `SetPolicyMetadata()` non-overwrite guarantee (unit test)
- `list` output with pre-populated state (unit test)
- `get` summary output via stderr capture (unit test)

### ComplyTime Constitution Alignment

Additionally assessed against the ComplyTime constitution
principles in `.specify/memory/constitution.md`:

- **I. Single Source of Truth**: Metadata cached in
  `state.json` (single source), not duplicated elsewhere.
  `list` and `get` both read from the same state file.
- **II. Simplicity & Isolation**: `ExtractPolicyMetadata()`
  is a focused method with single responsibility, separate
  from `ResolvePolicyGraph()`. `SetPolicyMetadata()` uses
  read-modify-write to preserve sync fields.
- **III. Incremental Improvement**: Single concern (metadata
  surfacing), no unrelated refactoring or formatting changes.
- **IV. Readability First**: Clear naming distinguishes
  `PolicyEvaluator` (policy metadata) from `EvaluatorID`
  (complypack binding). Design decisions documented with
  rationale and alternatives.
- **V. Do Not Reinvent the Wheel**: Reuses existing YAML
  parsing functions and mock infrastructure.
- **VI. Composability**: `list` reads state without needing
  Resolver; `get` extracts and caches independently.
  Output is tab-aligned plain text, consumable by scripts.
- **VII. Convention Over Configuration**: No new
  configuration required; metadata appears automatically
  after `get`. Follows existing `omitempty` pattern for
  backward compatibility.
