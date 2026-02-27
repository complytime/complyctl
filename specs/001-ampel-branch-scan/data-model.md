# Data Model: AMPEL Branch Protection Scanning Plugin

**Branch**: `001-ampel-branch-scan` | **Date**: 2026-02-11

## Entity Definitions

### 1. Granular AmpelPolicy

Represents a single AMPEL verification policy for one control.
Authored independently as a standalone JSON file in the policy
directory. Multiple granular policies are matched against OSCAL
rules and merged into a combined bundle during generate.

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Policy identifier matching OSCAL rule ID (e.g., "SC-CODE-01.01") |
| Meta | AmpelMeta | Policy metadata including description and control references |
| Tenets | []AmpelTenet | CEL-based verification checks |

**AmpelMeta fields**:

| Field | Type | Description |
|-------|------|-------------|
| Description | string | Human-readable policy description |
| Controls | []Control | OSCAL control references (framework, class, id) |

**AmpelTenet fields**:

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Unique tenet identifier within the policy |
| Code | string | CEL expression for verification |
| Predicates | PredicateSpec | Attestation types to evaluate |
| Assessment | Assessment | Message for passing tenets |
| Error | TenetError | Message and guidance for failing tenets |

**Relationships**:
- Matched to OSCAL rules by policy ID ↔ rule ID
- Merged into AmpelPolicyBundle during generate
- Each granular file is independently authored and testable

**Validation**:
- ID MUST be non-empty
- At least one tenet MUST exist
- Each tenet MUST have non-empty Code (CEL expression)

### 1b. AmpelPolicyBundle

The combined policy produced by merging matched granular policies.
Written to `{workspace}/ampel/policy/complytime-ampel-policy.json`.

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Always "complytime-ampel-policy" |
| Meta | BundleMeta | Bundle metadata with framework reference |
| Policies | []AmpelPolicy | Array of matched granular policies |

**BundleMeta fields**:

| Field | Type | Description |
|-------|------|-------------|
| Frameworks | []Framework | Single entry: ComplyTime-AMPEL-Policy |

### 2. TargetRepository

Represents a GitHub or GitLab repository to scan, parsed from
the workspace configuration file.

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| URL | string | Repository URL (https://github.com/org/repo) |
| Branches | []string | Branch names to evaluate protection rules on |
| Specs | []string | Snappy spec file references (e.g., "builtin:github/branch-rules.yaml") |

**Validation**:
- URL MUST be a valid HTTPS URL
- URL MUST point to a GitHub or GitLab host
- Branches MUST contain at least one entry
- Specs MUST contain at least one entry
- Specs support `builtin:` prefix for embedded files and absolute
  paths for custom specs
- Duplicate URL+branch combinations trigger a warning and are
  deduplicated
- Duplicate specs within a repository are deduplicated

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
- One PerRepoResult per TargetRepository+branch+spec combination
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
Granular AMPEL Policies (policy_dir/*.json)
    │
    ▼ [convert package: LoadGranularPolicies]
    │
OSCAL Policy ([]RuleSet)
    │
    ▼ [convert package: MatchPolicies + MergeToBundle]
AmpelPolicyBundle
    │
    ├── written to → PolicyDir/complytime-ampel-policy.json
    │
    ▼ [scan package]
TargetConfig
    │
    ├── parsed from → TargetsFile
    │
    ▼ [for each TargetRepository + branch + spec]
PerRepoResult
    │
    ├── written to → ResultsDir/{repo-name}-{branch}.json
    │
    ▼ [results package: ToPVPResult]
    │   Groups findings by CheckID → one ObservationByCheck
    │   per check with multiple Subjects (one per repo)
    │
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
