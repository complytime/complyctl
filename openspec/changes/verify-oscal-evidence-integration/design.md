# Design

## Context

See `proposal.md` for motivation and the accompanying spec for the
acceptance contract. PR #956 delegates conversion to go-gemara and
validates OSCAL 1.1.3 before writing the report. Its formatter tests
verify evidence, but begin with an in-memory EvaluationLog.

`TestAcceptance_FormatInterop` already runs the real CLI against the
lifecycle registry and test provider. It indexes observations by UUID
and checks the `force-push-protection` finding's requirement linkage.
The provider emits evidence descriptions, source reference IDs,
coordinates, and digests. This is sufficient input for the new checks.

## Goals / Non-Goals

**Goals:**

- Assert evidence on the assessment reached through the expected
  finding, with diagnostics that identify the missing field or link.
- Validate the report read from disk independently of the production
  conversion path.
- Keep negative checks isolated from the registry and provider so
  assertion failures are inexpensive to diagnose.

**Non-Goals:**

- Change production conversion, provider APIs, or report semantics.
- Add another container stack or duplicate the same assertions across
  shell integration, E2E, and acceptance suites.
- Expand upstream provenance-resource behavior beyond source fields
  supplied by the existing provider fixture.

## Decisions

### Extend the existing format interoperability test

Use `TestAcceptance_FormatInterop` for the positive full-pipeline
case. Preserve its finding and requirement checks, then verify the
associated observation's evidence description and source properties
against the seeded assessment and provider fixture. Resolve UUID
references rather than assuming slice order or fixed UUIDs.

A second CLI workflow would add runtime and setup duplication without
testing a different boundary. Formatter-only additions would leave
provider transport and EvaluationLog construction outside the check.

### Validate the complete serialized report

Add a focused acceptance helper that reads the generated JSON into
the existing OSCAL 1.1.3 model and invokes the existing go-oscal
validator. Preserve access to the full document rather than validating
only the first result returned by `parseOSCALResult`.

Evidence assertions may continue using the existing decoded-map
pattern. Sharing small parsing helpers within the acceptance package
is preferable to importing test helpers from `internal/output`.
No new dependency or network-fetched schema is needed.

### Prove the checks reject evidence loss

Keep report inspection checks in focused error-returning helpers so
negative cases can assert failures without deliberately failing a
parent test. Test copies of a known-valid report with evidence removed,
emptied, or changed, and with the expected finding link removed or
redirected to an unrelated observation. Leave observations and
findings otherwise intact so failure identifies the coverage gap.

For schema validation, mutate an evidence URI to an invalid value
already rejected by the project's validator coverage. Assert rejection
without changing the CLI's production validation behavior. Use the
acceptance build tag and temporary files where filesystem access is
needed. Avoid live-network dependencies in these negative cases.

Schema validation alone is insufficient: `relevant-evidence` is
optional, so a valid document may still have lost provider evidence.

## Risks / Trade-offs

- Fixture drift can break provenance expectations. → Derive expected
  values from the documented seeded assessment and provider fixture;
  do not snapshot generated UUIDs or collection timestamps.
- A report-wide search could accept evidence on the wrong assessment.
  → Follow the expected finding's UUID links and verify requirement
  identity before checking evidence.
- Default unit tests omit acceptance-tagged helpers. → Compile and
  run the acceptance suite explicitly, including its negative cases.
- Container infrastructure can fail independently of assertions.
  → Report infrastructure failures separately and retain normal
  Compose diagnostics; do not weaken assertions to obtain a pass.

## Migration Plan

No report or configuration migration is required. Add the checks to
the existing acceptance binary and run the CI-equivalent lifecycle
command, `make test-acceptance COMPOSE="docker compose"`.
Rollback consists of reverting the acceptance-test change.

During implementation, run the affected formatter tests and the
project's lint and unit checks. Run the acceptance-tagged lint check
as well, since normal linting does not necessarily include that tag.
Assess `CHANGELOG.md`, `AGENTS.md`, and `README.md`; no description,
structure, or build-command update is currently expected.
