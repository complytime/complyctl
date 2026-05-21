# Quick Start

## Step 1: Install complyctl

See [INSTALLATION.md](INSTALLATION.md).

## Step 2: Install a provider

Scanning providers are standalone executables placed in `~/.complytime/providers/`. The filename determines the evaluator ID.

```bash
mkdir -p ~/.complytime/providers
cp bin/complyctl-provider-<name> ~/.complytime/providers/
```

Naming convention: `complyctl-provider-<evaluator-id>`. The CLI strips the prefix to derive the evaluator ID used for routing.

### Available providers

| Provider | Binary | What it evaluates | Prerequisites |
|----------|--------|-------------------|---------------|
| [openscap](https://github.com/complytime/complytime-providers/blob/main/cmd/openscap-provider/docs/configuration.md) | `complyctl-provider-openscap` | SCAP policies (CIS, STIG, HIPAA, OSPP, etc.) | `openscap-scanner`, `scap-security-guide` |
| [ampel](https://github.com/complytime/complytime-providers/tree/main/cmd/ampel-provider) | `complyctl-provider-ampel` | GitHub / GitLab branch protection | `snappy`, `ampel`, `GITHUB_TOKEN` or `GITLAB_TOKEN` |

See the [Provider Guide](https://github.com/complytime/complytime-providers/blob/main/docs/provider-guide.md) for authoring details.

## Step 3: Create workspace config

Create `complytime.yaml` in your working directory. This is the runtime configuration — it declares policies, targets, and variables.

```yaml
policies:
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
