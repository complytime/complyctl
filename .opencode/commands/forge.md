---
description: Decompose task into subtasks and coordinate parallel agents
---

# /forge

Decompose a task and spawn parallel workers.

## Task

$ARGUMENTS

## Workflow

1. Initialize comms: `comms_init(project_path=".", task_description="Forge: <task>")`
2. Check prior learnings: `hivemind_find(query="<task keywords>")`
3. Decompose: `forge_decompose(task="<task>", context="<learnings>")`
4. Create epic: `org_create_epic(epic_title="<task>", subtasks=[...])`
5. For each subtask: `forge_spawn_subtask(bead_id, epic_id, subtask_title, files)`
6. Monitor: check `comms_inbox()` every few minutes
7. Review: `forge_review(task_id, files_touched)` for each completed worker
8. Complete: `forge_complete(bead_id, summary, files_touched)`
9. Store learnings: `hivemind_store(information="...", tags="forge,<topic>")`

## Rules

- Always create a forge, even for small tasks
- Coordinator orchestrates, workers execute
- Workers reserve their own files via `comms_reserve`
- Check inbox regularly for blocked workers
- Review every worker's output before marking complete
- Store learnings after completion

## Strategy Selection

Before decomposing, check historical success rates:

```
forge_get_strategy_insights(task="<task>")
```

Choose from: `file-based`, `feature-based`, `risk-based`, or `auto`.

## Monitoring

While workers are active:

1. `comms_inbox()` — check for messages from workers
2. `forge_status(epic_id, project_key)` — check worker progress
3. `org_cells(status="in_progress")` — see active cells

## Completion

After all workers finish:

1. `forge_complete(bead_id, summary, files_touched)` — mark epic done
2. `forge_record_outcome(bead_id, duration_ms, success)` — record for learning
3. `hivemind_store(information="...", tags="forge,<topic>")` — store learnings
4. `org_sync()` — persist state to git

## Error Recovery

If a worker is blocked:

1. Read the worker's message: `comms_read_message(message_id)`
2. Acknowledge: `comms_ack(message_id)`
3. Either unblock or reassign the subtask
