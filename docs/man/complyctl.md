% COMPLYCTL(1) Complyctl Manual
% Marcus Burghardt <maburgha@redhat.com>
% November 2026

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

Fetch policies and complypacks from OCI registries or a local OCI image
layout into the local cache. Registry sync performs incremental downloads and
uses Docker credential helpers for authentication.

```
complyctl get
complyctl get --skip-verify
complyctl get --from-oci-layout /path/to/oci-layout
```

**-t**, **--timeout** *duration*
: Maximum time for the get operation (default: **5m**).

**--skip-verify**
: Skip signature verification for fetched artifacts.

**--from-oci-layout** *path*
: Import digest-pinned configured policies and complypacks from a local OCI image layout.

### Offline provisioning

On a disconnected system, configure policies and complypacks with digest-pinned
URLs, then import a local OCI image layout:

```bash
complyctl get --from-oci-layout /path/to/oci-layout
```

The configured digest selects each artifact from the layout without contacting
the registry. If verification is configured, add `--skip-verify`; offline
signature verification is not yet supported and imported entries remain
unverified.

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
complyctl scan --policy-id my-policy --format oscal,sarif
complyctl scan --policy-id my-policy --format all
complyctl scan --policy-id my-policy --log-format json
```

**-p**, **--policy-id** *string*
: Policy ID to scan.

**-f**, **--format** *strings*
: Additional output format(s): **oscal**, **pretty** (Markdown), **sarif**,
  **all** (all three). Accepts comma-separated values
  (`--format oscal,sarif`) or repeated flags (`--format oscal --format sarif`).
  The default EvaluationLog is always produced regardless of this flag.

**-t**, **--timeout** *duration*
: Maximum time for the scan operation (default: **5m**).

**--show-passing**
: Include passing controls in the terminal scan summary table
  (default: **true**). Can also be set via **COMPLYTIME_SHOW_PASSING**.

**--log-format** *string*
: EvaluationLog serialization format: **yaml**, **json**
  (default: **yaml**). Can also be set via **COMPLYTIME_LOG_FORMAT**.

## clean

Expire managed report/evaluation groups beneath **.complytime/** in the
selected workspace. Successful expiry permanently removes both the
reports and their supporting evaluation directories, including all
contained outputs. Save anything needed for audit elsewhere and preview
before deleting:

```bash
complyctl clean --older-than 30d --dry-run
complyctl clean --older-than 30d
```

**--older-than** *days*d
: Required positive whole days, each exactly 24 hours; no default.
  Accepts **1d** through **106751d**, without leading zeroes. Rejects
  zero, negative, fractional, whitespace-padded, unsupported-unit, and
  overflowing values.

**--dry-run**
: Preview complete groups without changing workspace files (default:
  **false**). Does not create configuration, lock files, activity records,
  reports, evidence, or debug logs. Preview releases access when done;
  later cleanup independently repeats discovery and preflight.

No positional arguments are accepted. Workspace precedence is
**--workspace** (or **-w**), then **COMPLYTIME_WORKSPACE**, then the
current directory. Select the existing project root containing
**.complytime/**. Cleanup does not initialize a missing workspace or
require configuration, registry credentials, policy resolution, or
provider installation. Results use stdout; diagnostics use stderr even
with **--debug**, without creating or modifying the workspace log file.

### Scope and expiry

Cleanup recognizes retained-layout evaluation directories beneath only
**.complytime/ampel/results/**, **.complytime/opa/results/**, and
**.complytime/openscap/results/**. Valid intermediate labels contain
1–80 ASCII letters, digits, underscores, or hyphens. Evaluation names use
**evaluation-**, a UTC collection-start timestamp in **20060102T150405Z**
format, a hyphen, and exactly 32 lowercase hexadecimal characters.
Flat legacy files and unrecognized layouts remain untouched.

Managed reports live directly under **.complytime/scan/**: YAML/JSON
EvaluationLogs, Markdown with an embedded EvaluationLog, OSCAL, and
SARIF variants. Validated content establishes relationships; filenames
alone never prove ownership. Reports sharing any evaluation join one
group transitively, even when they cite different files within it.
An unreferenced or incomplete evaluation can qualify only after a
complete managed-report inventory establishes no report references it.

Every evaluation in a group must start strictly before one captured UTC
cutoff. Age uses collection start, not modification times, report
filenames, or optional metadata. A newer, exact-cutoff, future, or
invalid start retains the whole group. Reports follow their connected
evaluations' ages. Reports without an establishable evaluation age stay.

Cleanup preserves CLI logs, configuration, scan inputs, working clones,
generation state, caches, unrelated files, and other workspaces. It does
not prune labels or managed roots, fetch remote evidence, or expand its
scope through evidence references. External or renamed report copies
are outside managed scope; cleanup cannot guarantee their references
remain valid. Renaming is not an override: unknown report-like files
inside the report root can block cleanup.

Current SARIF output lacks originating-log and evidence provenance.
Cleanup therefore refuses workspace planning when a managed SARIF
report is present, including an empty all-passing report. It never
guesses associations from a matching filename or silently skips SARIF.
Unbounded report ambiguity
likewise refuses the workspace before deletion.

### Safety, support, and failures

Whole-group preflight retains every member unchanged if any member has
a detectable safety or access failure. Confined traversal refuses
symlinks, special files, replaced candidates, and mount crossings.
Eligible groups lose all evidence before report removal begins.
Unexpected removal failures stop the affected group, retain what
remains, print exact progress, and return nonzero; independent safe
groups can continue. An evidence failure leaves all group reports,
possibly referencing already removed evidence. A report failure can
leave some reports after all evidence is gone. There is no rollback.

Cleanup and preview require exclusive workspace access. Updated hosts
coordinate scan/generation, relevant **get** artifact removal, and
**init**; conflicting access fails promptly. Scan protection extends
through report publication and normal provider teardown. Normal RPC
completion and teardown allow removal of the invocation's own activity
record, including handled application errors. Host interruption, forced
or abnormal termination, transport failure, or uncertain cancellation
leaves unresolved activity that blocks cleanup even after lock release.
Normal completion is not proof that arbitrary descendant tools exited.

Cleanup targets Linux and macOS on amd64 and arm64. Allowed local
filesystems are ext4, XFS, Btrfs, and tmpfs on Linux, and APFS on macOS.
Other filesystems, including network, FUSE, overlay, and HFS+, are
refused. OS support and filesystem support are separate; unavailable
locking/confinement or unsafe mount layouts also cause refusal. These
targets do not imply completed native runtime verification on every
platform. Windows is outside the cleanup support boundary.

Stop older hosts, direct provider callers, and independent writers before
cleanup: host locks do not stop them. Keep directory and mount layouts
stable. Unchanged provider binaries work under updated hosts, but
provider availability does not limit cleanup of existing evidence.

Unresolved activity errors identify exact record paths under
**.complytime/coordination/operations/**. Pause new scans and stop all
writers, including provider-launched tools. If shutdown is uncertain,
restart the complete writer machine/VM or stop its complete isolated
container environment, prevent job restarts, and stop writers in other
environments too. Restarting a terminal or complyctl alone is
insufficient. Only then manually remove the exact identified records,
not the coordination directory or unrelated files. Records and PIDs are
diagnostic hints, not authority to kill processes. There is no force or
automatic stale-record override. Rerun preview; record removal never
bypasses a live host lock. See the evidence guide's
[Interrupted-Scan Recovery](https://github.com/complytime/complyctl/blob/main/docs/EVIDENCE.md#interrupted-scan-recovery)
for the full procedure.

Output includes workspace, UTC cutoff, consequences, stable group
membership, blockers, reasons, exclusions, and totals. Preview uses
**would-delete** or **retained**; execution uses **deleted**,
**partially-deleted**, or **retained** and prints successful removals.
No matches, normal age exclusions, and repeated successful cleanup exit
zero. Invalid arguments, missing workspaces, coordination/inventory
refusal, safety/access failures, removal errors, and output errors exit
nonzero, even when independent groups succeeded.

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
: Output debug logs to stderr and the workspace log file. Exception:
  **clean**, including preview, uses stderr only and leaves logs unchanged.

**-w**, **--workspace** *directory*
: Workspace directory. Takes precedence over **COMPLYTIME_WORKSPACE**;
  otherwise defaults to the current directory.

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

**COMPLYTIME_SYSTEM_PROVIDER_DIR**
: Override the system-wide provider directory that is scanned after
the user provider directory. Providers in the user directory take
precedence. Must be an absolute path; a relative path is ignored.
Default: **/usr/libexec/complytime/providers**.

**COMPLYTIME_MACHINE_ID_FILE**
: Override the path to the machine-id file used by providers when
constructing evidence host remarks. Default: **/etc/machine-id**. The
file must contain a non-empty hexadecimal string no larger than 1 KB.
If unreadable or invalid, the machine-id is silently omitted from the
remarks.

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

For **clean**, normal age exclusions, no matches, and repeated successful
cleanup return **0**. Validation, coordination, discovery, preflight,
removal, or output errors return **1**. Returned cleanup results are
printed even on failure; workspace-wide refusal can provide no plan.

To gate a pipeline on compliance results, parse the scan output
(SARIF, OSCAL) with your policy engine rather than relying on the exit
code.

# OUTPUT

**Scan reports**
: The **scan** command writes reports to **.complytime/scan/** in each
enabled format. Report filenames follow the pattern:

    *prefix*-*policy-id*-*target-id*-*YYYYMMDD*-*HHMMSS*.*ext*

When no target ID is present (single-target scans), the target
segment is omitted. Path separators in policy and target IDs are
replaced with dashes.

**Evidence source provenance**
: Each evidence entry in the EvaluationLog may include a **source**
block identifying where the data was collected from. The source
block contains five fields:

*reference-id*
: Names the external system or Gemara artifact the evidence was
collected from. Ties the evidence to a **mapping-references** entry
in the EvaluationLog metadata.

*coordinate*
: Precise location within the referenced system (API path, file
path, JSON path). Mutually exclusive with **entry-id**.

*entry-id*
: Specific entry within a referenced Gemara artifact. Mutually
exclusive with **coordinate**.

*digest*
: Cryptographic hash of the observed content at collection time.
Format: *algorithm*:*encoded* (e.g., sha256:abc123...).

*remarks*
: Provider prose about collection context, host identification, or
caveats. For providers that support it, the standard format is
**Collected on host "hostname" (machine-id: hexstring)**, where the
machine-id is read from **/etc/machine-id** (overridable via
**COMPLYTIME_MACHINE_ID_FILE**).

In the Markdown report (**--format pretty**), evidence source is
rendered as **source:** *reference-id* **@** *coordinate*. The full
set of five fields is available in EvaluationLog YAML/JSON output.
Evidence source requires provider support; when absent, the source
block is omitted.

For a detailed walkthrough and field reference, see the Evidence
Source Provenance documentation at
<https://github.com/complytime/complyctl/blob/main/docs/EVIDENCE.md>.

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
Filenames follow the pattern
*prefix*-*policy-id*-*target-id*-*YYYYMMDD*-*HHMMSS*.*ext*
(e.g., **evaluation-log-pol-v1-web-20260729-143012.yaml**).

**.complytime/complyctl.log**
: Debug log file (created when **--debug** is used, except by **clean**).

**.complytime/{ampel,opa,openscap}/results/**
: Provider results roots. Recognized evaluation directories and all their
contents are indivisible evidence members for **clean**.

**.complytime/coordination/operations/**
: Host activity records persisted before provider startup. Unresolved
records block cleanup and require the stopped-writer recovery procedure
above; they are not age-based cleanup candidates.

# SEE ALSO

Project repository: <https://github.com/complytime/complyctl>

Providers: <https://github.com/complytime/complytime-providers>

Gemara specification: <https://gemara.openssf.org/>

# COPYRIGHT

© 2025-2026 Red Hat, Inc. Complyctl is released under the terms of the
Apache-2.0 license.
