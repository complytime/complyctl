# Implementation Plan: Providers Repository Split

**Branch**: `004-providers-repository-split` | **Date**: 2026-04-21 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/004-providers-repository-split/spec.md`

## Summary

Extract the OpenSCAP and AMPEL provider implementations from the complyctl repository into the standalone `complytime-providers` repository, rename all "plugin" terminology to "provider" across complyctl (code, CLI, CI, docs, Makefile), rename `pkg/plugin/` to `pkg/provider/`, and update CI workflows to obtain provider binaries from `complytime-providers` release artifacts. The migration is sequenced so complyctl changes land first (with a tagged SDK release), then providers migrate using that published version.

### Impact from In-Flight PRs

Two PRs currently in review will affect this migration's scope when merged:

**PR #463 — `feat: add --format otel to scan command for ComplyBeacon evidence export`**

This PR adds a fourth RPC (`Export`) to the provider contract (`pkg/plugin/`, `api/plugin/plugin.proto`), adds `Export` stubs to `cmd/openscap-plugin/server/`, `cmd/ampel-plugin/server/`, and `cmd/test-plugin/`, updates `cmd/openscap-plugin/vendor/` with the new SDK files, and adds `golang.org/x/oauth2` to the root `go.mod`.

_Impact_:
- The `Plugin` interface in `pkg/plugin/manager.go` gains an `Export` method — this must be included in the `pkg/provider/` rename.
- The openscap-plugin vendor tree adds `pkg/plugin/client.go`, `manager.go`, `server.go` changes — the migration must re-vendor from the published SDK that includes Export.
- The `cmd/openscap-plugin/server/` and `cmd/ampel-plugin/server/` both gain `Export` stubs — these must be carried into `complytime-providers`.
- The `otel` output format constant is added to `internal/complytime/consts.go` — this stays in complyctl core, no conflict.
- The `CollectorConfig` in `internal/complytime/config.go` and the `CheckCollector` in `internal/doctor/doctor.go` are core-only changes — no conflict.
- **Action**: Ensure the complyctl SDK tagged release used by `complytime-providers` is taken *after* PR #463 merges, so the Export RPC is part of the published contract.

**PR #479 — `fix: get resolves pinned version instead of defaulting to latest tag`**

Small bug fix touching `internal/cache/sync.go`, `internal/doctor/doctor.go`, `internal/doctor/doctor_test.go`, `internal/registry/client.go`, `cmd/complyctl/cli/doctor.go`. All changes are in the complyctl core, none in provider code.

_Impact_: No impact on the providers migration scope. The `VersionResolver` interface gains a `ResolveVersion` method — this is core-only and does not affect `pkg/plugin/` or the provider binaries.

---

## Technical Context

**Language/Version**: Go 1.25 (complyctl root `go.mod`); Go 1.24 (current `cmd/openscap-plugin/go.mod` — will be updated to match root in `complytime-providers`)
**Primary Dependencies**:
- `github.com/hashicorp/go-plugin v1.7.0` — gRPC-based plugin framework (wire protocol, handshake)
- `github.com/hashicorp/go-hclog v1.6.3` — structured logging in provider binaries
- `google.golang.org/grpc` — underlying gRPC transport
- `github.com/complytime/complyctl` (published, versioned) — SDK consumed by `complytime-providers`
- Provider-specific: `github.com/antchfx/xmlquery` (openscap); `snappy`, `ampel` CLI (ampel)
**Storage**: Filesystem only — workspace `.complytime/` directories; no database
**Testing**: `go test ./...` (unit); existing E2E suite adapted to use externally-built provider binary
**Target Platform**: Linux (amd64); providers installed to `~/.complytime/providers/` or `/usr/libexec/complytime/providers/`
**Project Type**: CLI tool (complyctl) + plugin binaries (providers)
**Performance Goals**: No change from current — provider discovery and invocation latency unchanged
**Constraints**: Wire compatibility MUST be preserved — handshake values (`"COMPLYCTL_PLUGIN"`, UUID, protocol version 1) are stable; binary naming convention `complyctl-provider-<name>` is stable
**Scale/Scope**: Two providers migrated; 21 Go source files in `pkg/plugin/`; ~6 identifiers renamed in `pkg/plugin/`; ~30 "plugin" references across Makefile, CI, docs

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I. Single Source of Truth | **PASS** | `PluginExecutablePrefix` value preserved unchanged; SDK published once and referenced by version |
| II. Simplicity & Isolation | **PASS** | Split reduces coupling; each repo has a single concern |
| III. Incremental Improvement | **PASS** | Scope is tightly bounded to migration + terminology rename; RPM/testing-farm deferred |
| IV. Readability First | **PASS** | `pkg/provider/` naming is more accurate than `pkg/plugin/`; identifier renames are mechanical |
| V. Do Not Reinvent the Wheel | **PASS** | `hashicorp/go-plugin` framework retained; no custom IPC introduced |
| VI. Composability | **PASS** | Provider binaries remain independently buildable and composable with complyctl via discovery |
| VII. Convention Over Configuration | **PASS** | Discovery convention unchanged (`complyctl-provider-*`); no new configuration needed |

**Verdict**: All gates pass. No violations requiring justification.

---

## Project Structure

### Documentation (this feature)

```text
specs/004-providers-repository-split/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── provider-sdk.md  # Provider SDK contract (interface, handshake, naming)
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code Impact Map

#### complyctl repository (changes)

```text
pkg/plugin/              → pkg/provider/           # directory rename + all imports
  client.go                                         # Export RPC client method added (PR #463)
  server.go                                         # Export RPC server method added (PR #463)
  manager.go                                        # Plugin → Provider identifiers; Export route added
  discovery.go                                      # PluginInfo → ProviderInfo; DiscoverPlugins preserved
  plugin.go                                         # Handshake var rename (SHOULD); SupportedPlugins → SupportedProviders
  initialization.go                                 # NewClient → NewProviderClient (or similar)

internal/complytime/consts.go
  PluginExecutablePrefix → ProviderExecutablePrefix  # identifier only; value unchanged

cmd/complyctl/cli/
  providers.go                                       # user-facing "plugin" text → "provider"
  scan.go                                            # --format otel additions (PR #463); plugin log refs
  generate.go                                        # any plugin log/help text → provider
  doctor.go                                          # plugin diagnostic text → provider

docs/PLUGIN_GUIDE.md     → REMOVED                  # migrated to complytime-providers
docs/man/complyctl-openscap-plugin.md → REMOVED      # migrated to complytime-providers

cmd/openscap-plugin/     → REMOVED                  # migrated to complytime-providers
cmd/ampel-plugin/        → REMOVED                  # migrated to complytime-providers
cmd/test-plugin/         → cmd/test-provider/        # renamed; stays in complyctl

Makefile
  build-test-plugin      → build-test-provider
  MAN_OPENSCAP_PLUGIN    → MAN_OPENSCAP_PROVIDER
  cd cmd/openscap-plugin build step → REMOVED
  vendor step for openscap-plugin → REMOVED

.github/workflows/
  behavioral_assessment.yml  # build-test-plugin → build-test-provider
  e2e_test.yml               # build-test-plugin → build-test-provider
  ci_compliance.yml          # download complyctl-provider-ampel from complytime-providers release
```

#### complytime-providers repository (new content)

```text
complytime-providers/
├── go.mod                            # module github.com/complytime/complytime-providers
│                                     # require github.com/complytime/complyctl vX.Y.Z
├── go.sum
├── vendor/
├── Makefile                          # build, test, vendor, lint targets
├── README.md                         # updated from stub
├── docs/
│   └── provider-guide.md             # migrated from complyctl docs/PLUGIN_GUIDE.md
├── cmd/
│   ├── openscap-provider/            # migrated from cmd/openscap-plugin/
│   │   ├── main.go
│   │   ├── config/
│   │   ├── oscap/
│   │   ├── scan/
│   │   └── server/                   # includes Export stub (from PR #463)
│   └── ampel-provider/               # migrated from cmd/ampel-plugin/
│       ├── main.go
│       ├── config/
│       ├── convert/
│       ├── intoto/
│       ├── results/
│       ├── scan/
│       ├── server/                   # includes Export stub (from PR #463)
│       ├── targets/
│       └── toolcheck/
└── .github/
    └── workflows/
        └── ci.yml                    # build + test both providers; release artifacts
```

**Structure Decision**: Single Go module (`github.com/complytime/complytime-providers`) at the root of `complytime-providers`, covering both providers under `cmd/`. Each provider is built as a separate binary but shares a single `go.mod` and `vendor/` tree. This is the simplest layout with lowest maintenance overhead and aligns with how `cmd/ampel-plugin/` already lives in the complyctl root module today.

---

## Complexity Tracking

No constitution violations — table not required.
