## 1. Configuration

- [x] 1.1 Add `TrustedRoot string` field with `yaml:"trusted_root,omitempty"` tag to `VerificationConfig` in `internal/complytime/config.go`
- [x] 1.2 Add `trusted_root` + `key` mutual exclusivity check to `ValidateVerificationConfig()` — return error `"trusted_root cannot be used with key-based verification"`
- [x] 1.3 Add `trusted_root` requires `issuer` + `identity` check to `ValidateVerificationConfig()` — return error `"trusted_root requires issuer and identity for keyless verification"`
- [x] 1.4 Apply `filepath.Clean` to non-empty `TrustedRoot` before `os.Stat` file existence check in `ValidateVerificationConfig()`

## 2. Verifier Construction

- [x] 2.1 Add `trustedRootPath string` parameter to `NewKeylessVerifier()` in `internal/cache/verify.go`
- [x] 2.2 Implement branch: when `trustedRootPath` is non-empty, call `root.NewTrustedRootFromPath(trustedRootPath)` instead of TUF fetch; wrap errors with `"failed to load trusted root from %s: %w"`
- [x] 2.3 Update `buildVerifierFromConfig()` in `cmd/complyctl/cli/get.go` to pass `cfg.TrustedRoot` to `NewKeylessVerifier()`

## 3. Tests — Config Validation

- [x] 3.1 Test `trusted_root` with keyless config (issuer + identity) passes validation in `internal/complytime/config_test.go`
- [x] 3.2 Test `trusted_root` with keyed config (`key` set) returns mutual exclusivity error
- [x] 3.3 Test `trusted_root` without `issuer`/`identity` returns `"trusted_root requires issuer and identity for keyless verification"` error
- [x] 3.4 Test `trusted_root` with nonexistent file path returns file-not-found error
- [x] 3.5 Test YAML round-trip: `trusted_root` field populates `TrustedRoot` correctly

## 4. Tests — Verifier Construction

- [x] 4.1 Create minimal valid `trusted_root.json` test fixture in `internal/cache/testdata/` — use a well-formed Sigstore TrustedRoot protobuf-JSON document (can be extracted from vendored sigstore-go test fixtures or constructed minimally)
- [x] 4.2 Test `NewKeylessVerifier` with valid `trusted_root.json` fixture creates verifier without error in `internal/cache/verify_test.go`
- [x] 4.3 Test `NewKeylessVerifier` with nonexistent `trustedRootPath` returns error containing `"failed to load trusted root from"`
- [x] 4.4 Test `NewKeylessVerifier` with `trustedRootPath` pointing to invalid JSON returns error containing `"failed to load trusted root from"`
- [x] 4.5 Test `NewKeylessVerifier` with empty `trustedRootPath` does not call `root.NewTrustedRootFromPath` (verify code takes TUF branch by confirming no file-read error for a nonexistent path that would fail if the custom-root branch were taken)

## 5. Tests — Builder Passthrough

- [x] 5.1 Test `buildVerifierFromConfig` passes `TrustedRoot` to `NewKeylessVerifier` in `cmd/complyctl/cli/get_verify_test.go`

## 6. Documentation & Governance

- [x] 6.1 Add CHANGELOG.md entry under "Unreleased" → "Added" describing `trusted_root` config field for private Sigstore instances
- [x] 6.2 Update AGENTS.md "Recent Changes" section with feature summary
- [x] 6.3 Assess whether `docs/QUICK_START.md` verification guidance needs a `trusted_root` configuration example — assessed: Quick Start does not document `verification:` config at all (base sigstore-verification feature also omitted it); adding `trusted_root` alone would be premature; no change needed
- [x] 6.4 Add `CT.COMPLYCTL.THR02.MIT05` entry to `governance/threats/complytime-threats.yaml` documenting user-supplied trusted root file as trust anchor without TUF integrity protection

<!-- spec-review: passed -->
