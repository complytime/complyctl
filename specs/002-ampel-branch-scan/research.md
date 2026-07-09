# Research: AMPEL Branch Protection Scanning Plugin

**Branch**: `002-ampel-branch-scan` | **Date**: 2026-02-11

## R-001: Plugin Interface and Registration

**Decision**: Implement `pkg/plugin.Plugin` interface from the
in-tree `pkg/plugin` package.

**Rationale**: This is the Gemara-native scanning provider
interface used by complyctl. The framework handles gRPC
serialization/deserialization automatically via proto
transformations.

**Interface**:
```go
type Plugin interface {
    Generate(context.Context, *GenerateRequest) (*GenerateResponse, error)
    Scan(context.Context, *ScanRequest) (*ScanResponse, error)
    Describe(context.Context, *DescribeRequest) (*DescribeResponse, error)
}
```

Generate receives `[]AssessmentConfiguration` and global
variables. Scan receives targets with per-target variables.
Describe reports plugin health and required variables.

**Registration** uses `plugin.Serve(ampelPlugin)` with
`server.New()` creating the `PluginServer`, following the
openscap-plugin main.go pattern.

**Alternatives considered**:
- Direct gRPC service implementation: Rejected because the
  framework already handles proto conversion.
- Custom plugin framework: Rejected as it would break complyctl
  compatibility.

## R-002: AMPEL Policy Format

**Decision**: Use AMPEL Policy API v1 JSON format with tenets
containing CEL expressions.

**Rationale**: This is the canonical format consumed by the
`ampel verify` command. The structure is:

```json
{
  "id": "policy-id",
  "meta": {
    "runtime": "cel@v14.0",
    "description": "...",
    "assert_mode": "AND",
    "version": 1,
    "controls": [{"source": "...", "id": "..."}],
    "enforce": "ON"
  },
  "tenets": [
    {
      "id": "tenet-id",
      "title": "...",
      "predicates": {"types": ["predicate-uri"]},
      "code": "CEL expression"
    }
  ]
}
```

Branch protection rules use predicate type:
`http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml`

**Alternatives considered**:
- PolicySet format: Not needed for single-policy generation.
  Can be added later if multiple policy files are required.

## R-003: Assessment Requirement to AMPEL Policy Matching (Updated 2026-02-17)

**Decision**: Match assessment requirement IDs against granular
AMPEL policy file IDs and merge the matching policies into a
combined bundle. This replaces the earlier approach of generating
CEL expressions from OSCAL rules.

**Rationale**: Granular AMPEL policies are authored independently
(one JSON file per control) with hand-crafted CEL expressions,
attestation type references, and remediation guidance. The plugin
matches requirement IDs to policy file IDs, selecting only the
policies relevant to the current assessment configurations. This
approach:
- Preserves expert-authored CEL logic (no auto-generation)
- Supports independent policy authoring and testing
- Aligns with Gemara2Ampel workspace mode output
- Keeps the generate phase simple (match + merge)

**Matching flow**:
1. `LoadGranularPolicies(dir)` → loads all `*.json` from policy_dir
2. `MatchPolicies(requirementIDs, granularPolicies)` → matches by
   requirement ID ↔ policy ID
3. `MergeToBundle(matched)` → produces single policy bundle

**Alternatives considered**:
- CEL expression generation from assessment rules: Initially
  implemented but replaced. Auto-generated CEL was fragile,
  hard to maintain, and did not capture domain-specific
  verification logic that policy authors need to express.
- Direct rule-to-CLI-args mapping: Rejected because AMPEL
  requires policy files as input to `ampel verify`.

## R-004: External Tool Invocation Pattern

**Decision**: Use `os/exec.LookPath` + `exec.Command` for
invoking snappy and ampel, following the openscap-plugin
pattern.

**Rationale**: The openscap-plugin uses this exact pattern in
`oscap/oscap.go`. It provides tool presence validation via
LookPath, structured command construction, and proper error
capture via CombinedOutput.

**Tool invocation flow**:
1. `snappy` - Collects branch protection configuration from
   GitHub/GitLab repos, produces attestation data
2. `ampel verify` - Evaluates AMPEL policy against attestation
   data, produces verification results

**Alternatives considered**:
- Embedding AMPEL logic via Go library: Rejected per spec
  assumption that tools are externally installed.
- Shell script wrapper: Rejected for lack of error handling
  granularity.

## R-005: Dependencies

**Decision**: Use only libraries already present in the project.
Zero new dependencies.

**Rationale**: The user explicitly requires minimal dependencies
and reuse of existing libraries. All needed libraries are already
in `go.mod`:

| Need | Library | Already in go.mod |
| ------ | --------- | ------------------- |
| Plugin framework | pkg/plugin (in-tree) | Yes |
| Plugin hosting | hashicorp/go-plugin v1.7.0 | Yes |
| Structured logging | hashicorp/go-hclog v1.6.3 | Yes |
| Testing assertions | stretchr/testify v1.11.1 | Yes |
| YAML parsing | goccy/go-yaml v1.19.2 | Yes |
| JSON | encoding/json (stdlib) | Yes |
| External commands | os/exec (stdlib) | Yes |
| File I/O | os, path/filepath (stdlib) | Yes |

No CEL library is needed because the plugin generates CEL
expressions as strings; evaluation is performed by the external
`ampel` tool.

No Gemara library is needed in the current implementation. The
conversion layer is isolated for future migration.

**Alternatives considered**:
- Adding cel-go for expression validation: Rejected per
  simplicity principle. Ampel validates CEL at runtime.
- Adding go-github/go-gitlab for direct API calls: Rejected
  because snappy handles API interaction.

## R-006: Logging

**Decision**: Use `hashicorp/go-hclog` with JSON format to
stderr, following openscap-plugin pattern.

**Rationale**: The plugin runs as a hashicorp/go-plugin subprocess.
The framework expects hclog. The openscap-plugin uses:

```go
logger = hclog.New(&hclog.LoggerOptions{
    Name:       "ampel-plugin",
    Level:      hclog.Debug,
    Output:     os.Stderr,
    JSONFormat: true,
})
```

This satisfies Constitution Principle IV (Readability First).

## R-007: Target Configuration Format

**Decision**: Target repositories are configured as entries in
`complytime.yaml` with per-target variables (url, specs, branches,
access_token, platform). No separate target configuration file.

**Rationale**: complyctl's `pkg/plugin` interface passes target
information via `ScanRequest.Targets[].Variables` as plain string
key-value pairs. This eliminates the need for a separate YAML
file and custom parsing — the plugin receives targets directly
from complyctl.

**Example** (in `complytime.yaml`):
```yaml
targets:
  - id: myorg-frontend
    policies:
      - branch-protection
    variables:
      url: https://github.com/myorg/myrepo
      specs: builtin:github/branch-rules.yaml
      branches: main,release
  - id: myorg-infra
    policies:
      - branch-protection
    variables:
      url: https://gitlab.com/myorg/infrastructure
      specs: builtin:github/branch-rules.yaml
      branches: main
```

**Alternatives considered**:
- Separate YAML file (`ampel-targets.yaml`): Originally
  implemented but replaced. Using `complytime.yaml` target
  entries is simpler and consistent with how complyctl passes
  configuration to all plugins.
- JSON format: Rejected as less human-readable for config files.

## R-008: Convert Package Isolation

**Decision**: Isolate the requirement-to-AMPEL conversion in a
dedicated `convert` package with a clean interface boundary.

**Rationale**: The convert package encapsulates all
policy-matching logic behind:

```go
func LoadGranularPolicies(dir string) ([]AmpelPolicy, error)
func MatchPolicies(requirementIDs []string, policies []AmpelPolicy) []AmpelPolicy
func MergeToBundle(policies []AmpelPolicy) *AmpelPolicyBundle
```

Only `MatchPolicies` depends on the upstream policy model (the
requirement ID format). The load and merge functions, and all
downstream packages (scan, results, targets, config), are
independent of how requirements are sourced.

**Alternatives considered**:
- Interface-based abstraction with multiple implementations:
  Rejected per simplicity principle. A single concrete
  implementation is sufficient.

## R-009: Per-Repository Result Files

**Decision**: Write one JSON result file per repository to
`{workspace}/ampel/results/{repo-name}.json`.

**Rationale**: The spec requires separate result files per
repository. Using the repository name (sanitized) as the filename
makes results easily discoverable. The consolidated ScanResponse
returned to complyctl aggregates all per-repo assessment logs.

**Alternatives considered**:
- Single consolidated result file: Rejected by spec requirement
  FR-005.
- Timestamped filenames: Rejected per clarification that re-runs
  overwrite.

## R-010: DSSE Envelope Handling (Added 2026-02-17)

**Decision**: Unwrap DSSE-signed envelopes in ParseAmpelOutput
before parsing the in-toto result predicate.

**Rationale**: `ampel verify --attest-results` produces
DSSE-wrapped attestations (the standard signed attestation
format). The DSSE envelope has `payloadType`, `payload` (base64),
and `signatures` fields. When unmarshaled directly into the
result statement type, the predicate is empty because DSSE
wraps the actual statement in its `payload` field. The fix
tries base64 RawURL decoding first, then StdEncoding as fallback.

**Structure**:
```json
{
  "payloadType": "application/vnd.in-toto+json",
  "payload": "<base64url-encoded in-toto statement>",
  "signatures": [{"keyid": "...", "sig": "..."}]
}
```

**Impact**: Without DSSE unwrapping, all controls silently
appear as "fail" because the predicate field is empty.

## R-011: Multi-Target Observation Grouping (Added 2026-02-17)

**Decision**: Group findings with the same CheckID into a single
assessment log entry with multiple subjects (one per repository).

**Rationale**: When multiple results share the same CheckID,
grouping them into a single assessment log entry with multiple
subjects prevents duplicate entries and produces correct
multi-target assessment results. The `ToScanResponse` function
uses insertion-order tracking to produce deterministic output.

## R-012: Test Strategy with Mock Fixtures

**Decision**: Provide mock assessment configuration and AMPEL
policy fixtures in `convert/testdata/` for unit testing the
conversion layer.

**Rationale**: The user explicitly requires mocked data to verify:
1. What happens to AMPEL policy when assessment configurations change
2. Assessment configuration ↔ AMPEL policy linkage
3. Final AMPEL policy accuracy after generate

Test fixtures include:
- `assessment-plan-full.json` - Complete assessment configuration
  with multiple branch protection requirement IDs
- `assessment-plan-subset.json` - Plan with fewer rules (tests
  scope filtering)
- `ampel-policy-expected.json` - Expected AMPEL output for full
  plan
- `ampel-policy-existing-broader.json` - Pre-existing broader
  policy (tests FR-003 scope honoring)

Table-driven tests compare generated output against expected
fixtures, making linkage verification straightforward.
