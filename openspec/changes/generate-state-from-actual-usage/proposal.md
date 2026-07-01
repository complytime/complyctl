## Why

`complyctl generate` invokes the provider Generate RPC without a freshness check, then calls `saveGenerationAndPrint()` which reads `state.json` for current policy/complypack digests and records them in the generation state file. If `state.json` has drifted from actual cache contents (e.g. cache directory manually deleted), the recorded digests won't match what was actually used during generation.

On the next `complyctl scan`, `IsFresh()` compares recorded digests against current `state.json`. If they match (both stale), scan skips regeneration even though workspace artifacts may be based on different content.

This is tracked in [#648](https://github.com/complytime/complyctl/issues/648).

## What Changes

- `saveGenerationAndPrint()` records the complypack content path that was actually passed to the provider. When the path was empty (no complypack available), the recorded complypack digest is empty/absent — forcing regeneration on next scan when the complypack becomes available.
- Alternatively: verify complypack cache existence before reading `state.json` digests.

## Capabilities

### New Capabilities
- `generation-state-accuracy`: Generation state reflects what was actually used during generation, not what `state.json` claims.

### Modified Capabilities
- `generate`: `saveGenerationAndPrint` validates complypack content was actually available before recording its digest.

### Removed Capabilities
- None.

## Impact

- `cmd/complyctl/cli/generate.go` — `saveGenerationAndPrint()` logic change
- `cmd/complyctl/cli/scan.go` — potentially `saveGenerationAndPrint` reuse
- Generation state test updates

## Constitution Alignment

### I. Autonomous Collaboration

**Assessment**: PASS

Generation state becomes self-consistent — what's recorded is what was used.

### II. Composability First

**Assessment**: PASS

Change is localized to `saveGenerationAndPrint`. Callers are unaffected.

### III. Observable Quality

**Assessment**: PASS

Generation state files accurately represent the inputs that produced the artifacts, enabling reliable freshness detection.

### IV. Testability

**Assessment**: PASS

Testable by mocking scenarios where complypack content path is empty vs populated.
