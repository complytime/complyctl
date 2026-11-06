# Spec Delta

## Purpose

Let maintainers inspect the generated upstream release delta before
tagging or publishing, using the same changelog settings as releases.

## ADDED Requirements

### Requirement: FR-001 Single-command preview

The project MUST provide `make release-notes-preview` without a
required tag or baseline argument. It MUST render release-note
Markdown to standard output and send diagnostics to standard error.

#### Scenario: Preview without choosing a tag

- **GIVEN** the required tools and GitHub authentication are available
- **WHEN** a maintainer runs `make release-notes-preview`
- **THEN** the command outputs generated release-note Markdown
- **AND** diagnostics identify the baseline and candidate commit
- **AND** the maintainer does not supply a proposed release tag

### Requirement: FR-002 Automatic canonical baseline

Every invocation MUST discover GitHub's latest published stable
release in `complytime/complyctl`. It MUST NOT select a baseline
from local tags or a fork, accept a baseline override, or use a
cached baseline when upstream discovery fails.

#### Scenario: Local tags and inherited overrides differ

- **GIVEN** local tags or inherited renderer settings name another tag
- **WHEN** the command discovers the latest upstream release
- **THEN** it uses that upstream release as the comparison baseline

#### Scenario: No published stable release is available

- **GIVEN** upstream discovery fails or returns no stable release
- **WHEN** a maintainer requests a preview
- **THEN** the command exits unsuccessfully with a useful diagnostic
- **AND** it does not generate notes from another baseline

### Requirement: FR-003 Explicit commit comparison

The preview MUST compare the selected release with the checkout's
resolved HEAD commit, not uncommitted files. It MUST report that
commit scope. An unavailable candidate or a candidate behind or
diverged from the baseline MUST fail without misleading notes.

#### Scenario: Uncommitted files exist

- **GIVEN** the checkout contains modified or untracked files
- **WHEN** the maintainer runs the preview
- **THEN** the comparison endpoint remains the resolved HEAD commit
- **AND** the diagnostic explains that uncommitted changes are absent
- **AND** the command leaves those files untouched

#### Scenario: Candidate cannot be compared

- **GIVEN** GitHub cannot resolve HEAD or reports it behind or diverged
- **WHEN** the command attempts the upstream comparison
- **THEN** it exits unsuccessfully without partial release notes
- **AND** it explains the comparison failure and relevant next action

### Requirement: FR-004 Release renderer fidelity

The preview MUST use GoReleaser's changelog generator and preserve
the checkout's `.goreleaser.yaml` changelog settings. It MUST NOT
duplicate grouping, sorting, filtering, author attribution, or
formatting logic in a separate renderer.

#### Scenario: Generated notes match a published release

- **GIVEN** identical changelog settings and generator version
- **AND** a historical comparison used by a published release
- **WHEN** the preview renderer processes that test comparison
- **THEN** its notes match the published generated body apart from
  trailing whitespace

#### Scenario: Configuration cannot support the preview

- **GIVEN** the changelog configuration is missing or invalid
- **WHEN** a maintainer requests a preview
- **THEN** the command exits unsuccessfully with a useful diagnostic
- **AND** it does not substitute hardcoded changelog settings

### Requirement: FR-005 Canonical comparison from forks

Both release discovery and changelog retrieval MUST use canonical
upstream when `origin` points to a fork. The command MUST NOT
persist changes to Git configuration to achieve this behavior.

#### Scenario: Contributor fork is origin

- **GIVEN** `origin` points to a fork and a canonical remote exists
- **WHEN** the maintainer requests a preview
- **THEN** release discovery and changelog retrieval use
  `complytime/complyctl`
- **AND** all persistent remote and branch settings remain unchanged

#### Scenario: Canonical repository resolution fails

- **GIVEN** the command cannot establish canonical changelog retrieval
- **WHEN** a maintainer requests a preview
- **THEN** it fails with guidance rather than using the fork

### Requirement: FR-006 Preview has no publishing side effects

The command MUST NOT build complyctl, execute configured hooks,
create or move tags, publish releases or artifacts, or modify
tracked files or persistent Git settings. Temporary configuration
and generated files MUST stay outside the checkout and be removed
on successful or failed completion.

#### Scenario: Publishing hooks exist in release configuration

- **GIVEN** the release configuration contains build and signing hooks
- **WHEN** a maintainer requests a preview
- **THEN** those hooks and application builds do not execute
- **AND** only temporary preview files are created
- **AND** the temporary files are removed after notes are displayed

#### Scenario: Renderer fails after creating temporary output

- **GIVEN** GoReleaser fails while generating the notes
- **WHEN** the preview command handles that failure
- **THEN** it removes its temporary files and exits unsuccessfully
- **AND** it emits no partial release-note Markdown

### Requirement: FR-007 Empty delta is explicit

When the selected release and candidate identify the same commit,
the command MUST succeed and report that no unreleased commits
exist. It MUST NOT fall back to an earlier release to populate
the preview.

#### Scenario: HEAD is already released

- **GIVEN** the latest upstream release points at HEAD
- **WHEN** a maintainer requests a preview
- **THEN** the command reports no unreleased commits and succeeds
- **AND** no already released entries appear in the preview

### Requirement: FR-008 Actionable and bounded failures

Missing prerequisites, authentication errors, invalid responses,
timeouts, and renderer failures MUST produce nonzero exits and
actionable diagnostics. Child commands MUST have finite timeouts.
Diagnostics MUST NOT expose authentication tokens.

#### Scenario: A prerequisite is missing

- **GIVEN** an installed tool or Python YAML support is unavailable
- **WHEN** a maintainer requests a preview
- **THEN** the command identifies the missing prerequisite
- **AND** it exits unsuccessfully before running GoReleaser

#### Scenario: An external command fails or times out

- **GIVEN** GitHub authentication fails or a child command times out
- **WHEN** a maintainer requests a preview
- **THEN** the command terminates with an actionable diagnostic
- **AND** it does not print tokens or partial release notes
