# Design

## Context

See `proposal.md` for the motivation and
`specs/public-api-compatibility/spec.md` for the required behavior. Buf already
declares FILE-level breaking rules, but the local CI workflow runs only `buf
lint`. The repository has no Go API comparison command. Release preflight uses
an explicit list of CI checks, so a new job must be named and registered
consistently.

The current release tooling can identify the newest tag but does not select a
stable tag within a particular major version. The compatibility workflow must
therefore resolve and materialize its baseline without relying on a shallow
checkout. Provider-side compilation and runtime integration remain owned by
complytime-providers#187.

## Goals / Non-Goals

**Goals:**

- Give contributors a reproducible local command and CI result for protobuf and
  Go API compatibility.
- Fail closed on missing baselines and analysis operational errors.
- Retain policy guidance for a future audited major-version migration.
- Preserve existing runtime and cross-repository tests as complementary checks.

**Non-Goals:**

- Detect every Go source or runtime compatibility regression.
- Test providers from the complyctl repository.
- Change published APIs, release numbering, or existing quality thresholds.
- Allow an exception to bypass analyzer, package-loading, or baseline failures.

## Decisions

### Use a shared stable same-major baseline

A compatibility helper SHALL derive the candidate major version, list version
tags, discard prereleases, and select the highest stable tag in that major. It
shall resolve the selected tag to a commit once, print both values, and pass
that commit to the Buf and Go checks.

Using the latest stable same-major release protects interfaces added after
`v1.0.0`; comparing only with `v1.0.0` would miss later public additions.
Version sorting must be semantic rather than based on tag creation dates.

Alternatives considered:

- Compare only with `v1.0.0`: rejected because it does not protect APIs added
  in later v1 releases.
- Compare with the highest tag including prereleases: rejected because a
  release candidate is not a supported public baseline.
- Use separate baselines per analyzer: rejected because diagnostics would be
  inconsistent and difficult to audit.

### Materialize the baseline in an isolated worktree

The helper SHALL ensure tags and the selected commit are available, then use an
isolated temporary worktree for baseline commands. This makes local and CI
results independent of checkout depth and prevents baseline files from mixing
with the candidate workspace. Cleanup SHALL run on success and failure.

`buf breaking` shall compare the candidate protobuf module with the materialized
baseline module. The Go API analyzer shall compare `pkg/provider` in the
candidate with the same package at the baseline commit. Generated protobuf Go
code is covered by Buf rather than treated as a second independent Go surface.

Alternatives considered:

- Rely on `.git#tag=` references: rejected because shallow CI checkouts may not
  contain the selected tag or object.
- Fetch the entire repository history: rejected because only the selected tag
  and commit are needed.

### Make compatibility tools explicit and strict

New Make targets SHALL invoke the baseline helper and validate the
installed compatibility tools. Local protobuf analysis MUST accept a
stable Buf semantic version greater than or equal to `1.50.0`, so
contributors can use newer installations without downgrading. Version
comparison MUST use numeric major, minor, and patch components after
validating the complete version. Valid build metadata MUST NOT affect
precedence. Prereleases and malformed versions MUST be rejected.

When an older stable version is installed, the tool-setup error SHALL
state the minimum and detected versions, for example:
`buf >= 1.50.0 required, found 1.49.0`. A missing executable or failed
version command MUST still return non-zero with a tool-setup diagnostic.
Passing version validation MUST NOT suppress incompatibility or analyzer
operational failures.

CI MUST continue installing exactly Buf `1.50.0` for reproducible merge
and release evidence. Local runs with newer Buf versions do not replace
the required CI result. The Go API analyzer MUST retain its exact
`golang.org/x/exp` module revision; this change does not introduce
minimum-version acceptance for `apidiff`. CI installation versions and
methods SHALL remain recorded for automated dependency updates.

The helper shall report baseline resolution, tool setup, package loading, and
incompatible API failures. The current CI policy fails closed on every failure.

Alternatives considered:

- Reuse `lint-proto`: rejected because it can succeed after printing an
  installation hint and does not perform breaking analysis.
- Add the analyzer to application dependencies: rejected because it is CI and
  development tooling, not a runtime dependency.
- Require exactly Buf `1.50.0` locally: rejected because it prevents
  contributors from using newer installed versions. CI retains the exact
  installation for reproducibility.
- Accept any Buf version: rejected because older, malformed, or prerelease
  versions do not satisfy the supported local tooling requirement.

### Add one required compatibility job

`ci_local.yml` shall add a dedicated, SHA-pinned `api-compatibility` job that
runs the strict compatibility command with read-only credentials. The precise
GitHub check name shall be verified after the job is introduced and then added
to the explicit `ci_checks` list used by release preflight.

The check reports the selected baseline, commit, analyzer results, and any
incompatibility. It runs on pull requests and pushes to the supported release
line. Existing unit, integration, and cross-repository tests remain unchanged.

### Defer major-version authorization

The current major has no breaking-change override. `api-compatibility` runs on
pull requests and pushes to `main`, the current supported release line, and
fails on incompatible contracts and operational errors. The workflow contains
no next-major branch, exception job, GitHub Environment, or approval logic.

When maintainers approve an actual future major migration, they must separately
design and review an authorization workflow. It must retain diagnostics, bind
authorization to the reviewed commit, require consumer impact and migration
steps plus downstream-provider coordination, and reject baseline, tool, and
package-loading failures. This change records that future policy guidance; it
does not provision or reference its implementation.

Alternatives considered:

- A PR label: rejected because it does not prove who authorized it or bind
  authorization to a commit.
- A commit trailer: rejected because contributors control commit text.
- Silently skipping compatibility checks for a major branch: rejected because
  it hides diagnostics and permits operational failures.

## Risks / Trade-offs

- [Newer local Buf versions can differ from CI] → Keep the exact CI
  installation as the required merge and release result; document the
  distinction without claiming identical analyzer behavior across versions.
- [Future major authorization requires repository configuration] → Defer its
  design and configuration until a major migration is approved; current CI has
  no authorization path.
- [Version tags can be malformed or unavailable] → Parse only valid stable
  semantic-version tags and fail with the selected-major context when none are
  usable.
- [Go API tooling cannot detect every source incompatibility] → Document
  unkeyed-literal and behavioral limits; retain provider-side validation in #187.
- [Baseline worktrees increase CI time] → Fetch only the resolved baseline
  commit, clean temporary worktrees, and run one shared baseline resolution.
- [Required-check names can differ from job IDs] → Verify the observed GitHub
  check name before adding it to release preflight configuration.

## Migration Plan

1. Add and validate strict local compatibility commands against compatible and
   incompatible fixtures.
2. Add the read-only CI job and confirm its reported check name.
3. Configure required pull-request and release-preflight checks with maintainer
   approval; do not remove existing checks.
4. When a future major migration is approved, propose a separate authorization
   workflow before enabling any compatibility exception.
5. Publish contributor and release guidance, including the #187 handoff.
6. Roll back by removing the new job from required-check configuration only if
   it blocks due to an infrastructure defect; keep the diagnostic command and
   record the reason for the temporary policy change.
