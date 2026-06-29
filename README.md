# complyctl

[![OpenSSF Best Practices status](https://www.bestpractices.dev/projects/9761/badge)](https://www.bestpractices.dev/projects/9761)
[![GoDoc](https://img.shields.io/static/v1?label=godoc&message=reference&color=blue)](https://pkg.go.dev/github.com/complytime/complyctl)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/complytime/complyctl/badge)](https://scorecard.dev/viewer/?uri=github.com/complytime/complyctl)

A lightweight compliance runtime that pulls [Gemara](https://gemara.openssf.org/) policies from an OCI registry and executes scans via providers.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│  Host                                                            │
│                                                                  │
│  ┌──────────────┐      complyctl get   ┌───────────────────────┐ │
│  │ OCI Registry │ ◄──────────────────  │                       │ │
│  │              │  ───────────────────►│    complyctl CLI      │ │
│  │  Gemara      │   catalog + policy   │                       │ │
│  │  policies    │   layers (YAML)      │ init / get / list     │ │
│  └──────────────┘                      │ generate / scan       │ │
│                                        │ doctor / providers    │ │
│                                        │ version               │ │
│                                        └─────┬────────┬────────┘ │
│                                              │        │          │
│                                 ┌────────────┘        │          │
│                                 │                     │          │
│                                 ▼                     ▼          │
│                       ┌──────────────┐    ┌────────────────┐     │
│                       │    Cache     │    │   Providers    │     │
│                       │              │    │                │     │
│                       │ ~/.complytime│    │ ~/.complytime/ │     │
│                       │  /policies/  │    │  providers/    │     │
│                       │  state.json  │    │                │     │
│                       │              │    │ complyctl-     │     │
│                       │ OCI Layout   │    │  provider-*    │     │
│                       │ per policy   │    │                │     │
│                       └──────────────┘    │ gRPC: Describe │     │
│                                           │ Generate, Scan │     │
│  ┌──────────────┐                         └────────────────┘     │
│  │  Workspace   │                                                │
│  │              │  .complytime/complytime.yaml defines:           │
│  │ .complytime/ │   - registry URL                               │
│  │  complytime  │   - policy IDs + versions                      │
│  │   .yaml      │   - targets + variables                        │
│  │  scan/       │                                                │
│  │  (output)    │  Scan output (EvaluationLog, OSCAL,            │
│  └──────────────┘   SARIF, Markdown) written to workspace        │
└──────────────────────────────────────────────────────────────────┘
```

**Components:**

| Component | Description |
|:---|:---|
| **OCI Registry** | Remote store for Gemara policies. Supports two OCI manifest layouts: split-layer (distinct media types per artifact) and Gemara bundle format (single artifact media type with annotation-based differentiation). Both formats are auto-detected and resolved transparently. |
| **Workspace** | Resolved workspace directory containing `.complytime/complytime.yaml` (or legacy `complytime.yaml` at root). Configurable via `--workspace` flag or `COMPLYTIME_WORKSPACE` env var. Defines which registry, policies, and targets to use. Scan output lands in `.complytime/scan/`. |
| **Cache** | Local OCI Layout stores under `~/.complytime/policies/`. One store per policy ID. `state.json` tracks digests for incremental sync. |
| **Providers** | Standalone executables in `~/.complytime/providers/` matching the `complyctl-provider-*` naming convention. Communicate via gRPC (`Describe`, `Generate`, `Scan`). Evaluator ID derived from filename. |
| **CLI** | Orchestrates the workflow: fetch policies, resolve dependency graphs, dispatch to providers, produce compliance reports. |

## Documentation

- [Installation](./docs/INSTALLATION.md)
- [Quick Start](./docs/QUICK_START.md)
- [Provider Guide](https://github.com/complytime/complytime-providers/blob/main/docs/provider-guide.md)
- [E2E Testing](./tests/e2e/README.md)
- [Testing Environment](./docs/TESTING_ENVIRONMENT.md)

## CLI Commands

| Command | Description |
|:---|:---|
| `init` | Create a workspace configuration file |
| `get` | Fetch new/modified policies from OCI registry and update cache |
| `list` | List cached Gemara policies |
| `generate` | Generate policy graph and invoke providers |
| `scan` | Scan targets and produce compliance reports |
| `doctor` | Run pre-flight diagnostics on the workspace |
| `providers` | List discovered scanning providers and their health status |
| `version` | Print version |

Global flags: 
- `--debug` / `-d` — output debug logs
- `--workspace` / `-w` — workspace directory (project root containing `.complytime/`, defaults to current directory)

### Run Commands from Any Directory

Use the `--workspace` flag to run commands from any directory:

```bash
# Run from a different directory
complyctl scan --workspace ~/projects/myapp

# Using relative path
complyctl scan --workspace ../myapp

# Using environment variable
export COMPLYTIME_WORKSPACE=~/projects/myapp
complyctl scan
```

### Config File Location

complyctl organizes all workspace-specific files under `.complytime/` to keep your repository root clean and avoid configuration conflicts.

- `.complytime/complytime.yaml` - Configuration file (policies, targets, variables)
- `.complytime/scan/` - Scan output reports
- `.complytime/complyctl.log` - Debug log file
- `.complytime/generation/` - Generation state (per-policy freshness tracking)

**Note:** For backward compatibility, complyctl still supports `complytime.yaml` at the repository root, but this location is deprecated. Move your config to `.complytime/complytime.yaml`:

```bash
mkdir -p .complytime
mv complytime.yaml .complytime/complytime.yaml
```

### `init`

```bash
complyctl init
```

Creates a workspace configuration file (`.complytime/complytime.yaml`). Errors if one already exists.

### `get`

```bash
complyctl get
```

Performs incremental sync from the OCI registry defined in `complytime.yaml`. Only downloads new or modified content. Uses Docker credential helpers for authentication — if `docker login` works, `complyctl get` works.

### `list`

```bash
complyctl list
complyctl list --policy-id nist-800-53-r5
```

| Flag | Description |
|:---|:---|
| `--policy-id` | Filter output to a single policy |

### `generate`

```bash
complyctl generate --policy-id nist-800-53-r5
```

| Flag | Short | Description |
|:---|:---|:---|
| `--policy-id` | `-p` | Policy ID to generate (required) |

Resolves the policy dependency graph from cache, extracts assessment configurations, applies parameter overrides from `complytime.yaml`, and dispatches to the matching provider via Generate RPC.

### `scan`

```bash
# Scan a specific target (policy inferred if target has exactly one)
complyctl scan prod

# Scan a specific target for a specific policy
complyctl scan prod --policy-id nist-800-53-r5

# Scan all targets for a policy
complyctl scan --policy-id nist-800-53-r5

# With output format
complyctl scan prod --format oscal
complyctl scan --policy-id nist-800-53-r5 --format pretty
complyctl scan --policy-id nist-800-53-r5 --format sarif

```

| Argument / Flag | Short | Description |
|:---|:---|:---|
| `[target]` | | Optional target ID to scope the scan (from `complytime.yaml`) |
| `--policy-id` | `-p` | Policy ID to scan (required when no target is given, or target has multiple policies) |
| `--format` | `-f` | Output format: `oscal`, `pretty`, `sarif` |

When a target is specified and references exactly one policy, `--policy-id` is inferred.
At least one of `[target]` or `--policy-id` is required.

Output written to `./.complytime/scan/`.

#### Exit codes

| Exit Code | Meaning |
|:---|:---|
| `0` | Scan completed -- all targets evaluated (findings, if any, are in the report) |
| non-zero | Operational error -- one or more targets could not be evaluated, or zero requirements assessed (partial results written before exit) |

Policy violations (failed requirements) do **not** cause a non-zero exit.
Operational errors (missing tools, clone failures, auth errors, zero
requirements assessed) do.

### `doctor`

```bash
complyctl doctor
complyctl doctor --verbose
```

Validates workspace configuration, provider health, cache integrity, and provider variable requirements. Use `--verbose` for per-provider variable detail.

### `providers`

```bash
complyctl providers
```

Lists discovered scanning providers with their evaluator ID, path, health status, and version.

## Workspace Configuration

```yaml
# .complytime/complytime.yaml
policies:
  - url: registry.example.com/policies/nist-800-53-r5:v1.0.0
    id: nist
  - url: registry.example.com/policies/cis-benchmark
variables:
  output_dir: /tmp/scan-results
targets:
  - id: production-cluster
    policies:
      - nist
    variables:
      kubeconfig: /path/to/kubeconfig
      api_token: ${MY_API_TOKEN}
```

| Field | Description |
|:---|:---|
| `policies[].url` | Full OCI reference (registry + repository + optional `:tag`) |
| `policies[].id` | Optional shortname; if omitted, derived from last path segment of URL |
| `variables` | Workspace-scoped constants passed to providers via Generate RPC |
| `targets[].id` | Scan target identifier |
| `targets[].policies` | List of effective policy IDs to evaluate against this target |
| `targets[].variables` | Provider-specific key-value pairs; supports `${VAR}` env substitution |

## Contributing

- [Contributing Guidelines](./docs/CONTRIBUTING.md)
- [Style Guide](./docs/STYLE_GUIDE.md)
- [Code of Conduct](./docs/CODE_OF_CONDUCT.md)

*Interested in writing a provider?* See the [Provider Guide](https://github.com/complytime/complytime-providers/blob/main/docs/provider-guide.md).
