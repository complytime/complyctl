## ADDED Requirements

### FR-001: Evict prior versions on complypack store

**Given** a complypack with evaluator-id `E` version `2.0.0` is being stored
**And** a prior version directory `~/.complytime/complypacks/E/1.0.0/` exists
**When** `ComplypackCache.Store()` completes successfully
**Then** the directory `~/.complytime/complypacks/E/1.0.0/` MUST NOT exist
**And** the directory `~/.complytime/complypacks/E/2.0.0/` MUST exist with content

### FR-002: Eviction handles multiple stale versions

**Given** directories `~/.complytime/complypacks/E/0.1.0/`, `E/0.2.0/`, `E/0.3.0/` exist
**When** `Store()` writes version `1.0.0` for evaluator-id `E`
**Then** all three prior directories MUST be removed
**And** only `E/1.0.0/` remains

### FR-003: Eviction does not affect other evaluator-ids

**Given** directories `~/.complytime/complypacks/opa/1.0.0/` and `ampel/1.0.0/` exist
**When** `Store()` writes version `2.0.0` for evaluator-id `opa`
**Then** `ampel/1.0.0/` MUST NOT be affected

### FR-004: Eviction tolerates missing evaluator directory

**Given** no prior cache directory exists for evaluator-id `E`
**When** `Store()` writes version `1.0.0` for evaluator-id `E`
**Then** the store MUST succeed without error

### FR-005: Same version re-store is idempotent

**Given** `~/.complytime/complypacks/E/1.0.0/` already exists
**When** `Store()` writes version `1.0.0` for the same evaluator-id `E`
**Then** the directory is atomically replaced (existing behavior)
**And** no error occurs
