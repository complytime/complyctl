## Context

`saveGenerationAndPrint()` at line 165 of `generate.go`:
1. Loads `state.json` from the cache directory
2. Reads policy digest from `state.GetPolicyState(repository)`
3. Builds `complypackDigestsByEvaluator(cacheState)` — reads ALL complypack digests from state
4. Saves generation state with those digests

The problem: step 3 reads digests from `state.json` regardless of whether the complypack content was actually used during generation.

## Goals

- Generation state complypack digests reflect what was actually passed to providers
- If complypack content was unavailable (path empty), record no digest for that evaluator
- Maintain backward compatibility for the happy path (content available)

## Non-Goals

- Adding complypack content verification (checksum validation)
- Changing the Generate RPC to report what it consumed
- Fixing `state.json` drift itself (that's a separate concern)

## Decisions

### D1: Filter digests by actual content availability

Instead of blindly recording all complypack digests from `state.json`, only include digests for evaluators whose complypack content path was successfully resolved (non-empty) during the generate call.

The generate flow already resolves complypack content paths via `LookupByEvaluatorID`. Pass the set of evaluator-ids that had actual content into `saveGenerationAndPrint`.

### D2: Thread content availability through

Add a parameter to `saveGenerationAndPrint`:
- `availableEvaluators []string` — evaluator-ids for which complypack content was found

Filter `complypackDigestsByEvaluator` output to only include entries in `availableEvaluators`.

### D3: Scan path unchanged

`scan.go` already calls `saveGenerationAndPrint` through a similar path. Apply the same filtering there for consistency.

## Risks / Trade-offs

- **Signature change**: `saveGenerationAndPrint` gains a parameter. Internal-only, no public API impact.
- **Regeneration on first run**: If complypack becomes available after a generate-without-complypack, the next scan correctly detects the digest mismatch and regenerates. This is the desired behavior.
