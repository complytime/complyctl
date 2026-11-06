# Design

## Context

See `proposal.md` for motivation and the delta specification for
the preview contract. The release workflow delegates publishing to
org-infra, while `.goreleaser.yaml` owns changelog settings and uses
GitHub's commit comparison API.

The completed feasibility experiment used open-source GoReleaser
2.20.0 with a commit SHA as the current reference and validation
disabled. It skipped application builds and publishing operations.
A historical replay generated 60 entries matching the published
v1.2.0 release body after trimming trailing whitespace. Automatic
latest-release discovery also produced the correct empty delta
after v1.2.0 was published at the checkout's HEAD.

The experiment exposed one important constraint: GoReleaser infers
the changelog repository from Git's default remote, not its
`release.github` setting. Fork checkouts therefore require an
explicit process-local canonical remote selection.

## Goals / Non-Goals

**Goals:**

- Keep a single renderer and changelog configuration source.
- Provide a redirectable Markdown preview without choosing a tag.
- Bound external commands and keep temporary files and credentials
  out of the checkout and command output.

**Non-Goals:**

- Recommend or select the next semantic version automatically.
- Summarize commits with AI or curate migration notes.
- Preview uncommitted changes or unpublished local commits.
- Add baseline overrides, a publishing dry-run workflow, or new CI
  gates.
- Change `.goreleaser.yaml` or org-infra workflows.

## Decisions

### Use a Python coordinator around installed tools

Add `scripts/release-notes-preview.py`, invoked quietly by the
Make target. Python's standard library provides subprocess timeouts,
temporary-directory cleanup, JSON parsing, and test support. Use
PyYAML's safe loader to read the release configuration without
recreating a YAML parser. Document Git, authenticated GitHub CLI,
Python 3, PyYAML, and GoReleaser as prerequisites.

The helper will check supported GoReleaser v2 versions, with
2.20.0 as the verified minimum. It will not install tools or add
Go module dependencies. A separate renderer was rejected because
formatting and attribution would drift from published notes.

### Discover once and bind the comparison to HEAD

Query `repos/complytime/complyctl/releases/latest` through `gh api`.
This selects GitHub's latest published stable release, not the
highest local tag or an unpublished tag. Validate the response and
resolve the checkout's HEAD once. Check the canonical comparison
endpoint before rendering; accept only an ahead or identical
candidate. Fail clearly on behind, diverged, or unavailable commits.

Overwrite inherited `GORELEASER_PREVIOUS_TAG` and
`GORELEASER_CURRENT_TAG` in the child environment with the discovered
tag and resolved SHA. Do not accept command-line or environment
baseline overrides. Report the commit-only scope on standard error.
Untracked files do not prevent previewing a committed candidate.

### Isolate the configuration and force canonical retrieval

Read the checkout's `.goreleaser.yaml` and copy its complete
`changelog` mapping into a minimal temporary configuration. Retain
the project name and configuration version; explicitly disable all
builds and checksums and place the distribution directory inside
the temporary directory. Do not copy configured hooks or publishing
integrations. Reject missing or incompatible configuration rather
than substitute settings.

Select an existing remote whose URL identifies
`complytime/complyctl`, preferring `upstream` over `origin`. For a
named branch, append process-only Git configuration making it the
child process's default remote. Verify the effective remote before
running GoReleaser. A detached checkout may proceed only when its
effective default remote is already canonical. Otherwise, fail with
guidance; do not mutate local branch or remote configuration.

Preserve existing process-level Git configuration entries when
adding overrides. The helper must not depend on its branch being
named `main`; the experiment verified the override on both `main`
and `opsx/release-notes-preview`.

### Invoke the existing generator without publishing

Run the installed GoReleaser with the temporary configuration and
the verified flags:

```text
release --skip=publish,before,sign,sbom,archive,validate
```

Do not use `--snapshot`, which skips changelog generation. Disable
validation only within this isolated invocation to permit an
untagged commit SHA; leave actual release gates untouched. Use the
GitHub CLI's configured authentication without printing tokens or
placing them in subprocess argument lists.

Capture renderer output and display actionable diagnostics on
failure. Emit Markdown only after GoReleaser succeeds and the
generated changelog exists. Report an identical comparison as
having no unreleased commits without selecting an earlier baseline.
Use finite timeouts and clean the temporary directory on exit.

### Test coordination, then verify real rendering

Add `tests/release_notes_preview_test.py` using standard-library
`unittest` with temporary fixtures and controlled subprocesses.
Exercise discovery, fork resolution, environment isolation, YAML
copying, build suppression, cleanup, output channels, empty deltas,
and failure paths without live GitHub access or publishing.

Expose `make test-release-notes-preview` for this suite. Keep it
separate from application tests and do not change CI workflow
configuration. Perform a live local preview after implementation
and compare a historical rendering with the generated published
body when validating renderer fidelity. Historical fixtures do not
introduce a public baseline override.

## Risks / Trade-offs

- [GoReleaser pipeline behavior changes] -> Check the supported
  version, preserve focused invocation tests, and document the
  verified minimum version.
- [Candidate SHA is unavailable on GitHub] -> Fail with guidance to
  publish the branch before previewing; never silently use another
  commit or a local-only renderer.
- [Latest release changes during a run] -> Resolve its tag once and
  print the selected tag and candidate SHA for traceability.
- [Additional local prerequisites] -> Document Python YAML support
  and use installed tools without automatic downloads.
- [Published notes were manually edited] -> Promise renderer parity
  for generated content, not reproduction of later editorial edits.

## Migration Plan

No data or publishing migration is required. Add the local helper,
tests, Make targets, and release-process guidance on the feature
branch. Validate locally before requesting a PR. Removing those
additions rolls back the feature without affecting releases.
