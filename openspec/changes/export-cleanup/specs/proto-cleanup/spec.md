## Overview

Post-removal hygiene for the collector export infrastructure removed in PR #617.
Three independent mechanical tasks that prevent future wire-format accidents,
eliminate stale config noise, and correct the CRAP baseline.

## Functional Requirements

### FR-001: Proto field reservation

The `DescribeResponse` message in `api/plugin/plugin.proto` MUST contain:

```protobuf
reserved 6;
reserved "supports_export";
```

This prevents future field definitions from reusing number 6 or the name
`supports_export` with incompatible semantics.

### FR-002: Remove stale collector config

The workspace config file `.complytime/complytime.yaml` MUST NOT contain
the `collector:` block (including its `endpoint:` child). The Go parser
no longer reads this block; its presence is misleading.

### FR-003: Regenerate CRAP baseline

`.gaze/baseline.json` MUST NOT reference functions that no longer exist
in the codebase. After regeneration, every function listed in the baseline
MUST correspond to an existing function in the source tree.

## Non-Functional Requirements

- `buf lint` MUST pass after proto changes.
- `make proto` MUST produce no diff beyond the expected regeneration.
- No functional behavior change to any CLI command.

## Scenarios

### Given/When/Then

**FR-001:**
- Given the proto file has `reserved 6; reserved "supports_export";`
- When a developer adds field 6 to `DescribeResponse`
- Then `buf lint` / `protoc` rejects the definition

**FR-002:**
- Given `collector:` is removed from `.complytime/complytime.yaml`
- When `complyctl scan` is executed
- Then behavior is identical (no regression)

**FR-003:**
- Given `.gaze/baseline.json` is regenerated
- When `make crapload-check` runs
- Then no stale-function warnings appear
