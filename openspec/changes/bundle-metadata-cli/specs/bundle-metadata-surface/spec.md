## ADDED Requirements

### Requirement: Policy metadata cached at sync time

When `complyctl get` successfully fetches a fresh policy (i.e.,
`SyncPolicy()` returns `fetched=true`), OR when the existing
`PolicyState` entry has empty metadata fields (upgrade backfill),
the system MUST extract policy metadata from the cached OCI
layout and persist it in `state.json` alongside the existing
sync state fields.

The following metadata fields MUST be extracted and cached:

- `policy_title`: The `title` field from the Gemara Policy
  YAML (`gemara.Policy.Title`)
- `policy_evaluator`: The evaluator ID from the policy's
  evaluation methods. For single-evaluator policies, this is
  the executor ID from the policy-level or plan-level
  evaluation method. For multi-evaluator policies (assessment
  plans bound to different evaluators), this MUST be empty
  string (matching existing `extractFromGemaraPolicy()`
  behavior where `len(evalIDSet) > 1` yields `""`)
- `control_count`: The number of top-level entries in
  `ControlCatalog.Controls` (i.e., `len(catalog.Controls)`).
  Sub-controls and assessment requirements are not counted
  separately
- `assessment_count`: The total number of assessment plans in
  the Policy (i.e., `len(policy.Adherence.AssessmentPlans)`),
  regardless of evaluator binding

All fields MUST use `omitempty` JSON tags for backward
compatibility with existing `state.json` files.

**Call ordering**: Metadata extraction MUST happen after
`SyncPolicy()` returns. The sequence is: `SyncPolicy()` (which
internally calls `UpdatePolicyStateWithVerification` and
`SaveState`) -> `ExtractPolicyMetadata()` ->
`SetPolicyMetadata()` -> `SaveState()`. The double-save of
`state.json` (once inside `SyncPolicy`, once after metadata
is set) is intentional and acceptable for a small JSON file.

**Field name mapping**: The `PolicyMetadata.EvaluatorID` field
returned by `ExtractPolicyMetadata()` maps to
`PolicyState.PolicyEvaluator` when persisted. This is distinct
from `PolicyState.EvaluatorID`, which is reserved for
complypack entries (see design decision D6).

#### Scenario: Fresh fetch populates metadata
- **GIVEN** a workspace with a policy entry pointing to a
  valid OCI registry
- **WHEN** `complyctl get` fetches the policy for the first
  time
- **THEN** `state.json` MUST contain the `policy_title`,
  `policy_evaluator`, `control_count`, and `assessment_count`
  fields for that policy

#### Scenario: Re-fetch updates metadata
- **GIVEN** a policy was previously fetched and has metadata
  in `state.json`
- **WHEN** `complyctl get` detects a new digest, re-fetches
  the policy, and `SyncPolicy()` returns `fetched=true`
- **THEN** the metadata fields in `state.json` MUST be
  updated to reflect the new policy content

#### Scenario: No-change sync preserves existing metadata
- **GIVEN** a policy was previously fetched with metadata in
  `state.json`
- **WHEN** `complyctl get` determines the policy is already
  up-to-date (no digest change)
- **THEN** the existing metadata fields MUST be preserved
  unchanged in `state.json`

#### Scenario: Existing state.json without metadata fields
- **GIVEN** a `state.json` created by an older version of
  complyctl that does not contain metadata fields
- **WHEN** the state is loaded by the new version
- **THEN** the missing metadata fields MUST default to their
  zero values (empty string for strings, 0 for integers)
- **AND** no error MUST be returned

#### Scenario: Upgrade backfill for unchanged policies
- **GIVEN** a policy was fetched by an older complyctl version
  (no metadata in `state.json`) and the policy digest has not
  changed since the original fetch
- **WHEN** `complyctl get` runs and `SyncPolicy()` returns
  `fetched=false`
- **THEN** metadata MUST still be extracted and cached
  (backfill) because the existing `PolicyState` has empty
  `PolicyEvaluator` field
- **AND** `state.json` MUST be saved with the backfilled
  metadata

#### Scenario: Multi-evaluator policy
- **GIVEN** a policy with assessment plans bound to different
  evaluator IDs (e.g., one plan uses "opa", another uses
  "ampel")
- **WHEN** metadata is extracted
- **THEN** `policy_evaluator` MUST be empty string
- **AND** `complyctl list` MUST show "-" in the EVALUATOR
  column for that policy

#### Scenario: Policy with empty title
- **GIVEN** a cached policy whose Gemara Policy YAML has an
  empty `title` field
- **WHEN** metadata is extracted
- **THEN** `policy_title` MUST be set to empty string
- **AND** the `get` summary MUST omit the title portion
  and show only the evaluator

### Requirement: Lightweight metadata extraction

The `policy.Resolver` MUST provide an
`ExtractPolicyMetadata()` method that extracts
display-oriented metadata from a cached policy without
building a full `DependencyGraph`.

The method MUST return a `PolicyMetadata` struct containing
`Title`, `EvaluatorID`, `ControlCount`, and `AssessmentCount`.

The method MUST support both bundle-format and split-layer
OCI manifest shapes.

Note: `PolicyMetadata.EvaluatorID` maps to
`PolicyState.PolicyEvaluator` when persisted (not to
`PolicyState.EvaluatorID`, which is reserved for complypacks).
See design decision D6 for the naming rationale.

#### Scenario: Bundle-format policy
- **GIVEN** a cached policy using the Gemara bundle format
  (`application/vnd.gemara.manifest.v1+json` config media
  type)
- **WHEN** `ExtractPolicyMetadata()` is called
- **THEN** the returned `PolicyMetadata` MUST contain the
  correct title, evaluator ID, control count, and assessment
  count from the bundle's policy and catalog artifacts

#### Scenario: Split-layer policy
- **GIVEN** a cached policy using the split-layer format
  (distinct media types per layer)
- **WHEN** `ExtractPolicyMetadata()` is called
- **THEN** the returned `PolicyMetadata` MUST contain the
  correct title, evaluator ID, control count, and assessment
  count from the respective layers

#### Scenario: Policy without control catalog
- **GIVEN** a cached policy that contains a Policy YAML but
  no ControlCatalog layer
- **WHEN** `ExtractPolicyMetadata()` is called
- **THEN** `ControlCount` MUST be 0
- **AND** no error MUST be returned for the missing catalog

#### Scenario: Metadata extraction failure
- **GIVEN** a cached policy with corrupted or unparseable
  YAML
- **WHEN** `ExtractPolicyMetadata()` is called
- **THEN** a descriptive error MUST be returned
- **AND** the `get` command MUST NOT fail -- metadata
  extraction errors MUST be logged as warnings and the sync
  MUST still succeed

### Requirement: Get command prints post-sync summary

After successfully syncing a fresh policy, `complyctl get`
MUST print a human-readable summary to stderr showing the
policy's internal metadata.

The summary MUST include:
- Policy title and evaluator ID on one line
- Control count and assessment count on one line

The summary MUST be printed only for freshly fetched
policies or backfilled metadata (not for no-change syncs
where metadata already exists).

Metadata extraction is local-only (no network I/O) --
it reads from the already-cached OCI layout on disk.

#### Scenario: Fresh fetch prints summary
- **GIVEN** a workspace with one policy entry
- **WHEN** `complyctl get` fetches the policy for the
  first time
- **THEN** stderr MUST contain a summary line showing the
  policy title and evaluator, and a line showing control
  and assessment counts
- **AND** the summary MUST appear after the "done"
  confirmation

#### Scenario: No-change sync omits summary
- **GIVEN** a policy was previously fetched and is
  up-to-date with metadata already cached
- **WHEN** `complyctl get` runs and detects no digest
  change
- **THEN** no post-sync summary MUST be printed for that
  policy

#### Scenario: Metadata extraction fails gracefully
- **GIVEN** a policy was freshly fetched but metadata
  extraction encounters a parse error
- **WHEN** the extraction error is handled
- **THEN** a warning MUST be logged
- **AND** the sync MUST still report success ("done")
- **AND** no summary MUST be printed for that policy

### Requirement: List command displays policy metadata

`complyctl list` MUST display EVALUATOR and CONTROLS
columns in the policy table, sourced from the cached
`PolicyState` metadata in `state.json`.

Machine-readable `list` output (e.g., `--output json`) is
out of scope for this change.

#### Scenario: List with metadata available
- **GIVEN** policies have been fetched and metadata is
  cached in `state.json`
- **WHEN** `complyctl list` is run
- **THEN** the table MUST include EVALUATOR and CONTROLS
  columns showing the evaluator ID and control count for
  each policy

#### Scenario: List with no metadata (pre-existing cache)
- **GIVEN** policies were fetched by an older version of
  complyctl and `state.json` has no metadata fields
- **WHEN** `complyctl list` is run
- **THEN** the EVALUATOR and CONTROLS columns MUST show
  "-" for policies without cached metadata

#### Scenario: List with policy-id filter
- **GIVEN** multiple policies are cached with metadata
- **WHEN** `complyctl list --policy-id <id>` is run
- **THEN** only the matching policy MUST appear with its
  EVALUATOR and CONTROLS values

#### Scenario: Column order
- **GIVEN** policies are cached with metadata
- **WHEN** `complyctl list` is run
- **THEN** the table columns MUST appear in the order:
  POLICY ID, VERSION, EVALUATOR, CONTROLS, DIGEST,
  VERIFIED

## MODIFIED Requirements

### Requirement: PolicyState struct extended

Previously: `PolicyState` contained `Version`, `Digest`,
`EvaluatorID` (complypacks only), `LastUpdated`, `Verified`,
`SignerIdentity`, `Issuer`, `VerifiedAt`.

Now: `PolicyState` MUST additionally contain `PolicyTitle`
(string), `PolicyEvaluator` (string), `ControlCount` (int),
and `AssessmentCount` (int) fields with `omitempty` JSON
tags.

Note: The existing `EvaluatorID` field is used for
complypacks. The new `PolicyEvaluator` field is used for
policies. Both coexist on the same struct without conflict
because a given `PolicyState` entry is either a policy or a
complypack, not both.

### Requirement: SetPolicyMetadata preserves sync fields

The `State.SetPolicyMetadata()` method MUST read the
existing `PolicyState` entry for the given key and update
only the four metadata fields (`PolicyTitle`,
`PolicyEvaluator`, `ControlCount`, `AssessmentCount`). It
MUST NOT overwrite or zero out the sync fields (`Version`,
`Digest`, `EvaluatorID`, `LastUpdated`, `Verified`,
`SignerIdentity`, `Issuer`, `VerifiedAt`).

If the key does not exist in the `Policies` map, the
method MUST no-op (return without modification), since
metadata without sync fields is meaningless.

## REMOVED Requirements

None.
