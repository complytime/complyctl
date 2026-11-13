# Tasks

Implementation starts only after these planning artifacts are reviewed,
committed, and pushed. All tasks below remain implementation work.

## 1. Invocation and mapped evidence checks

- [x] 1.1 Extract reusable Bash/jq evidence-checking helpers under
  `tests/cross-repo/`; verify the existing generation-index fixtures
  still pass and expected failures preserve assertion counters.
- [x] 1.2 Select the invocation's JSON log and expected repository,
  target, and `block-force-push` assessment (FR-001/002); verify valid,
  stale, wrong-target, malformed, ambiguous, and absent-log fixtures.
- [x] 1.3 Resolve coordinates within the canonical workspace and verify
  regular files and SHA-256 digests (FR-003); verify relative/absolute
  success and missing, empty, changed, bad-digest, and escape failures.
- [x] 1.4 Verify exact reference-ID resolution and local file-URL
  agreement (FR-004); verify encoded paths, duplicate/missing IDs,
  invalid/remote URLs, and equal-content wrong-locator fixtures.
- [x] 1.5 Replace both live attestation globs with mapped selection and
  retain the selected Ampel file's content assertion (FR-002/006);
  verify legacy/retained fixtures and rejection of unmapped decoys,
  unrelated-assessment evidence, and missing requirement content.
- [x] 1.6 Document helper errors and mapped lookup in existing
  testing guidance; verify descriptions match the exercised fixtures
  and do not describe a fixed filename as an evidence contract.

## 2. Historical and identity regression coverage

- [x] 2.1 Add two-evaluation offline fixtures that snapshot the
  first log and rehash files after the second (FR-005); verify a
  retained case passes and overwrite/removal cases fail.
- [x] 2.2 Add separate equal-content artifact instances and shared-use
  cases (FR-005/006); verify unique IDs/references/coordinates survive,
  collapsed instances fail, and nearby metadata is not selected.
- [x] 2.3 Add `make test-cross-repo-evidence` for offline regressions;
  verify it succeeds with no token/provider binaries and that injected
  evidence failures make it return nonzero.
- [x] 2.4 Document the offline target, prerequisites, and distinction
  from live retention acceptance; assess `AGENTS.md` for the added
  command and verify the target executes as documented.

## 3. Explicit host selection

- [x] 3.1 Add optional absolute `COMPLYCTL_BIN_DIR` selection for both
  host executables (FR-007); verify default, external, and invalid
  directory cases with distinguishable executable fixtures.
- [x] 3.2 Verify canonical harness fixture ownership and no fallback or
  implicit build using the same dispatch path; fail tests if either
  default executable runs after explicit host selection.
- [x] 3.3 Document default and external-host invocation in the existing
  testing guidance; verify paths with spaces and document that the
  caller must prebuild both externally selected host binaries.

## 4. JSON schema coverage

- [x] 4.1 Extend log export and schema discovery to JSON and YAML
  (FR-008); verify JSON-only and mixed-format cases include the Ampel
  log without changing CUE definitions, schema pins, or exit behavior.
- [x] 4.2 Add invalid JSON/YAML and no-log cases; verify CUE rejects
  invalid documents and absence remains a configuration failure.
- [x] 4.3 Update affected script comments and testing/schema command
  descriptions; verify they describe both serializations and introduce
  no YAML parser or extra credentialed scan.

## 5. Existing CI integration

- [x] 5.1 Run the offline target before credentialed execution in the
  existing cross-repository workflow; verify shellcheck, actionlint,
  and configured YAML lint pass without changing job identities,
  timeouts, token scopes, or quality/governance gates.
- [x] 5.2 Verify Get, generation-index checks, bad-policy rejection,
  and both OPA fixture checks still execute (FR-006); run the
  existing indexed-generation fixture mode and offline evidence target.
- [x] 5.3 Assess `CHANGELOG.md`, `AGENTS.md`, and `README.md` with
  touched testing documentation; verify each necessary command or
  behavior update is present, or record why none is needed.

## 6. System verification and handoff

- [x] 6.1 Run applicable checks derived from the current workflows,
  including offline regressions, lint, and affected schema validation;
  record commands and actual results without marking skipped checks
  successful or altering any quality/governance gate.
- [x] 6.2 Run the canonical harness against current complyctl and
  unchanged v1.2.0 with the selected retained-provider build; record
  exact host/provider revisions, tool versions, and real credentialed
  results, leaving this task incomplete if prerequisites are absent.
- [x] 6.3 Exercise two actual retained-provider evaluations in one
  workspace, snapshot both actual JSON logs, and apply the same history
  and identity checks; verify earlier digests remain valid, separating
  these results from offline fixtures and schema-only validation.
- [x] 6.4 Validate this change strictly and complete required pre-PR
  review; verify all blocking findings are resolved before submission
  and no code changes follow final approval.

## Workflow follow-up

- Merge this complyctl PR before the providers wiring PR selects its
  full canonical harness commit. Do not invent or prefill that commit.
- Supply the merged commit, `COMPLYCTL_BIN_DIR` contract, and actual
  verification results to `use-reviewed-harness-for-release-validation`
  in the providers repository; do not implement that change here.
- Archive only after implementation and required review are complete.
