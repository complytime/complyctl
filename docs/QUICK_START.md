# Quick Start

## Step 1: Install complyctl

See [INSTALLATION.md](INSTALLATION.md).

## Step 2: Install a provider

Scanning providers are standalone executables placed in `~/.complytime/providers/`. The filename determines the evaluator ID.

```bash
mkdir -p ~/.complytime/providers
cp bin/complyctl-provider-openscap ~/.complytime/providers/
```

Naming convention: `complyctl-provider-<evaluator-id>`. The CLI strips the prefix to derive the evaluator ID used for routing.

For the openscap provider, install prerequisites:
- `openscap-scanner` package
- `scap-security-guide` package

See the [Provider Guide](https://github.com/complytime/complytime-providers/blob/main/docs/provider-guide.md) for authoring details.

## Step 3: Create workspace config

Create `complytime.yaml` in your working directory. This is the runtime configuration — it declares targets, variables, and policy selections.

```yaml
version: 1
policies:
  - url: registry.example.com/policies/nist-800-53-r5@v1.0
    id: nist
targets:
  - id: my-system
    policies:
      - nist
    variables:
      api_token: ${MY_API_TOKEN}
```

Or use interactive setup:

```bash
complyctl init
```

`init` prompts for policy URLs, IDs, and targets when no `complytime.yaml` exists.

**Variable expansion**: Only `targets[].variables` supports `${VAR}` environment variable substitution. Use this for secrets and per-target credentials. Top-level `variables` are workspace constants passed to providers as-is — `${...}` references there are **not** expanded.

## Step 4: Fetch policies

```bash
complyctl get
```

Downloads Gemara policies from the OCI registry into the local cache (`~/.complytime/policies/`). Incremental — only fetches new or modified content.

## Step 5: Verify cache

```bash
complyctl list
```

Displays cached policies and their versions.

## Step 6: Generate

```bash
complyctl generate --policy-id nist-800-53-r5
```

Resolves the policy dependency graph, extracts assessment configurations, and dispatches to the matching provider via Generate RPC.

## Step 7: Scan

```bash
# EvaluationLog (default)
complyctl scan --policy-id nist-800-53-r5

# Markdown report
complyctl scan --policy-id nist-800-53-r5 --format pretty

# OSCAL assessment-results
complyctl scan --policy-id nist-800-53-r5 --format oscal

# SARIF
complyctl scan --policy-id nist-800-53-r5 --format sarif
```

Output written to `./.complytime/scan/`.

## Authentication

complyctl uses Docker credential helpers via `oras-credentials-go`. No custom configuration needed — if `docker login` works, `complyctl get` works.

Supported sources:
- `~/.docker/config.json` (credHelpers, credsStore, inline auths)
- Credential helpers: `docker-credential-desktop`, `docker-credential-gcloud`, `docker-credential-ecr-login`, etc.
