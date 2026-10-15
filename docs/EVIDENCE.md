# Evidence Source Provenance

When `complyctl scan` evaluates a target, each compliance finding
includes evidence -- the raw data a provider collected to support
its assessment. Evidence source provenance tells auditors, CI
pipeline authors, and SREs exactly *where* that evidence came from:
which API endpoint was queried, which configuration file was read,
and what cryptographic digest anchored the observation at collection
time.

This document walks through a real EvaluationLog to show how
evidence source provenance works, how to interpret it in each
report format, and how to use it in your own workflows.

## Anatomy of an EvaluationLog

The following EvaluationLog excerpt is from an Ampel provider scan
evaluating branch protection settings on the
`complytime/complytime-demos` repository. It shows one control
(`force-push-restriction`) with two evidence entries -- an Ampel
policy attestation and a Snappy data-collection observation, both
stored as in-toto attestations.

This is real output from `complyctl scan`, trimmed to one control
for readability. The full EvaluationLog contained five controls
(force-push-restriction, approval-requirements,
admin-bypass-prevention, code-owner-enforcement,
pull-request-enforcement), all with the same evidence structure.

```yaml
metadata:
  id: complytime/policies-ampel-branch-protection
  type: EvaluationLog
  gemara-version: v1.5.0
  description: Compliance scan evaluation log
  author:
    id: complytime
    name: complytime
    type: Software
    uri: https://github.com/complytime/complyctl
  mapping-references:
  - id: repo-branch-protection
    title: Branch Protection Controls
    version: 1.0.0
    description: Control catalog for branch protection rules
    url: file://../catalogs/ampel-branch-protection-catalog.yaml
  - id: ampel-github-com-complytime-complytime-demos-main
    title: Ampel policy attestation for complytime/complytime-demos@main
    version: ""
    description: In-toto attestation from ampel policy evaluation
    url: file://.complytime/ampel/results/github-com-complytime-complytime-demos-main-branch-rules-ampel.intoto.json
  - id: snappy-github-com-complytime-complytime-demos-main
    title: Snappy observation for complytime/complytime-demos@main
    version: ""
    description: In-toto attestation from snappy data collection
    url: file://.complytime/ampel/results/github-com-complytime-complytime-demos-main-branch-rules-snappy.intoto.json
result: Passed
evaluations:
- name: force-push-restriction
  result: Passed
  message: Force pushing is disabled in protected branch.
  control:
    reference-id: complytime/policies-ampel-branch-protection
    entry-id: force-push-restriction
  assessment-logs:
  - requirement:
      reference-id: complytime/policies-ampel-branch-protection
      entry-id: block-force-push
    plan:
      reference-id: complytime/policies-ampel-branch-protection
      entry-id: block-force-push
    description: Force pushing is disabled in protected branch.
    result: Passed
    message: Force pushing is disabled in protected branch.
    applicability:
    - default
    steps:
    - complytime/complytime-demos@main
    steps-executed: 1
    start: "2026-10-09T12:40:48Z"
    confidence-level: High
    evidence:
    - id: ampel-github-com-complytime-complytime-demos-main
      type: IntotoAttestation
      collected-at: "2026-10-09T12:40:48Z"
      source:
        reference-id: ampel-github-com-complytime-complytime-demos-main
        coordinate: .complytime/ampel/results/github-com-complytime-complytime-demos-main-branch-rules-ampel.intoto.json
        digest: sha256:bfd7b62e770beb96d3ff8954a9c5524f5cf7fa7b26aca36dd04f0f36450b2504
        remarks: Collected on host "scan-runner-01"
    - id: snappy-github-com-complytime-complytime-demos-main
      type: IntotoAttestation
      collected-at: "2026-10-09T12:40:48Z"
      source:
        reference-id: snappy-github-com-complytime-complytime-demos-main
        coordinate: .complytime/ampel/results/github-com-complytime-complytime-demos-main-branch-rules-snappy.intoto.json
        digest: sha256:793d5c5ae0dc9f71b82a6064adf628382b5070d15b1665de94622092e41d806c
        remarks: Collected on host "scan-runner-01"
target:
  id: complytime-demos
  name: complytime-demos
  type: Software
```

## Tracing the Evidence Trail

Follow these five steps to trace any compliance finding back to its
origin.

### Step 1: Control Evaluation

Start at the `evaluations` array. Each entry represents one control
assessed by the provider. In the example above, the
`force-push-restriction` control has `result: Passed` with a
human-readable `message`: "Force pushing is disabled in protected
branch."

The `control` field links this evaluation to the policy definition
via `reference-id` and `entry-id`, connecting the runtime result to
the policy's static requirement.

### Step 2: Assessment Evidence

Inside each `assessment-logs` entry, the `evidence` array lists
every piece of data the provider collected for that requirement.
Each evidence entry has an `id`, `type`, `collected-at` timestamp,
and optionally a raw `payload` and `source` block.

In this example, both evidence entries are `IntotoAttestation` type
-- in-toto attestations produced by the Ampel policy evaluator and
the Snappy data collector respectively. The Ampel attestation
records the policy evaluation result, while the Snappy attestation
records the raw branch protection settings observed from the GitHub
API.

### Step 3: Source Provenance

The `source` block on each evidence entry identifies *where* the
data was collected from. Two fields establish the location:

- **`reference-id`**: Names the external system or artifact. This
  value ties the evidence to a `mapping-references` entry in the
  EvaluationLog metadata (see Step 5).
- **`coordinate`**: The precise location within that system -- an
  API path, file path, or JSON path expression.

In the first evidence entry,
`reference-id: ampel-github-com-complytime-complytime-demos-main`
and `coordinate: .complytime/ampel/results/github-com-complytime-complytime-demos-main-branch-rules-ampel.intoto.json`
together tell you: "This attestation was collected from the Ampel
results directory, specifically the in-toto attestation file for the
complytime-demos repository's main branch."

> **CI pipeline tip**: Extract the `coordinate` value to locate the
> source artifact on disk or construct direct links in your CI
> reports.

When `reference-id` points to a Gemara artifact rather than an
external system, `entry-id` replaces `coordinate` to identify a
specific entry within that artifact. The `coordinate` and `entry-id`
fields are mutually exclusive per the Gemara schema -- set one or
the other, never both.

### Step 4: Integrity and Host Identification

Two additional fields help verify evidence integrity:

- **`digest`**: A cryptographic hash (`algorithm:hex`, e.g.,
  `sha256:bfd7b62e770beb96d3ff...`) pinning the observed content at
  collection time. Compare this digest against the current state of
  the source to detect drift since the scan.
- **`remarks`**: Prose from the provider about the collection
  context -- which host performed the scan, any caveats about the
  data's freshness, or environmental conditions. In the example,
  `Collected on host "scan-runner-01"` identifies the machine that
  ran the scan.

> **SRE tip**: Use the `digest` field to build drift-detection
> pipelines. If a re-scan produces a different digest for the same
> `reference-id` and `coordinate`, the underlying configuration has
> changed since the last assessment.

### Step 5: Mapping References Cross-Reference

The `mapping-references` array in the EvaluationLog metadata
provides full context for each `reference-id` used in evidence
source blocks. Each mapping reference includes:

- **`id`**: The identifier that evidence `reference-id` fields
  point to.
- **`title`**: A human-readable name for the external system.
- **`version`**: The version of the referenced system or artifact.
- **`url`** (optional): Where the referenced artifact can be
  retrieved.

The following diagram shows how `reference-id` values in evidence
source blocks resolve to `mapping-references` entries:

```
Evidence Source                          Mapping References
+----------------------------------+    +------------------------------------------+
| evidence[0].source               |    | mapping-references[1]                    |
|   reference-id: ampel-github-    |    |   id: ampel-github-com-complytime-       |
|     com-complytime-complytime- +----->|     complytime-demos-main                |
|     demos-main                   |    |   title: Ampel policy attestation for    |
|   coordinate: .complytime/      |    |     complytime/complytime-demos@main     |
|     ampel/results/...ampel.json  |    +------------------------------------------+
|   digest: sha256:bfd7b62e...    |
+----------------------------------+    +------------------------------------------+
                                        | mapping-references[2]                    |
+----------------------------------+    |   id: snappy-github-com-complytime-      |
| evidence[1].source               |    |     complytime-demos-main                |
|   reference-id: snappy-github- +----->|   title: Snappy observation for          |
|     com-complytime-complytime-   |    |     complytime/complytime-demos@main     |
|     demos-main                   |    +------------------------------------------+
|   coordinate: .complytime/      |
|     ampel/results/...snappy.json |
|   digest: sha256:793d5c5a...    |
+----------------------------------+
```

When both a policy and a provider supply mapping references with the
same `id`, the policy definition takes precedence. complyctl logs a
collision warning and annotates the retained entry's `description`
field with the discarded provider reference.

## Field Reference

The five `source` fields on each evidence entry, with their
serialized YAML/JSON names from the Gemara `EvidenceMapping` type:

| Field | YAML/JSON Key | Purpose | Example |
|-------|--------------|---------|---------|
| Reference ID | `reference-id` | Names the external system or Gemara artifact this evidence was collected from. Ties the evidence to a `mapping-references` entry. | `ampel-github-com-complytime-complytime-demos-main` |
| Coordinate | `coordinate` | Precise location within the referenced system (API path, file path, JSON path). Mutually exclusive with `entry-id`. | `.complytime/ampel/results/...ampel.intoto.json` |
| Entry ID | `entry-id` | Specific entry within a referenced Gemara artifact. Mutually exclusive with `coordinate`. | `block-force-push` |
| Digest | `digest` | Cryptographic hash of the observed content at collection time. Format: `algorithm:hex`. | `sha256:bfd7b62e770beb96d3ff...` |
| Remarks | `remarks` | Provider prose about collection context, host identification, or caveats. | `Collected on host "scan-runner-01"` |

> **Note**: `coordinate` and `entry-id` are mutually exclusive per
> the Gemara schema. Use `coordinate` for locations within external
> systems (APIs, files). Use `entry-id` for entries within Gemara
> artifacts.

Field names are verified against the `gemara.EvidenceMapping` struct
YAML tags and the `EvidenceSource` struct in
`pkg/provider/client.go`.

## Report Formats

Evidence source provenance appears in different levels of detail
depending on the output format.

### EvaluationLog (YAML and JSON)

All five `source` fields are serialized when present. Fields with
empty values are omitted via `omitempty` in YAML. In JSON output
(`--log-format json`), evidence entries without source provenance
emit `"source": {"reference-id": ""}` because Go's `encoding/json`
does not omit zero-value struct fields and `reference-id` lacks an
`omitempty` tag. YAML output omits the `source:` key entirely in
this case (`goccy/go-yaml` treats zero-value structs as empty for
`omitempty`).

### Markdown (`--format pretty`)

The Markdown report renders only `reference-id` and `coordinate`
from the source, formatted as:

```
source: ampel-github-com-complytime-complytime-demos-main @ .complytime/ampel/results/github-com-complytime-complytime-demos-main-branch-rules-ampel.intoto.json
```

When only `reference-id` is present (no `coordinate`), the format
is:

```
source: ampel-github-com-complytime-complytime-demos-main
```

This rendering is produced by `formatEvidenceMeta()` in
`internal/output/markdown.go`. The remaining three fields (`entry-id`,
`digest`, `remarks`) are available in the full EvaluationLog output
for users who need the complete provenance chain.

### OSCAL

Evidence source fields are not currently included in complyctl's
OSCAL assessment-results output. The `go-gemara` library's OSCAL
converter does map evidence source to OSCAL properties, but
complyctl's `ToOSCAL()` function builds findings directly from
assessment logs without using this converter. This is tracked in
[go-gemara#127](https://github.com/gemaraproj/go-gemara/issues/127).

### SARIF

Evidence source fields are not currently included in complyctl's
SARIF output. The `go-gemara` SARIF converter does not process
evidence or source fields.

## Provider Support

Evidence source provenance requires provider support. When a
provider does not populate the `source` field on evidence entries,
the `source:` block is omitted from the EvaluationLog YAML output
and the `source:` line does not appear in Markdown reports.

Not all providers support evidence source provenance. Check your
provider's documentation for details on which evidence types include
source information and which fields are populated.

## See Also

- [Quick Start](QUICK_START.md) -- getting started with complyctl
- [Man page](man/complyctl.md) -- full CLI reference including
  OUTPUT and FILES sections
