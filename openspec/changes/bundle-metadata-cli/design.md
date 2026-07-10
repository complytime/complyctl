## Context

After `complyctl get` fetches a Gemara bundle from an OCI
registry, the CLI retains only OCI-level metadata (digest,
version, verification status) in `state.json`. All policy
content metadata (title, evaluator, controls, assessments) is
discarded until `complyctl scan` parses the cached YAML via
`policy.Resolver.ResolvePolicyGraph()`.

This means `complyctl list` shows only 4 columns (POLICY ID,
VERSION, DIGEST, VERIFIED) with no insight into what the
policy actually contains. Users must run `oras blob fetch` or
similar tools to discover the evaluator type, control count,
or policy title.

The relevant code paths are:

1. `syncSinglePolicy()` in `cmd/complyctl/cli/get.go` calls
   `sync.SyncPolicy()` and updates `state.json` with version,
   digest, and verification status. It never opens the policy
   layers.
2. `policy.Resolver.ResolvePolicyGraph()` in
   `internal/policy/resolver.go` parses all Gemara YAML and
   builds a `DependencyGraph` with evaluator ID, assessments,
   controls, and guidelines. This is only called during `scan`.
3. `listOptions.run()` in `cmd/complyctl/cli/list.go` reads
   `complytime.yaml` and `state.json` to build the table. It
   uses `policy.Loader` only for version tags, never for
   content.
4. `parsePolicyLayer()` in `internal/policy/resolver.go`
   unmarshals the full `gemara.Policy` struct (which has
   `Title`, `Metadata`, etc.) but `extractFromGemaraPolicy()`
   only extracts `EvaluatorID`, `Assessments`, and `Timeline`
   -- discarding `Title` and other display-oriented metadata.

## Goals / Non-Goals

### Goals

- Surface policy metadata (title, evaluator, control count,
  assessment count) via `complyctl list` and `complyctl get`
  summary
- Cache metadata in `state.json` at sync time so `list` does
  not require YAML parsing or OCI store access
- Provide a lightweight extraction path that does not build a
  full `DependencyGraph`
- Backfill metadata for pre-existing caches on upgrade

### Non-Goals

- Pre-pull discovery (`complyctl inspect <url>`) -- deferred
  to follow-up issue
- Policy title in `list` table (too long for terminal width)
- ID validation (warning when config `id` differs from bundle
  policy ID) -- separate concern
- Changes to scan output, report formatters, or provider
  protocol
- Changes to complypack metadata (already has `EvaluatorID`
  in state)
- Machine-readable `list` output (e.g., `--output json`) --
  composability enhancement deferred to separate change

## Decisions

### D1: Cache metadata in `state.json`

**Decision**: Add `policy_title`, `policy_evaluator`,
`control_count`, and `assessment_count` fields to
`PolicyState` with `omitempty` JSON tags. Populate at sync
time when a fresh policy is fetched.

**Rationale**: `state.json` is already the metadata store
for policies. `list` already reads it. Adding fields is
backward compatible (Go's JSON unmarshalling ignores unknown
fields, `omitempty` avoids noise for old entries). The
alternative (on-demand parsing) would require opening OCI
stores, resolving manifests, and parsing YAML for every
policy on every `list` call.

**Staleness risk**: Near-zero. The OCI layout directory is
an internal implementation detail managed exclusively by
`complyctl get`. The metadata is display-only -- `scan`
always re-parses fresh YAML via the Resolver. Any
`complyctl get` run refreshes the metadata.

### D2: Lightweight `ExtractPolicyMetadata()` method

**Decision**: Add `ExtractPolicyMetadata(policyID, version)`
to `policy.Resolver` that returns a `PolicyMetadata` struct
with `Title`, `EvaluatorID`, `ControlCount`,
`AssessmentCount`. The `policyID` parameter follows the
existing `ResolvePolicyGraph(policyID, version)` convention
where it represents the OCI repository path used as the
cache key (not the user-facing `EffectiveID()` alias).

**Rationale**: Reuses existing YAML parsing functions
(`parsePolicyLayer`, `parseControlCatalog`) without building
a full `DependencyGraph`. Avoids parsing guidance catalogs
(not needed for display metadata). The method follows the
same bundle-vs-split-layer detection pattern as
`ResolvePolicyGraph()` but with a smaller output struct.

**Alternative considered**: Extending `ResolvePolicyGraph()`
to return metadata. Rejected because it would couple display
metadata to the scan pipeline and parse guidance catalogs
unnecessarily.

**Alternative considered**: Extracting metadata in `get.go`
directly (inline parsing). Rejected because it would
duplicate YAML unmarshalling logic that already lives in the
`policy` package, violating DRY.

### D3: Post-sync summary on stderr

**Decision**: After a fresh policy fetch, print a two-line
summary to stderr:

```
  Policy: <title> (<evaluator>)
  Controls: <N> | Assessments: <M>
```

When the title is empty, omit the title portion and show
only the evaluator. When the evaluator is empty
(multi-evaluator policy), omit the evaluator parenthetical.

**Rationale**: Gives immediate "what did I pull" feedback.
Matches existing stderr convention (all `get` output goes
to stderr). Two lines keep it concise. The title + evaluator
combination tells users whether they pulled the right policy
for their provider setup.

**Alternative considered**: Printing a full table with all
metadata. Rejected as too verbose for a fetch operation --
the detailed view is `list`.

### D4: `list` columns: EVALUATOR and CONTROLS

**Decision**: Add two columns to `complyctl list`:
- EVALUATOR: evaluator ID (e.g., "openscap", "ampel", "opa")
- CONTROLS: control count (e.g., "42")

Column order: POLICY ID, VERSION, EVALUATOR, CONTROLS,
DIGEST, VERIFIED.

**Rationale**: Evaluator ID is the most actionable metadata
-- it tells users which provider they need installed. Control
count gives a sense of policy scope. Both are short values
that fit in a terminal table.

**Alternative considered**: Including POLICY NAME (title)
column. Rejected because policy titles are often 50+
characters (e.g., "CIS Fedora Linux - Level 1 Workstation
Policy") and would blow out terminal width. The title is
shown in the `get` summary instead.

**Alternative considered**: Including ASSESSMENTS column.
Deferred -- assessments and controls are often 1:1, and
adding too many columns makes the table hard to read. Can
be added later if needed.

### D5: Metadata extraction failure is non-fatal

**Decision**: If `ExtractPolicyMetadata()` fails (e.g.,
corrupted YAML in cache), the error is logged as a warning
and the sync still succeeds. The metadata fields remain at
zero values, and `list` shows "-".

**Rationale**: Metadata is display-only. A parse failure
should not prevent a successful OCI sync from being recorded.
The policy can still be used for `scan` (which has its own
error handling for parse failures). This follows the existing
pattern where unverified policies emit a NOTE but do not fail
the sync.

### D6: Naming `PolicyEvaluator` vs reusing `EvaluatorID`

**Decision**: Use `PolicyEvaluator` as the new field name,
not reusing the existing `EvaluatorID` field.

**Rationale**: `EvaluatorID` already exists on `PolicyState`
and is used exclusively for complypack entries (populated
from the complypack config). A policy entry and a complypack
entry are stored in separate maps (`Policies` vs
`Complypacks`), but they share the same `PolicyState` struct.
Using a distinct field name avoids semantic confusion and
makes intent clear: `EvaluatorID` is the complypack's
evaluator binding, while `PolicyEvaluator` is the policy's
declared evaluator from the Gemara YAML.

### D7: State update ordering and double-save

**Decision**: Metadata extraction MUST happen after
`SyncPolicy()` returns. The call sequence is:

1. `SyncPolicy()` -- internally calls
   `UpdatePolicyStateWithVerification()` and `SaveState()`
2. `ExtractPolicyMetadata()` -- parses cached YAML
3. `SetPolicyMetadata()` -- updates metadata fields on
   existing `PolicyState` entry (read-modify-write)
4. `SaveState()` -- second save with metadata included

**Rationale**: `UpdatePolicyStateWithVerification()` creates
a new `PolicyState` struct and replaces the map entry,
zeroing out any previously set metadata. By running metadata
extraction *after* sync, we ensure the metadata is added to
the freshly created entry rather than being overwritten.
The double-save of `state.json` is intentional and
acceptable for a small JSON file (~1KB typical).

### D8: Upgrade backfill for pre-existing caches

**Decision**: When `complyctl get` runs and `SyncPolicy()`
returns `fetched=false` (no digest change), check whether
the existing `PolicyState` has empty metadata fields
(`PolicyEvaluator == ""`). If so, run metadata extraction
and save -- this backfills metadata for policies cached by
older complyctl versions without requiring a manual cache
clear.

**Rationale**: Without backfill, upgrading users would see
"-" in EVALUATOR and CONTROLS columns indefinitely for
unchanged policies, requiring an undocumented manual cache
clear to fix. The backfill is a one-time cost per policy
(subsequent no-change syncs skip it because metadata is
already populated).

### D9: Resolver instantiation in get command

**Decision**: Create a `policy.Resolver` once in
`syncAllPolicies()` (via `policy.NewLoader(cacheMgr)` +
`policy.NewResolver(loader)`) and pass it to
`syncSinglePolicy()` as a parameter.

**Rationale**: Follows the existing pattern in `scan.go`
where the resolver is created once per command invocation.
Avoids creating a new `Resolver` per policy in the loop,
which would be wasteful (though harmless since `Resolver`
is stateless).

## Risks / Trade-offs

- **[Backward compatibility]** Adding fields to
  `PolicyState` is safe due to `omitempty` JSON tags and
  Go's permissive unmarshalling. Older complyctl versions
  reading a new `state.json` will silently ignore unknown
  fields.

- **[Parse cost at sync time]**
  `ExtractPolicyMetadata()` parses the policy and catalog
  YAML layers. This adds ~10ms per policy to the `get`
  operation (local I/O only, no network), which is
  negligible compared to network I/O for OCI fetch. The
  parsing is only done for freshly fetched policies and
  one-time backfill.

- **[Double-save of state.json]** `SyncPolicy()` saves
  state internally, then metadata extraction triggers a
  second save. This is two writes of a small JSON file
  (~1KB) per fresh policy -- acceptable and simpler than
  modifying `SyncPolicy()`'s internal save behavior.

- **[Multi-evaluator policies]** Policies with assessment
  plans bound to different evaluators will show "-" in the
  EVALUATOR column. This matches the existing behavior of
  `extractFromGemaraPolicy()` which sets `EvaluatorID` to
  empty string for multi-evaluator policies. Documenting
  this edge case is sufficient -- multi-evaluator policies
  are uncommon in practice.
