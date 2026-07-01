## ADDED Requirements

### FR-001: Record empty digest when complypack was unavailable

**Given** a complypack for evaluator-id `E` is configured in `state.json`
**But** the complypack cache directory is missing (no `content.tar.gz`)
**When** `complyctl generate` completes and saves generation state
**Then** the recorded complypack digest for `E` MUST be empty/absent
**And** the next `complyctl scan` MUST trigger regeneration (digest mismatch)

### FR-002: Record actual digest when complypack was available

**Given** a complypack for evaluator-id `E` is cached with digest `D`
**And** the content path was successfully resolved and passed to the provider
**When** `complyctl generate` saves generation state
**Then** the recorded complypack digest for `E` MUST be `D`

### FR-003: Policy digest always from state.json

**Given** a policy is cached with digest `P` in `state.json`
**When** generation state is saved
**Then** the policy digest MUST be `P` (no change to policy digest handling)

### FR-004: Mixed complypack availability

**Given** two evaluators: `opa` (complypack available, digest `D1`) and `ampel` (complypack missing)
**When** generation state is saved
**Then** `opa` digest MUST be `D1`
**And** `ampel` digest MUST be empty/absent
