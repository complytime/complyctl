## Why

`ComplypackCache.LookupByEvaluatorID()` iterates version directories via `os.ReadDir` and returns the first match. Go's `ReadDir` returns entries in directory order, which is filesystem-dependent and non-deterministic. When multiple version directories coexist (e.g. `0.1.0/` and `0.2.0/`), the provider may receive outdated complypack content silently.

The root cause is that `ComplypackCache.Store()` only removes the exact version path being written. If the complypack's version field changes between releases, the old version directory remains on disk indefinitely with no eviction.

This is tracked in [#645](https://github.com/complytime/complyctl/issues/645) and [#646](https://github.com/complytime/complyctl/issues/646).

## What Changes

- `ComplypackCache.Store()` evicts all sibling version directories for the same evaluator-id before writing the new version, ensuring only one version exists at any time.
- `LookupByEvaluatorID()` remains unchanged but is now guaranteed to find at most one version directory.

## Capabilities

### New Capabilities
- `complypack-version-eviction`: On store, all prior version directories for the same evaluator-id are removed before writing the new version.

### Modified Capabilities
- `complypack-store`: Now atomically evicts old versions as part of the store operation.

### Removed Capabilities
- None.

## Impact

- `internal/cache/complypack.go` — `Store()` gains eviction of sibling version directories
- `internal/cache/complypack_test.go` — new tests for eviction behavior
- No CLI interface changes, no config changes

## Constitution Alignment

### I. Autonomous Collaboration

**Assessment**: PASS

The eviction is automatic and transparent. Users don't need to manually clean old versions.

### II. Composability First

**Assessment**: PASS

Eviction is scoped per evaluator-id. Different evaluators' caches are untouched. The change is internal to `Store()` with no signature change.

### III. Observable Quality

**Assessment**: PASS

The cache directory structure becomes deterministic: one evaluator-id → one version directory. `complyctl providers` already shows the cached version.

### IV. Testability

**Assessment**: PASS

Testable via `t.TempDir()` with pre-seeded old version directories. Verify they're removed after `Store()`.
