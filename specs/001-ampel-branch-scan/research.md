# Research: AMPEL Branch Protection Scanning Plugin

**Branch**: `001-ampel-branch-scan` | **Date**: 2026-02-11

## R-001: Plugin Interface and Registration

**Decision**: Implement `policy.Provider` interface from
`compliance-to-policy-go/v2/policy` package.

**Rationale**: This is the exact same pattern used by the
openscap-plugin. The framework handles all gRPC
serialization/deserialization automatically via proto
transformations in `plugin/transform.go`.

**Interface**:
```go
type Provider interface {
    Configure(context.Context, map[string]string) error
    Generate(context.Context, Policy) error
    GetResults(context.Context, Policy) (PVPResult, error)
}
```

Where `Policy` is `[]extensions.RuleSet` from `oscal-sdk-go`.

**Registration** uses `plugin.Register(ServeConfig{...})` with
`plugin.PVPPlugin{Impl: server}` wrapping the Provider, identical
to the openscap-plugin main.go pattern.

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

## R-003: OSCAL to AMPEL Policy Matching (Updated 2026-02-17)

**Decision**: Match OSCAL rule IDs against granular AMPEL policy
file IDs and merge the matching policies into a combined bundle.
This replaces the earlier approach of generating CEL expressions
from OSCAL rules.

**Rationale**: Granular AMPEL policies are authored independently
(one JSON file per control) with hand-crafted CEL expressions,
attestation type references, and remediation guidance. The plugin
matches OSCAL rule IDs to policy file IDs, selecting only the
policies relevant to the current assessment plan. This approach:
- Preserves expert-authored CEL logic (no auto-generation)
- Supports independent policy authoring and testing
- Aligns with Gemara2Ampel workspace mode output
- Keeps the generate phase simple (match + merge)

**Matching flow**:
1. `LoadGranularPolicies(dir)` → loads all `*.json` from policy_dir
2. `MatchPolicies(oscalRules, granularPolicies)` → matches by
   rule ID ↔ policy ID
3. `MergeToBundle(matched)` → produces single policy bundle

**Alternatives considered**:
- CEL expression generation from OSCAL rules: Initially
  implemented but replaced. Auto-generated CEL was fragile,
  hard to maintain, and did not capture domain-specific
  verification logic that policy authors need to express.
- Direct OSCAL-to-CLI-args mapping: Rejected because AMPEL
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
|------|---------|-------------------|
| Plugin framework | compliance-to-policy-go/v2 | Yes (v2.0.0-rc.1) |
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

This satisfies Constitution Principle IV (Observability).

## R-007: Target Configuration File Format

**Decision**: YAML file in workspace (`ampel-targets.yaml`)
listing repositories with branch names.

**Rationale**: YAML is already parsed via `goccy/go-yaml` in the
project. The format is:

```yaml
repositories:
  - url: https://github.com/org/repo1
    branches:
      - main
      - release
  - url: https://gitlab.com/org/repo2
    branches:
      - main
```

**Alternatives considered**:
- JSON format: Rejected as less human-readable for config files.
- TOML: Rejected because no TOML library exists in go.mod.

## R-008: API Isolation for Future Gemara Migration

**Decision**: Isolate the OSCAL-to-AMPEL conversion in a
dedicated `convert` package with a clean interface boundary.

**Rationale**: The communication API will change when complyctl
moves from OSCAL to Gemara. The convert package encapsulates
all policy-source-specific logic behind:

```go
func LoadGranularPolicies(dir string) ([]AmpelPolicy, error)
func MatchPolicies(rules []extensions.RuleSet, policies []AmpelPolicy) []AmpelPolicy
func MergeToBundle(policies []AmpelPolicy) *AmpelPolicyBundle
```

When Gemara replaces OSCAL, only `MatchPolicies` changes to
accept Gemara policy identifiers. The load and merge functions,
and all downstream packages (scan, results, targets, config),
remain untouched.

**Alternatives considered**:
- Interface-based abstraction with multiple implementations:
  Rejected per simplicity principle. A single concrete
  implementation is sufficient; swap it when Gemara arrives.

## R-009: Per-Repository Result Files

**Decision**: Write one JSON result file per repository to
`{workspace}/ampel/results/{repo-name}.json`.

**Rationale**: The spec requires separate result files per
repository. Using the repository name (sanitized) as the filename
makes results easily discoverable. The consolidated PVPResult
returned to complyctl aggregates all per-repo observations.

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
ObservationByCheck with multiple Subjects (one per repository).

**Rationale**: The oscal-sdk-go library's observation manager
(`observations.go:112`) stores observations in a map keyed by
`observation.Title`. When multiple observations share the same
Title (CheckID), only the last one survives (last-write-wins).
The fix groups all repo subjects under one ObservationByCheck
per CheckID, which matches the OSCAL pattern and produces
correct multi-target assessment results.

**OSCAL conformance**: The `observation.subjects` array in OSCAL
Assessment Results is designed for multiple inventory items per
observation. The C2P library's `toOscalObservation` function
already handles multiple subjects correctly.

## R-012: Test Strategy with Mock Fixtures

**Decision**: Provide mock OSCAL assessment plan and AMPEL policy
fixtures in `convert/testdata/` for unit testing the conversion
layer.

**Rationale**: The user explicitly requires mocked data to verify:
1. What happens to AMPEL policy when assessment plan changes
2. Assessment plan ↔ AMPEL policy linkage
3. Final AMPEL policy accuracy after generate

Test fixtures include:
- `assessment-plan-full.json` - Complete OSCAL plan with multiple
  branch protection rules
- `assessment-plan-subset.json` - Plan with fewer rules (tests
  scope filtering)
- `ampel-policy-expected.json` - Expected AMPEL output for full
  plan
- `ampel-policy-existing-broader.json` - Pre-existing broader
  policy (tests FR-003 scope honoring)

Table-driven tests compare generated output against expected
fixtures, making linkage verification straightforward.
