# Data Model: AMPEL Branch Protection Scanning Plugin

**Branch**: `001-ampel-branch-scan` | **Date**: 2026-02-11

## Entity Definitions

### 1. AmpelPolicy

Represents a complete AMPEL verification policy generated from
OSCAL rules. Serialized as JSON.

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Policy identifier, derived from assessment plan context |
| Meta | AmpelMeta | Policy metadata |
| Tenets | []AmpelTenet | Verification checks |

**AmpelMeta fields**:

| Field | Type | Description |
|-------|------|-------------|
| Runtime | string | CEL runtime version (always "cel@v14.0") |
| Description | string | Human-readable policy description |
| AssertMode | string | "AND" (all tenets must pass) |
| Version | int64 | Integer version, incremented on re-generate |
| Controls | []Control | OSCAL control references |
| Enforce | string | "ON" for active enforcement |

**AmpelTenet fields**:

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Unique tenet identifier |
| Title | string | Human-readable tenet name |
| Predicates | PredicateSpec | Attestation types to evaluate |
| Code | string | CEL expression for verification |
| Outputs | map[string]Output | Named output extractors |

**Relationships**:
- Generated from: OSCAL `policy.Policy` ([]RuleSet)
- Each OSCAL Rule.Check maps to one AmpelTenet
- OSCAL Rule.Parameters feed into CEL expression construction

**Validation**:
- ID MUST be non-empty
- At least one tenet MUST exist
- Each tenet MUST have non-empty Code (CEL expression)
- Each tenet MUST reference exactly one predicate type

### 2. TargetRepository

Represents a GitHub or GitLab repository to scan, parsed from
the workspace configuration file.

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| URL | string | Repository URL (https://github.com/org/repo) |
| Branches | []string | Branch names to evaluate protection rules on |

**Validation**:
- URL MUST be a valid HTTPS URL
- URL MUST point to a GitHub or GitLab host
- Branches MUST contain at least one entry
- Duplicate URL+branch combinations trigger a warning and are
  deduplicated

**Source**: Parsed from `{workspace}/ampel/ampel-targets.yaml`

### 3. TargetConfig

Top-level structure of the target repository configuration file.

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| Repositories | []TargetRepository | List of repositories to scan |

**Validation**:
- Repositories MUST contain at least one entry
- Entries are deduplicated by URL+branch before scanning

### 4. PerRepoResult

Represents scan findings for a single repository, written as
a JSON file in the workspace.

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| Repository | string | Repository URL |
| Branch | string | Branch name evaluated |
| ScannedAt | time.Time | Timestamp of scan |
| Findings | []Finding | Individual rule results |
| Status | string | "pass", "fail", or "error" |
| Error | string | Error message if status is "error" |

**Finding fields**:

| Field | Type | Description |
|-------|------|-------------|
| TenetID | string | AMPEL tenet that was evaluated |
| Title | string | Human-readable rule name |
| Result | string | "pass" or "fail" |
| Reason | string | Explanation of the result |

**Relationships**:
- One PerRepoResult per TargetRepository+branch combination
- Findings map back to AmpelPolicy.Tenets via TenetID
- Aggregated into policy.PVPResult for complyctl

### 5. PluginConfig

Plugin configuration received from complyctl via Configure().

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| Workspace | string | Root workspace directory |
| Profile | string | Compliance profile identifier |
| PolicyDir | string | AMPEL policy directory (default: {workspace}/ampel/policy/) |
| ResultsDir | string | Results output directory (default: {workspace}/ampel/results/) |
| TargetsFile | string | Path to ampel-targets.yaml (default: {workspace}/ampel/ampel-targets.yaml) |

**Source**: Manifest configuration + user overrides via
`c2p-ampel-manifest.json`

## Entity Relationships

```text
OSCAL Policy ([]RuleSet)
    │
    ▼ [convert package]
AmpelPolicy
    │
    ├── written to → PolicyDir/{policy-file}.json
    │
    ▼ [scan package]
TargetConfig
    │
    ├── parsed from → TargetsFile
    │
    ▼ [for each TargetRepository + branch]
PerRepoResult
    │
    ├── written to → ResultsDir/{repo-name}-{branch}.json
    │
    ▼ [results package]
policy.PVPResult (returned to complyctl)
```

## State Transitions

### Generate Flow
```
No policy → Generate() → Policy file exists in PolicyDir
Policy exists → Generate() → Policy overwritten with new scope
```

### Scan Flow
```
No results → GetResults() → Per-repo result files created + PVPResult returned
Results exist → GetResults() → Results overwritten + PVPResult returned
```

### Error States
- Tool missing → Configure/Generate/GetResults returns error
  with tool name
- Target unreachable → PerRepoResult with status "error",
  scanning continues for remaining targets
- Rate limited → PerRepoResult with status "error" for affected
  repo, scanning continues
- Empty policy (no applicable rules) → Generate returns success
  with no output
