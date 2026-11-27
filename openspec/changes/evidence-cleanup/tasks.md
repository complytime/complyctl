# Tasks

Implementation MUST NOT begin until the user has reviewed these
artifacts and the specification has been committed and published as
required by repository policy. Creating this checklist does not
authorize committing, pushing, or starting implementation.

## 1. Workspace coordination and interrupted activity

- [x] 1.1 Implement canonical workspace-directory locking with Linux
  and Darwin backends and filesystem refusal in
  `internal/workspacecoord/`; verify shared/exclusive contention, alias
  paths, independent workspaces, unsupported filesystems, and read-only
  acquisition on both operating systems with subprocess tests, without
  creating a lock file (FR-007, FR-008, FR-010).
- [x] 1.2 Implement uniquely owned activity records persisted before
  provider startup and removed only after confirmed shutdown; verify
  normal errors, record-write failures, malformed records, failed
  shutdown, and a killed host leave the required safe state, using
  barriers and subprocesses rather than sleeps (FR-009).
- [x] 1.3 Integrate access into scan, generate, relevant get artifact
  removal, and init; verify contention fails before conflicting work,
  nested generation reuses access, and scan holds protection through
  report publication and provider cleanup with existing unchanged
  provider clients (FR-008, FR-009, FR-010).
- [x] 1.4 Document coordination support, crash refusal, and platform
  limitations in `docs/EVIDENCE.md`, requiring Linux and macOS cleanup
  and distinguishing OS support from supported local filesystems;
  verify claims against release configuration and lifecycle tests on
  both operating systems (FR-009, FR-010).
- [x] 1.5 Add exact-record diagnostics and manual interrupted-scan
  recovery guidance to errors and `docs/EVIDENCE.md`; verify messages
  require stopping all writers and pausing new scans, explain complete
  environment restart if needed, and do not recommend automatic PID
  killing. In subprocess tests, stop controlled writers, remove only
  their record, and verify preview resumes, other records remain
  blocking, and a live lock cannot be bypassed (FR-009, FR-014).

## 2. Report inventory, grouping, and preflight

- [x] 2.1 Implement centralized naming rules, positive day parsing,
  overflow checks, and a fixed-clock cutoff in
  `internal/evidencecleanup/`; verify valid input, invalid units,
  fractions, leading zeroes, overflow, and strict cutoff boundaries
  with table-driven tests (FR-002, FR-003).
- [x] 2.2 Implement confined discovery for the three approved provider
  results roots and bounded label grammar; verify nested labels,
  malformed/future dates, old retained-layout evaluations, incomplete
  units, optional metadata, and unrecognized layouts using temporary
  synthetic workspaces (FR-003, FR-004, FR-005, FR-012).
- [x] 2.3 Implement complete managed-report inventory and typed
  relationship adapters for YAML/JSON EvaluationLogs and Markdown,
  OSCAL, and SARIF variants; verify existing outputs without new
  sidecars, embedded logs, cross-second filenames, naming collisions,
  missing evidence, unresolved variants, malformed reports, and
  out-of-scope references. Verify filenames alone never authorize
  associations, and unbounded uncertainty blocks workspace cleanup
  before mutation (FR-005, FR-012, FR-015).
- [x] 2.4 Build transitive report/evaluation groups and whole-group
  preflight; verify reports sharing any evaluation merge, even when
  they cite different contained files. Verify one newer, inaccessible,
  unsafe, or unresolved member preserves every member byte-for-byte.
  Verify reportless evaluations qualify only after complete inventory
  establishes no report references, and report ages follow every
  connected evaluation timestamp (FR-003 through FR-006, FR-015,
  FR-016).
- [x] 2.5 Implement shared preview planning and deterministic group
  results; verify every group member and blocker is shown in stable
  order, busy/unresolved activity is refused, and workspace contents
  remain unchanged without configuration or provider installation
  (FR-001, FR-007 through FR-009, FR-013).
- [x] 2.6 Document group membership, collection-start age, shared
  evidence, orphan handling, conservative ambiguity refusal, and
  preview in `docs/EVIDENCE.md`; verify examples against fixed-clock
  and inventory fixtures and state external/renamed report copies
  are outside managed scope (FR-002 through FR-007, FR-012, FR-015).

## 3. Confined group deletion and runtime failures

- [x] 3.1 Implement descriptor-relative deletion in Linux and Darwin
  backends with identity revalidation and no-follow/no-mount-crossing
  constraints; verify symlinked roots, labels, units, nested symlinks,
  special files, replaced candidates, nested mounts, and unavailable
  platform facilities cannot redirect deletion outside approved units.
  Verify a normal APFS workspace works without Linux-only APIs and
  ambiguous mount layouts refuse deletion (FR-010, FR-011, FR-012).
- [x] 3.2 Implement evidence-first group execution after complete
  preflight; verify no member changes when preflight fails, reports
  are removed only after all group evidence succeeds, and injected
  unexpected failures stop that group with exact partial progress.
  Verify independent safe groups can proceed, no rollback is promised,
  errors return nonzero, and no-match/repeated cleanup return zero
  (FR-006, FR-013, FR-016).
- [x] 3.3 Add behavioral fixtures covering all three provider layouts
  and all managed report variants; verify fully eligible groups lose
  both reports and evidence, while blocked groups retain old and newer
  evidence and reports byte-for-byte. Verify CLI logs, excluded files,
  and a second workspace remain unchanged (FR-004 through FR-006,
  FR-012, FR-015, FR-016).
- [x] 3.4 Document irreversible report/evidence expiry and unexpected
  partial failures in `docs/EVIDENCE.md`; verify guidance distinguishes
  whole-group preflight from transactional rollback, describes
  evidence-first deletion and remaining reports after failures, and
  recommends preview without implying it reserves later groups
  (FR-006, FR-007, FR-013, FR-016).

## 4. CLI, documentation, and end-to-end behavior

- [x] 4.1 Add and register the thin clean command, required age flag,
  workspace selection, preview, text results, and exit behavior; verify
  registration, argument validation, precedence, missing workspace,
  no registry/provider requirement, blocked-group output, and partial
  runtime failure using CLI tests (FR-001, FR-002, FR-006, FR-013,
  FR-015, FR-016).
- [x] 4.2 Adapt command hooks so preview including debug mode writes
  diagnostics only to streams; verify a before/after workspace
  inventory contains no created or changed logs, config, activity
  records, reports, or evidence (FR-007).
- [x] 4.3 Add command-level subprocess tests for scan-first and
  cleanup-first ordering, long-running old evaluations, generation,
  late publication, normal failure, and interrupted hosts; verify
  supported activity is protected with unchanged provider binaries
  and different workspaces remain independent (FR-008 through FR-010).
- [x] 4.4 Update `docs/man/complyctl.md`, regenerate its man page with
  `make man`, and add a `CHANGELOG.md` entry; assess `README.md` impact
  and update `AGENTS.md` for the new command/packages. Verify help and
  guidance agree on scope, expiry, support boundary, and errors.

## 5. Integrated verification

- [x] 5.1 Run `make build`, `make build-test-provider`,
  `make test-unit`, `make lint`, and `make vet`; verify they pass
  without modifying quality gates. Record environmental failures
  separately.
- [x] 5.2 Run `make test-e2e`, `make test-integration`, and
  `make test-cross-repo-evidence`; verify mapping and generation
  regressions remain intact and cleanup uses synthetic workspaces.
- [x] 5.3 Run applicable live cross-repository and schema checks from
  `.github/workflows/ci_cross_repo_integration.yml` with unchanged
  provider binaries and required credentials; verify mapped evidence,
  JSON/YAML logs, and retained evidence still satisfy existing checks.
  If prerequisites are unavailable, record the missing validation
  explicitly rather than treating it as passed.
- [ ] 5.4 Record runtime results for the shared cleanup behavior and
  platform-backend tests on Linux and macOS, covering amd64 and arm64
  and the baseline filesystems, including macOS APFS. Verify busy-lock,
  preview, complete group deletion, whole-group preflight retention,
  runtime partial-failure handling, interrupted-operation refusal, and
  manual recovery. Treat unavailable platforms as missing
  verification; cross-compilation alone cannot complete this task.
  Use existing runners or development machines without changing CI
  workflow gates (FR-006 through FR-016).
- [x] 5.5 Validate the change with
  `openspec validate evidence-cleanup --strict` and run
  `/uf.review-council` before PR submission; resolve blocking findings
  and verify no code changes occur after the approving review.

## 6. PR #991 accepted feedback: specification only

These tasks track accepted items 2 (duration) and 3 (output). Item 1
(mount tests, Testing Farm, and gates) remains HELD and MUST NOT be
changed by this follow-up. Prior completion states, including 5.4,
MUST remain intact. New tasks MUST stay unchecked in this spec phase.

- [x] 6.1 Review proposal, design, and requirements for strict `d/h/m`
  parsing, unit bounds, exact durations, one UTC capture, and the
  unchanged exclusive cutoff (FR-001 through FR-003).
- [x] 6.2 Review concise/verbose text and versioned JSON contracts,
  early-error token handling, null unknowns, service-owned outcomes,
  failure visibility, and stdout-failure recovery. Confirm FR-006,
  FR-007, FR-013, FR-017, and FR-018 agree; no behavioral or gate
  changes are authorized beyond the accepted duration/output fixes.
- [ ] 6.3 Validate, commit, and push the spec update before source work
  begins, using the user's explicit execution authorization.

## 7. PR #991 accepted feedback: source and reference updates

Source work MUST wait for 6.1 through 6.3. It MUST NOT alter existing
grouping, eligibility checks, deletion order, coordination, or exit
semantics. Presentation MUST preserve underlying service failures.

- [ ] 7.1 Extend the duration parser with strict single-unit `d/h/m`
  grammar and pre-multiplication unit bounds. Preserve one UTC clock
  capture and the exclusive cutoff (FR-002, FR-003).
- [ ] 7.2 Add service-owned structured statuses, reason codes,
  exclusions, and exact removed-path progress, including contained
  removals before failure. Keep rendering separate from service policy
  and do not derive structured outcomes from English text (FR-017).
- [ ] 7.3 Add concise default and verbose text renderers, report and
  evaluation outcome totals, plain age reasons, and unhidden safety,
  access, discovery, and partial errors (FR-006, FR-007, FR-013).
- [ ] 7.4 Add `--format text|json`, the complete versioned envelope,
  nullable unknowns, one-document stdout, and clean-local early-error
  handling including format tokens after invalid arguments/flags.
  Keep verbose JSON unfiltered and diagnostics on stderr (FR-017,
  FR-018).
- [ ] 7.5 Preserve write errors together with service errors and emit
  available exact progress/recovery details to writable stderr without
  deletion retries or duplicate JSON output (FR-018).
- [ ] 7.6 Update clean help, evidence guidance, command reference/man
  page, and changelog for accepted duration/output changes. Retain
  irreversible/non-transactional warnings and move routine reference
  text out of successful output. Assess README and AGENTS impact;
  leave user-owned untracked documents alone (FR-006, FR-007, FR-013).

## 8. PR #991 accepted feedback: verification

- [ ] 8.1 Add table-driven duration tests for all units, rejected
  grammar, minimums, exact per-unit maxima and maxima plus one,
  integer parse overflow, equivalent durations, daylight saving,
  and before/at/after cutoff decisions (FR-002, FR-003).
- [ ] 8.2 Test concise execution/preview summaries, unique member
  counts, ordinary age reasons without `Blocker`, verbose detail,
  exclusions, mixed age/safety findings, and partial/failed member
  totals with exact progress (FR-006, FR-007, FR-013, FR-016).
- [ ] 8.3 Test one JSON document through EOF for preview, execution,
  failure, verbose, and debug; assert schema, null unknowns, partial
  inventories, typed reasons, exclusions, exact removed paths, and
  structured errors without parsing English (FR-017).
- [ ] 8.4 Test missing required age, bad positional arguments, invalid
  values, unknown/malformed flags, both format spellings/orderings,
  repeated formats, unreadable final format, and `--` boundaries.
  Assert early JSON errors start no service/hooks/logs and preserve
  null unknowns and known-empty removal progress (FR-018).
- [ ] 8.5 Inject stdout failures in text and JSON, with and without
  service failure/partial progress; assert nonzero, preserved causes,
  stderr recovery/progress, and no deletion retry. Verify preview is
  read-only in default, verbose, JSON, and debug modes (FR-007,
  FR-018).
- [ ] 8.6 Run applicable existing CI-parity checks and spec validation
  for these fixes; record unavailable checks separately. Reassess
  documentation consistency and run the existing required review.
  MUST NOT change mount tests, Testing Farm, task 5.4, workflows, or
  quality gates as part of this feedback scope.

## Workflow follow-up

- Preserve the user's report of team-lead confirmation of grouped
  report/evidence cleanup. Have the team reconcile GRCAUTO-286 and
  GRCAUTO-283 with that scope through its normal process; do not claim
  this session independently verified or posted a Jira approval.
- Commit and publish specification artifacts before implementation only
  after explicit user authorization; do not create a PR or post
  comments through this specification workflow.
- Archive the change after implementation and required review. Archival
  is not part of the tracked implementation tasks.
