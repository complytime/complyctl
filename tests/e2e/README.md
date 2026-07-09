# E2E Testing

## Automated

```bash
make test-e2e
```

Builds `complyctl` + `complyctl-provider-test`, then runs all e2e tests with an in-process mock OCI registry. No external services required.

Build tag: `e2e`.

| Test | Validates |
| :--- | :--- |
| `FullWorkflow` | get → list → generate → scan (oscal, pretty, sarif) |
| `PolicyCache` | OCI layout structure, state.json tracking |
| `MultiplePolicies` | Multi-policy fetch + list |
| `ScanDefaultFormat` | No --format = EvaluationLog only |
| `ScanTargetArg` | Positional target argument with policy inference |
| `InvalidFormat` | `--format pdf` rejected |
| `MissingPolicy` | Uncached policy fails with clear message |
| `MockRegistryOCICompliance` | v2 endpoint, catalog, tags, manifests, 404s |
| `MockPluginDescribe` | Provider discovery + Describe + Generate RPC |
| `NestedPolicyID` | Slashed policy IDs (`policies/nist-800-53-r5`) handled correctly |
| `Help` | CLI help output structure |
| `Version` | Version command output |
| `ListFilterByPolicyID` | `--policy-id` filter on list |
| `GetSkipVerify` | `--skip-verify` flag accepted, no verification warnings |
| `ListVerifiedColumn` | VERIFIED column present in list output |
| `DuplicateComplypackEvaluatorID` | Two complypacks with same evaluator-id → non-zero exit with actionable error |

## Mock Registry

The in-process mock registry (`helpers_test.go`) implements OCI Distribution Spec v2 endpoints with these seeded policies:

**Policies:**

| Repository | Layers | Tags |
| :--- | :--- | :--- |
| `nist-800-53-r5` | catalog + policy | v1.0.0, latest |
| `policies/nist-800-53-r5` | catalog + policy | v1.0.0, latest |
| `cis-benchmark` | catalog | v2.0.0, latest |

The policy layer uses evaluator ID `test`, which routes to the `complyctl-provider-test` binary.

**Complypacks:**

| Repository | Evaluator ID | Tags |
| :--- | :--- | :--- |
| `complypacks/opa-a` | `opa` | v1.0.0, latest |
| `complypacks/opa-b` | `opa` | v1.0.0, latest |

Both complypack repositories use evaluator ID `opa` (intentional duplicate for `DuplicateComplypackEvaluatorID` testing). Complypack artifacts include a config blob with `evaluator-id` and `version`, a content blob (tar.gz), and the `application/vnd.complypack.artifact.v1` artifact type.

## Manual Walkthrough

### Prerequisites

```bash
make build build-test-provider
```

### Step 1: Start mock OCI registry

```bash
make mock-registry
```

### Step 2: Install test provider

```bash
mkdir -p ~/.local/share/complytime/providers
cp bin/complyctl-provider-test ~/.local/share/complytime/providers/
```

The test provider responds to all RPCs (Describe, Generate, Scan) with predefined pass results. Evaluator ID: `test`.

### Step 3: Create workspace config

```bash
cat > complytime.yaml << 'EOF'
policies:
  - url: http://localhost:8765/nist-800-53-r5
    id: nist-800-53-r5
variables:
  workspace: /tmp/manual-test
targets:
  - id: local
    policies:
      - nist-800-53-r5
    variables:
      env: manual-test
EOF
```

### Step 4: Fetch policies

```bash
bin/complyctl get
```

**Verify:**

```bash
ls ~/.cache/complytime/policies/nist-800-53-r5/
cat ~/.local/share/complytime/state.json | jq .
```

Expected: `oci-layout` file exists, state.json contains policy digest and version.

### Step 5: List cached policies

```bash
bin/complyctl list
```

Expected: `nist-800-53-r5` appears with cached version.

### Step 6: Generate

```bash
bin/complyctl generate --policy-id nist-800-53-r5
```

Expected: `Generation completed.` output. Provider receives Generate RPC with assessment configurations extracted from the policy layer.

### Step 7: Scan — EvaluationLog only (default)

```bash
bin/complyctl scan --policy-id nist-800-53-r5
```

**Verify:**

```bash
ls .complytime/scan/
cat .complytime/scan/evaluation-log-*.yaml
```

Expected: Single `evaluation-log-*.yaml` file. No OSCAL, SARIF, or Markdown files.

### Step 8: Scan — OSCAL format

```bash
rm -rf .complytime/scan
bin/complyctl scan --policy-id nist-800-53-r5 --format oscal
```

**Verify:**

```bash
cat assessment-results-*.json | jq '.["assessment-results"].metadata'
```

Expected: `oscal-version: "1.1.3"`, results array with findings.

### Step 9: Scan — Markdown format

```bash
rm -rf .complytime/scan
bin/complyctl scan --policy-id nist-800-53-r5 --format pretty
```

**Verify:**

```bash
cat report-*.md
```

Expected: Markdown with `# Compliance Scan Report` header, target sections, step results.

### Step 10: Scan — SARIF format

```bash
rm -rf .complytime/scan
bin/complyctl scan --policy-id nist-800-53-r5 --format sarif
```

**Verify:**

```bash
cat scan-*.json | jq '.version'
```

Expected: SARIF version `"2.1.0"`.

### Step 11: Negative tests

```bash
# Invalid format
bin/complyctl scan --policy-id nist-800-53-r5 --format pdf
# Expected: error containing "invalid format"

# Missing policy (without running get)
rm -rf ~/.cache/complytime/policies
bin/complyctl scan --policy-id nonexistent
# Expected: error containing "not in cache"
```

### Cleanup

```bash
rm -rf .complytime/scan complytime.yaml
rm -rf ~/.cache/complytime/policies ~/.local/share/complytime/state.json
rm ~/.local/share/complytime/providers/complyctl-provider-test
```

## Adding New Tests

1. Add a `TestE2E_*` function in `e2e_test.go` using helpers from `helpers_test.go`.
2. Use `startMockRegistry(t)` for an isolated in-process registry per test.
3. Use `installTestPlugin(t, homeDir)` to deploy the test provider.
4. Use `runComplytime(t, binary, workDir, env, args...)` to execute commands (fatals on non-zero exit). For tests expecting failure, use `exec.Command` directly and assert `err != nil`.
5. To seed complypack artifacts, use the `addComplypackArtifact` closure inside `startMockRegistry`. See `DuplicateComplypackEvaluatorID` for an example.
6. Run: `make test-e2e`
