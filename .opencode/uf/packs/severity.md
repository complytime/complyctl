---
description: "Shared severity level definitions for all Divisor Council personas."
---
<!-- scaffolded by uf vdev -->

# Severity Convention Pack

This pack defines the shared severity levels used by all
five Divisor Council personas. All personas MUST reference
these definitions rather than defining severity inline.

## Severity Levels

### CRITICAL

**Definition**: The change introduces a defect that will
cause data loss, security breach, build failure, or
constitutional violation. The change MUST NOT be merged.

**Boundary**: Immediate, concrete harm. Not theoretical
risk — actual breakage or exposure.

| Persona | Examples |
|---------|---------|
| Adversary | Hardcoded production secret, SQL injection vector, panic in library code |
| Tester | Missing coverage strategy in spec/plan (Constitution IV violation), test that masks a real failure |
| Guard | Constitution principle violated without justification, implementation contradicts spec acceptance criteria |
| SRE | Release pipeline broken (won't produce artifacts), destructive operation without guard, critical CVE in dependency |
| Architect | Fundamental misalignment with project architecture (score 1-2), circular dependency introduced |

### HIGH

**Definition**: The change introduces significant risk or
technical debt that will cause problems if not addressed
before merge. Blocks the review.

**Boundary**: Likely to cause problems in the near term.
Requires action but not an emergency.

| Persona | Examples |
|---------|---------|
| Adversary | Credentials logged at INFO level, unpinned CI action on mutable tag, unchecked type assertion |
| Tester | Vague acceptance criteria ("works correctly"), shallow assertions (err == nil only), missing regression test for known bug |
| Guard | Scope creep beyond spec, acceptance criterion with no corresponding task, undocumented constitution trade-off |
| SRE | Missing upgrade path for format change, hardcoded environment values, no error recovery for I/O failure |
| Architect | Notable architectural deviation (score 5-6), competing pattern for same abstraction, significant DRY violation |

### MEDIUM

**Definition**: The change has a quality issue that should
be addressed but does not block the merge. In Spec Review
Mode, auto-fixable.

**Boundary**: Improvement opportunity. The code/spec works
but could be better.

| Persona | Examples |
|---------|---------|
| Adversary | Overly broad file permissions (0o755 → 0o644), missing context in error wrap, redundant file read |
| Tester | Missing fixture specification, test isolation concern (shared state but no observed failure), convention deviation |
| Guard | Minor scope addition beyond spec (gold plating), stale cross-reference, metadata inconsistency |
| SRE | Missing operational documentation section, incomplete platform support, unquantified performance requirement |
| Architect | Minor convention deviation, missing GoDoc on exported function, test naming doesn't follow pattern |

### LOW

**Definition**: Minor style or documentation improvement.
Non-blocking. In Spec Review Mode, auto-fixable.

**Boundary**: Cosmetic or informational. No functional
impact.

| Persona | Examples |
|---------|---------|
| Adversary | Comment suggesting security review for future feature, minor naming inconsistency in error variable |
| Tester | Minor test naming convention issue, optional observability enhancement in test output |
| Guard | Minor documentation wording improvement, optional cross-reference addition |
| SRE | Style improvement in error messages, optional health check enhancement, minor doc gap |
| Architect | Formatting preference, optional structural improvement, minor comment enhancement |

## Auto-Fix Policy (Spec Review Mode)

| Severity | Action | Rationale |
|----------|--------|-----------|
| LOW | Auto-fix | Cosmetic; safe to fix without human judgment |
| MEDIUM | Auto-fix | Quality improvement; deterministic fix |
| HIGH | Report only | Requires human judgment on intent/scope |
| CRITICAL | Report only | Requires human judgment; may indicate design issue |

This policy is implemented in `/review-council` Spec
Review Mode. The severity definitions above ensure all 5
personas classify the same type of issue at the same
level, making the auto-fix boundary predictable.
