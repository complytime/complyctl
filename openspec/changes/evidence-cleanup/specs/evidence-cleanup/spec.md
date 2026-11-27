# Evidence Cleanup

## Purpose

Let users expire workspace-local reports and their supporting evidence
as preflighted groups, coordinated with supported complyctl activity.

## ADDED Requirements

### Requirement: FR-001 Explicit workspace-scoped command

The CLI MUST provide `clean --older-than <duration>` with optional
`--dry-run`, `--verbose`, and `--format text|json`. It MUST accept no
positional arguments and MUST NOT have an implicit age default.
It MUST honor the existing workspace flag, environment variable, and
default precedence. It MUST NOT require registry access, credentials,
provider installation, or policy resolution to clean existing evidence.

#### Scenario: Select another workspace
- **GIVEN** two workspaces contain eligible evidence
- **WHEN** the user selects one workspace and requests cleanup
- **THEN** only managed reports and evidence there are considered
- **AND** no provider executable or registry connection is needed

#### Scenario: Invalid invocation
- **GIVEN** a missing threshold, extra argument, or missing workspace
- **WHEN** cleanup is invoked
- **THEN** it returns nonzero before changing the workspace

### Requirement: FR-002 Positive single-unit duration threshold

The threshold MUST match `[1-9][0-9]*[dhm]`: a positive decimal integer
and exactly one lowercase day, hour, or minute unit. It MUST reject
integer parse overflow and apply the selected unit's bound before
multiplication. One captured UTC time MUST determine one exclusive
cutoff for the invocation.

#### Scenario: Duration grammar and unit conversion
- **GIVEN** an explicit cleanup threshold
- **WHEN** the parser validates and converts it
- **THEN** a day MUST mean 24 hours, an hour 60 minutes, and a minute
  60 seconds, not calendar subtraction
- **AND** missing, zero, leading-zero, signed, fractional, compound,
  whitespace-containing, unsupported-unit, and overflowing values
  MUST fail before mutation
- **AND** integer parsing MUST reject overflow
- **AND** before multiplication, the count MUST be bounded by
  `floor(MaxInt64 / unitNanoseconds)` for the selected unit

#### Scenario: Valid threshold
- **GIVEN** the invocation time is 2026-11-18T12:00:00Z
- **WHEN** the threshold is `30d`
- **THEN** the cutoff is 2026-10-19T12:00:00Z for every candidate

#### Scenario: Invalid threshold
- **WHEN** input is `0d`, `01h`, `+1m`, `-1d`, `1.5d`, or `1h30m`
- **OR** it contains whitespace, lacks a unit, uses `D` or `s`,
  or exceeds the unit's bound
- **THEN** cleanup fails without deleting evidence

#### Scenario: Equivalent units and exclusive boundaries
- **GIVEN** one fixed UTC invocation time
- **WHEN** thresholds are `1d`, `24h`, and `1440m`
- **THEN** they produce identical cutoffs and group decisions
- **AND** evaluations exactly at the cutoff remain retained
- **AND** `1h` and `60m` also produce identical decisions
- **AND** daylight saving and calendar boundaries do not affect age

#### Scenario: Maximum safe multiplication by unit
- **GIVEN** the duration representation is signed 64-bit nanoseconds
- **WHEN** parsing `106751d`, `2562047h`, or `153722867m`
- **THEN** each is accepted without arithmetic overflow
- **AND** increasing any count by one fails before multiplication
- **AND** an integer too large to parse also fails without mutation

### Requirement: FR-003 Collection-start age selection

Cleanup MUST use the UTC collection-start timestamp encoded in each
recognized evaluation directory name, not filesystem modification
times. Every evaluation in a report group MUST start strictly before
the cutoff. A malformed, future, exact-cutoff, or newer timestamp MUST
preserve its entire group. Report eligibility MUST follow connected
evaluation ages, not report modification times.

#### Scenario: Boundary timestamps
- **GIVEN** evaluations before, at, and after the cutoff
- **WHEN** cleanup selects candidates using a fixed invocation time
- **THEN** only groups containing exclusively older evaluations qualify
- **AND** changing modification times does not change that result

#### Scenario: Invalid or future timestamp
- **GIVEN** a directory has an impossible date or starts in the future
- **WHEN** cleanup encounters it
- **THEN** it preserves that evaluation's entire group with a reason

### Requirement: FR-004 Recognized managed evaluation units

Cleanup MUST recognize evaluation directories beneath only
`.complytime/{ampel,opa,openscap}/results/`, through valid label
components. The recognized name MUST use `evaluation-`, UTC format
`20060102T150405Z`, a hyphen, and a 32-character lowercase hex token.
Each evaluation and its contained outputs MUST remain an indivisible
member of its cleanup group. Discovery MUST NOT split its files into
independent deletion candidates.

#### Scenario: All provider layouts
- **GIVEN** each provider has old evaluations under nested labels
- **AND** their report groups pass complete preflight
- **WHEN** cleanup executes
- **THEN** selected evaluations and their contained outputs are removed
- **AND** their labels and managed roots are not pruned

#### Scenario: Unrecognized layout
- **GIVEN** old flat files or unrecognized directory names exist
- **WHEN** cleanup discovers provider results
- **THEN** it leaves those entries untouched and reports exclusions

### Requirement: FR-005 Existing and incomplete retained evaluations

Existing evaluations MUST follow the same grouping and safety rules
as new evaluations, without new markers. Metadata presence MUST NOT
prove completion or inactivity. An incomplete or unreferenced
evaluation MUST qualify as a standalone group only when complete
managed-report discovery establishes no report references it and all
other preflight checks pass.

#### Scenario: Previously accumulated evidence
- **GIVEN** old recognized directories lack any cleanup-specific marker
- **AND** some are incomplete or have no metadata or report reference
- **WHEN** complete report discovery establishes their relationships
- **THEN** otherwise eligible groups can be deleted without migration

#### Scenario: Metadata does not establish inactivity
- **GIVEN** a supported scan is active in an old evaluation directory
- **WHEN** cleanup encounters metadata that appears complete
- **THEN** metadata does not permit deletion during the scan

### Requirement: FR-006 Reports and evidence expire together

Cleanup MUST delete an eligible group's managed reports and supporting
evaluations together in scope, after whole-group preflight. It MUST
finish evidence removal before starting report removal. If preflight
fails, every group member MUST remain unchanged. Retained reports
MUST NOT have their references rewritten to newer evidence.

#### Scenario: Explain irreversible expiry without routine noise
- **WHEN** the user reads help or evidence documentation
- **THEN** it MUST explain irreversible report/evidence deletion and
  the non-transactional limit
- **AND** preview MUST label planned removals and state no deletion
  occurred, without repeating reference warnings on successful runs

#### Scenario: Expire a fully eligible report group
- **GIVEN** all reports and evidence in a group pass preflight
- **WHEN** cleanup executes successfully
- **THEN** all group evidence and managed report variants are deleted
- **AND** reports are removed only after all group evidence is removed

#### Scenario: One member must be retained
- **GIVEN** a report references both old and newer evaluations
- **WHEN** preflight finds the newer evaluation is not eligible
- **THEN** the report and both evaluations remain byte-identical

#### Scenario: Known evidence access failure
- **GIVEN** one evidence member has a detectable access problem
- **WHEN** cleanup preflights the group
- **THEN** the report and all evidence remain unchanged
- **AND** it reports which member prevented group cleanup

### Requirement: FR-007 Non-mutating preview

`--dry-run` MUST use the same cutoff, grouping, whole-group preflight,
and coordination checks as execution, with concise default output
and full group/member detail in verbose text and JSON. It MUST NOT
change workspace files or create a lock file,
activity record, configuration, debug log, or report. A preview MUST
NOT authorize or freeze candidates for a later cleanup invocation.

#### Scenario: Preview a workspace with accumulated evidence
- **GIVEN** eligible evidence and no coordination files exist
- **WHEN** the user requests a preview, including with debug enabled
- **THEN** the default summary clearly states no deletion occurred
- **AND** verbose text and JSON expose paths and reasons
- **AND** workspace bytes remain unchanged
- **AND** diagnostics use streams rather than new log files

#### Scenario: State changes after preview
- **GIVEN** a preview completed successfully
- **WHEN** a later execution starts
- **THEN** it independently reacquires access and discovers candidates

### Requirement: FR-008 Supported concurrent operations

Updated complyctl MUST coordinate provider-starting scan and generation
operations, relevant parent-artifact removal, and cleanup in the same
workspace. Cleanup MUST acquire exclusive access before discovery and
hold it through deletion. Provider-starting activity MUST remain
protected through provider shutdown and report publication. Conflicting
operations MUST fail promptly before starting conflicting work.

#### Scenario: Scan starts first
- **GIVEN** a supported scan holds workspace access
- **AND** its evaluation start predates the cleanup cutoff
- **WHEN** cleanup or preview starts
- **THEN** it returns a busy error without deleting anything

#### Scenario: Cleanup starts first
- **GIVEN** cleanup holds exclusive workspace access
- **WHEN** another updated complyctl starts a scan or generation
- **THEN** the conflicting operation fails before starting a provider

#### Scenario: Provider returns before reports are published
- **GIVEN** a supported scan is writing its reports
- **WHEN** cleanup requests access
- **THEN** it cannot delete evidence before publication finishes

### Requirement: FR-009 Interrupted operations fail closed

If an updated complyctl operation ends without establishing provider
shutdown, cleanup MUST refuse deletion even if its ordinary workspace
lock has been released. A missing host process, elapsed time, or absent
provider metadata MUST NOT automatically clear that uncertainty.
The first increment MUST NOT offer a force-clean or automatic stale
activity override.

#### Scenario: Host exits while a provider could remain active
- **GIVEN** provider work started under the updated complyctl
- **WHEN** the host is killed before recording safe shutdown
- **THEN** a later cleanup returns nonzero without deleting evidence
- **AND** it identifies unresolved interrupted activity

#### Scenario: Normal completion or handled failure
- **GIVEN** all started providers have been confirmed stopped
- **WHEN** the host finishes normally or returns a handled scan error
- **THEN** its activity protection is released without a stale record

### Requirement: FR-010 Compatibility and platform boundary

Cleanup MUST work with unchanged existing provider binaries. Help and
evidence documentation MUST state that older hosts and direct provider
calls during cleanup are unsupported and are not stopped by host locks.
Cleanup MUST support Linux and macOS on amd64 and arm64, with matching
selection, preview, deletion, and recovery behavior. Unsupported
filesystems or failed coordination MUST cause refusal before deletion.

#### Scenario: Unchanged provider runs under updated host
- **WHEN** an existing provider is launched by updated complyctl
- **THEN** host-side coordination protects its supported operation
- **AND** no new provider RPC or provider marker is required

#### Scenario: Cleanup on each supported operating system
- **GIVEN** an ordinary supported local workspace on Linux or macOS
- **WHEN** cleanup previews and removes eligible accumulated evidence
- **THEN** both operating systems provide the specified behavior
- **AND** macOS does not fail merely because Linux APIs are unavailable

#### Scenario: Native runtime verification
- **GIVEN** the cleanup implementation targets amd64 and arm64
- **WHEN** platform support is verified
- **THEN** runtime tests cover both architectures on Linux and macOS
- **AND** cross-compilation alone is not runtime verification

#### Scenario: Platform or locking is unsupported
- **WHEN** cleanup cannot establish supported filesystem coordination
- **THEN** it returns an actionable error and deletes nothing
- **AND** existing non-cleanup commands retain platform compatibility

### Requirement: FR-011 Confined filesystem deletion

Cleanup MUST stay within approved provider results roots and managed
report files under `.complytime/scan/` in the selected workspace. It
MUST NOT follow symlinks, cross mount boundaries, or remove replaced
candidates without identity revalidation. A detected unsafe member
MUST block its entire group. Path checks alone MUST NOT authorize
unconfined deletion.

#### Scenario: Symlink or mounted content
- **GIVEN** a root, label, evaluation, or contained entry is a symlink
- **OR** a descendant crosses a mount boundary
- **WHEN** cleanup evaluates the affected path
- **THEN** it retains the entire affected group before deletion starts
- **AND** the link target or mounted content remains unchanged

#### Scenario: Candidate identity changes
- **GIVEN** discovery recorded an eligible evaluation
- **WHEN** that entry is replaced before deletion
- **THEN** cleanup does not delete the replacement as the original unit
- **AND** it reports a failure for the changed candidate

### Requirement: FR-012 Excluded workspace content

Cleanup MUST leave CLI logs, configuration, scan inputs, OPA working
clones, generation state, caches, unrelated files, unrecognized
layouts, and other workspaces untouched. External or renamed report
copies are outside the managed inventory. Evidence coordinates MUST
only establish relationships to recognized local evaluation units;
they MUST NOT authorize extra deletion roots or remote access.

#### Scenario: Mixed workspace content
- **GIVEN** eligible evidence coexists with excluded workspace content
- **WHEN** cleanup executes
- **THEN** excluded content and another workspace remain byte-identical

### Requirement: FR-013 Results and exit status

No matches and normal age exclusions MUST return zero. Busy, unsafe
or incomplete discovery, preflight access failures, and deletion
errors MUST return nonzero. Independent safe groups MUST remain
independently actionable. Text MUST be concise by default and keep
actionable failures visible; verbose text and JSON MUST expose full
outcome detail.

#### Scenario: Default summary and detailed outcome presentation
- **GIVEN** cleanup has service outcomes to render
- **WHEN** the user selects text output
- **THEN** text MUST be the default format and follow these contracts

Default output MUST identify the
workspace and summarize unique report/evaluation deleted and kept
counts, using would-delete counts for preview. It MUST explain
ordinary age retention without calling it a `Blocker`. Fully deleted
counts MUST NOT include partial members; partial/failed outcomes MUST
be identified separately. Verbose text MUST add exact UTC captured
time/cutoff, paths, stable group/member reasons, exclusions, and
removal progress. JSON MUST retain full detail regardless of verbose.

Actionable safety, access, discovery, coordination, and partial-failure
diagnostics MUST remain visible in nonverbose text and JSON error/group
fields. A failed preflight MUST name the affected member and retained
group even alongside normal age reasons. Generic platform and safety
reference text MUST reside in help/documentation rather than repeated
successful output.

#### Scenario: No matches or repeated cleanup
- **WHEN** no eligible evaluation remains
- **THEN** cleanup succeeds with zero deleted items

#### Scenario: Concise ordinary age retention
- **GIVEN** one report references one evaluation newer than `30d`
- **WHEN** execution uses default text output
- **THEN** stdout shows the resolved workspace first
- **AND** it says no report/evaluation groups qualify for deletion
- **AND** it summarizes one kept report and one kept evaluation
- **AND** it explains the newer-than-30-day-cutoff reason
- **AND** it omits member paths, `Blocker`, and routine safety warnings

#### Scenario: Mixed age and safety retention
- **GIVEN** results contain an ordinary age reason and a safety failure
- **WHEN** rendered without verbose in text or JSON
- **THEN** the safety failure remains actionable with affected paths
- **AND** age retention does not hide the failure or nonzero exit

#### Scenario: Verbose preview stays read-only
- **GIVEN** a synthetic workspace with eligible and excluded entries
- **WHEN** preview runs in default, verbose, or JSON mode with debug
- **THEN** summaries distinguish would-delete from actual deletion
- **AND** verbose text and JSON include members and exclusions
- **AND** no workspace bytes, logs, or coordination files change

#### Scenario: Blocked group and independent eligible group
- **GIVEN** one group has a preflight access failure
- **AND** another group independently passes complete preflight
- **WHEN** cleanup executes
- **THEN** it retains every member of the blocked group
- **AND** it can delete the independently eligible group
- **AND** it returns nonzero and names the blocked group and member

### Requirement: FR-014 Actionable interrupted-scan recovery

On unresolved activity, cleanup MUST identify the workspace and exact
record paths, state that no evidence was deleted, and provide manual
recovery steps and a documentation reference. Recovery MUST require
stopping all writers and pausing new scans before removing only the
identified records and retrying preview. Process identifiers MUST be
diagnostic only, not authorization to kill or clear activity.

#### Scenario: Recovery instructions after interruption
- **GIVEN** an unresolved activity record blocks cleanup
- **WHEN** cleanup reports the error
- **THEN** it gives the affected record's exact path and recovery steps
- **AND** guidance explains restarting the complete writer environment
  when shutdown is uncertain, not merely restarting a terminal
- **AND** it warns against proceeding if independent writers may remain

#### Scenario: Operator recovers a stopped operation
- **GIVEN** all relevant writers have stopped and new scans are paused
- **WHEN** the operator removes only the identified unresolved record
- **THEN** a subsequent preview can proceed if all other checks pass
- **AND** unrelated records and workspace content remain unchanged

#### Scenario: Manual record removal does not override live access
- **GIVEN** an updated host still holds workspace access
- **WHEN** its activity record is manually removed
- **THEN** cleanup refuses exclusive access while the host is active

### Requirement: FR-015 Complete report and evidence groups

Cleanup MUST inventory managed YAML/JSON EvaluationLogs and Markdown,
OSCAL, and SARIF variants before deleting evidence. It MUST establish
relationships from validated report content and provenance, not
timestamps or filenames alone. Reports sharing any evaluation MUST
belong to one transitive group, even when citing different files in
that evaluation. Unresolved relationships MUST preserve affected
groups; unknown scope MUST refuse workspace cleanup.

#### Scenario: Shared evidence across reports
- **GIVEN** two reports cite different files in the same evaluation
- **AND** one report also cites a newer evaluation
- **WHEN** cleanup forms connected groups and checks eligibility
- **THEN** both reports and all linked evaluations are retained

#### Scenario: Recognize existing report variants
- **GIVEN** existing report variants have validated relationships
- **WHEN** cleanup inventories managed reports without new sidecars
- **THEN** all associated variants join their evidence group
- **AND** matching timestamps alone do not establish a relationship

#### Scenario: Malformed or unresolvable report
- **GIVEN** a managed report is malformed or has unresolved evidence
- **WHEN** cleanup cannot bound its possible evidence relationships
- **THEN** it refuses workspace cleanup before deleting anything
- **AND** it identifies the report preventing complete discovery

### Requirement: FR-016 Whole-group preflight and runtime failures

Cleanup MUST complete eligibility, completeness, identity, confinement,
and detectable access checks for every group member before its first
deletion. Any failed check MUST preserve the whole group. Preflight
MUST NOT claim to guarantee future I/O success. An unexpected removal
failure MUST stop that group, retain its remaining files, identify
partial progress, and return nonzero. Rollback of already-deleted
files MUST NOT be required or promised.

#### Scenario: One newer or unsafe member blocks old evidence
- **GIVEN** a group has eligible old evidence and a blocked member
- **WHEN** whole-group preflight fails
- **THEN** none of its reports or evidence are modified

#### Scenario: Unexpected evidence error after successful preflight
- **GIVEN** all group members passed preflight
- **AND** one evidence member was successfully deleted
- **WHEN** an unexpected I/O error occurs on another evidence member
- **THEN** cleanup stops removing members of that group
- **AND** all group reports remain and partial loss is reported
- **AND** already-deleted files are not promised to be restored

#### Scenario: Unexpected report removal error
- **GIVEN** all group evidence was successfully removed
- **WHEN** an unexpected error interrupts report removal
- **THEN** cleanup stops the group and reports exact partial progress
- **AND** it does not report the group as fully deleted

### Requirement: FR-017 Versioned structured cleanup output

`--format` MUST accept only `text` or `json`, defaulting to `text`.
JSON MUST emit one stdout document and newline on success or failure
when stdout is writable. Diagnostics MUST go to stderr only. JSON
MUST NOT contain progress documents, banners, usage, or debug text.
`--verbose` MUST NOT filter JSON data or change its schema.

#### Scenario: Complete versioned result envelope
- **GIVEN** a cleanup result or error with known or unknown context
- **WHEN** the command renders JSON
- **THEN** it MUST use the following complete envelope contract

Schema version 1 MUST include `schema_version`, `mode`, `workspace`,
`captured_time`, `cutoff`, `totals`, `groups`, `exclusions`,
`removed_paths`, and `error`, as defined in design decision 5a.
Mode MUST be `preview` or `execution`, or null if intent is unreadable.
Workspace MUST be the resolved absolute path when known. Times MUST
be exact captured UTC RFC 3339 values when known. Unknown fields and
totals MUST be null, not invented values or zeroes. Unknown inventory
MUST be null, distinct from known-empty arrays. Partial inventories
MUST preserve known entries and mark incomplete totals unknown.

Groups and members MUST expose typed statuses, stable reason codes,
affected paths, and exact removal progress from service-owned data.
Exclusions MUST expose structured outcomes. Renderers MUST NOT parse
English messages to infer status, reason codes, or error categories.
Structured errors MUST include code, message, paths, recovery steps,
and causes where applicable. Exact removed paths MUST include actual
contained-path removals before a member fails. JSON MUST NOT claim
planned, remaining, or failed removals succeeded. These additions MUST
NOT change grouping, eligibility, deletion order, coordination, or
existing exit decisions.

#### Scenario: One JSON document on success and service failure
- **GIVEN** preview, successful execution, or partial deletion failure
- **WHEN** output uses JSON with or without verbose and debug
- **THEN** stdout decodes once and the next decode reaches EOF
- **AND** schema and complete outcome detail do not depend on verbose
- **AND** diagnostics appear only on stderr without file logging
- **AND** partial failures retain exact removed paths and group errors
- **AND** incomplete members do not count as fully deleted or kept

### Requirement: FR-018 Early errors and output failures

Readable JSON intent MUST cover validation and flag-parse failures,
even after bad arguments/flags, using decision 5a's clean-local token
rule. Normal Cobra validation MUST remain authoritative. Output
failure MUST preserve nonzero and any underlying service error.

#### Scenario: Error rendering without broad command changes
- **GIVEN** clean validation fails before service invocation
- **WHEN** the command selects the error format
- **THEN** clean-local handling MUST follow decision 5a before `--`,
  with the last explicit format occurrence winning
- **AND** invalid or unreadable final format MUST use stderr text
- **AND** the probe MUST NOT repair input, execute cleanup, initialize
  workspace hooks, or create logs
- **AND** unrelated command changes and broad root rewrites remain
  outside scope

#### Scenario: Preserve available context on early and write errors
- **GIVEN** early validation or output writing fails
- **WHEN** cleanup reports the failure
- **THEN** it MUST follow these context and error preservation rules

Early JSON errors MUST preserve known request data, set unknown
workspace/times/totals/inventory null, and include a structured error.
Known no-removal progress MUST be empty, not unknown. Output write
failure MUST return nonzero and preserve any underlying service error
alongside the write error. Available recovery and partial-progress
details MUST fall back to stderr when writable, even without verbose.
It MUST NOT emit a replacement JSON document or retry deletion.

#### Scenario: JSON validation envelope before service invocation
- **GIVEN** missing age, extra arguments, or a malformed/unknown flag
- **AND** readable `--format json` or `--format=json` precedes `--`
- **WHEN** cleanup validates the invocation, in either token order
- **THEN** it returns nonzero with exactly one JSON error document
- **AND** unknown fields/totals are null and removed paths are empty
- **AND** no service, workspace hooks, or file logging are started

#### Scenario: Deterministic format recovery boundaries
- **WHEN** repeated formats, a missing value, or `--` occur
- **THEN** the last explicit format before `--` selects error rendering
- **AND** unreadable or invalid final format uses stderr text errors
- **AND** tokens after `--` do not select JSON or repair invalid input

#### Scenario: Stdout fails after partial cleanup
- **GIVEN** cleanup has a service error with known partial removals
- **WHEN** text or JSON output encounters a failing stdout writer
- **THEN** the command remains nonzero and preserves both errors
- **AND** writable stderr receives exact progress and recovery details
- **AND** no deletion retries, rollback, or success claims occur
