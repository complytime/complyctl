# Design

## Context

`output.ToOSCAL` currently builds the OSCAL assessment-results model locally,
while `output.ToSARIF` delegates model conversion to go-gemara. The pinned
go-gemara version already converts evaluation logs to OSCAL assessment results
and maps assessment evidence to observation `relevant-evidence` entries.

The local converter produces a finding for every assessment log. The upstream
converter produces a finding for each control evaluation and observations for
its assessment logs. It links each finding to its observations and owns result
status mapping. See `proposal.md` and the `oscal-assessment-results` delta
specification for the required behavior.

## Goals / Non-Goals

**Goals:**

- Make go-gemara the single owner of EvaluationLog-to-OSCAL model conversion.
- Preserve complyctl's report delivery behavior around the converted model.
- Cover evidence, finding-to-observation links, and upstream result semantics
  with output-focused regression tests and OSCAL schema validation.

**Non-Goals:**

- Upgrade go-gemara or alter its conversion behavior.
- Change the public `output.ToOSCAL` function signature or CLI flags.
- Add OSCAL catalog or profile generation.

## Decisions

### Delegate model conversion and retain report delivery

`ToOSCAL` will call
`gemaraconv.EvaluationLogToOSCALAssessmentResults(*log, ...)`, wrap the result
in `oscalTypes.OscalModels`, marshal it as indented JSON, and retain existing
directory creation, `0600` report writes, and `BuildReportFilename` usage.

The conversion function returns a model rather than bytes, unlike
`gemaraconv.ToSARIF`; retaining serialization in complyctl is therefore
necessary. Reimplementing evidence conversion locally was rejected because it
would retain duplicate logic and allow behavior to drift from go-gemara.

### Preserve the assessment-plan reference through an upstream option

`ToOSCAL` will pass the existing policy-specific assessment-plan URI using
`gemaraconv.WithImportApHref`. This retains the current externally visible
reference while allowing go-gemara to construct the assessment-results model.

Accepting go-gemara's default `#` reference was rejected because it changes a
stable report field without advancing the conversion delegation goal.

### Adopt upstream document and result semantics

The implementation will remove the local result mapping, UUID helper, and
tailoring-specific remarks. Tests will assert the pinned converter's observable
result status and reason values rather than encode a second mapping table.

The prior requirement-level finding structure changes to a control-level
finding with linked observations. This is an intentional compatibility change
required to represent evidence correctly and to avoid parallel conversion
models.

### Test the serialized report contract

Tests will construct evaluation logs with evidence and source metadata, invoke
`ToOSCAL`, unmarshal the written JSON, and assert evidence entries and related
observation links. Existing tests for validation, filenames, and output
writing remain, with mapping-specific assertions adjusted to upstream output.

Direct tests of go-gemara's internal converter are out of scope because it is
an upstream dependency; complyctl tests cover the wrapper and serialized
contract instead.

## Risks / Trade-offs

- [Upstream output differs from legacy output] -> Record the decision in the
  spec, update assertions to the upstream contract, and retain schema checks.
- [A future dependency upgrade changes OSCAL output] -> Keep focused wrapper
  tests for evidence, links, report delivery, and expected result semantics.
- [Nil or incomplete evaluation data changes behavior] -> Retain and run
  package tests against representative logs before merging.

## Migration Plan

1. Replace the local model construction with the upstream conversion call.
2. Update OSCAL tests to validate the new serialized structure and evidence.
3. Run formatting, targeted tests, and the CI-equivalent unit test command.
4. Roll back by restoring the prior local converter if a downstream consumer
   cannot accept the documented output structure change.
