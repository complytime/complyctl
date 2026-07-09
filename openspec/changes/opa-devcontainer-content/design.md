## Context

The devcontainer environment provides a one-command path to
interactive CLI testing. It currently seeds a single Ampel
policy (`test-ampel-bp`) via the mock OCI registry and
configures the test workspace to exercise the `get` ->
`generate` -> `scan` pipeline with the Ampel provider.

PR #536 adds complypack pull support to complyctl, including
a `complypacks:` section in `complytime.yaml`, complypack
OCI artifact seeding in the mock registry
(`addComplypackArtifact()`), and the
`GenerateRequest.ComplypackContentPath` proto field that
delivers cached complypack content paths to providers.

PR #538 extended the mock registry with `seedFromDirectory()`
for serving mounted Gemara YAML files. The OPA provider (PR #31
in complytime-providers) implements Generate with
mapping-based Rego namespace filtering via `conftest`.

A companion PR in complytime-providers will update the OPA
provider to consume `ComplypackContentPath` as an alternative
to `opa_bundle_ref` + `conftest pull`.

## Goals / Non-Goals

**Goals:**
- Add embedded OPA Gemara testdata (catalog + policy) to the
  mock registry so `complyctl get` fetches an OPA policy
- Seed an OPA complypack artifact in the mock registry
  containing Rego policies and `complytime-mapping.json`
- Configure the test workspace `complytime.yaml` with the OPA
  policy-id and complypack entry
- Enable the full `get` -> `generate` -> `scan` pipeline with
  the OPA provider in the devcontainer
- Coexist with the existing Ampel workflow — both providers
  testable in the same environment

**Non-Goals:**
- Modifying the OPA provider itself (separate PR in
  complytime-providers)
- Adding OPA-specific granular policy content beyond what is
  needed for a minimal end-to-end test
- Supporting mounted private OPA bundles via
  `seedFromDirectory()` (that generalizes naturally from the
  existing mechanism in PR #538)

## Decisions

### D1: Embed OPA testdata in mock registry

OPA Gemara testdata (catalog.yaml + policy.yaml) is embedded
in `cmd/mock-oci-registry/testdata/` and seeded by
`seedDefaults()`, matching the existing Ampel pattern. This
avoids requiring users to mount external files for the
standard OPA test scenario.

Alternative: Require users to mount OPA content via
`seedFromDirectory()`. Rejected because the goal is
zero-setup end-to-end testing in the devcontainer.

### D2: Complypack artifact seeded via addComplypackArtifact

The OPA complypack (Rego files + `complytime-mapping.json`)
is seeded as a complypack OCI artifact in the mock registry
using `addComplypackArtifact()` from PR #536. The
`complytime.yaml` references it in the `complypacks:` section
so `complyctl get` fetches it into the complypack cache.

Alternative: Pre-populate the complypack cache in
post-create.sh via shell scripting. Rejected — same reasoning
as PR #538: let the registry serve content and `complyctl get`
populate the cache through normal code paths.

### D3: Policy-id naming convention

The OPA test policy uses `test-opa-bp` as the policy-id,
following the existing `test-ampel-bp` naming pattern. The
mock registry repository is `policies/test-opa-policy`.

### D4: Minimal Rego content for end-to-end validation

The embedded Rego policy is a minimal check that validates
the pipeline works end-to-end. It does not need to be a
production-quality compliance rule — it needs to produce a
pass/fail result that exercises the OPA provider's Generate
(mapping resolution) and Scan (conftest evaluation) paths.

## Risks / Trade-offs

- **[Dependency on PR #536]** This change cannot be
  implemented until PR #536 merges, since it depends on the
  `complypacks:` config schema, `addComplypackArtifact()`,
  and `ComplypackContentPath` proto field. Mitigation: PR is
  created now with a dependency note; implementation begins
  after 536 lands.
- **[Dependency on provider update]** The OPA provider must
  be updated to consume `ComplypackContentPath` for the
  end-to-end to work. Mitigation: Companion PR in
  complytime-providers; the complyctl-side changes are
  independently mergeable (testdata + config are inert until
  the provider supports the path).
- **[Rego content scope]** The minimal Rego policy may not
  exercise all OPA provider capabilities. Acceptable for
  initial integration — more complex scenarios can be added
  later.
