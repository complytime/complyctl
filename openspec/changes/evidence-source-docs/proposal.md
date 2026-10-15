# Proposal

## Why

PR #840 added evidence source provenance (`EvidenceMapping`) support
across complyctl's entire pipeline -- proto API, provider SDK,
evaluator, and Markdown formatter. All existing documentation is
developer-oriented (CHANGELOG, AGENTS.md, OpenSpec artifacts). Users
running `complyctl scan` encounter new `source:` blocks in
EvaluationLog output and `source: <ref> @ <coord>` lines in Markdown
reports with no documentation explaining what they mean or how to
interpret them. Auditors, SREs, and CI pipeline authors have no
reference for tracing a compliance finding back to the artifact that
produced it.

The complytime-providers PR #192 is adding evidence population to
all three providers (OpenSCAP, Ampel, OPA), making this documentation
timely -- users will see these fields as soon as providers ship the
update.

## What Changes

- **New `docs/EVIDENCE.md`**: Dedicated evidence source provenance
  reference document. Walks through a real Ampel branch-protection
  EvaluationLog with an auditor-narrative approach, covering the
  five `EvidenceSource` fields (`reference-id`, `coordinate`,
  `entry-id`, `digest`, `remarks`), how `mapping-references`
  cross-reference evidence entries, and how different audiences
  (auditors, CI pipelines, SREs) use each field. Includes a field
  reference table and a diagram showing the `reference-id` to
  `mapping-references` relationship.

- **`docs/QUICK_START.md` update (Step 7)**: Add a brief
  "Understanding Scan Output" subsection after the scan output
  path note. Shows one annotated evidence example, the Markdown
  rendering, and links to `EVIDENCE.md` for the full walkthrough.

- **`docs/man/complyctl.md` update**: Add an OUTPUT section
  documenting evidence source provenance fields and scan report
  filenames. This content ships with the RPM package as the man
  page.

## Capabilities

### New Capabilities

None. This is a documentation-only change. No new system behavior
is introduced.

### Modified Capabilities

None. The evidence source provenance feature is already implemented
in complyctl (PR #840). This change documents existing behavior
without modifying it.

## Impact

- **Files modified**: `docs/QUICK_START.md`, `docs/man/complyctl.md`, `CHANGELOG.md`
- **Files created**: `docs/EVIDENCE.md`
- **No code changes**: Documentation only
- **No API changes**: Describes existing proto/SDK types
- **No dependency changes**
- **Cross-repo**: Complements complytime-providers PR #192 which
  populates evidence fields in providers. The documentation uses
  real output from that PR as examples.
- **References**:
  - Implementation: complytime/complyctl#840
  - Original issue: complytime/complyctl#876
  - Jira: GRCAUTO-101
  - Provider adoption: complytime/complytime-providers#192
