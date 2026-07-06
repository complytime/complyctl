## ADDED Requirements

### FR-001: Detect duplicate evaluator-id across complypack entries

**Given** `state.json` contains two complypack entries with different repositories
**And** both entries have the same evaluator-id `E`
**When** `complyctl get` completes complypack synchronization
**Then** an error MUST be emitted identifying both repositories and the shared evaluator-id
**And** the command MUST exit with non-zero status

### FR-002: Error message is actionable

**Given** a duplicate evaluator-id conflict is detected
**When** the error is displayed
**Then** the message MUST include both repository URLs
**And** the message MUST include the shared evaluator-id
**And** the message MUST suggest removing one of the conflicting entries from `complytime.yaml`

### FR-003: No conflict when evaluator-ids are unique

**Given** all complypack entries resolve to distinct evaluator-ids
**When** validation runs
**Then** no error is produced and the command proceeds normally

### FR-004: Single complypack entry always passes

**Given** only one complypack entry exists in the configuration
**When** validation runs
**Then** no error is produced
