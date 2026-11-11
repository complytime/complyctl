## 1. Loader inventory

- [x] 1.1 Add and run the failing inventory preservation tests.
- [x] 1.2 Implement inventory, filters, and compatibility wrapper.
- [x] 1.3 Verify loader tests pass with standard SDK layouts.

## 2. Resolver consumption

- [x] 2.1 Add and run failing multi-artifact resolver tests.
- [x] 2.2 Consume every supporting artifact and imported Policy.
- [x] 2.3 Verify metadata totals, precedence, and failure paths.

## 3. Verification and delivery

- [x] 3.1 Assess documentation impact and update the changelog.
- [x] 3.2 Run required tests and applicable quality checks.
- [x] 3.3 Self-review scope and constitution alignment; commit Task 9.

## 4. Resolved tree freshness follow-up

- [x] 4.1 Review and commit/push the revised spec before implementation.
- [x] 4.2 Add failing canonical tree identifier regression tests.
- [x] 4.3 Implement deterministic resolved Gemara tree identification.
- [x] 4.4 Add failing freshness and legacy-state upgrade tests.
- [x] 4.5 Replace projection version and policy digest freshness checks.
- [x] 4.6 Share tree identification between explicit generate and scan.
- [x] 4.7 Verify complypack and missing-artifact invalidation still work.
- [x] 4.8 Verify failed generation never promotes freshness state.
- [x] 4.9 Update changelog and assess README/AGENTS documentation impact.
- [x] 4.10 Run CI-parity unit, E2E, integration, and applicable lint checks.

Verification: unit tests pass with `TZ=UTC` (the local timezone exposes
an existing cache timestamp assertion); E2E and all 26 integration
checks pass. Formatting, strict OpenSpec validation, and buf lint pass.
Full lint remains blocked by existing G703 findings in
`internal/cache/state.go`; API compatibility requires the missing
`apidiff` tool. README and AGENTS need no structural or usage changes.

## 5. Bounded PR feedback corrections

- [ ] 5.1 Review and commit/push this narrow spec amendment before code.
- [ ] 5.2 Add failing report identity tests; preserve returned plan IDs
  and allow requirement fallback only for unique plan mappings.
- [ ] 5.3 Add failing SDK-bundle parameter tests; preserve typed authored
  declarations and include them in the resolved tree digest.
- [ ] 5.4 Add failing imported evaluator routing tests; preserve each
  assessment's effective binding during grouping.
- [ ] 5.5 Add failing order-independent collision tests; reject ambiguous
  catalog/control/requirement IDs and conflicting mapping references,
  coalescing equal references while keeping duplicate-plan rejection.
- [ ] 5.6 Add failing complete-count and populated-cache refresh tests;
  update extraction and cache-hit metadata refresh.
- [ ] 5.7 Align earlier freshness specifications and quick-start guidance;
  update the changelog and assess README/AGENTS impact.
- [ ] 5.8 Run focused regression tests and applicable CI-parity checks;
  self-review the bounded diff and record verification evidence.
