# Proposal

## Why

The cross-repository harness infers evidence from moving filenames,
rejecting successful scans from providers that retain each evaluation.
Issue [#970][issue] requires
the harness to validate the evidence cited by the actual EvaluationLog
and make those assertions reusable against unchanged released hosts.

## What Changes

- Select JSON EvaluationLogs from each scan invocation and follow their
  assessment evidence mappings instead of searching for attestations.
- Verify workspace-relative coordinates, SHA-256 digests, reference-ID
  linkage, and matching absolute file URLs. Preserve the existing Ampel
  requirement-content assertion and all other pipeline assertions.
- Cover legacy and retained layouts through returned mappings, with
  negative fixtures, historical evidence checks, instance identity, and
  supporting metadata that is not evidence.
- Add optional `COMPLYCTL_BIN_DIR` input. It selects both host
  binaries without changing the checkout that supplies the harness.
- Export and schema-validate JSON and YAML logs without adding
  a YAML parser or changing the Gemara schema pin.
- Keep production code, provider APIs, release selection, and provider
  retention behavior outside this repository-local change.

## Capabilities

### New Capabilities

- `cross-repo-integration-test`: Mapping-driven evidence checks and
  explicit host selection for the Ampel/OPA integration pipeline. This
  reuses the capability name in the archived target-scoped generation
  change; no corresponding main spec is currently present.

### Modified Capabilities

None. The capability inventory contains no published main specs.

## Impact

- Primary implementation:
  `tests/cross-repo/cross_repo_integration_test.sh`
  and focused offline regressions under `tests/cross-repo/`.
- Supporting implementation: `Makefile`, the schema-validation script,
  and cross-repository CI test invocation/documentation as needed.
- Use Bash, jq, and existing CI tools; do not add a YAML dependency.
  Preserve timeouts, credentials scoping, and quality gates.
- No CLI, protobuf, handshake, or production formatter changes.
- The separate providers change is
  `use-reviewed-harness-for-release-validation` under that repository's
  `openspec/changes/`. It consumes a merged immutable harness revision;
  this change does not edit providers or require a new host release.
- Implement and review this PR first. Record documentation impact for
  `CHANGELOG.md`, `AGENTS.md`, and `README.md` during implementation.

[issue]: https://github.com/complytime/complyctl/issues/970
