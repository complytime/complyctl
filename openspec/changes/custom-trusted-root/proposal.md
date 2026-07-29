## Why

Organizations running private Sigstore deployments (self-hosted Fulcio CA and Rekor transparency log) cannot use `complyctl get --verification` because `NewKeylessVerifier()` hardcodes the public Sigstore TUF root. There is no way to supply a custom `trusted_root.json`, making keyless verification unusable in air-gapped or enterprise environments with private signing infrastructure. Tracked in [#768](https://github.com/complytime/complyctl/issues/768).

## What Changes

- Add `trusted_root` field to `VerificationConfig` (workspace-level and per-entry) that accepts a filesystem path to a `trusted_root.json` file.
- Extend `NewKeylessVerifier()` to load a custom trusted root from disk when `trusted_root` is set, bypassing the TUF-based public Sigstore root fetch.
- Add validation: `trusted_root` + `key` is rejected (mutual exclusivity); file existence is checked at config validation time.
- When `trusted_root` is empty (default), existing TUF-based behavior is unchanged.

## Capabilities

### New Capabilities
- `custom-trusted-root`: Support for specifying a custom `trusted_root.json` file for keyless signature verification against private Sigstore instances.

### Modified Capabilities
<!-- No existing specs to modify -->

## Impact

- **Config**: `VerificationConfig` struct gains `TrustedRoot` field; `complytime.yaml` schema gains `trusted_root` key under `verification:` at both workspace and per-entry levels.
- **Verification pipeline**: `NewKeylessVerifier()` signature changes (adds `trustedRootPath string` parameter); callers in `cmd/complyctl/cli/get.go` must pass the new argument.
- **Dependencies**: No new dependencies. Uses `root.NewTrustedRootFromPath()` already available in vendored `sigstore-go`.
- **Breaking**: None. Empty `trusted_root` preserves current behavior.
