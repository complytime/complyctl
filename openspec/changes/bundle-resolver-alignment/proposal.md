## Why

The production publish path for `complytime-policies` uses `gemara-publish-oci` with `go-gemara/bundle.Pack()`, producing OCI artifacts where all layers share the media type `application/vnd.gemara.artifact.v1+yaml` and are differentiated by annotations (`org.gemara.artifact.type`). The `complyctl` resolver in `internal/policy/loader.go` matches layers by distinct media types (`catalog.v1+yaml`, `guidance.v1+yaml`, `policy.v1+yaml`), so `ResolvePolicyGraph` fails with "layer with media type X not found" for any bundle-format artifact. This blocks `complyctl scan` for all policies published through the standard pipeline.

## What Changes

- Detect OCI manifest shape at load time by inspecting `manifest.Config.MediaType` against `bundle.MediaTypeManifest`
- Add a bundle resolution path that delegates to `go-gemara/bundle.Unpack()` and maps `bundle.File.Type` values (`Policy`, `ControlCatalog`, `GuidanceCatalog`) to the existing `DependencyGraph`
- Preserve the existing split-layer resolution path unchanged for backward compatibility
- Add explicit error categories for bundle-specific failures (missing required Policy artifact, unpack failures, malformed content)
- Extend `PolicyLoader` interface with `LoadBundleFiles` and `DetectManifestShape` methods
- Add `MockBundlePolicySource` test helper using `bundle.Pack()` for bundle-format test coverage

## Capabilities

### New Capabilities
- `bundle-resolution`: Detect and resolve Gemara bundle-format OCI artifacts in the policy resolver, using `go-gemara/bundle.Unpack()` for extraction and annotation-based artifact type matching

### Modified Capabilities

## Impact

- **Code**: `internal/policy/loader.go`, `internal/policy/resolver.go`, `internal/cache/cachetest/mock_source.go`
- **Tests**: `internal/policy/loader_test.go`, `internal/policy/resolver_test.go`
- **Dependencies**: `go-gemara/bundle` package (v0.4.0, already in `go.mod`) vendored
- **Upstream**: Aligns with [go-gemara #64](https://github.com/gemaraproj/go-gemara/issues/64) ("Create Gemara bundle resolution logic for consumers") as an interim consumer-side adapter; resolution logic will migrate upstream once "Effective" types land in `go-gemara`
- **No breaking changes**: split-layer resolution, `complyctl get`, and existing cache behavior are unchanged
