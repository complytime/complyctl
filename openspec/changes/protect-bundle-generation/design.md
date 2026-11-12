# Bounded consumer corrections

## Decisions and implementation plan

1. Track plan declarations in a resolver-local map keyed by authored
   plan ID. Register the bundle root and each imported Policy with its
   artifact filename and requirement ID; validate split-layer roots
   with their repository identity. Reject every second declaration,
   including identical requirements. Do not rewrite IDs or extend the
   provider/result identity model.
2. The original projection-version design is superseded by the
   resolved-tree identifier in `preserve-bundle-artifacts/design.md`.
   NewGenerationState records that identifier after successful
   generation; IsFresh compares it and complypack digests. Missing
   identifiers make legacy state stale. The policy OCI digest remains
   provenance, not an independent invalidation condition.
3. Document automatic upgrade regeneration and collision diagnostics
   in the changelog. No structural README or AGENTS update is needed.

## Analysis and verification checklist

The root/import append and scan's last-wins mapping confirm FR-001.
ensureGenerated skips generation on checkGenerationFreshness success;
The revised tree freshness check reuses both explicit generate and
scan generation constructors, as specified by preserve-bundle-artifacts.
This adds no dependencies or cache redesign. The user authorizes local
signed/signoff commits only, overriding the usual spec push step.

Use RED/GREEN tests with real SDK-packed bundles and persisted state.
Cover cross-artifact and internal duplicate IDs, matching requirement
IDs, valid single Policies, unchanged digests with nonempty directories,
tree-identifier round trips, resolved-content changes, and missing artifacts.
Run focused tests, policy and CLI race tests, make test-unit, vet, lint,
formatting, and git diff --check. Report inherited gate failures without
changing their configuration. Constitution and convention packs were
read before this plan; these scoped corrections follow their rules.
