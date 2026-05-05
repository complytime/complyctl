---
description: Multi-agent coordination patterns for forge sessions
tags: [forge, coordination, multi-agent]
---

# Forge Coordination

Patterns for coordinating parallel agent work.

## Coordinator Protocol

1. **Initialize**: `comms_init(project_path=".", task_description="...")`
2. **Check learnings**: `hivemind_find(query="<task keywords>")`
3. **Select strategy**: `forge_get_strategy_insights(task="<task>")`
4. **Decompose**: `forge_decompose(task="<task>", context="<learnings>")`
5. **Create epic**: `org_create_epic(epic_title="<task>", subtasks=[...])`
6. **Spawn workers**: `forge_spawn_subtask(bead_id, epic_id, subtask_title, files)`
7. **Monitor**: `comms_inbox()` + `forge_status(epic_id, project_key)`
8. **Review**: `forge_review(task_id, files_touched)` for each worker
9. **Complete**: `forge_complete(bead_id, summary, files_touched)`
10. **Learn**: `hivemind_store(information="...", tags="forge,<topic>")`

## Worker Protocol

1. **Initialize**: `comms_init(project_path=".", task_description="...")`
2. **Check learnings**: `hivemind_find(query="<task keywords>")`
3. **Reserve files**: `comms_reserve(paths=[...], reason="...")`
4. **Implement**: Make changes to reserved files
5. **Report progress**: `forge_progress(bead_id, progress_percent, status)`
6. **Store learnings**: `hivemind_store(information="...", tags="...")`
7. **Complete**: `forge_complete(bead_id, summary, files_touched)`

## File Reservation Rules

- Workers MUST reserve files before editing
- Coordinators NEVER reserve files
- Use `comms_reserve(paths=[...], exclusive=true)` for exclusive access
- Release files when done: `comms_release(paths=[...])`
- Emergency release: `comms_release_all()` (coordinator only)

## Progress Reporting

Report at milestones: 25%, 50%, 75%, 100%

```
forge_progress(
  project_key="replicator",
  agent_name="worker-1",
  bead_id="<id>",
  status="in_progress",
  progress_percent=50,
  message="Implemented core logic, starting tests"
)
```

## Conflict Resolution

If a file reservation fails:
1. Check who holds the reservation
2. Send a message via `comms_send` to negotiate
3. Wait for release or escalate to coordinator
