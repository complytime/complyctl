## Context

`complypackDigestsByEvaluator()` builds a map keyed by evaluator-id. With duplicate evaluator-ids, Go map semantics mean last-writer-wins with non-deterministic iteration order, causing silent data loss.

## Goals

- Detect duplicate evaluator-ids after complypack sync (when evaluator-ids are known from config.json)
- Fail fast with a clear error before generation state is written
- No false positives for valid configurations

## Non-Goals

- Validating evaluator-ids at config load time (evaluator-id is only known after fetching the artifact)
- Supporting intentional multi-repository evaluator-id sharing (not a valid use case)

## Decisions

### D1: Validate after sync, not at config load

Evaluator-ids are embedded in the complypack's `config.json`, which is only available after fetch. Validation must happen post-sync when `state.json` has all evaluator-ids populated.

### D2: Validation location

Add a `validateUniqueEvaluatorIDs(state *cache.State, complypacks []complytime.PolicyEntry) error` function called after `syncAllComplypacks` returns successfully. The `complypacks` parameter scopes validation to configured entries only (state may contain entries from previous configs). On error, `complyctl get` exits non-zero.

### D3: Error format

```text
Error: duplicate evaluator-id "opa" found in complypack entries:
  - ghcr.io/org-a/complypack-opa@v1
  - ghcr.io/org-b/complypack-opa@v2
remove one of the conflicting entries from complytime.yaml
```

## Risks / Trade-offs

- **Breaking existing configs**: If someone has duplicate evaluator-ids today (unknowingly), this will surface it as an error. This is intentional — the previous behavior was silently broken.
- **Post-sync validation**: Resources are spent fetching both complypacks before detecting the conflict. Acceptable — evaluator-id is only known after fetch.
