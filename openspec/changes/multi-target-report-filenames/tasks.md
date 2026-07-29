<!-- Tasks marked [P] can run in parallel within their section -->

## 1. Shared Helper

- [x] 1.1 Create `internal/output/filename.go` with `BuildReportFilename(prefix, policyID, targetID, ext string) string` using `complytime.FilenameSafe` for sanitization and `time.Now().Format("20060102-150405")` for timestamp; omit `targetID` segment when empty
- [x] 1.2 Create `internal/output/filename_test.go` with tests: standard multi-segment output, slash sanitization in policyID and targetID, compound extensions (`sarif.json`), empty targetID omits segment without double dashes, `evaluation-log` prefix with `yaml` extension

## 2. Formatter Migration

- [x] 2.1 Refactor `internal/output/evaluator.go:155` to call `BuildReportFilename("evaluation-log", e.policyID, e.targetID, "yaml")` replacing inline `fmt.Sprintf`
- [x] 2.2 Refactor `internal/output/oscal.go:95` to call `BuildReportFilename("assessment-results", policyID, log.Target.Id, "json")` replacing inline `fmt.Sprintf`
- [x] 2.3 Refactor `internal/output/sarif.go:33` to call `BuildReportFilename("scan", policyID, log.Target.Id, "sarif.json")` replacing inline `fmt.Sprintf`
- [x] 2.4 Refactor `internal/output/markdown.go:146` to call `BuildReportFilename("report", m.policyID, m.evalLog.Target.Id, "md")` replacing inline `fmt.Sprintf`

## 3. Test Updates

- [x] 3.1 Update `internal/output/oscal_test.go`: add `Target` field to `mockGemaraEvalLog()` and verify `TestToOSCAL_OutputFileNaming` asserts targetID in output filename
- [x] 3.2 Update `internal/output/sarif_test.go`: add `Target` field to `mockGemaraEvalLog()` and verify `TestToSARIF_OutputFileNaming` asserts targetID in output filename
- [x] 3.3 Update `internal/output/markdown_test.go`: add `Target` field to test fixture and verify `Write` output filename contains targetID
- [x] 3.4 Add `TestEvaluatorWrite_OutputFileNaming` in `internal/output/evaluator_test.go` asserting the filename contains both policyID and targetID segments, matching the `BuildReportFilename` pattern

## 4. Regression Test

- [x] 4.1 Add `TestMultiTargetDistinctFilenames` in `internal/output/filename_test.go` or a formatter test file: call at least one formatter (e.g., `ToOSCAL`) with two different `Target.Id` values and assert two distinct files are produced in the output directory — reproduces #773

## 5. Documentation

- [x] 5.1 Update `CHANGELOG.md` with bug-fix entry for #773
- [x] 5.2 Update `AGENTS.md` Recent Changes section

## 6. Verification

- [x] 6.1 Run `make test-unit` — all tests pass
- [x] 6.2 Run `make lint` — no new warnings
- [x] 6.3 Run `make vet` — no issues
- [ ] 6.4 Run `make sanity` — vendor + format + vet + git diff clean
- [x] 6.5 Run `make crapload-check` — no CRAP regressions
