## ADDED Requirements

### Requirement: Custom trusted root configuration
The system SHALL accept a `trusted_root` field in `VerificationConfig` that specifies a filesystem path to a `trusted_root.json` file for keyless signature verification against private Sigstore instances. The field SHALL be accepted at both workspace-level and per-entry verification configuration.

#### Scenario: Workspace-level trusted root in YAML
- **WHEN** `complytime.yaml` contains `verification.trusted_root: /path/to/trusted_root.json`
- **THEN** the `TrustedRoot` field of the parsed `VerificationConfig` SHALL equal `/path/to/trusted_root.json`

#### Scenario: Per-entry trusted root override
- **WHEN** a policy entry in `complytime.yaml` contains `verification.trusted_root: /path/to/entry_root.json`
- **THEN** the per-entry `TrustedRoot` field SHALL equal `/path/to/entry_root.json` and SHALL take precedence over workspace-level configuration

#### Scenario: Empty trusted root preserves default behavior
- **WHEN** `trusted_root` is not specified or is empty
- **THEN** the system SHALL use the existing TUF-based public Sigstore trusted root fetch

### Requirement: Trusted root mutual exclusivity with keyed verification
The system SHALL reject configurations that specify both `trusted_root` and `key` fields, because keyed verification does not use a trusted root.

#### Scenario: Trusted root with key is rejected
- **WHEN** a verification config contains both `trusted_root: /path/to/root.json` and `key: /path/to/key.pem`
- **THEN** validation SHALL return an error containing `"trusted_root cannot be used with key-based verification"`

#### Scenario: Trusted root with keyless config is valid
- **WHEN** a verification config contains `trusted_root`, `issuer`, and `identity` but no `key`
- **THEN** validation SHALL succeed

### Requirement: Trusted root requires keyless configuration
The system SHALL reject configurations that specify `trusted_root` without both `issuer` and `identity`, because `trusted_root` is only meaningful for keyless verification.

#### Scenario: Trusted root without issuer and identity
- **WHEN** a verification config contains `trusted_root: /path/to/root.json` but `issuer` and `identity` are both empty
- **THEN** validation SHALL return an error containing `"trusted_root requires issuer and identity for keyless verification"`

### Requirement: Trusted root file existence and path validation
The system SHALL sanitize the `trusted_root` path using `filepath.Clean` before any filesystem access, consistent with existing `key` path handling in `NewKeyedVerifier`. The system SHALL then verify that the cleaned file path exists at configuration validation time, failing early with a clear error message.

#### Scenario: Nonexistent trusted root file
- **WHEN** `trusted_root` is set to a path that does not exist on disk
- **THEN** validation SHALL return an error indicating the file was not found

#### Scenario: Existing trusted root file
- **WHEN** `trusted_root` is set to a path that exists on disk
- **THEN** validation SHALL succeed (content validation deferred to verifier construction)

#### Scenario: Path sanitization
- **WHEN** `trusted_root` is set to a path containing `..` or other non-canonical elements
- **THEN** validation SHALL clean the path using `filepath.Clean` before filesystem access

### Requirement: Keyless verifier loads custom trusted root
The `NewKeylessVerifier` function SHALL accept a `trustedRootPath` parameter. When non-empty, it SHALL load the trusted root from the specified file path using `root.NewTrustedRootFromPath()` instead of fetching via TUF.

#### Scenario: Custom trusted root loaded from file
- **WHEN** `NewKeylessVerifier` is called with a non-empty `trustedRootPath`
- **THEN** the verifier SHALL load the trusted root from that file path without network access

#### Scenario: Empty trusted root path uses TUF
- **WHEN** `NewKeylessVerifier` is called with an empty `trustedRootPath`
- **THEN** the verifier SHALL fetch the public Sigstore trusted root via TUF (existing behavior)

#### Scenario: Nonexistent trusted root file returns error
- **WHEN** `NewKeylessVerifier` is called with a `trustedRootPath` pointing to a nonexistent file
- **THEN** the function SHALL return an error containing `"failed to load trusted root from"`

#### Scenario: Invalid trusted root content returns error
- **WHEN** `NewKeylessVerifier` is called with a `trustedRootPath` pointing to a file containing invalid JSON
- **THEN** the function SHALL return an error containing `"failed to load trusted root from"`

### Requirement: Builder passthrough
The `buildVerifierFromConfig` function SHALL pass the `TrustedRoot` value from `VerificationConfig` to `NewKeylessVerifier` when constructing a keyless verifier.

#### Scenario: TrustedRoot propagated to verifier constructor
- **WHEN** `buildVerifierFromConfig` is called with a `VerificationConfig` containing a non-empty `TrustedRoot`
- **THEN** the `TrustedRoot` value SHALL be passed as the `trustedRootPath` argument to `NewKeylessVerifier`
