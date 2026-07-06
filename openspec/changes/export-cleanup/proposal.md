## Why

PR #617 removed the collector export infrastructure (OTEL export, `--format otel`,
`CollectorConfig`, `ExportRequest`/`ExportResponse` messages). To maintain
single-concern scope (Constitution III), cosmetic/hygiene items were deferred to
a follow-up. These items are non-blocking but create technical debt:

1. **Proto field reuse risk** — removed field number 6 (`supports_export`) in
   `DescribeResponse` can be accidentally reused with different semantics,
   causing wire-format incompatibility.
2. **Stale config noise** — `collector:` block in `.complytime/complytime.yaml`
   is silently ignored at runtime, confusing users reading the config.
3. **CRAP baseline drift** — `.gaze/baseline.json` references 17 deleted
   functions, inflating the baseline and masking real regressions.

This is tracked in [#650](https://github.com/complytime/complyctl/issues/650).

## What Changes

- `api/plugin/plugin.proto`: add `reserved 6; reserved "supports_export";` to
  `DescribeResponse`; regenerate Go bindings via `make proto`.
- `.complytime/complytime.yaml`: remove the stale `collector:` block.
- `.gaze/baseline.json`: regenerate via `make crapload-baseline`.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `proto-api`: Field reservation prevents accidental reuse of removed field.

### Removed Capabilities
- None.

## Impact

- `api/plugin/plugin.proto` — reserved field + name
- `api/plugin/*.pb.go` — regenerated (no functional change)
- `.complytime/complytime.yaml` — 2 lines removed
- `.gaze/baseline.json` — regenerated

## Constitution Alignment

### I. Autonomous Collaboration

**Assessment**: PASS

Mechanical cleanup with clear acceptance criteria. No ambiguity.

### II. Composability First

**Assessment**: PASS

No interface changes. Reserved statements are additive and backward-compatible.

### III. Observable Quality

**Assessment**: PASS

CRAP baseline becomes accurate. Config file reflects actual capabilities.

### IV. Testability

**Assessment**: PASS

`make proto` + `make crapload-baseline` are idempotent. `buf lint` validates proto.
