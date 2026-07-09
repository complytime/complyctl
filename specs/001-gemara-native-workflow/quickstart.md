# Quickstart: Gemara-Native Workflow

**Branch**: `001-gemara-native-workflow`

## Prerequisites

- Go 1.24+
- `buf` CLI (for protobuf generation)
- Access to an OCI registry (or use the mock registry for testing)

## Build

```bash
make build
```

## Workflow

### 1. Initialize workspace

```bash
complyctl init
```

`init` errors if `complytime.yaml` already exists (like `go mod init`). Prompts for policy URLs (with optional shortname ID per policy) and targets (referencing policies by effective ID), creates `complytime.yaml`, and exits. No `get`, no `doctor` — run those separately (Session 2026-02-25d).

```text
Enter policy URL: registry.example.com/policies/cis-fedora-l1-workstation@v1.0
Enter shortname ID (or press Enter to derive from URL) [cis-fedora-l1-workstation]: cis-fedora
Add another policy? (y/n): n
Enter target ID: local
Select policies for target 'local' (available: cis-fedora): cis-fedora
Enter target variables (key=value, empty to finish):
  profile=cis_workstation_l1
Workspace configuration created. (1 policy, 1 target)
```

**Result** — `complytime.yaml`:

```yaml
policies:
  - url: registry.example.com/policies/cis-fedora-l1-workstation@v1.0
    id: cis-fedora
variables:
  workspace: ./.complytime/scan
targets:
  - id: local
    policies:
      - cis-fedora
    variables:
      profile: cis_workstation_l1
```

### 1b. Manual config (CI/CD or advanced)

For CI/CD pipelines or advanced setups, create `complytime.yaml` manually instead of running `init`. Each policy URL is a full OCI reference — policies from different registries coexist:

```yaml
policies:
  - url: https://registry.example.com/policies/nist-800-53-r5@v2.0
  - url: https://registry.example.com/policies/cis-benchmark@v1.0.0
    id: cis
variables:
  workspace: ./.complytime/scan
targets:
  - id: local
    policies:
      - cis
    variables:
      profile: cis_workstation_l1
  - id: production-cluster
    policies:
      - nist-800-53-r5
    variables:
      kubeconfig: /path/to/kubeconfig
```

If `id` is omitted, it auto-derives from the last URL path segment (e.g., `nist-800-53-r5`). Targets reference policies by their effective ID (explicit or derived).

### 2. Fetch policies

```bash
complyctl get
```

Run after `init` to populate the policy cache. Performs incremental sync — only downloads new or modified content.

```text
Syncing policy 1/2: cis-fedora-l1-workstation... done
Syncing policy 2/2: cis-benchmark... done
Synchronization completed.
```

### 3. Run pre-flight diagnostics

```bash
complyctl doctor
```

Requires policy cache — run `complyctl get` first to populate. Doctor uses cached policies to resolve which providers serve which targets for variable validation (R51) and compares cached policy versions against latest available remotely (R55).

```text
✅ config: complytime.yaml valid
✅ provider/openscap: healthy (v1.2.0)
✅ policy/cis-fedora: v1.0.0 (latest)
⚠️ policy/nist-r5: cached v1.0.0, available v1.1.0 — run complyctl get to update
✅ cache: 2 cached policy store(s)
✅ provider/openscap: 1/1 global vars, 1/1 target vars
```

Use `--verbose` for per-provider variable detail:

```bash
complyctl doctor --verbose
```

```text
✅ config: complytime.yaml valid
✅ provider/openscap: healthy (v1.2.0)
✅ policy/cis-fedora: v1.0.0 (latest)
⚠️ policy/nist-r5: cached v1.0.0, available v1.1.0 — run complyctl get to update
✅ cache: 2 cached policy store(s)
✅ provider/openscap: 1/1 global vars, 1/1 target vars
   global: workspace ✅
   target[local]: profile ✅
```

Exit 0 if all blocking checks pass. Policy staleness and registry unreachability are warnings (non-blocking). Missing required variables (declared by providers via Describe) are blocking errors.

**CI/CD path**: Commit `complytime.yaml` to the repo, then run `complyctl get` and `complyctl doctor` directly. Do not run `init`.

**Troubleshooting**: Plugin-communicating commands (`generate`, `scan`, `providers`, `doctor`) write structured logs to `.complytime/complyctl.log` in the workspace directory. The log file is truncated on each run (fresh log per invocation). Simple commands (`version`, `list`, `get`, `init`) do not create a log file.

### 4. Fetch/update policies

```bash
complyctl get
```

Performs incremental sync — only downloads new or modified content. Each policy is a single multi-layer OCI manifest containing all Gemara Layers 1-3 (catalog, guidance, policy/assessments) as separate layers identified by media type.

Terminal output shows per-policy progress:

```text
Syncing policy 1/2: nist-800-53-r5... done
Syncing policy 2/2: cis-benchmark... done
Synchronization completed.
```

### 5. List available policies

```bash
complyctl list
```

Shows cached policies from the local policy cache. Plain aligned text (Session 2026-02-26e).

```text
POLICY ID                      VERSION
cis-fedora-l1-workstation      v1.0.0
nist-800-53-r5                 v2.0.0
```

### 6. Discover installed scanning providers

```bash
complyctl providers
```

Shows discovered scanning providers from `~/.complytime/providers/` — evaluator ID, path, health status, and version. Plain aligned text (Session 2026-02-26e).

```text
PROVIDER ID  PATH                            STATUS   VERSION
openscap     complyctl-provider-openscap     healthy  1.2.0
```

### 7. Generate policy artifacts (optional)

```bash
complyctl generate --policy-id nist-800-53-r5
```

Resolves dependency graph, preps scanning providers via Generate RPC, persists generated artifacts + policy digest, and outputs a structured plain-text execution plan (Session 2026-02-26e):

```text
Execution Plan: nist-800-53-r5

  Target: production-cluster
    Provider: openscap (✅ healthy)
    Requirements: 47

Generation completed.
```

**Note:** `generate` validates that required global variables are present in `complytime.yaml` before dispatching to scanning providers. Missing variables produce a clear error:

```text
Error: required global variable "workspace" not found in complytime.yaml variables section
```

This step is **optional** for simple workflows — `scan` auto-generates when needed. Use `generate` explicitly when:
- AI-driven generation is expensive and you want to review artifacts before scanning
- You want to generate once, scan multiple times (e.g., re-scan after remediation)

### 8. Execute scan

```bash
# Simple: auto-generates if needed, scans, produces EvaluationLog
complyctl scan --policy-id nist-800-53-r5

# OSCAL output
complyctl scan --policy-id nist-800-53-r5 --format oscal

# Markdown report
complyctl scan --policy-id nist-800-53-r5 --format pretty

# SARIF for security tooling
complyctl scan --policy-id nist-800-53-r5 --format sarif
```

`scan` is smart about generation:
- **No prior generate**: auto-generates (resolve graph, prep scanning providers, persist artifacts)
- **Fresh artifacts** (policy unchanged since last generate): reuses — skips Generate RPC
- **Stale artifacts** (policy updated via `get`): warns and auto-regenerates

After scan completes, a report-style summary is always displayed in the terminal (regardless of `--format`). Non-passing results shown in a 4-column plain text table. Totals line, EvaluationLog path, and formatted report path (when applicable) in conclusion (Session 2026-02-26e):

```text
Scan: nist-800-53-r5 | Target: production-cluster | 47 requirements

REQUIREMENT ID      CONTROL ID        STATUS  MESSAGE
xccdf_req_ssh_cip   ssh_ciphers       ❌      SSH service must be configured with approved ciphers
xccdf_req_pw_hash   pw_hashing        ❌      Password hashing algorithm must use SHA512
xccdf_req_audit     audit_log_rotate  ⚠️      openscap returned timeout for audit log rotation
xccdf_req_selinux   selinux_rule      ⏭️      SELinux rule not applicable to target configuration

48 requirements: 44 passed, 2 failed, 1 error, 1 skipped
Evaluation log: .complytime/scan/evaluation-log.yaml
```

With `--format`:

```bash
complyctl scan --policy-id nist-800-53-r5 --format oscal
```

```text
Scan: nist-800-53-r5 | Target: production-cluster | 47 requirements

REQUIREMENT ID      CONTROL ID        STATUS  MESSAGE
xccdf_req_ssh_cip   ssh_ciphers       ❌      SSH service must be configured with approved ciphers
xccdf_req_pw_hash   pw_hashing        ❌      Password hashing algorithm must use SHA512

47 requirements: 44 passed, 2 failed, 0 error, 0 skipped
Evaluation log: .complytime/scan/evaluation-log.yaml
OSCAL report written: ./oscal-assessment-results.json
```

**Output locations (R58)**:
- EvaluationLog (always): `.complytime/scan/` — diagnostic artifact, hidden directory
- Formatted reports (`--format`): current working directory — user-facing deliverables

Only non-passing results appear in the table. Passed results appear in the totals line only. Table shows Requirement ID + Control ID for identification, Status emoji for severity, and Message for actionability (sourced from scanning provider's `AssessmentLog.Steps[].Message`). Falls back to plain text when output is piped (R59).

## Scanning Provider Development

Scanning providers are standalone executables discovered from two directories (Session 2026-02-27): user directory (`~/.complytime/providers/`) checked first, then system directory (`/usr/libexec/complytime/providers/`) as fallback. User-installed providers take precedence. No manifest files, no configuration files — just a binary.

**Requirements:**
1. Executable name matches `complyctl-provider-*` (evaluator ID = name minus prefix)
2. Implements the `Plugin` gRPC service (scanning interface) defined in `contracts/plugin.proto`
3. Uses `hashicorp/go-plugin` handshake (Go providers can call `plugin.Serve(impl)`)

```bash
# Provider naming determines evaluator ID:
# complyctl-provider-kubernetes-evaluator → evaluator ID: kubernetes-evaluator
```

**Scanning interface methods (gRPC):**
- `Describe(DescribeRequest) → DescribeResponse` — reports health, version, and required variable names (R51). Doctor validates declared variable names against workspace config.
- `Generate(GenerateRequest) → GenerateResponse` — preps declarative policies. Receives global variables (workspace-scoped config) + test variables (per-requirement parameters from Gemara policy). Called during `complyctl generate` or auto-generate within `scan`.
- `Scan(ScanRequest) → ScanResponse` — executes compliance checks against targets. Receives targets only (target ID + target variables). No requirement_ids — provider evaluates all requirements from Generate-time state. Returns `ConfidenceLevel` enum per requirement.

**Three-tier variable model:**
- **Global variables** (Generate RPC): Workspace-scoped config from top-level `variables` in `complytime.yaml` (e.g., scan output directory). Applies to all policies and targets.
- **Target variables** (Scan RPC): Per-target runtime config from `targets[].variables` (e.g., profile, kubeconfig, auth tokens). Owned by system admin. Declare required variable *names* in `DescribeResponse.required_target_variables` (R51) — doctor validates them. Document valid *values* in your provider README.
- **Test variables** (Generate RPC): Per-requirement parameters from Gemara policy assessment plan (e.g., password hashing algorithm). Owned by policy author. Policy-defined defaults used directly — no admin overrides.

**Go scanning provider example (main.go):**

```go
package main

import (
    "context"

    "github.com/complytime/complyctl/pkg/plugin"
)

var _ plugin.Plugin = (*myProvider)(nil)

type myProvider struct{}

func (p *myProvider) Describe(_ context.Context, _ *plugin.DescribeRequest) (*plugin.DescribeResponse, error) {
    return &plugin.DescribeResponse{
        Healthy: true,
        Version: "1.0.0",
        RequiredGlobalVariables: []string{"workspace"},
        RequiredTargetVariables: []string{"profile"},
    }, nil
}
func (p *myProvider) Generate(_ context.Context, _ *plugin.GenerateRequest) (*plugin.GenerateResponse, error) {
    return &plugin.GenerateResponse{Success: true}, nil
}
func (p *myProvider) Scan(_ context.Context, req *plugin.ScanRequest) (*plugin.ScanResponse, error) {
    return &plugin.ScanResponse{}, nil
}

func main() {
    plugin.Serve(&myProvider{})
}
```

## Terminal Output

complyctl uses a two-tier output model. Only plugin-communicating commands create a log file (`.complytime/complyctl.log`, truncated per run). go-plugin output is filtered to WARN and above in the log file.

| Tier | Commands | Terminal Output | Log File | Channel |
| :--- | :--- | :--- | :--- | :--- |
| Progress | `init`, `get` | Real-time per-step status | No | stderr |
| Summary + log | `generate`, `scan`, `providers`, `doctor` | Tables, execution plans, scan summaries, diagnostics | Yes | stdout |
| No logging | `version`, `list` | Output only | No | stdout |

## Authentication

Zero custom auth code. complyctl uses `oras-credentials-go` to discover Docker credentials automatically:

- `~/.docker/config.json` → credHelpers, credsStore, inline auths
- Credential helpers: `docker-credential-desktop`, `docker-credential-gcloud`, `docker-credential-ecr-login`, etc.
- No configuration needed — if `docker login` works, `complyctl get` works.

## Testing

```bash
# Run unit tests
go test ./...

# Run E2E test with in-process mock OCI registry + test scanning provider
make test-e2e

# Build test scanning provider for local testing
make build-test-plugin
cp ./bin/complyctl-provider-test ~/.complytime/providers/

# Manual validation — see tests/e2e/README.md
```

## Comply-Pack Workflow (deferred to 002-comply-packs)

All pack CLI commands (`pack init`, `pack build`, `pack push`, `pack pull`, `pack doctor`) are deferred to the `002-comply-packs` feature branch. Pack manifest types (`PackManifest`, etc.) exist in `internal/complytime/pack.go` as a data model for 002. The pack builder is a separate tool from `complyctl` runtime (Session 2026-02-25d).

Pack design documentation will be produced in the `002-comply-packs` feature branch.

## CLI Commands

| Command | Description |
| :--- | :--- |
| `init` | Create `complytime.yaml` — prompts for PolicyEntry URLs + optional IDs + targets (Session 2026-02-25d) |
| `get` | Fetch/sync policies from OCI registry (per-registry clients from PolicyEntry URLs) |
| `list` | List cached policies (effective IDs + versions, plain text) |
| `providers` | List discovered scanning providers (evaluator ID, path, health, version; plain text) |
| `generate` | Resolve policy graph, prep scanning providers, persist artifacts, output execution plan |
| `scan` | Execute compliance scan (auto-generates if needed), display summary, produce reports |
| ~~`scan --dry-run`~~ | ~~Generate + show execution plan without scanning~~ (removed Session 2026-02-26e — use `generate` for plan preview) |
| `doctor` | Pre-flight diagnostics: config + providers + per-policy version comparison + per-provider config summary + variable validation. `--verbose` for key detail (requires policy cache) |
| `version` | Print complyctl version |
