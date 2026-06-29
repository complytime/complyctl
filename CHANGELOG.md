# Changelog

## Unreleased

### Removed

- **BREAKING**: Removed collector export infrastructure (`COMPLYTIME_EXPORT_ENABLED`, `collector:` config block, Export RPC). This was speculative infrastructure added before backend design was finalized. Export functionality will be redesigned and reintroduced when the backend shape is known. Tracking issue needed: "Design and implement evidence export v2 post-backend-finalization". (#606)

  **Migration**: Users who configured `collector:` in `complytime.yaml` or set `COMPLYTIME_EXPORT_ENABLED=true` should remove these configurations. Export functionality is not available in this release. If you require evidence export to a Beacon collector, remain on the previous version until the redesigned export infrastructure is released.

### Added

- Redesigned markdown report (`--format pretty`) with summary metadata
  table, pass rate, grouped controls table with messages, findings
  section grouped by result type with recommendation and collapsible
  evidence, and evaluation log wrapped in collapsible `<details>` (#572)
- `Evidence` and `Recommendation` fields added to provider gRPC API
  (`AssessmentLog` proto message). Providers can now return evidence
  collected during assessment and remediation guidance for non-passing
  results. Fields are optional and backward compatible (#572)
- Workspace configuration: `--workspace` / `-w` flag and `COMPLYTIME_WORKSPACE` environment variable for running commands from any directory (#433)
- Config file location: `complytime.yaml` moved to `.complytime/complytime.yaml` with backward compatibility (#527)
- Deprecation warning for legacy config file location at repository root
- Complypack pull support: `complyctl get` fetches complypack OCI
  artifacts declared in the `complypacks` section of `complytime.yaml`.
  Cached complypacks are dispatched to providers by evaluator-id during
  generation. `complyctl providers` shows cached complypack versions.
  `complyctl doctor` checks complypack availability.
- `complyctl scan` accepts a positional `[target]` argument to scope
  scans to a single target. When a target has exactly one policy, the
  `--policy-id` flag is inferred automatically.
- Policy resolver supports both split-layer and Gemara bundle-format OCI
  artifacts. `internal/policy/loader.go` gained `LoadBundleFiles()` and
  `DetectManifestShape()`.
- `complyctl scan` exits non-zero on operational errors from providers.
  `ScanResponse.errors` proto field surfaces provider-side errors.
- Exit code contract documented in `complyctl scan --help`, the man
  page (`docs/man/complyctl.md`), and the quick start guide
  (`docs/QUICK_START.md`). Exit 0 means scan completed; exit 1 means
  operational error or zero assessed requirements (#608).
- Devcontainer configuration for interactive CLI testing during PR
  reviews (`.devcontainer/`, `docs/TESTING_ENVIRONMENT.md`,
  `make test-devcontainer`). Supports GitHub Codespaces, DevPod, and
  VS Code Dev Containers.
- Cross-repo integration test infrastructure validating the complyctl + Ampel
  provider pipeline end-to-end (`tests/cross-repo/`, `make test-cross-repo`).
- CI workflow `ci_cross_repo_integration.yml` that builds complyctl from the PR
  branch and complytime-providers from main, then runs the full get → generate →
  scan pipeline with real snappy and ampel binaries.
- Minimal test Gemara policy (`policies/test-branch-protection`) seeded in the
  mock OCI registry for integration testing.
- OPA provider test content in devcontainer: Gemara testdata (catalog +
  policy with `executor.id: opa`), OPA complypack artifact (Rego policies
  + `complytime-mapping.json`), `test-opa-bp` policy-id and
  `test-k8s-deployment` target in workspace configuration. OPA command
  reference added to `docs/TESTING_ENVIRONMENT.md`.

### Deprecated

- The `@version` notation for policy URLs (e.g., `registry.com/repo@v1.0`)
  is deprecated. Use standard OCI `:tag` syntax instead (e.g.,
  `registry.com/repo:v1.0`). In the OCI Distribution Spec, `@` is
  exclusively a digest separator. A deprecation warning is now emitted
  when `@version` syntax is detected. `@version` support will be removed
  in a future release. (#600)

### Changed

- All commands now accept `--workspace` flag to specify workspace directory
- `NewWorkspace()` function signature changed from `NewWorkspace()` to `NewWorkspace(baseDir string)`
- Log file and scan output paths are now relative to resolved workspace directory

### Fixed

- `complyctl scan --format` reports (SARIF, OSCAL, Markdown) now written
  to `.complytime/scan/` alongside the evaluation log, matching
  documented behavior. Previously, format reports were written to the
  workspace root. (#615)
- Generation freshness detection now tracks complypack digests alongside
  policy digests. Previously, updating a complypack without changing the
  policy would skip regeneration, causing providers to use stale
  artifacts (#583).
- OCI reference parsing now supports standard `:tag` syntax (e.g.,
  `registry.com/org/image:v0.4.0`) in addition to the existing `@version`
  notation for both policies and complypacks. Digest references
  (`@sha256:...`) are also supported. Invalid OCI references in
  `complytime.yaml` are now detected at config load time with clear
  error messages. (#594)
- `complyctl doctor` now reports invalid policy references in
  `CheckPolicyActivePeriod` and `CheckComplypacks` as explicit
  failures instead of silently skipping them. `CheckVariables`
  now surfaces per-policy resolution errors with specific messages
  instead of a silent counter. (#600)
- Scan reports now resolve assessment plan IDs to requirement IDs,
  ensuring output displays meaningful identifiers instead of internal
  plan references. Affects EvaluationLog, OSCAL, SARIF, and Markdown
  output formats.
