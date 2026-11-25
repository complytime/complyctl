# Evidence Cleanup

`complyctl clean` expires accumulated compliance reports and the
evidence that supports them. Cleanup is limited to the selected
workspace and never needs registry access, credentials, provider
installation, or policy resolution. Use `complyctl clean --help` for the
authoritative flag list in your release.

## Choosing an age threshold

The age threshold is required and has no implicit default:

```bash
complyctl clean --older-than 30d --dry-run
complyctl clean --older-than 30d
complyctl --workspace /srv/compliance clean --older-than 30d
```

The value must be a positive whole number followed by `d`; one day is
24 hours. `0d`, negative, fractional, whitespace-padded, unsupported
units, and overflowing values are rejected before anything is changed.
A single UTC invocation time supplies the cutoff for the whole run.
The workspace follows the usual `--workspace` /
`COMPLYTIME_WORKSPACE` / current-directory precedence.

## Report and evidence grouping

Cleanup groups managed reports with the evaluations they reference:

- Reports are the EvaluationLog (YAML and JSON), Markdown, OSCAL, and
  SARIF variants written under `.complytime/scan/`.
- Evidence is the recognized `evaluation-<UTC-start>-<token>`
  directories beneath `.complytime/ampel/results/`,
  `.complytime/opa/results/`, and `.complytime/openscap/results/`.
- Relationships come from validated report content and provenance,
  not from filenames or modification times. Reports sharing any
  evaluation join one group, even when they cite different files in
  that evaluation.

A group is eligible only when **every** connected evaluation started
strictly before the cutoff. The age comes from the UTC collection-start
timestamp in the evaluation directory name, never from file timestamps.
Malformed, exact-cutoff, future, or newer timestamps retain their whole
group. Previously accumulated or incomplete evaluations follow the same
rules, without needing migration markers; a reportless evaluation can
expire only after a complete report inventory proves that no report
references it. Renamed or externally stored report copies are outside
the managed inventory, and retained reports are never rewritten to
point at newer evidence.

## Previewing a cleanup

`--dry-run` previews the same groups, cutoff, preflight checks, and
coordination checks that an execution uses. It reports the workspace,
cutoff, group membership, per-member status and reasons, and totals,
including groups that must be retained. It changes no workspace files:
no lock, activity record, configuration, debug log, report, or evidence
is created, and diagnostics go to the output streams. A preview does
not reserve or freeze groups for a later run; execution rediscovers and
rechecks everything at its own invocation time. No matches and ordinary
age exclusions are not errors.

## Expiration is irreversible

An execution removes all supporting evidence in an eligible group
before removing that group's managed reports. Whole-group preflight
must complete before the first deletion; if any member is newer,
unresolvable, unsafe, or inaccessible, the entire group is retained
byte-for-byte and the blocking member is named. Independent safe
groups can still be cleaned, but the command returns nonzero when any
group is blocked.

Preflight cannot guarantee that later I/O succeeds. If an unexpected
removal error occurs, cleanup stops that group, reports the exact
partial progress, and returns nonzero: already-deleted files are not
restored, and remaining reports may be left without their evidence.
No transactional rollback is promised, so preview carefully before
running without `--dry-run`.

## Safeguards and retained content

Cleanup deletes only inside the approved provider results roots and
managed report files. Symlinks, mount boundaries, and candidates whose
identity changes after discovery block the affected group. Logs,
configuration, scan inputs, caches, OPA working clones, generation
state, unrecognized layouts, and other workspaces are left untouched.

SARIF reports do not currently carry the evidence source provenance
needed to bound all relationships confidently. When a SARIF report
lacks that provenance, cleanup refuses workspace cleanup rather than
guessing; keep the original EvaluationLog and other report variants
until the provenance is available. Malformed reports with unbounded
evidence relationships are refused in the same way.

## Supported platforms and filesystems

Cleanup targets Linux and macOS on amd64 and arm64, using ordinary
local workspaces on supported local filesystems (including macOS APFS).
Unsupported filesystems or failed coordination cause refusal before
deletion, while existing non-cleanup commands keep their platform
compatibility. Host-side protection covers scans and generation started
by an updated complyctl with unchanged provider binaries. Older hosts
and providers invoked directly during cleanup are not protected by
these locks; do not run providers directly while cleanup is active.
Runtime verification for the supported platforms is tracked separately
from cross-compilation, so check your release notes for the platforms
covered by completed testing.

## Interrupted scans and recovery

Cleanup coordinates with scans, generation, and relevant artifact
removal in the same workspace. If another supported operation holds
access, cleanup or preview fails promptly without deleting anything.
If a host exits without confirming that its providers stopped, an
unresolved activity record keeps cleanup blocked even after the normal
lock is released. Elapsed time, a missing process, or absent provider
metadata does not clear that uncertainty, and there is no automatic
stale-record override.

The error names the workspace, the exact record paths, and the fact
that no evidence was deleted. To recover manually:

1. Stop every writer that may still be using the workspace and pause
   new scans. Process identifiers in the message are diagnostic only:
   do not use them as authorization to kill processes or clear records.
2. If shutdown is uncertain, restart the complete writer environment,
   not just the terminal, and confirm that no independent writers
   remain. Do not proceed if any may still be active.
3. Remove only the identified activity records, leaving unrelated
   records and workspace content in place, then retry
   `complyctl clean --older-than <days>d --dry-run`.

Removing a record does not override live workspace access held by an
active host, so cleanup still refuses while that host is running.

## See also

- [Evidence Source Provenance](EVIDENCE.md) -- how evidence coordinates
  and digests link reports to source artifacts
- [Man page](man/complyctl.md) -- complete CLI reference
