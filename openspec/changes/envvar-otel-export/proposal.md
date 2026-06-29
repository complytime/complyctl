## SUPERSEDED

**This proposal was superseded by issue #606 on 2026-06-24.**

The export infrastructure described in this proposal (including `COMPLYTIME_EXPORT_ENABLED`) was removed as speculative infrastructure built before backend design was finalized. Export functionality will be redesigned and reintroduced when the backend shape is known. See CHANGELOG.md for migration guidance.

---

## Why

The `--format otel` flag on the `scan` command conflates two concerns: output format selection and export behavior. The other `--format` values (`oscal`, `pretty`, `sarif`) control the local file format of scan results. `otel` is fundamentally different — it triggers a network export to a remote Beacon collector as a side effect, not a file format choice. This creates UX confusion and prevents users from exporting to a collector while also producing a local report (e.g., `--format sarif` + export).

Switching to an environment variable (`COMPLYTIME_EXPORT_ENABLED`) decouples the export trigger from format selection, aligns with how collector credentials are already configured (env vars in `complytime.yaml`), and allows export to work alongside any output format.

## What Changes

- **BREAKING**: Remove `otel` as a valid value for the `--format` flag on `complyctl scan`. Users currently passing `--format otel` must switch to setting `COMPLYTIME_EXPORT_ENABLED=true`.
- Add `COMPLYTIME_EXPORT_ENABLED` environment variable. When set to `true`, the scan command triggers the existing export-to-collector flow after scan completes, regardless of which `--format` is used (or none).
- The underlying export orchestration logic (`runExport`, `exportToProviders`, OIDC token resolution, export summary rendering) remains unchanged — only the trigger mechanism changes.
- Update `complyctl scan` help text, examples, and shell completions to remove `otel` from the format list.
- Update `doctor` command messaging to reference the new env var instead of `--format otel`.

## Capabilities

### New Capabilities
- `envvar-export-trigger`: Environment variable-based trigger for Beacon collector export, replacing the `--format otel` flag.

### Modified Capabilities
- `scan --format`: The `otel` value is removed from the accepted format list. The `--format` flag now only controls local file output format (`oscal`, `pretty`, `sarif`).
- `doctor`: Collector check messaging updated to reference `COMPLYTIME_EXPORT_ENABLED` instead of `--format otel`.

### Removed Capabilities
- `format-otel`: The `otel` value for the `--format` flag is removed. Export is now triggered via `COMPLYTIME_EXPORT_ENABLED=true` environment variable.

## Constitution Alignment

| Principle                          | Status | Evidence                                                                                                                                             |
|------------------------------------|--------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| I. Single Source of Truth          | PASS   | New `ExportEnabledEnvVar` constant centralizes the env var name. No magic strings.                                                                   |
| II. Simplicity & Isolation         | PASS   | Minimal change — replaces one check in `maybeExport()`. No structural changes to scan flow.                                                          |
| III. Incremental Improvement       | PASS   | Single-concern change. No unrelated refactoring bundled in.                                                                                          |
| IV. Readability First              | PASS   | `COMPLYTIME_EXPORT_ENABLED` is self-documenting. `--format otel` rejected with standard invalid format error; migration documented in release notes. |
| V. Do Not Reinvent the Wheel       | PASS   | Uses `strconv.ParseBool` (stdlib) for boolean parsing. No custom env var framework.                                                                  |
| VI. Composability                  | PASS   | Export and format are now independent — users can compose `--format sarif` + export. Improves composability.                                         |
| VII. Convention Over Configuration | PASS   | Export defaults to off (env var not set). Consistent with existing env var pattern for collector credentials.                                        |

## Impact

- **CLI surface**: `--format` flag loses the `otel` value. Breaking change for users and scripts using `--format otel`.
- **Code**: `cmd/complyctl/cli/scan.go` (trigger logic), `internal/complytime/consts.go` (remove `OutputFormatOTEL`), `internal/doctor/doctor.go` (messaging update), shell completions.
- **Tests**: E2E tests (`TestE2E_ScanFormatOtel`, `TestE2E_ScanFormatOtelNoCollector`), unit tests for format validation, behavioral tests.
- **Documentation**: Any user-facing docs referencing `--format otel`. README.md scan section, AGENTS.md recent changes, release notes.
- **Upstream specs**: `specs/003-complybeacon-export/spec.md` requirements FR-001, FR-011, FR-012 are superseded by this change and should be annotated.
- **No dependency changes**: No new Go modules needed. `golang.org/x/oauth2` and the provider export infrastructure remain as-is.
