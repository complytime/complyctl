# Tasks

## 1. Delegate OSCAL Model Conversion

- [ ] 1.1 Replace local OSCAL model construction in `internal/output/oscal.go` with `gemaraconv.EvaluationLogToOSCALAssessmentResults`, passing the policy-specific assessment-plan reference; verify `go test ./internal/output` passes.
- [ ] 1.2 Retain the `ToOSCAL` serialization, directory creation, `0600` write mode, and policy/target report filename behavior; verify the existing filename and output-write tests pass.
- [ ] 1.3 Remove obsolete local result-mapping and UUID helpers and the OSCAL FIXME; verify `go test ./internal/output` and `go vet ./internal/output` pass.
- [ ] 1.4 Assess `CHANGELOG.md`, `README.md`, and `AGENTS.md` for the documented OSCAL structure change, updating only affected documentation; verify every edited Markdown file passes its project linting or review requirements.

## 2. Cover Delegated OSCAL Output

- [ ] 2.1 Update OSCAL result assertions for the pinned Gemara converter's status mappings and control-level finding structure; verify `go test ./internal/output -run TestToOSCAL` passes.
- [ ] 2.2 Add a regression test with evidence and populated source provenance that verifies `relevant-evidence` in the serialized observation; verify the test fails against the former local conversion and passes after delegation.
- [ ] 2.3 Add a regression test that verifies a control finding links to every observation created for its assessment logs; verify `go test ./internal/output -run TestToOSCAL` passes.
- [ ] 2.4 Retain OSCAL schema validation coverage for evidence-bearing and evidence-free reports; verify `go test ./internal/output` passes.

## 3. Validate the Change

- [ ] 3.1 Run `make format` and verify it leaves no formatting changes.
- [ ] 3.2 Run `make test-unit` and verify the CI-equivalent unit-test suite passes.

## Workflow follow-up

- Run `/uf.review-council` before opening a pull request.
- Archive the change after implementation and review requirements are satisfied.
