# Data Model: AMPEL Branch Protection Scanning Plugin

**Branch**: `002-ampel-branch-scan` | **Date**: 2026-02-11

## Entity Definitions

### 1. Granular AmpelPolicy

Represents a single AMPEL verification policy for one control.
Authored independently as a standalone JSON file in the policy
directory. Multiple granular policies are matched against
assessment requirement IDs and merged into a combined bundle
during generate.

**Fields**:

| Field | Type | Description |
| ------- | ------ | ------------- |
| ID | string | Policy identifier matching assessment requirement ID (e.g., "BP-1.01") |
| Meta | AmpelMeta | Policy metadata including description and control references |
| Tenets | []AmpelTenet | CEL-based verification checks |

**AmpelMeta fields**:

| Field       | Type       | Description                               |
|-------------|------------|-------------------------------------------|
| Description | string     | Human-readable policy description         |
| Controls    | []Control  | Control references (framework, class, id) |

**AmpelTenet fields**:

| Field | Type | Description |
| ------- | ------ | ------------- |
| ID | string | Unique tenet identifier within the policy |
| Code | string | CEL expression for verification |
| Predicates | PredicateSpec | Attestation types to evaluate |
| Assessment | Assessment | Message for passing tenets |
| Error | TenetError | Message and guidance for failing tenets |

**Relationships**:
- Matched to assessment requirements by policy ID ↔ requirement ID
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
| ------- | ------ | ------------- |
| ID | string | Always "complytime-ampel-policy" |
| Meta | BundleMeta | Bundle metadata with framework reference |
| Policies | []AmpelPolicy | Array of matched granular policies |

**BundleMeta fields**:

| Field      | Type        | Description                           |
|------------|-------------|---------------------------------------|
| Frameworks | []Framework | Single entry: ComplyTime-AMPEL-Policy |

### 2. TargetRepository

Represents a GitHub or GitLab repository to scan, parsed from
the workspace configuration file.

**Fields**:

| Field | Type | Description |
| ------- | ------ | ------------- |
| URL | string | Repository URL (<https://github.com/org/repo>) |
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

**Source**: Received via `ScanRequest.Targets[].Variables` from
`complytime.yaml` target entries.

### 3. PerRepoResult

Represents scan findings for a single repository, written as
a JSON file in the workspace.

**Fields**:

| Field | Type | Description |
| ------- | ------ | ------------- |
| Repository | string | Repository URL |
| Branch | string | Branch name evaluated |
| ScannedAt | time.Time | Timestamp of scan |
| Findings | []Finding | Individual rule results |
| Status | string | "pass", "fail", or "error" |
| Error | string | Error message if status is "error" |

**Finding fields**:

| Field | Type | Description |
| ------- | ------ | ------------- |
| TenetID | string | AMPEL tenet that was evaluated |
| Title | string | Human-readable rule name |
| Result | string | "pass" or "fail" |
| Reason | string | Explanation of the result |

**Relationships**:
- One PerRepoResult per TargetRepository+branch+spec combination
- Findings map back to AmpelPolicy.Tenets via TenetID
- Aggregated into `plugin.ScanResponse` for complyctl

### 4. Plugin Configuration

Plugin configuration is stateless. Paths are derived from
package constants in `config/config.go`. The only user-configurable
override is the `ampel_policy_dir` global variable in
`complytime.yaml`, received via `GenerateRequest.GlobalVariables`.

**Package constants**:

| Constant | Value | Description |
| ---------- | ------- | ------------- |
| PluginDir | "ampel" | Plugin subdirectory under workspace |
| DefaultGranularPolicyDir | "granular-policies" | Default granular policy source directory |
| GeneratedPolicyDir | "policy" | Generated policy bundle output directory |
| DefaultResultsDir | "results" | Scan results output directory |

**Path helpers**: `GranularPolicyDirPath()`, `GeneratedPolicyDirPath()`,
`ResultsDirPath()`, `SpecDirPath()` derive absolute paths from
`complytime.WorkspaceDir`.

## Entity Relationships

```text
Granular AMPEL Policies (policy_dir/*.json)
    │
    ▼ [convert package: LoadGranularPolicies]
    │
Assessment Configurations ([]plugin.AssessmentConfiguration)
    │
    ▼ [convert package: MatchPolicies + MergeToBundle]
AmpelPolicyBundle
    │
    ├── written to → PolicyDir/complytime-ampel-policy.json
    │
    ▼ [scan package]
Target Variables (from ScanRequest.Targets[].Variables)
    │
    ├── received from → complytime.yaml target entries
    │
    ▼ [for each TargetRepository + branch + spec]
PerRepoResult
    │
    ├── written to → ResultsDir/{repo-name}-{branch}.json
    │
    ▼ [results package: ToScanResponse]
    │   Groups findings by CheckID → one assessment log entry
    │   per check with multiple subjects (one per repo)
    │
plugin.ScanResponse (returned to complyctl)
```

## State Transitions

### Generate Flow
```text
No policy → Generate() → Policy file exists in PolicyDir
Policy exists → Generate() → Policy overwritten with new scope
```

### Scan Flow
```text
No results → Scan() → Per-repo result files created + ScanResponse returned
Results exist → Scan() → Results overwritten + ScanResponse returned
```

### Error States
- Tool missing → Generate/Scan returns error with tool name
- Target unreachable → PerRepoResult with status "error",
  scanning continues for remaining targets
- Rate limited → PerRepoResult with status "error" for affected
  repo, scanning continues
- Empty policy (no applicable rules) → Generate returns success
  with no output
