# Shared decoder correction

Keep `parsePolicyLayer` as the single boundary used by root bundle,
split-layer, metadata, and supporting import paths. Replace its plan
count guard with existing SDK artifact-type detection and minimal
nonblank `metadata.id` validation, retaining typed `gemara.Policy`
unmarshalling and the existing projection. Do not add a role argument.
SDK type detection supplies strict discriminator handling; typed
unmarshalling rejects malformed known field shapes. This is bounded
consumer input validation, not complete Gemara schema conformance.

Projection continues to emit only authored plans. A zero-plan Policy
has no single-evaluator shortcut; its timeline and mapping references
still project. Imported executable plans can supply absent defaults
through the existing graph assembly. Keep every raw bundle artifact
unchanged, including guidance imports and constraints.

Replace the obsolete `TestParsePolicyLayer_MissingAssessmentPlans`
expectation explicitly with acceptance assertions. Regressions cover
real SDK-packed zero-plan roots/imports, split layers, catalogs,
guidance, mapping references, timeline/default precedence, and invalid
documents in each decoding role. Observe RED before implementation.

Run focused tests, policy race/vet, full `make test-unit`, changed-code
lint, formatting, and whitespace checks. Reuse checksum-verified real
Product Security output with the actual standards consume script.
Inspect zero-assessment scan behavior without changing CLI semantics;
report provider-only limits without claiming successful assessments.
