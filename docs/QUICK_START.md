# Quick Start

## Step 1: Install complyctl

See [INSTALLATION.md](INSTALLATION.md).

## Step 2: Install a provider

Scanning providers are standalone executables that integrate
complyctl with policy engines. At least one provider must be
installed before `complyctl scan` can run.

### Available providers

| Provider | Binary | What it evaluates | Prerequisites |
|----------|--------|-------------------|---------------|
| [openscap](https://github.com/complytime/complytime-providers/blob/main/cmd/openscap-provider/docs/configuration.md) | `complyctl-provider-openscap` | SCAP policies (CIS, STIG, HIPAA, OSPP, etc.) | `openscap-scanner`, `scap-security-guide` |
| [ampel](https://github.com/complytime/complytime-providers/tree/main/cmd/ampel-provider) | `complyctl-provider-ampel` | GitHub / GitLab branch protection | `snappy`, `ampel`, `GITHUB_TOKEN` or `GITLAB_TOKEN` |
| [opa](https://github.com/complytime/complytime-providers/tree/main/cmd/opa-provider) | `complyctl-provider-opa` | OPA/Rego policies via conftest | `conftest`, `git` |

Pre-built Linux binaries are available from the
[complytime-providers releases](https://github.com/complytime/complytime-providers/releases/latest)
page. To build from source, see the
[complytime-providers README](https://github.com/complytime/complytime-providers#install).
Provider binaries go in `~/.complytime/providers/`.

See the [Provider Guide](https://github.com/complytime/complytime-providers/blob/main/docs/provider-guide.md) for authoring new providers.

### Install provider prerequisites

Each provider requires external tools on `PATH`. Install the
prerequisites for the provider(s) you plan to use.

**Ampel:**

```bash
go install github.com/carabiner-dev/snappy@v0.2.4
go install github.com/carabiner-dev/ampel/cmd/ampel@v1.3.1
```

**OPA:**

```bash
go install github.com/open-policy-agent/conftest@v0.68.2
```

`git` is also required and is typically already installed.

**OpenSCAP** (Fedora / RHEL / CentOS):

```bash
sudo dnf install openscap-scanner scap-security-guide
```

### Verify the installation

```bash
complyctl doctor
```

`doctor` checks whether provider binaries are discovered and
whether each provider's prerequisites are met. It also reports
complypack cache health: total disk usage, orphaned versions
(on disk but not tracked in state.json), and untracked versions
(no state.json exists), with a suggestion to run `complyctl get`
to rebuild state. You can also list discovered providers
directly:

```bash
complyctl providers
```

## Step 3: Create workspace config

Create `complytime.yaml` in your working directory. This is the runtime configuration — it declares policies, targets, and variables.

```yaml
policies:
  - url: <oci-reference>
    id: <short-alias>
complypacks:  # optional — provider-specific content bundles
  - url: <oci-reference>
    id: <short-alias>

variables:
  key: value

targets:
  - id: <target-id>
    policies:
      - <policy-id>
    variables:
      key: value
```

| Section | Purpose |
|---------|---------|
| `policies` | OCI references to Gemara policy bundles. `id` is a short alias used by targets and for provider routing. |
| `complypacks` | Optional OCI references to provider-specific content bundles (policies, data files, scripts). Fetched alongside policies during `complyctl get`. |
| `variables` | Workspace-scoped constants passed to all providers (e.g., custom policy directories). |
| `targets` | Systems to evaluate. Each target selects one or more policies and provides provider-specific variables. |

**Variable expansion**: Only `targets[].variables` supports `${VAR}` environment variable substitution. Use this for secrets and per-target credentials. Top-level `variables` are workspace constants passed to providers as-is — `${...}` references there are **not** expanded.

### Example: ampel branch protection

```yaml
policies:
  - url: quay.io/complytime/policies-ampel-branch-protection:latest
    id: ampel-bp

targets:
  - id: my-repo
    policies:
      - ampel-bp
    variables:
      url: https://github.com/myorg/myrepo
      specs: builtin:github/branch-rules.yaml
```

See the [ampel provider configuration](https://github.com/complytime/complytime-providers/blob/main/cmd/ampel-provider/docs/configuration.md) for all target variables.

### Example: CIS Fedora L1 (OpenSCAP)

```yaml
policies:
  - url: quay.io/complytime/policies-cis-fedora-l1-workstation:latest
    id: cis-fedora-l1

targets:
  - id: my-system
    policies:
      - cis-fedora-l1
    variables:
      profile: xccdf_org.ssgproject.content_profile_cis_workstation_l1
```

Or use interactive setup:

```bash
complyctl init
```

`init` prompts for policy URLs, IDs, and targets when no `complytime.yaml` exists.

Available policy bundles are listed in the [complytime-policies usage guide](https://github.com/complytime/complytime-policies/blob/main/docs/usage.md).

## Step 4: Fetch policies and complypacks

```bash
complyctl get
```

Downloads Gemara policies from the OCI registry into the local cache (`~/.complytime/policies/`). If `complypacks:` entries are configured in `complytime.yaml`, their artifacts are also fetched into `~/.complytime/complypacks/`. Incremental — only fetches new or modified content.

### Complypack cache versioning

By default, complyctl retains one complypack version per evaluator-id. When you switch versions in `complytime.yaml` (e.g., from `:v2.0.0` to `:v1.0.0`), complyctl checks the local cache first and serves the previously cached version without a network fetch. This avoids re-downloads when switching between known versions.

To retain multiple versions simultaneously, set the `COMPLYTIME_CACHE_VERSIONS` environment variable:

```bash
export COMPLYTIME_CACHE_VERSIONS=3
complyctl get
```

| Value | Behavior |
|-------|----------|
| Unset or `1` | Default. One version per evaluator-id (current behavior). |
| `N` (> 1) | Retains up to N versions per evaluator-id. Oldest versions are evicted first. |
| `0` or negative | Clamped to `1`. |
| Non-integer | Warning logged, falls back to `1`. |

When a verifier is configured in `complytime.yaml`, local cache hits are re-verified via the registry API before being accepted.

## Step 5: Verify cache

```bash
complyctl list
```

Displays cached policies and their versions.

## Step 6: Generate

```bash
complyctl generate --policy-id ampel-bp
```

Resolves the policy dependency graph, extracts assessment configurations, and dispatches to the matching provider via Generate RPC.

## Step 7: Scan

Scan all targets for a policy:

```bash
# EvaluationLog (default)
complyctl scan --policy-id ampel-bp

# Markdown report
complyctl scan --policy-id ampel-bp --format pretty

# OSCAL assessment-results
complyctl scan --policy-id ampel-bp --format oscal

# SARIF
complyctl scan --policy-id ampel-bp --format sarif
```

Or scan a single target (policy is inferred when the target has exactly one):

```bash
complyctl scan my-repo
```

`complyctl scan` automatically calls `generate` if artifacts are missing or the policy digest has changed.

Output written to `./.complytime/scan/`.

> **Exit codes:** `scan` exits 0 on completion regardless of whether controls
> passed or failed — policy findings are data, not errors. The command exits
> non-zero only for operational errors (provider failures, bad configuration,
> or zero requirements assessed). To gate a pipeline on compliance results,
> parse the `--format` output (SARIF, OSCAL) with your policy engine.

## Authentication

**OCI registry:** complyctl uses Docker credential helpers via `oras-credentials-go`. No custom configuration needed — if `docker login` works, `complyctl get` works.

Supported sources:
- `~/.docker/config.json` (credHelpers, credsStore, inline auths)
- Credential helpers: `docker-credential-desktop`, `docker-credential-gcloud`, `docker-credential-ecr-login`, etc.

**GitHub / GitLab API (ampel provider):** Set the appropriate token before scanning:

```bash
export GITHUB_TOKEN=ghp_your_token_here   # GitHub
export GITLAB_TOKEN=glpat-your_token_here  # GitLab
```

Per-repository tokens can also be configured via the `access_token` target variable. See the [ampel provider configuration](https://github.com/complytime/complytime-providers/blob/main/cmd/ampel-provider/docs/configuration.md) for details.
