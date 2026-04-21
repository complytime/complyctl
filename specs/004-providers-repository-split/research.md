# Research: Providers Repository Split

**Phase**: 0 — Research & Unknowns Resolution
**Date**: 2026-04-21
**Plan**: [plan.md](./plan.md)

---

## Decision 1: Go module layout for `complytime-providers`

**Decision**: Single root `go.mod` covering both providers (monorepo, one module).

**Rationale**: Both providers depend on the same complyctl SDK version and are maintained by the same team. A single module minimizes vendoring overhead, keeps `go.sum` and `vendor/` coherent, and avoids the synchronization cost of keeping two separate modules on the same SDK version. The openscap-plugin's separate `go.mod` in complyctl was a workaround for co-location in a root module, not a principled isolation boundary. With providers in a dedicated repository, the workaround is unnecessary.

**Alternatives considered**:
- One `go.mod` per provider: rejected — doubled vendoring cost, version drift risk between providers on the SDK, no real isolation benefit since both providers are maintained together.
- Separate repositories per provider: rejected — too much overhead for two providers with the same maintainer team and shared SDK dependency.

---

## Decision 2: `pkg/plugin/` → `pkg/provider/` rename scope

**Decision**: Rename the directory and update all internal import paths. Rename all exported/unexported identifiers containing "Plugin" to "Provider" — except those whose names are dictated by the `hashicorp/go-plugin` library interface. Handshake constant identifier renaming is a convention preference (SHOULD, not MUST) and is left to the implementer.

**Rationale**: The directory rename is the most user-visible part of the terminology change — it appears in the published import path consumed by `complytime-providers`. The identifier rename inside the package is mechanical and improves internal consistency. Handshake identifiers (`Handshake`, `SupportedPlugins`, `GRPCEvaluatorPlugin`) carry no semantic weight beyond their wire values, which are frozen.

**Affected identifiers (confirmed by code inspection)**:

| Current | Renamed To | File |
|---|---|---|
| `type Plugin interface` | `type Provider interface` | `manager.go` |
| `type LoadedPlugin struct` | `type LoadedPlugin struct` | `manager.go` _(kept — "Loaded" qualifies a runtime state, not the concept)_ |
| `func (m *Manager) GetPlugin` | `func (m *Manager) GetProvider` | `manager.go` |
| `func (m *Manager) ListPlugins` | `func (m *Manager) ListProviders` | `manager.go` |
| `func (m *Manager) LoadPlugins` | `func (m *Manager) LoadProviders` | `manager.go` |
| `type PluginInfo struct` | `type ProviderInfo struct` | `discovery.go` |
| `type Discovery` (field `pluginDir`) | `type Discovery` (field `providerDir`) | `discovery.go` |
| `func (d *Discovery) DiscoverPlugins` | `func (d *Discovery) DiscoverProviders` | `discovery.go` |
| `type GRPCEvaluatorPlugin` | _(see note)_ | `plugin.go` |
| `var SupportedPlugins` | `var SupportedProviders` (SHOULD) | `plugin.go` |
| `var Handshake` | _(wire value frozen; identifier SHOULD rename)_ | `plugin.go` |
| `const PluginExecutablePrefix` | `const ProviderExecutablePrefix` | `internal/complytime/consts.go` |

Note on `GRPCEvaluatorPlugin`: this type name is referenced as the value type inside `SupportedPlugins`/`SupportedProviders` map and in the `Serve()` function. The embedded `goplugin.Plugin` field is dictated by the `hashicorp/go-plugin` interface and cannot be renamed. The struct name itself (`GRPCEvaluatorPlugin`) may be renamed to `GRPCEvaluatorProvider` since it is our own type, but this is low value.

**Wire values — FROZEN (must not change)**:
```
MagicCookieKey:   "COMPLYCTL_PLUGIN"
MagicCookieValue: "ddff478d-578e-4d9d-8253-35e8ebf548d2"
ProtocolVersion:  1
Binary prefix:    "complyctl-provider-"
```

**Alternatives considered**:
- Keep `pkg/plugin/` directory name, rename only identifiers: rejected — leaves an internal inconsistency in the published import path which is the most visible surface.
- Rename everything including wire values: rejected — breaks all existing installed provider binaries at runtime.

---

## Decision 3: `complytime-providers` module name

**Decision**: `github.com/complytime/complytime-providers`

**Rationale**: Consistent with the org naming convention and the GitHub repository URL (`github.com/complytime/complytime-providers`). The module name is what provider authors would see if they ever import utilities from this repository (unlikely, but conventional to match).

**Alternatives considered**: `github.com/complytime/providers` — rejected, too generic and doesn't match the repo URL.

---

## Decision 4: SDK release sequencing relative to provider migration

**Decision**: Parallel development is allowed. `complytime-providers` may use a `replace` directive during development. The `replace` directive MUST be removed before merging to `complytime-providers` main. A tagged complyctl release is a merge gate, not a development prerequisite.

**Rationale**: Enforcing a hard sequential gate (tag first, then start) adds unnecessary friction and delays development. Using a `replace` directive locally is idiomatic Go for cross-module development. The merge gate ensures the published repository is never in a broken state.

**Implementation note**: The complyctl SDK release must be taken *after* PR #463 merges (so `Export` RPC is included in the contract) and *after* PR #479 merges (so `ResolveVersion` is included). The tag should be the first release that includes both.

**Alternatives considered**:
- Hard sequential gate (tag first): rejected — adds a synchronization barrier with no safety benefit if `replace` usage is gated at merge time.
- Simultaneous coordinated release: possible but complex; the parallel approach with a merge gate achieves the same safety with less coordination overhead.

---

## Decision 5: CI workflow for provider binary acquisition in `ci_compliance.yml`

**Decision**: Download `complyctl-provider-ampel` as a pre-built binary from a `complytime-providers` GitHub release artifact.

**Rationale**: Treats `complytime-providers` as the authoritative source of provider binaries — identical to how operators install providers in production. Avoids encoding a source build of a separate repository inside complyctl's CI. Requires `complytime-providers` to publish release artifacts, which it should do regardless.

**Implementation note**: The `ci_compliance.yml` workflow delegates to the `reusable_compliance.yml` workflow in `complytime/org-infra`. The update to download the provider binary will need to happen either in `org-infra` (preferred — centralizes provider installation logic) or in `ci_compliance.yml` itself as a pre-step. This is a planning-time decision to flag for the org-infra maintainers.

**Alternatives considered**:
- Build from source by cloning `complytime-providers` in CI: rejected — couples complyctl CI to provider source code, negating the separation benefit.
- Install from RPM: rejected — depends on the deferred RPM spec and is not available at the time of this feature's implementation.
- Move the workflow to `complytime-providers`: possible long-term, but out of scope for this feature.

---

## Decision 6: Impact of PR #463 (Export RPC) on migration scope

**Decision**: The `Export` RPC added by PR #463 is in scope for the migration — providers must carry their `Export` stubs into `complytime-providers`, and the complyctl SDK release must include the Export contract.

**Rationale**: PR #463 adds `Export` to `pkg/plugin/client.go`, `server.go`, and `manager.go`, and adds stubs to both `cmd/openscap-plugin/server/server.go` and `cmd/ampel-plugin/server/server.go`. If the migration copies provider source without the Export stub, the providers will fail to compile against the SDK (the `Plugin`/`Provider` interface will require `Export`). The migration must therefore be sequenced to occur after PR #463 merges, or the provider source must include the Export stub explicitly.

**Key files affected by PR #463 that are also in scope for migration**:
- `pkg/plugin/client.go` — gains `Export` client method → must be in `pkg/provider/` after rename
- `pkg/plugin/manager.go` — gains `RouteExport` → must be in `pkg/provider/` after rename
- `pkg/plugin/server.go` — gains `Export` server adapter → must be in `pkg/provider/` after rename
- `cmd/openscap-plugin/server/server.go` — gains `Export` stub → carries to `cmd/openscap-provider/server/`
- `cmd/ampel-plugin/server/server.go` — gains `Export` stub → carries to `cmd/ampel-provider/server/`
- `cmd/openscap-plugin/vendor/` — updated vendored SDK files → will be replaced by `go mod vendor` in `complytime-providers` against the published module

**Alternatives considered**: Migrate before PR #463 merges, then carry the Export stub forward separately — rejected as unnecessarily complex. Wait for PR #463 to merge, then migrate.

---

## Decision 7: Impact of PR #479 (pinned version resolution) on migration scope

**Decision**: PR #479 has no impact on the migration scope. All changes are in the complyctl core (`internal/cache/`, `internal/doctor/`, `internal/registry/`, `cmd/complyctl/cli/doctor.go`). No provider source files are touched.

**Rationale**: The `VersionResolver` interface change (`ResolveVersion` method added) is core-only and does not flow into `pkg/plugin/` or any provider. The migration can proceed independently of PR #479's merge status for the provider code; however, the complyctl SDK tagged release should still be taken after PR #479 merges for completeness.

---

## Decision 8: Provider directory naming in `complytime-providers`

**Decision**: Rename directories to use `-provider` suffix: `cmd/openscap-provider/` and `cmd/ampel-provider/`.

**Rationale**: Consistent with the terminology rename. The provider binary names (`complyctl-provider-openscap`, `complyctl-provider-ampel`) are stable and unchanged — only the source directory names change within the new repository.

**Alternatives considered**: Keep `cmd/openscap-plugin/` and `cmd/ampel-plugin/` names in the new repository — rejected as it perpetuates the old terminology and would be confusing in a repository called `complytime-providers`.

---

## Decision 9: `cmd/test-plugin/` disposition

**Decision**: `cmd/test-plugin/` is renamed to `cmd/test-provider/` and stays in the complyctl repository. It is NOT migrated to `complytime-providers`.

**Rationale**: The test provider is a CI test harness for complyctl's E2E and behavioral test suite, not a real provider intended for end users. Its place is alongside the tests it supports. Moving it to `complytime-providers` would couple the providers repository to complyctl's internal CI infrastructure.

---

## Unresolved Items (deferred to planning/tasks)

- Whether the `ci_compliance.yml` update should go through `org-infra` (reusable workflow update) or be done locally in `ci_compliance.yml` as a pre-step. Flag for org-infra maintainers.
- The exact complyctl version tag to use in the `complytime-providers` `go.mod` (determined at release time, not during planning).
- Whether `complytime-providers` needs its own Packit/testing-farm configuration (out of scope for this feature; covered by the deferred RPM spec).
