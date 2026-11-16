# Harden bundle consumer boundaries

## Why

Final review found silently discarded YAML documents, mislabeled supporting
catalogs, and an imported evaluator shortcut leaking into root routing.
The user explicitly ruled that the importing Policy's evaluator settings
take precedence over imported settings. The contrary review proposal is
superseded by that ruling.

## What Changes

The shared Policy parser MUST require exactly one document in every role,
including legitimate zero-plan Policies. Supporting catalog parsers MUST
check declared types before projection, retaining missing-ID fallback.
Root evaluator routing MUST retain precedence. An absent root evaluator
MUST NOT become a whole-graph override from the first imported Policy.
Cross-boundary tests MUST check actual provider groups and plan membership.
Metadata tests MUST distinguish control sums from catalog counts.

## Impact

This tactical correction is scoped to `internal/policy` and its tests.
Dependencies, authored content, and assessment execution contracts stay
unchanged. The approved local-only workflow supersedes the repository's
push requirement. No PR or independent reviewer dispatch is part of this
fix wave.

## Archive clarification

The evaluator precedence proposal above is historical. The subsequent
bounded fixes merged in PR #967 supersede it: each assessment retains
its declaring Policy's effective evaluator binding. The amended
`bundle-consumption` delta and FR-002 of
`openspec/specs/bundle-artifact-inventory/spec.md` record that contract.
