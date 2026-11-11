# Verification Results

## Host Lifecycle Acceptance

The user supplied a passing acceptance-binary transcript on
2026-11-09 from macOS with Podman 6.1.4 and podman-compose 1.6.0.
The host checkout included the uncommitted acceptance-test changes.

The passing command was:

```bash
pc run --rm --no-deps sut 2>&1 | tee /tmp/complyctl-acceptance.log
```

Here `pc` selected the repository's lifecycle Compose profile with a
temporary Linux/AMD64 platform override. The CLI, test provider, and
acceptance binary were cross-compiled for Linux/AMD64 with CGO off.
The run used the existing zot registry and seeded policy fixtures.

The initial aggregate Podman run could not seed the registry because
the pinned AMD64 ORAS 1.2.2 binary crashed in Go's networking runtime
under emulation. The successful staged run followed instructions for
seeding with native Linux/ARM64 ORAS 1.2.2. This is host behavioral
verification, not a claim that Docker CI orchestration succeeded.

Results from the supplied transcript:

- `TestAcceptance_OCILifecycle`: PASS (2.92 seconds).
- `TestAcceptance_FormatInterop`: PASS (2.32 seconds).
- Eight evidence/schema helper top-level tests: PASS.
- All 42 helper subtests: PASS.
- Eight verification-profile tests: expected SKIP in lifecycle mode.
- Final acceptance-binary result: PASS; zero test failures.

The format-interoperability test verifies the actual generated scan
report, including evidence description, source reference ID,
coordinate, digest, finding linkage, and complete-document schema
validation. No assertions were disabled for the host run.

## Devcontainer Verification

The following commands passed with the implementation changes:

- `make lint`: zero issues in the default build-tag scope.
- `git diff --check`: no whitespace errors.
- `make test-unit`: 955 top-level tests, 1,131 including subtests;
  12 packages passed, 11 package results cached.
- `go test ./internal/output -run TestToOSCAL -count=1`: PASS.
- `make test-e2e`: 18 top-level tests and 23 subtests passed.
- `make test-integration`: PASS; summary reports 26 passed, zero
  failed. Three additional file checks run in subshells and do not
  increment the parent shell's summary counter.

Focused evidence/schema tests also passed locally with the race
detector. The devcontainer has no Docker or Podman runtime, so its
exact container acceptance invocation remains unavailable.

## Historical Validation Failures (Before Upstream Integration)

Before upstream integration, task 2.1 remained unchecked because
acceptance-tagged golangci-lint reported four findings reproduced
against the unchanged baseline:

- Two G704 findings on registry requests derived from test environment
  variables in `tests/acceptance/acceptance_test.go`.
- One G306 finding on a configuration write in
  `tests/acceptance/helpers_test.go`.
- One G306 finding on a configuration write in
  `tests/acceptance/verification_helpers_test.go`.

No findings were reported in the new evidence/schema helper files.
Go 1.26.10 matches `go.mod`. The initial scanner was golangci-lint
2.11.4. A follow-up used golangci-lint 2.13.2, matching the version
reported by GitHub CI. With that version:

- `golangci-lint run -c .golangci.yml`: PASS, zero issues.
- Acceptance-tagged lint on this branch: the same four findings.
- Acceptance-tagged lint in a clean baseline worktree at
  `f10c1426f3674bbcb967a8aad8ca529b32e21212`: the same four findings.

The baseline worktree was removed after verification. No suppressions
or lint configuration changes were made.

The inspected baseline and latest `main` MegaLinter jobs pass Go and
YAML linting, but fail on three unrelated Lychee documentation links.
CI's logged Go command does not enable the acceptance build tag.
Its YAML invocation uses a filtered file list rather than
`yamllint .`; its standardized workflow does not run pre-commit.
These differences explain why the checks below are not equivalent
to the passing Go/YAML CI checks.

CI evidence inspected on 2026-11-09:

- [Baseline linter job](https://github.com/complytime/complyctl/actions/runs/42184062642/job/127780104722).
- [Latest inspected main linter job](https://github.com/complytime/complyctl/actions/runs/42601889576/job/129108133089).

Repository-wide YAML and pre-commit checks also report baseline
issues in unchanged files; the YAML run includes an ignored generated
governance report. These results do not indicate a regression in the
new acceptance helpers. That run was not recorded as fully green.

## Integrated-Tree Verification (2026-11-11)

Upstream `main` at `c9e64bd7d555f8f6a0f222439405391be6ab468e`,
including PR #971, merged cleanly into this feature branch with
`git merge --no-commit --no-ff upstream/main`. These checks ran while
HEAD was `b83afb8b2a2add08e5369e6a6d6695e3cf817c41`, before the merge
was committed as `661c7caf1620bee780f2cd3744734d2d7d5dd9cd`.
The combined tree retains the evidence/schema assertions and
registry isolation, plus upstream's registry-request annotations,
owner-only config writes, and config-readability regression tests.

Go 1.26.10 matches `go.mod`. Lint used golangci-lint 2.13.2 from
`/tmp/opencode/golangci-v2.13.2`, matching the inspected CI version.
The following checks passed on the combined working tree:

- `make lint`: zero issues with that binary first on `PATH`.
- `golangci-lint run --build-tags=acceptance ./tests/acceptance/...`:
  zero issues. The four historical acceptance findings are resolved.
- `git diff --check` and `git diff --cached --check`: PASS.
- Focused acceptance tests with `-race -count=1`: all eight OSCAL
  evidence/schema tests, their 42 subtests, and both upstream
  owner-only/readability config tests passed. The selector included
  `TestOSCALEvidence`, `TestOSCALSchema`,
  `TestWriteWorkspaceConfig_OwnerOnlyReadable`, and
  `TestWriteVerificationConfig_OwnerOnlyReadable`.
- `make test-unit`: 955 top-level tests and 176 subtests passed
  across 12 packages; 11 package results were cached.
- `go test ./internal/output -run TestToOSCAL -count=1`: PASS.
- `make test-e2e`: 18 top-level tests and 23 subtests passed.
- `make test-integration`: PASS.
- `yamllint .` and `buf lint`: PASS.
- `openspec validate verify-oscal-evidence-integration --strict`:
  PASS.

Task 2.1 is now complete. Test logs are under `/tmp/opencode/`, with
the prefix `oscal-integrated-` and suffix `-20261111.log`.
Documentation impact remains test-only: no additional changelog,
README, or AGENTS update is needed for this follow-up.

### Separate Baseline and Environment Results

Pre-commit ran in disposable merged-index and upstream copies to
protect the active checkout from hooks that rewrite vendor files.
Both copies reproduced the same baseline whitespace, EOF, SPDX, and
vendor Go-formatting failures tracked separately in issue #972.
Large-file failures were identical in both copies and reflect all
files being newly staged in commit-free disposable repositories.
The Go-test hook exhausted local disk space in both copies; this is
an environment failure, not a test result. The direct unit suite
above passed. No hook edits were copied back to the active tree.
Logs and integrity evidence are in
`/tmp/opencode/merged-precommit-20261111-ksusbi1w/`.

Current upstream CI at `c9e64bd7` passes unit, E2E, integration,
acceptance lifecycle/verification, Buf, API compatibility, and
cross-repo checks. Its standardized linter job fails on baseline
Markdown formatting and link checks; SonarCloud also reports a
failure. These are upstream results, not CI results for the pending
merge. The linter evidence is run `43017059157`.

Gaze's quality and classification commands remain unavailable
because its binary was built with Go 1.25. Its default untagged CRAP
run does not analyze the changed acceptance helpers, so no helper
coverage or GazeCRAP claim is made.

The devcontainer still has no Docker or Podman runtime. Historical
host lifecycle verification above remains valid for task 1.3. The
preserved root acceptance logs show passing dirty-tree builds at
commit `e4053766` on 2026-11-10; they do not establish the identity
of this combined tree and are not claimed as a fresh container run.
Repository-wide pre-commit is not recorded as fully green.

## PR Review Follow-up (2026-11-11)

Review of PR #973 at `661c7caf` identified a first-match defect:
when two linked observations had the expected requirement ID,
only the first link's evidence was checked. The evidence-less link
coming first caused a schema-valid report to fail FR-002.

The new regression uses a complete schema-valid report and tests
both link orders and both observation-array orders. Before the fix,
the two evidence-less-link-first cases failed as expected. A
separate regression also showed that a valid first link could hide
a later dangling UUID from the helper.

The helper now validates all links before returning the matching
observations, then accepts complete evidence on any one of them.
Negative permutations verify that absent evidence and provenance
split across observations still fail. No fields are combined across
evidence items or observations, and no production converter code
changes are needed.

Local checks passed with Go 1.26.10:

- Focused acceptance tests with `-race -count=1`: 11 top-level tests
  and 55 subtests, including all 12 schema-valid permutations.
- Default and acceptance-tagged golangci-lint 2.13.2: zero issues.
- `make test-unit`: 955 top-level tests passed.
- `go test ./internal/output -run TestToOSCAL -count=1`: PASS.
- Markdown lint 0.49.1 using `.markdownlint.json`: PASS for the
  proposal and verification record; the bare-URL finding is fixed.
- Strict OpenSpec validation and whitespace checks: PASS.

The setup-helper comments now document HTTP preflight and fatal
test failures. Documentation impact remains limited to test helpers
and the existing change artifacts; no README, AGENTS, or changelog
update is needed.

The user supplied a fresh passing full Podman lifecycle transcript
for the preceding `661c7caf` checkout using Go 1.26.10 with
`GOOS=linux GOARCH=amd64 CGO_ENABLED=0`. That run included both live
acceptance tests, 10 helper/config tests, and 42 subtests, with the
eight verification-profile tests skipped as expected. Both host
lint scopes also passed with golangci-lint 2.12.2. These results
precede this follow-up and are not a new run of the revised helper.

The user then supplied successful fresh Podman lifecycle output for
the follow-up. Both live acceptance tests, 11 helper/config tests,
and 55 subtests passed, including all 12 repeated-requirement
permutations and the dangling-link-after-match regression. The
eight verification-profile tests skipped as expected, and the test
binary ended with `PASS`. This is user-executed host evidence;
this devcontainer still has no Docker or Podman runtime.

The refreshed project-local council reviewed all 14 feature files
and returned six APPROVE verdicts with zero findings. The source
tree was unchanged between that review and receipt of the passing
host result; only this verification record and task bookkeeping
were updated afterward. Task 3.3 is complete. Issue #972 remains
separate; aggregate fix hooks were not run in the active checkout.
