# Feature Specification: AMPEL Branch Protection Scanning Plugin

**Feature Branch**: `001-ampel-branch-scan`
**Created**: 2026-02-11
**Status**: Draft
**Input**: User description: "AMPEL plugin for complyctl that scans branch protection rules on GitHub and GitLab repositories"

## Clarifications

### Session 2026-02-11

- Q: What information does the target repository configuration file contain per entry? → A: Repository URLs plus target branch names to evaluate.
- Q: What happens when generate or scan is re-run and artifacts already exist? → A: Overwrite existing artifacts on each run.
- Q: What happens when the same repository appears multiple times in the target configuration? → A: Warn the user about duplicates and deduplicate before scanning.
- Q: Should the plugin enforce minimum AMPEL tool versions? → A: Check presence only; do not validate versions.
- Q: How should the plugin handle GitHub/GitLab API rate limiting? → A: Report rate limit error for affected repo and continue scanning remaining repos.

### Session 2026-02-17

- Q: How are AMPEL policies authored and consumed? → A: Granular AMPEL policies (one JSON file per control) are authored independently and stored in the policy_dir. The plugin matches OSCAL rules to these policies by ID and merges only the matching ones into a combined bundle at generate time. This replaces the earlier approach of generating CEL expressions from OSCAL rules.
- Q: What snappy spec files should be used per repository? → A: Each repository in the targets file MUST specify one or more spec references via a `specs` field. Specs can use the `builtin:` prefix for embedded specs (e.g., `builtin:github/branch-rules.yaml`) or absolute paths for custom specs.
- Q: How does snappy authenticate to the GitHub API? → A: The `GITHUB_TOKEN` environment variable must be set with a valid personal access token before running a scan. The token is consumed by snappy, not by the plugin directly.
- Q: What output format does ampel verify produce? → A: `ampel verify --attest-results` produces DSSE-wrapped in-toto attestations. The plugin must unwrap the DSSE envelope (base64-decode the payload) before parsing the result predicate.
- Q: How are results from multiple targets represented in OSCAL? → A: Findings with the same CheckID across repositories are grouped into a single OSCAL ObservationByCheck with multiple Subjects (one per repository). This matches the OSCAL pattern and prevents last-write-wins overwrites in the downstream observation manager.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Generate AMPEL Policies from Assessment Plan (Priority: P1)

A compliance administrator has already created an assessment plan
using `complyctl plan`. They now run `complyctl generate` to
produce the plugin-specific policy artifacts. The AMPEL plugin
receives the OSCAL rules from the assessment plan and matches
them against granular AMPEL policy files (one per control) stored
in the policy directory. It then merges only the matching policies
into a combined policy bundle. When more granular policies exist
than the assessment plan requires, the plugin honors the complyctl
policy scope and includes only what the assessment plan specifies.
The user does not need to understand AMPEL policy syntax or CEL
expressions.

**Why this priority**: Without policy generation, scanning cannot
occur. This is the first step in the complyctl workflow after
planning and directly enables the scan step.

**Independent Test**: Run `complyctl generate` with an assessment
plan containing branch protection rules. Verify AMPEL policy
files are created in the workspace with only the controls
specified in the assessment plan, regardless of any pre-existing
broader AMPEL policy.

**Acceptance Scenarios**:

1. **Given** an assessment plan with branch protection controls,
   **When** the user runs `complyctl generate`,
   **Then** AMPEL policy files are created in the workspace
   covering exactly the controls specified in the assessment plan.

2. **Given** a pre-existing AMPEL policy that covers more controls
   than the complyctl assessment plan,
   **When** the user runs `complyctl generate`,
   **Then** the generated policy includes only the controls from
   the complyctl assessment plan, ignoring the broader scope of
   the existing AMPEL policy.

3. **Given** an assessment plan with no controls mapped to the
   AMPEL plugin,
   **When** the user runs `complyctl generate`,
   **Then** the plugin produces no output and does not error.

---

### User Story 2 - Scan Branch Protection Rules Across Repositories (Priority: P1)

A DevSecOps engineer runs `complyctl scan` to evaluate branch
protection rules on their GitHub and GitLab repositories. The
AMPEL plugin invokes the installed AMPEL toolchain (ampel, snappy,
ampel) to verify attestations and branch protection configurations.
The plugin produces a separate result file for each scanned
repository in the workspace, and returns standardized OSCAL
assessment results to complyctl. The user interacts only with
complyctl commands and sees results in the same format as any
other plugin.

**Why this priority**: Scanning is the core value proposition of
the plugin. This story delivers the primary capability users need.

**Independent Test**: With a generated AMPEL policy in place,
run `complyctl scan` targeting at least two repositories (one
GitHub, one GitLab). Verify that per-repository result files
appear in the workspace and that `assessment-results.json`
contains observations for each repository.

**Acceptance Scenarios**:

1. **Given** generated AMPEL policies and configured target
   repositories,
   **When** the user runs `complyctl scan`,
   **Then** each repository is scanned for branch protection
   compliance and a separate result file is created per
   repository in the workspace.

2. **Given** multiple target repositories across GitHub and GitLab,
   **When** the scan completes,
   **Then** the assessment results contain distinct observations
   per repository, each identified by the repository name and
   hosting platform.

3. **Given** a repository whose branch protection does not meet
   policy requirements,
   **When** the scan completes,
   **Then** the corresponding result clearly indicates which
   specific rules failed and which branch protection settings
   were evaluated.

4. **Given** a repository that is unreachable or returns an error,
   **When** the scan completes,
   **Then** the result for that repository indicates an error
   status with a descriptive reason, and scanning continues for
   the remaining repositories.

---

### User Story 3 - Validate Required AMPEL Tools (Priority: P2)

Before performing any operation, the plugin checks that the
required AMPEL tools (snappy, ampel) are installed and
accessible on the system. If any tool is missing, the plugin
provides a clear error message identifying which tools are not
found and guidance on how to resolve the issue. This prevents
confusing runtime failures and helps users set up their
environment correctly.

**Why this priority**: While critical for a good user experience,
this is a pre-condition check rather than core functionality. The
plugin can technically function for partial operations without all
tools, but missing tools MUST be reported clearly.

**Independent Test**: Remove one of the required tools from the
system PATH and run `complyctl generate` or `complyctl scan`.
Verify the plugin reports the missing tool by name with
actionable guidance.

**Acceptance Scenarios**:

1. **Given** all required tools (snappy, ampel) are installed,
   **When** the user runs `complyctl generate` or `complyctl scan`,
   **Then** the plugin proceeds without any tool-related warnings.

2. **Given** one or more required tools are missing from the system,
   **When** the user attempts any plugin operation,
   **Then** the plugin reports each missing tool by name and
   provides guidance on how to install it, before the operation
   fails.

3. **Given** a required tool is installed but not on the system
   PATH,
   **When** the user attempts a plugin operation,
   **Then** the error message suggests checking the PATH
   configuration.

---

### User Story 4 - Configure AMPEL Policy Location (Priority: P2)

The user can specify where the plugin reads and writes AMPEL
policy files. By default, the plugin uses an intuitive location
within the complyctl workspace. Users who have existing AMPEL
policies in a different location can override the default through
plugin configuration without modifying their existing policy file
structure.

**Why this priority**: The default location covers most users.
Custom configuration is needed for teams with established AMPEL
policy repositories, but is not required for initial adoption.

**Independent Test**: Run `complyctl generate` without custom
configuration and verify policies appear in the default location.
Then configure a custom policy path and verify policies are read
from and written to the custom location.

**Acceptance Scenarios**:

1. **Given** no custom policy location is configured,
   **When** the user runs `complyctl generate`,
   **Then** AMPEL policies are written to the default location
   within the complyctl workspace.

2. **Given** a custom policy location is configured via the
   plugin manifest or user configuration,
   **When** the user runs `complyctl generate`,
   **Then** AMPEL policies are written to the specified custom
   location.

3. **Given** a custom policy location that does not exist,
   **When** the user runs `complyctl generate`,
   **Then** the plugin creates the directory structure and writes
   policies to the specified location.

---

### Edge Cases

- When a repository has branch protection partially configured,
  each rule is evaluated independently. The per-repository result
  indicates which specific rules passed and which failed (see
  US2 AS3).
- When GitHub or GitLab API rate limiting is encountered, the
  plugin reports the rate limit error for the affected repository
  and continues scanning the remaining repositories.
- When the same repository+branch combination is listed multiple
  times, the plugin warns the user and deduplicates before
  scanning.
- The plugin checks for tool presence only; version
  compatibility is not validated. If an incompatible version
  causes a runtime failure, the error from the AMPEL tool is
  surfaced to the user.
- When the assessment plan contains no rules applicable to the
  AMPEL plugin, the plugin produces no output and does not error
  (see US1 AS3).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The plugin MUST implement the complyctl plugin
  interface (Configure, Generate, and GetResults) so it is
  invoked transparently through standard complyctl commands.
  Configure receives the plugin settings from the manifest;
  Generate and GetResults are invoked by `complyctl generate`
  and `complyctl scan` respectively.

- **FR-002**: The plugin MUST match OSCAL rules from the complyctl
  assessment plan against granular AMPEL policy files and merge
  the matching policies into a combined bundle during the generate
  phase, abstracting AMPEL-specific formats from the user.

- **FR-003**: When more granular AMPEL policies exist than the
  complyctl assessment plan requires, the plugin MUST honor the
  complyctl policy scope and include only the policies that match
  controls specified in the assessment plan.

- **FR-004**: The plugin MUST scan branch protection rules on
  both GitHub and GitLab repositories using the installed AMPEL
  toolchain (snappy, ampel).

- **FR-005**: The plugin MUST produce a separate result file for
  each scanned repository within the complyctl workspace
  directory structure.

- **FR-006**: The plugin MUST return OSCAL-compatible assessment
  results to complyctl, with each repository identified as a
  distinct subject in the observations.

- **FR-007**: The plugin MUST check for the presence of required
  system tools (snappy, ampel) before performing operations
  and report missing tools with clear, actionable error messages.

- **FR-008**: The plugin MUST allow users to configure the
  location where AMPEL policies are consumed from, with an
  intuitive default within the complyctl workspace.

- **FR-009**: The plugin MUST be discoverable by complyctl through
  a plugin manifest file, following the same conventions as other
  complyctl plugins.

- **FR-010**: The plugin MUST preserve the standard complyctl user
  experience so that users can switch between plugins (e.g.,
  OpenSCAP, AMPEL) without learning plugin-specific commands or
  formats.

- **FR-011**: The plugin MUST allow users to specify which
  repositories to scan through a dedicated configuration file
  in the complyctl workspace. Each entry MUST include the
  repository URL, the target branch names to evaluate, and the
  snappy spec file references to use. Specs can reference
  embedded files via the `builtin:` prefix or custom files via
  absolute paths. Each workspace can define its own set of
  target repositories, enabling different target sets for
  different environments or assessment contexts.

- **FR-012**: The plugin MUST handle DSSE-wrapped in-toto
  attestations produced by `ampel verify --attest-results`,
  unwrapping the DSSE envelope before parsing the result
  predicate.

- **FR-013**: The plugin MUST group findings with the same CheckID
  across multiple repositories into a single OSCAL observation
  with multiple subjects, rather than creating separate
  observations that would be overwritten by the downstream
  observation manager.

### Key Entities

- **Granular AMPEL Policy**: A single JSON file defining one
  CEL-based verification control for branch protection. Authored
  independently and stored in the policy directory. Multiple
  granular policies are matched against the assessment plan and
  merged into a combined bundle during the generate phase.

- **Target Repository**: A GitHub or GitLab repository whose
  branch protection settings are evaluated during scanning.
  Identified by repository URL, one or more target branch names
  to evaluate, and one or more snappy spec file references.

- **Per-Repository Result**: A file in the complyctl workspace
  containing detailed scan findings for a single repository,
  including which branch protection rules passed or failed.

- **Plugin Manifest**: The `c2p-ampel-manifest.json` file that
  describes the plugin's identity, executable location,
  configuration schema, and version.

### Assumptions

- The AMPEL tools (snappy, ampel) are independently
  installed and the plugin invokes them as external commands
  rather than embedding their logic.

- Authentication to the GitHub API is handled by snappy via
  the `GITHUB_TOKEN` environment variable. The token must be
  set before running a scan and needs read access to the
  target repositories.

- The complyctl workspace directory structure follows the
  established convention: `{workspace}/ampel/policy/` for
  policies and `{workspace}/ampel/results/` for scan outputs.

- Branch protection rules map to specific OSCAL controls that
  are defined in the compliance framework used by the assessment
  plan.

- Re-running `complyctl generate` or `complyctl scan` overwrites
  existing plugin artifacts in the workspace, consistent with the
  behavior of other complyctl plugins (e.g., OpenSCAP).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can complete a full compliance assessment
  cycle (plan, generate, scan) for branch protection rules
  using only standard complyctl commands, with no AMPEL-specific
  knowledge required.

- **SC-002**: Scanning 10 repositories produces 10 separate
  per-repository result files and a consolidated assessment
  result, all accessible through the complyctl workspace.

- **SC-003**: When required AMPEL tools are missing, 100% of
  missing tools are identified by name in the error message
  before any operation is attempted.

- **SC-004**: Policy generation honors the complyctl assessment
  plan scope: if the plan includes N controls, the generated
  AMPEL policy covers exactly those N controls regardless of
  any broader pre-existing AMPEL policy.

- **SC-005**: Results from the AMPEL plugin are indistinguishable
  in format from results produced by other complyctl plugins
  (e.g., OpenSCAP), enabling unified compliance reporting across
  plugin types.

- **SC-006**: Users with existing AMPEL policy directories can
  configure the plugin to use their existing location without
  restructuring their files.
