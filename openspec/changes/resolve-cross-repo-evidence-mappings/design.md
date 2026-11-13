# Design

## Context

See `proposal.md` for motivation and the capability delta for FR-001
through FR-008. The shell harness already uses jq and isolated home and
workspace directories. Its Ampel scan currently checks two filename
globs; its generated-policy checks already resolve a target index and
fail closed rather than falling back from a malformed index.

Both current complyctl and v1.2.0 support JSON EvaluationLogs and print
`Evaluation log written:` with a path and target. Evidence lives under
`evaluations[].assessment-logs[].evidence[].source`; references live
under `metadata.mapping-references`. The log's policy identity is the
resolved repository, not necessarily the workspace policy's alias.

The exporter and schema-validation script discover only YAML files.
The formatter uses second-resolution report filenames, so rapid scans
can reuse a report pathname. These observations shape the harness,
not changes to production formatters.

## Goals / Non-Goals

**Goals:**

- Centralize evidence checks so offline and live cases exercise the
  same failure behavior without losing shell assertion counters.
- Keep host provenance explicit and fixture ownership unambiguous.
- Support the released host without a new launcher, protocol field,
  YAML parser, or general-purpose evidence retrieval subsystem.

**Non-Goals:**

- Change provider retention, evidence identities, or file permissions.
- Fix production report-name collisions, change output formats, or
  introduce automatic evidence cleanup or packaging.
- Change the released-host resolver or implement providers-side CI.

## Decisions

### Separate harness ownership from host binary selection

Keep the script's repository root as the source of assertions and
`tests/cross-repo/testdata`. Add one optional absolute directory input,
`COMPLYCTL_BIN_DIR`, defaulting to that root's `bin/`. Select both
`complyctl` and `mock-oci-registry` there and require each executable.
Reject an explicitly empty, relative, missing, or incomplete selection;
never fall back to one default binary or build a substitute.

This avoids independent overrides accidentally mixing host revisions.
It also avoids copying the script into the released checkout, where
its apparent root would change fixture ownership. The normal local
Make target keeps its build behavior; callers testing external hosts
invoke the script or the providers' existing bounded Make target after
building their selected host explicitly.

The mock registry comes from the host revision under test. Source
comparison on 2026-11-13 found no difference between local v1.2.0 and
main in `cmd/mock-oci-registry` or `tests/cross-repo/testdata`. Treat
that as preparation evidence, not proof of a completed integration run.
Future fixture incompatibility must fail rather than select a newer
registry silently.

### Select the exact scan output before inspecting evidence

Run the Ampel scan with explicit `--log-format json`. Extract its one
reported log path for `complyctl-repo`, verify it stays within
the workspace scan directory, parse it with jq, and check repository
identity and target. Require the expected assessment requirement,
`block-force-push`, before selecting evidence from that assessment.

Copy the selected JSON log immediately into test-owned snapshot storage
when testing repeated evaluations. Identify logs by invocation, not
mtime, filename sorting, or the first matching log in the scan tree.
Do not parse terminal summary columns as the evidence data model.

### Verify the complete source-to-file chain

Use focused Bash/jq helpers with explicit nonzero error results. For
each Ampel and Snappy source on the expected assessment:

1. Validate required field types and nonempty source values.
2. Resolve relative coordinates from the selected workspace; accept
   absolute legacy coordinates only inside the same workspace.
3. Canonicalize existing paths, reject traversal/symlink escapes, and
   require readable, nonempty regular files before hashing.
4. Require a supported `sha256:` digest with 64 hexadecimal digits and
   compare it against the complete file's actual digest.
5. Resolve the reference ID to exactly one metadata mapping-reference.
   Validate an absolute local `file:///` URL with correct escaping and
   require its resolved path to equal the coordinate's resolved path.
   Reject remote authorities, queries, fragments, malformed escapes,
   and non-file schemes. Cover spaces, `%`, and Unicode in fixtures.
6. Apply the existing Ampel policy-ID content check to the selected
   Ampel file. Do not search other files if any link fails.

Provider IDs currently distinguish attestation roles with `ampel_` and
`snappy_` prefixes; retain that role distinction without depending on
logical suffixes, evaluation tokens, directory depth, or timestamps.
Do not deduplicate by digest. Evidence may legitimately recur across
assessment logs, but separate expected instances need separate IDs and
coordinates. Multiple metadata references with one ID are ambiguous.

Using jq avoids adding a YAML parser. File URL comparison is limited
to local evidence locators; it must not perform network retrieval or
evaluate strings as shell commands. Shared helper code should live
under `tests/cross-repo/`, not production packages.

### Separate deterministic regressions from live provider validation

Add an offline shell regression entry point under `tests/cross-repo/`
and a dedicated Make target, `test-cross-repo-evidence`. Generate JSON
fixtures and temporary artifact trees without credentials, provider
binaries, or network access. Reuse helpers rather than reimplementing
their assertions in tests. Ensure expected failures do not accidentally
increment or hide the live harness's failure counters.

Cover both filename layouts, stale and wrong-target logs, missing or
duplicate references, missing fields/files, modified bytes, bad digest
syntax, mismatched equal-content locators, path escapes, encoded file
URLs, and missing Ampel requirement content. Include nearby decoys and
supporting metadata without selecting them as evidence.

For history fixtures, save log A, publish evaluation B, then validate
both snapshots and A's original digest. Include equal-content separate
instances and a negative case that deliberately overwrites A. A fixture
with multiple expected instances must reject collapsed references,
while accepting legitimate reuse across assessments.

These fixtures prove checker behavior, not provider retention. Preserve
the normal single-scan live pipeline so older provider layouts remain
valid when their returned mappings are valid. Before closing retention
verification, also exercise two actual evaluations from the retained
provider build in one workspace, preserve both real JSON logs, and
apply the same historical checks. Report that separately from fixture
results; no aliases or skipped assertions may manufacture a pass.

### Keep schema checks on the actual JSON logs

Extend export and schema discovery to both
`evaluation-log-*.json` and `evaluation-log-*.yaml`. Keep the current
Gemara module, definition, completeness check, and failure behavior.
No JSON-to-YAML conversion or extra scan is needed because CUE accepts
JSON directly. Add positive mixed-format/JSON-only cases and negative
invalid/absent-output coverage without fetching a different schema.

Wire the offline evidence target into the existing cross-repository
workflow before credentialed execution. Preserve existing timeouts,
job identities, permissions, failure propagation, and schema checks.
Any change classified as a quality/governance gate must stop for
maintainer handling rather than be altered by the implementation agent.

## Risks / Trade-offs

- [Terminal path message changes] -> Cover extraction against v1.2.0
  and current output, fail closed, and keep selection target-specific.
- [Equal hashes hide wrong locators] -> Compare resolved paths as well
  as bytes; never use digest alone as instance identity.
- [Shell helpers accept unsafe paths] -> Check canonical workspace
  containment and URL syntax before reading evidence; test escapes.
- [Snapshots mask report overwrite bugs] -> State that snapshots test
  harness history checks, not production filename retention guarantees.
- [Offline checks overstate coverage] -> Record fixture, actual host,
  live provider, and schema results as separate verification evidence.
- [Credentialed acceptance is unavailable] -> Record the prerequisite
  blocker and leave the corresponding verification task incomplete.

## Migration Plan

1. Commit and push the reviewed planning artifacts before source edits.
2. Implement helpers, regressions, host selection, and schema discovery
   in this repository only; preserve the default local invocation.
3. Run the offline target, existing generation-index fixtures, shell
   lint, affected schema tests, and applicable workflow-derived checks.
4. Validate current and unchanged v1.2.0 hosts with the same reviewed
   harness and provider build; record revisions and tool versions.
5. Run retained-provider repeat-evaluation acceptance and document any
   unavailable live prerequisite without marking it complete.
6. Merge this PR, then hand its full commit and interface documentation
   to the separately reviewed providers change. No host release is
   needed. Reverting this change does not modify retained files.

Assess documentation impact during implementation. Document the binary
override and offline target in the existing testing/contributor
guidance and reflect added commands in `AGENTS.md` only if needed.
