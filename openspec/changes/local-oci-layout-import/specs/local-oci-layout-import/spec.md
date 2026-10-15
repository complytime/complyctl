# Spec Delta

## Purpose

Enables air-gapped and network-restricted users to populate the policy and
complypack cache from a local OCI Image Layout directory, using the same cache
and state machinery as online `complyctl get`.

## ADDED Requirements

### Requirement: Local OCI layout flag
`complyctl get` SHALL accept a `--from-oci-layout <path>` flag that redirects
artifact retrieval to a local OCI Image Layout directory instead of a remote
registry.

#### Scenario: Flag absent — online behavior unchanged
- **WHEN** `complyctl get` is invoked without `--from-oci-layout`
- **THEN** the command behaves identically to its current online behavior

#### Scenario: Flag present — no registry initialization
- **WHEN** `complyctl get --from-oci-layout /media/usb` is invoked
- **THEN** local sources are selected before registry-client creation or
  credential resolution, and neither factory is called for configured policies
  or complypacks

#### Scenario: Path does not exist
- **WHEN** `--from-oci-layout` points to a path that does not exist
- **THEN** the command fails before importing any artifact and reports the
  missing path

#### Scenario: Path is not a valid OCI Image Layout
- **WHEN** `--from-oci-layout` points to a directory that lacks a valid
  `oci-layout` marker or `index.json`
- **THEN** the command fails before importing any artifact and reports the
  invalid layout

### Requirement: Artifact selection by configured digest
In local mode, each configured policy and complypack `url` SHALL contain an OCI
digest. The digest SHALL select the artifact from the local layout. The registry
and repository portions of the configured URL SHALL remain cache/state identity
only and SHALL NOT select an artifact or be contacted.

#### Scenario: Configured policy digest present in layout
- **WHEN** a policy is configured with an OCI digest
- **AND** the layout contains a manifest with that exact digest
- **THEN** that manifest and its full content DAG are imported

#### Scenario: Configured complypack digest present in layout
- **WHEN** a complypack is configured with an OCI digest
- **AND** the layout contains a manifest with that exact digest
- **THEN** that manifest is imported and unpacked normally

#### Scenario: Tag-only configured URL rejected
- **WHEN** a policy or complypack URL lacks an OCI digest in local mode
- **THEN** that entry fails before cache or state changes; other configured
  entries continue

#### Scenario: Configured digest absent from layout
- **WHEN** the local layout does not contain a manifest matching the configured
  digest
- **THEN** that entry fails with an error identifying the missing digest; other
  configured entries continue

### Requirement: Content integrity validation
The digest of each imported manifest SHALL be verified against the manifest bytes
before the artifact is accepted into the local cache.

#### Scenario: Digest matches
- **WHEN** the computed digest of an imported manifest matches its descriptor
- **THEN** the artifact is accepted and cache/state are updated

#### Scenario: Digest mismatch
- **WHEN** the computed digest of an imported manifest does not match its
  descriptor
- **THEN** the import fails for that entry; the existing cache and state are
  unchanged; other configured entries continue

### Requirement: Cache and state consistency
A successful local import SHALL produce the same cache structure and `state.json`
entries as an equivalent online `complyctl get`, including digest, version, policy
metadata (title, evaluator, control count, assessment count), and evaluator ID
for complypacks.

#### Scenario: Fresh import — entry absent from state
- **WHEN** a policy or complypack is imported for the first time via
  `--from-oci-layout`
- **THEN** `state.json` records the artifact digest, version, and associated
  metadata, as it would after `complyctl get`

#### Scenario: Incremental skip — cache and state already current
- **WHEN** the local state digest matches the layout manifest digest and the
  cache directory exists
- **THEN** no copy is performed and no state update is written

#### Scenario: Replacement — digest differs from stored state
- **WHEN** the layout manifest digest differs from the stored state digest
- **THEN** the cache is updated and state is overwritten with the new digest

### Requirement: Verification fail-closed behavior
When signature verification is configured for an entry and `--skip-verify` is
not supplied, `complyctl get --from-oci-layout` SHALL refuse to import that
entry and report that offline verification is not yet supported. It SHALL make
this decision from configuration before constructing a verifier, loading a
trusted root, starting TUF, creating a registry client, or resolving registry
credentials.

#### Scenario: Verification configured — no skip
- **WHEN** an entry has `verification:` configured
- **AND** `--skip-verify` is not supplied
- **THEN** the import fails for that entry with a message directing the user to
  use `--skip-verify` until offline verification is available; its cache and
  state remain unchanged and other configured entries continue

#### Scenario: Verification rejection performs no network-capable initialization
- **WHEN** a local-mode entry has `verification:` configured and
  `--skip-verify` is not supplied
- **THEN** observable guards for verifier construction, trusted-root loading,
  TUF startup, registry-client creation, and registry credential resolution
  record zero calls for that entry
- **AND** another eligible entry continues through the local import path

#### Scenario: Verification configured — explicit skip
- **WHEN** an entry has `verification:` configured
- **AND** `--skip-verify` is supplied
- **THEN** the import proceeds and the entry is recorded as `Verified: false`

#### Scenario: No verification configured — import proceeds
- **WHEN** no `verification:` is configured for an entry
- **THEN** the import proceeds and the entry is recorded as `Verified: false`

#### Scenario: Verification is never silently disabled
- **WHEN** `--from-oci-layout` is used without `--skip-verify`
- **AND** an entry has verification configured
- **THEN** that entry does not downgrade to an unverified import silently

### Requirement: Partial failure resilience
Failure for one configured entry SHALL NOT prevent import of other entries.

#### Scenario: One policy missing from layout
- **WHEN** a workspace configures two policies and only one is present in the
  local layout
- **THEN** the present policy is imported successfully; the missing policy fails
  with an actionable error; the command exits non-zero

#### Scenario: Error output per entry
- **WHEN** one or more entries fail
- **THEN** each failure is reported individually, consistent with existing
  `complyctl get` error reporting

### Requirement: Compatibility with existing flags
`--timeout` and `--skip-verify` SHALL apply in `--from-oci-layout` mode with
their existing semantics.

#### Scenario: Timeout honored
- **WHEN** `--timeout` is set and local I/O exceeds the duration
- **THEN** the operation is cancelled and an error is reported

#### Scenario: Skip-verify suppresses verification requirement
- **WHEN** `--skip-verify` is supplied alongside `--from-oci-layout`
- **THEN** no verification is attempted regardless of configured verification

### Requirement: Policy and complypack both supported
Both configured policies and configured complypacks SHALL be importable via
`--from-oci-layout`.

#### Scenario: Policy import
- **WHEN** `--from-oci-layout` is used and the workspace has configured policies
- **THEN** each policy is sourced from the local layout through the normal policy
  sync pipeline

#### Scenario: Complypack import
- **WHEN** `--from-oci-layout` is used and the workspace has configured complypacks
- **THEN** each complypack is sourced from the local layout, unpacked, and stored
  in the evaluator/version cache as normal

#### Scenario: Mixed workspace — some missing
- **WHEN** the layout contains some but not all configured artifacts
- **THEN** present artifacts import successfully; absent ones fail individually
