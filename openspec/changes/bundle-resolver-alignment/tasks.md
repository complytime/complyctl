## 1. Foundation: Shape Detection and Loader

- [x] 1.1 Add `isBundleManifest()` helper in `internal/policy/loader.go` that checks `manifest.Config.MediaType == bundle.MediaTypeManifest`
- [x] 1.2 Add `resolveManifest()` helper in `internal/policy/loader.go` to fetch and parse OCI manifest from store
- [x] 1.3 Implement `DetectManifestShape()` on `Loader` that opens the manifest for a cached policy and returns whether it is bundle or split-layer
- [x] 1.4 Implement `LoadBundleFiles()` on `Loader` that calls `bundle.Unpack()` and returns files keyed by artifact type (`Policy`, `ControlCatalog`, `GuidanceCatalog`)
- [x] 1.5 Vendor `go-gemara/bundle` package and update `vendor/modules.txt`

## 2. Resolver: Bundle Path

- [x] 2.1 Extend `PolicyLoader` interface in `internal/policy/resolver.go` with `LoadBundleFiles` and `DetectManifestShape` methods
- [x] 2.2 Update `ResolvePolicyGraph()` to call `DetectManifestShape` and branch to bundle or split-layer path
- [x] 2.3 Implement `resolveBundleGraph()` that maps `bundle.File.Type` values to `DependencyGraph` fields (controls, guidelines, assessments, timeline)
- [x] 2.4 Ensure `resolveBundleGraph()` treats `ControlCatalog` and `GuidanceCatalog` as optional while requiring `Policy`

## 3. Resolver: Split-Layer Compatibility

- [x] 3.1 Extract existing split-layer resolution into `resolveSplitGraph()` method for clarity
- [x] 3.2 Verify all existing split-layer resolver tests pass without modification

## 4. Error Diagnostics

- [x] 4.1 Add bundle-specific error messages: "bundle unpack failed", "missing required Policy artifact", parse failure context
- [x] 4.2 Ensure errors include policy ID and version in all bundle-path failure messages

## 5. Test Infrastructure

- [x] 5.1 Create `MockBundlePolicySource` in `internal/cache/cachetest/mock_source.go` using `bundle.Pack()` to produce bundle-format OCI stores
- [x] 5.2 Add bundle-shape happy-path test cases in `internal/policy/resolver_test.go` (full bundle with all three artifact types)
- [x] 5.3 Add bundle with optional artifacts missing test case in `internal/policy/resolver_test.go`
- [x] 5.4 Add bundle missing required Policy test case in `internal/policy/resolver_test.go`
- [x] 5.5 Add `DetectManifestShape` and `LoadBundleFiles` unit tests in `internal/policy/loader_test.go`

## 6. Validation

- [x] 6.1 Run full `go test ./internal/policy/...` and confirm all tests pass (bundle + split-layer)
- [x] 6.2 Run `go vet ./...` and linter checks
