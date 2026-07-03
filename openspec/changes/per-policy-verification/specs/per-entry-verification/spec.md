## ADDED Requirements

### FR-001: Per-entry verification configuration

The system MUST support an optional `verification:` field on each
policy and complypack entry in `complytime.yaml`. When present, the
entry-level verification config MUST be used instead of the
workspace-level `verification:` block. Each verification block
MUST be a complete, standalone configuration with no field
inheritance from the workspace level.

#### Scenario: Entry with its own verification config

- **GIVEN** a workspace with multiple policy entries configured
- **WHEN** a policy entry has a `verification:` block with
  `key: "/path/to/vendor.pub"`
- **AND** the workspace-level `verification:` is configured with
  keyless (issuer + identity)
- **THEN** the policy MUST be verified using the entry-level
  public key, not the workspace-level keyless config

#### Scenario: Entry without verification config inherits workspace

- **GIVEN** a workspace with workspace-level `verification:`
  configured
- **WHEN** a policy entry has no `verification:` field
- **THEN** the policy MUST be verified using the workspace-level
  verification config

#### Scenario: No verification configured at any level

- **GIVEN** a workspace with no workspace-level `verification:`
- **WHEN** a policy entry has no `verification:` field
- **THEN** the policy MUST be fetched without verification

### FR-002: Per-entry verification skip

The system MUST support an optional `skip_verify: true` field on
each policy and complypack entry in `complytime.yaml`. When set,
verification MUST be skipped for that entry regardless of the
workspace-level verification config.

#### Scenario: Entry with skip_verify and workspace verification

- **GIVEN** a workspace with workspace-level `verification:`
  configured
- **WHEN** a policy entry has `skip_verify: true`
- **THEN** the policy MUST be fetched without verification
- **AND** the cached policy state MUST record `verified: false`

#### Scenario: skip_verify with entry-level verification is invalid

- **GIVEN** a workspace configuration being validated
- **WHEN** a policy entry has both `skip_verify: true` and a
  `verification:` block
- **THEN** config validation MUST return an error indicating the
  fields are mutually exclusive

### FR-003: CLI flag overrides all entry-level config

The `--skip-verify` CLI flag MUST override all per-entry and
workspace-level verification configuration. When set, no entries
MUST be verified.

#### Scenario: skip-verify flag with per-entry verification

- **GIVEN** one or more entries have entry-level `verification:`
  configs
- **WHEN** the user runs `complyctl get --skip-verify`
- **THEN** all entries MUST be fetched without verification
- **AND** the existing `--skip-verify` warning MUST be printed to
  stderr (this is the current warning behavior; no new warning
  text is required)

### FR-004: Per-entry verification config validation

Each entry-level `verification:` block MUST be validated with the
same rules as the workspace-level block: keyed and keyless modes
are mutually exclusive, `issuer` requires `identity`, and
`identity` requires `issuer`.

#### Scenario: Invalid entry-level verification config

- **GIVEN** a workspace configuration being validated
- **WHEN** a policy entry has `verification:` with both `key:` and
  `issuer:` set
- **THEN** config validation MUST return an error identifying the
  policy entry and the validation failure

#### Scenario: Valid mixed verification configs

- **GIVEN** a workspace with policies from multiple publishers
- **WHEN** one policy entry has `verification:` with `key:` (keyed)
- **AND** another policy entry has no `verification:` field
- **AND** workspace-level `verification:` has `issuer:` + `identity:`
  (keyless)
- **THEN** config validation MUST pass

### FR-005: Verifier caching

The system MUST construct at most one verifier per distinct
verification configuration within a single `complyctl get`
invocation. Entries that resolve to the same verification config
MUST share the same verifier instance. Caching is tested via unit
tests using a mock verifier constructor injected into
`resolveVerifier()`, verifying that the constructor is called at
most once per distinct `VerificationConfig` value.

#### Scenario: Multiple entries with same workspace-level config

- **GIVEN** three policy entries with no entry-level `verification:`
- **AND** workspace-level `verification:` is configured (keyless)
- **WHEN** `complyctl get` resolves verifiers for all entries
- **THEN** the verifier constructor MUST be called once
- **AND** all three entries MUST receive the same verifier instance

#### Scenario: Entry config differs from workspace config

- **GIVEN** two policy entries with `verification: { key: "a.pub" }`
- **AND** a third policy entry with no entry-level `verification:`
- **AND** workspace-level `verification:` configured as keyless
- **WHEN** `complyctl get` resolves verifiers for all entries
- **THEN** the verifier constructor MUST be called twice: once for
  the keyed config (shared by the first two entries) and once for
  the keyless workspace config

### FR-006: Error collection across entries

The system MUST attempt to sync all policy and complypack entries
even when individual entries fail. All errors MUST be collected
and reported together after all entries have been attempted. The
error message MUST identify each failed entry by its effective
policy ID. Individual errors MUST be unwrappable via
`errors.Is`/`errors.As`.

#### Scenario: One policy fails verification, others succeed

- **GIVEN** three policy entries (A, B, C) configured
- **WHEN** policy A fails verification
- **AND** policy B succeeds verification
- **AND** policy C succeeds verification
- **THEN** policies B and C MUST be cached successfully
- **AND** the command MUST report the verification failure for
  policy A including its effective policy ID
- **AND** the command MUST exit with a non-zero exit code

#### Scenario: Multiple policies fail

- **GIVEN** three policy entries (A, B, C) configured
- **WHEN** policy A fails verification
- **AND** policy B fails with a network error
- **AND** policy C succeeds
- **THEN** policy C MUST be cached successfully
- **AND** the command MUST report both errors (A and B) with their
  effective policy IDs
- **AND** the command MUST exit with a non-zero exit code

#### Scenario: All policies fail

- **GIVEN** three policy entries (A, B, C) configured
- **WHEN** all three entries fail
- **THEN** all three errors MUST be collected and reported
- **AND** no artifacts MUST be cached
- **AND** the command MUST exit with a non-zero exit code

#### Scenario: All policies succeed

- **GIVEN** all policy and complypack entries configured
- **WHEN** all entries sync successfully
- **THEN** the command MUST exit with zero exit code
- **AND** no errors MUST be reported
