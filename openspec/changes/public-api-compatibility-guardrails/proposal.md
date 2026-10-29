# Proposal

## Why

complyctl v1 commits its provider-facing Go and protobuf contracts to stability,
but local CI does not enforce the already-configured Buf breaking rules and does
not compare exported Go APIs with a release baseline. An incompatible change can
therefore merge and force downstream providers or integrations into an unplanned
major-version migration.

## What Changes

- Add strict local and CI compatibility checks for `api/plugin` protobuf
  contracts and the `pkg/provider` public Go API.
- Accept stable local Buf versions at or above `1.50.0`, with validated
  semantic-version comparison and clear tool-setup errors. CI continues
  installing exactly Buf `1.50.0`; apidiff retains its exact revision.
- Resolve one comparison baseline: the latest stable release in the current
  major version. Ignore prerelease tags and report the selected tag and commit.
- Fail checks when baseline resolution, tool installation, package loading, or
  analysis fails; these failures MUST NOT be treated as compatible APIs.
- Require compatibility results during pull-request CI and release preflight.
- Record policy requirements for a future maintainer-authorized major
  migration. This change implements no breaking-change override; future
  authorization must retain diagnostics and reject operational failures.
- Document compatibility-tool limitations, including unkeyed composite literals
  and runtime interoperability, and link the downstream validation work in
  complytime-providers#187.

## Maintainer Authorization

The user stated in this session, "I'm a maintainer, I approve," authorizing
implementation of #880 CI/release gate changes. This authorization applies to
this change only; it does not authorize a future compatibility exception.

## Capabilities

### New Capabilities

- `public-api-compatibility`: Release-relative compatibility enforcement
  and future major-migration policy guidance for provider-facing APIs.

### Modified Capabilities

- None.

## Impact

- Affects `Makefile`, `.github/workflows/ci_local.yml`, and
  `.github/workflows/release.yml`.
- Pins compatibility-analysis tooling in CI and the local Go API analyzer;
  local protobuf checks accept supported Buf versions from `1.50.0` onward.
- Covers the public `pkg/provider` Go package and `api/plugin` protobuf
  contract; it does not replace provider-side build and integration validation
  tracked by complytime-providers#187.
- Records maintainer confirmation of required-check enforcement because it
  changes CI quality gates.
