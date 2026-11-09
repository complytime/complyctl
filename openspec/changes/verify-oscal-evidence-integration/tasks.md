# Tasks

## 1. Evidence Acceptance Coverage

- [ ] 1.1 Add focused report-inspection helpers and negative cases
  under `tests/acceptance/` for FR-001 and FR-002. Verify that absent,
  empty, and mismatched evidence, missing finding links, and evidence
  on an unrelated assessment return errors while a valid case passes;
  run `go test -tags=acceptance ./tests/acceptance -run TestOSCALEvidence`.
- [ ] 1.2 Add complete-document OSCAL 1.1.3 schema validation in the
  acceptance helpers for FR-003, reusing go-oscal. Add valid and
  schema-invalid evidence cases; verify with
  `go test -tags=acceptance ./tests/acceptance -run TestOSCALSchema`.
- [ ] 1.3 Extend `TestAcceptance_FormatInterop` to invoke the evidence,
  linkage, and complete-document schema checks on actual scan output.
  Verify the expected evidence description and source reference ID,
  coordinate, and digest against the existing provider fixture; run
  `make test-acceptance COMPOSE="docker compose"` and confirm both the
  full workflow and the isolated negative cases pass (FR-001–FR-003).
- [ ] 1.4 Assess documentation impact for `CHANGELOG.md`, `AGENTS.md`,
  and `README.md`; record the assessment in the implementation summary
  and update only documents affected by the delivered test changes.
  Verify any edits accurately describe coverage without claiming a
  new production feature or report-contract change.

## 2. Integration Verification

- [ ] 2.1 Run `make lint` and
  `golangci-lint run --build-tags=acceptance ./tests/acceptance/...`;
  verify both pass and `git diff --check` reports no whitespace errors.
- [ ] 2.2 Run `make test-unit` and
  `go test ./internal/output -run TestToOSCAL -count=1`; verify the
  existing formatter and unit suites pass alongside the new coverage.
- [ ] 2.3 Run `make test-e2e` and `make test-integration` using the
  existing CI targets; verify the scan workflows pass and the final
  diff contains only the planned acceptance coverage, artifacts, and
  any justified documentation updates.

## Workflow follow-up

- Review and commit/push the spec artifacts before implementation,
  following the repository's spec-first requirement.
- Run `/uf.review-council` before PR submission and resolve any
  REQUEST CHANGES verdict.
- Archive the change after implementation and review are complete.
