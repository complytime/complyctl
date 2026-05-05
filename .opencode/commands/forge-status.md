---
description: Check forge coordination status - workers, messages, cells
---

# /forge:status

Show active forge state.

## Workflow

1. `forge_status(epic_id, project_key)` — worker progress summary
2. `comms_inbox()` — pending messages from workers
3. `org_cells(status="in_progress")` — active cells

## Interpreting Results

- **Workers**: Each subtask shows completion percentage and status
- **Messages**: Unread messages may indicate blocked workers
- **Cells**: In-progress cells are actively being worked on

## Quick Health Check

Run all three tools to get a complete picture of forge state.
If any workers are blocked, read their messages and respond.
