# Proposal

## Why

PR #956 added OSCAL evidence conversion and formatter-level tests, but
the full scan acceptance test can still pass when provider evidence
disappears from the report. Explicit evidence assertions will protect
the provider-to-OSCAL pipeline against that regression.

## What Changes

- Extend the existing container-based scan acceptance coverage to
  check `relevant-evidence` on the expected assessment observation.
- Verify evidence descriptions and source provenance against the
  existing test-provider fixture, including the finding linkage.
- Validate the complete evidence-bearing report against OSCAL 1.1.3
  using the existing go-oscal dependency.
- Demonstrate that removing evidence fails the acceptance assertions
  even when observations and findings remain intact.

## Capabilities

### New Capabilities

- `oscal-evidence-acceptance`: Acceptance-suite verification of evidence
  preservation, observation linkage, and schema validity through the
  real CLI and provider pipeline.

### Modified Capabilities

None. The production report contract remains the one implemented by
PR #956. No canonical specifications exist under `openspec/specs/`;
the earlier change records that contract in
`openspec/changes/delegate-oscal-conversion/`.

## Impact

- Expected implementation files are
  `tests/acceptance/acceptance_test.go` and
  `tests/acceptance/helpers_test.go`.
- Reuse `cmd/test-provider/main.go`, which already emits evidence with
  descriptions, reference IDs, coordinates, and digests.
- Reuse the existing lifecycle Compose stack and
  `.github/workflows/acceptance_test.yml` invocation.
- No new dependencies, provider API changes, production conversion
  changes, or CI configuration changes are required.
- Assess `CHANGELOG.md`, `AGENTS.md`, and `README.md` during
  implementation; user-facing behavior and build commands do not
  change.

Related work: [PR #956](https://github.com/complytime/complyctl/pull/956).
