# Provider SDK Contract

**Phase**: 1 — Design & Contracts
**Date**: 2026-04-21
**Plan**: [plan.md](../plan.md)

---

This document defines the provider SDK contract that all first-party and third-party complyctl
providers MUST implement. It serves as the authoritative reference for the `pkg/provider/`
package (renamed from `pkg/plugin/`) published by the complyctl module.

---

## 1. Provider Interface

After the terminology rename (`FR-013`, `FR-020`), the central interface lives at:

```
import "github.com/complytime/complyctl/pkg/provider"
```

```go
// Provider is the interface that provider authors implement for evaluation RPCs.
type Provider interface {
    Describe(ctx context.Context, req *DescribeRequest) (*DescribeResponse, error)
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error)
}

// Exporter is an optional interface for providers that support shipping
// evidence to a Beacon collector. Provider authors opt in by implementing
// this interface and declaring SupportsExport: true in DescribeResponse.
type Exporter interface {
    Export(ctx context.Context, req *ExportRequest) (*ExportResponse, error)
}
```

> **Note**: `Export` is intentionally kept out of `Provider` to follow the Interface Segregation
> Principle. Providers that do not support evidence export need not implement `Exporter` at all —
> complyctl detects export capability via `DescribeResponse.SupportsExport` and a runtime type
> assertion on the client. This design was finalised after PR #463 merged.

### Entry Point

Provider binaries MUST call `provider.Serve(impl)` from `main()`:

```go
func main() {
    server := &MyProviderServer{} // implements provider.Provider
    provider.Serve(server)
}
```

`Serve` configures the hashicorp/go-plugin gRPC harness (logger, handshake, gRPC server) and
blocks until the parent process terminates the connection.

---

## 2. Wire Protocol (Frozen)

These values are stable and MUST NOT change. They are checked during the go-plugin handshake
before any RPC is dispatched. Existing installed provider binaries rely on these values.

| Field | Value | Changeability |
|---|---|---|
| `MagicCookieKey` | `"COMPLYCTL_PLUGIN"` | **FROZEN** |
| `MagicCookieValue` | `"ddff478d-578e-4d9d-8253-35e8ebf548d2"` | **FROZEN** |
| `ProtocolVersion` | `1` | **FROZEN** |
| gRPC service name | `complyctl.plugin.v1.Plugin` | **FROZEN** (proto rename deferred) |
| Proto package | `complyctl.plugin.v1` | **FROZEN** (rename deferred to a future major) |

> **Rationale**: Renaming the wire values would silently break any provider binary installed on
> an operator's machine that was built against the previous values. The Go constant *identifiers*
> that hold these values are renamed as a convention preference (`Handshake` var remains;
> `SupportedPlugins` → `SupportedProviders` map is SHOULD), but the values themselves are
> byte-for-byte identical before and after the migration.

---

## 3. Binary Naming Convention

Provider executables MUST follow the naming convention:

```
complyctl-provider-<name>
```

where `<name>` is the `EvaluatorID` by which complyctl routes requests to this provider.

**Examples**:
- `complyctl-provider-openscap` (EvaluatorID: `openscap`)
- `complyctl-provider-ampel`    (EvaluatorID: `ampel`)

The prefix value is defined as:

```go
// internal/complytime/consts.go
const ProviderExecutablePrefix = "complyctl-provider-"  // formerly PluginExecutablePrefix
```

The string value `"complyctl-provider-"` is **frozen**. Only the constant identifier name
changes as part of `FR-014`.

---

## 4. Discovery Paths

complyctl discovers provider binaries at runtime by scanning two directories in priority order:

| Priority | Path | Description |
|---|---|---|
| 1 (highest) | `~/.complytime/providers/` | User-installed providers |
| 2 | `/usr/libexec/complytime/providers/` | System-installed providers |

User-directory providers shadow system-directory providers with the same `EvaluatorID`.
These paths are **unchanged** by the repository split — operators need not move any installed
binaries.

---

## 5. RPC Message Types

All Go types below live in `pkg/provider/` after the rename. Their shapes are unchanged.

### Describe

```go
type DescribeRequest struct{}

type DescribeResponse struct {
    Healthy                 bool
    Version                 string
    ErrorMessage            string
    RequiredGlobalVariables []string
    RequiredTargetVariables []string
    SupportsExport          bool     // opt in to Exporter capability
}
```

The Manager calls `Describe` during `LoadProviders()` (formerly `LoadPlugins()`) to verify
provider health before registering it. Providers reporting `Healthy: false` are skipped with
a stderr warning.

### Generate

```go
type GenerateRequest struct {
    GlobalVariables map[string]string
    Configuration   []AssessmentConfiguration
    TargetVariables map[string]string
}

type AssessmentConfiguration struct {
    PlanID        string
    RequirementID string
    Parameters    map[string]string
    EvaluatorID   string  // routing only; not serialized over gRPC
}

type GenerateResponse struct {
    Success      bool
    ErrorMessage string
}
```

### Scan

```go
type ScanRequest struct {
    Targets []Target
}

type Target struct {
    TargetID  string
    Variables map[string]string
}

type ScanResponse struct {
    Assessments []AssessmentLog
}

type AssessmentLog struct {
    RequirementID string
    Steps         []Step
    Message       string
    Confidence    ConfidenceLevel
}

type Step struct {
    Name    string
    Result  Result
    Message string
}
```

### Result and ConfidenceLevel enumerations

```go
type Result int32

const (
    ResultUnspecified Result = 0
    ResultPassed      Result = 1
    ResultFailed      Result = 2
    ResultSkipped     Result = 3
    ResultError       Result = 4
)

type ConfidenceLevel int32

const (
    ConfidenceLevelNotSet       ConfidenceLevel = 0
    ConfidenceLevelUndetermined ConfidenceLevel = 1
    ConfidenceLevelLow          ConfidenceLevel = 2
    ConfidenceLevelMedium       ConfidenceLevel = 3
    ConfidenceLevelHigh         ConfidenceLevel = 4
)
```

### Export (added by PR #463, optional)

The `Export` message types are defined by PR #463 and will be present in the tagged SDK release.
Export support is **opt-in**: providers that do not ship evidence to a Beacon collector simply
skip implementing `Exporter`. No stub is required.

To opt in, a provider implements the `Exporter` interface and signals capability via `DescribeResponse`:

```go
// MyProviderServer implements provider.Provider (required) and provider.Exporter (optional).
type MyProviderServer struct{}

func (s *MyProviderServer) Export(ctx context.Context, req *provider.ExportRequest) (*provider.ExportResponse, error) {
    // ... send evidence to req.Collector.Endpoint ...
    return &provider.ExportResponse{Success: true, ExportedCount: n}, nil
}

func (s *MyProviderServer) Describe(_ context.Context, _ *provider.DescribeRequest) (*provider.DescribeResponse, error) {
    return &provider.DescribeResponse{
        Healthy:        true,
        Version:        "1.0.0",
        SupportsExport: true, // tells complyctl to probe for Exporter at runtime
    }, nil
}
```

complyctl detects export support at runtime via a type assertion on the gRPC client; providers
that return `SupportsExport: false` (or omit it) are silently skipped during `--format otel`.

---

## 6. Manager API (complyctl internal)

The `Manager` is used internally by complyctl. Provider authors do not instantiate it. It is
documented here for completeness and for contributors implementing the rename.

```go
func NewManager(providerDir string, logger hclog.Logger) (*Manager, error)

func (m *Manager) LoadProviders() error          // formerly LoadPlugins()
func (m *Manager) GetProvider(evaluatorID string) (*LoadedProvider, error)  // formerly GetPlugin()
func (m *Manager) ListProviders() []*LoadedProvider  // formerly ListPlugins()
func (m *Manager) Cleanup()

// Routing methods — names unchanged (verb-object, no "plugin" connotation):
func (m *Manager) RouteGenerate(ctx, evaluatorID string, globalVars, targetVars map[string]string, configs []AssessmentConfiguration) error
func (m *Manager) RouteScan(ctx, evaluatorID string, targets []Target) ([]AssessmentLog, error)
func (m *Manager) RouteExport(...)  // added by PR #463
```

`LoadedPlugin` is kept as the internal struct name (Decision: data-model.md — "Loaded" qualifies
runtime state, not the provider concept). Its exported fields are:

```go
type LoadedProvider struct {
    Info   ProviderInfo
    Client Provider
}

type ProviderInfo struct {
    ProviderID     string  // formerly PluginID
    EvaluatorID    string  // unchanged
    ExecutablePath string  // unchanged
}
```

---

## 7. Module Consumption

Providers in `complytime-providers` consume the SDK as a standard Go module dependency:

```go
// go.mod (complytime-providers)
module github.com/complytime/complytime-providers

go 1.24

require (
    github.com/complytime/complyctl vX.Y.Z
    // ... provider-specific deps
)
```

where `vX.Y.Z` is the first complyctl tag published after both PR #463 and PR #479 merge, which
will include:
- `pkg/provider/` (renamed from `pkg/plugin/`)
- `Export` RPC in the `Provider` interface
- `ProviderExecutablePrefix` constant (renamed from `PluginExecutablePrefix`)

During local development only, a `replace` directive is permitted:

```go
replace github.com/complytime/complyctl => /path/to/local/complyctl
```

This directive MUST be removed before any code is merged to the `complytime-providers` main
branch (`FR-003`).

---

## 8. Contract Stability Guarantees

| Surface | Stability | Notes |
|---|---|---|
| `Provider` interface (Describe, Generate, Scan) | **Stable** | Changes require a new major proto version |
| `Exporter` interface (Export) | **Stable** | Optional; detected via runtime type assertion |
| Wire protocol (handshake, gRPC service, proto package) | **Frozen** | `complyctl.plugin.v1` — rename deferred |
| Binary naming convention (`complyctl-provider-<name>`) | **Stable** | Value frozen; identifier renamed |
| Discovery paths | **Stable** | Unchanged by migration |
| `pkg/provider/` import path | **Stable after rename** | Breaking change from `pkg/plugin/`; version-gated by the SDK release |
| Manager API | **Internal** | Used only by complyctl core; not consumed by providers |
