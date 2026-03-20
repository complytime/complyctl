# Implementation Plan: ComplyBeacon Evidence Export

**Branch**: `003-complybeacon-export` | **Date**: 2026-03-10
**Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/003-complybeacon-export/spec.md`

## Summary

Add `otel` as an output format on the existing `complyctl scan`
command. When `--format otel` is used, the scan executes
normally and then orchestrates evidence export to a Beacon
OTEL collector. The scan command discovers which scanning
providers support export via the `Describe` RPC, then calls
a new `Export` RPC on each capable plugin, passing collector
configuration from `complytime.yaml`. Plugins use ProofWatch
to emit `GemaraEvidence` as OTLP log records directly to the
collector — complyctl does not touch the evidence data, only
orchestrates.

Changes span four areas: (1) extend the Plugin gRPC contract
with `Export` RPC and `supports_export` capability,
(2) extend the plugin SDK (`pkg/plugin`) with routing and
client/server support, (3) extend the `scan` CLI command to
handle the `otel` format, (4) add `collector` config to
`WorkspaceConfig` and doctor validation.

## Technical Context

**Language/Version**: Go 1.24.x (matching parent go.mod)
**Primary Dependencies**:
- `pkg/plugin` (in-tree) — scanning provider gRPC interface
- `hashicorp/go-plugin` (v1.7.0) — gRPC plugin hosting
- `hashicorp/go-hclog` (v1.6.3) — structured logging
- `spf13/cobra` (v1.10.2) — CLI command framework
- `gemaraproj/go-gemara` (v0.0.1) — Gemara types
- `google.golang.org/grpc` (v1.76.0) — gRPC transport
- `google.golang.org/protobuf` (v1.36.10) — proto codegen
**New Dependencies in complyctl**: `golang.org/x/oauth2`
for OIDC client credentials token exchange. complyctl does
not import ProofWatch or OTEL SDK — those dependencies are
added by plugins that implement Export.
**New Dependencies in plugins**: Plugins implementing Export
add `github.com/complytime/complytime-collector-components/proofwatch` and
transitive OTEL SDK dependencies.
**Storage**: Filesystem (workspace directory for scan state)
**Testing**: `go test -race -v ./...` with testify assertions
**Target Platform**: Linux (primary), macOS (development)
**Constraints**: One new dependency in complyctl core
(`golang.org/x/oauth2` for OIDC token exchange).
ProofWatch/OTEL only in plugins that opt in.

## Constitution Check

*GATE: Must pass before implementation. Re-checked post-design.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Single Source of Truth | PASS | Collector config centralized in `complytime.yaml` `collector` section. No magic strings — endpoint and auth read from config and passed via gRPC. |
| II. Simplicity & Isolation | PASS | Export logic is a well-isolated post-scan step within `scan.go`, triggered only by `--format otel`. Plugin SDK extension follows existing RouteScan pattern. No OTEL/ProofWatch in complyctl core — isolation between orchestrator and emitter. |
| III. Incremental Improvement | PASS | Self-contained addition: new proto RPC, SDK extension, format option, config field. No changes to existing scan/generate/doctor logic beyond additive doctor check and the new format path. |
| IV. Readability First | PASS | `RouteExport` mirrors `RouteScan` naming. `CollectorConfig` struct is explicit. `supports_export` is a clear boolean capability. |
| V. Do Not Reinvent the Wheel | PASS | Reuses ProofWatch (existing library) for OTEL emission. Reuses go-plugin gRPC transport. Reuses `golang.org/x/oauth2` for OIDC token exchange — no custom OAuth2 client. No custom OTLP client in complyctl. |
| VI. Composability | PASS | `--format otel` composes naturally with the existing scan flow. Scan produces results, the otel format path ships them. Same command, one step. |
| VII. Convention Over Configuration | PASS | Export is optional — plugins default to `supports_export: false`. Collector config is optional in `complytime.yaml` — only needed when using `--format otel`. |

## Project Structure

### Documentation (this feature)

```text
specs/003-complybeacon-export/
├── spec.md
├── plan.md              # This file
├── tasks.md             # Generated next
└── checklists/
    └── requirements.md
```

### Source Code Changes (repository root)

```text
api/plugin/
├── plugin.proto             # MODIFIED — add Export RPC, ExportRequest,
│                            #   ExportResponse, CollectorConfig;
│                            #   add supports_export to DescribeResponse
├── plugin.pb.go             # REGENERATED via buf
└── plugin_grpc.pb.go        # REGENERATED via buf

pkg/plugin/
├── client.go                # MODIFIED — add Export method to Client,
│                            #   ExportRequest/ExportResponse domain types,
│                            #   CollectorConfig domain type
├── server.go                # MODIFIED — add Export to grpcServer,
│                            #   proto ↔ domain mapping
├── manager.go               # MODIFIED — add RouteExport method,
│                            #   add Export to Plugin interface
├── plugin.go                # UNCHANGED
├── discovery.go             # UNCHANGED
└── initialization.go        # UNCHANGED

cmd/complyctl/cli/
├── scan.go                  # MODIFIED — add otel format handling,
│                            #   export orchestration after scan
└── root.go                  # UNCHANGED

internal/complytime/
├── config.go                # MODIFIED — add CollectorConfig struct
│                            #   to WorkspaceConfig, add Validate rules
├── consts.go                # MODIFIED — add export-related constants
└── workspace.go             # UNCHANGED

internal/doctor/
└── doctor.go                # MODIFIED — add collector reachability check
```

### Package Responsibilities

| Package | Change | Responsibility |
|---------|--------|----------------|
| `api/plugin` | Modified | Proto contract — new `Export` RPC, messages, `supports_export` field |
| `pkg/plugin` | Modified | Plugin SDK — `RouteExport`, domain types, client/server gRPC mapping |
| `cmd/complyctl/cli` | Modified | `scan.go` — add `otel` format path: validate collector config, resolve OIDC token, call `RouteExport` on capable plugins after scan, display export summary |
| `internal/complytime` | Modified | `CollectorConfig` struct, config validation, constants |
| `internal/doctor` | Modified | Collector endpoint reachability check (non-blocking) |

### What complyctl Does NOT Own

| Concern | Owner |
|---------|-------|
| OTLP emission | Plugin (via ProofWatch) |
| OTEL SDK setup | Plugin |
| Evidence format | Plugin (GemaraEvidence) |
| Enrichment | TruthBeam (in Beacon collector) |
| Transport/retry | OTEL SDK (in plugin process) |
| Collector deployment | Operations team |

### Proto Contract Changes

```protobuf
// New RPC on Plugin service
rpc Export(ExportRequest) returns (ExportResponse);

// New messages
message ExportRequest {
  CollectorConfig collector = 1;
}

message CollectorConfig {
  string endpoint = 1;
  string auth_token = 2;  // Resolved bearer token (complyctl handles OIDC exchange)
}

message ExportResponse {
  bool success = 1;
  int32 exported_count = 2;
  int32 failed_count = 3;
  string error_message = 4;
}

// Modified message
message DescribeResponse {
  // ... existing fields ...
  bool supports_export = 6;
}
```

### Config Changes

```yaml
# complytime.yaml — new optional section
collector:
  endpoint: "collector.example.com:4317"
  auth:
    client-id: "${BEACON_CLIENT_ID}"
    client-secret: "${BEACON_CLIENT_SECRET}"
    token-endpoint: "https://sso.example.com/realms/comply/protocol/openid-connect/token"
```

```go
// internal/complytime/config.go — new structs
type CollectorConfig struct {
    Endpoint string      `yaml:"endpoint"`
    Auth     *AuthConfig `yaml:"auth,omitempty"`
}

type AuthConfig struct {
    ClientID      string `yaml:"client-id"`
    ClientSecret  string `yaml:"client-secret"`
    TokenEndpoint string `yaml:"token-endpoint"`
}

// WorkspaceConfig gains:
//   Collector *CollectorConfig `yaml:"collector,omitempty"`
```

### Data Flow

```text
User runs: complyctl scan --policy-id cis-fedora --format otel

1. Parse --format otel → set exportMode = true
2. Validate collector config exists in complytime.yaml
3. Resolve auth token: POST client credentials to token_endpoint → get access_token
4. Run normal scan flow (generate → scan → produce scan summary)
5. Display scan summary as usual
6. Begin export phase:
   a. Call Describe on each plugin → filter to supports_export == true
   b. For each capable plugin:
      i.   Build ExportRequest{Collector: {endpoint, resolved_token}}
      ii.  Call RouteExport(ctx, evaluatorID, exportReq)
      iii. Plugin receives ExportRequest via gRPC
      iv.  Plugin initializes ProofWatch with collector endpoint
      v.   Plugin reads scan results from in-memory or workspace state
      vi.  Plugin creates GemaraEvidence per assessment result
      vii. Plugin calls proofwatch.Log(ctx, evidence) → OTLP
      viii.Plugin returns ExportResponse{success, counts}
7. Display export summary:
   PROVIDER        EXPORTED  FAILED  STATUS
   openscap        44        3       ❌
   ampel           5         0       ✅
   prowler         -         -       ⏭️ (no export support)

   openscap: 3 records failed: connection reset by collector after 44 records
```

### Test Architecture

**Unit tests** cover:
- `RouteExport` routing (capable plugin, incapable plugin,
  missing plugin, error cases)
- `CollectorConfig` validation (missing endpoint, missing
  or incomplete auth fields, env var expansion)
- OIDC token exchange (mocked token endpoint returning
  access token, error responses, unreachable endpoint)
- Proto ↔ domain type mapping for Export messages
- Scan command `--format otel` path: config validation,
  export orchestration, summary rendering

**Integration tests** (via test-plugin):
- `complyctl scan --format otel` with test-plugin
  implementing Export
- Verify ExportRequest reaches plugin with correct config
- Verify ExportResponse counts are surfaced in terminal
  after the scan summary

**No end-to-end OTEL tests in complyctl** — complyctl never
sends OTLP. OTEL integration is tested in the plugin repos
against a real Beacon collector.

## Complexity Tracking

No constitution violations. No complexity justification needed.

The design is intentionally thin on the complyctl side — complyctl
is an orchestrator that passes config and collects status. All
OTEL complexity lives in the plugins (via ProofWatch) and in the
collector (via TruthBeam). Integrating export into the scan command
via `--format otel` keeps the UX simple (one command) while
maintaining complyctl's minimal dependency footprint.
