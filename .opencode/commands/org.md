---
description: Query and manage work items (cells)
---

# /org

Manage cells with `org_*` tools.

## Common Actions

- List ready: `org_ready()`
- Query by status: `org_cells(status="open")`
- Create: `org_create(title="...", type="task", priority=1)`
- Update: `org_update(id="...", status="in_progress")`
- Close: `org_close(id="...", reason="Done")`
- Start: `org_start(id="...")`

## Usage

- `/org` — show ready cells
- `/org create "Fix auth bug"` — create a cell
- `/org close <id> "Done"` — close a cell
- `/org status` — show in-progress cells

## Epics

Create an epic with subtasks in one call:

```
org_create_epic(epic_title="...", subtasks=[{title: "...", files: [...]}])
```

## Sessions

- `org_session_start()` — begin a work session, get previous handoff notes
- `org_session_end(handoff_notes="...")` — end session with context for next agent
