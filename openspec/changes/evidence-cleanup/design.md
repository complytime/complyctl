# Design

## Context

See `proposal.md` for motivation and the user-approved scope.
`specs/evidence-cleanup/spec.md` defines observable behavior.

The provider storage implementation creates private evaluation units
with atomic directory allocation, timestamps, and random tokens. It
keeps descriptors open and validates artifact identity at finalization.
Neither descriptors nor finalization prevent concurrent deletion.
Metadata is optional supporting context, not an inactivity signal.

The host owns provider processes through go-plugin and writes reports
after provider results return. Provider shutdown is currently deferred
in `cmd/complyctl/cli/scan.go`. A safe host coordination interval must
extend through both report publication and confirmed provider shutdown.

The host release matrix in `.goreleaser.yaml` targets Linux and macOS,
each on amd64 and arm64. `docs/INSTALLATION.md` documents Homebrew for
both. Repository CI runs on Linux; there is no Windows release target.
The user requires cleanup on both release operating systems and both
architectures. macOS support is required in this increment.

The user reports team-lead confirmation of report-and-evidence expiry
as a group. The confirmed guarantee is whole-group preflight, not
transactional rollback after an unexpected filesystem failure. This
replaces the earlier report-preserving policy and the issue's proposed
exclusion of host reports; see proposal provenance.

PR #991 items 2 and 3 amend only duration input and output contracts.
Item 1 (mount tests, Testing Farm, and gates) remains HELD. Existing
platform obligations remain unchanged; this follow-up MUST NOT alter
their tests, task states, or gates. Implementation MUST wait for the
parent to commit and push this specification update.

Sources inspected during specification:

- Provider `internal/evidence/storage_linux.go`: allocation, `Close`,
  and `Finalize`; `metadata.go`: optional supporting metadata.
- Provider retention handlers in the AMPEL, OPA, and OpenSCAP servers.
- Host `cmd/complyctl/cli/scan.go`, `generate.go`, and `get.go`.
- Host `pkg/provider/initialization.go` and `manager.go`.
- Host `internal/complytime/workspace.go` and CLI `options.go`.
- Host `internal/output/evaluator.go`, `filename.go`, `markdown.go`,
  `oscal.go`, and `sarif.go`: report identity and evidence projection.
- Existing `policy-generation` and `cross-repo-integration-test`
  OpenSpec capabilities; neither needs a requirement change here.

## Goals / Non-Goals

**Goals:**

- Make existing retained-layout evaluations eligible without provider
  migration or changes to their contents.
- Establish one workspace identity and coordination boundary across
  updated host commands, including failure paths.
- Preserve complete report-and-evidence groups when any member is
  ineligible or fails preflight, including shared evidence links.
- Separate command presentation, selection policy, coordination, and
  confined filesystem operations so each can be tested independently.

**Non-Goals:**

- Detect or stop older hosts, direct provider callers, or arbitrary
  external writers. These are outside the accepted support boundary.
- Infer safe crash recovery from process lists, age, or metadata.
- Add a provider protocol, provider marker, or cross-repository change.
- Discover external or renamed report copies, delete remote evidence,
  clean generation state, or promise transactional rollback.

## Decisions

### 1. Keep the CLI thin

Add `cmd/complyctl/cli/clean.go` for argument validation, workspace
resolution, service invocation, and text/JSON output. Put candidate
policy and deletion orchestration in `internal/evidencecleanup/`.
Put shared workspace protection in `internal/workspacecoord/` for scan,
generate, get, and init where their lifetimes overlap managed storage.
The names describe responsibilities rather than provider types.

Use an injected clock and a narrow filesystem/coordination boundary.
Keep provider names, naming grammar, and statuses centralized within
the owning package. Reuse vendored platform libraries and applicable
Go rooted filesystem facilities; do not import a sibling `internal`
package. A common service owns policy and results, with build-tagged
Linux and Darwin backends owning locking and filesystem primitives.
Do not assume `os.Root` alone rejects symlinks or mount crossings.

### 2. Build complete groups before applying collection-start age

Accept only `[1-9][0-9]*[dhm]`: one positive decimal integer and one
lowercase unit. Reject leading zeroes, fractions, signs, compounds,
whitespace, missing units, and other units. Before multiplication,
bound the count by `floor(MaxInt64 / unitNanoseconds)` and reject
integer parse overflow. The inclusive maxima are `106751d`,
`2562047h`, and `153722867m`; each maximum plus one MUST fail.
Capture invocation time once in UTC and subtract the duration once.
Days mean exactly 24 hours, hours 60 minutes, and minutes 60 seconds;
calendar boundaries and daylight saving MUST NOT alter the cutoff.
The exclusive cutoff and all collection-start selection rules remain
unchanged. Equivalent durations MUST give identical fixed-clock plans.

Traverse the three approved provider results roots and the managed
report directory, `.complytime/scan/`. Provider intermediate labels
must match the existing provider label grammar, `[A-Za-z0-9_-]`,
with lengths from 1 through 80 characters. Recognize the current
32-lowercase-hex evaluation token; unknown future formats are skipped.
Do not descend into an evaluation to discover smaller cleanup units.
An evaluation directory and all its contained supporting outputs form
one evidence member. Two reports citing different files inside the
same evaluation are therefore connected.

Inventory reports before selecting any evaluation for deletion:

- Parse managed YAML and JSON EvaluationLogs with their serialized
  kebab-case fields, policy/target identities, assessments, evidence
  coordinates, and mapping references.
- Associate Markdown through its embedded EvaluationLog. Recognize
  OSCAL and SARIF companions only where validated content/provenance
  establishes their relationship to the inventoried EvaluationLogs.
- Never treat a timestamp or filename match alone as ownership proof.
  Each formatter computes its filename timestamp independently, so
  companion files can cross a second boundary. Concurrent outputs can
  also share policy, target, and timestamp fragments.
- SARIF omits the full evidence mappings. If its originating log and
  complete group cannot be established unambiguously, preserve its
  possible groups. Refuse workspace cleanup when the affected scope
  cannot be bounded. Do not silently ignore the variant.

Use report content and typed format adapters for these relationships;
do not parse a rendered summary table as an ownership index. Existing
data must work without a new manifest or a provider change. An
unprovable relationship is an explicit retained/blocked result, not
permission to guess. Parse reports as untrusted local input, using
bounded reads and existing YAML/JSON libraries.

Resolve local evidence coordinates relative to the selected workspace,
or validate absolute coordinates against it. A coordinate may add an
edge only to an existing recognized evaluation unit; it never grants
new deletion authority. Mapping-reference URLs are metadata, not a
reason to fetch a location or expand the managed roots. Missing files,
ambiguous or unsupported references, and references outside the managed
scope prevent complete group preflight. Inline evidence that has no
external file is not an extra filesystem deletion target.

Build connected components of report variants and evaluation members,
merging transitively through shared evaluations. Establish the complete
managed inventory before deleting any group: a later-discovered report
must not reveal an already-deleted evaluation was still protected.
Malformed reports or unresolved variants block their possible groups;
if their possible relationships cannot be bounded, refuse the whole
workspace before mutation. Report-like unknown files under the managed
report root must not be silently classified as harmless. Ordinary
unrelated files remain excluded and untouched.

The managed inventory covers host outputs in `.complytime/scan/` using
recognized formats and conventions. Moved, renamed, or externally
copied reports are outside this contract and must be documented as
such; cleanup cannot find every copy or protect its references.

Ignore report/evidence modification times and optional metadata for
age. Every evaluation in a group must have a valid collection-start
timestamp strictly before the cutoff. One newer, future, or malformed
timestamp keeps the entire group. Reports follow their connected
evaluation ages rather than a separately inferred filename age. A
report with no establishable evaluation age is retained.

Incomplete or unreferenced evaluations can form standalone groups only
after the complete managed inventory proves no report references them.
All activity, age, confinement, and access checks still apply. Do not
prune empty labels, results roots, or the report root. The evaluation
member includes supporting files without declaring every file evidence.

### 3. Lock the existing workspace directory

On Linux and macOS, open the existing workspace directory and use a
nonblocking kernel `flock` on that directory descriptor through the
platform backend. Vendored `golang.org/x/sys/unix` supplies this API
for both release architectures on each operating system. Validate
directory-lock behavior with real subprocess tests on both platforms.
This avoids creating a lock file during preview or before cleaning old
workspaces. Resolve aliases and pin directory identity consistently;
different path spellings of the same directory must contend.

Use shared access for provider-starting scan/generate operations and
exclusive access for cleanup. Use exclusive access for get's parent
artifact invalidation/removal and init's configuration initialization,
with acquisition before their workspace mutations. Acquire each
command's access once; nested generation during scan reuses access
instead of reacquiring it. Never upgrade a shared lock in place.

Hold scan protection from before provider startup through all report
writes and confirmed provider shutdown. Arrange defer ordering so
provider cleanup runs before access is released. A busy command fails
promptly with the workspace and conflicting-operation explanation.
Different workspaces continue operating independently.

Support local ext4, XFS, Btrfs, and tmpfs on Linux and APFS on macOS
as the baseline filesystem matrix. Reject other filesystem types for
cleanup, including network, FUSE, overlay, and HFS+, until equivalent
semantics are explicitly validated. Distinguish an unsupported
filesystem or unsafe mount layout from an unsupported OS: a normal
APFS workspace on macOS must support cleanup in this increment.
Refuse cleanup if locking or confined traversal is unavailable,
without making Linux-only syscalls a prerequisite on macOS. Existing
commands on other platforms retain their existing behavior. Within
either supported environment, scan/generate coordination failures must
stop provider startup rather than silently run unprotected.

Alternatives rejected: modification-age grace periods, process-list
checks, metadata checks, and renaming evaluations alone do not exclude
writers. Provider-side coordination would broaden repository scope.
Creating a conventional lock file during preview violates its
non-mutating behavior.

### 4. Preserve uncertainty after host interruption

A kernel lock owned only by the host can disappear before an orphaned
provider stops. Before starting any provider, write a private,
uniquely named host activity record under
`.complytime/coordination/operations/` while holding shared access.
Persist the record before provider startup. Remove only this
invocation's record after all its providers are confirmed stopped;
handled failures can remove it after the same confirmation.

Include the operation type, start time, canonical workspace identity,
and available host/provider process identifiers for diagnosis. Avoid
secrets, credentials, and full process environments. Recorded process
identifiers are diagnostic hints, never authority to kill a process or
declare an operation inactive.

Cleanup takes exclusive access, then checks for unresolved records.
Because live supported invocations retain shared access, a remaining
record is sufficient reason to refuse deletion. Malformed, unreadable,
or unsafe coordination state also causes refusal. Do not use PIDs,
timeouts, or provider metadata to discard records automatically.

Legacy workspaces have no such records and need no migration. These
records track activity by the updated host, not eligibility of old
evaluation directories. They do not certify that independent callers
are absent; that remains the accepted compatibility limitation.

The first increment has no force override or automatic process-killing
recovery command. It must provide the following manual recovery path
in the error message and in `docs/EVIDENCE.md`:

1. Identify the workspace and exact unresolved record paths. State that
   cleanup deleted no evidence and leave the records intact.
2. Pause scheduled jobs and prevent new scans against that workspace.
   Stop the affected hosts, providers, and provider-launched tools.
   The process details in the record can help diagnosis but do not
   prove that all writers have stopped.
3. If stopping all writers cannot be established confidently, restart
   the machine or VM that owns them, or stop the complete isolated
   container environment containing them. Restarting a terminal, the
   complyctl command alone, or an unrelated container is insufficient.
   Prevent automatic job restarts. If other machines or environments
   can write the workspace, stop those writers too or do not proceed.
4. After establishing that writers are stopped and new scans remain
   paused, manually remove only the exact unresolved activity records
   identified by the error. Do not remove the coordination directory,
   evidence, configuration, or unrelated files. This is an operator
   assertion of exclusive control, not proof supplied by complyctl.
5. Rerun cleanup preview, then explicit cleanup after reviewing it.
   Normal workspace locking still applies; record removal is not an
   override for a live updated host's lock.

For malformed or unreadable records, identify the affected path without
trusting its content, retain the same recovery conditions, and do not
generate shell commands from untrusted record fields. A successful
recovery of one record must not remove records of other operations.

Error output must name the recovery documentation section, not end at
"workspace busy". For example, its meaning should be:

> Cleanup is blocked by an interrupted scan. No evidence was deleted.
> Unresolved activity record: [the exact validated record path].
> Stop all writers and pause new scans before removing that record.
> If writer shutdown is uncertain, restart the complete writer
> environment first. Restarting a terminal is not sufficient.
> See the interrupted-scan recovery section of the evidence guide,
> then rerun cleanup preview.

Do not automatically kill processes from recorded PIDs: they may have
been reused, and terminating one provider does not prove that all its
external tools have exited. Preserve the record whenever provider
shutdown cannot be confirmed.

An inherited provider lock descriptor was considered, but an unchanged
provider has no obligation to retain unknown descriptors. A persistent
host record avoids making that assumption. It favors refusal after a
crash over risking deletion of active evidence.

### 5. Preview follows the deletion path without writes

Validation, workspace resolution, directory locking, activity checks,
complete report discovery, grouping, age decisions, and whole-group
preflight are shared with execution. Only the final removal action
differs. Preview never initializes
coordination files, provider state, or workspace configuration.

Root command hooks must not initialize file logging for clean preview;
debug diagnostics go to stderr. Cleanup does not load providers,
resolve policies, contact registries, or require a valid policy config
to remove otherwise recognized accumulated evidence.

Default text MUST show the workspace and concise report/evaluation
totals: deleted and kept for execution, would-delete and kept for
preview. Preview MUST clearly say no deletion occurred. Ordinary age
retention MUST use a plain age reason, never a `Blocker` label. Full
paths, stable group/member detail, exclusion detail, and removal
progress belong in `--verbose` text. Exact UTC cutoff and capture time
MUST remain available in verbose text and JSON. Generic safety and
platform reference text belongs in help and evidence documentation,
not repeated on successful runs. Help MUST retain irreversible
report/evidence deletion and non-transactional failure warnings.
Every invocation discovers its own candidates under exclusive access.

Concise execution example for a single too-new evaluation and report:

```text
Workspace: /home/analyst/workspace
Nothing to clean: no report/evaluation groups qualify for deletion.
Kept: 1 report and 1 evaluation (evaluation newer than the 30-day cutoff).
```

The displayed reason MUST distinguish newer, equal-cutoff, and future
timestamps. Summary counts MUST count unique report files and
evaluation units, not groups or contained evidence files. Partial
evaluations MUST NOT count as fully deleted or wholly kept; summaries
MUST identify partial/failed members separately. Preview MUST NOT
count planned removals as actual deletions.

Safety, access, discovery, and partial-removal failures MUST remain
actionable in default text and JSON, even alongside ordinary age
retention. Rendering MUST NOT stop after the first age reason and
hide an already-known safety failure. Name affected paths/members,
retained groups, actual partial loss, and recovery steps where needed.
This is presentation of service findings, not a new preflight policy.

### 5a. Versioned output and command-local validation errors

`--format` MUST accept only `text` and `json`, defaulting to `text`.
`--verbose` MUST affect text detail only; it MUST NOT filter JSON
fields or change the schema or outcomes. The service MUST own typed
statuses, reason codes, exclusions, and removal progress. Renderers
MUST NOT infer outcomes by parsing English errors or reason strings.
This follows separation of concerns; presentation changes MUST NOT
change service selection, deletion ordering, or exit decisions.

JSON schema version 1 MUST contain these top-level fields on success
and failure, with UTC timestamps serialized as RFC 3339 strings:

- `schema_version`: integer `1`.
- `mode`: `preview` or `execution`, null only if intent is unreadable.
- `workspace`: resolved absolute path, or null if unresolved.
- `captured_time` and `cutoff`: exact service times, null if unknown.
- `totals`: null before inventory, otherwise an object with `reports`
  and `evaluations` objects, each containing `deleted`, `kept`,
  `would_delete`, `partial`, and `failed` counts. Unknown counts MUST
  be null, not fabricated zeroes. Partial/failed counts MAY overlap;
  deleted and kept MUST count only complete member outcomes.
- `groups`: stable ordered group objects with `id`, `status`,
  `reasons`, `members`, `removed_paths`, and `error`.
- `exclusions`: ordered objects with `path`, `status`, and `reasons`.
- `removed_paths`: exact workspace-relative paths actually removed,
  in removal order, including contained paths removed before failure.
- `error`: null or an object with `code`, `message`, `paths`,
  `recovery_steps`, and `causes` for combined underlying errors.

Members MUST have `path`, `kind` (`report` or `evaluation`), `status`,
`reasons`, `removed_paths`, and `error`. Group/member errors MUST use
the top-level error shape, or null when no error applies.
Group/member status values MUST be
`would_delete`, `deleted`, `kept`, `blocked`, `partial`, or `failed`;
exclusions MUST use `excluded`. Each reason MUST carry a stable
`code`, human `message`, and affected `paths`. Centralized codes MUST
distinguish ordinary age outcomes from safety, access, discovery,
coordination, and removal failures. A group MAY have multiple reasons.
Messages MAY evolve without changing code meaning. Group membership
MUST remain available after partial execution. Group and top-level
errors MUST expose known failures without requiring English parsing.

Unknown inventory arrays MUST be null; known-empty arrays MUST be
empty arrays. Known partial inventories MUST retain discovered
entries, with unknown totals null and a discovery error explaining
incompleteness. Known no-removal progress MUST be an empty array even
on validation failure. Unknown workspace, times, or totals MUST NOT
be fabricated by reading workspace state merely to render an error.

JSON MUST be exactly one stdout document, followed by a newline,
without banners, progress lines, usage text, or debug diagnostics.
Diagnostics MUST use stderr only, without creating files. Buffering
the final result before encoding avoids progress document streams.
`--verbose --format json` MUST retain the same complete schema.

Missing required age, bad arguments, invalid age/format, and flag
parse errors MUST use this envelope when JSON intent is readable.
Use clean-local argument validation and a clean-local flag-error
handler, plus narrowly scoped error-output suppression if required.
A broad root-command rewrite or changes to other commands are out
of scope. Normal Cobra parsing MUST remain authoritative for errors.

For error rendering only, inspect clean's raw argument tokens before
the first literal `--`, including tokens after an invalid argument
or flag. Recognize exact `--format=json` and `--format json` forms;
for repeats, the last explicit format occurrence wins. A missing
value or value beginning with `--` is unreadable, not JSON intent.
An invalid or unreadable final format MUST fall back to stderr text
errors. Tokens after `--` MUST NOT select format. This narrow lexical
rule MUST NOT repair invalid arguments or allow service execution.
Requested mode follows readable `--dry-run` Boolean intent; invalid
Boolean intent MUST yield null. Early parse errors MUST NOT enter
workspace hooks or initialize logging. Root error/usage printing MUST
NOT append another stdout document or replace the structured result.

On stdout write failure, return nonzero and preserve both the write
error and any underlying service error. Do not retry with a second
JSON document or turn partial cleanup into apparent success. Emit
available recovery instructions and exact partial progress to stderr,
even without verbose, when stderr remains writable. Output failure
MUST NOT trigger rollback, retries of deletion, or new exit semantics.

### 6. Confine removal and revalidate candidate identity

Anchor operations to the opened workspace, approved provider roots,
and recognized report files under `.complytime/scan/`.
Use descriptor-relative, no-follow traversal and deletion, with
candidate identity recorded during discovery and rechecked immediately
before removal. Do not rely on string-prefix checks or validation
followed by an unconstrained `RemoveAll` of a user-derived pathname.

Both backends must implement beneath-root, no-follow traversal and
mount-boundary exclusion. Linux uses `openat2` resolution constraints
and mount-instance identity checks, including bind mounts, rather
than relying only on device IDs. If required Linux facilities are
unavailable, refuse rather than use unconstrained path traversal.

Darwin uses component-by-component `openat` with `O_NOFOLLOW` and
`O_DIRECTORY`, no-follow `fstatat`, and `unlinkat` relative to retained
directory descriptors. Check `fstatfs` identities and the platform
mount table to exclude nested mounts, including mounts that report
the same filesystem identity. Device or filesystem identity equality
alone is not proof that no nested mount exists. Validate the mount
table against the opened canonical workspace and recheck for changes
before removal. Refuse when mount identity or layout is ambiguous.
Darwin must not call a Linux-only helper or silently skip confinement.

The supported operating mode holds exclusive complyctl access and
requires a stable filesystem namespace during deletion. Independent
external writes, administrative mount changes, or moving managed
directories while cleanup runs are outside that operating mode on
both platforms. Neither OS supplies an inode-conditional unlink through
these APIs. Do not claim atomic protection against an arbitrary writer
between the final identity check and unlink. Detect observed namespace
changes and refuse further affected removal; do not treat a sequence
of pathname checks as equivalent to descriptor-relative confinement.

Preflight every member of a complete group before its first deletion:
all reports, complete evaluation contents, age, ownership/identity,
safe traversal, and detectable access restrictions. Check parent
directory permissions and platform ACL, sticky-bit, immutable, and
read-only restrictions where applicable; file readability alone does
not determine unlink permission. None of these checks guarantees a
future filesystem operation will succeed.

A known inaccessible member, symlink, mount, special file, or other
failed safety check retains the entire group, byte-for-byte. Do not
delete the group's eligible old members while skipping a blocked
member. A missing required evidence file is an incomplete group, not
a reason to delete the surviving report. Unexpected device, socket,
or FIFO entries block the group. Revalidate identities while removing
through confined handles; do not expand authority through coordinates.

Once a group passes preflight, delete evaluation members first, then
its managed report variants. Start report deletion only after all
group evidence removals have succeeded. On the first unexpected
removal error, stop that group and retain its remaining members.
If evidence deletion fails, no report deletion for that group starts.
An error during report removal may leave only some reports present;
report exactly which members were removed and which remain.

Deletion is not transactional. An unexpected I/O or permission change
after preflight can leave partial progress. Report the group as failed
with a partial-deletion warning, return nonzero, and continue only
independent fully preflighted groups. Do not promise rollback, attempt
to invent replacement evidence, or claim all-or-nothing disk mutation.
Normal age exclusions, no matches, and repeated successful cleanup
return zero; unsafe discovery, access failures, and removal errors do
not. An unresolved partially removed group remains protected on later
invocations rather than becoming an excuse to delete its reports.

### 7. Test the behavior and retain existing gates

Use temporary synthetic workspaces and a fixed clock for eligibility
and deletion tests. Include AMPEL, OPA, and OpenSCAP names and labels;
existing evaluations; missing metadata; incomplete outputs; symlinks;
replaced entries; permission failures; and excluded workspace content.
Cover existing YAML/JSON logs and Markdown/OSCAL/SARIF associations,
shared evaluations, transitive groups, cross-second filenames, naming
collisions, malformed reports, unresolved variants, missing evidence,
and references outside approved roots. Verify blocked groups remain
byte-identical, including old evidence that individually qualifies.
Verify fully eligible groups lose both evidence and reports. Inject
an unexpected failure after the first removal to distinguish preflight
retention from accepted partial runtime failure. Check report removal
never starts after evidence removal fails.

Use separate processes and barriers for real kernel-lock assertions,
not sleep-based timing or goroutines alone. Cover both command orders,
workspace aliases, independent workspaces, late report publication,
normal error paths, and a killed host with unresolved activity.
Verify the recovery error identifies exact records and gives the
required steps. Simulate operator recovery only after test-controlled
writers have exited; assert that other records remain blocking and
that removing a record never bypasses a live host lock.
Exercise the unchanged-provider path without modifying the provider
repository. Use stored-layout fixtures to test macOS cleanup even when
a particular provider executable is unavailable there; provider
availability must not prevent cleaning existing evidence. Run shared
behavioral tests and platform-backend tests on real Linux and macOS
kernels, including APFS on macOS. Cover both amd64 and arm64 release
targets. Cross-compilation checks do not replace runtime verification.
Tests must distinguish unsupported facilities from passing assertions,
and destructive tests must never target a developer's accumulated
evidence.

Required implementation checks come from the current CI workflows:
unit, E2E, integration, and cross-repository evidence checks. Add local
build, lint, and vet. Preserve API compatibility and existing mapping,
schema, and released-host assertions when running live cross-repository
tests. Do not change gates to accommodate this feature.

Current CI provides Linux validation only. Require recorded macOS
runtime results from a macOS runner or development machine before
marking the implementation complete. Unavailable macOS infrastructure
is missing validation, not a passing check or permission to ship a
Linux-only cleaner. Adding or changing CI workflows is outside this
specification's implementation scope.

## Risks / Trade-offs

- Independent callers ignore host locks → document the accepted support
  boundary in command help and guidance, without routine output noise.
- Host interruption can block later cleanup → retain activity records
  and fail closed, with documented manual recovery after writers stop;
  automatic process-killing recovery is outside this increment.
- Cleanup irreversibly removes reports and supporting files → require
  an explicit threshold, preview complete groups, and retain the whole
  group on preflight failure.
- Platform/filesystem restrictions limit availability → report an
  actionable filesystem error; validate both OS backends and do not
  substitute a blanket macOS refusal for implementation.
- Partial runtime deletion is possible → remove evidence before
  reports, stop the affected group, report exact progress, and return
  nonzero; preflight is not a rollback guarantee.
- Old report relationships can be ambiguous → preserve possible groups
  or refuse the workspace when scope cannot be bounded. Report why
  accumulated data was retained rather than silently omitting it.
- Workspace rename or mount replacement can change identity → pin and
  revalidate identity and refuse if the managed location changes.

## Migration Plan

1. Review the grouped-expiry specification. Preserve provenance that
   team-lead confirmation was relayed by the user; reconcile the Jira
   story's earlier report exclusion through the normal team process.
2. Commit and publish spec artifacts before implementation, as required
   by the repository. This session does not authorize publication.
3. Implement and test host coordination before enabling deletion.
4. Introduce report inventory and grouping without requiring changes
   to provider binaries, report formats, or existing evaluations.
   Only explicit cleanup removes selected complete groups.
5. Recommend preview before execution. No migration is needed for
   reconstructible retained-layout groups; preserve ambiguous groups.

Rolling back the host removes the cleanup command and its protection;
older hosts must not run concurrently with cleanup. Rollback cannot
restore deleted evidence. Unrecognized flat legacy layouts remain
outside scope in either version.
