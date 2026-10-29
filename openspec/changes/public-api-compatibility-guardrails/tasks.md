# Tasks

## 1. Compatibility baseline and analyzers

- [x] 1.1 Implement a strict shared baseline helper that selects the highest stable same-major tag, resolves its commit, materializes an isolated worktree, and reports the tag and commit; verify tag filtering, stable selection, missing-baseline failure, and worktree cleanup with automated tests.
- [x] 1.2 Add pinned Buf breaking and Go API analyzer commands that consume the shared baseline and fail closed on incompatible contracts, tool setup, package loading, and analyzer errors; verify compatible and incompatible protobuf and `pkg/provider` fixtures.
- [x] 1.3 Add strict Makefile targets for local protobuf, Go API, and combined compatibility analysis; verify each target returns non-zero when its tool or baseline is unavailable.
- [x] 1.4 Document the local commands and static-analysis limits, including unkeyed literals, struct comparability, and the complytime-providers#187 downstream validation handoff; verify documented commands and links are accurate.

## 2. Pull-request and release integration

- [x] 2.1 Add a read-only, SHA-pinned `api-compatibility` CI job that runs the combined strict target and publishes its baseline and diagnostics; verify it runs for pull requests and supported release-line pushes.
- [x] 2.2 Register the `api-compatibility` job ID in release preflight's explicit checks and verify the workflow configuration references it once.
- [x] 2.3 Update CI and contributor guidance for the compatibility job, pinned tooling, and merge/release evidence; verify the affected workflow documentation reflects the delivered job name.

## 3. Future major-version policy guidance

- [x] 3.1 Document that the current major has no breaking-change override and that incompatible APIs remain blocked on `main`.
- [x] 3.2 Retain future authorization requirements as policy guidance without creating a next-major branch, GitHub Environment, or exception job.
- [x] 3.3 Record that a future authorization workflow must preserve diagnostics, bind approval to the reviewed commit, collect consumer impact and migration steps, and coordinate complytime-providers#187.
- [x] 3.4 Verify baseline-resolution, tool-installation, package-loading, analyzer, and incompatible API failures all fail the current `api-compatibility` job.

## 4. Integration verification

- [x] 4.1 Run the compatibility targets against unchanged APIs and controlled incompatible Go and protobuf changes; verify pass and fail behavior matches `public-api-compatibility` scenarios.
- [x] 4.2 Run CI-parity checks required by the changed workflows and record locally verified configuration. Remote validation remains a workflow follow-up authorized by the maintainer.

Local CI-parity verification completed on 2026-10-28: Buf 1.50.0 and
`apidiff` v0.0.0-20251023183803-a4bb9ffd2546 passed through
`make api-compatibility`; `yamllint`, lint, vet, unit, build, E2E, and
integration checks also passed. GitHub check and preflight behavior remain
post-push verification.

## 5. Review feedback: local Buf minimum version

- [ ] 5.1 Commit and push the revised proposal, design, requirements, and
  tasks before changing analyzer source or tests.
- [ ] 5.2 Replace local Buf equality validation with validated semantic
  version comparison accepting stable versions at or above `1.50.0`.
  Report older versions as `buf >= 1.50.0 required, found <version>`;
  retain fail-closed tool-setup and analyzer errors.
- [ ] 5.3 Add regression coverage for the minimum, newer patch/minor/major
  versions, numeric ordering, valid build metadata, older versions,
  malformed versions, prereleases, missing tools, and version-command
  failures. Verify accepted versions still run analysis and propagate
  incompatibility and operational failures.
- [ ] 5.4 Update contributor and release guidance plus the changelog to
  distinguish local Buf minimum-version support from pinned CI execution;
  preserve the existing exact `apidiff` revision.
- [ ] 5.5 Run the compatibility regression suites and applicable CI-parity
  checks. Confirm CI still installs Buf `1.50.0`, apidiff remains pinned,
  and required compatibility checks remain enforced.

## Workflow follow-up

- Run `/uf.review-council` and resolve any request-for-changes verdict before PR submission.
- After push, confirm GitHub emits the expected `api-compatibility` check and a failed run blocks release preflight.
- When a future major migration is approved, propose and review a separate protected authorization workflow before enabling any compatibility exception.
- Archive the change after implementation, review, and repository acceptance requirements are satisfied.

<!--
Unleash execution checklist
- [x] Step 0: Startup Cleanup
- [x] Step 1: Branch Safety Gate
- [x] Step 2: Resumability Detection
- [x] Step 3: Clarify (Step 1)
- [x] Step 4: Plan (Step 2)
- [x] Step 5: Tasks (Step 3)
- [x] Step 6: Spec Review (Step 4) -- iteration: 1/3
- [x] Step 7: Implement (Step 5) -- phase: 4/4, batch: 0/0, workers: 0/0
- [x] Step 8: Code Review (Step 6) -- iteration: 3/3
- [ ] Step 9: Retrospective (Step 7)
- [ ] Step 10: Demo (Step 8)
-->

<!-- spec-review: passed -->

<!-- code-review: passed -->
