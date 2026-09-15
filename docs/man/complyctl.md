% COMPLYCTL(1) Complyctl Manual
% Marcus Burghardt <maburgha@redhat.com>
% September 2026

# NAME

complyctl - compliance assessment CLI using provider-based scanning

# SYNOPSIS

**complyctl** [command] [flags]

# DESCRIPTION

Complyctl is a lightweight compliance runtime that pulls Gemara policies
from an OCI registry and executes scans via providers, producing
compliance reports in multiple formats (EvaluationLog, OSCAL, SARIF,
Markdown).

## Providers

Providers are standalone executables that integrate complyctl with
policy engines such as OpenSCAP, Ampel, and OPA. Each provider converts
policy content into the engine's native input format and translates the
raw results back into the schema used by complyctl for report
generation.

Providers communicate with complyctl via gRPC and can be authored in any
language. They are distributed separately (e.g., via the
**complytime-providers** package) and discovered automatically by naming
convention (**complyctl-provider-\***) from the following directories:

- **~/.local/share/complytime/providers/** (user directory, takes
  precedence)
- **/usr/libexec/complytime/providers/** (system directory)

Run **complyctl providers** to list discovered providers on your system
and **complyctl doctor** to validate the full stack including provider
health and variable requirements.

See the complytime-providers repository at
<https://github.com/complytime/complytime-providers> for available
providers and their documentation. See the provider authoring guide at
<https://github.com/complytime/complytime-providers/blob/main/docs/provider-guide.md>
for writing custom providers.

# COMMANDS

## init

Create a workspace configuration file
(**.complytime/complytime.yaml**). Interactively prompts for policies
and targets. Errors if the configuration file already exists.

```
complyctl init
```

## get

Fetch policies and complypacks from OCI registries into the local
cache. Performs incremental sync — only downloads new or modified
content. Uses Docker credential helpers for authentication.

```
complyctl get
complyctl get --skip-verify
```

**-t**, **--timeout** *duration*
: Maximum time for the get operation (default: **5m**).

**--skip-verify**
: Skip signature verification for fetched artifacts.

## list

List cached Gemara policies with their evaluator, control count,
digest, and verification status.

```
complyctl list
complyctl list --policy-id nist-800-53-r5
```

**--policy-id** *string*
: Filter output to a single policy.

## generate

Resolve the policy dependency graph from cache, extract assessment
configurations, apply parameter overrides from **complytime.yaml**,
and dispatch to the matching provider via the Generate RPC.

```
complyctl generate --policy-id nist-800-53-r5
```

**-p**, **--policy-id** *string*
: Policy ID to generate (**required**).

**-t**, **--timeout** *duration*
: Maximum time for the generate operation (default: **5m**).

## scan

Scan targets and produce compliance reports. When a target is specified
and references exactly one policy, **--policy-id** is inferred. At
least one of *[target]* or **--policy-id** is required.

Output is written to **.complytime/scan/**.

```
complyctl scan --policy-id my-policy
complyctl scan my-target
complyctl scan my-target --policy-id my-policy --format pretty
complyctl scan --policy-id my-policy --log-format json
```

**-p**, **--policy-id** *string*
: Policy ID to scan.

**-f**, **--format** *string*
: Additional output format: **oscal**, **pretty** (Markdown), **sarif**.
  The default EvaluationLog is always produced regardless of this flag.

**-t**, **--timeout** *duration*
: Maximum time for the scan operation (default: **5m**).

**--show-passing**
: Include passing controls in the terminal scan summary table
  (default: **true**). Can also be set via **COMPLYTIME_SHOW_PASSING**.

**--log-format** *string*
: EvaluationLog serialization format: **yaml**, **json**
  (default: **yaml**). Can also be set via **COMPLYTIME_LOG_FORMAT**.

## doctor

Run pre-flight diagnostics on the workspace. Checks provider
discovery, policy cache integrity, configuration validation,
complypack availability, and cache health (disk usage, orphaned
versions).

```
complyctl doctor
complyctl doctor --verbose
complyctl doctor --format text
complyctl doctor --format json
```

**--verbose**
: Expand per-provider variable detail.

**-f**, **--format** *string*
: Output format: **human** (emoji, default), **text** (grep-friendly
  plain labels: [PASS]/[FAIL]/[WARN]), **json** (structured machine
  output). When **NO_COLOR** is set, **text** is selected
  automatically.

## providers

List discovered scanning providers with their evaluator ID, path,
health status, version, and cached complypack versions.

```
complyctl providers
```

## version

Print the version.

```
complyctl version
```

## completion

Generate shell completion scripts for Bash, Zsh, Fish, or PowerShell.

```
complyctl completion bash
complyctl completion zsh
complyctl completion fish
```

# OPTIONS

These options apply to all commands.

**-d**, **--debug**
: Output debug logs to stderr and the workspace log file.

**-w**, **--workspace** *directory*
: Workspace directory. Defaults to the current directory. Can also be
  set via **COMPLYTIME_WORKSPACE**.

**-h**, **--help**
: Show help for complyctl or any subcommand.

Run **complyctl [command] --help** for more information about a
specific command.

# CONFIGURATION

Complyctl reads its workspace configuration from
**.complytime/complytime.yaml** in the current (or **--workspace**)
directory. Create the file with **complyctl init** or write it
manually.

## policies

OCI references to Gemara policy bundles. At least one is required.

```
policies:
  - url: ghcr.io/myorg/policies/nist-800-53-r5:v1.0.0
    id: nist          # optional; derived from URL path if omitted
  - url: ghcr.io/myorg/policies/cis-benchmark:latest
```

**url** (required)
: Full OCI reference including registry, repository, and optional
  **:tag** or **@sha256:digest** (e.g.,
  **registry.example.com/policies/my-policy:v1.0**).

**id** (optional)
: Short alias used by **targets[].policies** to reference this entry.
  If omitted, the last path segment of the URL is used (e.g.,
  **ghcr.io/org/policies/nist-r5:v1.0** derives **nist-r5**).

**verification** (optional)
: Per-entry verification override. Same fields as the workspace-level
  **verification** block (see below). Overrides the workspace-level
  config for this entry.

**skip_verify** (optional)
: Set to **true** to skip verification for this entry even when
  workspace-level verification is configured. Mutually exclusive with
  per-entry **verification**.

## complypacks

OCI references to provider-specific content bundles (Rego policies,
XCCDF profiles, mapping files). Fetched alongside policies during
**complyctl get**. Each complypack is matched to a provider by its
evaluator-id.

```
complypacks:
  - url: ghcr.io/myorg/complypacks/ampel-bp:v1.0.0
    id: ampel-bp-pack
```

Fields are the same as **policies** (url, id, verification,
skip_verify). Run **complyctl providers** to see which providers have
complypacks and their evaluator IDs.

## targets

Systems to evaluate. Each target selects one or more policies by their
effective ID and provides provider-specific variables.

```
targets:
  - id: my-repo
    policies:
      - nist
    variables:
      url: https://github.com/myorg/myrepo
      access_token: ${GITHUB_TOKEN}
```

**id** (required)
: Scan target identifier. Must be unique across all targets.

**policies** (required)
: List of effective policy IDs to evaluate against this target.
  Each must match a **policies[].id** or the auto-derived ID from
  **policies[].url**.

**variables** (optional)
: Provider-specific key-value pairs passed to the provider during
  scan. Supports **${VAR}** environment variable substitution.
  Unset variables cause a configuration error.

Run **complyctl doctor --verbose** to see the required and optional
variables for each target, including one-of groups where at least one
variable in the group must be provided (e.g., **url** or
**input_path**).

## variables

Workspace-scoped constants passed to all providers. Unlike target
variables, workspace variables do **not** support **${VAR}**
substitution — they are passed as-is. Place
environment-dependent values in **targets[].variables** instead.

```
variables:
  output_dir: /tmp/scan-results
```

## verification

OCI artifact signature verification. Two mutually exclusive modes are
supported: keyless (Sigstore OIDC) and keyed (cosign key-pair).
When omitted, verification is skipped.

Keyless verification (GitHub Actions example):

```
verification:
  issuer: https://token.actions.githubusercontent.com
  identity: https://github.com/myorg/myrepo/.github/workflows/release.yml@refs/tags/*
```

Keyed verification (public key):

```
verification:
  key: /path/to/cosign.pub
```

**issuer** (keyless)
: OIDC token issuer URL (e.g.,
  **https://token.actions.githubusercontent.com**). Requires
  **identity**.

**identity** (keyless)
: Expected SAN identity in the signing certificate. Supports glob
  patterns (e.g., **\*@refs/tags/\***).

**key** (keyed)
: Path to a PEM-encoded public key. Mutually exclusive with
  **issuer**/**identity**.

**trusted_root** (keyless, optional)
: Path to a **trusted_root.json** file for keyless verification
  against private Sigstore instances. Mutually exclusive with
  **key**; requires **issuer** and **identity**.

Per-entry verification can be set on individual **policies[]** or
**complypacks[]** entries, overriding the workspace-level config:

```
policies:
  - url: ghcr.io/myorg/policies/internal-policy:v1.0
    verification:
      key: /path/to/internal.pub
  - url: ghcr.io/thirdparty/policy:v2.0
    skip_verify: true
```

## Complete example

```
policies:
  - url: quay.io/complytime/policies-ampel-branch-protection:latest
    id: ampel-bp
complypacks:
  - url: ghcr.io/myorg/complypacks/ampel-bp:v1.0.0
    id: ampel-bp-pack
variables:
  output_dir: /tmp/results
verification:
  issuer: https://token.actions.githubusercontent.com
  identity: https://github.com/complytime/complyctl/.github/workflows/release.yml@refs/tags/*
targets:
  - id: my-repo
    policies:
      - ampel-bp
    variables:
      url: https://github.com/myorg/myrepo
      access_token: ${GITHUB_TOKEN}
```

# ENVIRONMENT

**COMPLYTIME_WORKSPACE**
: Override the workspace directory used by complyctl. When set,
complyctl resolves configuration and output paths relative to this
directory. The **--workspace** flag takes precedence.

**COMPLYTIME_SHOW_PASSING**
: When set to **false**, exclude passing controls from the terminal
scan summary table. Default: **true** (show all controls). The
**--show-passing** flag takes precedence.

**COMPLYTIME_LOG_FORMAT**
: EvaluationLog serialization format: **yaml** or **json**.
Default: **yaml**. The **--log-format** flag takes precedence.

**COMPLYTIME_CACHE_VERSIONS**
: Number of complypack versions to retain per evaluator-id in the
local cache (~/.cache/complytime/complypacks/). Default: **1**.
Values less than 1 are clamped to 1.

**NO_COLOR**
: When set (any non-empty value), **complyctl doctor** automatically
selects **text** format instead of the default human-readable emoji
output. Follows the NO_COLOR convention at https://no-color.org. Has
no effect when **--format** is specified explicitly.

# EXIT CODES

**0**
: Command completed successfully. Scan findings (passed, failed, not
applicable) are reported in the output but do not affect the exit
code. Policy findings are data, not errors.

**1**
: An operational error occurred. This includes provider failures,
invalid configuration, or zero requirements assessed. Reports and
summaries are written before the non-zero exit so partial results
remain available.

To gate a pipeline on compliance results, parse the scan output
(SARIF, OSCAL) with your policy engine rather than relying on the exit
code.

# FILES

**~/.cache/complytime/policies/**
: OCI Layout cache for fetched policies (one store per policy ID).
Follows the XDG Base Directory Specification (**$XDG_CACHE_HOME**).

**~/.cache/complytime/complypacks/**
: Complypack cache with version retention per evaluator-id.

**~/.local/share/complytime/state.json**
: Digest tracking for incremental policy sync. Follows the XDG Base
Directory Specification (**$XDG_DATA_HOME**).

**~/.local/share/complytime/providers/**
: User-installed provider executables (takes precedence over system
directory).

**/usr/libexec/complytime/providers/**
: System-installed provider executables.

**.complytime/complytime.yaml**
: Workspace configuration file. See the **CONFIGURATION** section for
  the full schema reference.

**.complytime/scan/**
: Scan output reports (EvaluationLog, OSCAL, SARIF, Markdown).

**.complytime/complyctl.log**
: Debug log file (created when **--debug** is used).

# SEE ALSO

Project repository: <https://github.com/complytime/complyctl>

Providers: <https://github.com/complytime/complytime-providers>

Gemara specification: <https://gemara.openssf.org/>

# COPYRIGHT

© 2025-2026 Red Hat, Inc. Complyctl is released under the terms of the
Apache-2.0 license.
