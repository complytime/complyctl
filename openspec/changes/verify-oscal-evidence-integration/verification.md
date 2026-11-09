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

## Remaining Validation Failures

Task 2.1 remains unchecked. Acceptance-tagged golangci-lint reports
four findings reproduced against the unchanged baseline:

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
new acceptance helpers. The workflow is not recorded as fully green.
