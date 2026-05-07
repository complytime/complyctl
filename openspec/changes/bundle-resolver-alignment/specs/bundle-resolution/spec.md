## ADDED Requirements

### Requirement: Manifest shape detection
The resolver SHALL detect whether a cached OCI artifact uses bundle format or split-layer format by inspecting `manifest.Config.MediaType`. If the config media type equals `application/vnd.gemara.manifest.v1+json`, the artifact SHALL be classified as bundle format.

#### Scenario: Bundle-format artifact detected
- **WHEN** a cached policy has `manifest.Config.MediaType` equal to `application/vnd.gemara.manifest.v1+json`
- **THEN** `DetectManifestShape` SHALL return `isBundleShape = true`

#### Scenario: Split-layer artifact detected
- **WHEN** a cached policy has `manifest.Config.MediaType` equal to `application/vnd.oci.empty.v1+json` or any non-bundle config type
- **THEN** `DetectManifestShape` SHALL return `isBundleShape = false`

### Requirement: Bundle artifact resolution
The resolver SHALL load policy content from bundle-format OCI artifacts by delegating to `go-gemara/bundle.Unpack()` and mapping `bundle.File.Type` values to `DependencyGraph` fields. The mapping SHALL be: `"Policy"` to policy content, `"ControlCatalog"` to controls, `"GuidanceCatalog"` to guidelines.

#### Scenario: Successful bundle resolution with all artifacts
- **WHEN** a bundle contains files with types `Policy`, `ControlCatalog`, and `GuidanceCatalog`
- **THEN** `ResolvePolicyGraph` SHALL return a `DependencyGraph` populated with controls, guidelines, assessments, and evaluator ID

#### Scenario: Bundle resolution with optional artifacts missing
- **WHEN** a bundle contains a `Policy` file but omits `ControlCatalog` or `GuidanceCatalog`
- **THEN** `ResolvePolicyGraph` SHALL succeed with the available content and leave missing optional fields as empty collections

#### Scenario: Bundle missing required Policy artifact
- **WHEN** a bundle contains no file with type `Policy`
- **THEN** `ResolvePolicyGraph` SHALL return an error containing "missing required Policy artifact"

### Requirement: Split-layer backward compatibility
The resolver SHALL continue to support split-layer OCI artifacts using distinct media type matching via `LoadLayerByMediaType()`. Existing split-layer resolution behavior SHALL NOT change.

#### Scenario: Split-layer resolution unchanged
- **WHEN** a cached policy has a non-bundle config media type with layers using `application/vnd.gemara.catalog.v1+yaml`, `guidance.v1+yaml`, and `policy.v1+yaml`
- **THEN** `ResolvePolicyGraph` SHALL resolve using the existing media-type matching path and produce identical results to pre-change behavior

### Requirement: PolicyLoader interface extension
The `PolicyLoader` interface SHALL include `LoadBundleFiles(policyID, version string) (map[string][]byte, error)` and `DetectManifestShape(policyID, version string) (bool, error)` methods to support bundle resolution and enable mock injection for testing.

#### Scenario: Interface supports both paths
- **WHEN** a resolver is constructed with any `PolicyLoader` implementation
- **THEN** the resolver SHALL be able to call both bundle and split-layer loading methods without type assertions or casting

### Requirement: Bundle-specific error diagnostics
The resolver SHALL return error messages that distinguish bundle-specific failures from split-layer failures. Errors SHALL include the policy ID, version, and failure category (unpack failure, missing required artifact, parse failure).

#### Scenario: Bundle unpack failure
- **WHEN** `bundle.Unpack()` fails for a bundle-format artifact
- **THEN** the error message SHALL contain "bundle unpack failed" with the policy ID and version

#### Scenario: Bundle policy parse failure
- **WHEN** a bundle's Policy file contains invalid YAML
- **THEN** the error message SHALL identify the parsing failure with context about which artifact failed

### Requirement: Bundle test mock using Pack
A `MockBundlePolicySource` test helper SHALL exist in `internal/cache/cachetest/` that uses `bundle.Pack()` to produce realistic bundle-format OCI content for unit tests.

#### Scenario: Mock produces valid bundle content
- **WHEN** tests create a `MockBundlePolicySource` with policy, catalog, and guidance YAML
- **THEN** the mock SHALL produce an OCI store containing a valid bundle artifact that `LoadBundleFiles` can unpack
