# Tasks

## 1. Create docs/EVIDENCE.md

- [x] 1.1 Create `docs/EVIDENCE.md` with overview section explaining evidence source provenance and its value for auditors, CI pipelines, and SREs. Verify: file exists; opening section names all three target audiences (auditors, CI pipeline authors, SREs) and states the document's purpose within the first paragraph.
- [x] 1.2 Add "Anatomy of an EvaluationLog" section with the real Ampel branch-protection example (one control: `force-push-restriction`, two evidence entries). Use the output from complytime-providers PR #192. Sanitize any org-specific identifiers (repository URLs, user handles, hostnames) — use generic identifiers where the real values are not essential to the walkthrough. Verify: extract the fenced YAML block to a temporary file and validate with `yq '.' /tmp/evidence-example.yaml` or equivalent; YAML parses without syntax errors.
- [x] 1.3 Add "Tracing the Evidence Trail" section with five narrative steps: (1) control evaluation, (2) assessment evidence, (3) source provenance with coordinate/reference-id, (4) integrity and host identification with digest/remarks, (5) mapping-references cross-reference with ASCII diagram showing reference-id resolution. Verify: each step references concrete fields from the example in 1.2; ASCII diagram shows at least one `reference-id` value resolving to its corresponding `mapping-references` entry.
- [x] 1.4 Add "Field Reference" table covering the five EvidenceSource fields (reference-id, coordinate, entry-id, digest, remarks) with purpose and example values from the real output. Include a note that `coordinate` and `entry-id` are mutually exclusive per the Gemara schema. Verify: all five fields listed with non-empty purpose and example columns; cross-check field names against `gemara.EvidenceMapping` struct YAML tags and `EvidenceSource` struct in `pkg/provider/client.go` to confirm documented names match serialized output.
- [x] 1.5 Add "Report Formats" section documenting field visibility per format: EvaluationLog YAML/JSON (all five fields), Markdown (reference-id + coordinate only via `formatEvidenceMeta()`), OSCAL (evidence source not currently included — tracked in go-gemara#127), SARIF (evidence source not currently included). Document both Markdown rendering variants: `source: <ref> @ <coord>` (both fields present) and `source: <ref>` (coordinate absent). Note that JSON emits `"source": {}` for evidence without source provenance while YAML omits the `source:` key entirely. Verify: Markdown rendering examples match `formatEvidenceMeta()` branching logic at `internal/output/markdown.go:282-286`; OSCAL/SARIF statements are accurate against current formatters.
- [x] 1.6 Add "Provider Support" section noting that evidence source requires provider support, and the `source:` block is omitted when absent. Keep provider version references generic. Verify: section does not reference specific provider versions.

## 2. Update docs/QUICK_START.md

- [x] 2.1 Add "Understanding Scan Output" subsection in Step 7 after the line reading `Output written to ./.complytime/scan/.` and before the exit codes callout. Include one annotated evidence example, Markdown rendering line, provider support note, and link to `EVIDENCE.md`. Verify: subsection appears between scan output path and exit codes; link to EVIDENCE.md resolves correctly (`[Evidence Source Provenance](EVIDENCE.md)`). Line numbers are approximate (currently ~364/366) — use the quoted content as the authoritative anchor.

## 3. Update docs/man/complyctl.md

- [x] 3.1 Add OUTPUT section between the EXIT CODES and FILES sections. Include scan report directory, report filename pattern, and evidence source provenance field descriptions. Verify: OUTPUT section uses `# OUTPUT` heading consistent with adjacent sections; field descriptions use pandoc definition list syntax (`:` prefix) matching existing EXIT CODES style. Line numbers are approximate (currently EXIT CODES ends ~452, FILES starts ~454) — use the section headings as authoritative anchors.
- [x] 3.2 Add report filename pattern note to the `.complytime/scan/` FILES entry. Verify: filename example matches `BuildReportFilename()` output format in `internal/output/filename.go`. Line numbers are approximate (currently ~478-479) — use the `.complytime/scan/` entry text as the authoritative anchor. Note: if task 3.1 inserted content above, this entry will have shifted down.

## 4. Documentation gate

- [x] 4.1 Add CHANGELOG.md entry under `### Added` in `## Unreleased` for the documentation addition. Verify: entry appears under `### Added`, uses dash-prefix format, references #876, and describes the three documentation deliverables. AGENTS.md Recent Changes: not required (docs-only change, no structural/convention/build impact).
- [x] 4.2 Verify all internal links resolve: QUICK_START.md -> EVIDENCE.md, man page -> project repository EVIDENCE.md URL. Run: check that referenced files exist at the linked paths.

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.

<!-- spec-review: passed -->
<!-- code-review: passed -->
