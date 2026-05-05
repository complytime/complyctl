---
description: Cross-project forge coordination patterns
tags: [forge, global, coordination]
---

# Forge Global

Patterns for forge coordination that apply across projects.

## When to Forge

Use a forge when:
- Task touches 3+ files
- Task has independent subtasks that can parallelize
- Task benefits from specialized workers (e.g., tests vs implementation)

Don't forge when:
- Task is a single-file change
- Task requires sequential steps with tight coupling
- Task is exploratory or investigative

## File Reservation Protocol

1. Workers MUST call `comms_reserve(paths=[...])` before editing
2. Reservations are exclusive by default
3. Set `ttl_seconds` to auto-release after timeout
4. Always release when done: `comms_release(paths=[...])`
5. Coordinator can emergency release: `comms_release_all()`

## Worker Spawning

Each worker gets:
- A bead ID (cell in the org)
- An epic ID (parent cell)
- A list of assigned files
- Shared context from the coordinator

Workers operate independently and report back via comms.

## Broadcast

Coordinator can broadcast context updates to all workers:

```
forge_broadcast(
  project_path=".",
  agent_name="coordinator",
  epic_id="<id>",
  message="API contract changed, update imports"
)
```
