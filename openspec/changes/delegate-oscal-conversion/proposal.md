# Proposal

## Why

The local OSCAL converter duplicates conversion logic that now lives in
go-gemara. It also omits assessment evidence from OSCAL reports, leaving
consumers without the provenance needed to evaluate findings.

Delegating the model conversion to go-gemara aligns OSCAL output with the
existing SARIF delegation pattern and makes evidence conversion available to
complyctl.

## What Changes

- Replace complyctl's local EvaluationLog-to-OSCAL model conversion with
  go-gemara's `EvaluationLogToOSCALAssessmentResults` converter.
- Preserve complyctl's report serialization, output directory creation, file
  permissions, report filenames, and assessment-plan reference.
- Emit assessment evidence as OSCAL `relevant-evidence` on observations and
  link findings to those observations.
- Adopt go-gemara's control-level finding and assessment-level observation
  structure, including its documented result status mappings.
- Remove obsolete local OSCAL conversion helpers and the related FIXME.

## Capabilities

### Modified Capabilities

- `oscal-assessment-results`: Modify the existing OSCAL export required by
  `specs/001-gemara-native-workflow/spec.md` FR-014 to delegate model
  conversion to go-gemara while preserving report delivery and adding evidence
  and finding-to-observation links.

## Impact

- Affects `internal/output/oscal.go` and `internal/output/oscal_test.go`.
- Retains the public `output.ToOSCAL` function signature and report filename
  behavior.
- Updates `github.com/gemaraproj/go-gemara` to the revision that emits
  OSCAL-token-valid result status reasons.
- Changes the OSCAL document structure from requirement-level findings to
  control-level findings with associated assessment observations.
