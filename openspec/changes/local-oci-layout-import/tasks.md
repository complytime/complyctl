# Tasks

## 1. LocalPolicySource

- [x] 1.1 Create `internal/cache/local_source.go` with `LocalPolicySource`
  struct implementing `PolicySource`. Include SPDX header and GoDoc on all
  exported identifiers. Verify: `go build ./internal/cache/...` succeeds.

- [x] 1.2 Implement `LocalPolicySource.DefinitionVersion`: open the layout with
  `oci.NewFromFS`, resolve only the configured OCI digest from `index.json`,
  and return the manifest digest and version. Verify: unit tests in
  `internal/cache/local_source_test.go` cover digest found, digest absent, and
  tag-only reference rejected; `make test-unit` passes.

- [x] 1.3 Implement `LocalPolicySource.CopyPolicy`: call `oras.Copy` from the
  read-only local store into the destination OCI store. Verify: unit test
  confirms blobs are present in the destination store after copy;
  `make test-unit` passes.

- [x] 1.4 Add content integrity check: after resolving the manifest descriptor,
  verify the computed digest matches before proceeding. Verify: unit test with
  a tampered manifest returns an error and leaves the destination store
  unchanged; `make test-unit` passes.

- [x] 1.5 Propagate the `get --timeout` context through local-layout reads and
  copies so cancellation stops local I/O. Verify: a blocking filesystem/store
  test cancels its context, returns the deadline error promptly, and leaves no
  partial destination content; `make test-unit` passes.

## 2. LocalComplypackSource

- [x] 2.1 Add `LocalComplypackSource` struct to `internal/cache/local_source.go`
  (or a separate `local_complypack_source.go`) implementing `ComplypackSource`.
  Include GoDoc. Verify: `go build ./internal/cache/...` succeeds.

- [x] 2.2 Implement `LocalComplypackSource.DefinitionVersion` using the same
  digest-only layout resolution logic as `LocalPolicySource.DefinitionVersion`.
  Verify: unit tests cover digest found, digest absent, and tag-only reference
  rejected; `make test-unit` passes.

- [x] 2.3 Implement `LocalComplypackSource.CopyComplypack`: resolve and copy
  from the local layout, then delegate to the existing media-type validation
  helper. Verify: unit test confirms invalid media type returns an error and
  does not copy content; valid artifact copies successfully; `make test-unit`
  passes.

- [x] 2.4 Validate complypack manifest/content integrity before cache or state
  updates: reject a descriptor whose declared content digest differs from the
  copied content. Verify: a layout with mismatched complypack manifest and
  content returns an error and leaves both cache and state unchanged;
  `make test-unit` passes.

## 3. CLI flag and source construction

- [x] 3.1 Add `--from-oci-layout <path>` flag to `complyctl get` in
  `cmd/complyctl/cli/get.go`. Register alongside `--timeout` and
  `--skip-verify`. Verify: `complyctl get --help` shows the new flag.

- [x] 3.2 Add preflight validation before any sync: when `--from-oci-layout` is
  set, check that the path exists and contains a valid `oci-layout` marker.
  Return an actionable error if either check fails. Verify: running
  `complyctl get --from-oci-layout /nonexistent` returns a non-zero exit code
  with a clear error message; `make test-unit` covers both cases.

- [x] 3.3 Add entry-local verification fail-closed handling: when a local-mode
  entry has configured verification and `--skip-verify` is absent, fail that
  entry from configuration before verifier construction, trusted-root loading,
  TUF startup, registry-client creation, credential resolution, cache, or state
  changes; continue other entries. Verify: injected observable guards record
  zero calls to each initializer for rejected policy and complypack entries;
  a mixed workspace imports an eligible entry while the rejected entry fails;
  `make test-unit` passes.

- [x] 3.4 Branch `syncAllPolicies` and `syncAllComplypacks` (or equivalent
  helpers) on the flag: select `LocalPolicySource` / `LocalComplypackSource`
  before registry-client creation or credential resolution. Verify: tests prove
  local mode calls neither factory for policies or complypacks; online tests
  remain unaffected.

- [x] 3.5 Extend local-layout preflight validation to require a readable
  `index.json` in addition to the `oci-layout` marker before any sync starts.
  Verify: a layout with the marker but no `index.json` exits non-zero with an
  actionable error and leaves all configured cache and state entries unchanged;
  `make test-unit` passes.

## 4. Integration test

- [x] 4.1 Add an E2E or integration test that seeds a local OCI layout with a
  test policy using `oras push --oci-layout` (or equivalent in-process setup),
  then runs `complyctl get --from-oci-layout <path>` and asserts that:
  - the policy cache directory is populated,
  - `state.json` contains the correct digest and version,
  - `complyctl list` shows the policy,
  - `Verified` is `false`.
  Verify: `make test-unit` (or `make test-e2e` if gated) passes.

- [x] 4.2 Add a test that confirms `complyctl scan` succeeds after
  `complyctl get --from-oci-layout` with providers pre-installed. Verify: test
  passes; no registry access is attempted.

- [x] 4.3 Add a test covering complypack import: seed a local layout with a
  test complypack artifact, run `complyctl get --from-oci-layout`, and assert
  the complypack cache and state are populated correctly. Verify: test passes.

- [x] 4.4 Add integration coverage for digest binding: a digest-pinned policy
  and complypack import from the layout, while tag-only configured URLs fail
  entry-locally without cache/state changes. Verify: `make test-unit` (or
  `make test-e2e` if gated) passes.

- [x] 4.5 Add local-import incremental behavior coverage: reimporting an
  unchanged digest skips replacement, while a changed digest replaces the
  cached content and state metadata. Verify: tests assert cache paths and
  `state.json` are unchanged after the skip, then contain only the replacement
  digest/version after the update; `make test-unit` (or `make test-e2e` if
  gated) passes.

## 5. Documentation

- [x] 5.1 Update `docs/QUICK_START.md` offline-workflow section to document
  `complyctl get --from-oci-layout` as the primary offline provisioning path.
  Retain the manual `oras push` workaround as a note but clarify it is no
  longer the only option. State that local URLs require OCI digests and that
  configured verification requires `--skip-verify` per entry. Explain that
  entries may succeed independently, each failure is reported, the command
  exits non-zero when any entry fails, successful imports appear in
  `complyctl list`, and rerunning the same command safely imports only failed
  or changed entries. Verify: review the section against all listed behavior,
  confirm its command examples are copy-pasteable, and run `make lint`.

- [x] 5.2 Update `AGENTS.md` — Recent Changes section — with a brief entry
  describing the new flag, digest-only local selection, and entry-local
  verification fail-closed semantics. Verify: entry follows the established
  format.

## Workflow follow-up

- After implementation review passes, open the linked follow-up issue for
  fully offline signature verification (keyed first, then keyless with
  `trusted_root` from #768).
- After that issue is resolved, revisit whether the `complyctl doctor` check
  for unverified locally-imported entries is warranted.
- Whole-workspace export/import (`complyctl export` + `complyctl get
  --from-oci-layout` without `--policy-id`) is a separate follow-on issue.

<!-- spec-review: passed -->

<!-- code-review: passed -->
