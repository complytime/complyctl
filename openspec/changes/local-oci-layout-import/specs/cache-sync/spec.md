# Spec Delta

## ADDED Requirements

### Requirement: Local policy source
The cache sync pipeline SHALL accept a `LocalPolicySource` implementation of
`PolicySource` that resolves artifact version and copies content from a local
OCI Image Layout directory rather than a remote registry.

#### Scenario: Version resolved from layout digest
- **WHEN** `LocalPolicySource.DefinitionVersion` is called for a digest-pinned
  reference
- **THEN** it resolves that manifest digest from the local layout without
  contacting any network endpoint

#### Scenario: Content copied from layout
- **WHEN** `LocalPolicySource.CopyPolicy` is called
- **THEN** the selected manifest and its full content DAG are copied into the
  destination OCI store using the existing ORAS copy mechanism

#### Scenario: Missing digest returns error
- **WHEN** the requested digest is not present in the local layout
- **THEN** `DefinitionVersion` returns an error and no copy is attempted

#### Scenario: Tag-only reference rejected
- **WHEN** `LocalPolicySource.DefinitionVersion` receives a tag-only reference
- **THEN** it returns an error before reading or copying an artifact

### Requirement: Local complypack source
The cache sync pipeline SHALL accept a `LocalComplypackSource` implementation of
`ComplypackSource` that resolves and copies complypack artifacts from a local
OCI Image Layout directory.

#### Scenario: Digest-pinned complypack resolved from layout
- **WHEN** `LocalComplypackSource.DefinitionVersion` receives a digest-pinned
  reference present in the local layout
- **THEN** it resolves that manifest without contacting any network endpoint

#### Scenario: Tag-only complypack reference rejected
- **WHEN** `LocalComplypackSource.DefinitionVersion` receives a tag-only
  reference
- **THEN** it returns an error before reading or copying an artifact

#### Scenario: Artifact type validated
- **WHEN** `LocalComplypackSource` resolves a manifest from the layout
- **THEN** it verifies the OCI artifact media type matches the expected
  complypack media type before accepting the artifact

#### Scenario: Invalid media type rejected
- **WHEN** the resolved manifest has an unexpected media type
- **THEN** `LocalComplypackSource` returns an error and no content is copied

#### Scenario: Content copied and unpacked normally
- **WHEN** a valid complypack manifest is resolved from the local layout
- **THEN** the existing `ComplypackSync` unpack and atomic store pipeline runs
  without modification
