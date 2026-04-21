# complyctl

[![OpenSSF Best Practices status](https://www.bestpractices.dev/projects/9761/badge)](https://www.bestpractices.dev/projects/9761)
[![GoDoc](https://img.shields.io/static/v1?label=godoc&message=reference&color=blue)](https://pkg.go.dev/github.com/complytime/complyctl)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/complytime/complyctl/badge)](https://scorecard.dev/viewer/?uri=github.com/complyctl/complyctl)

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
│  │              │  complytime.yaml defines:                      │
│  │ ./complytime │   - registry URL                               │
│  │   .yaml      │   - policy IDs + versions                      │
│  │              │   - targets + variables                        │
│  │ ./.comply-   │                                                │
│  │   time/scan/ │                                                │
│  │  (output)    │  Scan output (EvaluationLog, OSCAL,            │
│  └──────────────┘   SARIF, Markdown) written to workspace        │
└──────────────────────────────────────────────────────────────────┘
```

**Components:**

| Component | Description |
|:---|:---|
| **OCI Registry** | Remote store for Gemara policies. Each policy is a multi-layer OCI manifest containing catalog, guidance, and policy/assessment YAML layers distinguished by media type. |
| **Workspace** | Current directory containing `complytime.yaml`. Defines which registry, policies, and targets to use. Scan output lands in `./.complytime/scan/`. |
| **Cache** | Local OCI Layout stores under `~/.complytime/policies/`. One store per policy ID. `state.json` tracks digests for incremental sync. |
| **Providers** | Standalone executables in `~/.complytime/providers/` matching the `complyctl-provider-*` naming convention. Communicate via gRPC (`Describe`, `Generate`, `Scan`). Evaluator ID derived from filename. |
| **CLI** | Orchestrates the workflow: fetch policies, resolve dependency graphs, dispatch to providers, produce compliance reports. |

## Documentation

- [Installation](./docs/INSTALLATION.md)
- [Quick Start](./docs/QUICK_START.md)
- [Provider Guide](https://github.com/complytime/complytime-providers/blob/main/docs/provider-guide.md)
- [E2E Testing](./tests/e2e/README.md)

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

Global flag: `--debug` / `-d` — output debug logs.

### `init`

```bash
complyctl init
```

Creates a workspace configuration file (`complytime.yaml`). When one already exists, validates and runs `get` automatically.

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
# Default: EvaluationLog only
complyctl scan --policy-id nist-800-53-r5

# OSCAL assessment-results
complyctl scan --policy-id nist-800-53-r5 --format oscal

# Markdown report
complyctl scan --policy-id nist-800-53-r5 --format pretty

# SARIF for security tooling
complyctl scan --policy-id nist-800-53-r5 --format sarif
```

| Flag | Short | Description |
|:---|:---|:---|
| `--policy-id` | `-p` | Policy ID to scan (required) |
| `--format` | `-f` | Output format: `oscal`, `pretty`, `sarif` |

Output written to `./.complytime/scan/`.

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
# complytime.yaml
policies:
  - url: registry.example.com/policies/nist-800-53-r5@v1.0.0
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
| `policies[].url` | Full OCI reference (registry + repository + optional `@version`) |
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
