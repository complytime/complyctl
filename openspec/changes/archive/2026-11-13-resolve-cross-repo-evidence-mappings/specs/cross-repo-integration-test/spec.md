# Spec Delta

## Purpose

Verify the real Ampel/OPA pipeline through its reported evidence while
allowing canonical assertions to exercise a separately selected host.

## ADDED Requirements

### Requirement: FR-001 Invocation-specific EvaluationLog selection

The harness MUST inspect the JSON EvaluationLog reported by the scan
being tested and MUST verify its expected policy and target identities.
It MUST fail for missing, malformed, ambiguous, or mismatched output
instead of accepting a stale log from another invocation.

#### Scenario: Scan reports its matching log
- **GIVEN** unrelated logs already exist in the workspace
- **WHEN** a scan reports one log for the expected policy and target
- **THEN** the harness validates that invocation's log only

#### Scenario: Reported log is absent or mismatched
- **GIVEN** a successful scan exit and a plausible older log
- **WHEN** the invocation reports no usable matching JSON log
- **THEN** the harness fails without selecting the older log

### Requirement: FR-002 Mapping-driven attestation selection

The harness MUST locate both Ampel and Snappy attestations through
evidence attached to the expected assessment in the selected log.
It MUST support legacy and retained layouts through those mappings,
without inferring evidence from directory contents or naming patterns.

#### Scenario: Returned mappings use either supported layout
- **GIVEN** a log cites both attestations for the expected assessment
- **WHEN** their mappings select legacy or per-evaluation files
- **THEN** the harness selects the cited files in either layout

#### Scenario: Unmapped decoy files exist
- **GIVEN** expected-looking attestations and metadata exist nearby
- **WHEN** the expected assessment lacks one attestation mapping
- **THEN** the harness fails rather than using a nearby file

#### Scenario: Evidence belongs to an unrelated assessment
- **GIVEN** another assessment contains both valid attestations
- **WHEN** the expected assessment has no usable evidence
- **THEN** the harness fails the expected assessment's evidence check

### Requirement: FR-003 Workspace-bound evidence integrity

For each selected attestation, the harness MUST resolve its coordinate
against the selected workspace, require a nonempty regular file within
that workspace, and verify its declared SHA-256 digest. Missing or
unsupported digests, unreadable files, and paths escaping the workspace
MUST fail without following an alternative evidence path.

#### Scenario: Valid relative or absolute coordinate
- **GIVEN** a coordinate identifies a file inside the workspace
- **WHEN** the file's bytes match its declared SHA-256 digest
- **THEN** the integrity check succeeds

#### Scenario: Evidence bytes or digest are invalid
- **GIVEN** a cited file is missing, empty, unreadable, or changed
- **WHEN** the harness verifies the recorded digest
- **THEN** it reports an evidence integrity failure

#### Scenario: Coordinate escapes the workspace
- **GIVEN** traversal or a symlink resolves outside the workspace
- **WHEN** the harness resolves the evidence coordinate
- **THEN** it rejects the coordinate before reading external bytes

### Requirement: FR-004 Reference and locator agreement

Every selected evidence source MUST resolve its nonempty reference ID
to exactly one metadata mapping-reference. The reference MUST supply an
absolute local file URL identifying the same file as the coordinate.
Missing or duplicate IDs, invalid URLs, and locator disagreement MUST
fail even when both files exist and their contents are identical.

#### Scenario: Encoded file URL agrees with coordinate
- **GIVEN** a workspace path contains spaces, percent signs, or Unicode
- **WHEN** a correctly escaped local file URL identifies the cited file
- **THEN** the harness accepts the locator agreement

#### Scenario: Reference or locator is inconsistent
- **GIVEN** a reference is absent, duplicated, remote, or malformed
- **WHEN** the harness resolves the selected evidence source
- **THEN** it fails without substituting another reference

#### Scenario: URL identifies a different file with equal bytes
- **GIVEN** two distinct files have the same digest
- **WHEN** the coordinate and file URL identify different files
- **THEN** the harness rejects the mismatched locator

### Requirement: FR-005 Historical evidence and instance identity

Repeat-evaluation regression coverage MUST preserve the first log and
revalidate its selected files after a later evaluation. Distinct
artifact instances MUST retain distinct evidence IDs, reference IDs,
and coordinates even when bytes match. Repeated use of one instance
across assessments MUST NOT count as multiple artifact instances.

#### Scenario: Later evaluation retains earlier evidence
- **GIVEN** the first log and its selected digests are preserved
- **WHEN** a later evaluation publishes independent artifacts
- **THEN** both logs' references remain valid and earlier bytes match

#### Scenario: Later evaluation overwrites an earlier artifact
- **GIVEN** a preserved first log cites an artifact
- **WHEN** a later evaluation replaces or removes its referenced bytes
- **THEN** historical evidence validation fails

#### Scenario: Equal-content instances remain distinct
- **GIVEN** two evaluations create separate equal-content attestations
- **WHEN** their evidence and mapping-references are inspected
- **THEN** both instances remain independently addressable

#### Scenario: Instance identities collapse
- **GIVEN** a fixture expects two separate artifact instances
- **WHEN** deduplication drops one instance or reuses its identities
- **THEN** the regression check fails despite matching content digests

### Requirement: FR-006 Existing pipeline assertions remain intact

The harness MUST preserve Get, Generate, generation-index validation,
bad-policy rejection, and OPA failing/passing checks. It MUST verify
the `block-force-push` requirement in the Ampel attestation selected
through the log, not an unrelated file. Supporting metadata MUST NOT
be accepted as an additional attestation.

#### Scenario: Selected Ampel attestation lacks required content
- **GIVEN** both selected attestations pass integrity checks
- **WHEN** the selected Ampel result lacks `block-force-push`
- **THEN** the requirement-content assertion fails

#### Scenario: Supporting metadata accompanies attestations
- **GIVEN** retained storage contains supporting metadata
- **WHEN** the pipeline validates its mapped attestations
- **THEN** metadata does not become implicit evidence
- **AND** all existing non-evidence pipeline assertions still execute

### Requirement: FR-007 Explicit host binary selection

The harness MUST accept an optional absolute `COMPLYCTL_BIN_DIR` that
selects both complyctl and mock-registry binaries. Without it, the
harness MUST retain its own checkout's `bin/` default. Assertions and
fixtures MUST remain owned by the harness checkout. An invalid explicit
selection MUST fail without building or falling back to other binaries.

#### Scenario: Harness exercises a separately built released host
- **GIVEN** harness and released binaries occupy separate checkouts
- **WHEN** the released binary directory is selected explicitly
- **THEN** both invoked host binaries come from that directory
- **AND** the harness uses its own assertions and fixtures

#### Scenario: Explicit host selection is incomplete
- **GIVEN** the selected directory lacks either executable
- **WHEN** the harness prepares the integration run
- **THEN** it fails rather than invoking its default binaries

#### Scenario: Default invocation remains available
- **WHEN** the harness runs without an explicit host directory
- **THEN** it selects both binaries from its own checkout's `bin/`

### Requirement: FR-008 EvaluationLog schema coverage

Export and schema validation MUST include the actual JSON and YAML
EvaluationLogs produced by the harness. Validation MUST retain the
configured Gemara schema and fail for invalid logs or no matching logs.
JSON-only output MUST NOT silently bypass Ampel schema validation.

#### Scenario: Ampel produces JSON while other scans produce YAML
- **WHEN** the harness exports the mixed-format scan output
- **THEN** every exported EvaluationLog is schema-validated

#### Scenario: Output is JSON-only or absent
- **WHEN** validation receives only JSON logs or no EvaluationLogs
- **THEN** it validates the JSON logs or fails for absent output

#### Scenario: Either serialization violates the schema
- **WHEN** an exported JSON or YAML EvaluationLog is invalid
- **THEN** schema validation fails without changing the schema pin
