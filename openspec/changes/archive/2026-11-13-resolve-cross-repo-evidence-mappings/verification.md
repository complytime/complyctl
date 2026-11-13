# Verification Record

## Scope and revision

Results recorded on 2026-11-13 against working-tree implementation
changes based on `aa11eecd40691f347a150ab2dde536522e93c71a`.
Planning artifacts were reviewed by the maintainer, committed, and
pushed before implementation. See branch history for implementation
commits; these results distinguish fixtures from live acceptance.

## Completed checks

- `env -u GITHUB_TOKEN -u PROVIDERS_BIN_DIR TMPDIR=/tmp/opencode
  make test-cross-repo-evidence`: passed 64 evidence assertions,
  13 host/export assertions, and three generation-index fixtures.
  The host/export suite also injects a digest failure and verifies
  that the evidence Make dependency returns nonzero. Its recipe is
  checked not to invoke the aggregate target. On this non-root run
  no fixtures were skipped; privileged readers explicitly report
  the unreadable-file fixture as skipped, never passed.
- `make test-schema-validation-regression`: seven cases passed
  using the real pinned Gemara CUE schema. Coverage includes
  JSON-only, mixed JSON/YAML, invalid and malformed documents in
  both formats, and absent logs. These are schema fixtures, not
  actual credentialed scan output.
- `shellcheck tests/cross-repo/*.sh tests/schema-validation/*.sh`:
  passed.
- `actionlint .github/workflows/ci_cross_repo_integration.yml`:
  passed.
- `yamllint -c .yamllint.yml
  .github/workflows/ci_cross_repo_integration.yml`: passed.
- `git diff --check`: passed.
- `openspec validate resolve-cross-repo-evidence-mappings --strict`:
  passed.
- `make test-e2e`: passed, including its host/test-provider builds.
  These tests use the test provider, not live retained Ampel
  acceptance. Output warns that installed OPA lacks conftest.
- `make test-integration`: passed, 26 assertions. This uses the
  test provider and does not establish live retained acceptance.
- `env TZ=UTC make test-unit`: passed with race detection and
  coverage. The ordinary local-time run remains recorded below
  as failed; no CI setting or test assertion was changed.

The offline evidence checks cover returned legacy and retained
coordinates, encoded paths, integrity/linkage failures, parent-shell
counters, preserved first-log snapshots, identical-content distinct
instances, shared-use instances, and historical overwrite/removal.
Host fixtures exercise the same selection function as the canonical
harness. They do not prove released-host/provider interoperability.

Get, Generate, generation-index resolution, bad-policy rejection,
and the two OPA fixture invocations remain in the live harness.
Their live execution is not claimed by the offline fixture results.

## Failed check and pause

`make test-unit` failed in
`TestSyncPolicy_ContextCancellationPreservesExistingCacheAndState`
at `internal/cache/local_source_context_test.go:117`.

The failure compares `2026-11-13 09:03:31.527700778 +0000 UTC`
with `2026-11-13 10:03:31.527700778 +0100 CET`: the same instant
with different time locations. This change does not modify that
test or production cache code. No unrelated fix was made.

The maintainer authorized diagnosis and a UTC rerun:

- `env TZ=UTC go test -mod=vendor -race ./internal/cache -run
  '^TestSyncPolicy_ContextCancellationPreservesExistingCacheAndState$'
  -count=1 -v`: passed.
- The same targeted command with `TZ=Europe/Berlin` reproduced
  the failure. This is timezone-dependent, not evidence loss:
  `time.Now()` records local time, JSON preserves its numeric
  offset, and unmarshalling may use `time.Local`. The assertion
  uses structural equality after normalizing only the expected
  value to UTC; structural equality also compares the location.
- `git diff HEAD -- internal/cache/local_source_context_test.go
  internal/cache/state.go`: empty. The test was introduced in
  `64c2ef09c535ecfc499bf01c54e6c873e3e38f08` (PR #913).

The appropriate separate test fix is to compare instants using
`time.Time.Equal`, or normalize both operands before comparison.
No test assertion, production behavior, or CI environment has been
modified to hide the failure. Task 6.1's workflow-derived checks
and result recording are complete. This does not classify the
ordinary local-time run as passed or waive any quality gate.

## Live acceptance and review

The initial agent session lacked live prerequisites. After
installing conftest, the maintainer reported all local tests
successful and explicitly confirmed both unchanged v1.2.0 and
the two-evaluation `validate_evidence_history` check. These are
maintainer-reported results, not agent-executed acceptance runs.
Raw logs and snapshot paths were not supplied to the agent.

- Provider revision supplied by the maintainer:
  `f8dec3f9af9b6a5b23e2878e5f94e44e0c0ea965`.
- Current host/harness branch revision:
  `ef860cc8c2a7b1ba5e4ea87149c5d04560e21db5`.
- Unchanged released host: v1.2.0,
  `f10c1426f3674bbcb967a8aad8ca529b32e21212`.
- Task 6.2: passed, per maintainer confirmation of the canonical
  harness against both hosts with the selected provider build.
- Task 6.3: passed, per maintainer confirmation of two actual
  evaluations in one workspace, log snapshots, and successful
  `validate_evidence_history` revalidation. Offline fixtures are
  not the evidence for this task.
- Task 5.2: passed. In addition to the maintainer's local results,
  PR #975's Cross-Repo Integration Test job completed the real
  integration step and schema validation successfully:
  https://github.com/complytime/complyctl/actions/runs/43411704074/job/131839452255
- Task 6.4: strict OpenSpec validation passed. An initial audit
  identified recursive failure-injection risk; the injected child
  now runs only the evidence dependency under a 120-second timeout,
  with a recursion guard and distinct timeout diagnostic.
  Independent council review approved candidate `8f0b4943` for
  draft submission (details below). No implementation code changed
  after approval; only this record and task bookkeeping changed.
Task 5.2 had been reopened because inspection alone was not live
execution evidence. It is now closed on the actual CI execution
and maintainer-confirmed local results, not on that inspection.

## Council fixes and evidence

Independent reviewers requested per-assessment role pairing,
root-safe unreadability coverage reporting, explicit fixture-mode
documentation, centralized scan-path naming, actionable root/hash
diagnostics, and accurate task status. These fixes preserve the
strict artifact/reference/digest contract. A mapped-evidence
failure now records FAILED but still runs the other pipeline
checks and exports logs before the final nonzero exit.

The suggested v1.2.0 API incompatibility was disproved by
`git grep -n -e 'EvidenceMapping source' -e 'mapping_references'
v1.2.0 -- api/plugin`: `plugin.proto:117` already contains
`mapping_references = 3`, and `plugin.proto:214` contains
`EvidenceMapping source = 6`. This source evidence is not a
substitute for the maintainer-reported v1.2.0 integration run.

Council pre-flight passed, including `make lint` (zero issues).
The full aggregate pre-commit run modified 2223 unrelated files;
identical formatter drift reproduced in a detached main worktree.
Only those verified, agent-generated formatter edits were restored.
They are not included in this branch. No quality gate was changed.

All nine discovered Divisor personas ran as independent reviewer
sessions, reviewing the full branch diff: Adversary, Architect,
Guard, Tester, SRE, Curator, Envoy, Herald, and Scribe. An earlier
supervisor-only audit lacked independent contexts and was not
counted as the required council. After one fix loop, all nine
returned APPROVE in the second independent round, specifically
for draft submission. No blocking code findings remain.

Council approval alone did not establish live acceptance; the
subsequent maintainer confirmation above closes tasks 5.2, 6.2,
and 6.3. Remaining non-blocking advisories
include existing token-redaction hardening, reused export
destinations, schema-infrastructure diagnostics, and some test
diagnostic polish; no specified requirement was waived.

## Documentation impact

- `CHANGELOG.md`: records mapping-driven harness verification,
  host override, offline target, and JSON/YAML validation.
- `AGENTS.md`: documents the two added test commands and both
  EvaluationLog serializations. Governance rules are unchanged.
- `docs/TESTING_ENVIRONMENT.md`: describes prerequisites,
  helper diagnostics, default/external hosts, and the distinction
  between fixture checks and actual retention acceptance.
- `README.md`: unchanged; the product description and public CLI
  are unchanged, and its existing testing-environment link leads
  to the updated guidance.

## Tool versions

- Bash 5.3.9; jq 1.8.1.
- CUE v0.17.1, using the unchanged default
  `github.com/gemaraproj/gemara@v1.6.0` module.
- ShellCheck 0.11.0; actionlint 1.7.12; yamllint 1.38.0.
- Go `go1.26.10-X:nodwarf5 linux/amd64`.

Live tools observed in the local environment after the maintainer
confirmation: Snappy v0.2.6, Ampel v1.4.2, and Conftest built from
`github.com/open-policy-agent/conftest@v0.68.2` with OPA 1.15.2.
Conftest prints `dev` as its version; its Go build metadata
identifies the module version. These observations are not a
reconstruction of unsupplied test logs.
