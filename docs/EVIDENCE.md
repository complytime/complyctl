# Evidence Source Provenance

When `complyctl scan` evaluates a target, each compliance finding
includes evidence -- the raw data a provider collected to support
its assessment. Evidence source provenance tells auditors, CI
pipeline authors, and SREs exactly *where* that evidence came from:
which API endpoint was queried, which configuration file was read,
and what cryptographic digest anchored the observation at collection
time.

This document walks through a real EvaluationLog to show how
evidence source provenance works, how to interpret it in each
report format, and how to use it in your own workflows.

## Anatomy of an EvaluationLog

The following EvaluationLog excerpt is from an Ampel provider scan
evaluating branch protection settings on the
`complytime/complytime-demos` repository. It shows one control
(`force-push-restriction`) with two evidence entries -- an Ampel
policy attestation and a Snappy data-collection observation, both
stored as in-toto attestations.

This is real output from `complyctl scan`, trimmed to one control
for readability. The full EvaluationLog contained five controls
(force-push-restriction, approval-requirements,
admin-bypass-prevention, code-owner-enforcement,
pull-request-enforcement), all with the same evidence structure.

```yaml
metadata:
  id: complytime/policies-ampel-branch-protection
  type: EvaluationLog
  gemara-version: v1.5.0
  description: Compliance scan evaluation log
  author:
    id: complytime
    name: complytime
    type: Software
    uri: https://github.com/complytime/complyctl
  mapping-references:
  - id: repo-branch-protection
    title: Branch Protection Controls
    version: 1.0.0
    description: Control catalog for branch protection rules
    url: file://../catalogs/ampel-branch-protection-catalog.yaml
  - id: ampel_github-com--complytime--complytime-demos_main_github--branch-rules
    title: Ampel policy attestation for complytime/complytime-demos@main
    version: ""
    description: In-toto attestation from ampel policy evaluation
    url: file:///home/user/workspace/.complytime/ampel/results/github-com-complytime-complytime-demos-main-github--branch-rules-ampel.intoto.json
  - id: snappy_github-com--complytime--complytime-demos_main_github--branch-rules
    title: Snappy observation for complytime/complytime-demos@main
    version: ""
    description: In-toto attestation from snappy data collection
    url: file:///home/user/workspace/.complytime/ampel/results/github-com-complytime-complytime-demos-main-github--branch-rules-snappy.intoto.json
result: Passed
evaluations:
- name: force-push-restriction
  result: Passed
  message: Force pushing is disabled in protected branch.
  control:
    reference-id: complytime/policies-ampel-branch-protection
    entry-id: force-push-restriction
  assessment-logs:
  - requirement:
      reference-id: complytime/policies-ampel-branch-protection
      entry-id: block-force-push
    plan:
      reference-id: complytime/policies-ampel-branch-protection
      entry-id: block-force-push
    description: Force pushing is disabled in protected branch.
    result: Passed
    message: Force pushing is disabled in protected branch.
    applicability:
    - default
    steps:
    - complytime/complytime-demos@main
    steps-executed: 1
    start: "2026-10-20T09:58:24Z"
    confidence-level: High
    evidence:
    - id: ampel_github-com--complytime--complytime-demos_main_github--branch-rules
      type: IntotoAttestation
      collected-at: "2026-10-20T09:58:24Z"
      payload: ""
      source:
        reference-id: ampel_github-com--complytime--complytime-demos_main_github--branch-rules
        coordinate: .complytime/ampel/results/github-com-complytime-complytime-demos-main-github--branch-rules-ampel.intoto.json
        digest: sha256:1db6fe3e86de51726867d8c85dbffa20f564ad1b388d4a0b42cbcb68338814b7
        remarks: "Collected on host \"scan-host\" (machine-id: a1b2c3d4e5f67890a1b2c3d4e5f67890)"
    - id: snappy_github-com--complytime--complytime-demos_main_github--branch-rules
      type: IntotoAttestation
      collected-at: "2026-10-20T09:58:24Z"
      payload: ""
      source:
        reference-id: snappy_github-com--complytime--complytime-demos_main_github--branch-rules
        coordinate: .complytime/ampel/results/github-com-complytime-complytime-demos-main-github--branch-rules-snappy.intoto.json
        digest: sha256:7bfca9fdf961ccb23aa4b67949a278c09c36d8d05b1f7af6bbf383d9f7e2a805
        remarks: "Collected on host \"scan-host\" (machine-id: a1b2c3d4e5f67890a1b2c3d4e5f67890)"
target:
  id: complytime-demos
  name: complytime-demos
  type: Software
```

## Tracing the Evidence Trail

Follow these five steps to trace any compliance finding back to its
origin.

### Step 1: Control Evaluation

Start at the `evaluations` array. Each entry represents one control
assessed by the provider. In the example above, the
`force-push-restriction` control has `result: Passed` with a
human-readable `message`: "Force pushing is disabled in protected
branch."

The `control` field links this evaluation to the policy definition
via `reference-id` and `entry-id`, connecting the runtime result to
the policy's static requirement.

### Step 2: Assessment Evidence

Inside each `assessment-logs` entry, the `evidence` array lists
every piece of data the provider collected for that requirement.
Each evidence entry has an `id`, `type`, `collected-at` timestamp,
and optionally a raw `payload` and `source` block.

In this example, both evidence entries are `IntotoAttestation` type
-- in-toto attestations produced by the Ampel policy evaluator and
the Snappy data collector respectively. The Ampel attestation
records the policy evaluation result, while the Snappy attestation
records the raw branch protection settings observed from the GitHub
API.

### Step 3: Source Provenance

The `source` block on each evidence entry identifies *where* the
data was collected from. Two fields establish the location:

- **`reference-id`**: Names the external system or artifact. This
  value ties the evidence to a `mapping-references` entry in the
  EvaluationLog metadata (see Step 5).
- **`coordinate`**: The precise location within that system -- an
  API path, file path, or JSON path expression.

In the first evidence entry,
`reference-id: ampel_github-com--complytime--complytime-demos_main_github--branch-rules`
and `coordinate: .complytime/ampel/results/github-com-complytime-complytime-demos-main-github--branch-rules-ampel.intoto.json`
together tell you: "This attestation was collected from the Ampel
results directory, specifically the in-toto attestation file for the
complytime-demos repository's main branch."

> **CI pipeline tip**: Extract the `coordinate` value to locate the
> source artifact on disk or construct direct links in your CI
> reports.

When `reference-id` points to a Gemara artifact rather than an
external system, `entry-id` replaces `coordinate` to identify a
specific entry within that artifact. The `coordinate` and `entry-id`
fields are mutually exclusive per the Gemara schema -- set one or
the other, never both.

### Step 4: Integrity and Host Identification

Two additional fields help verify evidence integrity:

- **`digest`**: A cryptographic hash (`algorithm:hex`, e.g.,
  `sha256:1db6fe3e86de5172...`) pinning the observed content at
  collection time. Compare this digest against the current state of
  the source to detect drift since the scan.
- **`remarks`**: Prose from the provider about the collection
  context -- which host performed the scan, any caveats about the
  data's freshness, or environmental conditions. In the example,
  `Collected on host "scan-host" (machine-id: a1b2c3d4e5f67890...)`
  identifies both the hostname and the unique hardware identity
  of the machine that ran the scan.

  The machine-id is read from `/etc/machine-id` by default. To
  override the file path (e.g., in containers or CI environments
  where `/etc/machine-id` is not available), set the
  `COMPLYTIME_MACHINE_ID_FILE` environment variable. The file must
  contain a non-empty hexadecimal string no larger than 1 KB. When
  the file is unreadable or contains invalid content, the
  machine-id is silently omitted from the remarks.

> **SRE tip**: Use the `digest` field to build drift-detection
> pipelines. If a re-scan produces a different digest for the same
> `reference-id` and `coordinate`, the underlying configuration has
> changed since the last assessment.

### Step 5: Mapping References Cross-Reference

The `mapping-references` array in the EvaluationLog metadata
provides full context for each `reference-id` used in evidence
source blocks. Each mapping reference includes:

- **`id`**: The identifier that evidence `reference-id` fields
  point to.
- **`title`**: A human-readable name for the external system.
- **`version`**: The version of the referenced system or artifact.
- **`url`** (optional): Where the referenced artifact can be
  retrieved.

The following diagram shows how `reference-id` values in evidence
source blocks resolve to `mapping-references` entries:

```
Evidence Source                                 Mapping References
+-------------------------------------------+  +-------------------------------------------+
| evidence[0].source                        |  | mapping-references[1]                     |
|   reference-id:                           |  |   id:                                     |
|     ampel_github-com--complytime--      +---->|     ampel_github-com--complytime--       |
|     complytime-demos_main_              |  |     complytime-demos_main_                |
|     github--branch-rules                  |  |     github--branch-rules                  |
|   coordinate: .complytime/ampel/         |  |   title: Ampel policy attestation for     |
|     results/...ampel.intoto.json          |  |     complytime/complytime-demos@main      |
|   digest: sha256:1db6fe3e...             |  +-------------------------------------------+
+-------------------------------------------+
                                               +-------------------------------------------+
+-------------------------------------------+  | mapping-references[2]                     |
| evidence[1].source                        |  |   id:                                     |
|   reference-id:                           |  |     snappy_github-com--complytime--      |
|     snappy_github-com--complytime--     +---->|     complytime-demos_main_                |
|     complytime-demos_main_              |  |     github--branch-rules                  |
|     github--branch-rules                  |  |   title: Snappy observation for            |
|   coordinate: .complytime/ampel/         |  |     complytime/complytime-demos@main      |
|     results/...snappy.intoto.json         |  +-------------------------------------------+
|   digest: sha256:7bfca9fd...             |
+-------------------------------------------+
```

When both a policy and a provider supply mapping references with the
same `id`, the policy definition takes precedence. complyctl logs a
collision warning and annotates the retained entry's `description`
field with the discarded provider reference.

## Field Reference

The five `source` fields on each evidence entry, with their
serialized YAML/JSON names from the Gemara `EvidenceMapping` type:

| Field | YAML/JSON Key | Purpose | Example |
|-------|--------------|---------|---------|
| Reference ID | `reference-id` | Names the external system or Gemara artifact this evidence was collected from. Ties the evidence to a `mapping-references` entry. | `ampel_github-com--complytime--complytime-demos_main_github--branch-rules` |
| Coordinate | `coordinate` | Precise location within the referenced system (API path, file path, JSON path). Mutually exclusive with `entry-id`. | `.complytime/ampel/results/...ampel.intoto.json` |
| Entry ID | `entry-id` | Specific entry within a referenced Gemara artifact. Mutually exclusive with `coordinate`. | `block-force-push` |
| Digest | `digest` | Cryptographic hash of the observed content at collection time. Format: `algorithm:hex`. | `sha256:bfd7b62e770beb96d3ff...` |
| Remarks | `remarks` | Provider prose about collection context, host identification, or caveats. | `Collected on host "scan-host" (machine-id: a1b2c3d4...)` |

> **Note**: `coordinate` and `entry-id` are mutually exclusive per
> the Gemara schema. Use `coordinate` for locations within external
> systems (APIs, files). Use `entry-id` for entries within Gemara
> artifacts.

Field names are verified against the `gemara.EvidenceMapping` struct
YAML tags and the `EvidenceSource` struct in
`pkg/provider/client.go`.

## Report Formats

Evidence source provenance appears in different levels of detail
depending on the output format.

### EvaluationLog (YAML and JSON)

All five `source` fields are serialized when present. Fields with
empty values are omitted via `omitempty` in YAML. In JSON output
(`--log-format json`), evidence entries without source provenance
emit `"source": {"reference-id": ""}` because Go's `encoding/json`
does not omit zero-value struct fields and `reference-id` lacks an
`omitempty` tag. YAML output omits the `source:` key entirely in
this case (`goccy/go-yaml` treats zero-value structs as empty for
`omitempty`).

### Markdown (`--format pretty`)

The Markdown report renders only `reference-id` and `coordinate`
from the source, formatted as:

```
source: ampel_github-com--complytime--complytime-demos_main_github--branch-rules @ .complytime/ampel/results/github-com-complytime-complytime-demos-main-github--branch-rules-ampel.intoto.json
```

When only `reference-id` is present (no `coordinate`), the format
is:

```
source: ampel_github-com--complytime--complytime-demos_main_github--branch-rules
```

This rendering is produced by `formatEvidenceMeta()` in
`internal/output/markdown.go`. The remaining three fields (`entry-id`,
`digest`, `remarks`) are available in the full EvaluationLog output
for users who need the complete provenance chain.

### OSCAL

OSCAL assessment-results reports preserve evidence on assessment
observations. Each control evaluation produces one finding, and each
assessment log produces one observation. The finding's
`related-observations` entries identify the observations created for
that control.

Each observation records its requirement and control IDs as Gemara
properties. Evidence is emitted in the observation's
`relevant-evidence` entries. Those entries retain the evidence
description, map available source identifiers and locations to
properties, and link to a matching mapping reference when one exists.

#### Migrating requirement-level finding parsers

This report structure is a breaking change for consumers that treat an
OSCAL finding as a requirement-level result. Update those consumers to:

OSCAL reports using this structure have `metadata.version` `2.0.0`; consumers
MUST select the control-level parser for that version.

1. Treat `results[].findings[]` as control-level findings. The finding
   target ID identifies the control, not an individual requirement.
2. Follow each finding's `related-observations[].observation-uuid` to
   the matching item in `results[].observations[]`.
3. Read the observation property named `requirement-id` to identify the
   requirement and the `result` property to identify its result.
4. Read `relevant-evidence` from that observation instead of expecting
   evidence on the finding.

Do not derive requirement counts from finding counts. A control with
multiple assessment logs has one finding and one linked observation per
assessment log.

#### Interpreting non-passing status

Do not treat every OSCAL `not-satisfied` finding as a failed assessment.
`NotApplicable` uses `not-satisfied` with the `not-applicable` reason, and
`NotRun` uses `not-satisfied` with the `not-run` reason rather than `unknown`.
Read each linked observation's `result` property and the finding status reason
to distinguish failed, not-applicable, and not-run assessments.

### SARIF

Evidence source fields are not currently included in complyctl's
SARIF output. The `go-gemara` SARIF converter does not process
evidence or source fields.

## Provider Support

Evidence source provenance requires provider support. When a
provider does not populate the `source` field on evidence entries,
the `source:` block is omitted from the EvaluationLog YAML output
and the `source:` line does not appear in Markdown reports.

Not all providers support evidence source provenance. Check your
provider's documentation for details on which evidence types include
source information and which fields are populated.

## Cleanup Grouping and Preview

Use `complyctl clean` to expire managed reports and their supporting
evaluations in one workspace. Preview first, then omit `--dry-run`
only after reviewing the groups and saving anything needed for audit:

```bash
complyctl clean --older-than 30d --dry-run
complyctl clean --older-than 30d
```

`--older-than` is required; there is no default age and no positional
argument. Workspace selection follows `--workspace` (or `-w`), then
`COMPLYTIME_WORKSPACE`, then the current directory. Select the project
root containing `.complytime/`, not `.complytime/` itself. The workspace
must already exist; cleanup does not initialize it or require policy
configuration, credentials, a registry connection, or providers.

The CLI calls `internal/evidencecleanup.Preview` for `--dry-run` and
`internal/evidencecleanup.Run` otherwise. Both share age, inventory,
grouping, coordination, and whole-group preflight rules. Cleanup sends
results to stdout and diagnostics to stderr, including with `--debug`;
neither mode creates or changes the workspace log file. The inherited
global debug help describes other commands' file logging, not cleanup.

### Managed Scope and Group Membership

The inventory considers recognized evaluation directories beneath
only these roots in the selected workspace:

- `.complytime/ampel/results/`
- `.complytime/opa/results/`
- `.complytime/openscap/results/`

An evaluation name contains `evaluation-`, a UTC collection-start
timestamp in `20060102T150405Z` format, a hyphen, and exactly 32
lowercase hexadecimal characters. Intermediate labels may be nested;
each label contains 1–80 ASCII letters, digits, underscores, or
hyphens. For example, the inventory fixtures use
`.complytime/opa/results/policy/target/evaluation-20261001T120000Z-0123456789abcdef0123456789abcdef`.
The evaluation directory and **all** its supporting outputs form one
indivisible evidence member, not separate file-level candidates.

Managed host reports live directly under `.complytime/scan/`. The
inventory selects format adapters using these naming conventions:

- `evaluation-log-` with a `.yaml` or `.json` suffix: EvaluationLog.
- `report-` with a `.md` suffix: Markdown with an embedded EvaluationLog.
- `assessment-results-` with a `.json` suffix: OSCAL assessment-results.
- `scan-` with a `.sarif.json` suffix: SARIF, subject to the provenance
  refusal described below.

Names select an adapter; they never prove a report's relationship to
evidence. The adapters read validated EvaluationLog content, the log
embedded in Markdown, or typed OSCAL evidence properties and mapping
references. No new sidecar or provider marker is required. Reports
with links to the same evaluation join one group, even when they cite
different files inside it. Sharing merges groups transitively across
reports and providers.

External or renamed report copies are outside the managed inventory
contract. Cleanup does not search for every copy or guarantee
protection of its references. Renaming is not a cleanup override:
report-like unknown files inside `.complytime/scan/` can instead block
the entire inventory. A changed filename that still follows a managed
convention still selects its adapter; its content determines its
relationships, not its apparent policy, target, or timestamp.

Evidence coordinates resolve relative to the selected workspace, or
as validated absolute local paths, including local `file://` URIs.
They can link only to already recognized evaluation units. They do
not authorize extra roots, access to another workspace, or remote
fetches. Mapping-reference URLs remain metadata, not fetch requests;
inline payloads without a source mapping are not filesystem targets.
CLI logs, configuration, scan inputs, OPA working clones, generation
state, caches, unrelated files, and unrecognized layouts remain
outside the managed groups. The flat evidence paths in the provenance
walkthrough above are not recognized retained-layout evaluations.

### Collection-Start Age and Shared Evidence

Age means elapsed time since the evaluation directory's encoded
collection start, not since report publication or the last filesystem
write. `--older-than` accepts canonical positive whole-day thresholds
such as `1d` and `30d`, with each day exactly 24 hours. It rejects
missing values, `0d`, `-1d`, `1.5d`, `01d`, whitespace padding,
unsupported units such as `30h`, and overflow. The maximum supported
threshold is `106751d`.

One captured UTC invocation time supplies the cutoff for every group.
With the fixed-clock fixture time `2026-11-18T12:00:00Z` and `30d`,
the cutoff is `2026-10-19T12:00:00Z`:

| Encoded collection start | Age decision                     |
| ------------------------ | -------------------------------- |
| `20261019T115959Z`       | Older; subject to preflight.     |
| `20261019T120000Z`       | Retain group: exact cutoff.      |
| `20261019T120001Z`       | Retain group: newer.             |
| `20261119T120000Z`       | Retain group: future start.      |
| `20260230T120000Z`       | Retain group: invalid date.      |

Every evaluation in a group must start **strictly before** the
cutoff. Reports follow all connected evaluation ages; neither report
filename timestamps nor modification times establish age. Optional
metadata does not establish age, completion, or inactivity.

For example, the shared-evidence fixtures have report A cite an old
Ampel evaluation and `first.json` in a shared OPA evaluation. Report B
cites `nested/second.json` in that same OPA evaluation and an old
OpenSCAP evaluation. Both reports and all three evaluations form one
group. If the shared OPA evaluation starts exactly at the cutoff,
every member is retained, including both older evaluations. A
detectable access failure, unsafe member, missing required evidence
file, or unresolved mapping reference likewise blocks the whole
connected group. Preflight leaves every member byte-for-byte unchanged
and identifies the blocking member; it does not rewrite references.

### Reportless Evaluations and Conservative Refusal

A reportless (orphan) evaluation is not automatically disposable.
Only a complete managed-report inventory can establish that no
managed report references it. An incomplete evaluation, including one
without metadata or a completion marker, can then form a standalone
group if its age, activity, access, and safety checks pass. A report
with no establishable evaluation age is retained rather than aged
from its filename. Labels and results/report roots are not expiry
members and are not candidates for pruning.

When a problem has a known scope, cleanup retains the affected
group while keeping independent groups' decisions. A missing file
inside a known evaluation, for example, keeps that evaluation linked
to every referring report. When possible relationships cannot be
bounded, it refuses workspace planning with no partial plan, rather
than treating apparently unreferenced evidence as orphaned. Malformed
reports, unsupported report-like variants, ambiguous coordinates,
and Markdown without an unambiguous embedded log can cause this
workspace-wide refusal.

Case-variant duplicate report fields are ambiguous and block planning
before typed decoding can overwrite evidence references. An evidence
coordinate rejected for unsafe or inaccessible traversal also blocks
workspace planning: cleanup does not follow a symlink to discover its
target or assume that target is an unreferenced evaluation.
An unmatched local evidence coordinate also blocks planning, even if
its final file is missing. Successful file access alone cannot
prove independence: case-insensitive filesystems can resolve different
spellings to the same evidence. Cleanup neither lowercases paths nor
assumes unmatched evidence is unrelated.
Absolute paths and file URIs outside the established workspace boundary
also block planning. An external alias can point directly into a managed
evaluation without passing through the workspace root; cleanup does not
treat that spelling as proof that the evaluation is unreferenced.

Current SARIF output lacks originating EvaluationLog and evidence
provenance. The adapter therefore refuses workspace planning whenever
a managed SARIF report is present, including an empty all-passing
report. Even a single apparently matching log, policy/target-like
filename, matching alert, or timestamp cannot prove ownership.
Formatters can publish across second boundaries, and names can
collide. Cleanup does not infer SARIF relationships from filenames
or silently omit SARIF to make other groups eligible.

### Read-Only Preview Decisions

`complyctl clean --older-than 30d --dry-run` calls the `Preview`
library API, which takes an existing workspace,
an explicit `OlderThan` threshold, and an injected clock. It captures
one age window, acquires exclusive workspace access, checks filesystem
support and unresolved activity, then inventories and preflights
complete groups. It needs no policy configuration, provider
installation, credentials, or registry connection. It does not create
or modify configuration, lock files, activity records, debug logs,
reports, or evidence.

For each otherwise eligible group, preview also prepares the same
descriptor-relative removal handles and platform checks as execution,
then closes them without unlinking anything. Insufficient file
descriptors or unavailable confinement facilities retain the group
rather than reporting `would-delete`.

The CLI prints the snapshot's canonical workspace and UTC cutoff,
expiry consequences, and a preview-only warning. It lists every group
member with a workspace-relative path,
kind (`evaluation` or `report`), status (`would-delete` or `retained`),
and reasons. Members and blockers have stable path ordering; groups
sort by their first member path. Totals count groups and indivisible
members, not every file inside an evaluation. Exclusions have their
own paths and reasons; their count is not a census of all unrelated
workspace files.

In the preview fixture with the fixed clock above:

- An old reportless Ampel evaluation is one `would-delete` member.
- An old OPA evaluation plus YAML/JSON EvaluationLogs, embedded-log
  Markdown, and OSCAL variants form another group: five
  `would-delete` members.
- An exact-cutoff OpenSCAP evaluation and its report form a retained
  group: two members, with the evaluation named as the age blocker.

The totals are two would-delete groups (six members) and one retained
group (two members); no member is actually deleted. Normal age
exclusions and no matches return no error. Bounded safety/access
failures return the full snapshot plus `ErrPreflightBlocked`, with
every member of each blocked group retained. Busy access, unresolved
activity, unsupported filesystems, or incomplete inventory return an
error without a partial plan. The CLI prints a returned snapshot even
when preflight reports blocked groups, then exits nonzero. No matches
and normal age exclusions exit zero.

Preview releases access when it returns. It does not reserve groups
or authorize later removal; a later execution must reacquire access
and repeat discovery and preflight. Passing preflight is not a
guarantee of future I/O success or transactional deletion. See
[Workspace Coordination and Cleanup Limitations](#workspace-coordination-and-cleanup-limitations)
for activity refusal, recovery, and the supported operating boundary.

### Irreversible Expiry and Partial Failures

Review `--dry-run` before running explicit cleanup, and retain
any reports and evidence needed for audit or recovery elsewhere.
Successful expiry permanently removes **both** the group's managed
reports and every supporting evaluation directory with its contents.
Cleanup provides no undo or transactional rollback. Preview is a
read-only snapshot, not a reservation of future groups; `Run` takes
fresh options, reacquires exclusive access, and repeats inventory and
whole-group preflight rather than accepting a saved preview.

Whole-group preflight checks every member before the first deletion,
including preparing access to every report and evaluation. A failed
check leaves the entire affected group unchanged and identifies the
blocking member. This zero-change guarantee for blocked preflight
does **not** guarantee future I/O success once removal begins.

`Run` finishes removing all group evidence before it starts removing
reports. An unexpected removal error stops that group immediately:

- If evidence removal fails, all group reports remain unchanged.
  Earlier evaluations may already be gone, and the failing evaluation
  may have lost some files or directories. Remaining reports can
  therefore reference evidence that no longer exists; cleanup does
  not rewrite those references or restore deleted evidence.
- If report removal fails, all group evidence is already gone.
  Earlier reports may also be gone; the failing and unattempted
  reports remain. The group is not reported as fully deleted.

Inspect cleanup's text output even when it exits nonzero. The CLI
prints the library's returned `Result` even when `Run` returns an error. It
identifies the workspace, cutoff, stable group membership, reasons,
blockers, failures, and totals. Each member has an actual status:
`deleted` means the entire member including its root was removed;
`partially-deleted` means some contained entries were removed but the
member remains; `retained` means this invocation removed nothing from
that member, even if removal failed before its first successful unlink.
`RemovedPaths` records exact successful removals, including directories,
in execution order per member. An unattempted member has no `Failure`.
Group `PartialLoss` records successful removals in a failed group;
totals distinguish complete deletions from partial members. These
results describe this invocation, not later external changes.

Independent safe groups continue after a bounded preflight or removal
failure, but the invocation still returns an error. Workspace-wide
refusals such as busy access, unresolved activity, or unbounded report
relationships stop execution before any deletion; a later workspace
revalidation failure stops further groups without undoing progress.
No matches, normal age exclusions, and repeated successful cleanup
return no error and exit zero. Invalid arguments, missing workspaces,
coordination or inventory refusal, safety/access failures, removal
errors, and result-output failures exit nonzero. Execution prints
per-member successful removals and failures, partial-deletion warnings,
and totals for deleted, retained, failed, and partially deleted work.

## Workspace Coordination and Cleanup Limitations

Use updated complyctl hosts for operations that share a workspace.
Host-side coordination protects provider work and report publication
from conflicting participating operations. This coordination is in
place for scan, generation, relevant `get` artifact removal, `init`,
and `clean`, including cleanup preview.

### Coordinated Operations

On Linux and macOS, scan and generation acquire shared access to the
existing workspace directory. Relevant `get` artifact removal and
`init` acquire exclusive access; cleanup also requires
exclusive access. Conflicting access fails promptly rather than
waiting or starting conflicting work. Workspace aliases contend on
the same directory, while separate workspaces remain independent.
Acquiring access does not create a lock file.

Before starting providers, the host persists a uniquely owned activity
record under `.complytime/coordination/operations/`. If locking or
record persistence fails, provider startup stops. Generation nested
inside a scan reuses the scan's access and activity record.

The scan holds access through report publication and normal provider
shutdown, not merely until a scan response arrives. After provider
RPCs complete normally and report writes finish, the host completes
normal provider teardown, removes only its own unchanged activity
record, and releases access. Handled application errors returned in a
successful provider response do not imply uncertain completion. A
handled scan or report-writing error can also release protection
without a leftover record after normal provider shutdown; this does
not mean that the scan succeeded or every report was published.

### Interrupted Activity and Cleanup Refusal

Treat a remaining activity record as unresolved activity, not as an
expired lock. A killed host can lose its kernel lock while a provider
or provider-launched tool still runs. Host interruption, abnormal or
forced provider termination, transport failure, or cancellation with
uncertain completion leaves the record in place. Closing workspace
access alone does not remove it.

The coordination checks reject unresolved activity even after the
kernel lock becomes available. Cleanup uses these checks before
deletion and refuses malformed, unreadable, or unsafe
coordination state as well. Elapsed time, a missing PID, and provider
metadata do not establish inactivity or automatically clear records.
Process identifiers in a record are diagnostic hints, not authority
to kill processes. There is no automatic stale-record or force-clean
override; unresolved records block cleanup until manual recovery.

Normal provider completion is the supported host-operation boundary,
not proof that every arbitrary descendant process has exited. Do not
clear records merely to bypass refusal: manual recovery requires
stopping all writers and preventing new scans. Absence of a record
does not establish that independent writers are absent.

### Interrupted-Scan Recovery

Use this procedure when an unresolved activity error blocks workspace
access. The error identifies the canonical workspace and exact
filesystem-discovered paths; the check leaves records intact and
deletes no reports or evidence. Cleanup preview uses this same
coordination check and cannot bypass unresolved activity.

1. Pause scheduled jobs and prevent new scans against this workspace.
   Keep them paused throughout recovery and the subsequent preview.
2. Stop all writers: affected complyctl hosts, providers, and
   provider-launched child tools, as well as independent callers.
   Record contents, elapsed time, and missing or reused PIDs cannot
   establish that all writers have stopped. Do not kill processes
   based on recorded PIDs or execute commands from record contents.
3. If you cannot confidently establish writer shutdown, restart the
   machine or VM that owns the writers, or stop the complete isolated
   container environment containing them. Restarting a terminal,
   complyctl alone, or an unrelated container is insufficient.
   Prevent automatic job restarts. If other machines or environments
   can write the workspace, stop those writers too or do not proceed.
4. Only after all writers have stopped and new scans remain paused,
   manually remove **only the exact unresolved activity records**
   identified by the error. This is your assertion of exclusive
   control, not proof supplied by complyctl. Do not use wildcards or
   remove the coordination directory, reports, evidence, configuration,
   or unrelated files. Recover each operation separately; other
   unresolved records continue blocking access.
5. Rerun `complyctl clean --older-than 30d --dry-run` with your chosen
   workspace and age, and review it before explicit cleanup. Removing a
   record never overrides a live updated host's kernel lock. Preview must
   still pass all other safety checks; recovery does not authorize deletion.

Malformed and unreadable records require the same precautions. Paths
come from filesystem discovery, not fields inside the records, and
diagnostics quote them to escape control characters. Quoted diagnostic
paths are not shell commands or shell-escaped arguments. Do not
construct executable shell commands from untrusted record contents.

An error may instead identify an unsafe or inaccessible coordination
directory. Inspect and repair that specific problem under the same
stopped-writer conditions; do not recursively delete the directory or
follow a symlink to clear the error. There is no automatic stale-record
expiry, PID-killing recovery, or force-clean override. See
[Interrupted Activity and Cleanup Refusal](#interrupted-activity-and-cleanup-refusal)
for the normal-completion boundary and why interruption retains records.

### Operating Systems and Filesystems

For exact devcontainer, native macOS, and Docker/Podman verification steps,
see [Cleanup native-platform testing](EVIDENCE_NATIVE_TESTING.md). The runbook
separates native matrix evidence from emulated runs and container filesystem
refusal checks.

The host release matrix targets Linux and macOS (`darwin`), each on
amd64 and arm64. Cleanup has Linux and Darwin backends for these
targets; macOS is not an excluded cleanup platform. This support
boundary is not a claim that all native runtime matrix checks or live
cross-repository checks are complete. Cross-compilation alone does
not verify runtime behavior. Operating system support does not imply
support for every filesystem:

- Linux cleanup requires a local ext4, XFS, Btrfs, or tmpfs workspace.
- macOS cleanup requires a local APFS workspace.
- The cleanup support check refuses unknown, network, FUSE, overlay,
  and HFS+ filesystems. This includes container workspaces on overlay
  filesystems, even when the host runs Linux.

macOS preflight inspects ACL metadata on the opened file descriptor through
libSystem, using the vendored PureGo binding on amd64 and arm64. Builds still
support `CGO_ENABLED=0`; no additional library installation is needed. An ACL,
unavailable binding, or failed metadata query blocks the affected group rather
than falling back to a pathname check or assuming that no ACL exists.

The cleanup filesystem allowlist does not apply to ordinary scan or
generation coordination. Those operations still require working
locking and activity-record persistence on Linux and macOS; they do
not silently proceed unprotected on failure. Passing the workspace
filesystem check alone does not validate descendant mounts or
authorize deletion. Cleanup also refuses unavailable
confinement facilities and unsafe or ambiguous mount layouts.

Coordination works with unchanged provider binaries launched by an
updated host; it requires no new provider RPC or provider-side marker.
Older hosts, direct provider calls, and independent external writers
do not participate in these host locks. Running them against the same
workspace during cleanup is unsupported. Keep the workspace namespace
stable: external directory moves and administrative mount changes are
outside the supported operating mode. Windows is not a host release
target or part of the cleanup support boundary; existing non-cleanup
commands retain their platform behavior outside Linux and macOS.

## See Also

- [Quick Start](QUICK_START.md) -- getting started with complyctl
- [Man page](man/complyctl.md) -- full CLI reference including
  OUTPUT and FILES sections
