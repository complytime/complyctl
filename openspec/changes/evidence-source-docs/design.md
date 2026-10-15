# Design

## Context

See proposal.md for motivation. The evidence source provenance
feature is fully implemented in complyctl (PR #840) and being adopted
by providers (complytime-providers PR #192). The code surfaces five
`EvidenceSource` fields through three output paths (EvaluationLog
YAML/JSON, Markdown, OSCAL/SARIF), but no user-facing documentation
explains the output or how to interpret it.

Key code references for accuracy:
- `internal/output/markdown.go:271-290` -- `formatEvidenceMeta()`
  renders `source: <ref> @ <coord>` in Markdown
- `internal/output/evaluator.go:229-253` -- maps provider evidence
  to gemara types; nil-checks `Source` pointer
- `internal/output/evaluator.go:326-355` -- shadow structs with
  yaml/json tags defining serialized field names (kebab-case)
- `pkg/provider/client.go:102-123` -- `Evidence` and
  `EvidenceSource` structs
- `internal/output/scan_summary.go:225-279` --
  `MergeMappingReferences()` with policy-wins collision semantics

Real output example: PR #192 comment showing Ampel branch-protection
EvaluationLog with `mapping-references` and `evidence[].source`
populated (one control: `force-push-restriction`).

## Goals / Non-Goals

**Goals:**
- Document evidence source provenance for multiple audiences:
  auditors tracing compliance findings, CI pipeline authors
  extracting coordinates, SREs detecting drift via digests
- Provide a narrative walkthrough using a real Ampel example
- Cover the `mapping-references` to `evidence.source.reference-id`
  cross-reference relationship
- Include content in the RPM man page (`docs/man/complyctl.md`)

**Non-Goals:**
- Documenting OSCAL BackMatter conversion of mapping references
  (tracked separately in go-gemara#127)
- Documenting provider-side implementation details (covered in
  complytime-providers documentation and man pages)
- Modifying any source code

## Decisions

### D1: Three-file documentation structure

**Decision**: Create `docs/EVIDENCE.md` as the detailed reference,
add a brief subsection to `docs/QUICK_START.md`, and add an OUTPUT
section to `docs/man/complyctl.md`.

**Rationale**: QUICK_START.md is 387 lines and focused on "get
running fast." The full evidence walkthrough (~120 lines) would
nearly double Step 7 and dilute the quick-start purpose. A dedicated
file keeps QUICK_START concise while providing depth where needed.
The man page needs its own section because it ships independently
via RPM and cannot rely on cross-references to markdown files.

**Alternatives**: (A) Everything in QUICK_START.md -- too dense for
a getting-started guide. (B) Only EVIDENCE.md with no QUICK_START
update -- users following the quick start would miss the feature
entirely.

### D2: Auditor-narrative structure with multi-audience callouts

**Decision**: EVIDENCE.md follows an auditor tracing a finding
through five steps (control evaluation, assessment evidence,
source provenance, integrity/host identification, mapping
references). Each step includes callouts for other audiences
(CI pipelines, SREs) where relevant.

**Rationale**: The auditor trail is the most complete path through
the data -- it touches every field. CI and SRE use cases are
subsets of this path (CI cares about coordinates, SREs care about
digests and remarks). Starting with the complete trail and calling
out shortcuts for other audiences avoids duplication while serving
everyone.

### D3: Real Ampel example, one control

**Decision**: Use the real Ampel branch-protection EvaluationLog
from complytime-providers PR #192 as the primary example, trimmed
to the `force-push-restriction` control with two evidence entries
(ampel attestation + snappy observation).

**Rationale**: Real output is more useful than illustrative
examples because it matches what users of `complytime-demos`
actually see. One control keeps the example focused while still
showing two evidence entries with different attestation types.
The full EvaluationLog structure (metadata + mapping-references +
evaluations) is shown once at the top, then referenced in
step-by-step walkthrough sections.

### D4: Markdown rendering note with field visibility

**Decision**: Document that `--format pretty` shows only
`reference-id` and `coordinate` (as `source: <ref> @ <coord>`),
while all five fields are available in EvaluationLog YAML/JSON.

**Rationale**: Users may wonder why the Markdown report shows
less than the YAML. This is a deliberate design choice in
`formatEvidenceMeta()` to avoid rendering density. Documenting
the field visibility difference prevents confusion.

### D5: QUICK_START.md insertion point

**Decision**: Insert the "Understanding Scan Output" subsection
after line 364 (`Output written to ./.complytime/scan/.`) and
before the exit codes callout block (line 366).

**Rationale**: This is the natural reading point -- the user just
learned where output goes and will next learn about exit codes.
Evidence interpretation belongs between "here's where it is" and
"here's what exit codes mean."

### D6: Man page OUTPUT section placement

**Decision**: Add a new OUTPUT section between EXIT CODES and
FILES in `docs/man/complyctl.md`.

**Rationale**: Man pages conventionally describe what the tool
produces. OUTPUT is not a standard man(7) section but is a common
project-specific addition. Placing it between EXIT CODES and FILES
follows the natural reading flow: what the exit code means, what
the output contains, where files live. The FILES section already
lists `.complytime/scan/` but doesn't describe what's in the
reports. The OUTPUT section fills this gap with evidence field
descriptions and report filename patterns.

## Risks / Trade-offs

- **[Example drift]** The real Ampel example may diverge from
  future provider output as the implementation evolves.
  Mitigation: use the committed output from PR #192 as the
  baseline; the example shows structure, not exact field values.
  Cross-repo integration tests will catch behavioral drift.

- **[Provider availability]** Documentation describes fields that
  providers may not yet populate. Mitigation: include a "Provider
  Support" note stating that evidence source requires provider
  support and that the `source:` block is omitted when absent.
  Keep provider version references generic.

- **[Man page length]** Adding ~20 lines to the man page increases
  its size. Mitigation: the OUTPUT section is concise and
  self-contained. The man page is already 495 lines; this is a
  modest addition that fills a documented gap.
