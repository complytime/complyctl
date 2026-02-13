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

## R-003: OSCAL to AMPEL Rule Mapping

**Decision**: Map OSCAL `RuleSet.Checks` to AMPEL `tenets` with
CEL expressions derived from check identifiers.

**Rationale**: Each OSCAL rule contains checks with IDs. These
check IDs map to specific branch protection verification logic.
The conversion layer translates each check into an AMPEL tenet
with the appropriate CEL expression for branch protection
evaluation.

**Mapping**:
- `Rule.Name` → Tenet context (used in tenet ID generation)
- `Rule.Checks[].ID` → Tenet ID
- `Rule.Checks[].Description` → Tenet title
- `Rule.Parameters[].SelectedValue` → CEL expression parameters
- Implicit: Branch protection predicate type is set per tenet

**Alternatives considered**:
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

**Rationale**: The user explicitly stated the communication API
will change when complyctl moves from OSCAL to Gemara. By
isolating conversion behind:

```go
func PolicyToAmpel(oscalPolicy policy.Policy, config ConvertConfig) (*AmpelPolicy, error)
```

...the future migration requires changing only this package. The
server, config, scan, and results packages remain untouched.

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

## R-010: Test Strategy with Mock Fixtures

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
