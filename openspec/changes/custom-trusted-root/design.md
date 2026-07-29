## Context

`complyctl get` verifies OCI artifact signatures via `sigstore-go` when a `verification:` block is configured in `complytime.yaml`. Two modes exist: keyless (OIDC issuer + SAN identity) and keyed (PEM public key). Keyless mode calls `root.FetchTrustedRootWithOptions()` via TUF to fetch the public Sigstore trusted root at verifier construction time. This hardcodes reliance on the public Sigstore infrastructure and prevents use with private Sigstore deployments.

The verifier pipeline is: `complytime.yaml` → `resolveVerifier()` (cache lookup) → `buildVerifierFromConfig()` → `NewKeylessVerifier(issuer, identity)` → `root.FetchTrustedRootWithOptions()` → verifier closure. The change inserts a branch point at the trusted root acquisition step.

## Goals / Non-Goals

**Goals:**
- Allow users to specify a custom `trusted_root.json` file path for keyless verification against private Sigstore instances.
- Validate the configuration at config-load time (fail-early on missing file, mutually exclusive with keyed mode).
- Preserve existing TUF-based behavior when no custom root is configured.

**Non-Goals:**
- Acceptance test Sigstore infrastructure (tracked on `opsx/678-policy-complypack-acceptance-tests`).
- Keyed verification changes (`NewKeyedVerifier` builds `TrustedPublicKeyMaterial` directly; no trusted root involved).
- Remote fetch of custom trusted roots (file path only, no URL support).
- TUF mirror or custom TUF repository configuration.

## Decisions

### D1: File path, not embedded JSON

**Decision**: Accept a filesystem path to `trusted_root.json`, not inline JSON in YAML config.

**Rationale**: Trusted roots are complex JSON documents (typically 2-5 KB). Embedding them in `complytime.yaml` would make configs unreadable, error-prone, and hard to manage across entries. File paths allow shared roots across workspaces and match how `key:` already works for keyed verification.

**Alternative**: Base64-encoded inline content. Rejected: same readability problem, harder to update.

### D2: Extend `NewKeylessVerifier` signature

**Decision**: Add `trustedRootPath string` as a third parameter to `NewKeylessVerifier()`. Branch on empty vs non-empty inside the function.

**Rationale**: The trusted root choice is an intrinsic part of keyless verifier construction. The function already handles TUF fetch; adding the custom-root branch keeps all root acquisition logic co-located. One caller (`buildVerifierFromConfig`) threads it through.

**Alternative**: Separate `NewKeylessVerifierWithRoot(path)` constructor. Rejected: duplicates the entire closure body for one branch difference.

### D3: Config validation with `os.Stat`

**Decision**: Check file existence at `ValidateVerificationConfig()` time using `os.Stat`.

**Rationale**: Fail-early semantics. Users get a clear config error at `complyctl init`/load time rather than a cryptic sigstore-go error during sync. Matches the pattern used elsewhere in the codebase for path validation.

**Trade-off**: The file could be removed between validation and use. Acceptable: `root.NewTrustedRootFromPath()` will still return a clear error at construction time.

### D4: Mutual exclusivity with `key`

**Decision**: `trusted_root` + `key` is a validation error.

**Rationale**: Keyed verification builds `TrustedPublicKeyMaterial` directly from the PEM key — it never uses a trusted root. Allowing both would be silently misleading (the trusted root would be ignored). Explicit rejection prevents user confusion.

## Risks / Trade-offs

- **[TOCTOU on file existence]** → Mitigation: `os.Stat` at validation is best-effort early feedback; `root.NewTrustedRootFromPath()` provides authoritative error at construction time. Both paths produce clear error messages.
- **[Signature change breaks callers]** → Mitigation: `NewKeylessVerifier` has exactly one caller (`buildVerifierFromConfig`). The change is mechanical. Test coverage exists for the builder.
- **[No content validation at config time]** → Mitigation: Validating trusted root JSON schema at config time would require importing sigstore-go into the config package. The existing pattern (validate path exists, let the library handle content) is consistent and sufficient.
