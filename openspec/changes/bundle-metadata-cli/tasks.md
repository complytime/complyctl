<!--
  [P] marks tasks eligible for parallel execution.
  Add [P] when a task: (a) touches different files from
  other [P] tasks in the group, (b) has no dependency
  on prior tasks in the group, (c) can safely execute
  without ordering constraints.
  Do NOT add [P] when tasks modify the same file --
  parallel workers will cause merge conflicts.
  Tasks without [P] run sequentially first, then [P]
  tasks run in parallel.
-->

## 1. Extend PolicyState with metadata fields

- [x] 1.1 Add `PolicyTitle string`, `PolicyEvaluator string`,
  `ControlCount int`, `AssessmentCount int` fields to
  `PolicyState` in `internal/cache/state.go` with
  `json:"...,omitempty"` tags
- [x] 1.2 Add `SetPolicyMetadata(repository string, title,
  evaluator string, controls, assessments int)` method to
  `State` that reads the existing `PolicyState` entry for
  the given repository key and updates only the four
  metadata fields. MUST NOT overwrite sync fields
  (`Version`, `Digest`, `Verified`, etc.). If the key does
  not exist in the `Policies` map, no-op (return without
  modification)
- [x] 1.3 Add unit tests for: (a) `PolicyState` JSON
  round-trip with new fields including backward
  compatibility test with old JSON (missing fields default
  to zero values), (b) `SetPolicyMetadata()` preserves all
  existing sync fields (`Version`, `Digest`, `Verified`,
  `SignerIdentity`, `Issuer`, `VerifiedAt`, `LastUpdated`)
  when updating metadata on a pre-existing entry (table-
  driven test with fully-populated `PolicyState`), (c)
  `SetPolicyMetadata()` no-ops when key does not exist

## 2. Add ExtractPolicyMetadata to Resolver

- [x] 2.1 Add `PolicyMetadata` struct to
  `internal/policy/resolver.go` with `Title`, `EvaluatorID`,
  `ControlCount`, `AssessmentCount` fields. Note:
  `EvaluatorID` here maps to `PolicyState.PolicyEvaluator`
  when persisted (see design D6)
- [x] 2.2 Implement `ExtractPolicyMetadata(policyID, version)`
  method on `Resolver` that detects manifest shape and
  extracts metadata from policy + catalog layers. The
  `policyID` parameter is the OCI repository path (cache
  key), following the same convention as
  `ResolvePolicyGraph()`
- [x] 2.3 Extend `parsePolicyLayer()` return or add a helper
  to extract `Title` from `gemara.Policy` alongside
  existing `EvaluatorID` and `Assessments`. The title
  extraction MUST be tested (assert specific title value
  from fixture, not just non-empty)
- [x] 2.4 Add unit tests for `ExtractPolicyMetadata()`
  covering: bundle-format, split-layer, missing catalog
  (`ControlCount` = 0), multi-evaluator policy
  (`EvaluatorID` = ""), empty title, and parse error
  scenarios. Use the existing `mockPolicyLoader` pattern
  from `internal/policy/resolver_test.go` (not
  `MockBundlePolicySource` which implements `PolicySource`,
  not `PolicyLoader`)

## 3. Integrate metadata extraction into get command

- [x] 3.1 Create a `policy.Resolver` once in
  `syncAllPolicies()` (via `policy.NewLoader(cacheMgr)` +
  `policy.NewResolver(loader)`) and pass it to
  `syncSinglePolicy()` as a new parameter. This follows
  the existing pattern in `scan.go`
- [x] 3.2 In `syncSinglePolicy()` in
  `cmd/complyctl/cli/get.go`, after `SyncPolicy()` returns,
  call `ExtractPolicyMetadata()` when either: (a) `fetched
  == true` (fresh fetch), or (b) the existing `PolicyState`
  has empty `PolicyEvaluator` (upgrade backfill). Call
  `state.SetPolicyMetadata()` then `cache.SaveState()` to
  persist. The call ordering is: `SyncPolicy()` ->
  `ExtractPolicyMetadata()` -> `SetPolicyMetadata()` ->
  `SaveState()` (see design D7)
- [x] 3.3 Print post-sync summary to stderr after metadata
  extraction succeeds: `  Policy: <title> (<evaluator>)`
  and `  Controls: <N> | Assessments: <M>`. Omit title
  portion when empty. Omit evaluator parenthetical when
  empty (multi-evaluator). Print only for fresh fetch or
  backfill, not for no-change syncs with existing metadata
- [x] 3.4 Handle metadata extraction errors gracefully: log
  warning, skip summary, do not fail the sync
- [x] 3.5 Add unit tests for: (a) get summary output on
  stderr (capture stderr, assert it contains
  `"Policy: <title> (<evaluator>)"` and
  `"Controls: <N> | Assessments: <M>"` after `"done"`),
  (b) graceful error handling (assert warning logged and
  no summary lines), (c) no-change sync with existing
  metadata (assert no summary lines), (d) re-fetch with
  new content (seed state with old metadata, simulate
  fresh fetch, assert metadata updated to new values)

## 4. Enrich list command with metadata columns

- [x] 4.1 Update `listOptions.run()` in
  `cmd/complyctl/cli/list.go` to read `PolicyEvaluator`
  and `ControlCount` from `PolicyState`
- [x] 4.2 Update `printGemaraPolicyTable()` headers to:
  `POLICY ID | VERSION | EVALUATOR | CONTROLS | DIGEST |
  VERIFIED`
- [x] 4.3 Show "-" when metadata fields are empty
  (pre-existing cache without metadata) or zero
  (`ControlCount` of 0 shows "0", but empty
  `PolicyEvaluator` shows "-")
- [x] 4.4 Update existing list tests to verify new column
  output. Include: (a) column order assertion (verify
  "EVALUATOR" appears after "VERSION" and before
  "CONTROLS" in header), (b) metadata values display
  correctly, (c) "-" shown for missing metadata, (d)
  `--policy-id` filter with metadata (multiple policies
  in config, only matching policy appears with correct
  EVALUATOR and CONTROLS values)

## 5. Verification and documentation

- [x] 5.1 Run `make test-unit` and verify all tests pass
- [x] 5.2 Run `make lint` and fix any issues
- [x] 5.3 Run `make vet` and fix any issues
- [x] 5.4 Run `make sanity` to ensure full CI parity
- [x] 5.5 Update `CHANGELOG.md` with entries for enriched
  `list` output and `get` post-sync summary
- [x] 5.6 Update `AGENTS.md` Recent Changes section with
  bundle-metadata-cli entry
<!-- spec-review: passed -->
<!-- code-review: passed -->
