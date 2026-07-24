## Assumptions

- `go-containerregistry`'s `name.ParseReference()` defaults bare references (without a registry host) to `index.docker.io`. This is the root cause of the verification mismatch this spec fixes.

## ADDED Requirements

### Requirement: BuildLookupRef includes registry host
`BuildLookupRef` SHALL accept a `registryHost` parameter as its first argument and prepend `registryHost + "/"` to the constructed OCI reference when `registryHost` is non-empty.

#### Scenario: Non-empty registry host produces host-qualified reference
- **WHEN** `BuildLookupRef` is called with `registryHost` = `registry.example.com`, `repository` = `policies/verify-keyed`, `tag` = `v1.0.0`, and empty `digest`
- **THEN** the returned reference SHALL be `registry.example.com/policies/verify-keyed:v1.0.0`

#### Scenario: Empty registry host preserves backward-compatible output
- **WHEN** `BuildLookupRef` is called with empty `registryHost`, `repository` = `policies/verify-keyed`, `tag` = `v1.0.0`, and empty `digest`
- **THEN** the returned reference SHALL match the pre-change behavior (`policies/verify-keyed:v1.0.0`) with no host prefix and no leading `/`

#### Scenario: Registry host with digest reference
- **WHEN** `BuildLookupRef` is called with `registryHost` = `registry.example.com`, `repository` = `policies/verify-keyed`, empty `tag`, and `digest` = `sha256:abc123`
- **THEN** the returned reference SHALL be `registry.example.com/policies/verify-keyed@sha256:abc123`

### Requirement: Sync structs carry registry host
`Sync` and `ComplypackSync` structs SHALL each contain a `registryHost` field populated at construction time. `NewSync` and `NewComplypackSync` SHALL each accept a `registryHost string` parameter.

#### Scenario: Sync constructed with registry host
- **WHEN** `NewSync` is called with `registryHost` = `registry.example.com`
- **THEN** the returned `Sync` struct SHALL store `registry.example.com` as its `registryHost`

#### Scenario: ComplypackSync constructed with registry host
- **WHEN** `NewComplypackSync` is called with `registryHost` = `registry.example.com`
- **THEN** the returned `ComplypackSync` struct SHALL store `registry.example.com` as its `registryHost`

### Requirement: Verification resolves against configured registry
All `VerifyFunc` invocations during policy and complypack sync SHALL receive a `registryRef` argument that includes the configured registry host as a prefix.

**Invariant:** The `registryHost` passed to `NewSync`/`NewComplypackSync` MUST be the same registry host used to construct the corresponding `PolicySource`/`ComplypackSource` for that sync operation. Both derive from `ref.Registry` at the single construction site in `get.go`.

#### Scenario: Policy pre-copy verification uses configured registry
- **WHEN** `SyncPolicy` is called on a `Sync` with `registryHost` = `registry.example.com` and verification enabled
- **THEN** the `registryRef` passed to `VerifyFunc` SHALL have prefix `registry.example.com/`
- **AND** the `registryRef` SHALL NOT have prefix `index.docker.io/`

#### Scenario: Complypack pre-copy verification uses configured registry
- **WHEN** `SyncComplypack` is called on a `ComplypackSync` with `registryHost` = `registry.example.com` and verification enabled
- **THEN** the `registryRef` passed to `VerifyFunc` for pre-copy verification SHALL have prefix `registry.example.com/`

#### Scenario: Complypack cache re-verification uses configured registry
- **WHEN** `SyncComplypack` encounters a local cache hit and re-verification is triggered on a `ComplypackSync` with `registryHost` = `registry.example.com`
- **THEN** the `registryRef` passed to `VerifyFunc` for cache re-verification SHALL have prefix `registry.example.com/`

### Requirement: Fail-closed when registry host missing with verifier
When a verifier is configured but `registryHost` is empty, sync operations SHALL return an error immediately without attempting verification. The error message SHALL contain the substring "registry host" to guide users toward correct configuration.

NOTE: `ValidateOCIRef` (called at config load time) rejects bare policy IDs without a registry host. The fail-closed guard here is defense-in-depth for the runtime sync path, catching any edge case where an empty `registryHost` reaches sync despite config validation.

#### Scenario: SyncPolicy fails closed with empty registry host
- **WHEN** `SyncPolicy` is called on a `Sync` with empty `registryHost` and a non-nil `VerifyFunc`
- **THEN** `SyncPolicy` SHALL return an error containing "registry host"
- **AND** the `VerifyFunc` SHALL NOT be called

#### Scenario: SyncComplypack fails closed with empty registry host
- **WHEN** `SyncComplypack` is called on a `ComplypackSync` with empty `registryHost` and a non-nil `VerifyFunc`
- **THEN** `SyncComplypack` SHALL return an error containing "registry host"
- **AND** the `VerifyFunc` SHALL NOT be called

### Requirement: get.go passes registry host to constructors
`cmd/complyctl/cli/get.go` SHALL pass `ref.Registry` (from `ParsePolicyRef`) to `NewSync` and `NewComplypackSync`.

#### Scenario: Policy sync receives registry from parsed reference
- **WHEN** `complyctl get` processes a policy entry with registry `registry.example.com`
- **THEN** the `NewSync` call SHALL receive `registry.example.com` as the `registryHost` argument

#### Scenario: Complypack sync receives registry from parsed reference
- **WHEN** `complyctl get` processes complypacks with registry `registry.example.com`
- **THEN** the `NewComplypackSync` call SHALL receive `registry.example.com` as the `registryHost` argument
