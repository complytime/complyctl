# Proposal

## Why

Provider evidence accumulates across scans, and users currently have
to locate and delete it by hand. GRCAUTO-286 needs an explicit way to
reclaim that space, including evidence collected before this feature.

Story: <https://redhat.atlassian.net/browse/GRCAUTO-286>.
Epic: <https://redhat.atlassian.net/browse/GRCAUTO-283>.

## What Changes

- Add `complyctl clean --older-than <days>d` and `--dry-run` for
  workspace-scoped cleanup of reports and their supporting evidence.
- Group managed EvaluationLog, OSCAL, SARIF, and Markdown reports with
  their referenced AMPEL, OPA, and OpenSCAP evaluation directories.
  Reports sharing an evaluation belong to the same cleanup group.
- Preflight every member before deleting anything in a group. If any
  member is too new, in use, inaccessible, unsafe, or unresolved, keep
  every report and all evidence in that group.
- Include existing accumulated evaluations and reports when ownership
  and relationships can be established safely, without migration or
  a new provider marker. Preserve ambiguous groups with a reason.
- Measure age from the directory's UTC collection-start timestamp.
  Require a positive whole number of 24-hour days and preserve entries
  exactly at the cutoff or newer.
- Delete a fully eligible group's evidence first, then its reports.
  An unexpected deletion failure stops that group, retains any reports
  not yet deleted, reports partial progress, and returns nonzero.
  Restoring already-deleted files is not required.
- Coordinate updated complyctl invocations through workspace locking,
  covering provider work, report publication, and cleanup. Refuse
  deletion when the workspace is busy or coordination fails.
- Give interrupted-scan errors an actionable manual recovery procedure:
  stop all writers, remove only the identified leftover activity
  record, and retry preview. Do not automatically kill recorded PIDs
  or discard unresolved activity records.
- Limit concurrent-operation support to participating complyctl
  invocations. Older hosts and direct provider calls against the same
  workspace during cleanup are unsupported.
- Keep changes in complyctl. Do not require provider source changes,
  an RPC addition, or a paired provider release.
- Support cleanup on both Linux and macOS, on amd64 and arm64,
  matching the host's release targets. The first increment MUST NOT
  treat macOS as an unsupported cleanup platform.

The user reports that the team lead who created GRCAUTO-286 confirmed
deleting reports and evidence together, with whole-group preflight.
This supersedes the issue's proposed exclusion of complyctl reports
and the earlier draft's report-preserving expiry policy. The user
also confirmed that rollback after an unexpected deletion failure is
not required. Record this as team-lead confirmation relayed by the
user, not as independently verified Jira approval or a Jira update.

Earlier user decisions retain day-based age rules, cleanup of existing
accumulation, Linux/macOS support, manual interrupted-scan recovery,
and the complyctl-only concurrency boundary. Independent older hosts
and direct provider callers remain outside that boundary.

Automatic cleanup, persistent retention settings, CLI-log expiry,
external or renamed report-copy discovery, archival, compression,
remote storage, cache eviction, generation cleanup, transactional
rollback, and deletion of unrecognized legacy layouts are excluded.

## Capabilities

### New Capabilities

- `evidence-cleanup`: Group-based expiry of workspace-local reports
  and provider evidence, preview, preflight, confined deletion, failure
  reporting, and coordination with complyctl-managed activity.

### Modified Capabilities

None. Existing generation and cross-repository mapping requirements
remain intact; ordinary scans continue preserving historical evidence.

## Impact

- CLI: command registration and help under `cmd/complyctl/cli/`.
- Services: focused cleanup and workspace-coordination packages under
  `internal/`, including managed-report parsing and relationship
  reconstruction, integrated with relevant command lifetimes.
- Compatibility: unchanged provider API and provider binaries.
- Existing host distributions: Linux and macOS, on amd64 and arm64;
  repository CI exercises Linux. Windows has no release target.
- Platform: Linux and macOS cleanup use platform-specific coordination
  and confined deletion with the same user-visible behavior. Baseline
  filesystems are Linux ext4, XFS, Btrfs, tmpfs, and macOS APFS.
  Unsupported filesystems and unsafe mount layouts must cause refusal,
  not an OS-wide exclusion of macOS. Windows remains outside scope.
- Documentation: evidence guidance, command reference, release notes,
  and structural guidance where required by repository conventions.
- Verification: isolated deletion fixtures, subprocess coordination
  and interrupted-operation tests on both operating systems, plus
  existing CI checks. Cross-compilation alone does not validate macOS
  behavior. No test may delete the user's accumulated evidence.
