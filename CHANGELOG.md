# Changelog

## Unreleased

### Changed

- **BREAKING**: User-scoped paths now follow the
  [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/)
  instead of the previous `~/.complytime/` layout. New default paths on Linux:
  - **Cache** (`$XDG_CACHE_HOME/complytime/`): `~/.cache/complytime/` — policies,
    complypacks
  - **Data** (`$XDG_DATA_HOME/complytime/`): `~/.local/share/complytime/` —
    providers, `state.json`

  Workspace-local `.complytime/` (config, scan output, logs) is **unchanged**.

  **Migration**: If the legacy `~/.complytime/` directory exists, complyctl
  prints a deprecation warning with migration instructions on first run. To
  migrate:
  ```bash
  # Copy data files (state, providers)
  mkdir -p ~/.local/share/complytime
  cp ~/.complytime/state.json ~/.local/share/complytime/
  cp -r ~/.complytime/providers ~/.local/share/complytime/

  # Copy cache files (policies, complypacks)
  mkdir -p ~/.cache/complytime
  cp -r ~/.complytime/policies ~/.cache/complytime/
  cp -r ~/.complytime/complypacks ~/.cache/complytime/

  # Remove legacy directory after verifying migration
  rm -rf ~/.complytime
  ```

  **Downgrade note**: Downgrading to a pre-XDG version requires manually moving
  files back to `~/.complytime/`.

  Implements [ADR-0016](https://github.com/complytime/complytime/pull/41) and
  aligns with [complypack#127](https://github.com/complytime/complypack/pull/127).
  (#734)

- **BREAKING**: `complyctl doctor` default output now includes
  `[PASS]`/`[FAIL]`/`[WARN]` labels alongside emoji status indicators.
  Scripts that parse the previous emoji-only format may need updating.
  The warning emoji changed from `⏭️` to `⚠️`. (#611, #744)

### Added

- `complyctl doctor --format text|json` flag for machine-readable
  output. `--format text` produces grep-stable `[PASS]`/`[FAIL]`/
  `[WARN]` labels without emoji; `--format json` produces structured
  JSON with `checks`, `summary`, and `blocking_failure` fields.
  `--format human` explicitly selects the default emoji output.
  When `NO_COLOR` is set, text format is selected automatically.
  (#611, #744)
- Custom trusted root support for private Sigstore instances.
  `VerificationConfig` gains a `trusted_root` field specifying a
  filesystem path to a `trusted_root.json` file for keyless
  verification against non-public Sigstore deployments.
  `NewKeylessVerifier` loads the custom trusted root via
  `root.NewTrustedRootFromPath` instead of fetching the public TUF
  root. Mutually exclusive with `key`; requires `issuer` and
  `identity`. File existence validated at config time with
  `filepath.Clean` path sanitization. (#768)

- Digest format validation at the state ingestion layer.
  `UpdatePolicyStateWithVerification` and
  `UpdateComplypackStateWithVerification` now return an error when
  the digest is non-empty and not a valid OCI content digest
  (`algorithm:hex` with correct hex length). `LoadState` warns and
  excludes entries with malformed digests while preserving valid and
  empty-digest entries. Defense-in-depth per NIST 800-53 SI-10
  input validation. (#677)

- Cross-repo integration workflow now validates EvaluationLog YAML
  output against the Gemara CUE schema with `cue vet`. Catches
  schema conformance regressions (e.g. empty `steps` arrays, missing
  required fields) before merge. CUE is also installed in the
  devcontainer for local developer validation. (#577)

- `complyctl list` now displays EVALUATOR and CONTROLS columns
  showing the evaluator type and control count for each cached
  policy. Metadata is sourced from cached state populated during
  `complyctl get`. Pre-existing caches show "-" until re-fetched.
  (#506)

- `complyctl get` now prints a post-sync summary to stderr after
  fetching a policy, showing the policy title, evaluator, control
  count, and assessment count. The summary appears only for freshly
  fetched policies. (#506)

- `PolicyState` in `state.json` gains `policy_title`,
  `policy_evaluator`, `control_count`, and `assessment_count` fields
  populated at sync time. Backward compatible via `omitempty` JSON
  tags. (#506)

### Fixed

- `complyctl doctor` no longer requires all target variables declared
  by a provider when those variables are mutually exclusive. Providers
  can now declare one-of variable groups via the
  `optional_target_variable_groups` field in the Describe protocol.
  Doctor validates that at least one member of each group is present
  per target, resolving the contradiction where `doctor` required both
  `url` and `input_path` while `scan` rejected configs with both set.
  (#759)

- **BREAKING**: `complyctl get` signature verification now resolves
  against the configured OCI registry instead of defaulting to Docker
  Hub. Previously, bare OCI references (without a registry host) were
  passed to the sigstore verifier, which resolved them against Docker
  Hub -- a different registry than where the artifact was actually
  fetched from. `BuildLookupRef()` now accepts a `registryHost`
  parameter, and `Sync`/`ComplypackSync` carry the registry host from
  the parsed policy reference. When a verifier is configured but the
  registry host is empty, `complyctl get` fails closed with an
  actionable error instead of silently verifying against the wrong
  registry. (#767)

- `complyctl get` recorded the OCI tag version (e.g., `v1.0.0`) in
  complypack state instead of the embedded `config.Version` (e.g.,
  `1.0.0`), causing a mismatch between state tracking and the on-disk
  cache directory name. (#694)

- `--debug` flag now produces visible output on stderr in addition to
  writing to the log file. Previously, `--debug` only wrote debug
  messages to `.complytime/complyctl.log` with no terminal output.
  A `Debug log: <path>` hint is printed to stderr so users know where
  the full log file lives. The flag description is updated to
  `"output debug logs to stderr and log file"`. (#614)

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
- `complyctl get` now detects and rejects duplicate evaluator-ids across
  complypack entries. Previously, multiple complypacks resolving to the
  same evaluator-id caused non-deterministic digest selection during
  scan, leading to unnecessary regeneration or stale artifacts (#647).
- `complyctl get` now re-fetches complypack artifacts when the cache
  directory is missing, even if `state.json` records a matching digest.
  Previously, a deleted cache directory caused the sync to be permanently
  skipped until the user manually cleared state (#649).

### Changed

- `complyctl doctor` output redesigned with grouped sections, nested
  sub-results, and per-policy registry warnings. Check results are
  organized under labeled section headers (Providers, Policies, Cache,
  Complypacks, Workspace, Verification) following the scan prerequisite
  sequence. Variable checks nest under their parent provider, and
  active-period checks nest under their parent policy. Unreachable
  registries now emit per-policy warnings instead of silently skipping
  affected policies. Complypacks are listed individually by evaluator-id.
  Summary counts top-level results and their direct children;
  verbose detail (grandchildren) is excluded for stable counts
  regardless of `--verbose`. (#692)
- Release workflow gains preflight validation gate: tag format and
  uniqueness check, semver ordering verification, CI status query
  via GitHub Checks API, unreleased commits guard, and idempotent
  annotated tag creation. Concurrency group prevents parallel
  releases. (#560)

- All commands now accept `--workspace` flag to specify workspace directory
- `NewWorkspace()` function signature changed from `NewWorkspace()` to `NewWorkspace(baseDir string)`
- Log file and scan output paths are now relative to resolved workspace directory

### Removed

- **BREAKING**: Removed collector export infrastructure (`COMPLYTIME_EXPORT_ENABLED`, `collector:` config block, Export RPC). This was speculative infrastructure added before backend design was finalized. Export functionality will be redesigned and reintroduced when the backend shape is known. (#606)

  **Migration**: Users who configured `collector:` in `complytime.yaml` or set `COMPLYTIME_EXPORT_ENABLED=true` should remove these configurations. Export functionality is not available in this release. If you require evidence export to a Beacon collector, remain on the previous version until the redesigned export infrastructure is released.

### Added

- Emoji status indicators in markdown report (`--format pretty`): Controls
  table rows, summary counts headers, and findings group headers now display
  emoji prefixes (`✅`, `❌`, `⏭️`, `⚠️`) matching the terminal scan
  summary. Makes the report directly usable as a GitHub Actions Step Summary
  via `cat report-*.md >> $GITHUB_STEP_SUMMARY` (#697)
- Complypack cache version management: `COMPLYTIME_CACHE_VERSIONS`
  environment variable configures how many complypack versions to
  retain per evaluator-id in the local cache (default: 1, preserving
  current single-version behavior). When set to N>1, `complyctl get`
  retains up to N versions, enabling version switching without
  re-downloading from the OCI registry. `complyctl doctor` reports
  complypack cache disk usage and orphaned versions not tracked in
  state.json. (#676)
- Signature verification for OCI artifacts: `complyctl get` verifies
  cosign/sigstore signatures via `sigstore-go` when `verification:`
  is configured in `complytime.yaml`. Supports keyless (OIDC issuer +
  identity) and keyed (public key path) verification. Pre-copy
  verification ensures unverified content never reaches the local
  cache. `--skip-verify` flag bypasses verification. `complyctl list`
  displays a VERIFIED column. `complyctl doctor` reports verification
  status of cached artifacts. (#643)
- Per-policy and per-complypack verification configuration: each entry
  in the `policies` or `complypacks` section of `complytime.yaml` can
  now declare its own `verification:` block or `skip_verify: true` to
  override the workspace-level verification default. Workspace-level
  `verification:` applies to all entries that do not provide their own.
  When multiple entries fail verification, all entries are still
  attempted and errors are collected rather than failing on the first
  error. Verifiers are cached by configuration to avoid redundant
  construction for entries sharing the same verification settings.
  (#680)
- Container-based acceptance tests (`make test-acceptance`): runs
  complyctl against a real zot OCI registry in a compose stack,
  validating OCI artifact interop that in-process mock registries
  cannot catch (#653)
- Redesigned markdown report (`--format pretty`) with summary metadata
  table, pass rate, grouped controls table with messages, findings
  section grouped by result type with recommendation and collapsible
  evidence, and evaluation log wrapped in collapsible `<details>` (#572)
- `Evidence` and `Recommendation` fields added to provider gRPC API
  (`AssessmentLog` proto message). Providers can now return evidence
  collected during assessment and remediation guidance for non-passing
  results. Fields are optional and backward compatible (#572)
- `complyctl scan --show-passing` flag (default: `true`) shows passing
  controls in the terminal summary table alongside non-passing results.
  Set `--show-passing=false` or `COMPLYTIME_SHOW_PASSING=false` to
  restore the previous behavior of only showing non-passing controls.
  If your CI pipeline parses scan stdout, add `--show-passing=false`
  or set `COMPLYTIME_SHOW_PASSING=false` to preserve previous behavior.
  (#438)
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
- `complyctl get` emits a NOTE to stderr when a freshly fetched policy
  or complypack was fetched without signature verification, directing
  users to the planned sigstore-go integration (#643) (#607)
- `complyctl list` displays a DIGEST column showing the abbreviated OCI
  manifest digest for each cached policy. Uncached policies show `-`. (#607)
- `SyncPolicy` returns `(bool, error)` to gate warnings on fresh fetches.
  THR02 mitigations MIT01 (warning) and MIT03 (digest visibility)
  documented in the threat catalog. (#607)

### Deprecated

- The `@version` notation for policy URLs (e.g., `registry.com/repo@v1.0`)
  is deprecated. Use standard OCI `:tag` syntax instead (e.g.,
  `registry.com/repo:v1.0`). In the OCI Distribution Spec, `@` is
  exclusively a digest separator. A deprecation warning is now emitted
  when `@version` syntax is detected. `@version` support will be removed
  in a future release. (#600)


