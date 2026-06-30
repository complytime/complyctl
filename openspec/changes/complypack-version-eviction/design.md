## Context

`ComplypackCache.Store()` currently:
1. Creates a temp directory with `content.tar.gz` and `config.json`
2. Calls `os.RemoveAll` on the target version path
3. Calls `os.Rename` to atomically move temp → target

This leaves old version directories when the version field changes.

## Goals

- Ensure at most one version directory per evaluator-id after any `Store()` call
- Maintain atomic store semantics (no partial state on failure)
- No change to `Store()` signature or return type

## Non-Goals

- Adding a `complyctl cache clean` command (future enhancement)
- Tracking version history or rollback capability

## Decisions

### D1: Evict before atomic rename

Eviction of old versions happens after the temp directory is prepared but before the final rename. If eviction of an old directory fails, the store still proceeds (log warning) — the new version takes priority.

Sequence:
1. Prepare temp dir with content
2. List sibling version directories under `{cacheDir}/complypacks/{evaluator-id}/`
3. Remove all directories that are not the target version or hidden (`.tmp-*`)
4. `RemoveAll` target version path (existing logic)
5. `Rename` temp → target (existing logic)

### D2: Skip hidden/temp directories

Directories starting with `.` are skipped during eviction (these are temp dirs from in-flight writes).

### D3: Eviction errors are non-fatal

If removing an old version directory fails (permissions, etc.), emit a warning but proceed with the store. The primary goal (write new version) takes priority over cleanup.

## Risks / Trade-offs

- **Race condition**: If two `Store()` calls run concurrently for the same evaluator-id with different versions, both may evict each other. Mitigation: complypack sync is sequential per-evaluator in practice.
- **Disk space not freed on error**: If eviction fails, old directories persist. Acceptable — no worse than current behavior.
