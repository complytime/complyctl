## 1. Specification

- [x] 1.1 Record the binding user precedence ruling and boundary fixes.
- [x] 1.2 Validate and commit tactical specification before implementation.

## 2. Regressions and fixes

- [x] 2.1 Observe RED for document cardinality and catalog type rejection.
- [x] 2.2 Exercise root/import evaluator disagreements through grouping.
- [x] 2.3 Implement minimal fixes and observe focused GREEN.
- [x] 2.4 Strengthen unequal-sized catalog count fixtures.

## 3. Verification

- [x] 3.1 Run policy race, unit, vet, formatting and OpenSpec checks.
- [x] 3.2 Review diff and documentation impact; commit locally.

Implementation commit: `c439bbed`. Policy race, full unit suite, vet,
formatting, strict OpenSpec and changed-code lint passed. Full lint still
reports the three unchanged cache G703 findings. Fresh real get, inventory
and graph checks passed; scan exit 1 remains provider-unavailable, with no
successful assessment claim. Full evidence is retained by the final fix
wave's controller handoff report; independent scoped review remains pending.
