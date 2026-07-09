# Plugin API Contracts

**Version**: 1.0
**Date**: 2026-02-12
**Feature**: Gemara-Native Decoupled Workflow

## Overview

This directory contains the gRPC API contracts for communication between complyctl and compliance assessment plugins.

## Files

- `plugin.proto`: Protocol Buffer definition for the PluginService gRPC API

## API Versioning

The API version is specified in the protobuf package name (`complyctl.plugin.v1`) and the `api_version` field in `HealthCheckResponse`.

**Version Compatibility**:
- Plugins must implement the exact API version expected by complyctl
- Version mismatches result in plugin rejection during health check
- Future API versions will use new package names (`v2`, `v3`, etc.)
- Pre-release breaking changes within `v1` are acceptable while no external consumers exist (R29)

## Service Methods

### Generate

Prepares declarative policies based on general policy configuration from assessment plan. Called synchronously during `complyctl generate` command execution (or auto-generate within `complyctl scan`).

**Request** (`GenerateRequest`):
- `global_variables`: Global variables from workspace config top-level `variables` section (R48, R49). Workspace-scoped config (e.g., scan output directory). Sent once per request.
- `configurations`: Repeated `AssessmentConfiguration` entries (one per requirement — test variables from policy)
  - `plan_id`: Assessment plan ID
  - `requirement_id`: Requirement ID
  - `parameters`: Parameters map (test variables — key-value configuration per requirement)

**Response** (`GenerateResponse`):
- `success`: Success indicator
- `error_message`: Error message if preparation failed
- `plugin_info`: Plugin metadata

**Artifact Persistence**: Generated artifacts (e.g., XCCDF tailoring XML for OpenSCAP) are created by the plugin as side effects — written to the workspace directory during Generate RPC execution. `GenerateResponse` confirms success but does not return artifact content. The host (`complyctl`) persists a `GenerationState` recording the policy cache digest at generation time for freshness detection.

**Error Handling**:
- gRPC status codes: `UNAVAILABLE` (plugin error), `DEADLINE_EXCEEDED` (timeout), `INVALID_ARGUMENT` (malformed request)

**Usage**: Called during `complyctl generate` (or auto-generate within `complyctl scan`) to allow plugins to prep declarative policies based on general policy configuration extracted from assessment plan.

### Scan

Executes assessment checks for requirements across multiple targets and returns AssessmentLog for each requirement evaluated.

**Request** (`ScanRequest`):
- `targets`: Repeated `Target` entries (one per target to scan). Provider evaluates all requirements from Generate-time state (R47).
  - `target_id`: Target identifier from workspace configuration
  - `variables`: Target variables — per-target runtime config (credentials, profile, kubeconfig). Document expected variables in your provider README.

**Response** (`ScanResponse`):
- `assessment_logs`: Array of AssessmentLog entries (one per requirement evaluated)
- `plugin_info`: Plugin metadata
- `duration_seconds`: Execution time

**Note**: All targets configured in workspace config for the specified policy ID are scanned in a single Scan call.

**AssessmentLog Structure** (per requirement):
- `requirement`: EntryMapping with requirement ID
- `plan`: EntryMapping with plan ID (optional)
- `description`: Summary of assessment procedure
- `result`: Result enum (passed, failed, skipped, error)
- `message`: Additional context about assessment result
- `applicability`: Applicability conditions (array of strings)
- `steps`: Array of AssessmentStep entries (sequential actions taken)
- `steps_executed`: Number of steps executed
- `start`: Timestamp when assessment began
- `end`: Timestamp when assessment concluded (optional)
- `recommendation`: Guidance on addressing failed assessment (optional)
- `confidence_level`: ConfidenceLevel enum (low, medium, high, optional)

**AssessmentStep Structure**:
- `step_id`: Step ID or name
- `description`: Step description
- `result`: Result enum (passed, failed, skipped, error)
- `message`: Step message
- `timestamp`: Step timestamp
- `sequence`: Step order/sequence number

**Error Handling**:
- gRPC status codes: `UNAVAILABLE` (plugin error), `DEADLINE_EXCEEDED` (timeout), `INVALID_ARGUMENT` (malformed request)

**Usage**: Called during `complyctl scan` to execute assessment checks and return detailed AssessmentLog entries for each requirement.

### HealthCheck

Verifies plugin availability and API version compatibility.

**Request**: Empty

**Response**:
- `healthy`: Plugin is operational
- `version`: Plugin version string
- `error_message`: Error details if not healthy
- `required_global_variables`: Variable names the provider requires in workspace config `variables` section (R51). Doctor validates these keys exist. Empty list = no requirements.
- `required_target_variables`: Variable names the provider requires in each relevant target's `variables` section (R51). Doctor validates these keys exist for targets whose policies route to this provider. Empty list = no requirements.

**Usage**: Called during workspace initialization, plugin discovery, and `complyctl doctor` diagnostics. Doctor uses `required_*_variables` fields to validate workspace config completeness before generate/scan time (R51).

## Result Enum Values

- `RESULT_PASSED`: Assessment check succeeded
- `RESULT_FAILED`: Assessment check failed (compliance violation)
- `RESULT_SKIPPED`: Assessment check skipped (e.g., not applicable due to tailoring)
- `RESULT_ERROR`: Assessment check encountered an error during execution

## ConfidenceLevel Enum Values

Mirrors `go-gemara` `ConfidenceLevel` type (R29). 1:1 mapping — no lossy conversion.

- `CONFIDENCE_LEVEL_NOT_SET`: Default/initial state (proto zero value)
- `CONFIDENCE_LEVEL_UNDETERMINED`: Confidence could not be determined
- `CONFIDENCE_LEVEL_LOW`: Low confidence in result
- `CONFIDENCE_LEVEL_MEDIUM`: Moderate confidence in result
- `CONFIDENCE_LEVEL_HIGH`: High confidence in result

## Three-Tier Variable Model (R48)

The scanning interface protocol distinguishes three variable tiers:

| Tier | Name | RPC | Proto field | Source | Owner |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | Global variables | Generate | `GenerateRequest.global_variables` | Workspace config top-level `variables` | System admin |
| 2 | Test variables | Generate | `AssessmentConfiguration.parameters` | Layer 3 Gemara policy | Policy author |
| 3 | Target variables | Scan | `Target.variables` | Workspace config `targets[].variables` | System admin |

Global variables provide workspace-scoped configuration (scan output directory, shared paths). Test variables configure *what* a test checks (e.g., password hashing algorithm). Target variables configure *how* to reach the target and *how* to run the scan (credentials, profile, kubeconfig). Providers declare required global and target variable *names* via `HealthCheckResponse` fields (R51) for `complyctl doctor` pre-flight validation. Documentation of valid *values* remains out-of-band (provider README).

## Evaluation Log Levels

- `"info"`: Informational message
- `"warn"`: Warning (non-fatal issue)
- `"error"`: Error (assessment failure detail)

## Plugin Implementation Requirements

1. **gRPC Server**: Plugin must implement `PluginService` gRPC server (Generate, Scan, HealthCheck)
2. **Health Check**: Must respond to `HealthCheck` within 1 second
3. **API Version**: Must report correct API version in `HealthCheckResponse`
4. **Error Handling**: Must return appropriate gRPC status codes
5. **Timeout**: Must complete `Scan` within configured timeout (default: 5 minutes)
6. **Evaluator ID**: Determined from plugin executable name (remove `complyctl-provider-` prefix). No GetCapabilities call needed. Verify via `complyctl providers`.

## Code Generation

Generate Go code from protobuf using buf:

```bash
# From api/proto directory
buf generate
```

**buf Configuration**:
- `buf.yaml`: Workspace configuration (linting, breaking change detection)
- `buf.gen.yaml`: Code generation plugins (Go gRPC)
- Generated code output: `pkg/plugin/api/`

**Setup**:
1. Install buf: <https://buf.build/docs/installation>
2. Configure `buf.yaml` and `buf.gen.yaml` in `api/proto/`
3. Run `buf generate` to generate Go code

**References**:
- buf documentation: <https://buf.build/docs>
- compliance-to-policy-go buf setup: <https://github.com/oscal-compass/compliance-to-policy-go/blob/main/buf.yaml>

## Example Plugin Implementation

See `pkg/plugin/examples/` for reference plugin implementations.
