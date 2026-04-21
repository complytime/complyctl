# Data Model: Providers Repository Split

**Phase**: 1 — Design & Contracts
**Date**: 2026-04-21
**Plan**: [plan.md](./plan.md)

---

This feature does not introduce a new persistent data model. It is a source code restructuring and terminology rename. This document records the entity changes that result from the migration — specifically the renamed types and their relationships — to guide implementers.

---

## Entity: Provider (formerly Plugin)

The central entity in the provider SDK is the `Provider` interface (currently named `Plugin`). After migration it lives at `github.com/complytime/complyctl/pkg/provider`.

```
Provider (interface)
  ├── Describe(ctx, *DescribeRequest) (*DescribeResponse, error)
  ├── Generate(ctx, *GenerateRequest) (*GenerateResponse, error)
  ├── Scan(ctx, *ScanRequest) (*ScanResponse, error)
  └── Export(ctx, *ExportRequest) (*ExportResponse, error)   ← added by PR #463
```

**Key attributes**:
- Identity: derived from binary name (`complyctl-provider-<name>`) → EvaluatorID = `<name>`
- Lifecycle: discovered at runtime from filesystem; loaded on first use; killed on `Cleanup()`
- State transitions: `Discovered → Loaded (Describe called) → Active | Unhealthy (skipped)`

---

## Entity: ProviderInfo (formerly PluginInfo)

Holds the identity and filesystem path of a discovered provider.

```
ProviderInfo
  ├── PluginID       string   → renamed: ProviderID string
  ├── EvaluatorID    string   (unchanged — routes Generate/Scan requests)
  └── ExecutablePath string   (unchanged)
```

**Rename note**: `PluginID` is renamed to `ProviderID` for consistency. `EvaluatorID` is unchanged — it is the routing key used at runtime and has no "plugin" connotation.

---

## Entity: Manager

The `Manager` orchestrates provider lifecycle and request routing. Stays in `pkg/provider/` after rename.

```
Manager
  ├── discovery  *Discovery
  ├── providers  map[string]*LoadedProvider   ← renamed from plugins map[string]*LoadedPlugin
  └── logger     hclog.Logger

LoadedProvider (formerly LoadedPlugin)
  ├── Info    ProviderInfo
  └── Client  Provider
```

**Methods renamed**:
- `LoadPlugins()` → `LoadProviders()`
- `GetPlugin(evaluatorID)` → `GetProvider(evaluatorID)`
- `ListPlugins()` → `ListProviders()`
- `RouteGenerate()`, `RouteScan()`, `RouteExport()` — method names unchanged (verb-object, no "plugin" connotation)

---

## Entity: Discovery

Scans filesystem directories for provider executables.

```
Discovery
  └── providerDir string   ← renamed from pluginDir
```

**Methods renamed**:
- `DiscoverPlugins()` → `DiscoverProviders()`

**Unchanged**: filesystem paths (`~/.complytime/providers/`, `/usr/libexec/complytime/providers/`), binary prefix value (`"complyctl-provider-"`), `scanDir()` internal function.

---

## Entity: ProviderExecutablePrefix (constant)

```
// internal/complytime/consts.go
const ProviderExecutablePrefix = "complyctl-provider-"   // formerly PluginExecutablePrefix
```

**Value is frozen**. Only the identifier name changes.

---

## Workspace Artifact Paths (unchanged)

The migration must not change any workspace paths — operators must experience no disruption.

```
~/.complytime/providers/          # user provider directory (discovery)
/usr/libexec/complytime/providers/ # system provider directory (discovery fallback)
.complytime/openscap/             # openscap workspace artifacts (unchanged)
.complytime/ampel/                # ampel workspace artifacts (unchanged)
.complytime/complyctl.log         # log file (unchanged)
```

---

## Wire Protocol (frozen)

The gRPC wire protocol is frozen. No changes to message types, field numbers, or service definition.

```
Package:          complyctl.plugin.v1   (UNCHANGED — proto package rename deferred)
Service:          Plugin
RPCs:             Generate, Scan, Describe, Export (Export added by PR #463)
Handshake key:    "COMPLYCTL_PLUGIN"
Handshake value:  "ddff478d-578e-4d9d-8253-35e8ebf548d2"
Protocol version: 1
```

---

## Module Boundaries After Migration

```
github.com/complytime/complyctl
  └── pkg/provider/          # SDK: Provider interface, Manager, Discovery, gRPC harness
  └── api/plugin/            # Protobuf generated code (package name unchanged)
  └── internal/complytime/   # Constants, config, workspace (ProviderExecutablePrefix here)

github.com/complytime/complytime-providers
  └── cmd/openscap-provider/ # OpenSCAP provider binary
  └── cmd/ampel-provider/    # AMPEL provider binary
  require: github.com/complytime/complyctl vX.Y.Z
```
