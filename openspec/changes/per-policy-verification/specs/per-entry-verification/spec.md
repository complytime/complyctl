## ADDED Requirements

### Requirement: Per-entry verification configuration

The system SHALL support an optional `verification:` field on each
policy and complypack entry in `complytime.yaml`. When present, the
entry-level verification config SHALL be used instead of the
workspace-level `verification:` block. Each verification block
SHALL be a complete, standalone configuration with no field
inheritance from the workspace level.

#### Scenario: Entry with its own verification config

- **WHEN** a policy entry has a `verification:` block with
  `key: "/path/to/vendor.pub"`
- **AND** the workspace-level `verification:` is configured with
  keyless (issuer + identity)
- **THEN** the policy SHALL be verified using the entry-level
  public key, not the workspace-level keyless config

#### Scenario: Entry without verification config inherits workspace

- **WHEN** a policy entry has no `verification:` field
- **AND** the workspace-level `verification:` is configured
- **THEN** the policy SHALL be verified using the workspace-level
  verification config

#### Scenario: No verification configured at any level

- **WHEN** a policy entry has no `verification:` field
- **AND** no workspace-level `verification:` is configured
- **THEN** the policy SHALL be fetched without verification

### Requirement: Per-entry verification skip

The system SHALL support an optional `skip_verify: true` field on
each policy and complypack entry in `complytime.yaml`. When set,
verification SHALL be skipped for that entry regardless of the
workspace-level verification config.

#### Scenario: Entry with skip_verify and workspace verification

- **WHEN** a policy entry has `skip_verify: true`
- **AND** the workspace-level `verification:` is configured
- **THEN** the policy SHALL be fetched without verification
- **AND** the cached policy state SHALL record `verified: false`

#### Scenario: skip_verify with entry-level verification is invalid

- **WHEN** a policy entry has both `skip_verify: true` and a
  `verification:` block
- **THEN** config validation SHALL return an error indicating the
  fields are mutually exclusive

### Requirement: CLI flag overrides all entry-level config

The `--skip-verify` CLI flag SHALL override all per-entry and
workspace-level verification configuration. When set, no entries
SHALL be verified.

#### Scenario: skip-verify flag with per-entry verification

- **WHEN** the user runs `complyctl get --skip-verify`
- **AND** one or more entries have entry-level `verification:`
  configs
- **THEN** all entries SHALL be fetched without verification
- **AND** a warning SHALL be printed to stderr

### Requirement: Per-entry verification config validation

Each entry-level `verification:` block SHALL be validated with the
same rules as the workspace-level block: keyed and keyless modes
are mutually exclusive, `issuer` requires `identity`, and
`identity` requires `issuer`.

#### Scenario: Invalid entry-level verification config

- **WHEN** a policy entry has `verification:` with both `key:` and
  `issuer:` set
- **THEN** config validation SHALL return an error identifying the
  policy entry and the validation failure

#### Scenario: Valid mixed verification configs

- **WHEN** one policy entry has `verification:` with `key:` (keyed)
- **AND** another policy entry has no `verification:` field
- **AND** workspace-level `verification:` has `issuer:` + `identity:`
  (keyless)
- **THEN** config validation SHALL pass

### Requirement: Verifier caching

The system SHALL construct at most one verifier per distinct
verification configuration within a single `complyctl get`
invocation. Entries that resolve to the same verification config
SHALL share the same verifier instance.

#### Scenario: Multiple entries with same workspace-level config

- **WHEN** three policy entries have no entry-level `verification:`
- **AND** workspace-level `verification:` is configured (keyless)
- **THEN** the system SHALL construct one keyless verifier and
  reuse it for all three entries

#### Scenario: Entry config differs from workspace config

- **WHEN** one policy entry has `verification: { key: "a.pub" }`
- **AND** another policy entry has `verification: { key: "a.pub" }`
- **AND** a third policy entry inherits workspace-level keyless
- **THEN** the system SHALL construct two verifiers: one keyed
  (shared by the first two entries) and one keyless

### Requirement: Error collection across entries

The system SHALL attempt to sync all policy and complypack entries
even when individual entries fail. All errors SHALL be collected
and reported together after all entries have been attempted.

#### Scenario: One policy fails verification, others succeed

- **WHEN** policy A fails verification
- **AND** policy B succeeds verification
- **AND** policy C succeeds verification
- **THEN** policies B and C SHALL be cached successfully
- **AND** the command SHALL report the verification failure for
  policy A
- **AND** the command SHALL exit with a non-zero exit code

#### Scenario: Multiple policies fail

- **WHEN** policy A fails verification
- **AND** policy B fails with a network error
- **AND** policy C succeeds
- **THEN** policy C SHALL be cached successfully
- **AND** the command SHALL report both errors (A and B)
- **AND** the command SHALL exit with a non-zero exit code

#### Scenario: All policies succeed

- **WHEN** all policy and complypack entries sync successfully
- **THEN** the command SHALL exit with zero exit code
- **AND** no errors SHALL be reported
