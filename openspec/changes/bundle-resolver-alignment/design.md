## Context

`complyctl`'s policy resolver loads OCI artifacts from a local cache (`~/.complytime`) populated by `complyctl get` (which uses `oras.Copy`). The resolver in `internal/policy/resolver.go` builds a `DependencyGraph` by loading layers matched by distinct media types (`application/vnd.gemara.catalog.v1+yaml`, `guidance.v1+yaml`, `policy.v1+yaml`) via `Loader.LoadLayerByMediaType()`.

The production publish pipeline (`gemara-publish-oci` + `go-gemara/bundle.Pack()`) produces OCI artifacts with a different layout: a single config media type (`application/vnd.gemara.manifest.v1+json`) and all layers sharing `application/vnd.gemara.artifact.v1+yaml`, differentiated only by OCI annotations (`org.gemara.artifact.type`). The resolver cannot match these layers by media type, causing `ResolvePolicyGraph` to fail.

`go-gemara` v0.4.0 (already in `go.mod`) provides `bundle.Unpack()` which handles annotation-based extraction and returns typed `bundle.File` structs. The upstream project has [go-gemara #64](https://github.com/gemaraproj/go-gemara/issues/64) to eventually move full resolution logic into the SDK.

## Goals / Non-Goals

**Goals:**
- Enable `complyctl scan` to resolve policy graphs from bundle-format OCI artifacts
- Preserve full backward compatibility with split-layer artifacts (existing caches, mock-oci-registry, tests)
- Delegate bundle extraction to `go-gemara/bundle.Unpack()` rather than reimplementing annotation matching
- Provide actionable error messages distinguishing bundle-specific failures from split-layer failures
- Position the resolver as a thin adapter so migration to go-gemara #64 upstream resolution is straightforward

**Non-Goals:**
- Modifying the publish pipeline or `gemara-publish-oci` action (separate repos)
- Changing `complyctl get` / `oras.Copy` behavior (format-agnostic, works as-is)
- Implementing full "Effective Catalog/Policy" resolution (upstream go-gemara #64 scope)
- Deprecating or removing split-layer support

## Decisions

### 1. Shape detection via Config MediaType

Detect manifest shape by checking `manifest.Config.MediaType == bundle.MediaTypeManifest` (`application/vnd.gemara.manifest.v1+json`). This is the canonical marker set by `bundle.Pack()`.

**Alternatives considered:**
- Inspect individual layer media types (fragile if layers are mixed or additional types added)
- Check annotation presence on layers (couples to annotation key naming which could change)

### 2. Delegate to bundle.Unpack() for extraction

Use `go-gemara/bundle.Unpack(ctx, store, version)` to extract the bundle, then map `bundle.File.Type` values (`"Policy"`, `"ControlCatalog"`, `"GuidanceCatalog"`) to `DependencyGraph` fields.

**Alternatives considered:**
- Manual annotation parsing on layer descriptors (duplicates logic already in go-gemara)
- Require publish-side reformatting to split layout (doesn't unblock current users)

### 3. Extend PolicyLoader interface

Add `LoadBundleFiles(policyID, version) (map[string][]byte, error)` and `DetectManifestShape(policyID, version) (bool, error)` to the `PolicyLoader` interface. This enables mock injection for both bundle and split paths in tests.

**Alternatives considered:**
- Single `Load` method with internal branching (harder to test, less explicit API surface)
- Separate loader type for bundles (unnecessary complexity for a branching concern)

### 4. MockBundlePolicySource for tests

Create `MockBundlePolicySource` in `internal/cache/cachetest/` that uses `bundle.Pack()` to produce realistic bundle-format OCI content for resolver tests.

**Alternatives considered:**
- Hand-crafted test fixtures (brittle, diverges from actual bundle format)
- Use remote registry in tests (slow, requires network, not unit-test appropriate)

## Risks / Trade-offs

- **[Bundle format evolution]** If `go-gemara` changes bundle layout or annotation keys in a future version, `Unpack()` handles it transparently since we delegate to the SDK. Mitigation: pinned dependency version + vendor.
- **[Interface expansion]** Adding methods to `PolicyLoader` requires updating all implementations (real + mock). Mitigation: only two implementations exist (Loader + test mocks), both in this repo.
- **[Interim duplication]** Graph assembly logic (mapping File.Type to DependencyGraph) exists in `complyctl` until go-gemara #64 provides upstream resolution. Mitigation: implementation is thin (~50 lines) and explicitly documented as interim.
- **[Optional artifacts]** Bundle may omit `ControlCatalog` or `GuidanceCatalog`. Mitigation: bundle path mirrors split-layer optional handling -- only `Policy` is required.
